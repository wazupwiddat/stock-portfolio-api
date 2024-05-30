package controllers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

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

	// Get the last imported transaction date for the account
	lastTransactionDate, err := models.GetLastTransactionDate(db, accountID)
	if err != nil {
		log.Println("Error retrieving last transaction date:", err)
		return
	}

	for _, file := range files {
		if file.Type().IsRegular() && filepath.Ext(file.Name()) == ".json" {
			filename := filepath.Join("./uploads", file.Name())
			importFile(filename, db, accountID, lastTransactionDate)
			if e := os.Remove(filename); e != nil {
				log.Println(err)
			}
		}
	}
}

func importFile(filePath string, db *gorm.DB, accountID uint, lastTransactionDate time.Time) {
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
		"Buy to Open":          true,
		"Buy to Close":         true,
		"Sell to Open":         true,
		"Sell to Close":        true,
		"Buy":                  true,
		"Sell":                 true,
		"Sell Short":           true,
		"Assigned":             true,
		"Expired":              true,
		"Options Frwd Split":   true,
		"Stock Split":          true,
		"Exchange or Exercise": true,
	}

	var transactions []models.Transaction
	for _, bt := range transactionsFile.BrokerageTransactions {
		if !allowedActions[bt.Action] {
			continue
		}

		// Extract the correct date from the Date field
		transactionDateStr := extractCorrectDate(bt.Date)
		transactionDate, err := time.Parse("01/02/2006", transactionDateStr)
		if err != nil {
			log.Println("Error parsing transaction date:", err)
			continue
		}

		// Convert to local time zone at the beginning of the day
		location, _ := time.LoadLocation("Local")
		transactionDate = time.Date(transactionDate.Year(), transactionDate.Month(), transactionDate.Day(), 0, 0, 0, 0, location)

		// Skip transactions that are older than or equal to the last transaction date
		if transactionDate.Before(lastTransactionDate) || transactionDate.Equal(lastTransactionDate) {
			continue
		}

		quantity, _ := parseMonetaryValue(bt.Quantity)
		price, _ := parseMonetaryValue(bt.Price)
		fees, _ := parseMonetaryValue(bt.FeesComm)
		amount, _ := parseMonetaryValue(bt.Amount)

		// Create a new transaction
		transaction := models.Transaction{
			Date:        transactionDate,
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

	if len(transactions) == 0 {
		log.Println("Nothing to import")
		return
	}

	// Sort transactions by date (oldest to newest)
	sort.Slice(transactions, func(i, j int) bool {
		return transactions[i].Date.Before(transactions[j].Date)
	})

	// Insert transactions into the database
	for _, transaction := range transactions {
		if _, err := models.Create(db, &transaction); err != nil {
			log.Println("Error inserting transaction into the database:", err)
		}
	}
}

// Helper function to extract the correct date from the date string
func extractCorrectDate(dateStr string) string {
	// Split the date string by " as of "
	parts := strings.Split(dateStr, " as of ")
	// Return the last part
	return parts[len(parts)-1]
}

// Helper function to parse monetary values by removing $ and ,
func parseMonetaryValue(value string) (float64, error) {
	cleanedValue := strings.ReplaceAll(strings.ReplaceAll(value, "$", ""), ",", "")
	return strconv.ParseFloat(cleanedValue, 64)
}
