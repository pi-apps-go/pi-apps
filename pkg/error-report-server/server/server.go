// This file is part of Pi-Apps Go - a modern, cross-architecture/cross-platform, and modular Pi-Apps implementation in Go.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Module: server.go
// Description: Provides a server for sending error reports to a Discord webhook.

// Notes: This module requires a webhook URL to be provided to the server, which is not provided in the Pi-Apps Go project if you were to self host it.
// To use this module, you will need to provide your own webhook URL as a .env file in the root of the project.
// The .env file should contain the following:
// DISCORD_WEBHOOK_URL=your_webhook_url_here
package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"golang.org/x/time/rate"
)

const (
	// TokenExpiration is how long tokens remain valid
	TokenExpiration = 12 * time.Hour
	// RateLimitRequests is the number of requests allowed per RateLimitPeriod
	RateLimitRequests = 10
	// RateLimitPeriod is the time window for rate limiting
	RateLimitPeriod = 1 * time.Hour
)

// Server represents the error report server
type Server struct {
	router      *mux.Router
	webhookURL  string
	tokens      map[string]time.Time
	tokensMutex sync.RWMutex
	limiter     *rate.Limiter
}

// TokenResponse represents the response when requesting a token
type TokenResponse struct {
	Token string `json:"token"`
}

// NewServer creates a new error report server instance
func NewServer(webhookURL string) *Server {
	// Load the .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	s := &Server{
		router:     mux.NewRouter(),
		webhookURL: os.Getenv("DISCORD_WEBHOOK_URL"),
		tokens:     make(map[string]time.Time),
		limiter:    rate.NewLimiter(rate.Every(RateLimitPeriod/RateLimitRequests), RateLimitRequests),
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures the server routes
func (s *Server) setupRoutes() {
	s.router.HandleFunc("/token", s.handleTokenRequest).Methods("GET")
	s.router.HandleFunc("/report", s.handleErrorReport).Methods("POST")
}

// generateToken creates a new random token
func (s *Server) generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// handleTokenRequest generates a new token for error reporting
func (s *Server) handleTokenRequest(w http.ResponseWriter, r *http.Request) {
	if !s.limiter.Allow() {
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	token, err := s.generateToken()
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	s.tokensMutex.Lock()
	s.tokens[token] = time.Now().Add(TokenExpiration)
	s.tokensMutex.Unlock()

	response := TokenResponse{Token: token}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleErrorReport processes an error report submission
func (s *Server) handleErrorReport(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("X-Error-Report-Token")
	if token == "" {
		http.Error(w, "Missing token", http.StatusUnauthorized)
		return
	}

	s.tokensMutex.RLock()
	expiry, valid := s.tokens[token]
	s.tokensMutex.RUnlock()

	if !valid || time.Now().After(expiry) {
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	// Remove the used token
	s.tokensMutex.Lock()
	delete(s.tokens, token)
	s.tokensMutex.Unlock()

	// Forward the report to Discord webhook
	if err := s.forwardToDiscord(r); err != nil {
		http.Error(w, "Failed to process report", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// forwardToDiscord forwards the error report to Discord
func (s *Server) forwardToDiscord(r *http.Request) error {
	// Create a new request to forward to Discord
	req, err := http.NewRequest("POST", s.webhookURL, r.Body)
	if err != nil {
		return err
	}

	// Copy relevant headers
	req.Header.Set("Content-Type", r.Header.Get("Content-Type"))

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("discord webhook returned status: %d", resp.StatusCode)
	}

	return nil
}

// Start starts the server on the specified address
func (s *Server) Start(addr string) error {
	log.Printf("Starting error report server on %s", addr)
	return http.ListenAndServe(addr, s.router)
}

// CleanupExpiredTokens periodically removes expired tokens
func (s *Server) CleanupExpiredTokens() {
	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		s.tokensMutex.Lock()
		now := time.Now()
		for token, expiry := range s.tokens {
			if now.After(expiry) {
				delete(s.tokens, token)
			}
		}
		s.tokensMutex.Unlock()
	}
}
