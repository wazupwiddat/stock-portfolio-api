package models

import "gorm.io/gorm"

type Position struct {
	gorm.Model
	ID               uint    `gorm:"primaryKey"`
	Symbol           string  `gorm:"not null"`
	UnderlyingSymbol string  `gorm:"not null"`
	Quantity         float64 `gorm:"not null"`
	CostBasis        float64 `gorm:"not null"`
	Opened           bool    `gorm:"not null"`
	GainLoss         float64
	Transactions     []Transaction `gorm:"foreignKey:PositionID"`
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
	err := db.Joins("JOIN transactions ON transactions.position_id = positions.id").
		Where("transactions.account_id = ?", accountID).
		Group("positions.id").
		Preload("Transactions").
		Find(&positions).Error
	return positions, err
}

// CalculateNetQuantity calculates the net quantity of transactions associated with the position
func (p *Position) CalculateNetQuantity(db *gorm.DB) (float64, error) {
	var totalQuantity float64
	result := db.Model(&Transaction{}).Where("position_id = ?", p.ID).Select("SUM(quantity)").Scan(&totalQuantity)
	if result.Error != nil {
		return 0, result.Error
	}
	return totalQuantity, nil
}

// IsOpen checks if the position is open based on the net quantity of transactions
func (p *Position) IsOpen(db *gorm.DB) (bool, error) {
	netQuantity, err := p.CalculateNetQuantity(db)
	if err != nil {
		return false, err
	}
	if (netQuantity > 0 && p.Quantity > 0) || (netQuantity < 0 && p.Quantity < 0) {
		return true, nil
	}
	return false, nil
}
