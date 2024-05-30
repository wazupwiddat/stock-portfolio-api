package models

import (
	"github.com/piquette/finance-go/quote"
)

func GetCurrentPrice(symbol string) (float64, error) {
	q, err := quote.Get(symbol)
	if err != nil {
		return 0, err
	}

	return q.Bid, nil
}
