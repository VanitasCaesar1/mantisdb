package monitoring

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// HealthChecker performs system health checks
type HealthChecker struct {
	checks map[string]HealthCheck
	mutex  sync.RWMutex

	// Dependencies
	aggregator *MetricsAggregator
	alerter    *Alerter

	// Configuration
	checkInterval time.Duration
	timeout       time.Duration

	// Control
	stopChan chan struct{}
	doneChan chan struct{}
}

// HealthCheck represents a single health check
type HealthCheck struct {
	Name        string
	Description string
	CheckFunc   HealthCheckFunc
	Timeout     time.Duration
	Critical    bool
	Enabled     bool
	LastRun     time.Time
	LastResult  HealthCheckResult
}

// HealthCheckFunc is the function signature for health checks
type HealthCheckFunc func(ctx context.Context) HealthCheckResult

// HealthCheckResult represents the result of a health check
type HealthCheckResult struct {
	Status    HealthStatusType       `json:"status"`
	Message   string                 `json:"message"`
	Duration  time.Duration          `json:"duration"`
	Timestamp time.Time              `json:"timestamp"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// SystemHealthReport represents the overall system health
type SystemHealthReport struct {
	Overall   HealthStatusType             `json:"overall"`
	Checks    map[string]HealthCheckResult `json:"checks"`
	Timestamp time.Time                    `json:"timestamp"`
	Duration  time.Duration                `json:"duration"`
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(aggregator *MetricsAggregator, alerter *Alerter) *HealthChecker {
	hc := &HealthChecker{
		checks:        make(map[string]HealthCheck),
		aggregator:    aggregator,
		alerter:       alerter,
		checkInterval: 30 * time.Second,
		timeout:       10 * time.Second,
		stopChan:      make(chan struct{}),
		doneChan:      make(chan struct{}),
	}

	// Register default health checks
	hc.registerDefaultChecks()

	return hc
}

// RegisterCheck registers a new health check
func (hc *HealthChecker) RegisterCheck(name, description string, checkFunc HealthCheckFunc, critical bool) {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()

	hc.checks[name] = HealthCheck{
		Name:        name,
		Description: description,
		CheckFunc:   checkFunc,
		Timeout:     hc.timeout,
		Critical:    critical,
		Enabled:     true,
	}
}

// EnableCheck enables a health check
func (hc *HealthChecker) EnableCheck(name string) {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()

	if check, exists := hc.checks[name]; exists {
		check.Enabled = true
		hc.checks[name] = check
	}
}

// DisableCheck disables a health check
func (hc *HealthChecker) DisableCheck(name string) {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()

	if check, exists := hc.checks[name]; exists {
		check.Enabled = false
		hc.checks[name] = check
	}
}

// Start begins health checking
func (hc *HealthChecker) Start(ctx context.Context) {
	go hc.runHealthChecks(ctx)
}

// Stop stops health checking
func (hc *HealthChecker) Stop() {
	close(hc.stopChan)
	<-hc.doneChan
}

// RunHealthCheck runs a single health check
func (hc *HealthChecker) RunHealthCheck(name string) (HealthCheckResult, error) {
	hc.mutex.RLock()
	check, exists := hc.checks[name]
	hc.mutex.RUnlock()

	if !exists {
		return HealthCheckResult{}, fmt.Errorf("health check '%s' not found", name)
	}

	if !check.Enabled {
		return HealthCheckResult{
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

	// Update the check result
	hc.mutex.Lock()
	check.LastRun = time.Now()
	check.LastResult = result
	hc.checks[name] = check
	hc.mutex.Unlock()

	return result, nil
}

// GetSystemHealth returns the overall system health
func (hc *HealthChecker) GetSystemHealth() SystemHealthReport {
	start := time.Now()

	hc.mutex.RLock()
	checks := make(map[string]HealthCheck)
	for k, v := range hc.checks {
		checks[k] = v
	}
	hc.mutex.RUnlock()

	report := SystemHealthReport{
		Overall:   HealthStatusHealthy,
		Checks:    make(map[string]HealthCheckResult),
		Timestamp: time.Now(),
	}

	// Run all enabled checks
	for name, check := range checks {
		if !check.Enabled {
			continue
		}

		result, err := hc.RunHealthCheck(name)
		if err != nil {
			result = HealthCheckResult{
				Status:    HealthStatusUnhealthy,
				Message:   fmt.Sprintf("Check failed: %v", err),
				Timestamp: time.Now(),
			}
		}

		report.Checks[name] = result

		// Update overall status
		if check.Critical && result.Status == HealthStatusUnhealthy {
			report.Overall = HealthStatusUnhealthy
		} else if result.Status == HealthStatusDegraded && report.Overall == HealthStatusHealthy {
			report.Overall = HealthStatusDegraded
		}
	}

	report.Duration = time.Since(start)
	return report
}

// runHealthChecks runs the health check loop
func (hc *HealthChecker) runHealthChecks(ctx context.Context) {
	defer close(hc.doneChan)

	ticker := time.NewTicker(hc.checkInterval)
	defer ticker.Stop()

	// Run initial health check
	hc.performHealthChecks()

	for {
		select {
		case <-ctx.Done():
			return
		case <-hc.stopChan:
			return
		case <-ticker.C:
			hc.performHealthChecks()
		}
	}
}

// performHealthChecks performs all health checks and handles alerting
func (hc *HealthChecker) performHealthChecks() {
	report := hc.GetSystemHealth()

	// Check for alerts
	for name, result := range report.Checks {
		hc.mutex.RLock()
		check := hc.checks[name]
		hc.mutex.RUnlock()

		// Send alerts for critical checks
		if check.Critical {
			switch result.Status {
			case HealthStatusUnhealthy:
				hc.alerter.SendAlert(Alert{
					Level:     AlertLevelCritical,
					Component: name,
					Message:   fmt.Sprintf("Critical health check failed: %s", result.Message),
					Timestamp: result.Timestamp,
					Details:   result.Details,
				})
			case HealthStatusDegraded:
				hc.alerter.SendAlert(Alert{
					Level:     AlertLevelWarning,
					Component: name,
					Message:   fmt.Sprintf("Health check degraded: %s", result.Message),
					Timestamp: result.Timestamp,
					Details:   result.Details,
				})
			}
		}
	}

	// Send overall system alert if unhealthy
	if report.Overall == HealthStatusUnhealthy {
		hc.alerter.SendAlert(Alert{
			Level:     AlertLevelCritical,
			Component: "system",
			Message:   "System health is unhealthy",
			Timestamp: report.Timestamp,
			Details: map[string]interface{}{
				"failed_checks": hc.getFailedChecks(report),
			},
		})
	}
}

// getFailedChecks returns a list of failed checks
func (hc *HealthChecker) getFailedChecks(report SystemHealthReport) []string {
	var failed []string
	for name, result := range report.Checks {
		if result.Status != HealthStatusHealthy {
			failed = append(failed, name)
		}
	}
	return failed
}

// registerDefaultChecks registers the default system health checks
func (hc *HealthChecker) registerDefaultChecks() {
	// WAL Health Check
	hc.RegisterCheck("wal", "Write-Ahead Log health", func(ctx context.Context) HealthCheckResult {
		if hc.aggregator == nil {
			return HealthCheckResult{
				Status:  HealthStatusUnhealthy,
				Message: "Metrics aggregator not available",
			}
		}

		summary := hc.aggregator.GetCollector().GetSummaryMetrics()
		walWrites := summary["wal_writes_total"]
		walErrors := summary["wal_write_errors_total"]

		if walWrites == 0 {
			return HealthCheckResult{
				Status:  HealthStatusHealthy,
				Message: "No WAL activity",
			}
		}

		errorRate := float64(walErrors) / float64(walWrites)
		if errorRate > 0.1 {
			return HealthCheckResult{
				Status:  HealthStatusUnhealthy,
				Message: fmt.Sprintf("High WAL error rate: %.2f%%", errorRate*100),
				Details: map[string]interface{}{
					"error_rate":   errorRate,
					"total_writes": walWrites,
					"total_errors": walErrors,
				},
			}
		} else if errorRate > 0.05 {
			return HealthCheckResult{
				Status:  HealthStatusDegraded,
				Message: fmt.Sprintf("Elevated WAL error rate: %.2f%%", errorRate*100),
				Details: map[string]interface{}{
					"error_rate":   errorRate,
					"total_writes": walWrites,
					"total_errors": walErrors,
				},
			}
		}

		return HealthCheckResult{
			Status:  HealthStatusHealthy,
			Message: "WAL operating normally",
			Details: map[string]interface{}{
				"error_rate":   errorRate,
				"total_writes": walWrites,
			},
		}
	}, true)

	// Transaction Health Check
	hc.RegisterCheck("transactions", "Transaction system health", func(ctx context.Context) HealthCheckResult {
		if hc.aggregator == nil {
			return HealthCheckResult{
				Status:  HealthStatusUnhealthy,
				Message: "Metrics aggregator not available",
			}
		}

		summary := hc.aggregator.GetCollector().GetSummaryMetrics()
		txnStarted := summary["transactions_started_total"]
		txnAborted := summary["transactions_aborted_total"]
		deadlocks := summary["transaction_deadlocks_total"]

		if txnStarted == 0 {
			return HealthCheckResult{
				Status:  HealthStatusHealthy,
				Message: "No transaction activity",
			}
		}

		abortRate := float64(txnAborted) / float64(txnStarted)

		if deadlocks > 10 || abortRate > 0.2 {
			return HealthCheckResult{
				Status:  HealthStatusDegraded,
				Message: fmt.Sprintf("High transaction issues: %.2f%% abort rate, %d deadlocks", abortRate*100, deadlocks),
				Details: map[string]interface{}{
					"abort_rate": abortRate,
					"deadlocks":  deadlocks,
					"started":    txnStarted,
					"aborted":    txnAborted,
				},
			}
		}

		return HealthCheckResult{
			Status:  HealthStatusHealthy,
			Message: "Transactions operating normally",
			Details: map[string]interface{}{
				"abort_rate": abortRate,
				"deadlocks":  deadlocks,
			},
		}
	}, true)

	// Memory Health Check
	hc.RegisterCheck("memory", "Memory usage health", func(ctx context.Context) HealthCheckResult {
		if hc.aggregator == nil {
			return HealthCheckResult{
				Status:  HealthStatusUnhealthy,
				Message: "Metrics aggregator not available",
			}
		}

		summary := hc.aggregator.GetCollector().GetSummaryMetrics()
		memoryUsage := summary["memory_usage_bytes"]

		// Simple memory check - in production this would be more sophisticated
		const maxMemory = 1024 * 1024 * 1024 // 1GB threshold

		if memoryUsage > maxMemory {
			return HealthCheckResult{
				Status:  HealthStatusDegraded,
				Message: fmt.Sprintf("High memory usage: %d bytes", memoryUsage),
				Details: map[string]interface{}{
					"memory_usage_bytes": memoryUsage,
					"threshold_bytes":    maxMemory,
				},
			}
		}

		return HealthCheckResult{
			Status:  HealthStatusHealthy,
			Message: "Memory usage normal",
			Details: map[string]interface{}{
				"memory_usage_bytes": memoryUsage,
			},
		}
	}, false)

	// Corruption Health Check
	hc.RegisterCheck("corruption", "Data corruption health", func(ctx context.Context) HealthCheckResult {
		if hc.aggregator == nil {
			return HealthCheckResult{
				Status:  HealthStatusUnhealthy,
				Message: "Metrics aggregator not available",
			}
		}

		summary := hc.aggregator.GetCollector().GetSummaryMetrics()
		corruptionEvents := summary["corruption_events_total"]

		if corruptionEvents > 0 {
			return HealthCheckResult{
				Status:  HealthStatusUnhealthy,
				Message: fmt.Sprintf("Data corruption detected: %d events", corruptionEvents),
				Details: map[string]interface{}{
					"corruption_events": corruptionEvents,
				},
			}
		}

		return HealthCheckResult{
			Status:  HealthStatusHealthy,
			Message: "No corruption detected",
		}
	}, true)
}

// SetCheckInterval sets the health check interval
func (hc *HealthChecker) SetCheckInterval(interval time.Duration) {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()
	hc.checkInterval = interval
}

// SetTimeout sets the default timeout for health checks
func (hc *HealthChecker) SetTimeout(timeout time.Duration) {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()
	hc.timeout = timeout
}
