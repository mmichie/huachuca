package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func (s *Server) handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	state, err := generateState()
	if err != nil {
		s.logger.Error("failed to generate state", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// TODO: Store state in session/redis for validation

	authURL := s.oauth.GetAuthURL(state)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func (s *Server) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	state := r.URL.Query().Get("state")
	if state == "" {
		http.Error(w, "Missing state parameter", http.StatusBadRequest)
		return
	}

	// TODO: Validate state from session/redis

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Missing code parameter", http.StatusBadRequest)
		return
	}

	token, err := s.oauth.Exchange(r.Context(), code)
	if err != nil {
		s.logger.Error("failed to exchange token", "error", err)
		http.Error(w, "Failed to exchange token", http.StatusInternalServerError)
		return
	}

	googleUser, err := s.oauth.GetUserInfo(r.Context(), token)
	if err != nil {
		s.logger.Error("failed to get user info", "error", err)
		http.Error(w, "Failed to get user info", http.StatusInternalServerError)
		return
	}

	// Look up user by email
	var user *User
	user, err = s.db.GetUserByEmail(r.Context(), googleUser.Email)
	if err != nil {
		// Create new user if not found
		user = &User{
			ID:          uuid.New(),
			Email:       googleUser.Email,
			Name:        googleUser.Name,
			Role:        "owner", // First user becomes owner
			Permissions: Permissions{"admin": true},
		}

		// Create organization for new user
		org := &Organization{
			ID:              uuid.New(),
			Name:           fmt.Sprintf("%s's Organization", googleUser.Name),
			OwnerID:        user.ID,
			SubscriptionTier: "free",
			MaxSubAccounts:  5,
		}

		user.OrganizationID = org.ID

		if err := s.db.CreateOrganizationWithOwner(r.Context(), org, user); err != nil {
			s.logger.Error("failed to create organization and user", "error", err)
			http.Error(w, "Failed to create account", http.StatusInternalServerError)
			return
		}
	}

	// Generate JWT token
	jwtToken, err := s.tokenManager.GenerateToken(user)
	if err != nil {
		s.logger.Error("failed to generate token", "error", err)
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Return JWT token
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token": jwtToken,
	})
}
