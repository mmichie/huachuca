package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"encoding/json"
	"github.com/stretchr/testify/require"
)

func TestConcurrentAccess(t *testing.T) {
	suite := setupIntegrationTest(t)
	defer suite.cleanupDB.teardown(t)

	t.Run("Concurrent Organization Creation", func(t *testing.T) {
		var wg sync.WaitGroup
		numGoroutines := 10
		organizations := make([]*Organization, numGoroutines)
		errors := make([]error, numGoroutines)

		// Create organizations concurrently
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				createOrgReq := CreateOrganizationRequest{
					Name:       fmt.Sprintf("Concurrent Org %d", index),
					OwnerEmail: fmt.Sprintf("owner%d@test.com", index),
					OwnerName:  fmt.Sprintf("Owner %d", index),
				}

				w := suite.makeRequest(t, "POST", "/organizations", createOrgReq)
				if w.Code != 200 {
					errors[index] = fmt.Errorf("failed to create organization: status %d", w.Code)
					return
				}

				var org Organization
				if err := json.NewDecoder(w.Body).Decode(&org); err != nil {
					errors[index] = err
					return
				}
				organizations[index] = &org
			}(i)
		}

		wg.Wait()

		// Verify all organizations were created
		for i, err := range errors {
			require.NoError(t, err, "Organization %d creation failed", i)
			require.NotNil(t, organizations[i], "Organization %d was not created", i)
		}
	})

	t.Run("Concurrent User Addition", func(t *testing.T) {
		// First create an organization
		createOrgReq := CreateOrganizationRequest{
			Name:       "Concurrent Users Org",
			OwnerEmail: "concurrent.owner@test.com",
			OwnerName:  "Concurrent Owner",
		}

		w := suite.makeRequest(t, "POST", "/organizations", createOrgReq)
		require.Equal(t, 200, w.Code)

		var org Organization
		err := json.NewDecoder(w.Body).Decode(&org)
		require.NoError(t, err)

		// Get owner token
		var owner User
		err = suite.db.GetContext(context.Background(), &owner,
			`SELECT * FROM users WHERE email = $1`, createOrgReq.OwnerEmail)
		require.NoError(t, err)

		ownerToken, err := suite.server.tokenManager.GenerateToken(&owner)
		require.NoError(t, err)

		// Store original token
		originalToken := suite.token
		suite.token = ownerToken

		var wg sync.WaitGroup
		numGoroutines := 5 // Max sub accounts allowed
		users := make([]*User, numGoroutines)
		errors := make([]error, numGoroutines)

		// Add users concurrently
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				addUserReq := AddUserRequest{
					Email: fmt.Sprintf("concurrent.user%d@test.com", index),
					Name:  fmt.Sprintf("Concurrent User %d", index),
				}

				w := suite.makeRequest(t, "POST",
					fmt.Sprintf("/organizations/%s/users", org.ID), addUserReq)
				if w.Code != 200 {
					errors[index] = fmt.Errorf("failed to add user: status %d", w.Code)
					return
				}

				var user User
				if err := json.NewDecoder(w.Body).Decode(&user); err != nil {
					errors[index] = err
					return
				}
				users[index] = &user
			}(i)
		}

		wg.Wait()

		// Restore original token
		suite.token = originalToken

		// Verify all users were added
		for i, err := range errors {
			require.NoError(t, err, "User %d addition failed", i)
			require.NotNil(t, users[i], "User %d was not added", i)
		}

		// Verify total number of users in organization
		var count int
		err = suite.db.GetContext(context.Background(), &count,
			`SELECT COUNT(*) FROM users WHERE organization_id = $1`, org.ID)
		require.NoError(t, err)
		require.Equal(t, numGoroutines+1, count) // +1 for owner
	})

	t.Run("Concurrent Database Operations", func(t *testing.T) {
		ctx := context.Background()
		var wg sync.WaitGroup
		numGoroutines := 20
		errors := make([]error, numGoroutines)

		// Perform concurrent database operations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				// Mix of different database operations
				switch index % 4 {
				case 0:
					// Query organizations
					var orgs []Organization
					if err := suite.db.SelectContext(ctx, &orgs,
						`SELECT * FROM organizations LIMIT 5`); err != nil {
						errors[index] = err
					}
				case 1:
					// Query users
					var users []User
					if err := suite.db.SelectContext(ctx, &users,
						`SELECT * FROM users LIMIT 5`); err != nil {
						errors[index] = err
					}
				case 2:
					// Check organization existence
					var exists bool
					if err := suite.db.GetContext(ctx, &exists,
						`SELECT EXISTS(SELECT 1 FROM organizations LIMIT 1)`); err != nil {
						errors[index] = err
					}
				case 3:
					// Count users
					var count int
					if err := suite.db.GetContext(ctx, &count,
						`SELECT COUNT(*) FROM users`); err != nil {
						errors[index] = err
					}
				}
			}(i)
		}

		wg.Wait()

		// Verify no errors occurred
		for i, err := range errors {
			require.NoError(t, err, "Database operation %d failed", i)
		}
	})

	t.Run("Connection Pool Under Load", func(t *testing.T) {
		ctx := context.Background()
		var wg sync.WaitGroup
		numGoroutines := 50 // More than max connections
		errors := make([]error, numGoroutines)
		timeout := time.After(5 * time.Second)
		done := make(chan bool)

		go func() {
			for i := 0; i < numGoroutines; i++ {
				wg.Add(1)
				go func(index int) {
					defer wg.Done()
					// Simple query that holds connection briefly
					var count int
					if err := suite.db.GetContext(ctx, &count,
						`SELECT COUNT(*) FROM users`); err != nil {
						errors[index] = err
					}
					time.Sleep(100 * time.Millisecond) // Simulate work
				}(i)
			}
			wg.Wait()
			done <- true
		}()

		// Wait for either completion or timeout
		select {
		case <-timeout:
			t.Fatal("Test timed out")
		case <-done:
			// Check connection pool stats
			stats := suite.db.Stats()
			t.Logf("Max open connections: %d", stats.MaxOpenConnections)
			t.Logf("Open connections: %d", stats.OpenConnections)
			t.Logf("In use: %d", stats.InUse)
			t.Logf("Idle: %d", stats.Idle)

			// Verify no errors occurred
			for i, err := range errors {
				require.NoError(t, err, "Database operation %d failed under load", i)
			}
		}
	})
}
