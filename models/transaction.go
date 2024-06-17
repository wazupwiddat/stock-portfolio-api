package models

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Transaction struct {
	gorm.Model
	ID          uint      `gorm:"primaryKey"`
	Date        time.Time `gorm:"type:date"`
	Action      string    `gorm:"size:50"`
	Symbol      string    `gorm:"size:50"`
	Description string    `gorm:"size:250"`
	Quantity    float64
	Price       float64
	Fees        float64
	Amount      float64
	AccountID   uint    `gorm:"index"`
	Account     Account `gorm:"foreignKey:AccountID"`
	// Remove PositionID and Position fields
	Processed bool `gorm:"default:false"` // Add this field
}

// MarshalJSON customizes the JSON representation of the Transaction struct
func (t Transaction) MarshalJSON() ([]byte, error) {
	type Alias Transaction
	return json.Marshal(&struct {
		Alias
		Date string `json:"Date"`
	}{
		Alias: (Alias)(t),
		Date:  t.Date.Format("01/02/2006"),
	})
}

// DeleteTransaction deletes a transaction by ID and removes the associated position if no other transactions exist
func DeleteTransaction(db *gorm.DB, id uint) error {
	var transaction Transaction
	if err := db.First(&transaction, id).Error; err != nil {
		return err
	}

	// Delete the transaction
	if err := db.Delete(&transaction).Error; err != nil {
		return err
	}

	return nil
}

// FindTransactionByID fetches a transaction by ID and includes the associated account information
func FindTransactionByID(db *gorm.DB, id uint) (*Transaction, error) {
	var transaction Transaction
	result := db.Preload("Account").First(&transaction, id)
	if result.Error != nil {
		return nil, result.Error
	}
	return &transaction, nil
}

// Create creates a new transaction in the database
func Create(db *gorm.DB, t *Transaction) (uint, error) {
	err := db.Create(t).Error
	if err != nil {
		return 0, err
	}
	return t.ID, nil
}

// CreateMany creates multiple transactions in the database
func CreateMany(db *gorm.DB, trans []Transaction) error {
	return db.Create(trans).Error
}

// FindAllByAccount fetches all transactions for a given account
func FindAllByAccount(db *gorm.DB, a *Account) ([]Transaction, error) {
	var transactions []Transaction
	res := db.Where("account_id = ?", a.ID).Order("date DESC").Find(&transactions)
	if res.Error != nil {
		return nil, res.Error
	}
	return transactions, nil
}

// FetchTransactionsByAccountIDAndSymbol fetches transactions by account ID and symbol with pagination
func FetchTransactionsByAccountIDAndSymbol(db *gorm.DB, accountID uint, symbol string, page, limit int) ([]Transaction, error) {
	var transactions []Transaction

	query := db.Where("account_id = ?", accountID)
	if symbol != "" {
		query = query.Where("symbol = ?", symbol)
	}

	offset := (page - 1) * limit
	if err := query.Order("date DESC").Offset(offset).Limit(limit).Find(&transactions).Error; err != nil {
		return nil, err
	}

	return transactions, nil
}

// CountTransactionsByAccountIDAndSymbol counts the number of transactions by account ID and optional symbol
func CountTransactionsByAccountIDAndSymbol(db *gorm.DB, accountID uint, symbol string) (int64, error) {
	var count int64
	query := db.Model(&Transaction{}).Where("account_id = ?", accountID)
	if symbol != "" {
		query = query.Where("symbol = ?", symbol)
	}
	err := query.Count(&count).Error
	return count, err
}

// GetLastTransactionDate returns the date of the last transaction for the given account
func GetLastTransactionDate(db *gorm.DB, accountID uint) (time.Time, error) {
	var lastTransaction Transaction
	err := db.Where("account_id = ?", accountID).Order("date DESC").First(&lastTransaction).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// If no transactions are found, return a zero time
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return lastTransaction.Date, nil
}

// GeneratePositions generates positions based on the current transactions in the database
func GeneratePositions(db *gorm.DB, accountID uint) error {
	var transactions []Transaction
	if err := db.Where("account_id = ?", accountID).Order("date ASC").Find(&transactions).Error; err != nil {
		return err
	}

	// Let's just rip through and handle Splits, Reverse Splits
	for _, t := range transactions {
		if t.Processed {
			continue
		}
		AdjustTransactionValues(db, t)
		if t.Action == "Stock Split" {
			HandleStockSplit(db, t)
		}
		if t.Action == "Reverse Split" {
			err := HandleReverseSplit(db, t)
			if err != nil {
				log.Println("Handle Reverse Split: ", err)
				continue
			}
		}
		if t.Action == "Options Frwd Split" {
			err := HandleOptionsForwardSplit(db, t)
			if err != nil {
				log.Println("Handle Options Forward Split: ", err)
				continue
			}
		}
	}

	// Reload transaction now that Splits are handled.
	if err := db.Where("account_id = ?", accountID).Order("date ASC").Find(&transactions).Error; err != nil {
		return err
	}

	// this is destructive but ok :)
	if err := db.Unscoped().Where("account_id = ?", accountID).Delete(Position{}).Error; err != nil {
		return err
	}

	positions := make(map[string]*Position)

	for _, t := range transactions {
		if _, exists := positions[t.Symbol]; !exists {
			if !validOpenTransaction(t) {
				continue
			}
			positions[t.Symbol] = &Position{
				Symbol:           t.Symbol,
				UnderlyingSymbol: strings.Split(t.Symbol, " ")[0],
				AccountID:        accountID,
				OpenDate:         t.Date,
				Short:            t.Quantity < 0,
			}
		}

		pos := positions[t.Symbol]
		pos.Quantity += t.Quantity
		pos.CostBasis += t.Price * t.Quantity
		pos.Transactions = append(pos.Transactions, t)
	}

	for _, pos := range positions {
		pos.Opened = pos.Quantity != 0
		pos.CostBasis = 0.0
		if pos.Opened {
			pos.CostBasis = pos.CalculateTotalCost() / pos.Quantity
		} else {
			pos.GainLoss = pos.CalculateNetAmount()
		}

		// Save or update the position
		if err := db.Save(pos).Error; err != nil {
			return err
		}
	}

	return nil
}

func HandleOptionsForwardSplit(db *gorm.DB, t Transaction) error {
	newSymbolParts := strings.Split(t.Symbol, " ")
	if len(newSymbolParts) < 4 {
		return fmt.Errorf("invalid option symbol format: %s", t.Symbol)
	}

	underlyingSymbol := newSymbolParts[0]
	expirationDate := newSymbolParts[1]
	newStrikePriceStr := newSymbolParts[2]
	optionType := newSymbolParts[3]

	// Convert new strike price to float64
	newStrikePrice, err := strconv.ParseFloat(newStrikePriceStr, 64)
	if err != nil {
		return fmt.Errorf("invalid new strike price: %s", newStrikePriceStr)
	}

	// Find the stock split data
	var stockSplit StockSplit
	if err := db.Where("symbol = ? AND split_date <= ?", underlyingSymbol, t.Date).Order("split_date DESC").First(&stockSplit).Error; err != nil {
		return fmt.Errorf("failed to find the stock split data: %v", err)
	}

	// Determine the split ratio based on the stock split data
	ratio := stockSplit.SplitRatio

	// Find old transactions that match the underlying symbol, expiration date, and option type
	var oldTransactions []Transaction
	query := fmt.Sprintf("%s %s %% %s", underlyingSymbol, expirationDate, optionType)
	if err := db.Model(&Transaction{}).Where("symbol LIKE ? AND account_id = ? AND date < ?", query, t.AccountID, t.Date).Find(&oldTransactions).Error; err != nil {
		return err
	}

	for _, transaction := range oldTransactions {
		// Parse the old transaction symbol to get its parts
		oldSymbolParts := strings.Split(transaction.Symbol, " ")
		if len(oldSymbolParts) < 4 {
			continue
		}
		oldStrikePriceStr := oldSymbolParts[2]
		oldStrikePrice, err := strconv.ParseFloat(oldStrikePriceStr, 64)
		if err != nil {
			return fmt.Errorf("invalid new strike price: %s", newStrikePriceStr)
		}
		roundedOldStrikePrice := math.Round(newStrikePrice * ratio)
		// Only update the symbol if the old strike price matches the expected one
		if oldStrikePrice == roundedOldStrikePrice {
			transaction.Symbol = t.Symbol

			if err := db.Save(&transaction).Error; err != nil {
				return err
			}
		}
	}

	t.Processed = true
	if err := db.Save(&t).Error; err != nil {
		return err
	}
	return nil
}

// AdjustTransactionValues adjusts transaction values based on the action type
func AdjustTransactionValues(db *gorm.DB, t Transaction) error {
	if strings.Contains(strings.ToLower(t.Action), "buy") && t.Amount > 0 {
		t.Amount = -t.Amount
	}
	if strings.Contains(strings.ToLower(t.Action), "sell") && t.Quantity > 0 {
		t.Quantity = -t.Quantity
	}
	t.Processed = true
	if err := db.Save(&t).Error; err != nil {
		return err
	}
	return nil
}

// HandleStockSplit adjusts the price for stock split transactions
func HandleStockSplit(db *gorm.DB, t Transaction) error {
	t.Price = 0
	t.Processed = true
	if err := db.Save(&t).Error; err != nil {
		return err
	}
	return nil
}

// HandleReverseSplit handles reverse split transactions
func HandleReverseSplit(db *gorm.DB, t Transaction) error {
	if strings.Contains(t.Description, "XXXREVERSE SPLIT EFF:") {
		companyName := strings.Split(t.Description, " XXXREVERSE SPLIT EFF:")[0]

		var transactions []Transaction
		if err := db.Model(&Transaction{}).Where("description = ? AND account_id = ?", companyName, t.AccountID).Find(&transactions).Error; err != nil {
			return err
		}

		if len(transactions) > 0 {
			t.Symbol = transactions[0].Symbol
		}
		t.Processed = true
		if err := db.Save(&t).Error; err != nil {
			return err
		}
	}
	return nil
}

func validOpenTransaction(t Transaction) bool {
	// This transaction needs to be a valid Opening
	// Buy, Sell Short, Sell to Open, Buy to Open
	openActions := []string{"buy", "sell short", "sell to open", "buy to open", "reverse split"}
	for _, a := range openActions {
		if strings.ToLower(t.Action) == a {
			return true
		}
	}
	return false
}
