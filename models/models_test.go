package models

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupDB() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	db.AutoMigrate(&Transaction{}, &User{}, &Position{}, &Account{})
	return db, nil
}

func TestTransactionModel(t *testing.T) {
	Convey("Given a database and a transaction", t, func() {
		db, err := setupDB()
		So(err, ShouldBeNil)

		date, _ := time.Parse("2006-01-02", "2023-05-22")

		transaction := Transaction{
			Date:        date,
			Action:      "BUY",
			Symbol:      "AAPL",
			Description: "Apple stock",
			Quantity:    10,
			Price:       150.00,
			Fees:        1.00,
			Amount:      1500.00,
			AccountID:   1,
		}

		Convey("When creating a transaction", func() {
			id, err := Create(db, &transaction)
			So(err, ShouldBeNil)
			So(id, ShouldEqual, transaction.ID)

			Convey("Then it should be retrievable from the database", func() {
				var retrievedTransaction Transaction
				err := db.First(&retrievedTransaction, id).Error
				So(err, ShouldBeNil)
				So(retrievedTransaction.Symbol, ShouldEqual, "AAPL")
			})
		})
	})
}

func TestUserModel(t *testing.T) {
	Convey("Given a database and a user", t, func() {
		db, err := setupDB()
		So(err, ShouldBeNil)

		user := User{
			Email:        "test@example.com",
			PasswordHash: "hashedpassword",
		}

		Convey("When creating a user", func() {
			id, err := CreateUser(db, &user)
			So(err, ShouldBeNil)
			So(id, ShouldEqual, user.ID)

			Convey("Then it should be retrievable from the database", func() {
				var retrievedUser User
				err := db.First(&retrievedUser, id).Error
				So(err, ShouldBeNil)
				So(retrievedUser.Email, ShouldEqual, "test@example.com")
			})
		})
	})
}

func TestAccountModel(t *testing.T) {
	Convey("Given a database and an account", t, func() {
		db, err := setupDB()
		So(err, ShouldBeNil)

		account := Account{
			UserID: 1,
			Name:   "Test Account",
		}

		Convey("When creating an account", func() {
			createdAccount, err := CreateAccount(db, account.Name)
			So(err, ShouldBeNil)
			So(createdAccount.Name, ShouldEqual, "Test Account")

			Convey("Then it should be retrievable from the database", func() {
				var retrievedAccount Account
				err := db.First(&retrievedAccount, createdAccount.ID).Error
				So(err, ShouldBeNil)
				So(retrievedAccount.Name, ShouldEqual, "Test Account")
			})
		})
	})
}

func TestPositionModel(t *testing.T) {
	Convey("Given a database and positions", t, func() {
		db, err := setupDB()
		So(err, ShouldBeNil)

		Convey("When creating a new transaction with no open positions for the symbol", func() {
			date, _ := time.Parse("2006-01-02", "2023-05-22")

			transaction := Transaction{
				Date:        date,
				Action:      "BUY",
				Symbol:      "AAPL",
				Description: "Apple stock",
				Quantity:    10,
				Price:       150.00,
				Fees:        1.00,
				Amount:      -1500.00,
				AccountID:   1,
			}

			id, err := Create(db, &transaction)
			So(err, ShouldBeNil)
			So(id, ShouldEqual, transaction.ID)

			Convey("Then a new position should be created", func() {
				var position Position
				err := db.First(&position, transaction.PositionID).Error
				So(err, ShouldBeNil)
				So(position.Symbol, ShouldEqual, "AAPL")
				So(position.Quantity, ShouldEqual, 10)
				So(position.Opened, ShouldBeTrue)
				So(position.GainLoss, ShouldEqual, 0) // Initially, GainLoss should be 0

				Convey("When adding a new transaction that sums the quantity to 0", func() {
					date2, _ := time.Parse("2006-01-02", "2023-05-23")

					newTransaction := Transaction{
						Date:        date2,
						Action:      "SELL",
						Symbol:      "AAPL",
						Description: "Apple stock",
						Quantity:    -10,
						Price:       160.00,
						Fees:        1.00,
						Amount:      1600.00,
						AccountID:   1,
						PositionID:  position.ID,
					}

					newID, err := Create(db, &newTransaction)
					So(err, ShouldBeNil)
					So(newID, ShouldEqual, newTransaction.ID)

					Convey("Then the position should be closed and GainLoss calculated", func() {
						err := db.First(&position, newTransaction.PositionID).Error
						So(err, ShouldBeNil)
						So(position.Symbol, ShouldEqual, "AAPL")

						// net quantity and cost basis
						So(position.CostBasis, ShouldEqual, 0)
						So(position.Quantity, ShouldEqual, 0)
						So(position.Opened, ShouldBeFalse)

						// Check GainLoss
						expectedGainLoss := 100.0 // Expected GainLoss when position is closed
						So(position.GainLoss, ShouldEqual, expectedGainLoss)
					})
				})
			})
		})
	})
}

func TestCreateManyTransactions(t *testing.T) {
	Convey("Given a database and multiple transactions", t, func() {
		db, err := setupDB()
		So(err, ShouldBeNil)

		Convey("When creating multiple transactions", func() {
			date1, _ := time.Parse("2006-01-02", "2023-05-22")
			date2, _ := time.Parse("2006-01-02", "2023-05-23")
			date3, _ := time.Parse("2006-01-02", "2023-05-24")

			transactions := []Transaction{
				{
					Date:        date1,
					Action:      "BUY",
					Symbol:      "GOOG",
					Description: "Google stock",
					Quantity:    5,
					Price:       200.00,
					Fees:        1.00,
					Amount:      1000.00,
					AccountID:   1,
				},
				{
					Date:        date2,
					Action:      "BUY",
					Symbol:      "GOOG",
					Description: "Google stock",
					Quantity:    5,
					Price:       210.00,
					Fees:        1.00,
					Amount:      1050.00,
					AccountID:   1,
				},
				{
					Date:        date3,
					Action:      "SELL",
					Symbol:      "GOOG",
					Description: "Google stock",
					Quantity:    -10,
					Price:       220.00,
					Fees:        1.00,
					Amount:      2200.00,
					AccountID:   1,
				},
			}

			err := CreateMany(db, transactions)
			So(err, ShouldBeNil)

			Convey("Then the transactions should be retrievable from the database", func() {
				var retrievedTransactions []Transaction
				err := db.Where("symbol = ?", "GOOG").Find(&retrievedTransactions).Error
				So(err, ShouldBeNil)
				So(len(retrievedTransactions), ShouldEqual, 3)

				Convey("And the related position should be updated correctly", func() {
					var position Position
					err := db.Where("symbol = ?", "GOOG").First(&position).Error
					So(err, ShouldBeNil)
					So(position.Symbol, ShouldEqual, "GOOG")

					// net quantity and cost basis
					So(position.CostBasis, ShouldEqual, 0)
					So(position.Quantity, ShouldEqual, 0)
					So(position.Opened, ShouldBeFalse)

					expectedGainLoss := 150.0 // Expected GainLoss when position is closed
					So(position.GainLoss, ShouldEqual, expectedGainLoss)
				})
			})

		})
	})
}
