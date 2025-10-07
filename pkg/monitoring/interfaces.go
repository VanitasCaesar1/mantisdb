// Package monitoring provides public interfaces for MantisDB monitoring components
package monitoring

import (
	"context"
	"time"
)

// MetricsCollector defines the interface for collecting metrics
type MetricsCollector interface {
	// Counter metrics
	IncrementCounter(name string, labels map[string]string, value float64)

	// Gauge metrics
	SetGauge(name string, labels map[string]string, value float64)

	// Histogram metrics
	RecordHistogram(name string, labels map[string]string, value float64)

	// Summary metrics
	RecordSummary(name string, labels map[string]string, value float64)

	// Export metrics
	Export(ctx context.Context) ([]byte, error)
}

// HealthChecker defines the interface for health checking
type HealthChecker interface {
	Check(ctx context.Context) HealthStatus
	RegisterCheck(name string, check HealthCheck)
	UnregisterCheck(name string)
}

// HealthCheck defines a single health check function
type HealthCheck func(ctx context.Context) error

// HealthStatus represents the overall health status
type HealthStatus struct {
	Status    Status
	Timestamp time.Time
	Checks    map[string]CheckResult
}

// CheckResult represents the result of a single health check
type CheckResult struct {
	Status   Status
	Message  string
	Duration time.Duration
	Error    error
}

// Status represents health status
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDegraded  Status = "degraded"
)

// Logger defines the interface for structured logging
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	Fatal(msg string, fields ...Field)

	With(fields ...Field) Logger
	WithContext(ctx context.Context) Logger
}

// Field represents a structured log field
type Field struct {
	Key   string
	Value interface{}
}

// Alerter defines the interface for alerting
type Alerter interface {
	SendAlert(ctx context.Context, alert Alert) error
	RegisterRule(rule AlertRule) error
	UnregisterRule(ruleID string) error
}

// Alert represents an alert to be sent
type Alert struct {
	ID          string
	Title       string
	Description string
	Severity    Severity
	Timestamp   time.Time
	Labels      map[string]string
}

// AlertRule defines conditions for triggering alerts
type AlertRule struct {
	ID        string
	Name      string
	Condition string
	Severity  Severity
	Cooldown  time.Duration
}

// Severity represents alert severity
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)
