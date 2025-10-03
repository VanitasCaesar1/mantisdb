package compression

import (
	"sync"
	"time"
)

// CompressionMonitor tracks compression metrics and performance
type CompressionMonitor struct {
	metrics map[string]*AlgorithmMetrics
	mutex   sync.RWMutex
}

// AlgorithmMetrics tracks metrics for a specific compression algorithm
type AlgorithmMetrics struct {
	CompressionCount       int64
	DecompressionCount     int64
	TotalInputBytes        int64
	TotalOutputBytes       int64
	TotalCompressionTime   time.Duration
	TotalDecompressionTime time.Duration
	AverageRatio           float64
	mutex                  sync.RWMutex
}

// AlgorithmMetricsSnapshot represents algorithm metrics without mutex for safe copying
type AlgorithmMetricsSnapshot struct {
	CompressionCount       int64         `json:"compression_count"`
	DecompressionCount     int64         `json:"decompression_count"`
	TotalInputBytes        int64         `json:"total_input_bytes"`
	TotalOutputBytes       int64         `json:"total_output_bytes"`
	TotalCompressionTime   time.Duration `json:"total_compression_time"`
	TotalDecompressionTime time.Duration `json:"total_decompression_time"`
	AverageRatio           float64       `json:"average_ratio"`
}

// CompressionMetrics represents overall compression system metrics
type CompressionMetrics struct {
	Algorithms        map[string]AlgorithmMetricsSnapshot `json:"algorithms"`
	TotalCompressed   int64                               `json:"total_compressed"`
	TotalDecompressed int64                               `json:"total_decompressed"`
	OverallRatio      float64                             `json:"overall_ratio"`
	CompressionRate   float64                             `json:"compression_rate_mb_per_sec"`
	DecompressionRate float64                             `json:"decompression_rate_mb_per_sec"`
}

// NewCompressionMonitor creates a new compression monitor
func NewCompressionMonitor() *CompressionMonitor {
	return &CompressionMonitor{
		metrics: make(map[string]*AlgorithmMetrics),
	}
}

// RecordCompression records compression operation metrics
func (m *CompressionMonitor) RecordCompression(algorithm string, inputSize, outputSize int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.metrics[algorithm]; !exists {
		m.metrics[algorithm] = &AlgorithmMetrics{}
	}

	metrics := m.metrics[algorithm]
	metrics.mutex.Lock()
	defer metrics.mutex.Unlock()

	metrics.CompressionCount++
	metrics.TotalInputBytes += int64(inputSize)
	metrics.TotalOutputBytes += int64(outputSize)

	// Update average compression ratio
	if outputSize > 0 {
		ratio := float64(inputSize) / float64(outputSize)
		if metrics.CompressionCount == 1 {
			metrics.AverageRatio = ratio
		} else {
			// Running average
			metrics.AverageRatio = (metrics.AverageRatio*float64(metrics.CompressionCount-1) + ratio) / float64(metrics.CompressionCount)
		}
	}
}

// RecordDecompression records decompression operation metrics
func (m *CompressionMonitor) RecordDecompression(algorithm string, inputSize, outputSize int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.metrics[algorithm]; !exists {
		m.metrics[algorithm] = &AlgorithmMetrics{}
	}

	metrics := m.metrics[algorithm]
	metrics.mutex.Lock()
	defer metrics.mutex.Unlock()

	metrics.DecompressionCount++
}

// RecordCompressionTime records time taken for compression
func (m *CompressionMonitor) RecordCompressionTime(algorithm string, duration time.Duration) {
	m.mutex.RLock()
	metrics, exists := m.metrics[algorithm]
	m.mutex.RUnlock()

	if !exists {
		return
	}

	metrics.mutex.Lock()
	defer metrics.mutex.Unlock()
	metrics.TotalCompressionTime += duration
}

// RecordDecompressionTime records time taken for decompression
func (m *CompressionMonitor) RecordDecompressionTime(algorithm string, duration time.Duration) {
	m.mutex.RLock()
	metrics, exists := m.metrics[algorithm]
	m.mutex.RUnlock()

	if !exists {
		return
	}

	metrics.mutex.Lock()
	defer metrics.mutex.Unlock()
	metrics.TotalDecompressionTime += duration
}

// GetMetrics returns current compression metrics
func (m *CompressionMonitor) GetMetrics() CompressionMetrics {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := CompressionMetrics{
		Algorithms: make(map[string]AlgorithmMetricsSnapshot),
	}

	var totalInput, totalOutput int64
	var totalCompressionTime, totalDecompressionTime time.Duration

	for algo, metrics := range m.metrics {
		metrics.mutex.RLock()

		algorithmMetrics := AlgorithmMetricsSnapshot{
			CompressionCount:       metrics.CompressionCount,
			DecompressionCount:     metrics.DecompressionCount,
			TotalInputBytes:        metrics.TotalInputBytes,
			TotalOutputBytes:       metrics.TotalOutputBytes,
			TotalCompressionTime:   metrics.TotalCompressionTime,
			TotalDecompressionTime: metrics.TotalDecompressionTime,
			AverageRatio:           metrics.AverageRatio,
		}

		result.Algorithms[algo] = algorithmMetrics

		totalInput += metrics.TotalInputBytes
		totalOutput += metrics.TotalOutputBytes
		totalCompressionTime += metrics.TotalCompressionTime
		totalDecompressionTime += metrics.TotalDecompressionTime

		metrics.mutex.RUnlock()
	}

	result.TotalCompressed = totalInput
	result.TotalDecompressed = totalOutput

	if totalOutput > 0 {
		result.OverallRatio = float64(totalInput) / float64(totalOutput)
	}

	// Calculate rates in MB/s
	if totalCompressionTime > 0 {
		mbCompressed := float64(totalInput) / (1024 * 1024)
		result.CompressionRate = mbCompressed / totalCompressionTime.Seconds()
	}

	if totalDecompressionTime > 0 {
		mbDecompressed := float64(totalOutput) / (1024 * 1024)
		result.DecompressionRate = mbDecompressed / totalDecompressionTime.Seconds()
	}

	return result
}

// GetAlgorithmMetrics returns metrics for a specific algorithm
func (m *CompressionMonitor) GetAlgorithmMetrics(algorithm string) (AlgorithmMetrics, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	metrics, exists := m.metrics[algorithm]
	if !exists {
		return AlgorithmMetrics{}, false
	}

	metrics.mutex.RLock()
	defer metrics.mutex.RUnlock()

	return AlgorithmMetrics{
		CompressionCount:       metrics.CompressionCount,
		DecompressionCount:     metrics.DecompressionCount,
		TotalInputBytes:        metrics.TotalInputBytes,
		TotalOutputBytes:       metrics.TotalOutputBytes,
		TotalCompressionTime:   metrics.TotalCompressionTime,
		TotalDecompressionTime: metrics.TotalDecompressionTime,
		AverageRatio:           metrics.AverageRatio,
	}, true
}

// Reset clears all metrics
func (m *CompressionMonitor) Reset() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.metrics = make(map[string]*AlgorithmMetrics)
}

// GetCompressionRatio returns the overall compression ratio
func (m *CompressionMonitor) GetCompressionRatio() float64 {
	metrics := m.GetMetrics()
	return metrics.OverallRatio
}

// GetThroughput returns compression and decompression throughput in MB/s
func (m *CompressionMonitor) GetThroughput() (compressionMBps, decompressionMBps float64) {
	metrics := m.GetMetrics()
	return metrics.CompressionRate, metrics.DecompressionRate
}
