# MantisDB Monitoring and Observability

This package provides comprehensive monitoring and observability capabilities for MantisDB, including metrics collection, health checks, alerting, operational logging, and audit trails.

## Features

### 1. Metrics Collection
- **Real-time metrics collection** for WAL, transactions, errors, and performance
- **Atomic counters** for thread-safe metric updates
- **Configurable collection intervals** for system and performance metrics
- **Multiple export formats**: JSON, Prometheus, Plain Text
- **Automatic metric aggregation** from registered providers

### 2. Health Checks
- **System health monitoring** with configurable checks
- **Component-specific health checks** (WAL, transactions, memory, corruption)
- **Automatic health status determination** (Healthy, Degraded, Unhealthy)
- **Critical vs non-critical check classification**
- **Configurable check intervals and timeouts**

### 3. Alerting System
- **Multi-level alerting** (Info, Warning, Critical)
- **Multiple alert handlers** (Log, Console, File, Webhook)
- **Alert suppression** to prevent spam
- **Alert escalation** with configurable timeouts
- **Alert resolution tracking**

### 4. Operational Logging
- **Structured logging** with JSON and text formatters
- **Log rotation** with configurable size and retention
- **Context-aware logging** with fields and trace IDs
- **Component-specific logging** for WAL, transactions, recovery, errors
- **Performance metric logging**

### 5. Audit Trails
- **Comprehensive audit logging** for all critical operations
- **Cryptographic integrity** with SHA-256 checksums
- **Event chaining** for tamper detection
- **Configurable retention policies**
- **Multiple audit event types** (data access, modifications, transactions, etc.)
- **Audit trail verification** and export capabilities

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

For complete documentation, see the [monitoring package documentation](../monitoring/).