package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrRefreshTokenNotFound = errors.New("refresh token not found")
	ErrRefreshTokenExpired = errors.New("refresh token expired")
)

type RefreshToken struct {
	ID        uuid.UUID `db:"id" json:"id"`
	UserID    uuid.UUID `db:"user_id" json:"user_id"`
	TokenHash string    `db:"token_hash" json:"-"`
	ExpiresAt time.Time `db:"expires_at" json:"expires_at"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// GenerateRefreshToken creates a new refresh token string
func GenerateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// HashToken creates a SHA-256 hash of the token
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// CreateRefreshToken creates a new refresh token for a user
func (db *DB) CreateRefreshToken(ctx context.Context, userID uuid.UUID) (string, error) {
    // First cleanup any expired tokens
    if err := db.CleanupExpiredTokens(ctx); err != nil {
        return "", err
    }

    // Generate the token
    token, err := GenerateRefreshToken()
    if err != nil {
        return "", err
    }

    // Hash the token for storage
    tokenHash := HashToken(token)

    // Delete any existing refresh tokens for this user
    _, err = db.ExecContext(ctx, `
        DELETE FROM refresh_tokens WHERE user_id = $1
    `, userID)
    if err != nil {
        return "", err
    }

    // Create new refresh token
    refreshToken := &RefreshToken{
        ID:        uuid.New(),
        UserID:    userID,
        TokenHash: tokenHash,
        ExpiresAt: time.Now().Add(7 * 24 * time.Hour), // 7 days
    }

    _, err = db.ExecContext(ctx, `
        INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at)
        VALUES ($1, $2, $3, $4)
    `, refreshToken.ID, refreshToken.UserID, refreshToken.TokenHash, refreshToken.ExpiresAt)
    if err != nil {
        return "", err
    }

    return token, nil
}

// ValidateRefreshToken validates a refresh token and returns the associated user
func (db *DB) ValidateRefreshToken(ctx context.Context, token string) (*User, error) {
    // First cleanup expired tokens
    if err := db.CleanupExpiredTokens(ctx); err != nil {
        return nil, err
    }

    tokenHash := HashToken(token)

    var rt RefreshToken
    err := db.GetContext(ctx, &rt, `
        SELECT * FROM refresh_tokens
        WHERE token_hash = $1
        AND expires_at > NOW()
    `, tokenHash)
    if err != nil {
        return nil, ErrRefreshTokenNotFound
    }

    // Get associated user
    user, err := db.GetUser(ctx, rt.UserID)
    if err != nil {
        return nil, err
    }

    return user, nil
}

// InvalidateRefreshToken deletes a refresh token
func (db *DB) InvalidateRefreshToken(ctx context.Context, token string) error {
	tokenHash := HashToken(token)

	_, err := db.ExecContext(ctx, `
		DELETE FROM refresh_tokens WHERE token_hash = $1
	`, tokenHash)
	return err
}

// InvalidateUserRefreshTokens deletes all refresh tokens for a user
func (db *DB) InvalidateUserRefreshTokens(ctx context.Context, userID uuid.UUID) error {
	_, err := db.ExecContext(ctx, `
		DELETE FROM refresh_tokens WHERE user_id = $1
	`, userID)
	return err
}

func (db *DB) CleanupExpiredTokens(ctx context.Context) error {
    _, err := db.ExecContext(ctx, `
        DELETE FROM refresh_tokens
        WHERE expires_at <= NOW()
    `)
    return err
}
