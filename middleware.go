package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

type contextKey string

const (
	userContextKey contextKey = "user"
)

type AuthMiddleware struct {
	tokenManager *TokenManager
	db           *DB
}

func NewAuthMiddleware(tokenManager *TokenManager, db *DB) *AuthMiddleware {
	return &AuthMiddleware{
		tokenManager: tokenManager,
		db:           db,
	}
}

// GetUserFromContext retrieves the user from the context
func GetUserFromContext(ctx context.Context) (*User, error) {
	user, ok := ctx.Value(userContextKey).(*User)
	if !ok {
		return nil, fmt.Errorf("user not found in context")
	}
	return user, nil
}

func (am *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Extract token from Bearer scheme
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid authorization header", http.StatusUnauthorized)
			return
		}

		claims, err := am.tokenManager.ValidateToken(parts[1])
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Get user from database to ensure they still exist and have proper permissions
		user, err := am.db.GetUser(r.Context(), claims.UserID)
		if err != nil {
			http.Error(w, "User not found", http.StatusUnauthorized)
			return
		}

		// Add user to request context
		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequirePermissions middleware ensures the user has all required permissions
func (am *AuthMiddleware) RequirePermissions(perms ...Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := GetUserFromContext(r.Context())
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if !user.HasAllPermissions(perms...) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAnyPermission middleware ensures the user has at least one of the required permissions
func (am *AuthMiddleware) RequireAnyPermission(perms ...Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := GetUserFromContext(r.Context())
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if !user.HasAnyPermission(perms...) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireSameOrg middleware ensures the user belongs to the organization they're trying to access
func (am *AuthMiddleware) RequireSameOrg(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := GetUserFromContext(r.Context())
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Extract org ID from URL or request body
		var targetOrgID string
		if strings.Contains(r.URL.Path, "/organizations/") {
			parts := strings.Split(r.URL.Path, "/")
			for i, part := range parts {
				if part == "organizations" && i+1 < len(parts) {
					targetOrgID = parts[i+1]
					break
				}
			}
		}

		if targetOrgID != "" && targetOrgID != user.OrganizationID.String() {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
