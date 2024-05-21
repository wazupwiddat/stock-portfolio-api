package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"stock-portfolio-api/models"
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
		http.Error(w, "Unable to find acct", http.StatusUnauthorized)
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

	// Determine if the action is a buy or sell and adjust quantity accordingly
	if strings.Contains(strings.ToLower(req.Action), "sell") {
		quantity = -quantity
	}

	// Extract stock symbol (e.g., "DDOG" or "DDOG 04/19/2024 125.00 C")
	parts := strings.Split(req.Symbol, " ")

	// Handle the error condition first
	if len(parts) < 1 {
		http.Error(w, "Invalid Symbol", http.StatusBadRequest)
		return
	}

	// If valid, extract the stock symbol
	stockSymbol := parts[0]

	var position models.Position
	// Find an open position for the given stock symbol
	err = c.db.Where("symbol = ? AND opened = ?", req.Symbol, true).First(&position).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// No open position found, create a new position
			position = models.Position{
				Symbol:           req.Symbol,
				UnderlyingSymbol: stockSymbol,
				Quantity:         0,
				CostBasis:        0,
				Opened:           true,
			}
			if err := c.db.Create(&position).Error; err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Create the new transaction
	transaction := &models.Transaction{
		Date:        req.Date,
		Symbol:      req.Symbol,
		Description: req.Description,
		Position:    position,
		Action:      req.Action,
		Quantity:    quantity,
		Price:       price,
		Fees:        fees,
		Amount:      amount,
		AccountID:   acct.ID,
	}
	if err := c.db.Create(transaction).Error; err != nil {
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

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(transactions)
}
