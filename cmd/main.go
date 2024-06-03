package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"stock-portfolio-api/config"
	"stock-portfolio-api/controllers"
	"stock-portfolio-api/models"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	// Config
	cfg, err := config.NewConfig("./config.yml")
	if err != nil {
		log.Fatal(err)
		return
	}
	// create schema if does not exist
	// Connect to MySQL server without specifying a database
	rootDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/", cfg.MySQL.User, cfg.MySQL.Password, cfg.MySQL.Server, cfg.MySQL.Port)
	rootDB, err := sql.Open("mysql", rootDSN)
	if err != nil {
		log.Fatalf("failed to connect to MySQL server: %v", err)
	}
	defer rootDB.Close()

	// Create the database if it doesn't exist
	_, err = rootDB.Exec("CREATE DATABASE IF NOT EXISTS " + cfg.MySQL.Schema)
	if err != nil {
		log.Fatalf("failed to create database: %v", err)
	}
	// connect to the database
	dsn := cfg.MySQLDNS()
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatal(err)
	}

	db.AutoMigrate(&models.Account{}, &models.User{}, &models.Transaction{}, &models.Position{})

	router := mux.NewRouter()
	controller := controllers.InitController(db, cfg)
	c := cors.AllowAll()

	// Health check endpoint
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	router.HandleFunc("/signup", controller.HandleSignup).Methods("POST")
	router.HandleFunc("/login", controller.HandleLogin).Methods("POST")

	protected := router.PathPrefix("/protected").Subrouter()
	protected.HandleFunc("/accounts", controller.HandleCreateAccount).Methods("POST")
	protected.HandleFunc("/accounts", controller.HandleGetAccounts).Methods("GET")
	protected.HandleFunc("/accounts/{id}", controller.HandleGetAccount).Methods("GET")
	protected.HandleFunc("/accounts/{id}", controller.HandleDeleteAccount).Methods("DELETE") // Added delete account endpoint
	protected.HandleFunc("/transactions", controller.HandleCreateTransaction).Methods("POST")
	protected.HandleFunc("/transactions", controller.HandleGetTransactions).Methods("GET")
	protected.HandleFunc("/transactions/{id}", controller.HandleDeleteTransaction).Methods("DELETE")
	protected.HandleFunc("/transactions/import", controller.HandleImport).Methods("POST") // Add this line for the import endpoint
	protected.HandleFunc("/positions", controller.HandleGetPositions).Methods("GET")
	protected.HandleFunc("/quote", controller.HandleGetCurrentPrice).Methods("GET")
	protected.HandleFunc("/quotes", controller.HandleHistoricalPrices).Methods("GET")

	protected.Use(controller.VerifyJWT)

	http.ListenAndServe(fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port), c.Handler(router))
}
