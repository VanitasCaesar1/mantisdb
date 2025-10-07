# MantisDB Metrics and Observability System

This package provides a comprehensive metrics and observability system for MantisDB, implementing Prometheus-compatible metrics, health checks, and alerting with multiple notification channels.

## Features

### 🔍 Prometheus Metrics
- **Query Performance**: Query duration, throughput, error rates
- **Resource Usage**: Memory, disk, CPU utilization
- **Database Operations**: WAL writes, transactions, lock wait times
- **System Metrics**: Active connections, cache hit ratios, uptime
- **HTTP Endpoint**: Prometheus-compatible `/metrics` endpoint

### 🏥 Health Checks
- **System Health**: Memory, goroutines, disk space
- **Database Health**: Connectivity, performance indicators
- **Load Balancer Integration**: Readiness and liveness probes
- **Custom Checks**: Extensible health check system
- **Dependency Monitoring**: External service health tracking

### 🚨 Alerting System
- **Configurable Rules**: Threshold-based alerting with multiple conditions
- **Multiple Channels**: Email, Slack, webhooks, console
- **Alert Suppression**: Prevent alert spam with configurable suppression
- **Escalation**: Automatic alert escalation for unresolved issues
- **History Tracking**: Complete alert history and resolution tracking

## Quick Start

```go
package main

import (
    "log"
    "github.com/mantisdb/advanced/metrics"
)

func main() {
    // Create observability system with default configuration
    obs := metrics.NewObservabilitySystem(nil)
    
    // Start all components
    if err := obs.Start(); err != nil {
        log.Fatalf("Failed to start observability: %v", err)
    }
    defer obs.Stop()
    
    // Record some metrics
    obs.RecordDatabaseOperation("SELECT", "users", 50*time.Millisecond, true)
    obs.SetConnectionCount(25)
    obs.UpdateResourceMetrics(512*1024*1024, 50*1024*1024*1024, 25.5)
    
    // System will now expose:
    // - Metrics at http://localhost:9090/metrics
    // - Health checks at http://localhost:8080/health
    // - Readiness probe at http://localhost:8080/health/ready
    // - Liveness probe at http://localhost:8080/health/live
}
```

For complete documentation, see the [metrics package documentation](../advanced/metrics/).