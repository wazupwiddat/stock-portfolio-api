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

// BeforeCreate hook to adjust transaction values and ensure the position exists before creating the transaction
func (t *Transaction) BeforeCreate(tx *gorm.DB) error {
	// Adjust transaction values
	if strings.Contains(strings.ToLower(t.Action), "buy") && t.Amount > 0 {
		t.Amount = -t.Amount
	}
	if strings.Contains(strings.ToLower(t.Action), "sell") && t.Quantity > 0 {
		t.Quantity = -t.Quantity
	}

	// Handle Stock Split, price needs to be 0 in order to calculate CostBasis on the position correctly
	if t.Action == "Stock Split" {
		t.Price = 0
	}

	// Handle Reverse Split specifically
	if t.Action == "Reverse Split" && strings.Contains(t.Description, "XXXREVERSE SPLIT EFF:") {
		// Extract the company name from the description
		companyName := strings.Split(t.Description, " XXXREVERSE SPLIT EFF:")[0]

		var transactions []Transaction
		if err := tx.Model(&Transaction{}).Where("description = ? AND account_id = ?", companyName, t.AccountID).Find(&transactions).Error; err != nil {
			return err
		}

		// Find the first matching transaction and update the symbol of the reverse split transaction
		if len(transactions) > 0 {
			t.Symbol = transactions[0].Symbol
		}
	}

	var position Position

	// Check if there's an existing open position with the same symbol and account
	if err := tx.Where("symbol = ? AND opened = ? AND account_id = ?", t.Symbol, true, t.AccountID).First(&position).Error; err == gorm.ErrRecordNotFound {
		// If no open position exists, create a new one

		underlyingSymbol := t.Symbol
		parts := strings.Split(t.Symbol, " ")
		if len(parts) > 1 {
			underlyingSymbol = parts[0]
		}
		position = Position{
			Symbol:           t.Symbol,
			UnderlyingSymbol: underlyingSymbol, // Assuming UnderlyingSymbol is the same as Symbol here
			Quantity:         t.Quantity,
			CostBasis:        t.Price * t.Quantity,
			Opened:           t.Quantity != 0,
			AccountID:        t.AccountID, // Ensure AccountID is set
		}
		if err := tx.Create(&position).Error; err != nil {
			return err
		}

		// Link the transaction to the newly created position
		t.PositionID = position.ID
	} else if err != nil {
		return err
	} else {
		// Link the transaction to the position
		t.PositionID = position.ID
	}

	return nil
}

// AfterSave hook to enforce the position's open status, update cost basis, and calculate gain/loss
func (t *Transaction) AfterSave(tx *gorm.DB) error {
	var position Position

	// Load the existing position
	if err := tx.First(&position, t.PositionID).Error; err != nil {
		return err
	}

	// Handle Options Forward Split
	if t.Action == "Options Frwd Split" {
		newSymbolParts := strings.Split(t.Symbol, " ")
		if len(newSymbolParts) < 4 {
			return fmt.Errorf("invalid option symbol format: %s", t.Symbol)
		}

		underlyingSymbol := newSymbolParts[0]
		expirationDate := newSymbolParts[1]
		optionType := newSymbolParts[3]

		// Find the old position using LIKE
		var oldPosition Position
		if err := tx.Where("symbol LIKE ? AND account_id = ? AND opened = ?", fmt.Sprintf("%s %s %% %s", underlyingSymbol, expirationDate, optionType), t.AccountID, true).First(&oldPosition).Error; err != nil {
			return err
		}

		// Move all transactions from the old position to the new position and update their symbols
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

		// Delete the old position
		if err := tx.Delete(&oldPosition).Error; err != nil {
			return err
		}
	}

	// Update the existing position
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
		// Calculate GainLoss when the position is closed
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

// AfterDelete hook to update the position after a transaction is deleted
func (t *Transaction) AfterDelete(tx *gorm.DB) error {
	var position Position

	// Load the existing position
	if err := tx.First(&position, t.PositionID).Error; err != nil {
		return err
	}

	// Check if the position has any other transactions
	var count int64
	tx.Model(&Transaction{}).Where("position_id = ?", t.PositionID).Count(&count)
	if count == 0 {
		// No more transactions for this position, delete the position
		if err := tx.Delete(&Position{}, t.PositionID).Error; err != nil {
			return err
		}
	} else {
		// Recalculate position attributes
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
			// Calculate GainLoss when the position is closed
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
