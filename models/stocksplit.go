package models

import "gorm.io/gorm"

// This information is only used to adjust Option Symbols since there is a "Stock Split" transaction for the symbol
//	which updates the cost basis.  We don't adjust past transaction at this point

type StockSplit struct {
	gorm.Model
	Symbol     string  `gorm:"index"`
	SplitDate  string  `gorm:"type:date"`
	SplitRatio float64 `gorm:"not null"`
}

func InitializeStockSplits(db *gorm.DB) error {
	splits := []StockSplit{
		{Symbol: "AAPL", SplitDate: "2014-06-09", SplitRatio: 7},
		{Symbol: "AAPL", SplitDate: "2020-08-31", SplitRatio: 4},
		{Symbol: "AMZN", SplitDate: "1998-06-06", SplitRatio: 2},
		{Symbol: "AMZN", SplitDate: "1999-01-05", SplitRatio: 3},
		{Symbol: "AMZN", SplitDate: "1999-09-02", SplitRatio: 2},
		{Symbol: "AMZN", SplitDate: "2022-06-06", SplitRatio: 20},
		{Symbol: "TSLA", SplitDate: "2020-08-31", SplitRatio: 5},
		{Symbol: "TSLA", SplitDate: "2022-08-25", SplitRatio: 3},
	}

	for _, split := range splits {
		var count int64
		db.Model(&StockSplit{}).Where("symbol = ? AND split_date = ?", split.Symbol, split.SplitDate).Count(&count)
		if count == 0 {
			if err := db.Create(&split).Error; err != nil {
				return err
			}
		}
	}

	return nil
}
