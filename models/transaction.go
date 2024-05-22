package models

import (
	"strings"

	"gorm.io/gorm"
)

type Transaction struct {
	gorm.Model
	ID          uint   `gorm:"primaryKey"`
	Date        string `gorm:"size:50"`
	Action      string `gorm:"size:50"`
	Symbol      string `gorm:"size:50"`
	Description string `gorm:"size:250"`
	Quantity    float64
	Price       float64
	Fees        float64
	Amount      float64
	AccountID   uint `gorm:"index"`
	PositionID  uint
	Position    Position `gorm:"foreignKey:PositionID"`
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

	var position Position

	// Check if there's an existing open position with the same symbol
	if err := tx.Where("symbol = ? AND opened = ?", t.Symbol, true).First(&position).Error; err == gorm.ErrRecordNotFound {
		// If no open position exists, create a new one
		position = Position{
			Symbol:           t.Symbol,
			UnderlyingSymbol: t.Symbol, // Assuming UnderlyingSymbol is the same as Symbol here
			Quantity:         t.Quantity,
			CostBasis:        t.Price * t.Quantity,
			Opened:           t.Quantity != 0,
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

// CalculateTotalCost calculates the total cost of transactions associated with the position
func (p *Position) CalculateTotalCost(db *gorm.DB) (float64, error) {
	var totalCost float64
	result := db.Model(&Transaction{}).Where("position_id = ?", p.ID).Select("SUM(price * quantity)").Scan(&totalCost)
	if result.Error != nil {
		return 0, result.Error
	}
	return totalCost, nil
}

func Create(db *gorm.DB, t *Transaction) (uint, error) {
	err := db.Create(t).Error
	if err != nil {
		return 0, err
	}
	return t.ID, nil
}

func CreateMany(db *gorm.DB, trans []Transaction) error {
	return db.Create(trans).Error
}

func FindAllByAccount(db *gorm.DB, a *Account) ([]Transaction, error) {
	var transactions []Transaction
	res := db.Find(&transactions, &Transaction{AccountID: a.ID})
	if res.Error != nil {
		return nil, res.Error
	}
	return transactions, nil
}

func FetchTransactionsByAccountIDAndSymbol(db *gorm.DB, accountID uint, symbol string, page, limit int) ([]Transaction, error) {
	var transactions []Transaction

	query := db.Where("account_id = ?", accountID)
	if symbol != "" {
		query = query.Where("symbol = ?", symbol)
	}

	offset := (page - 1) * limit
	if err := query.Offset(offset).Limit(limit).Find(&transactions).Error; err != nil {
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
