package memory

import (
	"context"
	"fmt"
	"log"
	"time"
)

// MemorySystem integrates all memory management components
type MemorySystem struct {
	cacheManager       *CacheManager
	monitor            *MemoryMonitor
	collector          *MetricsCollector
	healthChecker      *HealthChecker
	performanceTracker *PerformanceTracker
	alertManager       *AlertManager
	dashboard          *Dashboard
	config             *SystemConfig
	ctx                context.Context
	cancel             context.CancelFunc
}

// SystemConfig holds configuration for the entire memory system
type SystemConfig struct {
	Cache       *Config
	Dashboard   *DashboardConfig
	Monitoring  MonitoringConfig
	Alerting    AlertingConfig
	Performance PerformanceConfig
}

// MonitoringConfig holds monitoring configuration
type MonitoringConfig struct {
	Interval        time.Duration
	MemoryThreshold float64
	HeapThreshold   float64
	GCPressureLimit float64
	EnableProfiling bool
}

// AlertingConfig holds alerting configuration
type AlertingConfig struct {
	MaxAlerts          int
	RetentionPeriod    time.Duration
	EmailNotifications bool
	SlackWebhook       string
}

// PerformanceConfig holds performance tracking configuration
type PerformanceConfig struct {
	MaxSamples       int
	TrackingEnabled  bool
	HistogramBuckets []float64
}

// DefaultSystemConfig returns a default system configuration
func DefaultSystemConfig() *SystemConfig {
	return &SystemConfig{
		Cache:     DefaultConfig(),
		Dashboard: DefaultDashboardConfig(),
		Monitoring: MonitoringConfig{
			Interval:        time.Second * 30,
			MemoryThreshold: 0.85,
			HeapThreshold:   0.80,
			GCPressureLimit: 5.0,
			EnableProfiling: false,
		},
		Alerting: AlertingConfig{
			MaxAlerts:          100,
			RetentionPeriod:    time.Hour * 24,
			EmailNotifications: false,
			SlackWebhook:       "",
		},
		Performance: PerformanceConfig{
			MaxSamples:      1000,
			TrackingEnabled: true,
			HistogramBuckets: []float64{
				0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0,
			},
		},
	}
}

// NewMemorySystem creates a new integrated memory management system
func NewMemorySystem(config *SystemConfig) (*MemorySystem, error) {
	if config == nil {
		config = DefaultSystemConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Create cache manager
	cacheManager := NewCacheManager(config.Cache)

	// Create memory monitor
	monitor := NewMemoryMonitor(config.Monitoring.Interval)

	// Create alert manager
	alertManager := NewAlertManager(config.Alerting.MaxAlerts)

	// Create metrics collector
	collector := NewMetricsCollector(cacheManager, monitor, alertManager)

	// Create health checker
	healthChecker := NewHealthChecker(cacheManager, monitor, collector)

	// Create performance tracker
	var performanceTracker *PerformanceTracker
	if config.Performance.TrackingEnabled {
		performanceTracker = NewPerformanceTracker(config.Performance.MaxSamples)
	}

	// Create dashboard
	dashboard := NewDashboard(
		cacheManager,
		monitor,
		collector,
		healthChecker,
		performanceTracker,
		alertManager,
		config.Dashboard,
	)

	system := &MemorySystem{
		cacheManager:       cacheManager,
		monitor:            monitor,
		collector:          collector,
		healthChecker:      healthChecker,
		performanceTracker: performanceTracker,
		alertManager:       alertManager,
		dashboard:          dashboard,
		config:             config,
		ctx:                ctx,
		cancel:             cancel,
	}

	return system, nil
}

// Start initializes and starts all memory management components
func (ms *MemorySystem) Start() error {
	log.Println("Starting MantisDB Memory Management System...")

	// Start memory monitor
	ms.monitor.Start()
	log.Println("Memory monitor started")

	// Set up memory thresholds and alerts
	ms.setupMemoryAlerts()

	// Start metrics collector
	ms.collector.Start(ms.ctx, time.Second*10)
	log.Println("Metrics collector started")

	// Start dashboard
	go func() {
		if err := ms.dashboard.Start(ms.ctx); err != nil {
			log.Printf("Dashboard server error: %v", err)
		}
	}()
	log.Printf("Dashboard started on port %d", ms.config.Dashboard.Port)

	log.Println("Memory Management System fully initialized")
	return nil
}

// Stop gracefully shuts down all components
func (ms *MemorySystem) Stop() error {
	log.Println("Stopping Memory Management System...")

	ms.cancel()

	// Stop dashboard
	if err := ms.dashboard.Stop(); err != nil {
		log.Printf("Error stopping dashboard: %v", err)
	}

	// Stop metrics collector
	ms.collector.Stop()

	// Stop memory monitor
	ms.monitor.Stop()

	// Close cache manager
	if err := ms.cacheManager.Close(); err != nil {
		log.Printf("Error closing cache manager: %v", err)
	}

	log.Println("Memory Management System stopped")
	return nil
}

// GetCacheManager returns the cache manager instance
func (ms *MemorySystem) GetCacheManager() *CacheManager {
	return ms.cacheManager
}

// GetMonitor returns the memory monitor instance
func (ms *MemorySystem) GetMonitor() *MemoryMonitor {
	return ms.monitor
}

// GetHealthChecker returns the health checker instance
func (ms *MemorySystem) GetHealthChecker() *HealthChecker {
	return ms.healthChecker
}

// GetMetrics returns current system metrics
func (ms *MemorySystem) GetMetrics() *Metrics {
	return ms.collector.GetMetrics()
}

// GetHealthStatus returns current health status
func (ms *MemorySystem) GetHealthStatus() *HealthStatus {
	return ms.healthChecker.CheckHealth()
}

// RecordCacheAccess records a cache access for performance tracking
func (ms *MemorySystem) RecordCacheAccess(duration time.Duration) {
	if ms.performanceTracker != nil {
		ms.performanceTracker.RecordAccess(duration)
	}
}

// Private methods

func (ms *MemorySystem) setupMemoryAlerts() {
	// Memory usage alert
	ms.monitor.SetThreshold("memory", ms.config.Monitoring.MemoryThreshold, func(stats *MemoryStats, threshold float64) {
		alert := MemoryAlert{
			Name:      "High Memory Usage",
			Threshold: threshold,
			Current:   stats.MemoryUsage,
			Timestamp: time.Now(),
			Stats:     stats,
		}
		ms.alertManager.AddAlert(alert)
		log.Printf("ALERT: High memory usage - %.1f%% (threshold: %.1f%%)",
			stats.MemoryUsage*100, threshold*100)
	})

	// Heap usage alert
	ms.monitor.SetThreshold("heap", ms.config.Monitoring.HeapThreshold, func(stats *MemoryStats, threshold float64) {
		alert := MemoryAlert{
			Name:      "High Heap Usage",
			Threshold: threshold,
			Current:   stats.HeapUsage,
			Timestamp: time.Now(),
			Stats:     stats,
		}
		ms.alertManager.AddAlert(alert)
		log.Printf("ALERT: High heap usage - %.1f%% (threshold: %.1f%%)",
			stats.HeapUsage*100, threshold*100)
	})

	// GC pressure alert
	ms.monitor.SetThreshold("gc_pressure", ms.config.Monitoring.GCPressureLimit, func(stats *MemoryStats, threshold float64) {
		alert := MemoryAlert{
			Name:      "High GC Pressure",
			Threshold: threshold,
			Current:   stats.GCPressure,
			Timestamp: time.Now(),
			Stats:     stats,
		}
		ms.alertManager.AddAlert(alert)
		log.Printf("ALERT: High GC pressure - %.2f GCs/min (threshold: %.2f)",
			stats.GCPressure, threshold)
	})
}

// Example usage and integration helpers

// ExampleUsage demonstrates how to use the memory system
func ExampleUsage() {
	// Create system with default configuration
	system, err := NewMemorySystem(nil)
	if err != nil {
		log.Fatal(err)
	}

	// Start the system
	if err := system.Start(); err != nil {
		log.Fatal(err)
	}

	// Use the cache
	cache := system.GetCacheManager()

	// Put some data
	ctx := context.Background()
	cache.Put(ctx, "key1", "value1", time.Minute*5)
	cache.Put(ctx, "key2", []byte("binary data"), time.Hour)

	// Get data and record performance
	start := time.Now()
	value, found := cache.Get(ctx, "key1")
	duration := time.Since(start)

	if found {
		fmt.Printf("Retrieved: %v\n", value)
	}

	// Record the access time for performance tracking
	system.RecordCacheAccess(duration)

	// Check system health
	health := system.GetHealthStatus()
	fmt.Printf("System healthy: %v\n", health.Healthy)

	// Get metrics
	metrics := system.GetMetrics()
	fmt.Printf("Cache hit ratio: %.2f%%\n", metrics.CacheHitRatio*100)
	fmt.Printf("Memory usage: %.2f%%\n", metrics.MemoryUsage*100)

	// Dashboard is available at http://localhost:8090
	fmt.Println("Dashboard available at http://localhost:8090")

	// Graceful shutdown
	defer system.Stop()
}

// ConfigurationExample shows how to customize the system configuration
func ConfigurationExample() *SystemConfig {
	config := &SystemConfig{
		Cache: &Config{
			MaxSize:         2 * 1024 * 1024 * 1024, // 2GB
			MaxEntries:      500000,
			MemoryThreshold: 0.75, // 75%
			CheckInterval:   time.Second * 15,
			DefaultPolicy:   "adaptive",
			CleanupInterval: time.Minute * 2,
		},
		Dashboard: &DashboardConfig{
			Port:            8091,
			RefreshInterval: time.Second * 3,
			MaxDataPoints:   200,
			EnableProfiling: true,
			EnableDebug:     true,
		},
		Monitoring: MonitoringConfig{
			Interval:        time.Second * 15,
			MemoryThreshold: 0.80, // 80%
			HeapThreshold:   0.75, // 75%
			GCPressureLimit: 3.0,  // 3 GCs per minute
			EnableProfiling: true,
		},
		Alerting: AlertingConfig{
			MaxAlerts:          200,
			RetentionPeriod:    time.Hour * 48,
			EmailNotifications: true,
			SlackWebhook:       "https://hooks.slack.com/services/...",
		},
		Performance: PerformanceConfig{
			MaxSamples:      2000,
			TrackingEnabled: true,
			HistogramBuckets: []float64{
				0.0001, 0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0,
			},
		},
	}

	return config
}
