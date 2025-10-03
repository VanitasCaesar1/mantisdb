package metrics

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// PrometheusMetrics provides Prometheus-compatible metrics collection and export
type PrometheusMetrics struct {
	// Core metrics
	queryDuration     *HistogramMetric
	queryCount        *CounterMetric
	activeConnections *GaugeMetric
	throughput        *GaugeMetric
	errorCount        *CounterMetric

	// Resource metrics
	memoryUsage *GaugeMetric
	diskUsage   *GaugeMetric
	cpuUsage    *GaugeMetric

	// Database-specific metrics
	walWrites        *CounterMetric
	walSyncDuration  *HistogramMetric
	transactionCount *CounterMetric
	lockWaitDuration *HistogramMetric
	cacheHitRatio    *GaugeMetric

	// System metrics
	uptime    *GaugeMetric
	startTime time.Time

	mutex sync.RWMutex
}

// MetricType represents different types of Prometheus metrics
type MetricType string

const (
	CounterMetricType   MetricType = "counter"
	GaugeMetricType     MetricType = "gauge"
	HistogramMetricType MetricType = "histogram"
)

// Metric represents a Prometheus metric
type Metric interface {
	Name() string
	Type() MetricType
	Help() string
	Export() string
}

// CounterMetric represents a Prometheus counter
type CounterMetric struct {
	name   string
	help   string
	value  float64
	labels map[string]string
	mutex  sync.RWMutex
}

// GaugeMetric represents a Prometheus gauge
type GaugeMetric struct {
	name   string
	help   string
	value  float64
	labels map[string]string
	mutex  sync.RWMutex
}

// HistogramMetric represents a Prometheus histogram
type HistogramMetric struct {
	name    string
	help    string
	buckets []float64
	counts  []uint64
	sum     float64
	count   uint64
	labels  map[string]string
	mutex   sync.RWMutex
}

// NewPrometheusMetrics creates a new Prometheus metrics collector
func NewPrometheusMetrics() *PrometheusMetrics {
	pm := &PrometheusMetrics{
		startTime: time.Now(),
	}

	pm.initializeMetrics()
	return pm
}

// initializeMetrics sets up all the metrics with their definitions
func (pm *PrometheusMetrics) initializeMetrics() {
	// Query metrics
	pm.queryDuration = NewHistogramMetric(
		"mantisdb_query_duration_seconds",
		"Time spent executing queries",
		[]float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
		map[string]string{},
	)

	pm.queryCount = NewCounterMetric(
		"mantisdb_queries_total",
		"Total number of queries executed",
		map[string]string{},
	)

	pm.activeConnections = NewGaugeMetric(
		"mantisdb_active_connections",
		"Number of active database connections",
		map[string]string{},
	)

	pm.throughput = NewGaugeMetric(
		"mantisdb_throughput_ops_per_second",
		"Current throughput in operations per second",
		map[string]string{},
	)

	pm.errorCount = NewCounterMetric(
		"mantisdb_errors_total",
		"Total number of errors",
		map[string]string{},
	)

	// Resource metrics
	pm.memoryUsage = NewGaugeMetric(
		"mantisdb_memory_usage_bytes",
		"Current memory usage in bytes",
		map[string]string{},
	)

	pm.diskUsage = NewGaugeMetric(
		"mantisdb_disk_usage_bytes",
		"Current disk usage in bytes",
		map[string]string{},
	)

	pm.cpuUsage = NewGaugeMetric(
		"mantisdb_cpu_usage_percent",
		"Current CPU usage percentage",
		map[string]string{},
	)

	// Database-specific metrics
	pm.walWrites = NewCounterMetric(
		"mantisdb_wal_writes_total",
		"Total number of WAL writes",
		map[string]string{},
	)

	pm.walSyncDuration = NewHistogramMetric(
		"mantisdb_wal_sync_duration_seconds",
		"Time spent syncing WAL to disk",
		[]float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
		map[string]string{},
	)

	pm.transactionCount = NewCounterMetric(
		"mantisdb_transactions_total",
		"Total number of transactions",
		map[string]string{},
	)

	pm.lockWaitDuration = NewHistogramMetric(
		"mantisdb_lock_wait_duration_seconds",
		"Time spent waiting for locks",
		[]float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
		map[string]string{},
	)

	pm.cacheHitRatio = NewGaugeMetric(
		"mantisdb_cache_hit_ratio",
		"Cache hit ratio (0.0 to 1.0)",
		map[string]string{},
	)

	// System metrics
	pm.uptime = NewGaugeMetric(
		"mantisdb_uptime_seconds",
		"System uptime in seconds",
		map[string]string{},
	)
}

// RecordQueryDuration records the duration of a query execution
func (pm *PrometheusMetrics) RecordQueryDuration(duration time.Duration, operation string, table string) {
	labels := map[string]string{
		"operation": operation,
		"table":     table,
	}
	pm.queryDuration.ObserveWithLabels(duration.Seconds(), labels)
	pm.queryCount.IncWithLabels(labels)
}

// SetActiveConnections sets the current number of active connections
func (pm *PrometheusMetrics) SetActiveConnections(count int) {
	pm.activeConnections.Set(float64(count))
}

// SetThroughput sets the current throughput
func (pm *PrometheusMetrics) SetThroughput(opsPerSecond float64) {
	pm.throughput.Set(opsPerSecond)
}

// RecordError records an error occurrence
func (pm *PrometheusMetrics) RecordError(errorType string, component string) {
	labels := map[string]string{
		"type":      errorType,
		"component": component,
	}
	pm.errorCount.IncWithLabels(labels)
}

// SetMemoryUsage sets the current memory usage
func (pm *PrometheusMetrics) SetMemoryUsage(bytes int64) {
	pm.memoryUsage.Set(float64(bytes))
}

// SetDiskUsage sets the current disk usage
func (pm *PrometheusMetrics) SetDiskUsage(bytes int64) {
	pm.diskUsage.Set(float64(bytes))
}

// SetCPUUsage sets the current CPU usage percentage
func (pm *PrometheusMetrics) SetCPUUsage(percent float64) {
	pm.cpuUsage.Set(percent)
}

// RecordWALWrite records a WAL write operation
func (pm *PrometheusMetrics) RecordWALWrite() {
	pm.walWrites.Inc()
}

// RecordWALSyncDuration records the duration of a WAL sync operation
func (pm *PrometheusMetrics) RecordWALSyncDuration(duration time.Duration) {
	pm.walSyncDuration.Observe(duration.Seconds())
}

// RecordTransaction records a transaction
func (pm *PrometheusMetrics) RecordTransaction(txType string, status string) {
	labels := map[string]string{
		"type":   txType,
		"status": status,
	}
	pm.transactionCount.IncWithLabels(labels)
}

// RecordLockWaitDuration records the time spent waiting for locks
func (pm *PrometheusMetrics) RecordLockWaitDuration(duration time.Duration, lockType string) {
	labels := map[string]string{
		"lock_type": lockType,
	}
	pm.lockWaitDuration.ObserveWithLabels(duration.Seconds(), labels)
}

// SetCacheHitRatio sets the cache hit ratio
func (pm *PrometheusMetrics) SetCacheHitRatio(ratio float64) {
	pm.cacheHitRatio.Set(ratio)
}

// updateUptime updates the uptime metric
func (pm *PrometheusMetrics) updateUptime() {
	uptime := time.Since(pm.startTime).Seconds()
	pm.uptime.Set(uptime)
}

// ExportMetrics returns all metrics in Prometheus format
func (pm *PrometheusMetrics) ExportMetrics() string {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	// Update uptime before export
	pm.updateUptime()

	var output strings.Builder

	// Export all metrics
	metrics := []Metric{
		pm.queryDuration,
		pm.queryCount,
		pm.activeConnections,
		pm.throughput,
		pm.errorCount,
		pm.memoryUsage,
		pm.diskUsage,
		pm.cpuUsage,
		pm.walWrites,
		pm.walSyncDuration,
		pm.transactionCount,
		pm.lockWaitDuration,
		pm.cacheHitRatio,
		pm.uptime,
	}

	for _, metric := range metrics {
		output.WriteString(metric.Export())
		output.WriteString("\n")
	}

	return output.String()
}

// ServeHTTP implements http.Handler for Prometheus metrics endpoint
func (pm *PrometheusMetrics) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(pm.ExportMetrics()))
}

// NewCounterMetric creates a new counter metric
func NewCounterMetric(name, help string, labels map[string]string) *CounterMetric {
	return &CounterMetric{
		name:   name,
		help:   help,
		value:  0,
		labels: labels,
	}
}

// Name returns the metric name
func (c *CounterMetric) Name() string {
	return c.name
}

// Type returns the metric type
func (c *CounterMetric) Type() MetricType {
	return CounterMetricType
}

// Help returns the metric help text
func (c *CounterMetric) Help() string {
	return c.help
}

// Inc increments the counter by 1
func (c *CounterMetric) Inc() {
	c.Add(1)
}

// Add adds the given value to the counter
func (c *CounterMetric) Add(value float64) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.value += value
}

// IncWithLabels increments the counter with specific labels
func (c *CounterMetric) IncWithLabels(labels map[string]string) {
	c.AddWithLabels(1, labels)
}

// AddWithLabels adds value with specific labels
func (c *CounterMetric) AddWithLabels(value float64, labels map[string]string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.value += value
	// In a full implementation, we'd track separate values per label combination
}

// Export returns the metric in Prometheus format
func (c *CounterMetric) Export() string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	var output strings.Builder
	output.WriteString(fmt.Sprintf("# HELP %s %s\n", c.name, c.help))
	output.WriteString(fmt.Sprintf("# TYPE %s counter\n", c.name))

	if len(c.labels) > 0 {
		labelStr := formatLabels(c.labels)
		output.WriteString(fmt.Sprintf("%s{%s} %g\n", c.name, labelStr, c.value))
	} else {
		output.WriteString(fmt.Sprintf("%s %g\n", c.name, c.value))
	}

	return output.String()
}

// NewGaugeMetric creates a new gauge metric
func NewGaugeMetric(name, help string, labels map[string]string) *GaugeMetric {
	return &GaugeMetric{
		name:   name,
		help:   help,
		value:  0,
		labels: labels,
	}
}

// Name returns the metric name
func (g *GaugeMetric) Name() string {
	return g.name
}

// Type returns the metric type
func (g *GaugeMetric) Type() MetricType {
	return GaugeMetricType
}

// Help returns the metric help text
func (g *GaugeMetric) Help() string {
	return g.help
}

// Set sets the gauge value
func (g *GaugeMetric) Set(value float64) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.value = value
}

// Inc increments the gauge by 1
func (g *GaugeMetric) Inc() {
	g.Add(1)
}

// Dec decrements the gauge by 1
func (g *GaugeMetric) Dec() {
	g.Add(-1)
}

// Add adds the given value to the gauge
func (g *GaugeMetric) Add(value float64) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.value += value
}

// Export returns the metric in Prometheus format
func (g *GaugeMetric) Export() string {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	var output strings.Builder
	output.WriteString(fmt.Sprintf("# HELP %s %s\n", g.name, g.help))
	output.WriteString(fmt.Sprintf("# TYPE %s gauge\n", g.name))

	if len(g.labels) > 0 {
		labelStr := formatLabels(g.labels)
		output.WriteString(fmt.Sprintf("%s{%s} %g\n", g.name, labelStr, g.value))
	} else {
		output.WriteString(fmt.Sprintf("%s %g\n", g.name, g.value))
	}

	return output.String()
}

// NewHistogramMetric creates a new histogram metric
func NewHistogramMetric(name, help string, buckets []float64, labels map[string]string) *HistogramMetric {
	return &HistogramMetric{
		name:    name,
		help:    help,
		buckets: buckets,
		counts:  make([]uint64, len(buckets)+1), // +1 for +Inf bucket
		sum:     0,
		count:   0,
		labels:  labels,
	}
}

// Name returns the metric name
func (h *HistogramMetric) Name() string {
	return h.name
}

// Type returns the metric type
func (h *HistogramMetric) Type() MetricType {
	return HistogramMetricType
}

// Help returns the metric help text
func (h *HistogramMetric) Help() string {
	return h.help
}

// Observe adds an observation to the histogram
func (h *HistogramMetric) Observe(value float64) {
	h.ObserveWithLabels(value, nil)
}

// ObserveWithLabels adds an observation with specific labels
func (h *HistogramMetric) ObserveWithLabels(value float64, labels map[string]string) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.sum += value
	h.count++

	// Find the appropriate bucket
	for i, bucket := range h.buckets {
		if value <= bucket {
			h.counts[i]++
		}
	}
	// Always increment the +Inf bucket
	h.counts[len(h.buckets)]++
}

// Export returns the metric in Prometheus format
func (h *HistogramMetric) Export() string {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	var output strings.Builder
	output.WriteString(fmt.Sprintf("# HELP %s %s\n", h.name, h.help))
	output.WriteString(fmt.Sprintf("# TYPE %s histogram\n", h.name))

	labelStr := ""
	if len(h.labels) > 0 {
		labelStr = formatLabels(h.labels)
	}

	// Export bucket counts
	for i, bucket := range h.buckets {
		bucketLabel := fmt.Sprintf("le=\"%g\"", bucket)
		if labelStr != "" {
			bucketLabel = labelStr + "," + bucketLabel
		}
		output.WriteString(fmt.Sprintf("%s_bucket{%s} %d\n", h.name, bucketLabel, h.counts[i]))
	}

	// Export +Inf bucket
	infLabel := "le=\"+Inf\""
	if labelStr != "" {
		infLabel = labelStr + "," + infLabel
	}
	output.WriteString(fmt.Sprintf("%s_bucket{%s} %d\n", h.name, infLabel, h.counts[len(h.buckets)]))

	// Export sum and count
	if labelStr != "" {
		output.WriteString(fmt.Sprintf("%s_sum{%s} %g\n", h.name, labelStr, h.sum))
		output.WriteString(fmt.Sprintf("%s_count{%s} %d\n", h.name, labelStr, h.count))
	} else {
		output.WriteString(fmt.Sprintf("%s_sum %g\n", h.name, h.sum))
		output.WriteString(fmt.Sprintf("%s_count %d\n", h.name, h.count))
	}

	return output.String()
}

// formatLabels formats labels for Prometheus output
func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}

	var parts []string
	for key, value := range labels {
		parts = append(parts, fmt.Sprintf("%s=\"%s\"", key, value))
	}

	return strings.Join(parts, ",")
}

// MetricsServer provides an HTTP server for Prometheus metrics
type MetricsServer struct {
	server  *http.Server
	metrics *PrometheusMetrics
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewMetricsServer creates a new metrics server
func NewMetricsServer(addr string, metrics *PrometheusMetrics) *MetricsServer {
	ctx, cancel := context.WithCancel(context.Background())

	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return &MetricsServer{
		server:  server,
		metrics: metrics,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start starts the metrics server
func (ms *MetricsServer) Start() error {
	go func() {
		if err := ms.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Metrics server error: %v\n", err)
		}
	}()

	return nil
}

// Stop stops the metrics server
func (ms *MetricsServer) Stop() error {
	ms.cancel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return ms.server.Shutdown(ctx)
}

// GetMetrics returns the metrics collector
func (ms *MetricsServer) GetMetrics() *PrometheusMetrics {
	return ms.metrics
}
