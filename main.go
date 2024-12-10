package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"regexp"
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
	cors         *CORSMiddleware
	health       *HealthChecker
	stateStore   *StateStore
}

func (s *Server) logError(err error, msg string) {
	errStr := err.Error()
	errStr = regexp.MustCompile(`(?i)(sql|database|token|password|secret|key)`).
		ReplaceAllString(errStr, "[REDACTED]")

	s.logger.Error(msg,
		"error_type", fmt.Sprintf("%T", err),
		"sanitized_error", errStr,
	)
}

func NewServer(db *DB) (*Server, error) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	tokenManager, err := NewTokenManager()
	if err != nil {
		return nil, err
	}

	// Initialize state store with 15-minute cleanup interval
	stateStore := NewStateStore(15 * time.Minute)

	srv := &Server{
		db:           db,
		logger:       logger,
		tokenManager: tokenManager,
		oauth:        NewOAuthConfig(),
		cors:         NewCORSMiddleware(NewCORSConfig()),
		stateStore:   stateStore,
	}

	srv.auth = NewAuthMiddleware(tokenManager, db)
	srv.health = NewHealthChecker("0.1.0", db, logger)
	return srv, nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	response := s.health.CheckHealth(ctx)

	w.Header().Set("Content-Type", "application/json")
	if response.Status != StatusHealthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	s.logger.Info("health check completed",
		"status", response.Status,
		"checks", len(response.Checks),
		"duration", time.Since(response.CheckTime),
	)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("failed to encode health response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
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
	case "/.well-known/jwks.json":
		s.handleJWKS(w, r)
		return
	case "/auth/login/google":
		s.handleGoogleLogin(w, r)
		return
	case "/auth/refresh":
		s.handleRefreshToken(w, r)
		return
	case "/csrf/token":
		s.handleGetCSRFToken(w, r)
		return
	}

	// Basic request validation first
	if strings.Contains(r.URL.Path, "/organizations/") {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) >= 3 {
			if orgID := parts[2]; orgID != "" && orgID != "users" {
				if _, err := uuid.Parse(orgID); err != nil {
					http.Error(w, "Invalid organization ID format", http.StatusBadRequest)
					return
				}
			}
		}
	}

	// Protected endpoints with authentication and CSRF
	protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/organizations":
			s.auth.RequirePermissions(PermCreateOrg)(
				handlerFuncToHandler(s.CSRFHandler(s.handleCreateOrganization)),
			).ServeHTTP(w, r)
		case strings.HasPrefix(r.URL.Path, "/organizations/") && strings.HasSuffix(r.URL.Path, "/users"):
			s.auth.RequirePermissions(PermInviteUser)(
				s.auth.RequireSameOrg(
					handlerFuncToHandler(s.CSRFHandler(s.handleAddUser)),
				),
			).ServeHTTP(w, r)
		case strings.HasPrefix(r.URL.Path, "/organizations/"):
			s.auth.RequirePermissions(PermReadOrg)(
				s.auth.RequireSameOrg(
					handlerFuncToHandler(s.handleGetOrganizationUsers),
				),
			).ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})

	// Apply authentication middleware after validation
	s.auth.RequireAuth(protectedHandler).ServeHTTP(w, r)
}

func main() {
	// Force production environment so Secure cookies are set
	os.Setenv("ENVIRONMENT", "production")

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

	csrfConfig := NewCSRFConfig()

	// Create HTTP server with timeouts
	httpServer := &http.Server{
		Addr:         ":8080",
		Handler:      NewCSRFMiddleware(csrfConfig)(srv),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		srv.logger.Info("starting server", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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
