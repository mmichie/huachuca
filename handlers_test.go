package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOrganizationHandlers(t *testing.T) {
	testdb := setupTestDB(t)
	defer testdb.teardown(t)

	srv := NewServer(testdb.DB)

	t.Run("Create Organization", func(t *testing.T) {
		payload := CreateOrganizationRequest{
			Name:       "Test Org",
			OwnerEmail: "owner@example.com",
			OwnerName:  "Test Owner",
		}

		body, err := json.Marshal(payload)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/organizations", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
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
			Name:       "Another Org",
			OwnerEmail: "owner@example.com", // Same email as previous test
			OwnerName:  "Another Owner",
		}

		body, err := json.Marshal(payload)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/organizations", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		srv.ServeHTTP(w, req)

		require.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("Add User to Organization", func(t *testing.T) {
		// First create an organization
		org, err := testdb.DB.CreateOrganization(
			context.Background(),
			"Test Org for Users",
			"org_owner@example.com",
			"Org Owner",
		)
		require.NoError(t, err)

		payload := AddUserRequest{
			Email: "newuser@example.com",
			Name:  "New User",
		}

		body, err := json.Marshal(payload)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodPost,
			fmt.Sprintf("/organizations/users?org_id=%s", org.ID),
			bytes.NewReader(body),
		)
		req.Header.Set("Content-Type", "application/json")
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
		// Create an organization with owner
		org, err := testdb.DB.CreateOrganization(
			context.Background(),
			"Test Org for Listing",
			"list_owner@example.com",
			"List Owner",
		)
		require.NoError(t, err)

		// Add a sub-account
		_, err = testdb.DB.AddUserToOrganization(
			context.Background(),
			org.ID,
			"list_sub@example.com",
			"List Sub User",
		)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodGet,
			fmt.Sprintf("/organizations/%s", org.ID),
			nil,
		)
		w := httptest.NewRecorder()

		srv.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var users []User
		err = json.NewDecoder(w.Body).Decode(&users)
		require.NoError(t, err)
		require.Len(t, users, 2) // Owner + sub-account

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
