# MantisDB Monitoring and Observability

This package provides comprehensive monitoring and observability capabilities for MantisDB, including metrics collection, health checks, alerting, operational logging, and audit trails.

## Features

### 1. Metrics Collection (`metrics.go`, `aggregator.go`)
- **Real-time metrics collection** for WAL, transactions, errors, and performance
- **Atomic counters** for thread-safe metric updates
- **Configurable collection intervals** for system and performance metrics
- **Multiple export formats**: JSON, Prometheus, Plain Text
- **Automatic metric aggregation** from registered providers

### 2. Health Checks (`health.go`)
- **System health monitoring** with configurable checks
- **Component-specific health checks** (WAL, transactions, memory, corruption)
- **Automatic health status determination** (Healthy, Degraded, Unhealthy)
- **Critical vs non-critical check classification**
- **Configurable check intervals and timeouts**

### 3. Alerting System (`alerting.go`)
- **Multi-level alerting** (Info, Warning, Critical)
- **Multiple alert handlers** (Log, Console, File, Webhook)
- **Alert suppression** to prevent spam
- **Alert escalation** with configurable timeouts
- **Alert resolution tracking**

### 4. Operational Logging (`logging.go`)
- **Structured logging** with JSON and text formatters
- **Log rotation** with configurable size and retention
- **Context-aware logging** with fields and trace IDs
- **Component-specific logging** for WAL, transactions, recovery, errors
- **Performance metric logging**

### 5. Audit Trails (`audit.go`)
- **Comprehensive audit logging** for all critical operations
- **Cryptographic integrity** with SHA-256 checksums
- **Event chaining** for tamper detection
- **Configurable retention policies**
- **Multiple audit event types** (data access, modifications, transactions, etc.)
- **Audit trail verification** and export capabilities

### 6. Unified Monitoring System (`system.go`)
- **Single entry point** for all monitoring capabilities
- **HTTP endpoints** for metrics, health, and audit data
- **Background maintenance** tasks
- **Configurable monitoring policies**
- **Component registration** for automatic metric collection

## Usage

### Basic Setup

```go
// Create monitoring system with default configuration
config := monitoring.DefaultMonitoringConfig()
ms := monitoring.NewMonitoringSystem(config)

// Start monitoring
if err := ms.Start(); err != nil {
    log.Fatal("Failed to start monitoring:", err)
}
defer ms.Stop()
```

### Recording Metrics

```go
collector := ms.GetMetricsCollector()

// Record WAL operations
collector.RecordWALWrite()
collector.RecordWALSyncLatency(10 * time.Millisecond)

// Record transaction operations
collector.RecordTransactionStart()
collector.RecordTransactionCommit()

// Record errors
collector.RecordError("io_error")
collector.RecordCorruptionEvent()
```

### Operational Logging

```go
logger := ms.GetOperationalLogger()

// Log WAL operations
logger.LogWALOperation("write", lsn, txnID, true, duration, details)

// Log transaction operations
logger.LogTransactionOperation("commit", txnID, true, duration, details)

// Log recovery operations
logger.LogRecoveryOperation("replay", true, duration, details)
```

### Audit Logging

```go
auditTrail := ms.GetAuditTrail()

// Log data access
auditTrail.LogDataAccess(userID, sessionID, resource, operation, success, details)

// Log data modifications
auditTrail.LogDataModification(userID, sessionID, resource, operation, success, details)

// Log transactions
auditTrail.LogTransaction(userID, sessionID, txnID, operation, success, details)
```

### Health Checks

```go
healthChecker := ms.GetHealthChecker()

// Get system health report
report := healthChecker.GetSystemHealth()
fmt.Printf("System health: %v\n", report.Overall)

// Register custom health check
healthChecker.RegisterCheck("custom", "Custom component", func(ctx context.Context) HealthCheckResult {
    return HealthCheckResult{
        Status:  HealthStatusHealthy,
        Message: "Component is healthy",
    }
}, false)
```

### Alerting

```go
alerter := ms.GetAlerter()

// Send alert
alerter.SendAlert(Alert{
    Level:     AlertLevelCritical,
    Component: "wal",
    Message:   "WAL write failed",
    Timestamp: time.Now(),
})

// Register custom alert handler
alerter.RegisterHandler(NewWebhookAlertHandler("https://alerts.example.com/webhook"))
```

### Registering Metrics Providers

```go
// Implement metrics provider interfaces
type MyWALMetrics struct {
    // ... implementation
}

func (w *MyWALMetrics) GetWALStats() WALStats {
    return WALStats{
        WritesTotal: w.writeCount,
        WriteErrors: w.errorCount,
        // ... other stats
    }
}

// Register with monitoring system
ms.RegisterWALMetrics(&MyWALMetrics{})
```

## HTTP Endpoints

When HTTP is enabled, the following endpoints are available:

- `GET /metrics` - Plain text metrics summary
- `GET /metrics/json` - JSON formatted metrics
- `GET /metrics/prometheus` - Prometheus formatted metrics
- `GET /health` - Overall system health status
- `GET /health/ready` - Readiness check
- `GET /health/live` - Liveness check
- `GET /alerts` - Alert summary
- `GET /alerts/active` - Active alerts only
- `GET /audit` - Audit system status
- `GET /audit/export` - Export recent audit events

## Configuration

```go
config := &MonitoringConfig{
    MetricsEnabled:         true,
    MetricsInterval:        30 * time.Second,
    HealthCheckEnabled:     true,
    HealthCheckInterval:    30 * time.Second,
    AlertingEnabled:        true,
    AlertSuppressionWindow: 5 * time.Minute,
    LoggingEnabled:         true,
    LogLevel:               LogLevelInfo,
    AuditEnabled:           true,
    AuditRetention:         365 * 24 * time.Hour,
    HTTPEnabled:            true,
    HTTPAddress:            "localhost",
    HTTPPort:               8080,
}
```

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ Metrics         │    │ Health Checks   │    │ Alerting        │
│ Collector       │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ Operational     │    │ Audit Trail     │    │ HTTP Server     │
│ Logging         │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Monitoring System                            │
└─────────────────────────────────────────────────────────────────┘
```

## Requirements Satisfied

This implementation satisfies the following requirements from the Critical Data Safety specification:

- **Requirement 8.1**: Comprehensive metrics collection for WAL, transaction, and error metrics
- **Requirement 8.2**: System health check endpoints with critical and warning alerts
- **Requirement 8.3**: Structured logging for all operations with audit trails
- **Requirement 8.4**: Audit trails for critical operations with integrity verification
- **Requirement 8.5**: Monitoring and alerting for system health degradation

## Testing

Run the example tests to see the monitoring system in action:

```bash
go test -v ./monitoring/
```

The tests demonstrate:
- Basic monitoring system usage
- Custom metrics provider registration
- Custom health check registration
- Audit trail verification
- Metrics export in different formats