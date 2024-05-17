package main

import (
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
)

func main() {
	// Config
	cfg, err := config.NewConfig("./config.yml")
	if err != nil {
		log.Fatal(err)
		return
	}
	// connect to the database
	dsn := cfg.MySQLDNS()
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
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
	protected.HandleFunc("/transactions", controller.HandleCreateTransaction).Methods("POST")
	protected.HandleFunc("/transactions", controller.HandleGetTransactions).Methods("GET")
	protected.HandleFunc("/positions", controller.HandleGetPositions).Methods("GET")

	protected.Use(controller.VerifyJWT)

	http.ListenAndServe(fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port), c.Handler(router))

}
