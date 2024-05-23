package controllers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"stock-portfolio-api/models"

	"gorm.io/gorm"
)

const MAX_UPLOAD_SIZE = 1024 * 1024 // 1MB

// HandleImport handles the import of transactions from a JSON file
func (c *Controller) HandleImport(w http.ResponseWriter, r *http.Request) {
	log.Println("Handling import")

	u, err := userFromRequestContext(r, c.db)
	if err != nil {
		log.Println(err)
		http.Error(w, "Unable to find user", http.StatusUnauthorized)
		return
	}

	// Parse form data
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get account_id from form data
	accountIDStr := r.FormValue("account_id")
	if accountIDStr == "" {
		http.Error(w, "account_id is required", http.StatusBadRequest)
		return
	}
	accountID, err := strconv.Atoi(accountIDStr)
	if err != nil {
		http.Error(w, "Invalid account_id", http.StatusBadRequest)
		return
	}

	// Validate account ownership
	acct, err := models.FindAccountByID(c.db, uint(accountID))
	if err != nil || acct.UserID != u.ID {
		http.Error(w, "Unauthorized or account not found", http.StatusUnauthorized)
		return
	}

	// Get a reference to the fileHeaders
	files := r.MultipartForm.File["file"]
	uploadedFiles := []string{}
	for _, fileHeader := range files {
		if fileHeader.Size > MAX_UPLOAD_SIZE {
			http.Error(w, fmt.Sprintf("The uploaded file is too big: %s. Please use a file less than 1MB in size", fileHeader.Filename), http.StatusBadRequest)
			return
		}

		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		err = os.MkdirAll("./uploads", os.ModePerm)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		f, err := os.Create(fmt.Sprintf("./uploads/%s", fileHeader.Filename))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer f.Close()

		_, err = io.Copy(f, file)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		uploadedFiles = append(uploadedFiles, fileHeader.Filename)
	}

	// Kick off the import into MySQL
	go importUploadedJSONFiles(c.db, uint(accountID))

	// Send the uploaded files as a response
	json.NewEncoder(w).Encode(map[string]interface{}{
		"files": uploadedFiles,
	})
}

// importUploadedJSONFiles reads uploaded JSON files and imports transactions
func importUploadedJSONFiles(db *gorm.DB, accountID uint) {
	files, err := os.ReadDir("./uploads")
	if err != nil {
		log.Println("Error reading uploads directory:", err)
		return
	}

	for _, file := range files {
		if file.Type().IsRegular() && filepath.Ext(file.Name()) == ".json" {
			importFile(filepath.Join("./uploads", file.Name()), db, accountID)
		}
	}
}

func importFile(filePath string, db *gorm.DB, accountID uint) {
	file, err := os.Open(filePath)
	if err != nil {
		log.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	// Define a structure that matches the JSON file
	type BrokerageTransaction struct {
		Date        string `json:"Date"`
		Action      string `json:"Action"`
		Symbol      string `json:"Symbol"`
		Description string `json:"Description"`
		Quantity    string `json:"Quantity"`
		Price       string `json:"Price"`
		FeesComm    string `json:"Fees & Comm"`
		Amount      string `json:"Amount"`
	}

	type TransactionsFile struct {
		BrokerageTransactions []BrokerageTransaction `json:"BrokerageTransactions"`
	}

	var transactionsFile TransactionsFile
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&transactionsFile); err != nil {
		log.Println("Error decoding JSON file:", err)
		return
	}

	// Define the allowed actions
	allowedActions := map[string]bool{
		"Buy to Open":   true,
		"Buy to Close":  true,
		"Sell to Open":  true,
		"Sell to Close": true,
		"Buy":           true,
		"Sell":          true,
		"Assigned":      true,
		"Expired":       true,
		"Sell Short":    true,
	}

	var transactions []models.Transaction
	for _, bt := range transactionsFile.BrokerageTransactions {
		if !allowedActions[bt.Action] {
			continue
		}

		quantity, _ := parseMonetaryValue(bt.Quantity)
		price, _ := parseMonetaryValue(bt.Price)
		fees, _ := parseMonetaryValue(bt.FeesComm)
		amount, _ := parseMonetaryValue(bt.Amount)

		// Create a new transaction
		transaction := models.Transaction{
			Date:        bt.Date,
			Action:      bt.Action,
			Symbol:      bt.Symbol,
			Description: bt.Description,
			Quantity:    quantity,
			Price:       price,
			Fees:        fees,
			Amount:      amount,
			AccountID:   accountID,
		}
		transactions = append(transactions, transaction)
	}

	if err := models.CreateMany(db, transactions); err != nil {
		log.Println("Error inserting transactions into the database:", err)
	}
}

// Helper function to parse monetary values by removing $ and ,
func parseMonetaryValue(value string) (float64, error) {
	cleanedValue := strings.ReplaceAll(strings.ReplaceAll(value, "$", ""), ",", "")
	return strconv.ParseFloat(cleanedValue, 64)
}
