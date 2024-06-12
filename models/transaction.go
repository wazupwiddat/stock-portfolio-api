package models

import (
	"encoding/json"
	"fmt"
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
	PositionID  uint
	Position    Position `gorm:"foreignKey:PositionID"`
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

// AdjustTransactionValues adjusts transaction values based on the action type
func AdjustTransactionValues(t *Transaction) {
	if strings.Contains(strings.ToLower(t.Action), "buy") && t.Amount > 0 {
		t.Amount = -t.Amount
	}
	if strings.Contains(strings.ToLower(t.Action), "sell") && t.Quantity > 0 {
		t.Quantity = -t.Quantity
	}
}

// HandleStockSplit adjusts the price for stock split transactions
func HandleStockSplit(t *Transaction) {
	if t.Action == "Stock Split" {
		t.Price = 0
	}
}

// HandleReverseSplit handles reverse split transactions
func HandleReverseSplit(tx *gorm.DB, t *Transaction) error {
	if t.Action == "Reverse Split" && strings.Contains(t.Description, "XXXREVERSE SPLIT EFF:") {
		companyName := strings.Split(t.Description, " XXXREVERSE SPLIT EFF:")[0]

		var transactions []Transaction
		if err := tx.Model(&Transaction{}).Where("description = ? AND account_id = ?", companyName, t.AccountID).Find(&transactions).Error; err != nil {
			return err
		}

		if len(transactions) > 0 {
			t.Symbol = transactions[0].Symbol
		}
	}
	return nil
}

// EnsurePositionExists ensures that a position exists for the given transaction
func EnsurePositionExists(tx *gorm.DB, t *Transaction) error {
	var position Position

	// Check if there's an existing open position with the same symbol and account
	if err := tx.Where("symbol = ? AND opened = ? AND account_id = ?", t.Symbol, true, t.AccountID).First(&position).Error; err == gorm.ErrRecordNotFound {
		if strings.ToLower(t.Action) == "sell" || t.Action == "Buy to Close" || t.Action == "Sell to Close" {
			return fmt.Errorf("cannot %s without an existing open position for symbol %s", t.Action, t.Symbol)
		}
		// If no open position exists and the action is not sell-related, create a new one
		underlyingSymbol := t.Symbol
		parts := strings.Split(t.Symbol, " ")
		if len(parts) > 1 {
			underlyingSymbol = parts[0]
		}
		position = Position{
			Symbol:           t.Symbol,
			UnderlyingSymbol: underlyingSymbol,
			Quantity:         t.Quantity,
			CostBasis:        t.Price * t.Quantity,
			Opened:           t.Quantity != 0,
			AccountID:        t.AccountID, // Ensure AccountID is set
		}
		if err := tx.Create(&position).Error; err != nil {
			return err
		}
		t.PositionID = position.ID
	} else if err != nil {
		return err
	} else {
		t.PositionID = position.ID
	}

	return nil
}

// UpdatePosition updates the position based on the transaction
func UpdatePosition(tx *gorm.DB, t *Transaction) error {
	var position Position

	if err := tx.First(&position, t.PositionID).Error; err != nil {
		return err
	}

	if t.Action == "Options Frwd Split" {
		newSymbolParts := strings.Split(t.Symbol, " ")
		if len(newSymbolParts) < 4 {
			return fmt.Errorf("invalid option symbol format: %s", t.Symbol)
		}

		underlyingSymbol := newSymbolParts[0]
		expirationDate := newSymbolParts[1]
		optionType := newSymbolParts[3]

		var oldPosition Position
		if err := tx.Where("symbol LIKE ? AND account_id = ? AND opened = ?", fmt.Sprintf("%s %s %% %s", underlyingSymbol, expirationDate, optionType), t.AccountID, true).First(&oldPosition).Error; err != nil {
			return err
		}

		var transactions []Transaction
		if err := tx.Model(&Transaction{}).Where("position_id = ?", oldPosition.ID).Find(&transactions).Error; err != nil {
			return err
		}
		for _, transaction := range transactions {
			transaction.PositionID = t.PositionID
			transaction.Symbol = t.Symbol
			if err := tx.Save(&transaction).Error; err != nil {
				return err
			}
		}

		if err := tx.Delete(&oldPosition).Error; err != nil {
			return err
		}
	}

	netQuantity, err := position.CalculateNetQuantity(tx)
	if err != nil {
		return err
	}

	totalCost, err := position.CalculateTotalCost(tx)
	if err != nil {
		return err
	}

	opened := netQuantity != 0
	costBasis := 0.0
	gainLoss := 0.0

	if netQuantity != 0 {
		costBasis = totalCost / netQuantity
	} else {
		result := tx.Model(&Transaction{}).Where("position_id = ?", position.ID).Select("SUM(amount)").Scan(&gainLoss)
		if result.Error != nil {
			return result.Error
		}
	}

	position.Quantity = netQuantity
	position.CostBasis = costBasis
	position.Opened = opened
	position.GainLoss = gainLoss

	return tx.Save(&position).Error
}

// RecalculatePositionAttributes recalculates the attributes of a position after a transaction is deleted
func RecalculatePositionAttributes(tx *gorm.DB, t *Transaction) error {
	var position Position

	if err := tx.First(&position, t.PositionID).Error; err != nil {
		return err
	}

	var count int64
	tx.Model(&Transaction{}).Where("position_id = ?", t.PositionID).Count(&count)
	if count == 0 {
		if err := tx.Delete(&Position{}, t.PositionID).Error; err != nil {
			return err
		}
	} else {
		netQuantity, err := position.CalculateNetQuantity(tx)
		if err != nil {
			return err
		}

		totalCost, err := position.CalculateTotalCost(tx)
		if err != nil {
			return err
		}

		opened := netQuantity != 0
		costBasis := 0.0
		gainLoss := 0.0

		if netQuantity != 0 {
			costBasis = totalCost / netQuantity
		} else {
			result := tx.Model(&Transaction{}).Where("position_id = ?", position.ID).Select("SUM(amount)").Scan(&gainLoss)
			if result.Error != nil {
				return result.Error
			}
		}

		position.Quantity = netQuantity
		position.CostBasis = costBasis
		position.Opened = opened
		position.GainLoss = gainLoss

		if err := tx.Save(&position).Error; err != nil {
			return err
		}
	}

	return nil
}

// ValidateAndAdjustTransaction validates and adjusts the transaction quantity if necessary
func ValidateAndAdjustTransaction(tx *gorm.DB, t *Transaction) error {
	var position Position
	if err := tx.First(&position, t.PositionID).Error; err != nil {
		return err
	}

	// Action = Sell
	//  the transaction will have a quanity of some negative number like -15 and the position will have a positive Quantity like 10.
	// Action = Buy to Close
	//  the transaction will have a quanity of a positive number like 15 and the position will be -10
	// Sell to Close will be similar to Sell

	// Adjust the transaction quantity if it exceeds the position quantity
	action := strings.ToLower(t.Action)
	if (action == "sell" && t.Quantity < 0 && -t.Quantity > position.Quantity) ||
		(action == "sell to close") && t.Quantity < 0 && -t.Quantity > position.Quantity ||
		(action == "Buy to Close" && t.Quantity > 0 && t.Quantity > -position.Quantity) {
		t.Amount = position.Quantity * t.Price
		t.Quantity = -position.Quantity
	}

	return nil
}
