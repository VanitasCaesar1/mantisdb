package monitoring

import (
	"context"
	"testing"
	"time"
)

// Example of how to use the monitoring system
func TestMonitoringSystemExample(t *testing.T) {
	// Create monitoring system with default configuration
	config := DefaultMonitoringConfig()
	config.HTTPEnabled = false // Disable HTTP for testing

	ms := NewMonitoringSystem(config)

	// Start the monitoring system
	if err := ms.Start(); err != nil {
		t.Fatalf("Failed to start monitoring system: %v", err)
	}
	defer ms.Stop()

	// Get components
	collector := ms.GetMetricsCollector()
	logger := ms.GetOperationalLogger()
	auditTrail := ms.GetAuditTrail()
	healthChecker := ms.GetHealthChecker()

	// Example: Record some metrics
	collector.RecordWALWrite()
	collector.RecordWALSyncLatency(10 * time.Millisecond)
	collector.RecordTransactionStart()
	collector.RecordTransactionCommit()

	// Example: Log operational events
	logger.LogWALOperation("write", 12345, 67890, true, 5*time.Millisecond, map[string]interface{}{
		"size": 1024,
	})

	logger.LogTransactionOperation("commit", 67890, true, 15*time.Millisecond, map[string]interface{}{
		"operations": 3,
	})

	// Example: Log audit events
	auditTrail.LogDataAccess("user123", "session456", "table:users", "SELECT", true, map[string]interface{}{
		"query":  "SELECT * FROM users WHERE id = ?",
		"params": []interface{}{123},
	})

	auditTrail.LogTransaction("user123", "session456", 67890, "commit", true, map[string]interface{}{
		"duration": "15ms",
	})

	// Example: Check system health
	healthReport := healthChecker.GetSystemHealth()
	t.Logf("System health: %v", healthReport.Overall)

	// Example: Get metrics summary
	summary := collector.GetSummaryMetrics()
	t.Logf("WAL writes: %d", summary["wal_writes_total"])
	t.Logf("Transactions started: %d", summary["transactions_started_total"])

	// Example: Export metrics
	exporter := NewMetricsExporter(collector)
	jsonMetrics, err := exporter.ExportToString(JSONFormat)
	if err != nil {
		t.Errorf("Failed to export JSON metrics: %v", err)
	} else {
		t.Logf("JSON metrics length: %d", len(jsonMetrics))
	}

	// Example: Verify audit integrity
	valid, issues := auditTrail.VerifyIntegrity()
	if !valid {
		t.Errorf("Audit integrity check failed: %v", issues)
	} else {
		t.Log("Audit integrity verified")
	}

	// Example: Get audit events
	filter := AuditFilter{
		EventType: AuditEventTypeDataAccess,
		UserID:    "user123",
		Limit:     10,
	}

	events, err := auditTrail.GetEvents(filter)
	if err != nil {
		t.Errorf("Failed to get audit events: %v", err)
	} else {
		t.Logf("Found %d audit events", len(events))
	}
}

// Example WAL metrics provider implementation
type ExampleWALMetrics struct {
	writes       int64
	errors       int64
	syncTime     int64
	rotations    int64
	recoveryTime int64
}

func (w *ExampleWALMetrics) GetWALStats() WALStats {
	return WALStats{
		WritesTotal:     w.writes,
		WriteErrors:     w.errors,
		SyncLatencyNs:   w.syncTime,
		FileRotations:   w.rotations,
		RecoveryTimeNs:  w.recoveryTime,
		CurrentFileSize: 1024 * 1024, // 1MB
		TotalFilesCount: 5,
	}
}

// Example transaction metrics provider implementation
type ExampleTransactionMetrics struct {
	started   int64
	committed int64
	aborted   int64
	deadlocks int64
	waitTime  int64
}

func (t *ExampleTransactionMetrics) GetTransactionStats() TransactionStats {
	return TransactionStats{
		StartedTotal:     t.started,
		CommittedTotal:   t.committed,
		AbortedTotal:     t.aborted,
		DeadlocksTotal:   t.deadlocks,
		LockWaitTimeNs:   t.waitTime,
		ActiveCount:      2,
		LongestRunningNs: int64(30 * time.Second),
	}
}

// Example of registering custom metrics providers
func TestCustomMetricsProviders(t *testing.T) {
	config := DefaultMonitoringConfig()
	config.HTTPEnabled = false

	ms := NewMonitoringSystem(config)

	// Create custom metrics providers
	walMetrics := &ExampleWALMetrics{
		writes:       100,
		errors:       2,
		syncTime:     int64(5 * time.Millisecond),
		rotations:    3,
		recoveryTime: int64(2 * time.Second),
	}

	txnMetrics := &ExampleTransactionMetrics{
		started:   50,
		committed: 48,
		aborted:   2,
		deadlocks: 0,
		waitTime:  int64(10 * time.Millisecond),
	}

	// Register providers
	ms.RegisterWALMetrics(walMetrics)
	ms.RegisterTransactionMetrics(txnMetrics)

	// Start monitoring
	if err := ms.Start(); err != nil {
		t.Fatalf("Failed to start monitoring system: %v", err)
	}
	defer ms.Stop()

	// Wait a bit for metrics collection
	time.Sleep(100 * time.Millisecond)

	// Check that metrics were collected
	collector := ms.GetMetricsCollector()
	summary := collector.GetSummaryMetrics()

	t.Logf("Collected metrics: %+v", summary)
}

// Example of custom health check
func TestCustomHealthCheck(t *testing.T) {
	config := DefaultMonitoringConfig()
	config.HTTPEnabled = false

	ms := NewMonitoringSystem(config)
	healthChecker := ms.GetHealthChecker()

	// Register a custom health check
	healthChecker.RegisterCheck("custom", "Custom component health", func(ctx context.Context) HealthCheckResult {
		// Simulate some health check logic
		return HealthCheckResult{
			Status:  HealthStatusHealthy,
			Message: "Custom component is healthy",
			Details: map[string]interface{}{
				"version": "1.0.0",
				"uptime":  "5m30s",
			},
		}
	}, false)

	if err := ms.Start(); err != nil {
		t.Fatalf("Failed to start monitoring system: %v", err)
	}
	defer ms.Stop()

	// Run the custom health check
	result, err := healthChecker.RunHealthCheck("custom")
	if err != nil {
		t.Errorf("Custom health check failed: %v", err)
	} else {
		t.Logf("Custom health check result: %+v", result)
	}
}
