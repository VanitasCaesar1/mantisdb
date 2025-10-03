package metrics

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Example demonstrates how to use the complete metrics and observability system
func Example() {
	// Create observability system with default configuration
	config := DefaultObservabilityConfig()

	// Customize configuration if needed
	config.MetricsAddr = ":9090"
	config.HealthAddr = ":8080"

	// Enable alerting with email notifications
	config.AlertingConfig.EmailEnabled = true
	config.AlertingConfig.SMTPHost = "smtp.example.com"
	config.AlertingConfig.SMTPPort = 587
	config.AlertingConfig.EmailFrom = "alerts@mantisdb.com"
	config.AlertingConfig.EmailTo = []string{"admin@mantisdb.com"}

	// Create the observability system
	obs := NewObservabilitySystem(config)

	// Start all components
	if err := obs.Start(); err != nil {
		log.Fatalf("Failed to start observability system: %v", err)
	}

	fmt.Println("Observability system started successfully!")
	fmt.Printf("Metrics available at: http://localhost%s/metrics\n", config.MetricsAddr)
	fmt.Printf("Health checks available at: http://localhost%s/health\n", config.HealthAddr)
	fmt.Printf("Detailed health at: http://localhost%s/health/detailed\n", config.HealthAddr)
	fmt.Printf("Readiness probe at: http://localhost%s/health/ready\n", config.HealthAddr)
	fmt.Printf("Liveness probe at: http://localhost%s/health/live\n", config.HealthAddr)

	// Simulate some database operations
	go simulateOperations(obs)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	fmt.Println("\nShutting down observability system...")

	// Stop all components
	if err := obs.Stop(); err != nil {
		log.Printf("Error stopping observability system: %v", err)
	}

	fmt.Println("Observability system stopped")
}

// simulateOperations simulates database operations to generate metrics
func simulateOperations(obs *ObservabilitySystem) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	connectionCount := 10
	throughput := 100.0

	for {
		select {
		case <-ticker.C:
			// Simulate query operations
			obs.RecordDatabaseOperation("SELECT", "users", 50*time.Millisecond, true)
			obs.RecordDatabaseOperation("INSERT", "orders", 100*time.Millisecond, true)
			obs.RecordDatabaseOperation("UPDATE", "products", 75*time.Millisecond, true)

			// Occasionally simulate an error
			if time.Now().Unix()%10 == 0 {
				obs.RecordDatabaseOperation("DELETE", "temp", 200*time.Millisecond, false)
			}

			// Simulate WAL operations
			obs.RecordWALOperation(10*time.Millisecond, true)

			// Simulate transactions
			obs.RecordTransaction("read_write", "committed")
			obs.RecordTransaction("read_only", "committed")

			// Update resource metrics
			memoryMB := int64(512 * 1024 * 1024)     // 512MB
			diskGB := int64(50 * 1024 * 1024 * 1024) // 50GB
			cpuPercent := 25.5

			obs.UpdateResourceMetrics(memoryMB, diskGB, cpuPercent)

			// Update connection metrics
			connectionCount += int((time.Now().Unix() % 3) - 1) // Vary by -1, 0, or 1
			if connectionCount < 5 {
				connectionCount = 5
			}
			if connectionCount > 50 {
				connectionCount = 50
			}
			obs.SetConnectionCount(connectionCount)

			// Update throughput
			throughput += (float64(time.Now().Unix()%10) - 5.0) * 2.0 // Vary throughput
			if throughput < 50 {
				throughput = 50
			}
			if throughput > 200 {
				throughput = 200
			}
			obs.SetThroughput(throughput)

			// Update cache metrics
			cacheHitRatio := 0.85 + (float64(time.Now().Unix()%10) * 0.01) // 85-95%
			obs.SetCacheMetrics(cacheHitRatio)

			// Print current status
			status := obs.GetSystemStatus()
			fmt.Printf("[%s] Health: %s, Alerts: %d (Critical: %d), Connections: %d, Throughput: %.1f ops/s\n",
				status.Timestamp.Format("15:04:05"),
				status.OverallHealth,
				status.ActiveAlerts,
				status.CriticalAlerts,
				connectionCount,
				throughput)
		}
	}
}

// ExampleCustomHealthCheck demonstrates how to add custom health checks
func ExampleCustomHealthCheck() {
	obs := NewObservabilitySystem(nil)

	// Add a custom health check for external service
	obs.GetHealthSystem().RegisterHealthCheck(
		"external_api",
		"External API connectivity",
		func(ctx context.Context) *HealthCheckResult {
			// Simulate checking external API
			// In real implementation, you'd make an actual HTTP request

			return &HealthCheckResult{
				Status:  HealthStatusHealthy,
				Message: "External API is responding",
				Details: map[string]interface{}{
					"response_time_ms": 150,
					"endpoint":         "https://api.example.com/health",
				},
			}
		},
		true, // Critical check
	)

	// Add a custom dependency check
	obs.GetHealthSystem().RegisterDependency(
		"redis_cache",
		"cache",
		"redis://localhost:6379",
		func(ctx context.Context) *HealthCheckResult {
			// Simulate Redis connectivity check
			return &HealthCheckResult{
				Status:  HealthStatusHealthy,
				Message: "Redis cache is available",
				Details: map[string]interface{}{
					"connected_clients": 5,
					"used_memory_mb":    128,
				},
			}
		},
		false, // Non-critical
	)

	fmt.Println("Custom health checks registered")
}

// ExampleCustomAlertRule demonstrates how to add custom alert rules
func ExampleCustomAlertRule() {
	config := DefaultObservabilityConfig()
	obs := NewObservabilitySystem(config)

	// Add a custom alert rule for connection pool exhaustion
	obs.GetAlertingSystem().AddRule(&AlertRule{
		Name:        "connection_pool_exhaustion",
		Description: "Connection pool is nearly exhausted",
		Metric:      "active_connections",
		Condition:   ConditionGreaterThan,
		Threshold:   45, // Alert when more than 45 connections
		Duration:    2 * time.Minute,
		Severity:    AlertSeverityWarning,
		Enabled:     true,
		Suppression: 10 * time.Minute,
		Labels: map[string]string{
			"component": "database",
			"type":      "resource",
		},
		Annotations: map[string]string{
			"summary":     "Connection pool nearly exhausted",
			"description": "Active connections have exceeded 45 for more than 2 minutes",
			"runbook":     "https://docs.mantisdb.com/runbooks/connection-pool",
		},
	})

	// Add a custom alert rule for cache performance
	obs.GetAlertingSystem().AddRule(&AlertRule{
		Name:        "low_cache_hit_ratio",
		Description: "Cache hit ratio is below optimal threshold",
		Metric:      "cache_hit_ratio",
		Condition:   ConditionLessThan,
		Threshold:   0.8, // Alert when hit ratio drops below 80%
		Duration:    5 * time.Minute,
		Severity:    AlertSeverityWarning,
		Enabled:     true,
		Suppression: 15 * time.Minute,
		Labels: map[string]string{
			"component": "cache",
			"type":      "performance",
		},
		Annotations: map[string]string{
			"summary":     "Low cache hit ratio detected",
			"description": "Cache hit ratio has been below 80% for more than 5 minutes",
			"impact":      "Query performance may be degraded",
		},
	})

	fmt.Println("Custom alert rules registered")
}

// ExampleEnvironmentConfiguration demonstrates loading configuration from environment
func ExampleEnvironmentConfiguration() {
	// Set environment variables for configuration
	os.Setenv("MANTIS_SMTP_HOST", "smtp.gmail.com")
	os.Setenv("MANTIS_SMTP_PORT", "587")
	os.Setenv("MANTIS_SMTP_USERNAME", "alerts@mantisdb.com")
	os.Setenv("MANTIS_SMTP_PASSWORD", "app-password")
	os.Setenv("MANTIS_EMAIL_FROM", "alerts@mantisdb.com")
	os.Setenv("MANTIS_SLACK_WEBHOOK_URL", "https://hooks.slack.com/services/...")
	os.Setenv("MANTIS_SLACK_CHANNEL", "#alerts")

	// Load configuration from environment
	alertingConfig := LoadAlertingConfigFromEnv()

	config := DefaultObservabilityConfig()
	config.AlertingConfig = alertingConfig

	obs := NewObservabilitySystem(config)

	fmt.Printf("Loaded configuration from environment:\n")
	fmt.Printf("  Email enabled: %t\n", alertingConfig.EmailEnabled)
	fmt.Printf("  Slack enabled: %t\n", alertingConfig.SlackEnabled)
	fmt.Printf("  SMTP host: %s\n", alertingConfig.SMTPHost)

	_ = obs // Use the observability system as needed
}
