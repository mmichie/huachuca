package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHealthCheck(t *testing.T) {
	// Create a test database
	testdb := setupTestDB(t)
	defer testdb.teardown(t)

	// Create server with actual database
	srv, err := NewServer(testdb.DB)
	require.NoError(t, err)

	tests := []struct {
		name           string
		method         string
		expectedStatus int
		expectedDB     bool
	}{
		{
			name:           "GET request returns OK",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			expectedDB:     true,
		},
		{
			name:           "POST request returns Method Not Allowed",
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedDB:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/health", nil)
			w := httptest.NewRecorder()

			srv.ServeHTTP(w, req)

			require.Equal(t, tc.expectedStatus, w.Code)

			if tc.expectedStatus == http.StatusOK {
				var resp HealthResponse
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				require.Equal(t, "ok", resp.Status)
				require.Equal(t, tc.expectedDB, resp.DB)
			}
		})
	}
}
