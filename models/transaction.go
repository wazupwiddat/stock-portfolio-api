package models

import (
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

func Create(db *gorm.DB, t *Transaction) (uint, error) {
	err := db.Create(&t).Error
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

// FetchTransactionsByAccountIDAndSymbol fetches transactions by account ID and optional symbol
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

// AfterSave hook to enforce the position's open status and update cost basis
func (t *Transaction) AfterSave(tx *gorm.DB) (err error) {
	var position Position
	if err := tx.First(&position, t.Position.ID).Error; err != nil {
		return err
	}

	netQuantity, err := position.CalculateNetQuantity(tx)
	if err != nil {
		return err
	}

	totalCost, err := position.CalculateTotalCost(tx)
	if err != nil {
		return err
	}

	var opened bool
	if (netQuantity > 0 && position.Quantity > 0) || (netQuantity < 0 && position.Quantity < 0) {
		opened = true
	} else {
		opened = false
	}

	costBasis := 0.0
	if netQuantity != 0 {
		costBasis = totalCost / netQuantity
	}

	return tx.Model(&position).Updates(Position{Opened: opened, Quantity: netQuantity, CostBasis: costBasis}).Error
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
