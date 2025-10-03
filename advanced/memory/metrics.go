package memory

import (
	"context"
	"sync"
	"time"
)

// MetricsCollector collects and aggregates memory and cache metrics
type MetricsCollector struct {
	cacheManager *CacheManager
	monitor      *MemoryMonitor
	alertManager *AlertManager
	metrics      *Metrics
	mutex        sync.RWMutex
	stopCh       chan struct{}
	running      bool
}

// Metrics holds aggregated performance metrics
type Metrics struct {
	// Cache metrics
	CacheHitRatio  float64
	CacheSize      int64
	CacheEntries   int64
	CacheEvictions int64
	CacheHits      int64
	CacheMisses    int64

	// Memory metrics
	MemoryUsage float64
	HeapUsage   float64
	GCPressure  float64
	AllocRate   float64 // Bytes allocated per second
	GCPauseTime float64 // Average GC pause time in ms

	// Performance metrics
	AvgAccessTime time.Duration
	P95AccessTime time.Duration
	P99AccessTime time.Duration
	ThroughputRPS float64 // Requests per second

	// System metrics
	SystemMemory    int64
	AvailableMemory int64
	MemoryPressure  float64

	// Timestamps
	LastUpdated      time.Time
	CollectionPeriod time.Duration
}

// PerformanceTracker tracks cache operation performance
type PerformanceTracker struct {
	accessTimes []time.Duration
	mutex       sync.RWMutex
	maxSamples  int
	requests    int64
	startTime   time.Time
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(cacheManager *CacheManager, monitor *MemoryMonitor, alertManager *AlertManager) *MetricsCollector {
	return &MetricsCollector{
		cacheManager: cacheManager,
		monitor:      monitor,
		alertManager: alertManager,
		metrics:      &Metrics{},
		stopCh:       make(chan struct{}),
	}
}

// Start begins metrics collection
func (mc *MetricsCollector) Start(ctx context.Context, interval time.Duration) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	if mc.running {
		return
	}

	mc.running = true
	go mc.collect(ctx, interval)
}

// Stop stops metrics collection
func (mc *MetricsCollector) Stop() {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	if !mc.running {
		return
	}

	mc.running = false
	close(mc.stopCh)
}

// GetMetrics returns current metrics snapshot
func (mc *MetricsCollector) GetMetrics() *Metrics {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()

	// Return a copy to avoid race conditions
	metrics := *mc.metrics
	return &metrics
}

// Private methods

func (mc *MetricsCollector) collect(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mc.updateMetrics()
		case <-mc.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (mc *MetricsCollector) updateMetrics() {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	// Update cache metrics
	if mc.cacheManager != nil {
		cacheStats := mc.cacheManager.GetStats()
		mc.metrics.CacheHitRatio = cacheStats.HitRatio
		mc.metrics.CacheSize = cacheStats.TotalSize
		mc.metrics.CacheEntries = cacheStats.TotalEntries
		mc.metrics.CacheEvictions = cacheStats.EvictionCount
		mc.metrics.CacheHits = cacheStats.HitCount
		mc.metrics.CacheMisses = cacheStats.MissCount
	}

	// Update memory metrics
	if mc.monitor != nil {
		memStats := mc.monitor.GetStats()
		mc.metrics.MemoryUsage = memStats.MemoryUsage
		mc.metrics.HeapUsage = memStats.HeapUsage
		mc.metrics.GCPressure = memStats.GCPressure
		mc.metrics.SystemMemory = int64(memStats.Sys)
		mc.metrics.AvailableMemory = int64(memStats.Sys - memStats.Alloc)

		// Calculate allocation rate
		if mc.metrics.LastUpdated.IsZero() {
			mc.metrics.AllocRate = 0
		} else {
			timeDiff := time.Since(mc.metrics.LastUpdated).Seconds()
			if timeDiff > 0 {
				allocDiff := float64(memStats.TotalAlloc) - float64(mc.metrics.SystemMemory)
				mc.metrics.AllocRate = allocDiff / timeDiff
			}
		}

		// Calculate average GC pause time
		if memStats.NumGC > 0 {
			mc.metrics.GCPauseTime = float64(memStats.PauseTotalNs) / float64(memStats.NumGC) / 1e6 // Convert to ms
		}

		// Calculate memory pressure
		mc.metrics.MemoryPressure = mc.calculateMemoryPressure(memStats)
	}

	mc.metrics.LastUpdated = time.Now()
	mc.metrics.CollectionPeriod = time.Since(mc.metrics.LastUpdated)
}

func (mc *MetricsCollector) calculateMemoryPressure(stats *MemoryStats) float64 {
	// Memory pressure is a composite metric based on:
	// - Memory usage ratio
	// - GC frequency
	// - Heap fragmentation

	memoryWeight := 0.4
	gcWeight := 0.3
	fragWeight := 0.3

	memoryPressure := stats.MemoryUsage * memoryWeight
	gcPressure := (stats.GCPressure / 10.0) * gcWeight // Normalize GC pressure

	// Calculate heap fragmentation
	var fragmentation float64
	if stats.HeapSys > 0 {
		fragmentation = float64(stats.HeapIdle) / float64(stats.HeapSys)
	}
	fragPressure := fragmentation * fragWeight

	return memoryPressure + gcPressure + fragPressure
}

// NewPerformanceTracker creates a new performance tracker
func NewPerformanceTracker(maxSamples int) *PerformanceTracker {
	return &PerformanceTracker{
		accessTimes: make([]time.Duration, 0, maxSamples),
		maxSamples:  maxSamples,
		startTime:   time.Now(),
	}
}

// RecordAccess records a cache access time
func (pt *PerformanceTracker) RecordAccess(duration time.Duration) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	pt.accessTimes = append(pt.accessTimes, duration)
	pt.requests++

	// Keep only the most recent samples
	if len(pt.accessTimes) > pt.maxSamples {
		pt.accessTimes = pt.accessTimes[len(pt.accessTimes)-pt.maxSamples:]
	}
}

// GetPerformanceMetrics returns performance statistics
func (pt *PerformanceTracker) GetPerformanceMetrics() (avg, p95, p99 time.Duration, rps float64) {
	pt.mutex.RLock()
	defer pt.mutex.RUnlock()

	if len(pt.accessTimes) == 0 {
		return 0, 0, 0, 0
	}

	// Calculate average
	var total time.Duration
	for _, t := range pt.accessTimes {
		total += t
	}
	avg = total / time.Duration(len(pt.accessTimes))

	// Calculate percentiles
	sorted := make([]time.Duration, len(pt.accessTimes))
	copy(sorted, pt.accessTimes)

	// Simple bubble sort for small arrays
	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-i-1; j++ {
			if sorted[j] > sorted[j+1] {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}

	p95Index := int(float64(len(sorted)) * 0.95)
	p99Index := int(float64(len(sorted)) * 0.99)

	if p95Index >= len(sorted) {
		p95Index = len(sorted) - 1
	}
	if p99Index >= len(sorted) {
		p99Index = len(sorted) - 1
	}

	p95 = sorted[p95Index]
	p99 = sorted[p99Index]

	// Calculate RPS
	elapsed := time.Since(pt.startTime).Seconds()
	if elapsed > 0 {
		rps = float64(pt.requests) / elapsed
	}

	return avg, p95, p99, rps
}

// Reset clears all performance data
func (pt *PerformanceTracker) Reset() {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	pt.accessTimes = pt.accessTimes[:0]
	pt.requests = 0
	pt.startTime = time.Now()
}

// HealthChecker provides health status for memory and cache systems
type HealthChecker struct {
	cacheManager *CacheManager
	monitor      *MemoryMonitor
	collector    *MetricsCollector
	thresholds   *HealthThresholds
}

// HealthThresholds defines health check thresholds
type HealthThresholds struct {
	MaxMemoryUsage    float64       // Maximum acceptable memory usage ratio
	MaxGCPressure     float64       // Maximum acceptable GC pressure
	MinHitRatio       float64       // Minimum acceptable cache hit ratio
	MaxResponseTime   time.Duration // Maximum acceptable response time
	MaxMemoryPressure float64       // Maximum acceptable memory pressure
}

// HealthStatus represents the health status of the memory system
type HealthStatus struct {
	Healthy      bool
	Issues       []string
	Metrics      *Metrics
	Timestamp    time.Time
	OverallScore float64 // 0.0 to 1.0, where 1.0 is perfect health
}

// DefaultHealthThresholds returns default health check thresholds
func DefaultHealthThresholds() *HealthThresholds {
	return &HealthThresholds{
		MaxMemoryUsage:    0.85,                   // 85% memory usage
		MaxGCPressure:     5.0,                    // 5 GCs per minute
		MinHitRatio:       0.80,                   // 80% cache hit ratio
		MaxResponseTime:   time.Millisecond * 100, // 100ms max response time
		MaxMemoryPressure: 0.75,                   // 75% memory pressure
	}
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(cacheManager *CacheManager, monitor *MemoryMonitor, collector *MetricsCollector) *HealthChecker {
	return &HealthChecker{
		cacheManager: cacheManager,
		monitor:      monitor,
		collector:    collector,
		thresholds:   DefaultHealthThresholds(),
	}
}

// SetThresholds updates health check thresholds
func (hc *HealthChecker) SetThresholds(thresholds *HealthThresholds) {
	hc.thresholds = thresholds
}

// CheckHealth performs a comprehensive health check
func (hc *HealthChecker) CheckHealth() *HealthStatus {
	status := &HealthStatus{
		Healthy:   true,
		Issues:    make([]string, 0),
		Timestamp: time.Now(),
	}

	if hc.collector != nil {
		status.Metrics = hc.collector.GetMetrics()
	}

	var scores []float64

	// Check memory usage
	if status.Metrics != nil && status.Metrics.MemoryUsage > hc.thresholds.MaxMemoryUsage {
		status.Healthy = false
		status.Issues = append(status.Issues, "High memory usage")
		scores = append(scores, 0.0)
	} else {
		scores = append(scores, 1.0)
	}

	// Check GC pressure
	if status.Metrics != nil && status.Metrics.GCPressure > hc.thresholds.MaxGCPressure {
		status.Healthy = false
		status.Issues = append(status.Issues, "High GC pressure")
		scores = append(scores, 0.0)
	} else {
		scores = append(scores, 1.0)
	}

	// Check cache hit ratio
	if status.Metrics != nil && status.Metrics.CacheHitRatio < hc.thresholds.MinHitRatio {
		status.Healthy = false
		status.Issues = append(status.Issues, "Low cache hit ratio")
		scores = append(scores, 0.0)
	} else {
		scores = append(scores, 1.0)
	}

	// Check response time
	if status.Metrics != nil && status.Metrics.AvgAccessTime > hc.thresholds.MaxResponseTime {
		status.Healthy = false
		status.Issues = append(status.Issues, "High response time")
		scores = append(scores, 0.0)
	} else {
		scores = append(scores, 1.0)
	}

	// Check memory pressure
	if status.Metrics != nil && status.Metrics.MemoryPressure > hc.thresholds.MaxMemoryPressure {
		status.Healthy = false
		status.Issues = append(status.Issues, "High memory pressure")
		scores = append(scores, 0.0)
	} else {
		scores = append(scores, 1.0)
	}

	// Calculate overall score
	if len(scores) > 0 {
		var total float64
		for _, score := range scores {
			total += score
		}
		status.OverallScore = total / float64(len(scores))
	}

	return status
}
