package models

import (
	"time"

	"gorm.io/gorm"
)

type Position struct {
	gorm.Model
	ID               uint      `gorm:"primaryKey"`
	Symbol           string    `gorm:"not null"`
	UnderlyingSymbol string    `gorm:"not null"`
	OpenDate         time.Time `gorm:"type:date"`
	Quantity         float64   `gorm:"not null"`
	CostBasis        float64   `gorm:"not null"`
	Opened           bool      `gorm:"not null"`
	GainLoss         float64
	Short            bool
	AccountID        uint          `gorm:"index"`
	Transactions     []Transaction `gorm:"-"`
}

// FetchAllPositions fetches all positions for a given stock symbol
func FetchAllPositions(db *gorm.DB, symbol string) ([]Position, error) {
	var positions []Position
	result := db.Where("symbol = ?", symbol).Find(&positions)
	return positions, result.Error
}

// FetchOpenPositions fetches all open positions for a given stock symbol
func FetchOpenPositions(db *gorm.DB, symbol string) ([]Position, error) {
	var openPositions []Position
	result := db.Where("symbol = ? AND opened = ?", symbol, true).Find(&openPositions)
	return openPositions, result.Error
}

// FetchPositionsByAccount fetches all positions for a given account
func FetchPositionsByAccount(db *gorm.DB, accountID uint) ([]Position, error) {
	var positions []Position
	err := db.Where("account_id = ?", accountID).Find(&positions).Error
	return positions, err
}

// CalculateNetQuantity calculates the net quantity of transactions associated with the position
func (p *Position) CalculateNetQuantity() float64 {
	var totalQuantity float64
	for _, t := range p.Transactions {
		totalQuantity = totalQuantity + t.Quantity
	}
	return totalQuantity
}

// CalculateNetAmount calculates the net amount of transactions associated with the position
func (p *Position) CalculateNetAmount() float64 {
	var totalAmount float64
	for _, t := range p.Transactions {
		totalAmount = totalAmount + t.Amount
	}
	return totalAmount
}

// IsOpen checks if the position is open based on the net quantity of transactions
func (p *Position) IsOpen() bool {
	netQuantity := p.CalculateNetQuantity()
	if (netQuantity > 0 && p.Quantity > 0) || (netQuantity < 0 && p.Quantity < 0) {
		return true
	}
	return false
}

// CalculateTotalCost calculates the total cost of transactions associated with the position
func (p *Position) CalculateTotalCost() float64 {
	var totalCost float64
	for _, t := range p.Transactions {
		totalCost = totalCost + (t.Price * t.Quantity)
	}
	return totalCost
}
