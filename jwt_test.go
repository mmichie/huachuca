package main

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestJWT(t *testing.T) {
	tm, err := NewTokenManager()
	require.NoError(t, err)

	user := &User{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Role:           "owner",
	}

	t.Run("Generate and validate token", func(t *testing.T) {
		token, err := tm.GenerateToken(user)
		require.NoError(t, err)
		require.NotEmpty(t, token)

		claims, err := tm.ValidateToken(token)
		require.NoError(t, err)
		require.Equal(t, user.ID, claims.UserID)
		require.Equal(t, user.OrganizationID, claims.OrganizationID)
		require.Equal(t, user.Role, claims.Role)
	})

	t.Run("Expired token", func(t *testing.T) {
		claims := Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
				NotBefore: jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			},
			UserID:         user.ID,
			OrganizationID: user.OrganizationID,
			Role:           user.Role,
		}

		token := jwt.NewWithClaims(jwt.SigningMethodRS256, &claims)
		tokenString, err := token.SignedString(tm.privateKey)
		require.NoError(t, err)

		_, err = tm.ValidateToken(tokenString)
		require.Error(t, err)
	})
}
