package main

import (
	"testing"
)

func TestDatabaseConnection(t *testing.T) {
	testdb := setupTestDB(t)
	defer testdb.teardown(t)

	// Test database ping
	if err := testdb.DB.Ping(); err != nil {
		t.Errorf("Failed to ping database: %v", err)
	}

	// Test simple query
	var result int
	err := testdb.DB.Get(&result, "SELECT 1")
	if err != nil {
		t.Errorf("Failed to execute simple query: %v", err)
	}
	if result != 1 {
		t.Errorf("Expected 1, got %d", result)
	}
}
