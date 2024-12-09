package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
)

type Server struct {
	db     *DB
	logger *slog.Logger
}

func NewServer(db *DB) *Server {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	return &Server{
		db:     db,
		logger: logger,
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("received request",
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", r.RemoteAddr,
	)

	switch {
	case r.URL.Path == "/health":
		s.handleHealth(w, r)
	case r.URL.Path == "/organizations":
		s.handleCreateOrganization(w, r)
	case strings.HasPrefix(r.URL.Path, "/organizations/users"):
		s.handleAddUser(w, r)
	case strings.HasPrefix(r.URL.Path, "/organizations/"):
		s.handleGetOrganizationUsers(w, r)
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
