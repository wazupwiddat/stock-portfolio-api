package controllers

import (
	"encoding/json"
	"net/http"

	"stock-portfolio-api/models"
)

type CreateAccountRequest struct {
	Name string `json:"name" binding:"required"`
}

// HandleCreateAccount handles the creation of a new account
func (c *Controller) HandleCreateAccount(w http.ResponseWriter, r *http.Request) {
	var req CreateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	u, err := userFromRequestContext(r, c.db)
	if err != nil {
		http.Error(w, "Unable to find user", http.StatusUnauthorized)
		return
	}

	account := &models.Account{UserID: u.ID, Name: req.Name}
	if err := c.db.Create(account).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(account)
}

// HandleGetAccounts handles fetching accounts for a specific user
func (c *Controller) HandleGetAccounts(w http.ResponseWriter, r *http.Request) {
	u, err := userFromRequestContext(r, c.db)
	if err != nil {
		http.Error(w, "Unable to find user", http.StatusUnauthorized)
		return
	}

	accounts, err := models.FetchAccountsByUserID(c.db, u.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(accounts)
}
