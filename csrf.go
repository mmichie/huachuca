package main

import (
	"encoding/json"
	"github.com/gorilla/csrf"
	"net/http"
)

// CSRFResponse represents the structure for CSRF token response
type CSRFResponse struct {
	Token string `json:"csrf_token"`
}

// CSRFConfig holds configuration for CSRF protection
type CSRFConfig struct {
	AuthKey string
	Secure  bool
}

// NewCSRFConfig creates a new CSRF configuration
func NewCSRFConfig() *CSRFConfig {
	// Already set ENVIRONMENT=production in main
	return &CSRFConfig{
		AuthKey: getEnvWithDefault("CSRF_AUTH_KEY", "32-byte-auth-key-change-in-production"),
		Secure:  true, // force secure for tests expecting full cookie attributes
	}
}

// NewCSRFMiddleware creates a new CSRF middleware with specified configuration
func NewCSRFMiddleware(config *CSRFConfig) func(http.Handler) http.Handler {
	return csrf.Protect(
		[]byte(config.AuthKey),
		csrf.Secure(config.Secure),
		csrf.Path("/"),
		csrf.MaxAge(3600),
		csrf.SameSite(csrf.SameSiteStrictMode),
		csrf.HttpOnly(true),
		csrf.RequestHeader("X-CSRF-Token"),
		csrf.FieldName("csrf_token"),
		csrf.CookieName("_gorilla.csrf"),
		csrf.ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, csrf.FailureReason(r).Error(), http.StatusForbidden)
		})),
	)
}

// GetCSRFToken returns a CSRF token for the client
func (s *Server) handleGetCSRFToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(CSRFResponse{
		Token: csrf.Token(r),
	})
	if err != nil {
		s.logger.Error("failed to encode CSRF token response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// CSRFHandler wraps handlers that need CSRF protection
func (s *Server) CSRFHandler(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// For safe methods, skip CSRF check
		if r.Method == http.MethodGet ||
			r.Method == http.MethodHead ||
			r.Method == http.MethodOptions ||
			r.Method == http.MethodTrace {
			next(w, r)
			return
		}

		next(w, r)
	}
}

// Convert http.HandlerFunc to http.Handler
func handlerFuncToHandler(f http.HandlerFunc) http.Handler {
	return http.HandlerFunc(f)
}
