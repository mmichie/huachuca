package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type HealthResponse struct {
	Status string `json:"status"`
	DB     bool   `json:"database"`
}

func TestHealthCheck(t *testing.T) {
	// Setup test database
	testdb := setupTestDB(t)
	defer testdb.teardown(t)

	// Create server with test database
	srv := NewServer(testdb.DB)

	// Test cases
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

			if w.Code != tc.expectedStatus {
				t.Errorf("Expected status code %d, got %d", tc.expectedStatus, w.Code)
			}

			if tc.expectedStatus == http.StatusOK {
				var resp HealthResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if resp.Status != "ok" {
					t.Errorf("Expected status 'ok', got '%s'", resp.Status)
				}

				if resp.DB != tc.expectedDB {
					t.Errorf("Expected DB status %v, got %v", tc.expectedDB, resp.DB)
				}
			}
		})
	}
}
