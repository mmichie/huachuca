package main

import (
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
