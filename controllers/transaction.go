package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"stock-portfolio-api/models"

	"github.com/gorilla/mux"
)

type CreateTransactionRequest struct {
	Date        string `json:"Date" binding:"required"`
	Action      string `json:"Action" binding:"required"`
	Symbol      string `json:"Symbol" binding:"required"`
	Description string `json:"Description" binding:"required"`
	Quantity    string `json:"Quantity" binding:"required"`
	Price       string `json:"Price" binding:"required"`
	FeesComm    string `json:"FeesComm" binding:"required"`
	Amount      string `json:"Amount" binding:"required"`
	AccountID   uint   `json:"AccountID" binding:"required"`
}

// HandleCreateTransaction handles the creation of a new transaction
func (c *Controller) HandleCreateTransaction(w http.ResponseWriter, r *http.Request) {
	var req CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Helper function to parse monetary values by removing $ and ,
	parseMonetaryValue := func(value string) (float64, error) {
		cleanedValue := strings.ReplaceAll(strings.ReplaceAll(value, "$", ""), ",", "")
		return strconv.ParseFloat(cleanedValue, 64)
	}

	u, err := userFromRequestContext(r, c.db)
	if err != nil {
		http.Error(w, "Unable to find user", http.StatusUnauthorized)
		return
	}

	acct, err := models.FindAccountByID(c.db, req.AccountID)
	if err != nil || acct.UserID != u.ID {
		http.Error(w, "Unauthorized or account not found", http.StatusUnauthorized)
		return
	}

	// Parse Date
	transactionDate, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		http.Error(w, "Invalid Date", http.StatusBadRequest)
		return
	}

	// Parse Quantity
	quantity, err := parseMonetaryValue(req.Quantity)
	if err != nil {
		http.Error(w, "Invalid Quantity", http.StatusBadRequest)
		return
	}

	// Parse Price
	price, err := parseMonetaryValue(req.Price)
	if err != nil {
		http.Error(w, "Invalid Price", http.StatusBadRequest)
		return
	}

	// Parse Fees & Comm (optional)
	fees, err := parseMonetaryValue(req.FeesComm)
	if err != nil {
		http.Error(w, "Invalid Fees & Comm", http.StatusBadRequest)
		return
	}

	// Parse Amount (optional)
	amount, err := parseMonetaryValue(req.Amount)
	if err != nil {
		http.Error(w, "Invalid Amount", http.StatusBadRequest)
		return
	}

	// Create the new transaction
	transaction := &models.Transaction{
		Date:        transactionDate,
		Symbol:      req.Symbol,
		Description: req.Description,
		Action:      req.Action,
		Quantity:    quantity,
		Price:       price,
		Fees:        fees,
		Amount:      amount,
		AccountID:   acct.ID,
	}

	if _, err := models.Create(c.db, transaction); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Recalculate position attributes after deleting the transaction
	if err := models.GeneratePositions(c.db, transaction.AccountID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(transaction)
}

// HandleGetTransactions handles fetching transactions for a specific account ID
func (c *Controller) HandleGetTransactions(w http.ResponseWriter, r *http.Request) {
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

	symbol := r.URL.Query().Get("symbol")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 10
	}

	transactions, err := models.FetchTransactionsByAccountIDAndSymbol(c.db, uint(accountID), symbol, page, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	total, err := models.CountTransactionsByAccountIDAndSymbol(c.db, uint(accountID), symbol)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"transactions": transactions,
		"total":        total,
	})
}

// HandleDeleteTransaction handles the deletion of a transaction by ID
func (c *Controller) HandleDeleteTransaction(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid transaction ID", http.StatusBadRequest)
		return
	}

	u, err := userFromRequestContext(r, c.db)
	if err != nil {
		http.Error(w, "Unable to find user", http.StatusUnauthorized)
		return
	}

	transaction, err := models.FindTransactionByID(c.db, uint(id))
	if err != nil || transaction == nil || transaction.Account.UserID != u.ID {
		http.Error(w, "Transaction not found or unauthorized", http.StatusNotFound)
		return
	}

	if err := models.DeleteTransaction(c.db, uint(id)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Recalculate position attributes after deleting the transaction
	if err := models.GeneratePositions(c.db, transaction.AccountID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
