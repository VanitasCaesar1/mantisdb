package concurrency

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// MetricsExporter exports lock metrics in a simple JSON format
type MetricsExporter struct {
	// Metrics storage
	lockMetrics   map[string]*LockResourceMetrics
	globalMetrics *GlobalMetrics
	mutex         sync.RWMutex

	// Data sources
	lockManager      *EnhancedLockManager
	deadlockDetector *EnhancedDeadlockDetector
	profiler         *LockProfiler

	// HTTP server
	server         *http.Server
	updateInterval time.Duration

	// Lifecycle
	stopChan chan struct{}
	wg       sync.WaitGroup
	running  int32 // atomic
}

// LockResourceMetrics tracks metrics for a specific resource
type LockResourceMetrics struct {
	Resource          string    `json:"resource"`
	RequestsTotal     int64     `json:"requests_total"`
	AcquisitionsTotal int64     `json:"acquisitions_total"`
	ReleasesTotal     int64     `json:"releases_total"`
	TimeoutsTotal     int64     `json:"timeouts_total"`
	ContentionTotal   int64     `json:"contention_total"`
	AvgWaitTime       float64   `json:"avg_wait_time_ms"`
	MaxWaitTime       float64   `json:"max_wait_time_ms"`
	AvgHoldTime       float64   `json:"avg_hold_time_ms"`
	MaxHoldTime       float64   `json:"max_hold_time_ms"`
	CurrentQueueDepth int32     `json:"current_queue_depth"`
	MaxQueueDepth     int32     `json:"max_queue_depth"`
	LastUpdated       time.Time `json:"last_updated"`
}

// GlobalMetrics tracks system-wide metrics
type GlobalMetrics struct {
	TotalLockRequests     int64     `json:"total_lock_requests"`
	TotalLockAcquisitions int64     `json:"total_lock_acquisitions"`
	TotalLockReleases     int64     `json:"total_lock_releases"`
	TotalLockTimeouts     int64     `json:"total_lock_timeouts"`
	TotalContentionEvents int64     `json:"total_contention_events"`
	ActiveLocks           int32     `json:"active_locks"`
	MaxConcurrentLocks    int32     `json:"max_concurrent_locks"`
	DeadlocksDetected     int64     `json:"deadlocks_detected"`
	DeadlocksResolved     int64     `json:"deadlocks_resolved"`
	FastPathHits          int64     `json:"fast_path_hits"`
	FastPathMisses        int64     `json:"fast_path_misses"`
	SystemUptime          float64   `json:"system_uptime_seconds"`
	LastUpdated           time.Time `json:"last_updated"`
}

// MetricsSnapshot represents a complete metrics snapshot
type MetricsSnapshot struct {
	Timestamp       time.Time                       `json:"timestamp"`
	GlobalMetrics   *GlobalMetrics                  `json:"global_metrics"`
	ResourceMetrics map[string]*LockResourceMetrics `json:"resource_metrics"`
	TopHotspots     []*HotspotInfo                  `json:"top_hotspots"`
}

// NewMetricsExporter creates a new metrics exporter
func NewMetricsExporter(config *MetricsExporterConfig) *MetricsExporter {
	if config == nil {
		config = DefaultMetricsExporterConfig()
	}

	return &MetricsExporter{
		lockMetrics:    make(map[string]*LockResourceMetrics),
		globalMetrics:  &GlobalMetrics{},
		updateInterval: config.UpdateInterval,
		stopChan:       make(chan struct{}),
	}
}

// MetricsExporterConfig holds configuration for the metrics exporter
type MetricsExporterConfig struct {
	UpdateInterval time.Duration
	ListenAddress  string
	MetricsPath    string
}

// DefaultMetricsExporterConfig returns default configuration
func DefaultMetricsExporterConfig() *MetricsExporterConfig {
	return &MetricsExporterConfig{
		UpdateInterval: 10 * time.Second,
		ListenAddress:  ":9090",
		MetricsPath:    "/metrics",
	}
}

// SetDataSources sets the data sources for metrics collection
func (me *MetricsExporter) SetDataSources(lockManager *EnhancedLockManager,
	deadlockDetector *EnhancedDeadlockDetector, profiler *LockProfiler) {
	me.mutex.Lock()
	defer me.mutex.Unlock()

	me.lockManager = lockManager
	me.deadlockDetector = deadlockDetector
	me.profiler = profiler
}

// Start starts the metrics exporter
func (me *MetricsExporter) Start(ctx context.Context, config *MetricsExporterConfig) error {
	if !atomic.CompareAndSwapInt32(&me.running, 0, 1) {
		return fmt.Errorf("metrics exporter is already running")
	}

	if config == nil {
		config = DefaultMetricsExporterConfig()
	}

	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc(config.MetricsPath, me.handleMetrics)
	mux.HandleFunc("/health", me.handleHealth)

	me.server = &http.Server{
		Addr:    config.ListenAddress,
		Handler: mux,
	}

	// Start HTTP server
	go func() {
		if err := me.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Log error in production
			fmt.Printf("Metrics server error: %v\n", err)
		}
	}()

	// Start metrics collection
	me.wg.Add(1)
	go me.metricsCollectionLoop(ctx)

	return nil
}

// Stop stops the metrics exporter
func (me *MetricsExporter) Stop() error {
	if !atomic.CompareAndSwapInt32(&me.running, 1, 0) {
		return nil
	}

	// Stop metrics collection
	close(me.stopChan)
	me.wg.Wait()

	// Stop HTTP server
	if me.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		me.server.Shutdown(ctx)
	}

	return nil
}

// metricsCollectionLoop periodically collects metrics
func (me *MetricsExporter) metricsCollectionLoop(ctx context.Context) {
	defer me.wg.Done()

	ticker := time.NewTicker(me.updateInterval)
	defer ticker.Stop()

	startTime := time.Now()

	for {
		select {
		case <-ticker.C:
			me.collectMetrics(startTime)
		case <-me.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// collectMetrics collects current metrics from data sources
func (me *MetricsExporter) collectMetrics(startTime time.Time) {
	me.mutex.Lock()
	defer me.mutex.Unlock()

	now := time.Now()

	// Update global metrics
	me.globalMetrics.SystemUptime = now.Sub(startTime).Seconds()
	me.globalMetrics.LastUpdated = now

	// Collect from lock manager
	if me.lockManager != nil {
		lockMetrics := me.lockManager.GetMetrics()
		me.globalMetrics.TotalLockRequests = lockMetrics.locksAcquired + lockMetrics.lockTimeouts
		me.globalMetrics.TotalLockAcquisitions = lockMetrics.locksAcquired
		me.globalMetrics.TotalLockReleases = lockMetrics.locksReleased
		me.globalMetrics.TotalLockTimeouts = lockMetrics.lockTimeouts
		me.globalMetrics.TotalContentionEvents = lockMetrics.contentionEvents
		me.globalMetrics.MaxConcurrentLocks = lockMetrics.maxQueueDepth
		me.globalMetrics.FastPathHits = lockMetrics.fastPathHits
		me.globalMetrics.FastPathMisses = lockMetrics.fastPathMisses
	}

	// Collect from deadlock detector
	if me.deadlockDetector != nil {
		deadlockMetrics := me.deadlockDetector.GetMetrics()
		me.globalMetrics.DeadlocksDetected = deadlockMetrics.deadlocksFound
		me.globalMetrics.DeadlocksResolved = deadlockMetrics.deadlocksResolved
	}

	// Collect per-resource metrics from profiler
	if me.profiler != nil && me.profiler.IsEnabled() {
		me.collectResourceMetrics()
	}
}

// collectResourceMetrics collects per-resource metrics
func (me *MetricsExporter) collectResourceMetrics() {
	// Get top hotspots from profiler
	hotspots := me.profiler.GetTopHotspots(20)

	for _, hotspot := range hotspots {
		resource := hotspot.Resource

		// Get or create resource metrics
		if me.lockMetrics[resource] == nil {
			me.lockMetrics[resource] = &LockResourceMetrics{
				Resource: resource,
			}
		}

		metrics := me.lockMetrics[resource]

		// Update metrics from hotspot info
		metrics.RequestsTotal = hotspot.AccessCount
		metrics.ContentionTotal = int64(hotspot.ContentionRate * float64(hotspot.AccessCount))
		metrics.AvgHoldTime = float64(hotspot.AvgHoldTime.Nanoseconds()) / 1e6 // Convert to milliseconds
		metrics.LastUpdated = time.Now()

		// Get detailed profile for this resource
		profile := me.profiler.GetResourceProfile(resource)
		if profile.ContentionInfo != nil {
			metrics.AvgWaitTime = float64(profile.ContentionInfo.AvgWaitTime) / 1e6 // Convert to milliseconds
			metrics.MaxWaitTime = float64(profile.ContentionInfo.MaxWaitTime) / 1e6
			metrics.MaxQueueDepth = profile.ContentionInfo.MaxQueueDepth
		}
	}
}

// handleMetrics handles HTTP requests for metrics
func (me *MetricsExporter) handleMetrics(w http.ResponseWriter, r *http.Request) {
	me.mutex.RLock()
	defer me.mutex.RUnlock()

	// Create snapshot
	snapshot := &MetricsSnapshot{
		Timestamp:       time.Now(),
		GlobalMetrics:   me.globalMetrics,
		ResourceMetrics: me.lockMetrics,
	}

	// Add top hotspots if profiler is available
	if me.profiler != nil && me.profiler.IsEnabled() {
		snapshot.TopHotspots = me.profiler.GetTopHotspots(10)
	}

	// Set content type
	w.Header().Set("Content-Type", "application/json")

	// Encode and send response
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(snapshot); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode metrics: %v", err), http.StatusInternalServerError)
		return
	}
}

// handleHealth handles health check requests
func (me *MetricsExporter) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"running":   atomic.LoadInt32(&me.running) == 1,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// GetSnapshot returns a current metrics snapshot
func (me *MetricsExporter) GetSnapshot() *MetricsSnapshot {
	me.mutex.RLock()
	defer me.mutex.RUnlock()

	// Create a deep copy of the metrics
	resourceMetrics := make(map[string]*LockResourceMetrics)
	for k, v := range me.lockMetrics {
		resourceMetrics[k] = &LockResourceMetrics{
			Resource:          v.Resource,
			RequestsTotal:     v.RequestsTotal,
			AcquisitionsTotal: v.AcquisitionsTotal,
			ReleasesTotal:     v.ReleasesTotal,
			TimeoutsTotal:     v.TimeoutsTotal,
			ContentionTotal:   v.ContentionTotal,
			AvgWaitTime:       v.AvgWaitTime,
			MaxWaitTime:       v.MaxWaitTime,
			AvgHoldTime:       v.AvgHoldTime,
			MaxHoldTime:       v.MaxHoldTime,
			CurrentQueueDepth: v.CurrentQueueDepth,
			MaxQueueDepth:     v.MaxQueueDepth,
			LastUpdated:       v.LastUpdated,
		}
	}

	globalMetrics := &GlobalMetrics{
		TotalLockRequests:     me.globalMetrics.TotalLockRequests,
		TotalLockAcquisitions: me.globalMetrics.TotalLockAcquisitions,
		TotalLockReleases:     me.globalMetrics.TotalLockReleases,
		TotalLockTimeouts:     me.globalMetrics.TotalLockTimeouts,
		TotalContentionEvents: me.globalMetrics.TotalContentionEvents,
		ActiveLocks:           me.globalMetrics.ActiveLocks,
		MaxConcurrentLocks:    me.globalMetrics.MaxConcurrentLocks,
		DeadlocksDetected:     me.globalMetrics.DeadlocksDetected,
		DeadlocksResolved:     me.globalMetrics.DeadlocksResolved,
		FastPathHits:          me.globalMetrics.FastPathHits,
		FastPathMisses:        me.globalMetrics.FastPathMisses,
		SystemUptime:          me.globalMetrics.SystemUptime,
		LastUpdated:           me.globalMetrics.LastUpdated,
	}

	snapshot := &MetricsSnapshot{
		Timestamp:       time.Now(),
		GlobalMetrics:   globalMetrics,
		ResourceMetrics: resourceMetrics,
	}

	// Add top hotspots if profiler is available
	if me.profiler != nil && me.profiler.IsEnabled() {
		snapshot.TopHotspots = me.profiler.GetTopHotspots(10)
	}

	return snapshot
}

// RecordLockRequest records a lock request event
func (me *MetricsExporter) RecordLockRequest(resource string, lockType LockType) {
	me.mutex.Lock()
	defer me.mutex.Unlock()

	if me.lockMetrics[resource] == nil {
		me.lockMetrics[resource] = &LockResourceMetrics{
			Resource: resource,
		}
	}

	me.lockMetrics[resource].RequestsTotal++
	me.globalMetrics.TotalLockRequests++
}

// RecordLockAcquisition records a lock acquisition event
func (me *MetricsExporter) RecordLockAcquisition(resource string, lockType LockType, waitTime time.Duration) {
	me.mutex.Lock()
	defer me.mutex.Unlock()

	if me.lockMetrics[resource] == nil {
		me.lockMetrics[resource] = &LockResourceMetrics{
			Resource: resource,
		}
	}

	metrics := me.lockMetrics[resource]
	metrics.AcquisitionsTotal++

	waitTimeMs := float64(waitTime.Nanoseconds()) / 1e6
	if waitTimeMs > metrics.MaxWaitTime {
		metrics.MaxWaitTime = waitTimeMs
	}

	// Update average (simplified)
	if metrics.AcquisitionsTotal > 0 {
		metrics.AvgWaitTime = (metrics.AvgWaitTime*float64(metrics.AcquisitionsTotal-1) + waitTimeMs) / float64(metrics.AcquisitionsTotal)
	}

	me.globalMetrics.TotalLockAcquisitions++
}

// RecordLockRelease records a lock release event
func (me *MetricsExporter) RecordLockRelease(resource string, lockType LockType, holdTime time.Duration) {
	me.mutex.Lock()
	defer me.mutex.Unlock()

	if me.lockMetrics[resource] == nil {
		me.lockMetrics[resource] = &LockResourceMetrics{
			Resource: resource,
		}
	}

	metrics := me.lockMetrics[resource]
	metrics.ReleasesTotal++

	holdTimeMs := float64(holdTime.Nanoseconds()) / 1e6
	if holdTimeMs > metrics.MaxHoldTime {
		metrics.MaxHoldTime = holdTimeMs
	}

	// Update average (simplified)
	if metrics.ReleasesTotal > 0 {
		metrics.AvgHoldTime = (metrics.AvgHoldTime*float64(metrics.ReleasesTotal-1) + holdTimeMs) / float64(metrics.ReleasesTotal)
	}

	me.globalMetrics.TotalLockReleases++
}

// RecordLockTimeout records a lock timeout event
func (me *MetricsExporter) RecordLockTimeout(resource string, lockType LockType) {
	me.mutex.Lock()
	defer me.mutex.Unlock()

	if me.lockMetrics[resource] == nil {
		me.lockMetrics[resource] = &LockResourceMetrics{
			Resource: resource,
		}
	}

	me.lockMetrics[resource].TimeoutsTotal++
	me.globalMetrics.TotalLockTimeouts++
}

// RecordLockContention records a lock contention event
func (me *MetricsExporter) RecordLockContention(resource string) {
	me.mutex.Lock()
	defer me.mutex.Unlock()

	if me.lockMetrics[resource] == nil {
		me.lockMetrics[resource] = &LockResourceMetrics{
			Resource: resource,
		}
	}

	me.lockMetrics[resource].ContentionTotal++
	me.globalMetrics.TotalContentionEvents++
}
