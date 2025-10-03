package compression

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// CompressionMetricsCollector collects and reports compression metrics
type CompressionMetricsCollector struct {
	metrics           map[string]*MetricSeries
	globalMetrics     *GlobalCompressionMetrics
	alertThresholds   *AlertThresholds
	reportingInterval time.Duration
	mutex             sync.RWMutex
	stopChan          chan struct{}
	callbacks         []MetricsCallback
}

// MetricSeries stores time-series data for a specific metric
type MetricSeries struct {
	Name       string             `json:"name"`
	Unit       string             `json:"unit"`
	DataPoints []MetricDataPoint  `json:"data_points"`
	Aggregates map[string]float64 `json:"aggregates"`
	mutex      sync.RWMutex
}

// MetricDataPoint represents a single metric measurement
type MetricDataPoint struct {
	Timestamp time.Time         `json:"timestamp"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// GlobalCompressionMetrics tracks overall system compression metrics
type GlobalCompressionMetrics struct {
	TotalDataProcessed      int64     `json:"total_data_processed"`
	TotalDataCompressed     int64     `json:"total_data_compressed"`
	TotalDataDecompressed   int64     `json:"total_data_decompressed"`
	AverageCompressionRatio float64   `json:"average_compression_ratio"`
	CompressionEfficiency   float64   `json:"compression_efficiency"`
	StorageSavings          int64     `json:"storage_savings"`
	CPUOverhead             float64   `json:"cpu_overhead"`
	MemoryOverhead          int64     `json:"memory_overhead"`
	LastUpdated             time.Time `json:"last_updated"`
	mutex                   sync.RWMutex
}

// AlertThresholds defines thresholds for compression alerts
type AlertThresholds struct {
	MinCompressionRatio     float64       `json:"min_compression_ratio"`
	MaxCPUOverhead          float64       `json:"max_cpu_overhead"`
	MaxMemoryOverhead       int64         `json:"max_memory_overhead"`
	MaxCompressionLatency   time.Duration `json:"max_compression_latency"`
	MaxDecompressionLatency time.Duration `json:"max_decompression_latency"`
	MinStorageSavings       int64         `json:"min_storage_savings"`
}

// MetricsCallback is called when metrics are updated
type MetricsCallback func(metrics *CompressionReport)

// CompressionReport represents a comprehensive compression metrics report
type CompressionReport struct {
	Timestamp          time.Time                           `json:"timestamp"`
	GlobalMetrics      *GlobalCompressionMetrics           `json:"global_metrics"`
	AlgorithmMetrics   map[string]AlgorithmMetricsSnapshot `json:"algorithm_metrics"`
	PerformanceMetrics *PerformanceMetrics                 `json:"performance_metrics"`
	StorageMetrics     *StorageMetrics                     `json:"storage_metrics"`
	Alerts             []CompressionAlert                  `json:"alerts"`
	Recommendations    []string                            `json:"recommendations"`
}

// PerformanceMetrics tracks compression performance
type PerformanceMetrics struct {
	AverageCompressionLatency   time.Duration `json:"average_compression_latency"`
	AverageDecompressionLatency time.Duration `json:"average_decompression_latency"`
	CompressionThroughput       float64       `json:"compression_throughput_mbps"`
	DecompressionThroughput     float64       `json:"decompression_throughput_mbps"`
	CPUUtilization              float64       `json:"cpu_utilization"`
	MemoryUtilization           int64         `json:"memory_utilization"`
}

// StorageMetrics tracks storage-related compression metrics
type StorageMetrics struct {
	TotalStorageUsed        int64   `json:"total_storage_used"`
	CompressedStorageUsed   int64   `json:"compressed_storage_used"`
	UncompressedStorageUsed int64   `json:"uncompressed_storage_used"`
	StorageSavingsBytes     int64   `json:"storage_savings_bytes"`
	StorageSavingsPercent   float64 `json:"storage_savings_percent"`
	CompressionRatio        float64 `json:"compression_ratio"`
}

// CompressionAlert represents an alert condition
type CompressionAlert struct {
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Metric    string    `json:"metric"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	Timestamp time.Time `json:"timestamp"`
	Algorithm string    `json:"algorithm,omitempty"`
}

// NewCompressionMetricsCollector creates a new metrics collector
func NewCompressionMetricsCollector(reportingInterval time.Duration) *CompressionMetricsCollector {
	collector := &CompressionMetricsCollector{
		metrics:           make(map[string]*MetricSeries),
		globalMetrics:     &GlobalCompressionMetrics{},
		reportingInterval: reportingInterval,
		stopChan:          make(chan struct{}),
		callbacks:         make([]MetricsCallback, 0),
		alertThresholds: &AlertThresholds{
			MinCompressionRatio:     1.5,
			MaxCPUOverhead:          20.0,              // 20%
			MaxMemoryOverhead:       100 * 1024 * 1024, // 100MB
			MaxCompressionLatency:   100 * time.Millisecond,
			MaxDecompressionLatency: 50 * time.Millisecond,
			MinStorageSavings:       1024 * 1024, // 1MB
		},
	}

	// Initialize metric series
	collector.initializeMetrics()

	// Start background reporting
	go collector.startReporting()

	return collector
}

// initializeMetrics initializes the metric series
func (cmc *CompressionMetricsCollector) initializeMetrics() {
	metrics := []struct {
		name string
		unit string
	}{
		{"compression_ratio", "ratio"},
		{"compression_latency", "milliseconds"},
		{"decompression_latency", "milliseconds"},
		{"compression_throughput", "mbps"},
		{"decompression_throughput", "mbps"},
		{"cpu_overhead", "percent"},
		{"memory_overhead", "bytes"},
		{"storage_savings", "bytes"},
		{"compression_efficiency", "percent"},
	}

	for _, metric := range metrics {
		cmc.metrics[metric.name] = &MetricSeries{
			Name:       metric.name,
			Unit:       metric.unit,
			DataPoints: make([]MetricDataPoint, 0),
			Aggregates: make(map[string]float64),
		}
	}
}

// RecordMetric records a metric value
func (cmc *CompressionMetricsCollector) RecordMetric(name string, value float64, labels map[string]string) {
	cmc.mutex.Lock()
	defer cmc.mutex.Unlock()

	series, exists := cmc.metrics[name]
	if !exists {
		return
	}

	series.mutex.Lock()
	defer series.mutex.Unlock()

	dataPoint := MetricDataPoint{
		Timestamp: time.Now(),
		Value:     value,
		Labels:    labels,
	}

	series.DataPoints = append(series.DataPoints, dataPoint)

	// Keep only last 1000 data points to prevent memory growth
	if len(series.DataPoints) > 1000 {
		series.DataPoints = series.DataPoints[len(series.DataPoints)-1000:]
	}

	// Update aggregates
	cmc.updateAggregates(series)
}

// updateAggregates updates aggregate statistics for a metric series
func (cmc *CompressionMetricsCollector) updateAggregates(series *MetricSeries) {
	if len(series.DataPoints) == 0 {
		return
	}

	var sum, min, max float64
	min = series.DataPoints[0].Value
	max = series.DataPoints[0].Value

	for _, point := range series.DataPoints {
		sum += point.Value
		if point.Value < min {
			min = point.Value
		}
		if point.Value > max {
			max = point.Value
		}
	}

	count := float64(len(series.DataPoints))
	series.Aggregates["avg"] = sum / count
	series.Aggregates["min"] = min
	series.Aggregates["max"] = max
	series.Aggregates["sum"] = sum
	series.Aggregates["count"] = count

	// Calculate percentiles (simplified)
	if len(series.DataPoints) >= 10 {
		// Sort values for percentile calculation (simplified approach)
		values := make([]float64, len(series.DataPoints))
		for i, point := range series.DataPoints {
			values[i] = point.Value
		}

		// Simple percentile approximation
		p50Index := len(values) / 2
		p95Index := int(float64(len(values)) * 0.95)
		p99Index := int(float64(len(values)) * 0.99)

		if p50Index < len(values) {
			series.Aggregates["p50"] = values[p50Index]
		}
		if p95Index < len(values) {
			series.Aggregates["p95"] = values[p95Index]
		}
		if p99Index < len(values) {
			series.Aggregates["p99"] = values[p99Index]
		}
	}
}

// UpdateGlobalMetrics updates global compression metrics
func (cmc *CompressionMetricsCollector) UpdateGlobalMetrics(monitor *CompressionMonitor) {
	cmc.globalMetrics.mutex.Lock()
	defer cmc.globalMetrics.mutex.Unlock()

	metrics := monitor.GetMetrics()

	cmc.globalMetrics.TotalDataProcessed = metrics.TotalCompressed + metrics.TotalDecompressed
	cmc.globalMetrics.TotalDataCompressed = metrics.TotalCompressed
	cmc.globalMetrics.TotalDataDecompressed = metrics.TotalDecompressed
	cmc.globalMetrics.AverageCompressionRatio = metrics.OverallRatio
	cmc.globalMetrics.LastUpdated = time.Now()

	// Calculate storage savings
	if metrics.TotalCompressed > 0 && metrics.OverallRatio > 1 {
		originalSize := float64(metrics.TotalCompressed) * metrics.OverallRatio
		cmc.globalMetrics.StorageSavings = int64(originalSize) - metrics.TotalCompressed
	}

	// Calculate compression efficiency (ratio of savings to processing overhead)
	if cmc.globalMetrics.StorageSavings > 0 {
		cmc.globalMetrics.CompressionEfficiency = float64(cmc.globalMetrics.StorageSavings) / float64(cmc.globalMetrics.TotalDataProcessed) * 100
	}

	// Record metrics
	cmc.RecordMetric("compression_ratio", metrics.OverallRatio, nil)
	cmc.RecordMetric("storage_savings", float64(cmc.globalMetrics.StorageSavings), nil)
	cmc.RecordMetric("compression_efficiency", cmc.globalMetrics.CompressionEfficiency, nil)
}

// GenerateReport generates a comprehensive compression report
func (cmc *CompressionMetricsCollector) GenerateReport(monitor *CompressionMonitor) *CompressionReport {
	cmc.mutex.RLock()
	defer cmc.mutex.RUnlock()

	// Update global metrics first
	cmc.UpdateGlobalMetrics(monitor)

	report := &CompressionReport{
		Timestamp:          time.Now(),
		GlobalMetrics:      cmc.getGlobalMetricsCopy(),
		AlgorithmMetrics:   monitor.GetMetrics().Algorithms,
		PerformanceMetrics: cmc.generatePerformanceMetrics(monitor),
		StorageMetrics:     cmc.generateStorageMetrics(),
		Alerts:             cmc.checkAlerts(monitor),
		Recommendations:    cmc.generateRecommendations(monitor),
	}

	return report
}

// getGlobalMetricsCopy returns a copy of global metrics
func (cmc *CompressionMetricsCollector) getGlobalMetricsCopy() *GlobalCompressionMetrics {
	cmc.globalMetrics.mutex.RLock()
	defer cmc.globalMetrics.mutex.RUnlock()

	return &GlobalCompressionMetrics{
		TotalDataProcessed:      cmc.globalMetrics.TotalDataProcessed,
		TotalDataCompressed:     cmc.globalMetrics.TotalDataCompressed,
		TotalDataDecompressed:   cmc.globalMetrics.TotalDataDecompressed,
		AverageCompressionRatio: cmc.globalMetrics.AverageCompressionRatio,
		CompressionEfficiency:   cmc.globalMetrics.CompressionEfficiency,
		StorageSavings:          cmc.globalMetrics.StorageSavings,
		CPUOverhead:             cmc.globalMetrics.CPUOverhead,
		MemoryOverhead:          cmc.globalMetrics.MemoryOverhead,
		LastUpdated:             cmc.globalMetrics.LastUpdated,
	}
}

// generatePerformanceMetrics generates performance metrics
func (cmc *CompressionMetricsCollector) generatePerformanceMetrics(monitor *CompressionMonitor) *PerformanceMetrics {
	metrics := monitor.GetMetrics()

	return &PerformanceMetrics{
		CompressionThroughput:   metrics.CompressionRate,
		DecompressionThroughput: metrics.DecompressionRate,
		CPUUtilization:          cmc.globalMetrics.CPUOverhead,
		MemoryUtilization:       cmc.globalMetrics.MemoryOverhead,
	}
}

// generateStorageMetrics generates storage-related metrics
func (cmc *CompressionMetricsCollector) generateStorageMetrics() *StorageMetrics {
	cmc.globalMetrics.mutex.RLock()
	defer cmc.globalMetrics.mutex.RUnlock()

	totalStorage := cmc.globalMetrics.TotalDataCompressed + cmc.globalMetrics.TotalDataDecompressed
	savingsPercent := 0.0
	if totalStorage > 0 {
		savingsPercent = float64(cmc.globalMetrics.StorageSavings) / float64(totalStorage) * 100
	}

	return &StorageMetrics{
		TotalStorageUsed:        totalStorage,
		CompressedStorageUsed:   cmc.globalMetrics.TotalDataCompressed,
		UncompressedStorageUsed: cmc.globalMetrics.TotalDataDecompressed,
		StorageSavingsBytes:     cmc.globalMetrics.StorageSavings,
		StorageSavingsPercent:   savingsPercent,
		CompressionRatio:        cmc.globalMetrics.AverageCompressionRatio,
	}
}

// checkAlerts checks for alert conditions
func (cmc *CompressionMetricsCollector) checkAlerts(monitor *CompressionMonitor) []CompressionAlert {
	alerts := make([]CompressionAlert, 0)
	metrics := monitor.GetMetrics()

	// Check compression ratio
	if metrics.OverallRatio < cmc.alertThresholds.MinCompressionRatio {
		alerts = append(alerts, CompressionAlert{
			Level:     "warning",
			Message:   "Compression ratio below threshold",
			Metric:    "compression_ratio",
			Value:     metrics.OverallRatio,
			Threshold: cmc.alertThresholds.MinCompressionRatio,
			Timestamp: time.Now(),
		})
	}

	// Check CPU overhead
	if cmc.globalMetrics.CPUOverhead > cmc.alertThresholds.MaxCPUOverhead {
		alerts = append(alerts, CompressionAlert{
			Level:     "critical",
			Message:   "CPU overhead too high",
			Metric:    "cpu_overhead",
			Value:     cmc.globalMetrics.CPUOverhead,
			Threshold: cmc.alertThresholds.MaxCPUOverhead,
			Timestamp: time.Now(),
		})
	}

	// Check memory overhead
	if cmc.globalMetrics.MemoryOverhead > cmc.alertThresholds.MaxMemoryOverhead {
		alerts = append(alerts, CompressionAlert{
			Level:     "critical",
			Message:   "Memory overhead too high",
			Metric:    "memory_overhead",
			Value:     float64(cmc.globalMetrics.MemoryOverhead),
			Threshold: float64(cmc.alertThresholds.MaxMemoryOverhead),
			Timestamp: time.Now(),
		})
	}

	return alerts
}

// generateRecommendations generates optimization recommendations
func (cmc *CompressionMetricsCollector) generateRecommendations(monitor *CompressionMonitor) []string {
	recommendations := make([]string, 0)
	metrics := monitor.GetMetrics()

	// Analyze algorithm performance
	bestRatio := 0.0
	bestAlgo := ""
	for algo, algoMetrics := range metrics.Algorithms {
		if algoMetrics.AverageRatio > bestRatio {
			bestRatio = algoMetrics.AverageRatio
			bestAlgo = algo
		}
	}

	if bestAlgo != "" && metrics.OverallRatio < bestRatio*0.8 {
		recommendations = append(recommendations,
			fmt.Sprintf("Consider using %s algorithm more frequently for better compression ratio", bestAlgo))
	}

	// Check for low compression efficiency
	if cmc.globalMetrics.CompressionEfficiency < 10 {
		recommendations = append(recommendations,
			"Compression efficiency is low. Consider adjusting cold data thresholds or compression policies")
	}

	// Check for high CPU overhead
	if cmc.globalMetrics.CPUOverhead > 15 {
		recommendations = append(recommendations,
			"CPU overhead is high. Consider using faster compression algorithms like LZ4 or Snappy")
	}

	return recommendations
}

// AddCallback adds a metrics callback
func (cmc *CompressionMetricsCollector) AddCallback(callback MetricsCallback) {
	cmc.mutex.Lock()
	defer cmc.mutex.Unlock()
	cmc.callbacks = append(cmc.callbacks, callback)
}

// startReporting starts the background reporting routine
func (cmc *CompressionMetricsCollector) startReporting() {
	ticker := time.NewTicker(cmc.reportingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// This would be called with actual monitor instance
			// For now, we'll skip the reporting
		case <-cmc.stopChan:
			return
		}
	}
}

// Stop stops the metrics collector
func (cmc *CompressionMetricsCollector) Stop() {
	close(cmc.stopChan)
}

// GetMetricSeries returns a metric series by name
func (cmc *CompressionMetricsCollector) GetMetricSeries(name string) (*MetricSeries, bool) {
	cmc.mutex.RLock()
	defer cmc.mutex.RUnlock()

	series, exists := cmc.metrics[name]
	if !exists {
		return nil, false
	}

	// Return a copy to prevent external modification
	series.mutex.RLock()
	defer series.mutex.RUnlock()

	seriesCopy := &MetricSeries{
		Name:       series.Name,
		Unit:       series.Unit,
		DataPoints: make([]MetricDataPoint, len(series.DataPoints)),
		Aggregates: make(map[string]float64),
	}

	copy(seriesCopy.DataPoints, series.DataPoints)
	for k, v := range series.Aggregates {
		seriesCopy.Aggregates[k] = v
	}

	return seriesCopy, true
}

// ExportMetrics exports metrics in JSON format
func (cmc *CompressionMetricsCollector) ExportMetrics() ([]byte, error) {
	cmc.mutex.RLock()
	defer cmc.mutex.RUnlock()

	export := make(map[string]interface{})
	export["global_metrics"] = cmc.getGlobalMetricsCopy()
	export["metric_series"] = make(map[string]*MetricSeries)

	for name := range cmc.metrics {
		if exportSeries, exists := cmc.GetMetricSeries(name); exists {
			export["metric_series"].(map[string]*MetricSeries)[name] = exportSeries
		}
	}

	return json.Marshal(export)
}
