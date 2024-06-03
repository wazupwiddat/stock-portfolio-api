package models

import (
	"time"

	"github.com/piquette/finance-go/chart"
	"github.com/piquette/finance-go/datetime"
	"github.com/piquette/finance-go/quote"
)

type HistoricalPrice struct {
	Date  time.Time
	Close float64
}

func GetCurrentPrice(symbol string) (float64, error) {
	q, err := quote.Get(symbol)
	if err != nil {
		return 0, err
	}

	return q.Bid, nil
}

func GetHistoricalPrices(symbol string, startDate time.Time) ([]HistoricalPrice, error) {
	t := time.Now()
	params := &chart.Params{
		Symbol:   symbol,
		Interval: datetime.OneMonth,
		Start:    datetime.New(&startDate),
		End:      datetime.New(&t),
	}
	iter := chart.Get(params)

	prices := []HistoricalPrice{}
	for iter.Next() {
		bar := iter.Bar()
		close, _ := bar.AdjClose.Float64() // Convert decimal.Decimal to float64
		price := HistoricalPrice{
			Date:  time.Unix(int64(bar.Timestamp), 0), // Convert timestamp to time.Time
			Close: close,
		}
		prices = append(prices, price)
	}
	if err := iter.Err(); err != nil {
		return prices, err
	}
	return prices, nil
}
