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

## Configuration

### Environment Variables

Configure alerting through environment variables:

```bash
# Email notifications
export MANTIS_SMTP_HOST=smtp.gmail.com
export MANTIS_SMTP_PORT=587
export MANTIS_SMTP_USERNAME=alerts@mantisdb.com
export MANTIS_SMTP_PASSWORD=app-password
export MANTIS_EMAIL_FROM=alerts@mantisdb.com

# Slack notifications
export MANTIS_SLACK_WEBHOOK_URL=https://hooks.slack.com/services/...
export MANTIS_SLACK_CHANNEL=#alerts

# Webhook notifications
export MANTIS_WEBHOOK_URL=https://api.example.com/webhooks/alerts
```

### Programmatic Configuration

```go
config := &metrics.ObservabilityConfig{
    MetricsEnabled: true,
    MetricsAddr:    ":9090",
    HealthEnabled:  true,
    HealthAddr:     ":8080",
    AlertingConfig: &metrics.AlertingConfig{
        Enabled:            true,
        EvaluationInterval: 30 * time.Second,
        EmailEnabled:       true,
        SMTPHost:          "smtp.example.com",
        SMTPPort:          587,
        EmailFrom:         "alerts@mantisdb.com",
        EmailTo:           []string{"admin@mantisdb.com"},
    },
}

obs := metrics.NewObservabilitySystem(config)
```

## Metrics Reference

### Query Metrics
- `mantisdb_query_duration_seconds` - Query execution time histogram
- `mantisdb_queries_total` - Total number of queries executed
- `mantisdb_active_connections` - Current active database connections
- `mantisdb_throughput_ops_per_second` - Current throughput

### Resource Metrics
- `mantisdb_memory_usage_bytes` - Current memory usage
- `mantisdb_disk_usage_bytes` - Current disk usage
- `mantisdb_cpu_usage_percent` - Current CPU usage percentage

### Database Metrics
- `mantisdb_wal_writes_total` - Total WAL write operations
- `mantisdb_wal_sync_duration_seconds` - WAL sync duration histogram
- `mantisdb_transactions_total` - Total transactions by type and status
- `mantisdb_lock_wait_duration_seconds` - Lock wait time histogram
- `mantisdb_cache_hit_ratio` - Cache hit ratio (0.0 to 1.0)

### System Metrics
- `mantisdb_uptime_seconds` - System uptime in seconds
- `mantisdb_errors_total` - Total errors by type and component

## Health Check Endpoints

### `/health`
Basic health status with HTTP status codes:
- `200 OK` - System healthy
- `206 Partial Content` - System degraded
- `503 Service Unavailable` - System unhealthy

### `/health/ready`
Readiness probe for load balancers. Returns `503` if:
- System is in startup grace period
- Critical components are failing

### `/health/live`
Liveness probe for load balancers. Returns `503` if:
- Critical health checks have failed multiple times
- System is unresponsive

### `/health/detailed`
Comprehensive health report including:
- Individual check results
- Dependency status
- System summary
- Detailed error information

## Custom Health Checks

Add custom health checks for your specific needs:

```go
obs.GetHealthSystem().RegisterHealthCheck(
    "external_api",
    "External API connectivity",
    func(ctx context.Context) *metrics.HealthCheckResult {
        // Your custom check logic here
        resp, err := http.Get("https://api.example.com/health")
        if err != nil {
            return &metrics.HealthCheckResult{
                Status:  metrics.HealthStatusUnhealthy,
                Message: fmt.Sprintf("API unreachable: %v", err),
            }
        }
        defer resp.Body.Close()
        
        if resp.StatusCode != 200 {
            return &metrics.HealthCheckResult{
                Status:  metrics.HealthStatusDegraded,
                Message: fmt.Sprintf("API returned %d", resp.StatusCode),
            }
        }
        
        return &metrics.HealthCheckResult{
            Status:  metrics.HealthStatusHealthy,
            Message: "External API is healthy",
        }
    },
    true, // Critical check
)
```

## Custom Alert Rules

Define custom alert rules for your metrics:

```go
obs.GetAlertingSystem().AddRule(&metrics.AlertRule{
    Name:        "high_connection_count",
    Description: "Too many active connections",
    Metric:      "active_connections",
    Condition:   metrics.ConditionGreaterThan,
    Threshold:   100,
    Duration:    5 * time.Minute,
    Severity:    metrics.AlertSeverityWarning,
    Enabled:     true,
    Suppression: 10 * time.Minute,
    Labels: map[string]string{
        "component": "database",
        "type":      "resource",
    },
    Annotations: map[string]string{
        "summary":     "High connection count detected",
        "description": "Active connections exceed 100",
        "runbook":     "https://docs.mantisdb.com/runbooks/connections",
    },
})
```

## Alert Conditions

Available alert conditions:
- `ConditionGreaterThan` - Value > threshold
- `ConditionLessThan` - Value < threshold
- `ConditionEquals` - Value == threshold
- `ConditionNotEquals` - Value != threshold
- `ConditionGreaterOrEqual` - Value >= threshold
- `ConditionLessOrEqual` - Value <= threshold

## Notification Channels

### Email
Sends formatted email alerts via SMTP:
```go
config.AlertingConfig.EmailEnabled = true
config.AlertingConfig.SMTPHost = "smtp.gmail.com"
config.AlertingConfig.SMTPPort = 587
config.AlertingConfig.SMTPUsername = "alerts@mantisdb.com"
config.AlertingConfig.SMTPPassword = "app-password"
config.AlertingConfig.EmailFrom = "alerts@mantisdb.com"
config.AlertingConfig.EmailTo = []string{"admin@mantisdb.com"}
```

### Slack
Sends rich Slack messages with color coding:
```go
config.AlertingConfig.SlackEnabled = true
config.AlertingConfig.SlackWebhookURL = "https://hooks.slack.com/services/..."
config.AlertingConfig.SlackChannel = "#alerts"
```

### Webhook
Sends JSON payloads to custom endpoints:
```go
config.AlertingConfig.WebhookEnabled = true
config.AlertingConfig.WebhookURL = "https://api.example.com/alerts"
config.AlertingConfig.WebhookTimeout = 10 * time.Second
```

### Console
Always-enabled console output for development and debugging.

## Integration with Load Balancers

### Kubernetes
```yaml
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: mantisdb
    image: mantisdb:latest
    ports:
    - containerPort: 8080
      name: health
    readinessProbe:
      httpGet:
        path: /health/ready
        port: health
      initialDelaySeconds: 10
      periodSeconds: 5
    livenessProbe:
      httpGet:
        path: /health/live
        port: health
      initialDelaySeconds: 30
      periodSeconds: 10
```

### HAProxy
```
backend mantisdb
    option httpchk GET /health/ready
    server mantis1 10.0.0.1:8080 check
    server mantis2 10.0.0.2:8080 check
```

### NGINX
```nginx
upstream mantisdb {
    server 10.0.0.1:8080;
    server 10.0.0.2:8080;
}

location /health {
    proxy_pass http://mantisdb/health/ready;
}
```

## Monitoring Integration

### Prometheus Configuration
```yaml
scrape_configs:
  - job_name: 'mantisdb'
    static_configs:
      - targets: ['localhost:9090']
    scrape_interval: 15s
    metrics_path: /metrics
```

### Grafana Dashboard
Import metrics into Grafana for visualization:
- Query performance graphs
- Resource utilization charts
- Alert status panels
- System health overview

## Best Practices

1. **Metric Naming**: Follow Prometheus naming conventions
2. **Label Usage**: Use labels for dimensions, not high-cardinality data
3. **Alert Tuning**: Start with conservative thresholds and adjust based on baseline
4. **Health Checks**: Keep checks lightweight and fast
5. **Suppression**: Use appropriate suppression windows to prevent alert fatigue
6. **Documentation**: Document custom metrics and alert runbooks

## Troubleshooting

### Common Issues

**Metrics not appearing**
- Check if metrics server is running on correct port
- Verify Prometheus scrape configuration
- Check for firewall blocking metrics port

**Health checks failing**
- Review health check logs for specific failures
- Verify dependencies are accessible
- Check timeout configurations

**Alerts not firing**
- Verify alert rules are enabled
- Check metric values against thresholds
- Review suppression settings
- Confirm notification channels are configured

**High memory usage**
- Monitor metric cardinality
- Review alert history retention settings
- Check for metric label explosion

### Debug Mode

Enable debug logging for troubleshooting:
```go
config.AlertingConfig.EvaluationInterval = 10 * time.Second // More frequent evaluation
// Add console channel for immediate feedback
```

## Performance Considerations

- **Metric Collection**: Minimal overhead, uses atomic operations
- **Health Checks**: Configurable timeouts and intervals
- **Alert Evaluation**: Efficient rule evaluation with caching
- **Memory Usage**: Bounded alert history with automatic cleanup
- **Network**: Async notification delivery to prevent blocking

## Security

- **SMTP**: Use app passwords or OAuth for email authentication
- **Webhooks**: Implement proper authentication and HTTPS
- **Endpoints**: Consider authentication for sensitive health endpoints
- **Secrets**: Store credentials in environment variables or secret management systems