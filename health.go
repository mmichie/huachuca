package main

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"
)

type HealthStatus string

const (
	StatusHealthy   HealthStatus = "healthy"
	StatusDegraded  HealthStatus = "degraded"
	StatusUnhealthy HealthStatus = "unhealthy"
)

type HealthCheck struct {
	Name     string            `json:"name"`
	Status   HealthStatus      `json:"status"`
	Error    string            `json:"error,omitempty"`
	Details  map[string]string `json:"details,omitempty"`
	Duration time.Duration     `json:"duration"`
}

type HealthResponse struct {
	Status    HealthStatus  `json:"status"`
	Version   string        `json:"version"`
	Checks    []HealthCheck `json:"checks"`
	StartTime time.Time     `json:"start_time"`
	CheckTime time.Time     `json:"check_time"`
}

type HealthChecker struct {
	version   string
	startTime time.Time
	db        *DB
	logger    *slog.Logger
}

func NewHealthChecker(version string, db *DB, logger *slog.Logger) *HealthChecker {
	return &HealthChecker{
		version:   version,
		startTime: time.Now(),
		db:        db,
		logger:    logger,
	}
}

func (h *HealthChecker) CheckHealth(ctx context.Context) *HealthResponse {
	response := &HealthResponse{
		Status:    StatusHealthy,
		Version:   h.version,
		StartTime: h.startTime,
		CheckTime: time.Now(),
	}

	var wg sync.WaitGroup
	checks := make([]HealthCheck, 0)
	checksChan := make(chan HealthCheck, 3) // Buffer for all checks

	// Run all checks in parallel
	wg.Add(3)
	go func() {
		defer wg.Done()
		checksChan <- h.checkDatabase(ctx)
	}()

	go func() {
		defer wg.Done()
		checksChan <- h.checkMigrations(ctx)
	}()

	go func() {
		defer wg.Done()
		checksChan <- h.checkMemory()
	}()

	// Wait for all checks in a separate goroutine
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(checksChan)
		close(done)
	}()

	// Wait for either context cancellation or all checks to complete
	select {
	case <-ctx.Done():
		check := HealthCheck{
			Name:    "system",
			Status:  StatusUnhealthy,
			Error:   "health check timeout",
			Details: map[string]string{"error": ctx.Err().Error()},
		}
		checks = append(checks, check)
		response.Status = StatusUnhealthy
	case <-done:
		// Collect all results
		for check := range checksChan {
			checks = append(checks, check)
			if check.Status == StatusUnhealthy {
				response.Status = StatusUnhealthy
			} else if check.Status == StatusDegraded && response.Status != StatusUnhealthy {
				response.Status = StatusDegraded
			}
		}
	}

	response.Checks = checks
	return response
}

func (h *HealthChecker) checkDatabase(ctx context.Context) HealthCheck {
	start := time.Now()
	check := HealthCheck{
		Name:    "database",
		Status:  StatusHealthy,
		Details: make(map[string]string),
	}

	if h.db == nil {
		check.Status = StatusUnhealthy
		check.Error = "database connection not initialized"
		check.Duration = time.Since(start)
		return check
	}

	// Check basic connectivity
	if err := h.db.PingContext(ctx); err != nil {
		check.Status = StatusUnhealthy
		check.Error = fmt.Sprintf("database ping failed: %v", err)
		check.Duration = time.Since(start)
		return check
	}

	// Check connection pool stats
	stats := h.db.Stats()
	check.Details["open_connections"] = fmt.Sprintf("%d", stats.OpenConnections)
	check.Details["in_use"] = fmt.Sprintf("%d", stats.InUse)
	check.Details["idle"] = fmt.Sprintf("%d", stats.Idle)
	check.Details["max_open_connections"] = fmt.Sprintf("%d", stats.MaxOpenConnections)

	// Consider it degraded if we're close to max connections
	if float64(stats.OpenConnections)/float64(stats.MaxOpenConnections) > 0.8 {
		check.Status = StatusDegraded
		check.Error = "database connection pool near capacity"
	}

	check.Duration = time.Since(start)
	return check
}

func (h *HealthChecker) checkMigrations(ctx context.Context) HealthCheck {
	start := time.Now()
	check := HealthCheck{
		Name:    "migrations",
		Status:  StatusHealthy,
		Details: make(map[string]string),
	}

	var version int64
	err := h.db.GetContext(ctx, &version, `
		SELECT COALESCE(MAX(version_id), 0)
		FROM goose_db_version
		WHERE is_applied = true
	`)
	if err != nil {
		check.Status = StatusUnhealthy
		check.Error = fmt.Sprintf("failed to get migration version: %v", err)
		check.Duration = time.Since(start)
		return check
	}

	check.Details["current_version"] = fmt.Sprintf("%d", version)
	check.Details["is_applied"] = "true"
	check.Duration = time.Since(start)
	return check
}

func (h *HealthChecker) checkMemory() HealthCheck {
	start := time.Now()
	check := HealthCheck{
		Name:    "memory",
		Status:  StatusHealthy,
		Details: make(map[string]string),
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	check.Details["alloc_mb"] = fmt.Sprintf("%.2f", float64(memStats.Alloc)/1024/1024)
	check.Details["total_alloc_mb"] = fmt.Sprintf("%.2f", float64(memStats.TotalAlloc)/1024/1024)
	check.Details["sys_mb"] = fmt.Sprintf("%.2f", float64(memStats.Sys)/1024/1024)
	check.Details["gc_cycles"] = fmt.Sprintf("%d", memStats.NumGC)

	// Consider it degraded if we're using a lot of memory
	if float64(memStats.Alloc)/float64(memStats.Sys) > 0.8 {
		check.Status = StatusDegraded
		check.Error = "high memory utilization"
	}

	check.Duration = time.Since(start)
	return check
}
