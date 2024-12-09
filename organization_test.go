package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOrganizationOperations(t *testing.T) {
	testdb := setupTestDB(t)
	defer testdb.teardown(t)

	ctx := context.Background()

	t.Run("Create organization with owner", func(t *testing.T) {
		org, err := testdb.DB.CreateOrganization(ctx, "Test Org", "owner@test.com", "Test Owner")
		require.NoError(t, err)
		require.NotNil(t, org)
		require.Equal(t, "Test Org", org.Name)

		// Verify organization in database
		dbOrg, err := testdb.DB.GetOrganization(ctx, org.ID)
		require.NoError(t, err)
		require.Equal(t, org.ID, dbOrg.ID)
		require.Equal(t, "Test Org", dbOrg.Name)

		// Verify owner was created
		users, err := testdb.DB.GetOrganizationUsers(ctx, org.ID)
		require.NoError(t, err)
		require.Len(t, users, 1)
		require.Equal(t, "owner@test.com", users[0].Email)
		require.Equal(t, "owner", users[0].Role)
	})

	t.Run("Prevent duplicate emails", func(t *testing.T) {
		_, err := testdb.DB.CreateOrganization(ctx, "Test Org 2", "owner@test.com", "Test Owner 2")
		require.ErrorIs(t, err, ErrEmailTaken)
	})

	t.Run("Add users to organization", func(t *testing.T) {
		// Create initial organization
		org, err := testdb.DB.CreateOrganization(ctx, "Test Org 3", "owner3@test.com", "Test Owner 3")
		require.NoError(t, err)

		// Add sub-account
		user, err := testdb.DB.AddUserToOrganization(ctx, org.ID, "sub1@test.com", "Sub User 1")
		require.NoError(t, err)
		require.Equal(t, "sub_account", user.Role)
		require.Equal(t, org.ID, user.OrganizationID)

		// Verify users
		users, err := testdb.DB.GetOrganizationUsers(ctx, org.ID)
		require.NoError(t, err)
		require.Len(t, users, 2) // owner + sub-account
	})

	t.Run("Enforce max sub-accounts limit", func(t *testing.T) {
		org, err := testdb.DB.CreateOrganization(ctx, "Test Org 4", "owner4@test.com", "Test Owner 4")
		require.NoError(t, err)

		// Add max number of sub-accounts with unique emails
		for i := 0; i < 5; i++ {
			_, err := testdb.DB.AddUserToOrganization(ctx, org.ID,
				fmt.Sprintf("sub4_%d@test.com", i), // Changed to ensure unique emails
				fmt.Sprintf("Sub User %d", i))
			require.NoError(t, err, "Failed to add sub-account %d", i)
		}

		// Try to add one more
		_, err = testdb.DB.AddUserToOrganization(ctx, org.ID, "extra4@test.com", "Extra User")
		require.ErrorIs(t, err, ErrMaxSubAccounts)
	})
}
