package health

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Status represents the health status of a component
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDegraded  Status = "degraded"
	StatusUnknown   Status = "unknown"
)

// Check represents a health check
type Check interface {
	Name() string
	Check(ctx context.Context) CheckResult
}

// CheckResult represents the result of a health check
type CheckResult struct {
	Status    Status                 `json:"status"`
	Message   string                 `json:"message,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Duration  time.Duration          `json:"duration"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// HealthChecker manages health checks
type HealthChecker struct {
	checks   map[string]Check
	results  map[string]CheckResult
	mutex    sync.RWMutex
	interval time.Duration
	timeout  time.Duration
	enabled  bool
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(interval, timeout time.Duration, enabled bool) *HealthChecker {
	return &HealthChecker{
		checks:   make(map[string]Check),
		results:  make(map[string]CheckResult),
		interval: interval,
		timeout:  timeout,
		enabled:  enabled,
		stopCh:   make(chan struct{}),
	}
}

// RegisterCheck registers a health check
func (hc *HealthChecker) RegisterCheck(check Check) {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()
	hc.checks[check.Name()] = check
}

// UnregisterCheck unregisters a health check
func (hc *HealthChecker) UnregisterCheck(name string) {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()
	delete(hc.checks, name)
	delete(hc.results, name)
}

// Start starts the health checker
func (hc *HealthChecker) Start(ctx context.Context) {
	if !hc.enabled {
		return
	}

	hc.wg.Add(1)
	go hc.run(ctx)
}

// Stop stops the health checker
func (hc *HealthChecker) Stop() {
	if !hc.enabled {
		return
	}

	close(hc.stopCh)
	hc.wg.Wait()
}

// GetStatus returns the overall health status
func (hc *HealthChecker) GetStatus() Status {
	hc.mutex.RLock()
	defer hc.mutex.RUnlock()

	if len(hc.results) == 0 {
		return StatusUnknown
	}

	hasUnhealthy := false
	hasDegraded := false

	for _, result := range hc.results {
		switch result.Status {
		case StatusUnhealthy:
			hasUnhealthy = true
		case StatusDegraded:
			hasDegraded = true
		}
	}

	if hasUnhealthy {
		return StatusUnhealthy
	}
	if hasDegraded {
		return StatusDegraded
	}
	return StatusHealthy
}

// GetResults returns all health check results
func (hc *HealthChecker) GetResults() map[string]CheckResult {
	hc.mutex.RLock()
	defer hc.mutex.RUnlock()

	results := make(map[string]CheckResult)
	for name, result := range hc.results {
		results[name] = result
	}
	return results
}

// GetResult returns a specific health check result
func (hc *HealthChecker) GetResult(name string) (CheckResult, bool) {
	hc.mutex.RLock()
	defer hc.mutex.RUnlock()

	result, exists := hc.results[name]
	return result, exists
}

// RunCheck runs a specific health check
func (hc *HealthChecker) RunCheck(ctx context.Context, name string) (CheckResult, error) {
	hc.mutex.RLock()
	check, exists := hc.checks[name]
	hc.mutex.RUnlock()

	if !exists {
		return CheckResult{}, fmt.Errorf("health check %s not found", name)
	}

	return hc.executeCheck(ctx, check), nil
}

// RunAllChecks runs all registered health checks
func (hc *HealthChecker) RunAllChecks(ctx context.Context) map[string]CheckResult {
	hc.mutex.RLock()
	checks := make(map[string]Check)
	for name, check := range hc.checks {
		checks[name] = check
	}
	hc.mutex.RUnlock()

	results := make(map[string]CheckResult)
	for name, check := range checks {
		results[name] = hc.executeCheck(ctx, check)
	}

	hc.mutex.Lock()
	for name, result := range results {
		hc.results[name] = result
	}
	hc.mutex.Unlock()

	return results
}

// run runs the health checker loop
func (hc *HealthChecker) run(ctx context.Context) {
	defer hc.wg.Done()

	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	// Run initial checks
	hc.RunAllChecks(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-hc.stopCh:
			return
		case <-ticker.C:
			hc.RunAllChecks(ctx)
		}
	}
}

// executeCheck executes a single health check with timeout
func (hc *HealthChecker) executeCheck(ctx context.Context, check Check) CheckResult {
	start := time.Now()

	// Create context with timeout
	checkCtx, cancel := context.WithTimeout(ctx, hc.timeout)
	defer cancel()

	// Run check in goroutine to handle timeout
	resultCh := make(chan CheckResult, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				resultCh <- CheckResult{
					Status:    StatusUnhealthy,
					Error:     fmt.Sprintf("panic in health check: %v", r),
					Duration:  time.Since(start),
					Timestamp: time.Now(),
				}
			}
		}()

		result := check.Check(checkCtx)
		result.Duration = time.Since(start)
		result.Timestamp = time.Now()
		resultCh <- result
	}()

	select {
	case result := <-resultCh:
		return result
	case <-checkCtx.Done():
		return CheckResult{
			Status:    StatusUnhealthy,
			Error:     "health check timeout",
			Duration:  time.Since(start),
			Timestamp: time.Now(),
		}
	}
}

// DatabaseCheck implements a database health check
type DatabaseCheck struct {
	name string
	db   DatabaseHealthChecker
}

// DatabaseHealthChecker interface for database health checking
type DatabaseHealthChecker interface {
	HealthCheck(ctx context.Context) error
}

// NewDatabaseCheck creates a new database health check
func NewDatabaseCheck(name string, db DatabaseHealthChecker) *DatabaseCheck {
	return &DatabaseCheck{
		name: name,
		db:   db,
	}
}

// Name returns the check name
func (dc *DatabaseCheck) Name() string {
	return dc.name
}

// Check performs the database health check
func (dc *DatabaseCheck) Check(ctx context.Context) CheckResult {
	err := dc.db.HealthCheck(ctx)
	if err != nil {
		return CheckResult{
			Status:  StatusUnhealthy,
			Message: "Database health check failed",
			Error:   err.Error(),
		}
	}

	return CheckResult{
		Status:  StatusHealthy,
		Message: "Database is healthy",
	}
}

// MemoryCheck implements a memory usage health check
type MemoryCheck struct {
	name      string
	threshold float64 // percentage threshold (0-100)
}

// NewMemoryCheck creates a new memory health check
func NewMemoryCheck(name string, threshold float64) *MemoryCheck {
	return &MemoryCheck{
		name:      name,
		threshold: threshold,
	}
}

// Name returns the check name
func (mc *MemoryCheck) Name() string {
	return mc.name
}

// Check performs the memory health check
func (mc *MemoryCheck) Check(ctx context.Context) CheckResult {
	// This is a simplified implementation
	// In a real implementation, you would check actual memory usage
	memUsage := 45.0 // Mock memory usage percentage

	metadata := map[string]interface{}{
		"memory_usage_percent": memUsage,
		"threshold_percent":    mc.threshold,
	}

	if memUsage > mc.threshold {
		return CheckResult{
			Status:   StatusDegraded,
			Message:  fmt.Sprintf("Memory usage is high: %.1f%%", memUsage),
			Metadata: metadata,
		}
	}

	return CheckResult{
		Status:   StatusHealthy,
		Message:  fmt.Sprintf("Memory usage is normal: %.1f%%", memUsage),
		Metadata: metadata,
	}
}

// DiskCheck implements a disk space health check
type DiskCheck struct {
	name      string
	path      string
	threshold float64 // percentage threshold (0-100)
}

// NewDiskCheck creates a new disk health check
func NewDiskCheck(name, path string, threshold float64) *DiskCheck {
	return &DiskCheck{
		name:      name,
		path:      path,
		threshold: threshold,
	}
}

// Name returns the check name
func (dc *DiskCheck) Name() string {
	return dc.name
}

// Check performs the disk health check
func (dc *DiskCheck) Check(ctx context.Context) CheckResult {
	// This is a simplified implementation
	// In a real implementation, you would check actual disk usage
	diskUsage := 35.0 // Mock disk usage percentage

	metadata := map[string]interface{}{
		"disk_usage_percent": diskUsage,
		"threshold_percent":  dc.threshold,
		"path":               dc.path,
	}

	if diskUsage > dc.threshold {
		return CheckResult{
			Status:   StatusDegraded,
			Message:  fmt.Sprintf("Disk usage is high: %.1f%%", diskUsage),
			Metadata: metadata,
		}
	}

	return CheckResult{
		Status:   StatusHealthy,
		Message:  fmt.Sprintf("Disk usage is normal: %.1f%%", diskUsage),
		Metadata: metadata,
	}
}
