package models

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	ID           uint   `gorm:"primary_key"`
	Email        string `gorm:"size:100;unique;not null"`
	PasswordHash string `gorm:"size:100"`
}

const (
	UniqueConstraintEmail = "users_email_key"
)

type EmailDuplicateError struct {
	Email string
}

func (e *EmailDuplicateError) Error() string {
	return fmt.Sprintf("Email '%s' already exists", e.Email)
}

func CreateUser(db *gorm.DB, u *User) (uint, error) {
	err := db.Create(u).Error
	if err != nil {
		return 0, &EmailDuplicateError{Email: u.Email}
	}
	return u.ID, nil
}

type EmailNotExistsError struct{}

func (*EmailNotExistsError) Error() string {
	return "email not exists"
}

type UserIDDoesNotExistError struct{}

func (*UserIDDoesNotExistError) Error() string {
	return "user by id does not exist"
}

func FindUserByEmail(db *gorm.DB, email string) (*User, error) {
	var user User
	res := db.Find(&user, &User{Email: email})
	if errors.Is(res.Error, gorm.ErrRecordNotFound) {
		return nil, &EmailNotExistsError{}
	}
	return &user, nil
}

func FindUserByID(db *gorm.DB, id uint) (*User, error) {
	var user User
	res := db.Find(&user, &User{ID: id})
	if errors.Is(res.Error, gorm.ErrRecordNotFound) {
		return nil, &UserIDDoesNotExistError{}
	}
	return &user, nil
}
