package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestOrganizationHandlers(t *testing.T) {
	testdb := setupTestDB(t)
	defer testdb.teardown(t)

	srv, err := NewServer(testdb.DB)
	require.NoError(t, err)

	// Create initial test user and organization
	testUser := &User{
		ID:          uuid.New(),
		Email:       "test@example.com",
		Name:        "Test User",
		Role:        "owner",
		Permissions: Permissions{"admin": true},
	}

	testOrg := &Organization{
		ID:               uuid.New(),
		Name:             "Test Org",
		OwnerID:          testUser.ID, // Set the owner ID
		SubscriptionTier: "free",
		MaxSubAccounts:   5,
	}

	// Set the organization ID for the user
	testUser.OrganizationID = testOrg.ID

	// Insert organization first
	_, err = testdb.DB.ExecContext(context.Background(), `
		INSERT INTO organizations (id, name, owner_id, subscription_tier, max_sub_accounts)
		VALUES ($1, $2, $3, $4, $5)
	`, testOrg.ID, testOrg.Name, testOrg.OwnerID, testOrg.SubscriptionTier, testOrg.MaxSubAccounts)
	require.NoError(t, err)

	// Then insert the user
	_, err = testdb.DB.ExecContext(context.Background(), `
		INSERT INTO users (id, email, name, organization_id, role, permissions)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, testUser.ID, testUser.Email, testUser.Name, testUser.OrganizationID, testUser.Role, testUser.Permissions)
	require.NoError(t, err)

	// Generate token for the test user
	token, err := srv.tokenManager.GenerateToken(testUser)
	require.NoError(t, err)

	t.Run("Create Organization", func(t *testing.T) {
		payload := CreateOrganizationRequest{
			Name:       "Another Org",
			OwnerEmail: "another@example.com",
			OwnerName:  "Another Owner",
		}

		body, err := json.Marshal(payload)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/organizations", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		w := httptest.NewRecorder()

		srv.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var resp Organization
		err = json.NewDecoder(w.Body).Decode(&resp)
		require.NoError(t, err)
		require.Equal(t, payload.Name, resp.Name)
	})

	t.Run("Create Organization - Duplicate Email", func(t *testing.T) {
		payload := CreateOrganizationRequest{
			Name:       "Duplicate Org",
			OwnerEmail: "another@example.com", // Same email as previous test
			OwnerName:  "Duplicate Owner",
		}

		body, err := json.Marshal(payload)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/organizations", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		w := httptest.NewRecorder()

		srv.ServeHTTP(w, req)

		require.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("Add User to Organization", func(t *testing.T) {
		payload := AddUserRequest{
			Email: "newuser@example.com",
			Name:  "New User",
		}

		body, err := json.Marshal(payload)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodPost,
			fmt.Sprintf("/organizations/%s/users", testOrg.ID),
			bytes.NewReader(body),
		)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		w := httptest.NewRecorder()

		srv.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var resp User
		err = json.NewDecoder(w.Body).Decode(&resp)
		require.NoError(t, err)
		require.Equal(t, payload.Email, resp.Email)
		require.Equal(t, payload.Name, resp.Name)
		require.Equal(t, "sub_account", resp.Role)
	})

	t.Run("Get Organization Users", func(t *testing.T) {
		// Add a sub-account to the test organization
		_, err = testdb.DB.AddUserToOrganization(
			context.Background(),
			testOrg.ID,
			"sub@example.com",
			"Sub User",
		)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodGet,
			fmt.Sprintf("/organizations/%s", testOrg.ID),
			nil,
		)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		w := httptest.NewRecorder()

		srv.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var users []User
		err = json.NewDecoder(w.Body).Decode(&users)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(users), 2) // At least owner + sub-account

		// Verify we have both an owner and a sub-account
		hasOwner := false
		hasSubAccount := false
		for _, user := range users {
			if user.Role == "owner" {
				hasOwner = true
			}
			if user.Role == "sub_account" {
				hasSubAccount = true
			}
		}
		require.True(t, hasOwner, "Organization should have an owner")
		require.True(t, hasSubAccount, "Organization should have a sub-account")
	})
}
