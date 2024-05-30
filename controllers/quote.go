package controllers

import (
	"encoding/json"
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
