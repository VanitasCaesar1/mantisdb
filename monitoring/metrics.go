package monitoring

import (
	"sync"
	"sync/atomic"
	"time"
)

// MetricType represents the type of metric
type MetricType int

const (
	CounterType MetricType = iota
	GaugeType
	HistogramType
	TimerType
)

// Metric represents a single metric
type Metric struct {
	Name        string
	Type        MetricType
	Value       int64
	Labels      map[string]string
	Timestamp   time.Time
	Description string
}

// MetricsCollector handles collection and aggregation of metrics
type MetricsCollector struct {
	metrics map[string]*Metric
	mutex   sync.RWMutex

	// WAL metrics
	walWriteCount    int64
	walWriteErrors   int64
	walSyncLatency   int64
	walFileRotations int64
	walRecoveryTime  int64

	// Transaction metrics
	txnStartCount   int64
	txnCommitCount  int64
	txnAbortCount   int64
	txnDeadlocks    int64
	txnLockWaitTime int64

	// Error metrics
	errorCount       int64
	corruptionEvents int64
	recoveryAttempts int64
	recoveryFailures int64

	// Performance metrics
	operationLatency int64
	throughput       int64
	memoryUsage      int64
	diskUsage        int64
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		metrics: make(map[string]*Metric),
	}
}

// Counter operations
func (mc *MetricsCollector) IncrementCounter(name string, labels map[string]string) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	key := mc.buildKey(name, labels)
	if metric, exists := mc.metrics[key]; exists {
		atomic.AddInt64(&metric.Value, 1)
		metric.Timestamp = time.Now()
	} else {
		mc.metrics[key] = &Metric{
			Name:      name,
			Type:      CounterType,
			Value:     1,
			Labels:    labels,
			Timestamp: time.Now(),
		}
	}
}

func (mc *MetricsCollector) AddToCounter(name string, value int64, labels map[string]string) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	key := mc.buildKey(name, labels)
	if metric, exists := mc.metrics[key]; exists {
		atomic.AddInt64(&metric.Value, value)
		metric.Timestamp = time.Now()
	} else {
		mc.metrics[key] = &Metric{
			Name:      name,
			Type:      CounterType,
			Value:     value,
			Labels:    labels,
			Timestamp: time.Now(),
		}
	}
}

// Gauge operations
func (mc *MetricsCollector) SetGauge(name string, value int64, labels map[string]string) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	key := mc.buildKey(name, labels)
	mc.metrics[key] = &Metric{
		Name:      name,
		Type:      GaugeType,
		Value:     value,
		Labels:    labels,
		Timestamp: time.Now(),
	}
}

// Timer operations
func (mc *MetricsCollector) RecordTimer(name string, duration time.Duration, labels map[string]string) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	key := mc.buildKey(name, labels)
	mc.metrics[key] = &Metric{
		Name:      name,
		Type:      TimerType,
		Value:     duration.Nanoseconds(),
		Labels:    labels,
		Timestamp: time.Now(),
	}
}

// WAL-specific metrics
func (mc *MetricsCollector) RecordWALWrite() {
	atomic.AddInt64(&mc.walWriteCount, 1)
	mc.IncrementCounter("wal_writes_total", nil)
}

func (mc *MetricsCollector) RecordWALWriteError() {
	atomic.AddInt64(&mc.walWriteErrors, 1)
	mc.IncrementCounter("wal_write_errors_total", nil)
}

func (mc *MetricsCollector) RecordWALSyncLatency(duration time.Duration) {
	atomic.StoreInt64(&mc.walSyncLatency, duration.Nanoseconds())
	mc.RecordTimer("wal_sync_duration", duration, nil)
}

func (mc *MetricsCollector) RecordWALFileRotation() {
	atomic.AddInt64(&mc.walFileRotations, 1)
	mc.IncrementCounter("wal_file_rotations_total", nil)
}

func (mc *MetricsCollector) RecordWALRecoveryTime(duration time.Duration) {
	atomic.StoreInt64(&mc.walRecoveryTime, duration.Nanoseconds())
	mc.RecordTimer("wal_recovery_duration", duration, nil)
}

// Transaction-specific metrics
func (mc *MetricsCollector) RecordTransactionStart() {
	atomic.AddInt64(&mc.txnStartCount, 1)
	mc.IncrementCounter("transactions_started_total", nil)
}

func (mc *MetricsCollector) RecordTransactionCommit() {
	atomic.AddInt64(&mc.txnCommitCount, 1)
	mc.IncrementCounter("transactions_committed_total", nil)
}

func (mc *MetricsCollector) RecordTransactionAbort() {
	atomic.AddInt64(&mc.txnAbortCount, 1)
	mc.IncrementCounter("transactions_aborted_total", nil)
}

func (mc *MetricsCollector) RecordDeadlock() {
	atomic.AddInt64(&mc.txnDeadlocks, 1)
	mc.IncrementCounter("transaction_deadlocks_total", nil)
}

func (mc *MetricsCollector) RecordLockWaitTime(duration time.Duration) {
	atomic.StoreInt64(&mc.txnLockWaitTime, duration.Nanoseconds())
	mc.RecordTimer("lock_wait_duration", duration, nil)
}

// Error-specific metrics
func (mc *MetricsCollector) RecordError(errorType string) {
	atomic.AddInt64(&mc.errorCount, 1)
	mc.IncrementCounter("errors_total", map[string]string{"type": errorType})
}

func (mc *MetricsCollector) RecordCorruptionEvent() {
	atomic.AddInt64(&mc.corruptionEvents, 1)
	mc.IncrementCounter("corruption_events_total", nil)
}

func (mc *MetricsCollector) RecordRecoveryAttempt() {
	atomic.AddInt64(&mc.recoveryAttempts, 1)
	mc.IncrementCounter("recovery_attempts_total", nil)
}

func (mc *MetricsCollector) RecordRecoveryFailure() {
	atomic.AddInt64(&mc.recoveryFailures, 1)
	mc.IncrementCounter("recovery_failures_total", nil)
}

// Performance metrics
func (mc *MetricsCollector) RecordOperationLatency(operation string, duration time.Duration) {
	atomic.StoreInt64(&mc.operationLatency, duration.Nanoseconds())
	mc.RecordTimer("operation_duration", duration, map[string]string{"operation": operation})
}

func (mc *MetricsCollector) SetThroughput(ops int64) {
	atomic.StoreInt64(&mc.throughput, ops)
	mc.SetGauge("throughput_ops_per_second", ops, nil)
}

func (mc *MetricsCollector) SetMemoryUsage(bytes int64) {
	atomic.StoreInt64(&mc.memoryUsage, bytes)
	mc.SetGauge("memory_usage_bytes", bytes, nil)
}

func (mc *MetricsCollector) SetDiskUsage(bytes int64) {
	atomic.StoreInt64(&mc.diskUsage, bytes)
	mc.SetGauge("disk_usage_bytes", bytes, nil)
}

// GetMetrics returns all collected metrics
func (mc *MetricsCollector) GetMetrics() map[string]*Metric {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()

	result := make(map[string]*Metric)
	for k, v := range mc.metrics {
		result[k] = &Metric{
			Name:      v.Name,
			Type:      v.Type,
			Value:     atomic.LoadInt64(&v.Value),
			Labels:    v.Labels,
			Timestamp: v.Timestamp,
		}
	}
	return result
}

// GetMetric returns a specific metric
func (mc *MetricsCollector) GetMetric(name string, labels map[string]string) *Metric {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()

	key := mc.buildKey(name, labels)
	if metric, exists := mc.metrics[key]; exists {
		return &Metric{
			Name:      metric.Name,
			Type:      metric.Type,
			Value:     atomic.LoadInt64(&metric.Value),
			Labels:    metric.Labels,
			Timestamp: metric.Timestamp,
		}
	}
	return nil
}

// GetSummaryMetrics returns key performance indicators
func (mc *MetricsCollector) GetSummaryMetrics() map[string]int64 {
	return map[string]int64{
		"wal_writes_total":             atomic.LoadInt64(&mc.walWriteCount),
		"wal_write_errors_total":       atomic.LoadInt64(&mc.walWriteErrors),
		"wal_sync_latency_ns":          atomic.LoadInt64(&mc.walSyncLatency),
		"wal_file_rotations_total":     atomic.LoadInt64(&mc.walFileRotations),
		"wal_recovery_time_ns":         atomic.LoadInt64(&mc.walRecoveryTime),
		"transactions_started_total":   atomic.LoadInt64(&mc.txnStartCount),
		"transactions_committed_total": atomic.LoadInt64(&mc.txnCommitCount),
		"transactions_aborted_total":   atomic.LoadInt64(&mc.txnAbortCount),
		"transaction_deadlocks_total":  atomic.LoadInt64(&mc.txnDeadlocks),
		"lock_wait_time_ns":            atomic.LoadInt64(&mc.txnLockWaitTime),
		"errors_total":                 atomic.LoadInt64(&mc.errorCount),
		"corruption_events_total":      atomic.LoadInt64(&mc.corruptionEvents),
		"recovery_attempts_total":      atomic.LoadInt64(&mc.recoveryAttempts),
		"recovery_failures_total":      atomic.LoadInt64(&mc.recoveryFailures),
		"operation_latency_ns":         atomic.LoadInt64(&mc.operationLatency),
		"throughput_ops_per_second":    atomic.LoadInt64(&mc.throughput),
		"memory_usage_bytes":           atomic.LoadInt64(&mc.memoryUsage),
		"disk_usage_bytes":             atomic.LoadInt64(&mc.diskUsage),
	}
}

// Reset clears all metrics
func (mc *MetricsCollector) Reset() {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.metrics = make(map[string]*Metric)

	// Reset atomic counters
	atomic.StoreInt64(&mc.walWriteCount, 0)
	atomic.StoreInt64(&mc.walWriteErrors, 0)
	atomic.StoreInt64(&mc.walSyncLatency, 0)
	atomic.StoreInt64(&mc.walFileRotations, 0)
	atomic.StoreInt64(&mc.walRecoveryTime, 0)
	atomic.StoreInt64(&mc.txnStartCount, 0)
	atomic.StoreInt64(&mc.txnCommitCount, 0)
	atomic.StoreInt64(&mc.txnAbortCount, 0)
	atomic.StoreInt64(&mc.txnDeadlocks, 0)
	atomic.StoreInt64(&mc.txnLockWaitTime, 0)
	atomic.StoreInt64(&mc.errorCount, 0)
	atomic.StoreInt64(&mc.corruptionEvents, 0)
	atomic.StoreInt64(&mc.recoveryAttempts, 0)
	atomic.StoreInt64(&mc.recoveryFailures, 0)
	atomic.StoreInt64(&mc.operationLatency, 0)
	atomic.StoreInt64(&mc.throughput, 0)
	atomic.StoreInt64(&mc.memoryUsage, 0)
	atomic.StoreInt64(&mc.diskUsage, 0)
}

// buildKey creates a unique key for a metric with labels
func (mc *MetricsCollector) buildKey(name string, labels map[string]string) string {
	key := name
	if labels != nil {
		for k, v := range labels {
			key += ":" + k + "=" + v
		}
	}
	return key
}
