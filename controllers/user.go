package controllers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"

	"gorm.io/gorm"

	"stock-portfolio-api/models"
)

type LoginReqest struct {
	Email    string
	Password string
}

func (c Controller) HandleSignup(w http.ResponseWriter, r *http.Request) {
	var req LoginReqest
	json.NewDecoder(r.Body).Decode(&req)

	// validate the request
	if req.Email == "" || req.Password == "" {
		http.Error(w, "Email and Password is required", http.StatusBadRequest)
		return
	}

	// Signup
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	newUser := &models.User{
		Email:        req.Email,
		PasswordHash: string(passwordHash),
	}
	id, err := models.CreateUser(c.db, newUser)

	if err != nil || id == 0 {
		http.Error(w, "Failed to create user", http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (c Controller) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginReqest
	json.NewDecoder(r.Body).Decode(&req)

	// validate the request
	if req.Email == "" || req.Password == "" {
		http.Error(w, "Email and Password is required", http.StatusBadRequest)
		return
	}

	// login
	user, err := models.FindUserByEmail(c.db, req.Email)
	if err != nil {
		http.Error(w, "Invalid login", http.StatusNotFound)
		return
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		http.Error(w, "Invalid login", http.StatusNotFound)
		return
	}

	// create a JWT token"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":    user.ID,
		"email": user.Email,
		"exp":   time.Now().Add(time.Hour * 24).Unix(),
	})

	// sign the JWT token with a secret key
	tokenString, err := token.SignedString([]byte(c.cfg.JWT.Secret))
	if err != nil {
		http.Error(w, "Failed to create JWT token", http.StatusInternalServerError)
		return
	}

	// send the JWT token as a response
	json.NewEncoder(w).Encode(map[string]string{
		"token": tokenString,
	})
}

func userFromRequestContext(r *http.Request, db *gorm.DB) (*models.User, error) {
	userID, ok := r.Context().Value("id").(float64)
	if !ok {
		log.Println(r.Context().Value("id"))
		return nil, fmt.Errorf("No user ID on Request context")
	}
	log.Println("UserID: ", userID)
	u, err := models.FindUserByID(db, uint(userID))
	if err != nil {
		return nil, fmt.Errorf("User with ID:%f does not exist", userID)
	}
	return u, nil
}
