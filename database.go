package main

import (
	"context"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// DB wraps sqlx.DB to add custom functionality
type DB struct {
	*sqlx.DB
}

// NewDB creates a new database connection
func NewDB(dataSourceName string) (*DB, error) {
	db, err := sqlx.Connect("postgres", dataSourceName)
	if err != nil {
		return nil, err
	}

	// Set reasonable defaults for connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)

	return &DB{DB: db}, nil
}

// Ping checks database connectivity
func (db *DB) Ping() error {
	return db.DB.Ping()
}

// GetUser retrieves a user by ID
func (db *DB) GetUser(ctx context.Context, id uuid.UUID) (*User, error) {
	user := &User{}
	err := db.GetContext(ctx, user, `
		SELECT id, email, name, organization_id, role, permissions, created_at
		FROM users WHERE id = $1
	`, id)
	if err != nil {
		return nil, err
	}
	return user, nil
}
