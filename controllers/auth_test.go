package controllers_test

import (
	"net/http"
	"net/http/httptest"
	"stock-portfolio-api/config"
	"stock-portfolio-api/controllers"
	"testing"

	"github.com/golang-jwt/jwt"
)

func TestVerifyJWT(t *testing.T) {
	// create a test handler that just writes "OK" to the response
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	cfg := &config.Config{}
	cfg.JWT.Secret = "secret"

	cont := controllers.InitController(nil, cfg)
	// create a test case for a valid token
	t.Run("Valid token", func(t *testing.T) {
		// create a request with a valid token in the Authorization header
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"id": "123",
		})
		tokenString, _ := token.SignedString([]byte(cfg.JWT.Secret))
		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Add("Authorization", "Bearer "+tokenString)

		// create a response recorder to record the response
		rr := httptest.NewRecorder()

		// call the VerifyJWT function and pass in the request
		cont.VerifyJWT(testHandler).ServeHTTP(rr, req)

		// check that the response has a status OK
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}

	})

	// create a test case for an invalid token
	t.Run("Invalid token", func(t *testing.T) {
		// create a request with an invalid token in the Authorization header
		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Add("Authorization", "Bearer invalid_token")

		// create a response recorder to record the response
		rr := httptest.NewRecorder()

		// call the VerifyJWT function and pass in the request
		cont.VerifyJWT(testHandler).ServeHTTP(rr, req)

		// check that the response has a status of 401 Unauthorized
		if status := rr.Code; status != http.StatusUnauthorized {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusUnauthorized)
		}

		// check that the response body is "Invalid token"
		expected := "token contains an invalid number of segments\n"
		if rr.Body.String() != expected {
			t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
		}
	})

	// create a test case for no token passed in the request
	t.Run("No token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()

		cont.VerifyJWT(testHandler).ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusUnauthorized {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusUnauthorized)
		}

		expected := "Invalid token\n"
		if rr.Body.String() != expected {
			t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
		}
	})

}
