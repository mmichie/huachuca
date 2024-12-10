package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCORS(t *testing.T) {
	config := &CORSConfig{
		AllowedOrigins: []string{"http://localhost:3000", "https://app.example.com"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
		MaxAge:         3600,
	}

	middleware := NewCORSMiddleware(config)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name           string
		method         string
		origin         string
		expectedStatus int
		checkHeaders   bool
	}{
		{
			name:           "Allowed origin",
			method:         "GET",
			origin:         "http://localhost:3000",
			expectedStatus: http.StatusOK,
			checkHeaders:   true,
		},
		{
			name:           "Disallowed origin",
			method:         "GET",
			origin:         "http://evil.com",
			expectedStatus: http.StatusOK,
			checkHeaders:   false,
		},
		{
			name:           "OPTIONS request",
			method:         "OPTIONS",
			origin:         "http://localhost:3000",
			expectedStatus: http.StatusOK,
			checkHeaders:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/test", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			require.Equal(t, tc.expectedStatus, w.Code)

			if tc.checkHeaders {
				require.Equal(t, tc.origin, w.Header().Get("Access-Control-Allow-Origin"))
				require.NotEmpty(t, w.Header().Get("Access-Control-Allow-Methods"))
				require.NotEmpty(t, w.Header().Get("Access-Control-Allow-Headers"))
				require.NotEmpty(t, w.Header().Get("Access-Control-Max-Age"))
				require.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
			} else {
				require.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
			}
		})
	}
}
