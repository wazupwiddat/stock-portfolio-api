package controllers

import (
	"testing"
	"time"

	"stock-portfolio-api/models"

	. "github.com/smartystreets/goconvey/convey"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupDB() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	db.AutoMigrate(&models.Transaction{}, &models.User{}, &models.Position{}, &models.Account{})
	return db, nil
}

func TestStockSplit(t *testing.T) {
	Convey("Given an existing stock position created through a transaction", t, func() {
		db, err := setupDB()
		So(err, ShouldBeNil)

		account := models.Account{ID: 1, Name: "Test Account", UserID: 1}
		db.Create(&account)

		initialTransaction := models.Transaction{
			Date:      time.Now().AddDate(0, -1, 0),
			Action:    "Buy",
			Symbol:    "TSLA",
			Quantity:  200,
			Price:     300,
			Amount:    60000,
			AccountID: account.ID,
		}
		db.Create(&initialTransaction)

		transaction := models.Transaction{
			Date:      time.Now(),
			Action:    "Stock Split",
			Symbol:    "TSLA",
			Quantity:  400, // The split quantity added
			Price:     200, // This should be 0'd out before we create "Stock Split transaction"
			AccountID: account.ID,
		}

		Convey("When a stock split transaction is processed", func() {
			id, err := models.Create(db, &transaction)
			So(err, ShouldBeNil)
			So(id, ShouldEqual, transaction.ID)

			var createdTransaction models.Transaction
			db.Where("action = ? AND symbol = ?", "Stock Split", "TSLA").First(&createdTransaction)

			Convey("A new stock split transaction should be created", func() {
				So(createdTransaction.Quantity, ShouldEqual, 400)
				So(createdTransaction.Price, ShouldEqual, 0)
			})

			Convey("The position quantity and cost basis should be updated correctly", func() {
				var updatedPosition models.Position
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

		account := models.Account{ID: 1, Name: "Test Account", UserID: 1}
		db.Create(&account)

		transaction := models.Transaction{
			Date:      time.Now(),
			Action:    "Sell to Open",
			Symbol:    "TSLA 01/20/2023 1000.00 C",
			Price:     283.93,
			Quantity:  1,
			AccountID: account.ID,
			Amount:    28392.21,
		}
		db.Create(&transaction)

		oldPosition := transaction.Position

		forwardSplitTrans := models.Transaction{
			Date:      time.Now(),
			Action:    "Options Frwd Split",
			Symbol:    "TSLA 01/20/2023 333.33 C",
			Quantity:  -2,
			AccountID: account.ID,
		}

		Convey("When an options forward split transaction is processed", func() {
			id, err := models.Create(db, &forwardSplitTrans)
			So(err, ShouldBeNil)
			So(id, ShouldEqual, forwardSplitTrans.ID)

			var createdTransaction models.Transaction
			db.Where("action = ? AND symbol = ?", "Options Frwd Split", "TSLA 01/20/2023 333.33 C").First(&createdTransaction)

			Convey("A new options forward split transaction should be created", func() {
				So(createdTransaction.Quantity, ShouldEqual, -2)
				So(createdTransaction.Symbol, ShouldEqual, "TSLA 01/20/2023 333.33 C")
				So(createdTransaction.PositionID, ShouldNotEqual, oldPosition.ID)
			})

			Convey("The position should be updated correctly by AfterSave hook", func() {
				var newPosition models.Position
				db.Where("symbol = ?", "TSLA 01/20/2023 333.33 C").First(&newPosition)

				So(newPosition.Symbol, ShouldEqual, "TSLA 01/20/2023 333.33 C")
				So(newPosition.Quantity, ShouldEqual, -3)                 // The split and subsequent buy/sell should result in -3 quantity
				So(newPosition.CostBasis, ShouldAlmostEqual, 94.64, 0.01) // The cost basis should be adjusted accordingly
			})

			Convey("The old position should be deleted", func() {
				var deletedPosition models.Position
				err := db.Where("id = ?", oldPosition.ID).First(&deletedPosition).Error
				So(err, ShouldEqual, gorm.ErrRecordNotFound)
			})

			Convey("The moved transactions should have the new symbol", func() {
				var movedTransaction models.Transaction
				db.Where("id = ?", transaction.ID).First(&movedTransaction)
				So(movedTransaction.Symbol, ShouldEqual, "TSLA 01/20/2023 333.33 C")
				So(movedTransaction.PositionID, ShouldEqual, createdTransaction.PositionID)
			})
		})
	})
}
