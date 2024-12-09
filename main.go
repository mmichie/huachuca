package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
)

// Server holds all the dependencies for our service
type Server struct {
	db     *DB // We'll implement this later
	logger *slog.Logger
}

// NewServer creates a new instance of our server
func NewServer(db *DB) *Server {
	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	return &Server{
		db:     db,
		logger: logger,
	}
}

// ServeHTTP implements the http.Handler interface
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("received request",
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", r.RemoteAddr,
	)

	switch r.URL.Path {
	case "/health":
		s.handleHealth(w, r)
	default:
		http.NotFound(w, r)
	}
}

// handleHealth handles the health check endpoint
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
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	srv := NewServer(nil)

	addr := ":8080"
	logger.Info("starting server", "addr", addr)

	if err := http.ListenAndServe(addr, srv); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}
