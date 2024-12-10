package main

import (
	"context"
	"embed"
	"fmt"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// testDB represents a test database instance
type testDB struct {
	Container *postgres.PostgresContainer
	DB        *DB
}

// setupTestDB creates a new Postgres container and returns a DB connection
func setupTestDB(t *testing.T) *testDB {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %s", err)
	}

	// Get the container's host and port
	host, err := pgContainer.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get container host: %s", err)
	}

	port, err := pgContainer.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("failed to get container port: %s", err)
	}

	// Construct the connection string
	connStr := fmt.Sprintf("postgres://test:test@%s:%d/test?sslmode=disable", host, port.Int())

	// Connect to the database
	db, err := NewDB(connStr)
	if err != nil {
		t.Fatalf("failed to connect to test database: %s", err)
	}

	// Run migrations
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("failed to set dialect: %s", err)
	}

	if err := goose.Up(db.DB.DB, "migrations"); err != nil {
		t.Fatalf("failed to run migrations: %s", err)
	}

	return &testDB{
		Container: pgContainer,
		DB:        db,
	}
}

// teardown closes the database connection and stops the container
func (tdb *testDB) teardown(t *testing.T) {
	t.Helper()

	if err := tdb.DB.Close(); err != nil {
		t.Errorf("failed to close database: %s", err)
	}

	if err := tdb.Container.Terminate(context.Background()); err != nil {
		t.Errorf("failed to terminate container: %s", err)
	}
}
