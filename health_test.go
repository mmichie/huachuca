package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHealthCheck(t *testing.T) {
	testdb := setupTestDB(t)
	defer testdb.teardown(t)

	// Ensure goose_db_version table exists and has a version
	_, err := testdb.DB.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS goose_db_version (
			id SERIAL PRIMARY KEY,
			version_id bigint NOT NULL,
			is_applied boolean NOT NULL,
			tstamp timestamp with time zone DEFAULT now()
		);
		INSERT INTO goose_db_version (version_id, is_applied) VALUES (1, true);
	`)
	require.NoError(t, err)

	srv, err := NewServer(testdb.DB)
	require.NoError(t, err)

	t.Run("Healthy System", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()

		srv.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var resp HealthResponse
		err := json.NewDecoder(w.Body).Decode(&resp)
		require.NoError(t, err)
		require.Equal(t, StatusHealthy, resp.Status)
		require.NotEmpty(t, resp.Version)
		require.NotEmpty(t, resp.Checks)

		// Check that all individual checks were performed
		checkNames := make(map[string]bool)
		for _, check := range resp.Checks {
			checkNames[check.Name] = true
			require.NotZero(t, check.Duration)
			require.Equal(t, StatusHealthy, check.Status, "Check %s should be healthy", check.Name)
		}

		require.True(t, checkNames["database"])
		require.True(t, checkNames["migrations"])
		require.True(t, checkNames["memory"])
	})

	t.Run("Database Timeout", func(t *testing.T) {
		// Create a context that's already cancelled
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		time.Sleep(time.Millisecond) // Ensure timeout

		resp := srv.health.CheckHealth(ctx)
		require.Equal(t, StatusUnhealthy, resp.Status)

		var hasUnhealthyCheck bool
		for _, check := range resp.Checks {
			if check.Status == StatusUnhealthy {
				hasUnhealthyCheck = true
				break
			}
		}
		require.True(t, hasUnhealthyCheck)
	})

	t.Run("Method Not Allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/health", nil)
		w := httptest.NewRecorder()

		srv.ServeHTTP(w, req)

		require.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})
}
