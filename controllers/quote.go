package controllers

import (
	"encoding/json"
	"log"
	"net/http"

	"stock-portfolio-api/models"
)

func (c *Controller) HandleGetCurrentPrice(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		http.Error(w, "symbol parameter is required", http.StatusBadRequest)
		return
	}

	price, err := models.GetCurrentPrice(symbol)
	if err != nil {
		http.Error(w, "failed to fetch current price", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]float64{"currentPrice": price})
}

func (c *Controller) HandleHistoricalPrices(w http.ResponseWriter, r *http.Request) {

	// this should only getting historical prices for stocks, no options
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		http.Error(w, "symbol parameter is required", http.StatusBadRequest)
		return
	}

	// Fetch the first transaction of the position
	var transaction models.Transaction
	err := c.db.Where("symbol = ?", symbol).Order("date ASC").First(&transaction).Error
	if err != nil {
		http.Error(w, "failed to fetch transaction", http.StatusNoContent)
		return
	}

	startDate := transaction.Date

	prices, err := models.GetHistoricalPrices(symbol, startDate)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]models.HistoricalPrice{"prices": prices})
}
