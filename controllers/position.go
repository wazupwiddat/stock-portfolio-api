package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"gorm.io/gorm"

	"stock-portfolio-api/models"
)

// HandleGetPositions handles fetching positions for a specific account ID
func (c *Controller) HandleGetPositions(w http.ResponseWriter, r *http.Request) {
	accountIDStr := r.URL.Query().Get("account_id")
	if accountIDStr == "" {
		http.Error(w, "account_id is required", http.StatusBadRequest)
		return
	}
	accountID, err := strconv.Atoi(accountIDStr)
	if err != nil {
		http.Error(w, "Invalid account_id", http.StatusBadRequest)
		return
	}

	positions, err := models.FetchPositionsByAccount(c.db, uint(accountID))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "Account not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(positions)
}
