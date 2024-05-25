package models

import "gorm.io/gorm"

type Account struct {
	gorm.Model
	ID           uint   `gorm:"primaryKey"`
	UserID       uint   `gorm:"index"`
	User         User   `gorm:"foreignKey:UserID"`
	Name         string `gorm:"size:100"`
	Balance      float64
	Positions    []Position    `gorm:"foreignKey:AccountID"`
	Transactions []Transaction `gorm:"foreignKey:AccountID"`
}

// CreateAccount creates a new account in the database
func CreateAccount(db *gorm.DB, name string) (*Account, error) {
	account := &Account{
		Name: name,
	}
	result := db.Create(account)
	if result.Error != nil {
		return nil, result.Error
	}
	return account, nil
}

// FindAccountByID finds an account by its ID
func FindAccountByID(db *gorm.DB, id uint) (*Account, error) {
	var account Account
	result := db.First(&account, id)
	if result.Error != nil {
		return nil, result.Error
	}
	return &account, nil
}

// FindAccountByName finds an account by its name
func FindAccountByName(db *gorm.DB, name string) (*Account, error) {
	var account Account
	result := db.Where("name = ?", name).First(&account)
	if result.Error != nil {
		return nil, result.Error
	}
	return &account, nil
}

// FetchAccountsByUserID fetches all accounts for a given user ID
func FetchAccountsByUserID(db *gorm.DB, userID uint) ([]Account, error) {
	var accounts []Account
	result := db.Where("user_id = ?", userID).Find(&accounts)
	return accounts, result.Error
}
