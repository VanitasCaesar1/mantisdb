package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// MonitoringSystem provides a comprehensive monitoring solution
type MonitoringSystem struct {
	// Core components
	aggregator        *MetricsAggregator
	healthChecker     *HealthChecker
	alertManager      *AlertManager
	operationalLogger *OperationalLogger
	auditTrail        *AuditTrail

	// Configuration
	config *MonitoringConfig

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// HTTP server for metrics and health endpoints
	httpServer *http.Server
}

// MonitoringConfig holds configuration for the monitoring system
type MonitoringConfig struct {
	// Metrics configuration
	MetricsEnabled   bool          `json:"metrics_enabled"`
	MetricsInterval  time.Duration `json:"metrics_interval"`
	MetricsRetention time.Duration `json:"metrics_retention"`

	// Health check configuration
	HealthCheckEnabled  bool          `json:"health_check_enabled"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	HealthCheckTimeout  time.Duration `json:"health_check_timeout"`

	// Alerting configuration
	AlertingEnabled        bool          `json:"alerting_enabled"`
	AlertSuppressionWindow time.Duration `json:"alert_suppression_window"`

	// Logging configuration
	LoggingEnabled   bool     `json:"logging_enabled"`
	LogLevel         LogLevel `json:"log_level"`
	LogRotationSize  int64    `json:"log_rotation_size"`
	LogRetentionDays int      `json:"log_retention_days"`

	// Audit configuration
	AuditEnabled        bool          `json:"audit_enabled"`
	AuditRetention      time.Duration `json:"audit_retention"`
	AuditIntegrityCheck bool          `json:"audit_integrity_check"`

	// HTTP server configuration
	HTTPEnabled bool   `json:"http_enabled"`
	HTTPAddress string `json:"http_address"`
	HTTPPort    int    `json:"http_port"`
}

// DefaultMonitoringConfig returns default monitoring configuration
func DefaultMonitoringConfig() *MonitoringConfig {
	return &MonitoringConfig{
		MetricsEnabled:         true,
		MetricsInterval:        30 * time.Second,
		MetricsRetention:       24 * time.Hour,
		HealthCheckEnabled:     true,
		HealthCheckInterval:    30 * time.Second,
		HealthCheckTimeout:     10 * time.Second,
		AlertingEnabled:        true,
		AlertSuppressionWindow: 5 * time.Minute,
		LoggingEnabled:         true,
		LogLevel:               LogLevelInfo,
		LogRotationSize:        100 * 1024 * 1024, // 100MB
		LogRetentionDays:       30,
		AuditEnabled:           true,
		AuditRetention:         365 * 24 * time.Hour, // 1 year
		AuditIntegrityCheck:    true,
		HTTPEnabled:            true,
		HTTPAddress:            "localhost",
		HTTPPort:               8080,
	}
}

// NewMonitoringSystem creates a new monitoring system
func NewMonitoringSystem(config *MonitoringConfig) *MonitoringSystem {
	if config == nil {
		config = DefaultMonitoringConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Create core components
	aggregator := NewMetricsAggregator()
	alertManager := NewAlertManager()
	healthChecker := NewHealthChecker(aggregator, alertManager.GetAlerter())
	operationalLogger := NewOperationalLogger()
	auditStorage := NewInMemoryAuditStorage()
	auditTrail := NewAuditTrail(auditStorage)

	// Configure components
	aggregator.SetCollectionIntervals(config.MetricsInterval, config.MetricsInterval/2)
	healthChecker.SetCheckInterval(config.HealthCheckInterval)
	healthChecker.SetTimeout(config.HealthCheckTimeout)

	// Set up alert handlers
	alerter := alertManager.GetAlerter()
	alerter.RegisterHandler(NewLogAlertHandler())
	alerter.RegisterHandler(NewConsoleAlertHandler())
	alerter.SetSuppressionRule("system", config.AlertSuppressionWindow)

	ms := &MonitoringSystem{
		aggregator:        aggregator,
		healthChecker:     healthChecker,
		alertManager:      alertManager,
		operationalLogger: operationalLogger,
		auditTrail:        auditTrail,
		config:            config,
		ctx:               ctx,
		cancel:            cancel,
	}

	// Set up HTTP server if enabled
	if config.HTTPEnabled {
		ms.setupHTTPServer()
	}

	return ms
}

// Start starts the monitoring system
func (ms *MonitoringSystem) Start() error {
	// Start metrics aggregation
	if ms.config.MetricsEnabled {
		ms.aggregator.Start(ms.ctx)
	}

	// Start health checking
	if ms.config.HealthCheckEnabled {
		ms.healthChecker.Start(ms.ctx)
	}

	// Start alert manager
	if ms.config.AlertingEnabled {
		ms.alertManager.Start()
	}

	// Start HTTP server
	if ms.config.HTTPEnabled && ms.httpServer != nil {
		ms.wg.Add(1)
		go func() {
			defer ms.wg.Done()
			if err := ms.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				fmt.Printf("HTTP server error: %v\n", err)
			}
		}()
	}

	// Start background maintenance tasks
	ms.wg.Add(1)
	go ms.maintenanceLoop()

	return nil
}

// Stop stops the monitoring system
func (ms *MonitoringSystem) Stop() error {
	// Cancel context to stop all components
	ms.cancel()

	// Stop individual components
	ms.aggregator.Stop()
	ms.healthChecker.Stop()
	ms.alertManager.Stop()

	// Stop HTTP server
	if ms.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ms.httpServer.Shutdown(ctx)
	}

	// Wait for all goroutines to finish
	ms.wg.Wait()

	return nil
}

// GetMetricsCollector returns the metrics collector
func (ms *MonitoringSystem) GetMetricsCollector() *MetricsCollector {
	return ms.aggregator.GetCollector()
}

// GetHealthChecker returns the health checker
func (ms *MonitoringSystem) GetHealthChecker() *HealthChecker {
	return ms.healthChecker
}

// GetAlerter returns the alerter
func (ms *MonitoringSystem) GetAlerter() *Alerter {
	return ms.alertManager.GetAlerter()
}

// GetOperationalLogger returns the operational logger
func (ms *MonitoringSystem) GetOperationalLogger() *OperationalLogger {
	return ms.operationalLogger
}

// GetAuditTrail returns the audit trail
func (ms *MonitoringSystem) GetAuditTrail() *AuditTrail {
	return ms.auditTrail
}

// RegisterWALMetrics registers WAL metrics provider
func (ms *MonitoringSystem) RegisterWALMetrics(provider WALMetricsProvider) {
	ms.aggregator.RegisterWALMetrics(provider)
}

// RegisterTransactionMetrics registers transaction metrics provider
func (ms *MonitoringSystem) RegisterTransactionMetrics(provider TransactionMetricsProvider) {
	ms.aggregator.RegisterTransactionMetrics(provider)
}

// RegisterErrorMetrics registers error metrics provider
func (ms *MonitoringSystem) RegisterErrorMetrics(provider ErrorMetricsProvider) {
	ms.aggregator.RegisterErrorMetrics(provider)
}

// RegisterStorageMetrics registers storage metrics provider
func (ms *MonitoringSystem) RegisterStorageMetrics(provider StorageMetricsProvider) {
	ms.aggregator.RegisterStorageMetrics(provider)
}

// setupHTTPServer sets up the HTTP server for metrics and health endpoints
func (ms *MonitoringSystem) setupHTTPServer() {
	mux := http.NewServeMux()

	// Metrics endpoint
	mux.HandleFunc("/metrics", ms.handleMetrics)
	mux.HandleFunc("/metrics/prometheus", ms.handlePrometheusMetrics)
	mux.HandleFunc("/metrics/json", ms.handleJSONMetrics)

	// Health endpoints
	mux.HandleFunc("/health", ms.handleHealth)
	mux.HandleFunc("/health/ready", ms.handleReadiness)
	mux.HandleFunc("/health/live", ms.handleLiveness)

	// Alert endpoints
	mux.HandleFunc("/alerts", ms.handleAlerts)
	mux.HandleFunc("/alerts/active", ms.handleActiveAlerts)

	// Audit endpoints
	mux.HandleFunc("/audit", ms.handleAudit)
	mux.HandleFunc("/audit/export", ms.handleAuditExport)

	ms.httpServer = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", ms.config.HTTPAddress, ms.config.HTTPPort),
		Handler: mux,
	}
}

// HTTP handlers
func (ms *MonitoringSystem) handleMetrics(w http.ResponseWriter, r *http.Request) {
	exporter := ms.aggregator.GetExporter()
	w.Header().Set("Content-Type", "text/plain")
	exporter.Export(PlainTextFormat, w)
}

func (ms *MonitoringSystem) handlePrometheusMetrics(w http.ResponseWriter, r *http.Request) {
	exporter := ms.aggregator.GetExporter()
	w.Header().Set("Content-Type", "text/plain")
	exporter.Export(PrometheusFormat, w)
}

func (ms *MonitoringSystem) handleJSONMetrics(w http.ResponseWriter, r *http.Request) {
	exporter := ms.aggregator.GetExporter()
	w.Header().Set("Content-Type", "application/json")
	exporter.Export(JSONFormat, w)
}

func (ms *MonitoringSystem) handleHealth(w http.ResponseWriter, r *http.Request) {
	report := ms.healthChecker.GetSystemHealth()
	w.Header().Set("Content-Type", "application/json")

	status := http.StatusOK
	if report.Overall == HealthStatusUnhealthy {
		status = http.StatusServiceUnavailable
	} else if report.Overall == HealthStatusDegraded {
		status = http.StatusPartialContent
	}

	w.WriteHeader(status)

	// Simple JSON response
	fmt.Fprintf(w, `{"status": "%s", "timestamp": "%s"}`,
		ms.healthStatusToString(report.Overall),
		report.Timestamp.Format(time.RFC3339))
}

func (ms *MonitoringSystem) handleReadiness(w http.ResponseWriter, r *http.Request) {
	// Simple readiness check
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"ready": true, "timestamp": "%s"}`, time.Now().Format(time.RFC3339))
}

func (ms *MonitoringSystem) handleLiveness(w http.ResponseWriter, r *http.Request) {
	// Simple liveness check
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"alive": true, "timestamp": "%s"}`, time.Now().Format(time.RFC3339))
}

func (ms *MonitoringSystem) handleAlerts(w http.ResponseWriter, r *http.Request) {
	alerts := ms.alertManager.GetAlerter().GetAlerts()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"alerts": %d, "timestamp": "%s"}`, len(alerts), time.Now().Format(time.RFC3339))
}

func (ms *MonitoringSystem) handleActiveAlerts(w http.ResponseWriter, r *http.Request) {
	alerts := ms.alertManager.GetAlerter().GetActiveAlerts()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"active_alerts": %d, "timestamp": "%s"}`, len(alerts), time.Now().Format(time.RFC3339))
}

func (ms *MonitoringSystem) handleAudit(w http.ResponseWriter, r *http.Request) {
	// Simple audit summary
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"audit_enabled": %t, "timestamp": "%s"}`, ms.config.AuditEnabled, time.Now().Format(time.RFC3339))
}

func (ms *MonitoringSystem) handleAuditExport(w http.ResponseWriter, r *http.Request) {
	// Export recent audit events
	filter := AuditFilter{
		StartTime: time.Now().Add(-24 * time.Hour),
		Limit:     1000,
	}

	data, err := ms.auditTrail.ExportEvents(filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// maintenanceLoop runs background maintenance tasks
func (ms *MonitoringSystem) maintenanceLoop() {
	defer ms.wg.Done()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ms.ctx.Done():
			return
		case <-ticker.C:
			ms.performMaintenance()
		}
	}
}

// performMaintenance performs periodic maintenance tasks
func (ms *MonitoringSystem) performMaintenance() {
	// Purge old audit events
	if ms.config.AuditEnabled {
		if err := ms.auditTrail.PurgeOldEvents(); err != nil {
			ms.operationalLogger.LogErrorEvent("monitoring", "maintenance", "audit_purge", err, nil)
		}
	}

	// Verify audit integrity if enabled
	if ms.config.AuditEnabled && ms.config.AuditIntegrityCheck {
		if valid, issues := ms.auditTrail.VerifyIntegrity(); !valid {
			ms.alertManager.GetAlerter().SendAlert(Alert{
				Level:     AlertLevelCritical,
				Component: "audit",
				Message:   "Audit trail integrity check failed",
				Timestamp: time.Now(),
				Details: map[string]interface{}{
					"issues": issues,
				},
			})
		}
	}

	// Log maintenance completion
	ms.operationalLogger.LogSystemEvent("maintenance_completed", map[string]interface{}{
		"timestamp": time.Now(),
	})
}

// healthStatusToString converts health status to string
func (ms *MonitoringSystem) healthStatusToString(status HealthStatusType) string {
	switch status {
	case HealthStatusHealthy:
		return "healthy"
	case HealthStatusDegraded:
		return "degraded"
	case HealthStatusUnhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}
