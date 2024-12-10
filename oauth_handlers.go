package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

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
	b := make([]byte, 64) // Increased from 32 to 64 bytes for better security
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func (s *Server) handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state, err := generateState()
	if err != nil {
		s.logger.Error("failed to generate state", "error", err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}

	// Store state with 5-minute expiration
	s.stateStore.StoreState(state, 5*time.Minute)

	authURL := s.oauth.GetAuthURL(state)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func (s *Server) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state := r.URL.Query().Get("state")
	if state == "" {
		http.Error(w, "Missing state parameter", http.StatusBadRequest)
		return
	}

	// Validate and delete state atomically
	if !s.stateStore.ValidateAndDeleteState(state) {
		http.Error(w, "Invalid or expired state", http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Missing code parameter", http.StatusBadRequest)
		return
	}

	token, err := s.oauth.Exchange(r.Context(), code)
	if err != nil {
		s.logger.Error("failed to exchange token", "error", err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}

	googleUser, err := s.oauth.GetUserInfo(r.Context(), token)
	if err != nil {
		s.logger.Error("failed to get user info", "error", err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}

	// Look up user by email
	var user *User
	user, err = s.db.GetUserByEmail(r.Context(), googleUser.Email)
	if err != nil {
		s.logger.Error("database error during user lookup", "error", err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}

	if user == nil {
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
			http.Error(w, "Account creation failed", http.StatusInternalServerError)
			return
		}
	}

	// Generate JWT access token
	accessToken, err := s.tokenManager.GenerateToken(user)
	if err != nil {
		s.logger.Error("failed to generate access token", "error", err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}

	// Generate refresh token
	refreshToken, err := s.db.CreateRefreshToken(r.Context(), user.ID)
	if err != nil {
		s.logger.Error("failed to create refresh token", "error", err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
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
		return
	}
}

func (s *Server) handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
		case ErrRefreshTokenNotFound, ErrRefreshTokenExpired:
			http.Error(w, "Invalid or expired refresh token", http.StatusUnauthorized)
		default:
			s.logger.Error("failed to validate refresh token", "error", err)
			http.Error(w, "Authentication failed", http.StatusInternalServerError)
		}
		return
	}

	// Generate new access token
	accessToken, err := s.tokenManager.GenerateToken(user)
	if err != nil {
		s.logger.Error("failed to generate access token", "error", err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}

	// Generate new refresh token
	refreshToken, err := s.db.CreateRefreshToken(r.Context(), user.ID)
	if err != nil {
		s.logger.Error("failed to create refresh token", "error", err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
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
		return
	}
}
