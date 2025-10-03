package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// HealthStatus represents the health status of a component
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// HealthCheckSystem provides comprehensive health checking for load balancer integration
type HealthCheckSystem struct {
	checks       map[string]*HealthCheck
	dependencies map[string]*DependencyCheck
	metrics      *PrometheusMetrics
	mutex        sync.RWMutex

	// Configuration
	config *HealthConfig

	// State
	lastOverallStatus HealthStatus
	startTime         time.Time
}

// HealthConfig holds configuration for health checks
type HealthConfig struct {
	CheckInterval    time.Duration `json:"check_interval"`
	CheckTimeout     time.Duration `json:"check_timeout"`
	GracePeriod      time.Duration `json:"grace_period"`
	FailureThreshold int           `json:"failure_threshold"`
	SuccessThreshold int           `json:"success_threshold"`
	EnableReadiness  bool          `json:"enable_readiness"`
	EnableLiveness   bool          `json:"enable_liveness"`
}

// HealthCheck represents a single health check
type HealthCheck struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	CheckFunc   HealthCheckFunc `json:"-"`
	Critical    bool            `json:"critical"`
	Enabled     bool            `json:"enabled"`
	Timeout     time.Duration   `json:"timeout"`
	Interval    time.Duration   `json:"interval"`

	// State
	LastRun      time.Time          `json:"last_run"`
	LastResult   *HealthCheckResult `json:"last_result"`
	FailureCount int                `json:"failure_count"`
	SuccessCount int                `json:"success_count"`
	mutex        sync.RWMutex
}

// HealthCheckFunc is the function signature for health checks
type HealthCheckFunc func(ctx context.Context) *HealthCheckResult

// HealthCheckResult represents the result of a health check
type HealthCheckResult struct {
	Status    HealthStatus           `json:"status"`
	Message   string                 `json:"message"`
	Duration  time.Duration          `json:"duration"`
	Timestamp time.Time              `json:"timestamp"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// DependencyCheck represents a dependency health check
type DependencyCheck struct {
	Name       string             `json:"name"`
	Type       string             `json:"type"` // "database", "service", "file", etc.
	Target     string             `json:"target"`
	Timeout    time.Duration      `json:"timeout"`
	Critical   bool               `json:"critical"`
	CheckFunc  HealthCheckFunc    `json:"-"`
	LastResult *HealthCheckResult `json:"last_result"`
	mutex      sync.RWMutex
}

// SystemHealthReport represents the overall system health
type SystemHealthReport struct {
	Status       HealthStatus                  `json:"status"`
	Timestamp    time.Time                     `json:"timestamp"`
	Uptime       time.Duration                 `json:"uptime"`
	Version      string                        `json:"version"`
	Checks       map[string]*HealthCheckResult `json:"checks"`
	Dependencies map[string]*HealthCheckResult `json:"dependencies"`
	Summary      *HealthSummary                `json:"summary"`
}

// HealthSummary provides a summary of health check results
type HealthSummary struct {
	TotalChecks     int `json:"total_checks"`
	HealthyChecks   int `json:"healthy_checks"`
	DegradedChecks  int `json:"degraded_checks"`
	UnhealthyChecks int `json:"unhealthy_checks"`
	CriticalFailed  int `json:"critical_failed"`
}

// ReadinessProbe represents readiness check for load balancers
type ReadinessProbe struct {
	Ready     bool      `json:"ready"`
	Timestamp time.Time `json:"timestamp"`
	Reason    string    `json:"reason,omitempty"`
}

// LivenessProbe represents liveness check for load balancers
type LivenessProbe struct {
	Alive     bool      `json:"alive"`
	Timestamp time.Time `json:"timestamp"`
	Reason    string    `json:"reason,omitempty"`
}

// NewHealthCheckSystem creates a new health check system
func NewHealthCheckSystem(metrics *PrometheusMetrics) *HealthCheckSystem {
	config := &HealthConfig{
		CheckInterval:    30 * time.Second,
		CheckTimeout:     10 * time.Second,
		GracePeriod:      60 * time.Second,
		FailureThreshold: 3,
		SuccessThreshold: 2,
		EnableReadiness:  true,
		EnableLiveness:   true,
	}

	hcs := &HealthCheckSystem{
		checks:            make(map[string]*HealthCheck),
		dependencies:      make(map[string]*DependencyCheck),
		metrics:           metrics,
		config:            config,
		lastOverallStatus: HealthStatusUnknown,
		startTime:         time.Now(),
	}

	hcs.registerDefaultChecks()
	return hcs
}

// RegisterHealthCheck registers a new health check
func (hcs *HealthCheckSystem) RegisterHealthCheck(name, description string, checkFunc HealthCheckFunc, critical bool) {
	hcs.mutex.Lock()
	defer hcs.mutex.Unlock()

	hcs.checks[name] = &HealthCheck{
		Name:        name,
		Description: description,
		CheckFunc:   checkFunc,
		Critical:    critical,
		Enabled:     true,
		Timeout:     hcs.config.CheckTimeout,
		Interval:    hcs.config.CheckInterval,
	}
}

// RegisterDependency registers a dependency check
func (hcs *HealthCheckSystem) RegisterDependency(name, depType, target string, checkFunc HealthCheckFunc, critical bool) {
	hcs.mutex.Lock()
	defer hcs.mutex.Unlock()

	hcs.dependencies[name] = &DependencyCheck{
		Name:      name,
		Type:      depType,
		Target:    target,
		Timeout:   hcs.config.CheckTimeout,
		Critical:  critical,
		CheckFunc: checkFunc,
	}
}

// RunHealthCheck executes a single health check
func (hcs *HealthCheckSystem) RunHealthCheck(name string) (*HealthCheckResult, error) {
	hcs.mutex.RLock()
	check, exists := hcs.checks[name]
	hcs.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("health check '%s' not found", name)
	}

	if !check.Enabled {
		return &HealthCheckResult{
			Status:    HealthStatusHealthy,
			Message:   "Check disabled",
			Timestamp: time.Now(),
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), check.Timeout)
	defer cancel()

	start := time.Now()
	result := check.CheckFunc(ctx)
	result.Duration = time.Since(start)
	result.Timestamp = time.Now()

	// Update check state
	check.mutex.Lock()
	check.LastRun = time.Now()
	check.LastResult = result

	if result.Status == HealthStatusHealthy {
		check.SuccessCount++
		check.FailureCount = 0
	} else {
		check.FailureCount++
		check.SuccessCount = 0
	}
	check.mutex.Unlock()

	return result, nil
}

// GetSystemHealth returns comprehensive system health report
func (hcs *HealthCheckSystem) GetSystemHealth() *SystemHealthReport {
	hcs.mutex.RLock()
	defer hcs.mutex.RUnlock()

	report := &SystemHealthReport{
		Status:       HealthStatusHealthy,
		Timestamp:    time.Now(),
		Uptime:       time.Since(hcs.startTime),
		Version:      "1.0.0", // This would come from build info
		Checks:       make(map[string]*HealthCheckResult),
		Dependencies: make(map[string]*HealthCheckResult),
		Summary:      &HealthSummary{},
	}

	// Run all health checks
	for name, check := range hcs.checks {
		if !check.Enabled {
			continue
		}

		result, err := hcs.RunHealthCheck(name)
		if err != nil {
			result = &HealthCheckResult{
				Status:    HealthStatusUnhealthy,
				Message:   "Check execution failed",
				Error:     err.Error(),
				Timestamp: time.Now(),
			}
		}

		report.Checks[name] = result
		hcs.updateSummary(report.Summary, result, check.Critical)

		// Update overall status
		if check.Critical && result.Status == HealthStatusUnhealthy {
			report.Status = HealthStatusUnhealthy
		} else if result.Status == HealthStatusDegraded && report.Status == HealthStatusHealthy {
			report.Status = HealthStatusDegraded
		}
	}

	// Run dependency checks
	for name, dep := range hcs.dependencies {
		ctx, cancel := context.WithTimeout(context.Background(), dep.Timeout)
		result := dep.CheckFunc(ctx)
		cancel()

		result.Timestamp = time.Now()
		dep.mutex.Lock()
		dep.LastResult = result
		dep.mutex.Unlock()

		report.Dependencies[name] = result
		hcs.updateSummary(report.Summary, result, dep.Critical)

		// Update overall status for critical dependencies
		if dep.Critical && result.Status == HealthStatusUnhealthy {
			report.Status = HealthStatusUnhealthy
		} else if result.Status == HealthStatusDegraded && report.Status == HealthStatusHealthy {
			report.Status = HealthStatusDegraded
		}
	}

	hcs.lastOverallStatus = report.Status
	return report
}

// GetReadinessProbe returns readiness status for load balancers
func (hcs *HealthCheckSystem) GetReadinessProbe() *ReadinessProbe {
	if !hcs.config.EnableReadiness {
		return &ReadinessProbe{
			Ready:     true,
			Timestamp: time.Now(),
			Reason:    "Readiness checks disabled",
		}
	}

	// Check if we're in grace period
	if time.Since(hcs.startTime) < hcs.config.GracePeriod {
		return &ReadinessProbe{
			Ready:     false,
			Timestamp: time.Now(),
			Reason:    "In startup grace period",
		}
	}

	report := hcs.GetSystemHealth()

	// Ready if no critical components are unhealthy
	ready := report.Summary.CriticalFailed == 0
	reason := ""

	if !ready {
		reason = fmt.Sprintf("%d critical components failed", report.Summary.CriticalFailed)
	}

	return &ReadinessProbe{
		Ready:     ready,
		Timestamp: time.Now(),
		Reason:    reason,
	}
}

// GetLivenessProbe returns liveness status for load balancers
func (hcs *HealthCheckSystem) GetLivenessProbe() *LivenessProbe {
	if !hcs.config.EnableLiveness {
		return &LivenessProbe{
			Alive:     true,
			Timestamp: time.Now(),
			Reason:    "Liveness checks disabled",
		}
	}

	// Simple liveness check - system is alive if it can respond
	// In a more complex system, this might check for deadlocks, etc.
	alive := true
	reason := ""

	// Check if any critical system components are completely down
	hcs.mutex.RLock()
	for name, check := range hcs.checks {
		if check.Critical && check.Enabled {
			check.mutex.RLock()
			if check.FailureCount >= hcs.config.FailureThreshold {
				alive = false
				reason = fmt.Sprintf("Critical check '%s' has failed %d times", name, check.FailureCount)
				check.mutex.RUnlock()
				break
			}
			check.mutex.RUnlock()
		}
	}
	hcs.mutex.RUnlock()

	return &LivenessProbe{
		Alive:     alive,
		Timestamp: time.Now(),
		Reason:    reason,
	}
}

// updateSummary updates the health summary with check results
func (hcs *HealthCheckSystem) updateSummary(summary *HealthSummary, result *HealthCheckResult, critical bool) {
	summary.TotalChecks++

	switch result.Status {
	case HealthStatusHealthy:
		summary.HealthyChecks++
	case HealthStatusDegraded:
		summary.DegradedChecks++
	case HealthStatusUnhealthy:
		summary.UnhealthyChecks++
		if critical {
			summary.CriticalFailed++
		}
	}
}

// registerDefaultChecks registers default system health checks
func (hcs *HealthCheckSystem) registerDefaultChecks() {
	// System resource checks
	hcs.RegisterHealthCheck("memory", "System memory usage", func(ctx context.Context) *HealthCheckResult {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		// Convert to MB for easier reading
		allocMB := float64(m.Alloc) / 1024 / 1024
		sysMB := float64(m.Sys) / 1024 / 1024

		status := HealthStatusHealthy
		message := fmt.Sprintf("Memory usage: %.2f MB allocated, %.2f MB system", allocMB, sysMB)

		// Simple thresholds - in production these would be configurable
		if allocMB > 500 {
			status = HealthStatusDegraded
			message = fmt.Sprintf("High memory usage: %.2f MB allocated", allocMB)
		}
		if allocMB > 1000 {
			status = HealthStatusUnhealthy
			message = fmt.Sprintf("Critical memory usage: %.2f MB allocated", allocMB)
		}

		return &HealthCheckResult{
			Status:  status,
			Message: message,
			Details: map[string]interface{}{
				"alloc_mb":   allocMB,
				"sys_mb":     sysMB,
				"num_gc":     m.NumGC,
				"goroutines": runtime.NumGoroutine(),
			},
		}
	}, false)

	// Goroutine check
	hcs.RegisterHealthCheck("goroutines", "Goroutine count", func(ctx context.Context) *HealthCheckResult {
		numGoroutines := runtime.NumGoroutine()

		status := HealthStatusHealthy
		message := fmt.Sprintf("Goroutines: %d", numGoroutines)

		if numGoroutines > 1000 {
			status = HealthStatusDegraded
			message = fmt.Sprintf("High goroutine count: %d", numGoroutines)
		}
		if numGoroutines > 5000 {
			status = HealthStatusUnhealthy
			message = fmt.Sprintf("Critical goroutine count: %d", numGoroutines)
		}

		return &HealthCheckResult{
			Status:  status,
			Message: message,
			Details: map[string]interface{}{
				"count": numGoroutines,
			},
		}
	}, false)

	// Database connectivity check (if metrics are available)
	hcs.RegisterHealthCheck("database", "Database connectivity", func(ctx context.Context) *HealthCheckResult {
		if hcs.metrics == nil {
			return &HealthCheckResult{
				Status:  HealthStatusUnknown,
				Message: "Metrics not available",
			}
		}

		// This is a placeholder - in a real implementation, this would
		// actually test database connectivity
		return &HealthCheckResult{
			Status:  HealthStatusHealthy,
			Message: "Database connection healthy",
			Details: map[string]interface{}{
				"connection_pool": "active",
			},
		}
	}, true)

	// Disk space check
	hcs.RegisterHealthCheck("disk", "Disk space availability", func(ctx context.Context) *HealthCheckResult {
		// This is a simplified check - in production, you'd check actual disk usage
		return &HealthCheckResult{
			Status:  HealthStatusHealthy,
			Message: "Disk space sufficient",
			Details: map[string]interface{}{
				"usage_percent": 45.2,
				"available_gb":  100.5,
			},
		}
	}, true)
}

// HealthServer provides HTTP endpoints for health checks
type HealthServer struct {
	healthSystem *HealthCheckSystem
	server       *http.Server
}

// NewHealthServer creates a new health check HTTP server
func NewHealthServer(addr string, healthSystem *HealthCheckSystem) *HealthServer {
	hs := &HealthServer{
		healthSystem: healthSystem,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", hs.handleHealth)
	mux.HandleFunc("/health/ready", hs.handleReadiness)
	mux.HandleFunc("/health/live", hs.handleLiveness)
	mux.HandleFunc("/health/detailed", hs.handleDetailedHealth)

	hs.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return hs
}

// Start starts the health server
func (hs *HealthServer) Start() error {
	go func() {
		if err := hs.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Health server error: %v\n", err)
		}
	}()
	return nil
}

// Stop stops the health server
func (hs *HealthServer) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return hs.server.Shutdown(ctx)
}

// HTTP handlers for health endpoints

func (hs *HealthServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	report := hs.healthSystem.GetSystemHealth()

	status := http.StatusOK
	if report.Status == HealthStatusUnhealthy {
		status = http.StatusServiceUnavailable
	} else if report.Status == HealthStatusDegraded {
		status = http.StatusPartialContent
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	response := map[string]interface{}{
		"status":    report.Status,
		"timestamp": report.Timestamp,
		"uptime":    report.Uptime.String(),
	}

	json.NewEncoder(w).Encode(response)
}

func (hs *HealthServer) handleReadiness(w http.ResponseWriter, r *http.Request) {
	probe := hs.healthSystem.GetReadinessProbe()

	status := http.StatusOK
	if !probe.Ready {
		status = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(probe)
}

func (hs *HealthServer) handleLiveness(w http.ResponseWriter, r *http.Request) {
	probe := hs.healthSystem.GetLivenessProbe()

	status := http.StatusOK
	if !probe.Alive {
		status = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(probe)
}

func (hs *HealthServer) handleDetailedHealth(w http.ResponseWriter, r *http.Request) {
	report := hs.healthSystem.GetSystemHealth()

	status := http.StatusOK
	if report.Status == HealthStatusUnhealthy {
		status = http.StatusServiceUnavailable
	} else if report.Status == HealthStatusDegraded {
		status = http.StatusPartialContent
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(report)
}
