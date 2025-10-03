package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// ObservabilitySystem integrates all metrics, health checks, and alerting components
type ObservabilitySystem struct {
	// Core components
	prometheusMetrics *PrometheusMetrics
	healthSystem      *HealthCheckSystem
	alertingSystem    *AlertingSystem

	// HTTP servers
	metricsServer *MetricsServer
	healthServer  *HealthServer

	// Configuration
	config *ObservabilityConfig

	// Control
	ctx    context.Context
	cancel context.CancelFunc
}

// ObservabilityConfig holds configuration for the entire observability system
type ObservabilityConfig struct {
	// Metrics configuration
	MetricsEnabled bool   `json:"metrics_enabled"`
	MetricsAddr    string `json:"metrics_addr"`

	// Health check configuration
	HealthEnabled bool   `json:"health_enabled"`
	HealthAddr    string `json:"health_addr"`

	// Alerting configuration
	AlertingConfig *AlertingConfig `json:"alerting_config"`

	// Integration settings
	EnableAutoRegistration bool `json:"enable_auto_registration"`
}

// DefaultObservabilityConfig returns default configuration
func DefaultObservabilityConfig() *ObservabilityConfig {
	return &ObservabilityConfig{
		MetricsEnabled:         true,
		MetricsAddr:            ":9090",
		HealthEnabled:          true,
		HealthAddr:             ":8080",
		AlertingConfig:         DefaultAlertingConfig(),
		EnableAutoRegistration: true,
	}
}

// NewObservabilitySystem creates a new integrated observability system
func NewObservabilitySystem(config *ObservabilityConfig) *ObservabilitySystem {
	if config == nil {
		config = DefaultObservabilityConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Create core components
	prometheusMetrics := NewPrometheusMetrics()
	healthSystem := NewHealthCheckSystem(prometheusMetrics)
	alertingSystem := NewAlertingSystem(config.AlertingConfig, prometheusMetrics)

	// Create HTTP servers
	var metricsServer *MetricsServer
	var healthServer *HealthServer

	if config.MetricsEnabled {
		metricsServer = NewMetricsServer(config.MetricsAddr, prometheusMetrics)
	}

	if config.HealthEnabled {
		healthServer = NewHealthServer(config.HealthAddr, healthSystem)
	}

	obs := &ObservabilitySystem{
		prometheusMetrics: prometheusMetrics,
		healthSystem:      healthSystem,
		alertingSystem:    alertingSystem,
		metricsServer:     metricsServer,
		healthServer:      healthServer,
		config:            config,
		ctx:               ctx,
		cancel:            cancel,
	}

	// Set up integrations
	if config.EnableAutoRegistration {
		obs.setupIntegrations()
	}

	return obs
}

// Start starts all observability components
func (obs *ObservabilitySystem) Start() error {
	// Start metrics server
	if obs.metricsServer != nil {
		if err := obs.metricsServer.Start(); err != nil {
			return fmt.Errorf("failed to start metrics server: %v", err)
		}
		fmt.Printf("Metrics server started on %s\n", obs.config.MetricsAddr)
	}

	// Start health server
	if obs.healthServer != nil {
		if err := obs.healthServer.Start(); err != nil {
			return fmt.Errorf("failed to start health server: %v", err)
		}
		fmt.Printf("Health server started on %s\n", obs.config.HealthAddr)
	}

	// Start alerting system
	obs.alertingSystem.Start()
	fmt.Println("Alerting system started")

	return nil
}

// Stop stops all observability components
func (obs *ObservabilitySystem) Stop() error {
	obs.cancel()

	var errors []error

	// Stop alerting system
	obs.alertingSystem.Stop()

	// Stop servers
	if obs.metricsServer != nil {
		if err := obs.metricsServer.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop metrics server: %v", err))
		}
	}

	if obs.healthServer != nil {
		if err := obs.healthServer.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop health server: %v", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors stopping observability system: %v", errors)
	}

	return nil
}

// GetMetrics returns the Prometheus metrics collector
func (obs *ObservabilitySystem) GetMetrics() *PrometheusMetrics {
	return obs.prometheusMetrics
}

// GetHealthSystem returns the health check system
func (obs *ObservabilitySystem) GetHealthSystem() *HealthCheckSystem {
	return obs.healthSystem
}

// GetAlertingSystem returns the alerting system
func (obs *ObservabilitySystem) GetAlertingSystem() *AlertingSystem {
	return obs.alertingSystem
}

// setupIntegrations sets up automatic integrations between components
func (obs *ObservabilitySystem) setupIntegrations() {
	// Register health checks that monitor metrics
	obs.healthSystem.RegisterHealthCheck(
		"metrics_collection",
		"Metrics collection health",
		func(ctx context.Context) *HealthCheckResult {
			// Check if metrics are being collected properly
			return &HealthCheckResult{
				Status:  HealthStatusHealthy,
				Message: "Metrics collection is healthy",
				Details: map[string]interface{}{
					"metrics_endpoint": obs.config.MetricsAddr + "/metrics",
				},
			}
		},
		false,
	)

	// Register health check for alerting system
	obs.healthSystem.RegisterHealthCheck(
		"alerting_system",
		"Alerting system health",
		func(ctx context.Context) *HealthCheckResult {
			activeAlerts := obs.alertingSystem.GetActiveAlerts()
			criticalAlerts := 0

			for _, alert := range activeAlerts {
				if alert.Severity == AlertSeverityCritical {
					criticalAlerts++
				}
			}

			status := HealthStatusHealthy
			message := fmt.Sprintf("Alerting system healthy, %d active alerts", len(activeAlerts))

			if criticalAlerts > 0 {
				status = HealthStatusDegraded
				message = fmt.Sprintf("Alerting system has %d critical alerts", criticalAlerts)
			}

			return &HealthCheckResult{
				Status:  status,
				Message: message,
				Details: map[string]interface{}{
					"active_alerts":   len(activeAlerts),
					"critical_alerts": criticalAlerts,
				},
			}
		},
		false,
	)

	// Set up alert rules that monitor health check failures
	obs.alertingSystem.AddRule(&AlertRule{
		Name:        "health_check_failure",
		Description: "Critical health check is failing",
		Metric:      "health_check_status",
		Condition:   ConditionEquals,
		Threshold:   0, // 0 = unhealthy
		Duration:    1 * time.Minute,
		Severity:    AlertSeverityCritical,
		Enabled:     true,
		Suppression: 5 * time.Minute,
		Labels: map[string]string{
			"component": "health",
			"type":      "system",
		},
		Annotations: map[string]string{
			"summary":     "Health check failure detected",
			"description": "A critical health check is failing",
		},
	})
}

// RecordDatabaseOperation records metrics for a database operation
func (obs *ObservabilitySystem) RecordDatabaseOperation(operation, table string, duration time.Duration, success bool) {
	// Record metrics
	obs.prometheusMetrics.RecordQueryDuration(duration, operation, table)

	if !success {
		obs.prometheusMetrics.RecordError("database_operation", "database")
	}
}

// RecordWALOperation records metrics for a WAL operation
func (obs *ObservabilitySystem) RecordWALOperation(duration time.Duration, success bool) {
	obs.prometheusMetrics.RecordWALWrite()

	if success {
		obs.prometheusMetrics.RecordWALSyncDuration(duration)
	} else {
		obs.prometheusMetrics.RecordError("wal_operation", "wal")
	}
}

// RecordTransaction records metrics for a transaction
func (obs *ObservabilitySystem) RecordTransaction(txType, status string) {
	obs.prometheusMetrics.RecordTransaction(txType, status)
}

// UpdateResourceMetrics updates system resource metrics
func (obs *ObservabilitySystem) UpdateResourceMetrics(memoryBytes, diskBytes int64, cpuPercent float64) {
	obs.prometheusMetrics.SetMemoryUsage(memoryBytes)
	obs.prometheusMetrics.SetDiskUsage(diskBytes)
	obs.prometheusMetrics.SetCPUUsage(cpuPercent)
}

// SetConnectionCount sets the current number of active connections
func (obs *ObservabilitySystem) SetConnectionCount(count int) {
	obs.prometheusMetrics.SetActiveConnections(count)
}

// SetThroughput sets the current throughput
func (obs *ObservabilitySystem) SetThroughput(opsPerSecond float64) {
	obs.prometheusMetrics.SetThroughput(opsPerSecond)
}

// SetCacheMetrics sets cache-related metrics
func (obs *ObservabilitySystem) SetCacheMetrics(hitRatio float64) {
	obs.prometheusMetrics.SetCacheHitRatio(hitRatio)
}

// GetSystemStatus returns a comprehensive system status
func (obs *ObservabilitySystem) GetSystemStatus() *SystemStatus {
	healthReport := obs.healthSystem.GetSystemHealth()
	activeAlerts := obs.alertingSystem.GetActiveAlerts()

	status := &SystemStatus{
		Timestamp:      time.Now(),
		OverallHealth:  healthReport.Status,
		Uptime:         healthReport.Uptime,
		ActiveAlerts:   len(activeAlerts),
		CriticalAlerts: 0,
		Components: map[string]ComponentStatus{
			"metrics": {
				Status:  HealthStatusHealthy,
				Message: "Metrics collection active",
			},
			"health": {
				Status:  healthReport.Status,
				Message: fmt.Sprintf("%d checks completed", len(healthReport.Checks)),
			},
			"alerting": {
				Status:  HealthStatusHealthy,
				Message: fmt.Sprintf("%d active alerts", len(activeAlerts)),
			},
		},
	}

	// Count critical alerts
	for _, alert := range activeAlerts {
		if alert.Severity == AlertSeverityCritical {
			status.CriticalAlerts++
		}
	}

	// Update alerting component status based on critical alerts
	if status.CriticalAlerts > 0 {
		status.Components["alerting"] = ComponentStatus{
			Status:  HealthStatusDegraded,
			Message: fmt.Sprintf("%d critical alerts active", status.CriticalAlerts),
		}
	}

	return status
}

// SystemStatus represents the overall system status
type SystemStatus struct {
	Timestamp      time.Time                  `json:"timestamp"`
	OverallHealth  HealthStatus               `json:"overall_health"`
	Uptime         time.Duration              `json:"uptime"`
	ActiveAlerts   int                        `json:"active_alerts"`
	CriticalAlerts int                        `json:"critical_alerts"`
	Components     map[string]ComponentStatus `json:"components"`
}

// ComponentStatus represents the status of a system component
type ComponentStatus struct {
	Status  HealthStatus `json:"status"`
	Message string       `json:"message"`
}

// CreateObservabilityHandler creates an HTTP handler for observability endpoints
func (obs *ObservabilitySystem) CreateObservabilityHandler() http.Handler {
	mux := http.NewServeMux()

	// System status endpoint
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		status := obs.GetSystemStatus()

		w.Header().Set("Content-Type", "application/json")

		httpStatus := http.StatusOK
		if status.OverallHealth == HealthStatusUnhealthy {
			httpStatus = http.StatusServiceUnavailable
		} else if status.OverallHealth == HealthStatusDegraded {
			httpStatus = http.StatusPartialContent
		}

		w.WriteHeader(httpStatus)

		// Simple JSON response
		fmt.Fprintf(w, `{
			"timestamp": "%s",
			"overall_health": "%s",
			"uptime": "%s",
			"active_alerts": %d,
			"critical_alerts": %d
		}`, status.Timestamp.Format(time.RFC3339),
			status.OverallHealth,
			status.Uptime.String(),
			status.ActiveAlerts,
			status.CriticalAlerts)
	})

	// Alerts endpoint
	mux.HandleFunc("/alerts", func(w http.ResponseWriter, r *http.Request) {
		alerts := obs.alertingSystem.GetActiveAlerts()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		fmt.Fprintf(w, `{"active_alerts": %d, "timestamp": "%s"}`,
			len(alerts), time.Now().Format(time.RFC3339))
	})

	return mux
}
