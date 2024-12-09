package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type Server struct {
	db           *DB
	logger       *slog.Logger
	tokenManager *TokenManager
	auth         *AuthMiddleware
	oauth        *OAuthConfig
}

type HealthResponse struct {
	Status string `json:"status"`
	DB     bool   `json:"database"`
}

func NewServer(db *DB) (*Server, error) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	tokenManager, err := NewTokenManager()
	if err != nil {
		return nil, err
	}

	srv := &Server{
		db:           db,
		logger:       logger,
		tokenManager: tokenManager,
		oauth:        NewOAuthConfig(),
	}

	srv.auth = NewAuthMiddleware(tokenManager, db)
	return srv, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("received request",
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", r.RemoteAddr,
	)

	// Set security headers
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-XSS-Protection", "1; mode=block")

	// Public endpoints
	switch r.URL.Path {
	case "/health":
		s.handleHealth(w, r)
		return
	case "/auth/login/google":
		s.handleGoogleLogin(w, r)
		return
	case "/auth/callback/google":
		s.handleGoogleCallback(w, r)
		return
	}

	// Protected endpoints with authentication
	baseHandler := s.auth.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/organizations":
			// Creating an org requires the create:org permission
			s.auth.RequirePermissions(PermCreateOrg)(
				http.HandlerFunc(s.handleCreateOrganization),
			).ServeHTTP(w, r)

		case strings.HasPrefix(r.URL.Path, "/organizations/") && strings.HasSuffix(r.URL.Path, "/users"):
			// Adding users requires the invite:user permission and same org
			s.auth.RequirePermissions(PermInviteUser)(
				s.auth.RequireSameOrg(
					http.HandlerFunc(s.handleAddUser),
				),
			).ServeHTTP(w, r)

		case strings.HasPrefix(r.URL.Path, "/organizations/"):
			// Reading org users requires the read:org permission and same org
			s.auth.RequirePermissions(PermReadOrg)(
				s.auth.RequireSameOrg(
					http.HandlerFunc(s.handleGetOrganizationUsers),
				),
			).ServeHTTP(w, r)

		default:
			http.NotFound(w, r)
		}
	}))

	baseHandler.ServeHTTP(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	resp := HealthResponse{
		Status: "ok",
		DB:     s.db != nil && s.db.Ping() == nil,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		s.logger.Error("failed to encode response", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func main() {
	// Load configuration from environment
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://huachuca_user:huachuca_password@localhost:5432/huachuca?sslmode=disable"
	}

	// Connect to database
	db, err := NewDB(dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Create server
	srv, err := NewServer(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create server: %v\n", err)
		os.Exit(1)
	}

	// Create HTTP server with timeouts
	httpServer := &http.Server{
		Addr:         ":8080",
		Handler:      srv,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		srv.logger.Info("starting server", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			srv.logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	srv.logger.Info("shutting down server", "signal", sig)

	// Create context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := httpServer.Shutdown(ctx); err != nil {
		srv.logger.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	srv.logger.Info("server stopped gracefully")
}
