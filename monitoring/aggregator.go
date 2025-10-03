package monitoring

import (
	"context"
	"runtime"
	"sync"
	"time"
)

// MetricsAggregator collects metrics from various system components
type MetricsAggregator struct {
	collector *MetricsCollector
	exporter  *MetricsExporter

	// Collection intervals
	systemMetricsInterval time.Duration
	performanceInterval   time.Duration

	// Control channels
	stopChan chan struct{}
	doneChan chan struct{}

	// Component interfaces for metric collection
	walMetrics         WALMetricsProvider
	transactionMetrics TransactionMetricsProvider
	errorMetrics       ErrorMetricsProvider
	storageMetrics     StorageMetricsProvider

	mutex sync.RWMutex
}

// WALMetricsProvider interface for WAL components to provide metrics
type WALMetricsProvider interface {
	GetWALStats() WALStats
}

// TransactionMetricsProvider interface for transaction components
type TransactionMetricsProvider interface {
	GetTransactionStats() TransactionStats
}

// ErrorMetricsProvider interface for error handling components
type ErrorMetricsProvider interface {
	GetErrorStats() ErrorStats
}

// StorageMetricsProvider interface for storage components
type StorageMetricsProvider interface {
	GetStorageStats() StorageStats
}

// Stats structures
type WALStats struct {
	WritesTotal     int64
	WriteErrors     int64
	SyncLatencyNs   int64
	FileRotations   int64
	RecoveryTimeNs  int64
	CurrentFileSize int64
	TotalFilesCount int64
}

type TransactionStats struct {
	StartedTotal     int64
	CommittedTotal   int64
	AbortedTotal     int64
	DeadlocksTotal   int64
	LockWaitTimeNs   int64
	ActiveCount      int64
	LongestRunningNs int64
}

type ErrorStats struct {
	ErrorsTotal      int64
	CorruptionEvents int64
	RecoveryAttempts int64
	RecoveryFailures int64
	IOErrors         int64
	MemoryErrors     int64
	DiskErrors       int64
}

type StorageStats struct {
	DiskUsageBytes    int64
	MemoryUsageBytes  int64
	CacheHitRate      float64
	IOOperationsTotal int64
	IOLatencyNs       int64
}

// NewMetricsAggregator creates a new metrics aggregator
func NewMetricsAggregator() *MetricsAggregator {
	collector := NewMetricsCollector()
	exporter := NewMetricsExporter(collector)

	return &MetricsAggregator{
		collector:             collector,
		exporter:              exporter,
		systemMetricsInterval: 30 * time.Second,
		performanceInterval:   5 * time.Second,
		stopChan:              make(chan struct{}),
		doneChan:              make(chan struct{}),
	}
}

// RegisterWALMetrics registers a WAL metrics provider
func (ma *MetricsAggregator) RegisterWALMetrics(provider WALMetricsProvider) {
	ma.mutex.Lock()
	defer ma.mutex.Unlock()
	ma.walMetrics = provider
}

// RegisterTransactionMetrics registers a transaction metrics provider
func (ma *MetricsAggregator) RegisterTransactionMetrics(provider TransactionMetricsProvider) {
	ma.mutex.Lock()
	defer ma.mutex.Unlock()
	ma.transactionMetrics = provider
}

// RegisterErrorMetrics registers an error metrics provider
func (ma *MetricsAggregator) RegisterErrorMetrics(provider ErrorMetricsProvider) {
	ma.mutex.Lock()
	defer ma.mutex.Unlock()
	ma.errorMetrics = provider
}

// RegisterStorageMetrics registers a storage metrics provider
func (ma *MetricsAggregator) RegisterStorageMetrics(provider StorageMetricsProvider) {
	ma.mutex.Lock()
	defer ma.mutex.Unlock()
	ma.storageMetrics = provider
}

// Start begins metric collection
func (ma *MetricsAggregator) Start(ctx context.Context) {
	go ma.collectMetrics(ctx)
}

// Stop stops metric collection
func (ma *MetricsAggregator) Stop() {
	close(ma.stopChan)
	<-ma.doneChan
}

// GetCollector returns the metrics collector
func (ma *MetricsAggregator) GetCollector() *MetricsCollector {
	return ma.collector
}

// GetExporter returns the metrics exporter
func (ma *MetricsAggregator) GetExporter() *MetricsExporter {
	return ma.exporter
}

// collectMetrics runs the metric collection loop
func (ma *MetricsAggregator) collectMetrics(ctx context.Context) {
	defer close(ma.doneChan)

	systemTicker := time.NewTicker(ma.systemMetricsInterval)
	performanceTicker := time.NewTicker(ma.performanceInterval)

	defer systemTicker.Stop()
	defer performanceTicker.Stop()

	// Collect initial metrics
	ma.collectAllMetrics()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ma.stopChan:
			return
		case <-systemTicker.C:
			ma.collectSystemMetrics()
		case <-performanceTicker.C:
			ma.collectPerformanceMetrics()
		}
	}
}

// collectAllMetrics collects metrics from all registered providers
func (ma *MetricsAggregator) collectAllMetrics() {
	ma.collectSystemMetrics()
	ma.collectPerformanceMetrics()
	ma.collectComponentMetrics()
}

// collectSystemMetrics collects system-level metrics
func (ma *MetricsAggregator) collectSystemMetrics() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Memory metrics
	ma.collector.SetMemoryUsage(int64(memStats.Alloc))
	ma.collector.SetGauge("memory_heap_alloc_bytes", int64(memStats.HeapAlloc), nil)
	ma.collector.SetGauge("memory_heap_sys_bytes", int64(memStats.HeapSys), nil)
	ma.collector.SetGauge("memory_gc_runs_total", int64(memStats.NumGC), nil)
	ma.collector.SetGauge("memory_gc_pause_ns", int64(memStats.PauseNs[(memStats.NumGC+255)%256]), nil)

	// Goroutine metrics
	ma.collector.SetGauge("goroutines_count", int64(runtime.NumGoroutine()), nil)

	// CPU metrics (simplified)
	ma.collector.SetGauge("cpu_cores", int64(runtime.NumCPU()), nil)
}

// collectPerformanceMetrics collects performance-related metrics
func (ma *MetricsAggregator) collectPerformanceMetrics() {
	ma.mutex.RLock()
	defer ma.mutex.RUnlock()

	// Collect from registered providers
	if ma.storageMetrics != nil {
		stats := ma.storageMetrics.GetStorageStats()
		ma.collector.SetDiskUsage(stats.DiskUsageBytes)
		ma.collector.SetGauge("cache_hit_rate", int64(stats.CacheHitRate*100), nil)
		ma.collector.SetGauge("io_operations_total", stats.IOOperationsTotal, nil)
		ma.collector.RecordTimer("io_latency", time.Duration(stats.IOLatencyNs), nil)
	}
}

// collectComponentMetrics collects metrics from all registered component providers
func (ma *MetricsAggregator) collectComponentMetrics() {
	ma.mutex.RLock()
	defer ma.mutex.RUnlock()

	// WAL metrics
	if ma.walMetrics != nil {
		stats := ma.walMetrics.GetWALStats()
		ma.collector.SetGauge("wal_writes_total", stats.WritesTotal, nil)
		ma.collector.SetGauge("wal_write_errors_total", stats.WriteErrors, nil)
		ma.collector.SetGauge("wal_sync_latency_ns", stats.SyncLatencyNs, nil)
		ma.collector.SetGauge("wal_file_rotations_total", stats.FileRotations, nil)
		ma.collector.SetGauge("wal_recovery_time_ns", stats.RecoveryTimeNs, nil)
		ma.collector.SetGauge("wal_current_file_size_bytes", stats.CurrentFileSize, nil)
		ma.collector.SetGauge("wal_total_files_count", stats.TotalFilesCount, nil)
	}

	// Transaction metrics
	if ma.transactionMetrics != nil {
		stats := ma.transactionMetrics.GetTransactionStats()
		ma.collector.SetGauge("transactions_started_total", stats.StartedTotal, nil)
		ma.collector.SetGauge("transactions_committed_total", stats.CommittedTotal, nil)
		ma.collector.SetGauge("transactions_aborted_total", stats.AbortedTotal, nil)
		ma.collector.SetGauge("transaction_deadlocks_total", stats.DeadlocksTotal, nil)
		ma.collector.SetGauge("lock_wait_time_ns", stats.LockWaitTimeNs, nil)
		ma.collector.SetGauge("transactions_active_count", stats.ActiveCount, nil)
		ma.collector.SetGauge("transaction_longest_running_ns", stats.LongestRunningNs, nil)
	}

	// Error metrics
	if ma.errorMetrics != nil {
		stats := ma.errorMetrics.GetErrorStats()
		ma.collector.SetGauge("errors_total", stats.ErrorsTotal, nil)
		ma.collector.SetGauge("corruption_events_total", stats.CorruptionEvents, nil)
		ma.collector.SetGauge("recovery_attempts_total", stats.RecoveryAttempts, nil)
		ma.collector.SetGauge("recovery_failures_total", stats.RecoveryFailures, nil)
		ma.collector.SetGauge("io_errors_total", stats.IOErrors, nil)
		ma.collector.SetGauge("memory_errors_total", stats.MemoryErrors, nil)
		ma.collector.SetGauge("disk_errors_total", stats.DiskErrors, nil)
	}
}

// SetCollectionIntervals sets the collection intervals
func (ma *MetricsAggregator) SetCollectionIntervals(systemInterval, performanceInterval time.Duration) {
	ma.mutex.Lock()
	defer ma.mutex.Unlock()

	ma.systemMetricsInterval = systemInterval
	ma.performanceInterval = performanceInterval
}

// GetHealthStatus returns the current health status based on metrics
func (ma *MetricsAggregator) GetHealthStatus() HealthStatus {
	summary := ma.collector.GetSummaryMetrics()

	status := HealthStatus{
		Overall:    HealthStatusHealthy,
		Components: make(map[string]ComponentHealth),
		Timestamp:  time.Now(),
	}

	// Check WAL health
	walHealth := ma.checkWALHealth(summary)
	status.Components["wal"] = walHealth
	if walHealth.Status != HealthStatusHealthy {
		status.Overall = HealthStatusDegraded
	}

	// Check transaction health
	txnHealth := ma.checkTransactionHealth(summary)
	status.Components["transactions"] = txnHealth
	if txnHealth.Status == HealthStatusUnhealthy {
		status.Overall = HealthStatusUnhealthy
	} else if txnHealth.Status == HealthStatusDegraded && status.Overall == HealthStatusHealthy {
		status.Overall = HealthStatusDegraded
	}

	// Check error health
	errorHealth := ma.checkErrorHealth(summary)
	status.Components["errors"] = errorHealth
	if errorHealth.Status == HealthStatusUnhealthy {
		status.Overall = HealthStatusUnhealthy
	} else if errorHealth.Status == HealthStatusDegraded && status.Overall == HealthStatusHealthy {
		status.Overall = HealthStatusDegraded
	}

	return status
}

// Health status types
type HealthStatusType int

const (
	HealthStatusHealthy HealthStatusType = iota
	HealthStatusDegraded
	HealthStatusUnhealthy
)

type HealthStatus struct {
	Overall    HealthStatusType           `json:"overall"`
	Components map[string]ComponentHealth `json:"components"`
	Timestamp  time.Time                  `json:"timestamp"`
}

type ComponentHealth struct {
	Status  HealthStatusType `json:"status"`
	Message string           `json:"message"`
	Metrics map[string]int64 `json:"metrics"`
}

// checkWALHealth checks WAL component health
func (ma *MetricsAggregator) checkWALHealth(summary map[string]int64) ComponentHealth {
	walWrites := summary["wal_writes_total"]
	walErrors := summary["wal_write_errors_total"]

	health := ComponentHealth{
		Status: HealthStatusHealthy,
		Metrics: map[string]int64{
			"writes_total": walWrites,
			"errors_total": walErrors,
		},
	}

	if walWrites > 0 {
		errorRate := float64(walErrors) / float64(walWrites)
		if errorRate > 0.1 { // 10% error rate
			health.Status = HealthStatusUnhealthy
			health.Message = "High WAL error rate"
		} else if errorRate > 0.05 { // 5% error rate
			health.Status = HealthStatusDegraded
			health.Message = "Elevated WAL error rate"
		}
	}

	return health
}

// checkTransactionHealth checks transaction component health
func (ma *MetricsAggregator) checkTransactionHealth(summary map[string]int64) ComponentHealth {
	txnStarted := summary["transactions_started_total"]
	txnAborted := summary["transactions_aborted_total"]
	deadlocks := summary["transaction_deadlocks_total"]

	health := ComponentHealth{
		Status: HealthStatusHealthy,
		Metrics: map[string]int64{
			"started_total":   txnStarted,
			"aborted_total":   txnAborted,
			"deadlocks_total": deadlocks,
		},
	}

	if txnStarted > 0 {
		abortRate := float64(txnAborted) / float64(txnStarted)
		if abortRate > 0.2 { // 20% abort rate
			health.Status = HealthStatusDegraded
			health.Message = "High transaction abort rate"
		}
	}

	if deadlocks > 10 {
		health.Status = HealthStatusDegraded
		health.Message = "High deadlock frequency"
	}

	return health
}

// checkErrorHealth checks error component health
func (ma *MetricsAggregator) checkErrorHealth(summary map[string]int64) ComponentHealth {
	totalErrors := summary["errors_total"]
	corruptionEvents := summary["corruption_events_total"]
	recoveryFailures := summary["recovery_failures_total"]

	health := ComponentHealth{
		Status: HealthStatusHealthy,
		Metrics: map[string]int64{
			"errors_total":            totalErrors,
			"corruption_events_total": corruptionEvents,
			"recovery_failures_total": recoveryFailures,
		},
	}

	if corruptionEvents > 0 {
		health.Status = HealthStatusUnhealthy
		health.Message = "Data corruption detected"
	} else if recoveryFailures > 0 {
		health.Status = HealthStatusDegraded
		health.Message = "Recovery failures detected"
	} else if totalErrors > 100 {
		health.Status = HealthStatusDegraded
		health.Message = "High error count"
	}

	return health
}
