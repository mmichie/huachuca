package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"` // seconds until access token expires
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

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
			ID:    uuid.New(),
			Email: googleUser.Email,
			Name:  googleUser.Name,
			Role:  "owner", // First user becomes owner
			Permissions: Permissions{
				string(PermCreateOrg):      true,
				string(PermReadOrg):        true,
				string(PermUpdateOrg):      true,
				string(PermDeleteOrg):      true,
				string(PermInviteUser):     true,
				string(PermRemoveUser):     true,
				string(PermUpdateUser):     true,
				string(PermManageSettings): true,
				"admin":                    true,
			},
		}

		// Create organization for new user
		org := &Organization{
			ID:               uuid.New(),
			Name:             fmt.Sprintf("%s's Organization", googleUser.Name),
			OwnerID:          user.ID,
			SubscriptionTier: "free",
			MaxSubAccounts:   5,
		}

		user.OrganizationID = org.ID

		if err := s.db.CreateOrganizationWithOwner(r.Context(), org, user); err != nil {
			s.logger.Error("failed to create organization and user", "error", err)
			http.Error(w, "Failed to create account", http.StatusInternalServerError)
			return
		}
	}

	// Generate JWT access token
	accessToken, err := s.tokenManager.GenerateToken(user)
	if err != nil {
		s.logger.Error("failed to generate access token", "error", err)
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Generate refresh token
	refreshToken, err := s.db.CreateRefreshToken(r.Context(), user.ID)
	if err != nil {
		s.logger.Error("failed to create refresh token", "error", err)
		http.Error(w, "Failed to generate refresh token", http.StatusInternalServerError)
		return
	}

	// Return tokens
	response := TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    900, // 15 minutes in seconds
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("failed to encode response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (s *Server) handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req RefreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate refresh token
	user, err := s.db.ValidateRefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		switch err {
		case ErrRefreshTokenNotFound:
			http.Error(w, "Invalid refresh token", http.StatusUnauthorized)
		case ErrRefreshTokenExpired:
			http.Error(w, "Refresh token expired", http.StatusUnauthorized)
		default:
			s.logger.Error("failed to validate refresh token", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// Generate new access token
	accessToken, err := s.tokenManager.GenerateToken(user)
	if err != nil {
		s.logger.Error("failed to generate access token", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Generate new refresh token
	refreshToken, err := s.db.CreateRefreshToken(r.Context(), user.ID)
	if err != nil {
		s.logger.Error("failed to create refresh token", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Return new tokens
	response := TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    900, // 15 minutes in seconds
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("failed to encode response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
