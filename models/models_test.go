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
	db.AutoMigrate(&Transaction{}, &User{}, &Position{}, &Account{}, &StockSplit{})
	return db, nil
}

func TestTransactionModel(t *testing.T) {
	Convey("Given a database and a transaction", t, func() {
		db, err := setupDB()
		So(err, ShouldBeNil)

		// Create a user and account
		user := User{Email: "test@example.com", PasswordHash: "hashedpassword"}
		db.Create(&user)
		account := Account{UserID: user.ID, Name: "Test Account"}
		db.Create(&account)

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
			AccountID:   account.ID,
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

func TestPositionModel(t *testing.T) {
	Convey("Given a database and positions", t, func() {
		db, err := setupDB()
		So(err, ShouldBeNil)

		// Create a user and account
		user := User{Email: "test@example.com", PasswordHash: "hashedpassword"}
		db.Create(&user)
		account := Account{UserID: user.ID, Name: "Test Account"}
		db.Create(&account)

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
				AccountID:   account.ID,
			}

			id, err := Create(db, &transaction)
			So(err, ShouldBeNil)
			So(id, ShouldEqual, transaction.ID)

			GeneratePositions(db, account.ID)

			Convey("Then a new position should be created", func() {
				positions, err := FetchAllPositions(db, "AAPL")
				So(err, ShouldBeNil)
				So(positions, ShouldHaveLength, 1)
				position := positions[0]
				So(position.Symbol, ShouldEqual, "AAPL")
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
						AccountID:   account.ID,
					}

					newID, err := Create(db, &newTransaction)
					So(err, ShouldBeNil)
					So(newID, ShouldEqual, newTransaction.ID)

					GeneratePositions(db, newTransaction.AccountID)

					Convey("Then the position should be closed and GainLoss calculated", func() {
						positions, err := FetchAllPositions(db, "AAPL")
						So(err, ShouldBeNil)
						So(positions, ShouldHaveLength, 1)
						position := positions[0]
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

		// Create a user and account
		user := User{Email: "test@example.com", PasswordHash: "hashedpassword"}
		db.Create(&user)
		account := Account{UserID: user.ID, Name: "Test Account"}
		db.Create(&account)

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
					Amount:      -1000.00,
					AccountID:   account.ID,
				},
				{
					Date:        date2,
					Action:      "BUY",
					Symbol:      "GOOG",
					Description: "Google stock",
					Quantity:    5,
					Price:       210.00,
					Fees:        1.00,
					Amount:      -1050.00,
					AccountID:   account.ID,
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
					AccountID:   account.ID,
				},
			}

			err := CreateMany(db, transactions)
			So(err, ShouldBeNil)

			GeneratePositions(db, account.ID)

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

func TestDeleteTransaction(t *testing.T) {
	Convey("Given a database with a transaction", t, func() {
		db, err := setupDB()
		So(err, ShouldBeNil)

		// Create a user and account
		user := User{Email: "test@example.com", PasswordHash: "hashedpassword"}
		db.Create(&user)
		account := Account{UserID: user.ID, Name: "Test Account"}
		db.Create(&account)

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
			AccountID:   account.ID,
		}

		id, err := Create(db, &transaction)
		So(err, ShouldBeNil)
		So(id, ShouldEqual, transaction.ID)

		GeneratePositions(db, account.ID)

		Convey("When deleting the transaction", func() {
			err := DeleteTransaction(db, transaction.ID)
			So(err, ShouldBeNil)

			GeneratePositions(db, account.ID)

			Convey("Then the transaction should be removed from the database", func() {
				var count int64
				db.Model(&Transaction{}).Where("id = ?", transaction.ID).Count(&count)
				So(count, ShouldEqual, 0)
			})

			Convey("And the position should also be removed if it has no other transactions", func() {
				var position Position
				err := db.Where("symbol = ?", transaction.Symbol).First(&position).Error
				So(err, ShouldEqual, gorm.ErrRecordNotFound)
			})
		})
	})
}

func TestStockSplit(t *testing.T) {
	Convey("Given an existing stock position created through a transaction", t, func() {
		db, err := setupDB()
		So(err, ShouldBeNil)

		account := Account{ID: 1, Name: "Test Account", UserID: 1}
		db.Create(&account)

		initialTransaction := Transaction{
			Date:      time.Now().AddDate(0, -1, 0),
			Action:    "Buy",
			Symbol:    "TSLA",
			Quantity:  200,
			Price:     300,
			Amount:    60000,
			AccountID: account.ID,
		}

		id, err := Create(db, &initialTransaction)
		So(err, ShouldBeNil)
		So(id, ShouldEqual, initialTransaction.ID)

		GeneratePositions(db, account.ID)

		transaction := Transaction{
			Date:      time.Now(),
			Action:    "Stock Split",
			Symbol:    "TSLA",
			Quantity:  400, // The split quantity added
			Price:     200, // This should be 0'd out before we create "Stock Split transaction"
			AccountID: account.ID,
		}

		Convey("When a stock split transaction is processed", func() {
			id, err := Create(db, &transaction)
			So(err, ShouldBeNil)
			So(id, ShouldEqual, transaction.ID)

			GeneratePositions(db, account.ID)

			var createdTransaction Transaction
			db.Where("action = ? AND symbol = ?", "Stock Split", "TSLA").First(&createdTransaction)

			Convey("A new stock split transaction should be created", func() {
				So(createdTransaction.Quantity, ShouldEqual, 400)
				So(createdTransaction.Price, ShouldEqual, 0)
			})

			Convey("The position quantity and cost basis should be updated correctly", func() {
				var updatedPosition Position
				db.Where("symbol = ?", "TSLA").First(&updatedPosition)

				So(updatedPosition.Quantity, ShouldEqual, 600)  // 200 existing + 400 from split
				So(updatedPosition.CostBasis, ShouldEqual, 100) // Updated cost basis
			})
		})
	})
}

func TestOptionsFrwdSplit(t *testing.T) {
	Convey("Given an existing option position", t, func() {
		db, err := setupDB()
		So(err, ShouldBeNil)

		account := Account{ID: 1, Name: "Test Account", UserID: 1}
		db.Create(&account)

		// Create the stock split data
		stockSplit := StockSplit{
			Symbol:     "TSLA",
			SplitDate:  "2022-08-24",
			SplitRatio: 3,
		}
		err = db.Create(&stockSplit).Error
		So(err, ShouldBeNil)

		// Create initial option transactions
		optionTransactions := []Transaction{
			{
				Date:      time.Date(2022, 8, 10, 0, 0, 0, 0, time.UTC),
				Action:    "Sell to Open",
				Symbol:    "TSLA 01/20/2023 960.00 C",
				Price:     99.76,
				Quantity:  1,
				AccountID: account.ID,
				Amount:    9975.12,
			},
			{
				Date:      time.Date(2021, 10, 26, 0, 0, 0, 0, time.UTC),
				Action:    "Sell to Open",
				Symbol:    "TSLA 01/20/2023 1000.00 C",
				Price:     283.93,
				Quantity:  1,
				AccountID: account.ID,
				Amount:    28392.21,
			},
		}

		for _, trans := range optionTransactions {
			id, err := Create(db, &trans)
			So(err, ShouldBeNil)
			So(id, ShouldEqual, trans.ID)
		}

		GeneratePositions(db, account.ID)

		forwardSplitTrans := Transaction{
			Date:      time.Date(2022, 8, 25, 0, 0, 0, 0, time.UTC),
			Action:    "Options Frwd Split",
			Symbol:    "TSLA 01/20/2023 333.33 C",
			Quantity:  -2,
			AccountID: account.ID,
		}

		Convey("When an options forward split transaction is processed", func() {
			id, err := Create(db, &forwardSplitTrans)
			So(err, ShouldBeNil)
			So(id, ShouldEqual, forwardSplitTrans.ID)

			GeneratePositions(db, account.ID)

			var createdTransaction Transaction
			db.Where("action = ? AND symbol = ?", "Options Frwd Split", "TSLA 01/20/2023 333.33 C").First(&createdTransaction)

			Convey("A new options forward split transaction should be created", func() {
				So(createdTransaction.Quantity, ShouldEqual, -2)
				So(createdTransaction.Symbol, ShouldEqual, "TSLA 01/20/2023 333.33 C")
			})

			Convey("The position should be updated correctly", func() {
				var newPosition Position
				db.Where("symbol = ?", "TSLA 01/20/2023 333.33 C").First(&newPosition)

				So(newPosition.Symbol, ShouldEqual, "TSLA 01/20/2023 333.33 C")
				So(newPosition.Quantity, ShouldEqual, -3) // The split and subsequent buy/sell should result in 3 quantity
				// Cost basis should be adjusted accordingly based on the split ratio
				So(newPosition.CostBasis, ShouldAlmostEqual, 94.64, 0.01)
			})
		})
	})
}

func TestReverseSplit(t *testing.T) {
	Convey("Given an existing stock position", t, func() {
		db, err := setupDB()
		So(err, ShouldBeNil)

		account := Account{ID: 1, Name: "Test Account", UserID: 1}
		db.Create(&account)

		initialTransaction := Transaction{
			Date:        time.Now().AddDate(0, -2, 0),
			Action:      "Buy",
			Symbol:      "ACB",
			Description: "AURORA CANNABIS INC",
			Quantity:    2000,
			Price:       10,
			Amount:      20000,
			AccountID:   account.ID,
		}
		id, err := Create(db, &initialTransaction)
		So(err, ShouldBeNil)
		So(id, ShouldEqual, initialTransaction.ID)

		GeneratePositions(db, account.ID)

		reverseSplitTransaction := Transaction{
			Date:        time.Now().AddDate(0, -1, 0),
			Action:      "Reverse Split",
			Symbol:      "05156X884",
			Description: "AURORA CANNABIS INC XXXREVERSE SPLIT EFF: 02/20/24",
			Quantity:    -2000,
			AccountID:   account.ID,
		}
		id, err = Create(db, &reverseSplitTransaction)
		So(err, ShouldBeNil)
		So(id, ShouldEqual, reverseSplitTransaction.ID)

		GeneratePositions(db, account.ID)

		finalSplitTransaction := Transaction{
			Date:        time.Now(),
			Action:      "Reverse Split",
			Symbol:      "ACBNEW",
			Description: "AURORA NEW HOLDINGS INC",
			Quantity:    200,
			AccountID:   account.ID,
		}

		id, err = Create(db, &finalSplitTransaction)
		So(err, ShouldBeNil)
		So(id, ShouldEqual, finalSplitTransaction.ID)

		GeneratePositions(db, account.ID)

		Convey("When a reverse split transaction is processed", func() {
			var oldPosition Position
			db.Where("symbol = ?", "ACB").First(&oldPosition)

			So(oldPosition.Quantity, ShouldEqual, 0)
			So(oldPosition.Symbol, ShouldEqual, "ACB")
			So(oldPosition.Opened, ShouldBeFalse)

			var updatedPosition Position
			db.Where("symbol = ?", "ACBNEW").First(&updatedPosition)

			So(updatedPosition.Quantity, ShouldEqual, 200)
			So(updatedPosition.Symbol, ShouldEqual, "ACBNEW")
			So(updatedPosition.Opened, ShouldBeTrue)
		})
	})
}

func TestInvalidSellTransactionWithoutPosition(t *testing.T) {
	Convey("Given a database and an invalid sell transaction without a position", t, func() {
		db, err := setupDB()
		So(err, ShouldBeNil)

		account := Account{ID: 1, Name: "Test Account", UserID: 1}
		db.Create(&account)

		transaction := Transaction{
			Date:      time.Now(),
			Action:    "SELL",
			Symbol:    "AAPL",
			Quantity:  -10,
			Price:     150.00,
			Amount:    -1500.00,
			AccountID: account.ID,
		}

		id, err := Create(db, &transaction)
		So(err, ShouldBeNil)
		So(id, ShouldEqual, transaction.ID)

		GeneratePositions(db, account.ID)

		Convey("Transaction is created", func() {
			var transactions []Transaction
			err := db.Where("symbol = ?", "AAPL").Find(&transactions).Error
			So(err, ShouldBeNil)
			So(len(transactions), ShouldEqual, 1)

			Convey("There should be no position until we have a OPEN transaction", func() {
				var position Position
				err := db.Where("symbol = ?", "AAPL").First(&position)
				So(err, ShouldNotBeNil)
			})
		})
	})
}

func TestGMEOptionsTransactions(t *testing.T) {
	Convey("Given a database and a sequence of GME options transactions", t, func() {
		db, err := setupDB()
		So(err, ShouldBeNil)

		account := Account{ID: 1, Name: "Test Account", UserID: 1}
		db.Create(&account)

		transactions := []Transaction{
			{
				Date:        time.Now(),
				Action:      "Sell to Open",
				Symbol:      "GME 02/12/2021 50.00 P",
				Description: "PUT GAMESTOP CORP $50 EXP 02/12/21",
				Price:       10.89,
				Quantity:    5,
				Fees:        3.42,
				Amount:      5441.58,
				AccountID:   account.ID,
			},
			{
				Date:        time.Now(),
				Action:      "Buy to Close",
				Symbol:      "GME 02/12/2021 50.00 P",
				Description: "PUT GAMESTOP CORP $50 EXP 02/12/21",
				Price:       9.50,
				Quantity:    2,
				Fees:        1.33,
				Amount:      -1901.33,
				AccountID:   account.ID,
			},
			{
				Date:        time.Now(),
				Action:      "Buy to Close",
				Symbol:      "GME 02/12/2021 50.00 P",
				Description: "PUT GAMESTOP CORP $50 EXP 02/12/21",
				Price:       9.5,
				Quantity:    3,
				Fees:        1.99,
				Amount:      -2851.99,
				AccountID:   account.ID,
			},
		}

		Convey("When processing the transactions", func() {
			for _, transaction := range transactions {

				id, err := Create(db, &transaction)
				So(err, ShouldBeNil)
				So(id, ShouldEqual, transaction.ID)

				GeneratePositions(db, account.ID)
			}

			Convey("Then the positions should be updated correctly", func() {
				var position Position
				err := db.Where("symbol = ?", "GME 02/12/2021 50.00 P").First(&position).Error
				So(err, ShouldBeNil)

				// The final position should be closed
				So(position.Symbol, ShouldEqual, "GME 02/12/2021 50.00 P")
				So(position.Quantity, ShouldEqual, 0)
				So(position.Opened, ShouldBeFalse)

				// Check GainLoss
				expectedGainLoss := 688.26 // Expected GainLoss from the provided transactions
				So(position.GainLoss, ShouldAlmostEqual, expectedGainLoss, 0.01)
			})
		})
	})
}
