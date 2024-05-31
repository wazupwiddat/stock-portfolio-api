package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"stock-portfolio-api/models"

	"github.com/gorilla/mux"
)

// CreateAccountRequest is used to create a new account
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
func (c *Controller) HandleGetAccount(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	accountID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid account ID", http.StatusBadRequest)
		return
	}

	u, err := userFromRequestContext(r, c.db)
	if err != nil {
		http.Error(w, "Unable to find user", http.StatusUnauthorized)
		return
	}

	account, err := models.FindAccountByID(c.db, uint(accountID))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if account.UserID != u.ID {
		http.Error(w, "Unable to find user", http.StatusUnauthorized)
		return
	}

	w.WriteHeader(http.StatusOK)
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

// HandleDeleteAccount handles deleting an account and its related transactions and positions
func (c *Controller) HandleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	accountID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid account ID", http.StatusBadRequest)
		return
	}

	u, err := userFromRequestContext(r, c.db)
	if err != nil {
		http.Error(w, "Unable to find user", http.StatusUnauthorized)
		return
	}

	account, err := models.FindAccountByID(c.db, uint(accountID))
	if err != nil {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}

	if account.UserID != u.ID {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Delete all transactions for the account
	if err := c.db.Where("account_id = ?", account.ID).Delete(&models.Transaction{}).Error; err != nil {
		http.Error(w, "Failed to delete transactions", http.StatusInternalServerError)
		return
	}

	// Delete all positions for the account
	if err := c.db.Where("account_id = ?", account.ID).Delete(&models.Position{}).Error; err != nil {
		http.Error(w, "Failed to delete positions", http.StatusInternalServerError)
		return
	}

	// Delete the account
	if err := c.db.Delete(account).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
