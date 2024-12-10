package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJWKS(t *testing.T) {
	testdb := setupTestDB(t)
	defer testdb.teardown(t)

	srv, err := NewServer(testdb.DB)
	require.NoError(t, err)

	t.Run("JWKS Endpoint", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil)
		w := httptest.NewRecorder()

		srv.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var jwks JWKS
		err := json.NewDecoder(w.Body).Decode(&jwks)
		require.NoError(t, err)

		// Verify JWKS structure
		require.Len(t, jwks.Keys, 1)
		key := jwks.Keys[0]
		require.Equal(t, "RSA", key.Kty)
		require.Equal(t, "RS256", key.Alg)
		require.Equal(t, "sig", key.Use)
		require.NotEmpty(t, key.N)
		require.NotEmpty(t, key.E)
		require.NotEmpty(t, key.X5c)
	})

	t.Run("Invalid Method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/.well-known/jwks.json", nil)
		w := httptest.NewRecorder()

		srv.ServeHTTP(w, req)

		require.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})

	t.Run("Cache Headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil)
		w := httptest.NewRecorder()

		srv.ServeHTTP(w, req)

		require.Equal(t, "public, max-age=3600", w.Header().Get("Cache-Control"))
	})
}
