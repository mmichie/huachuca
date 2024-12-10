package main

import (
	"net/http"
	"strconv"
	"strings"
)

type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	MaxAge         int // in seconds
}

func NewCORSConfig() *CORSConfig {
	allowedOrigins := strings.Split(getEnvWithDefault("ALLOWED_ORIGINS", "http://localhost:3000"), ",")

	return &CORSConfig{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{
			"Authorization",
			"Content-Type",
			"X-Requested-With",
			"Accept",
			"Origin",
		},
		MaxAge: 86400, // 24 hours
	}
}

type CORSMiddleware struct {
	config *CORSConfig
}

func NewCORSMiddleware(config *CORSConfig) *CORSMiddleware {
	return &CORSMiddleware{config: config}
}

func (m *CORSMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Check if the origin is allowed
		allowed := false
		for _, allowedOrigin := range m.config.AllowedOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				allowed = true
				break
			}
		}

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", strings.Join(m.config.AllowedMethods, ","))
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(m.config.AllowedHeaders, ","))
			w.Header().Set("Access-Control-Max-Age", strconv.Itoa(m.config.MaxAge))  // Fixed: proper integer to string conversion
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
