package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type CreateOrganizationRequest struct {
	Name       string `json:"name"`
	OwnerEmail string `json:"owner_email"`
	OwnerName  string `json:"owner_name"`
}

type AddUserRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (s *Server) handleCreateOrganization(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req CreateOrganizationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := ValidateCreateOrganizationRequest(&req); err != nil {
		var valErr *ValidationError
		if errors.As(err, &valErr) {
			http.Error(w, valErr.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	org, err := s.db.CreateOrganization(r.Context(), req.Name, req.OwnerEmail, req.OwnerName)
	if err != nil {
		switch err {
		case ErrEmailTaken:
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			s.logger.Error("failed to create organization", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(org)
}

func (s *Server) handleAddUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Extract and validate organization ID from URL path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	if err := ValidateUUID(parts[2]); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	orgID, _ := uuid.Parse(parts[2]) // Already validated

	var req AddUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := ValidateAddUserRequest(&req); err != nil {
		var valErr *ValidationError
		if errors.As(err, &valErr) {
			http.Error(w, valErr.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	user, err := s.db.AddUserToOrganization(r.Context(), orgID, req.Email, req.Name)
	if err != nil {
		switch err {
		case ErrEmailTaken:
			http.Error(w, err.Error(), http.StatusConflict)
		case ErrMaxSubAccounts:
			http.Error(w, err.Error(), http.StatusForbidden)
		default:
			s.logger.Error("failed to add user", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (s *Server) handleGetOrganizationUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Extract organization ID from URL path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 3 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	orgID, err := uuid.Parse(parts[2])
	if err != nil {
		http.Error(w, "Invalid organization ID", http.StatusBadRequest)
		return
	}

	users, err := s.db.GetOrganizationUsers(r.Context(), orgID)
	if err != nil {
		s.logger.Error("failed to get organization users", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}
