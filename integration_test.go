package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// IntegrationTestSuite holds the test state
type IntegrationTestSuite struct {
	server      *Server
	db          *DB
	token       string
	cleanupDB   *testDB
	initialOrg  *Organization
	initialUser *User
}

func setupIntegrationTest(t *testing.T) *IntegrationTestSuite {
	testdb := setupTestDB(t)

	srv, err := NewServer(testdb.DB)
	require.NoError(t, err)

	// Create initial test user and organization
	initialUser := &User{
		ID:          uuid.New(),
		Email:       "initial@test.com",
		Name:        "Initial User",
		Role:        "owner",
		Permissions: Permissions{
			string(PermCreateOrg):     true,
			string(PermReadOrg):       true,
			string(PermUpdateOrg):     true,
			string(PermDeleteOrg):     true,
			string(PermInviteUser):    true,
			string(PermRemoveUser):    true,
			string(PermUpdateUser):    true,
			string(PermManageSettings): true,
			"admin":                   true,
		},
	}

	initialOrg := &Organization{
		ID:              uuid.New(),
		Name:            "Initial Org",
		OwnerID:         initialUser.ID,
		SubscriptionTier: "free",
		MaxSubAccounts:  5,
	}

	// Set the organization ID for the user
	initialUser.OrganizationID = initialOrg.ID

	// Insert organization first
	_, err = testdb.DB.ExecContext(context.Background(), `
		INSERT INTO organizations (id, name, owner_id, subscription_tier, max_sub_accounts)
		VALUES ($1, $2, $3, $4, $5)
	`, initialOrg.ID, initialOrg.Name, initialOrg.OwnerID, initialOrg.SubscriptionTier, initialOrg.MaxSubAccounts)
	require.NoError(t, err)

	// Then insert the user
	_, err = testdb.DB.ExecContext(context.Background(), `
		INSERT INTO users (id, email, name, organization_id, role, permissions)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, initialUser.ID, initialUser.Email, initialUser.Name, initialUser.OrganizationID, initialUser.Role, initialUser.Permissions)
	require.NoError(t, err)

	// Generate token for initial user
	token, err := srv.tokenManager.GenerateToken(initialUser)
	require.NoError(t, err)

	return &IntegrationTestSuite{
		server:      srv,
		db:          testdb.DB,
		token:       token,
		cleanupDB:   testdb,
		initialOrg:  initialOrg,
		initialUser: initialUser,
	}
}

func (s *IntegrationTestSuite) makeRequest(t *testing.T, method, path string, body interface{}) *httptest.ResponseRecorder {
	var bodyReader bytes.Buffer
	if body != nil {
		err := json.NewEncoder(&bodyReader).Encode(body)
		require.NoError(t, err)
	}

	req := httptest.NewRequest(method, path, &bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if s.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.token))
	}

	w := httptest.NewRecorder()
	s.server.ServeHTTP(w, req)
	return w
}

func TestUserFlow(t *testing.T) {
	suite := setupIntegrationTest(t)
	defer suite.cleanupDB.teardown(t)

	t.Run("Complete User Flow", func(t *testing.T) {
		// Create a new organization (using initial token)
		createOrgReq := CreateOrganizationRequest{
			Name:       "Test Organization",
			OwnerEmail: "owner@test.com",
			OwnerName:  "Test Owner",
		}

		w := suite.makeRequest(t, http.MethodPost, "/organizations", createOrgReq)
		require.Equal(t, http.StatusOK, w.Code)

		var org Organization
		err := json.NewDecoder(w.Body).Decode(&org)
		require.NoError(t, err)
		require.NotEmpty(t, org.ID)

		// Get the created owner to use their token
		var owner User
		err = suite.db.GetContext(context.Background(), &owner,
			`SELECT * FROM users WHERE email = $1`, createOrgReq.OwnerEmail)
		require.NoError(t, err)

		// Generate token for owner
		ownerToken, err := suite.server.tokenManager.GenerateToken(&owner)
		require.NoError(t, err)

		// Store original token and use owner's token
		originalToken := suite.token
		suite.token = ownerToken

		// Add a sub-account user
		addUserReq := AddUserRequest{
			Email: "sub@test.com",
			Name:  "Sub User",
		}

		w = suite.makeRequest(t, http.MethodPost,
			fmt.Sprintf("/organizations/%s/users", org.ID), addUserReq)
		require.Equal(t, http.StatusOK, w.Code)

		var newUser User
		err = json.NewDecoder(w.Body).Decode(&newUser)
		require.NoError(t, err)
		require.Equal(t, "sub_account", newUser.Role)

		// Verify organization users
		w = suite.makeRequest(t, http.MethodGet,
			fmt.Sprintf("/organizations/%s", org.ID), nil)
		require.Equal(t, http.StatusOK, w.Code)

		var users []User
		err = json.NewDecoder(w.Body).Decode(&users)
		require.NoError(t, err)
		require.Len(t, users, 2)

		// Verify roles
		hasOwner := false
		hasSubAccount := false
		for _, user := range users {
			switch user.Role {
			case "owner":
				hasOwner = true
				require.True(t, user.HasPermission(PermCreateOrg))
			case "sub_account":
				hasSubAccount = true
				require.False(t, user.HasPermission(PermCreateOrg))
			}
		}
		require.True(t, hasOwner, "Organization should have an owner")
		require.True(t, hasSubAccount, "Organization should have a sub-account")

		// Restore original token
		suite.token = originalToken
	})

t.Run("Error Cases", func(t *testing.T) {
		// Test duplicate email
		createOrgReq := CreateOrganizationRequest{
			Name:       "Another Org",
			OwnerEmail: "owner@test.com", // Already used in previous test
			OwnerName:  "Another Owner",
		}

		w := suite.makeRequest(t, http.MethodPost, "/organizations", createOrgReq)
		require.Equal(t, http.StatusConflict, w.Code)

		// First create a valid organization for testing invalid user operations
		validOrgReq := CreateOrganizationRequest{
			Name:       "Valid Test Org",
			OwnerEmail: "valid.owner@test.com",
			OwnerName:  "Valid Owner",
		}

		w = suite.makeRequest(t, http.MethodPost, "/organizations", validOrgReq)
		require.Equal(t, http.StatusOK, w.Code)

		var org Organization
		err := json.NewDecoder(w.Body).Decode(&org)
		require.NoError(t, err)

		// Get the owner's token
		var owner User
		err = suite.db.GetContext(context.Background(), &owner,
			`SELECT * FROM users WHERE email = $1`, validOrgReq.OwnerEmail)
		require.NoError(t, err)

		ownerToken, err := suite.server.tokenManager.GenerateToken(&owner)
		require.NoError(t, err)

		// Use owner's token for user operations
		originalToken := suite.token
		suite.token = ownerToken

		// Test invalid organization ID format
		addUserReq := AddUserRequest{
			Email: "new@test.com",
			Name:  "New User",
		}

		w = suite.makeRequest(t, http.MethodPost,
			"/organizations/invalid-uuid/users", addUserReq)
		require.Equal(t, http.StatusBadRequest, w.Code)

		// Test invalid email format with valid organization
		addUserReq.Email = "not-an-email"
		w = suite.makeRequest(t, http.MethodPost,
			fmt.Sprintf("/organizations/%s/users", org.ID), addUserReq)
		require.Equal(t, http.StatusBadRequest, w.Code)

		// Test non-existent organization with valid UUID
		w = suite.makeRequest(t, http.MethodPost,
			fmt.Sprintf("/organizations/%s/users", uuid.New()), addUserReq)
		require.Equal(t, http.StatusForbidden, w.Code)

		// Restore original token
		suite.token = originalToken
	})

	t.Run("Permission Checks", func(t *testing.T) {
		// Create organization with owner
		createOrgReq := CreateOrganizationRequest{
			Name:       "Perm Test Org",
			OwnerEmail: "perm.owner@test.com",
			OwnerName:  "Perm Owner",
		}

		w := suite.makeRequest(t, http.MethodPost, "/organizations", createOrgReq)
		require.Equal(t, http.StatusOK, w.Code)

		var org Organization
		err := json.NewDecoder(w.Body).Decode(&org)
		require.NoError(t, err)

		// Get the created owner
		var owner User
		err = suite.db.GetContext(context.Background(), &owner,
			`SELECT * FROM users WHERE email = $1`, createOrgReq.OwnerEmail)
		require.NoError(t, err)

		// Create sub-account token
		ownerToken, err := suite.server.tokenManager.GenerateToken(&owner)
		require.NoError(t, err)

		// Store original token and use owner's token
		originalToken := suite.token
		suite.token = ownerToken

		// Add a sub-account
		addUserReq := AddUserRequest{
			Email: "perm.sub@test.com",
			Name:  "Perm Sub User",
		}

		w = suite.makeRequest(t, http.MethodPost,
			fmt.Sprintf("/organizations/%s/users", org.ID), addUserReq)
		require.Equal(t, http.StatusOK, w.Code)

		var subUser User
		err = json.NewDecoder(w.Body).Decode(&subUser)
		require.NoError(t, err)

		// Generate token for sub-account
		subToken, err := suite.server.tokenManager.GenerateToken(&subUser)
		require.NoError(t, err)

		// Try operations with sub-account token
		suite.token = subToken

		// Try to create organization (should fail)
		createOrgReq = CreateOrganizationRequest{
			Name:       "Sub Org",
			OwnerEmail: "sub.owner@test.com",
			OwnerName:  "Sub Owner",
		}

		w = suite.makeRequest(t, http.MethodPost, "/organizations", createOrgReq)
		require.Equal(t, http.StatusForbidden, w.Code)

		// Try to add user (should fail)
		addUserReq = AddUserRequest{
			Email: "another.sub@test.com",
			Name:  "Another Sub User",
		}

		w = suite.makeRequest(t, http.MethodPost,
			fmt.Sprintf("/organizations/%s/users", org.ID), addUserReq)
		require.Equal(t, http.StatusForbidden, w.Code)

		// Restore original token
		suite.token = originalToken
	})

	t.Run("Cross-Organization Access", func(t *testing.T) {
		// Create second organization
		createOrgReq := CreateOrganizationRequest{
			Name:       "Second Org",
			OwnerEmail: "second.owner@test.com",
			OwnerName:  "Second Owner",
		}

		w := suite.makeRequest(t, http.MethodPost, "/organizations", createOrgReq)
		require.Equal(t, http.StatusOK, w.Code)

		var secondOrg Organization
		err := json.NewDecoder(w.Body).Decode(&secondOrg)
		require.NoError(t, err)

		// Try to access second org with initial token (should fail)
		w = suite.makeRequest(t, http.MethodGet,
			fmt.Sprintf("/organizations/%s", secondOrg.ID), nil)
		require.Equal(t, http.StatusForbidden, w.Code)

		// Try to add user to second org with initial token (should fail)
		addUserReq := AddUserRequest{
			Email: "cross.org@test.com",
			Name:  "Cross Org User",
		}

		w = suite.makeRequest(t, http.MethodPost,
			fmt.Sprintf("/organizations/%s/users", secondOrg.ID), addUserReq)
		require.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("Invalid Token Cases", func(t *testing.T) {
		// Test with invalid token
		oldToken := suite.token
		suite.token = "invalid.token.here"

		w := suite.makeRequest(t, http.MethodGet,
			fmt.Sprintf("/organizations/%s", suite.initialOrg.ID), nil)
		require.Equal(t, http.StatusUnauthorized, w.Code)

		// Test with empty token
		suite.token = ""
		w = suite.makeRequest(t, http.MethodGet,
			fmt.Sprintf("/organizations/%s", suite.initialOrg.ID), nil)
		require.Equal(t, http.StatusUnauthorized, w.Code)

		// Restore valid token
		suite.token = oldToken
	})
}


func TestAuthFlow(t *testing.T) {
	suite := setupIntegrationTest(t)
	defer suite.cleanupDB.teardown(t)

	t.Run("OAuth and Refresh Token Flow", func(t *testing.T) {
		// Use initial token for authorization
		createOrgReq := CreateOrganizationRequest{
			Name:       "Test OAuth Org",
			OwnerEmail: "oauth.test@example.com",
			OwnerName:  "OAuth Test User",
		}

		// Create organization
		w := suite.makeRequest(t, http.MethodPost, "/organizations", createOrgReq)
		require.Equal(t, http.StatusOK, w.Code)

		var org Organization
		err := json.NewDecoder(w.Body).Decode(&org)
		require.NoError(t, err)

		// Get the created user and generate tokens
		time.Sleep(100 * time.Millisecond) // Small delay to ensure db write is complete

		var user User
		err = suite.db.GetContext(context.Background(), &user,
			`SELECT * FROM users WHERE email = $1`, createOrgReq.OwnerEmail)
		require.NoError(t, err)
		require.NotEmpty(t, user.ID)

		// Generate refresh token
		refreshToken, err := suite.db.CreateRefreshToken(context.Background(), user.ID)
		require.NoError(t, err)

		// Verify refresh token was stored
		var count int
		err = suite.db.GetContext(context.Background(), &count,
			`SELECT COUNT(*) FROM refresh_tokens WHERE user_id = $1`, user.ID)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		// Test token refresh
		refreshReq := RefreshTokenRequest{
			RefreshToken: refreshToken,
		}

		w = suite.makeRequest(t, http.MethodPost, "/auth/refresh", refreshReq)
		require.Equal(t, http.StatusOK, w.Code)

		var tokenResp TokenResponse
		err = json.NewDecoder(w.Body).Decode(&tokenResp)
		require.NoError(t, err)
		require.NotEmpty(t, tokenResp.AccessToken)
		require.NotEmpty(t, tokenResp.RefreshToken)
		require.Equal(t, 900, tokenResp.ExpiresIn)

		// Verify old refresh token was replaced
		err = suite.db.GetContext(context.Background(), &count,
			`SELECT COUNT(*) FROM refresh_tokens WHERE user_id = $1`, user.ID)
		require.NoError(t, err)
		require.Equal(t, 1, count, "Should still have exactly one refresh token")

		// Try to use the old refresh token (should fail)
		refreshReq.RefreshToken = refreshToken
		w = suite.makeRequest(t, http.MethodPost, "/auth/refresh", refreshReq)
		require.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Refresh Token Expiration", func(t *testing.T) {
		// Create a user with an expired refresh token
		createOrgReq := CreateOrganizationRequest{
			Name:       "Expired Token Org",
			OwnerEmail: "expired.test@example.com",
			OwnerName:  "Expired Test User",
		}

		w := suite.makeRequest(t, http.MethodPost, "/organizations", createOrgReq)
		require.Equal(t, http.StatusOK, w.Code)

		time.Sleep(100 * time.Millisecond) // Small delay to ensure db write is complete

		var user User
		err := suite.db.GetContext(context.Background(), &user,
			`SELECT * FROM users WHERE email = $1`, createOrgReq.OwnerEmail)
		require.NoError(t, err)

		// Create expired refresh token
		token, err := GenerateRefreshToken()
		require.NoError(t, err)

		tokenHash := HashToken(token)
		_, err = suite.db.ExecContext(context.Background(), `
			INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at)
			VALUES ($1, $2, $3, $4)
		`, uuid.New(), user.ID, tokenHash, time.Now().Add(-24*time.Hour))
		require.NoError(t, err)

		// Try to use expired token
		refreshReq := RefreshTokenRequest{
			RefreshToken: token,
		}

		w = suite.makeRequest(t, http.MethodPost, "/auth/refresh", refreshReq)
		require.Equal(t, http.StatusUnauthorized, w.Code)

		// Verify expired token was cleaned up
		time.Sleep(100 * time.Millisecond) // Small delay to ensure cleanup is complete

		var count int
		err = suite.db.GetContext(context.Background(), &count,
			`SELECT COUNT(*) FROM refresh_tokens WHERE user_id = $1`, user.ID)
		require.NoError(t, err)
		require.Equal(t, 0, count, "Expired token should be deleted")
	})

	t.Run("Invalid Refresh Token", func(t *testing.T) {
		refreshReq := RefreshTokenRequest{
			RefreshToken: "invalid-token",
		}

		w := suite.makeRequest(t, http.MethodPost, "/auth/refresh", refreshReq)
		require.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Multiple Login Sessions", func(t *testing.T) {
		// Create initial user
		createOrgReq := CreateOrganizationRequest{
			Name:       "Multi Session Org",
			OwnerEmail: "multi.test@example.com",
			OwnerName:  "Multi Test User",
		}

		w := suite.makeRequest(t, http.MethodPost, "/organizations", createOrgReq)
		require.Equal(t, http.StatusOK, w.Code)

		time.Sleep(100 * time.Millisecond) // Small delay to ensure db write is complete

		var user User
		err := suite.db.GetContext(context.Background(), &user,
			`SELECT * FROM users WHERE email = $1`, createOrgReq.OwnerEmail)
		require.NoError(t, err)

		// Create first refresh token
		token1, err := suite.db.CreateRefreshToken(context.Background(), user.ID)
		require.NoError(t, err)

		// Verify first token works
		refreshReq := RefreshTokenRequest{
			RefreshToken: token1,
		}

		w = suite.makeRequest(t, http.MethodPost, "/auth/refresh", refreshReq)
		require.Equal(t, http.StatusOK, w.Code)

		// Create second refresh token (simulating login from another device)
		token2, err := suite.db.CreateRefreshToken(context.Background(), user.ID)
		require.NoError(t, err)

		// Try to use the first token (should fail as it was invalidated)
		refreshReq.RefreshToken = token1
		w = suite.makeRequest(t, http.MethodPost, "/auth/refresh", refreshReq)
		require.Equal(t, http.StatusUnauthorized, w.Code)

		// Verify second token works
		refreshReq.RefreshToken = token2
		w = suite.makeRequest(t, http.MethodPost, "/auth/refresh", refreshReq)
		require.Equal(t, http.StatusOK, w.Code)
	})
}
