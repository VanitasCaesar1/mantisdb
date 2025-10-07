package concurrency

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// LockProfiler provides detailed profiling and analysis of lock performance
type LockProfiler struct {
	// Configuration
	enabled           int32 // atomic
	samplingRate      float64
	maxProfileEntries int

	// Profiling data
	lockEvents    sync.Map // resource -> *LockEventHistory
	contentionMap sync.Map // resource -> *ContentionInfo
	hotspots      sync.Map // resource -> *HotspotInfo

	// Aggregated metrics
	globalStats   *GlobalLockStats
	periodicStats *PeriodicStats

	// Background processing
	processingInterval time.Duration
	stopChan           chan struct{}
	wg                 sync.WaitGroup
	running            int32 // atomic
}

// LockEventHistory tracks the history of lock events for a resource
type LockEventHistory struct {
	Resource    string
	Events      []LockEvent
	MaxEvents   int
	mutex       sync.RWMutex
	TotalEvents int64 // atomic
}

// LockEvent represents a single lock-related event
type LockEvent struct {
	Timestamp  time.Time
	TxnID      uint64
	EventType  LockEventType
	LockType   LockType
	Duration   time.Duration // For acquire/release events
	WaitTime   time.Duration // Time spent waiting
	QueueDepth int           // Queue depth at time of event
}

// LockEventType defines the type of lock event
type LockEventType int

const (
	EventLockRequested LockEventType = iota
	EventLockAcquired
	EventLockReleased
	EventLockTimeout
	EventLockContention
	EventDeadlockDetected
)

func (e LockEventType) String() string {
	switch e {
	case EventLockRequested:
		return "REQUESTED"
	case EventLockAcquired:
		return "ACQUIRED"
	case EventLockReleased:
		return "RELEASED"
	case EventLockTimeout:
		return "TIMEOUT"
	case EventLockContention:
		return "CONTENTION"
	case EventDeadlockDetected:
		return "DEADLOCK"
	default:
		return "UNKNOWN"
	}
}

// ContentionInfo tracks contention statistics for a resource
type ContentionInfo struct {
	Resource           string
	ContentionCount    int64 // atomic
	TotalWaitTime      int64 // atomic, nanoseconds
	MaxWaitTime        int64 // atomic, nanoseconds
	AvgWaitTime        int64 // atomic, nanoseconds
	MaxQueueDepth      int32 // atomic
	LastContentionTime time.Time
	mutex              sync.RWMutex
}

// HotspotInfo identifies lock hotspots
type HotspotInfo struct {
	Resource       string
	AccessCount    int64   // atomic
	ContentionRate float64 // contention events / total accesses
	AvgHoldTime    time.Duration
	LastAccessTime time.Time
	HotspotScore   float64 // Composite score indicating hotspot severity
	mutex          sync.RWMutex
}

// GlobalLockStats tracks system-wide lock statistics
type GlobalLockStats struct {
	// Counters
	TotalLockRequests     int64 // atomic
	TotalLockAcquisitions int64 // atomic
	TotalLockReleases     int64 // atomic
	TotalLockTimeouts     int64 // atomic
	TotalContentionEvents int64 // atomic

	// Timing
	TotalWaitTime int64 // atomic, nanoseconds
	TotalHoldTime int64 // atomic, nanoseconds
	MaxWaitTime   int64 // atomic, nanoseconds
	MaxHoldTime   int64 // atomic, nanoseconds

	// Concurrency
	MaxConcurrentLocks int32 // atomic
	CurrentActiveLocks int32 // atomic

	// Performance
	AvgAcquisitionTime int64 // atomic, nanoseconds
	AvgContentionTime  int64 // atomic, nanoseconds

	// Last update
	LastUpdateTime time.Time
	mutex          sync.RWMutex
}

// PeriodicStats tracks statistics over time periods
type PeriodicStats struct {
	periods    []StatsPeriod
	maxPeriods int
	mutex      sync.RWMutex
}

// StatsPeriod represents statistics for a specific time period
type StatsPeriod struct {
	StartTime        time.Time
	EndTime          time.Time
	LockRequests     int64
	LockAcquisitions int64
	LockTimeouts     int64
	ContentionEvents int64
	AvgWaitTime      time.Duration
	MaxWaitTime      time.Duration
	ThroughputLPS    float64 // Locks per second
}

// NewLockProfiler creates a new lock profiler
func NewLockProfiler(config *LockProfilerConfig) *LockProfiler {
	if config == nil {
		config = DefaultLockProfilerConfig()
	}

	return &LockProfiler{
		enabled:            1,
		samplingRate:       config.SamplingRate,
		maxProfileEntries:  config.MaxProfileEntries,
		globalStats:        &GlobalLockStats{},
		periodicStats:      &PeriodicStats{maxPeriods: config.MaxPeriodicStats},
		processingInterval: config.ProcessingInterval,
		stopChan:           make(chan struct{}),
	}
}

// LockProfilerConfig holds configuration for the lock profiler
type LockProfilerConfig struct {
	SamplingRate       float64
	MaxProfileEntries  int
	MaxPeriodicStats   int
	ProcessingInterval time.Duration
}

// DefaultLockProfilerConfig returns default profiler configuration
func DefaultLockProfilerConfig() *LockProfilerConfig {
	return &LockProfilerConfig{
		SamplingRate:       1.0, // Profile all events initially
		MaxProfileEntries:  10000,
		MaxPeriodicStats:   100,
		ProcessingInterval: 10 * time.Second,
	}
}

// Start begins the lock profiler background processing
func (lp *LockProfiler) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&lp.running, 0, 1) {
		return fmt.Errorf("lock profiler is already running")
	}

	lp.wg.Add(1)
	go lp.processingLoop(ctx)

	return nil
}

// Stop stops the lock profiler
func (lp *LockProfiler) Stop() error {
	if !atomic.CompareAndSwapInt32(&lp.running, 1, 0) {
		return nil
	}

	close(lp.stopChan)
	lp.wg.Wait()

	return nil
}

// RecordLockEvent records a lock event for profiling
func (lp *LockProfiler) RecordLockEvent(resource string, txnID uint64, eventType LockEventType,
	lockType LockType, duration, waitTime time.Duration, queueDepth int) {

	if atomic.LoadInt32(&lp.enabled) == 0 {
		return
	}

	// Sample events based on sampling rate
	if lp.samplingRate < 1.0 && time.Now().UnixNano()%100 >= int64(lp.samplingRate*100) {
		return
	}

	event := LockEvent{
		Timestamp:  time.Now(),
		TxnID:      txnID,
		EventType:  eventType,
		LockType:   lockType,
		Duration:   duration,
		WaitTime:   waitTime,
		QueueDepth: queueDepth,
	}

	// Record in event history
	lp.recordEventHistory(resource, event)

	// Update contention info
	if eventType == EventLockContention || waitTime > 0 {
		lp.updateContentionInfo(resource, waitTime, queueDepth)
	}

	// Update hotspot info
	lp.updateHotspotInfo(resource, eventType, duration)

	// Update global stats
	lp.updateGlobalStats(eventType, duration, waitTime)
}

// recordEventHistory records an event in the resource's event history
func (lp *LockProfiler) recordEventHistory(resource string, event LockEvent) {
	value, _ := lp.lockEvents.LoadOrStore(resource, &LockEventHistory{
		Resource:  resource,
		Events:    make([]LockEvent, 0, lp.maxProfileEntries),
		MaxEvents: lp.maxProfileEntries,
	})

	history := value.(*LockEventHistory)
	history.mutex.Lock()
	defer history.mutex.Unlock()

	// Add event to history
	if len(history.Events) >= history.MaxEvents {
		// Remove oldest event
		copy(history.Events, history.Events[1:])
		history.Events = history.Events[:len(history.Events)-1]
	}

	history.Events = append(history.Events, event)
	atomic.AddInt64(&history.TotalEvents, 1)
}

// updateContentionInfo updates contention statistics
func (lp *LockProfiler) updateContentionInfo(resource string, waitTime time.Duration, queueDepth int) {
	value, _ := lp.contentionMap.LoadOrStore(resource, &ContentionInfo{
		Resource: resource,
	})

	info := value.(*ContentionInfo)

	atomic.AddInt64(&info.ContentionCount, 1)
	waitNanos := waitTime.Nanoseconds()
	atomic.AddInt64(&info.TotalWaitTime, waitNanos)

	// Update max wait time
	for {
		current := atomic.LoadInt64(&info.MaxWaitTime)
		if waitNanos <= current || atomic.CompareAndSwapInt64(&info.MaxWaitTime, current, waitNanos) {
			break
		}
	}

	// Update average wait time
	count := atomic.LoadInt64(&info.ContentionCount)
	total := atomic.LoadInt64(&info.TotalWaitTime)
	atomic.StoreInt64(&info.AvgWaitTime, total/count)

	// Update max queue depth
	for {
		current := atomic.LoadInt32(&info.MaxQueueDepth)
		if int32(queueDepth) <= current || atomic.CompareAndSwapInt32(&info.MaxQueueDepth, current, int32(queueDepth)) {
			break
		}
	}

	info.mutex.Lock()
	info.LastContentionTime = time.Now()
	info.mutex.Unlock()
}

// updateHotspotInfo updates hotspot statistics
func (lp *LockProfiler) updateHotspotInfo(resource string, eventType LockEventType, duration time.Duration) {
	value, _ := lp.hotspots.LoadOrStore(resource, &HotspotInfo{
		Resource: resource,
	})

	info := value.(*HotspotInfo)

	atomic.AddInt64(&info.AccessCount, 1)

	info.mutex.Lock()
	defer info.mutex.Unlock()

	info.LastAccessTime = time.Now()

	// Update average hold time
	if eventType == EventLockReleased && duration > 0 {
		if info.AvgHoldTime == 0 {
			info.AvgHoldTime = duration
		} else {
			info.AvgHoldTime = (info.AvgHoldTime + duration) / 2
		}
	}

	// Calculate contention rate
	if contentionValue, exists := lp.contentionMap.Load(resource); exists {
		contentionInfo := contentionValue.(*ContentionInfo)
		contentionCount := atomic.LoadInt64(&contentionInfo.ContentionCount)
		accessCount := atomic.LoadInt64(&info.AccessCount)

		if accessCount > 0 {
			info.ContentionRate = float64(contentionCount) / float64(accessCount)
		}
	}

	// Calculate hotspot score (higher = more problematic)
	info.HotspotScore = info.ContentionRate * float64(info.AvgHoldTime.Nanoseconds()) / 1e6 // Convert to milliseconds
}

// updateGlobalStats updates system-wide statistics
func (lp *LockProfiler) updateGlobalStats(eventType LockEventType, duration, waitTime time.Duration) {
	switch eventType {
	case EventLockRequested:
		atomic.AddInt64(&lp.globalStats.TotalLockRequests, 1)
	case EventLockAcquired:
		atomic.AddInt64(&lp.globalStats.TotalLockAcquisitions, 1)
		atomic.AddInt32(&lp.globalStats.CurrentActiveLocks, 1)

		// Update max concurrent locks
		current := atomic.LoadInt32(&lp.globalStats.CurrentActiveLocks)
		for {
			max := atomic.LoadInt32(&lp.globalStats.MaxConcurrentLocks)
			if current <= max || atomic.CompareAndSwapInt32(&lp.globalStats.MaxConcurrentLocks, max, current) {
				break
			}
		}

	case EventLockReleased:
		atomic.AddInt64(&lp.globalStats.TotalLockReleases, 1)
		atomic.AddInt32(&lp.globalStats.CurrentActiveLocks, -1)

		if duration > 0 {
			holdNanos := duration.Nanoseconds()
			atomic.AddInt64(&lp.globalStats.TotalHoldTime, holdNanos)

			// Update max hold time
			for {
				max := atomic.LoadInt64(&lp.globalStats.MaxHoldTime)
				if holdNanos <= max || atomic.CompareAndSwapInt64(&lp.globalStats.MaxHoldTime, max, holdNanos) {
					break
				}
			}
		}

	case EventLockTimeout:
		atomic.AddInt64(&lp.globalStats.TotalLockTimeouts, 1)

	case EventLockContention:
		atomic.AddInt64(&lp.globalStats.TotalContentionEvents, 1)
	}

	if waitTime > 0 {
		waitNanos := waitTime.Nanoseconds()
		atomic.AddInt64(&lp.globalStats.TotalWaitTime, waitNanos)

		// Update max wait time
		for {
			max := atomic.LoadInt64(&lp.globalStats.MaxWaitTime)
			if waitNanos <= max || atomic.CompareAndSwapInt64(&lp.globalStats.MaxWaitTime, max, waitNanos) {
				break
			}
		}
	}

	lp.globalStats.mutex.Lock()
	lp.globalStats.LastUpdateTime = time.Now()
	lp.globalStats.mutex.Unlock()
}

// processingLoop runs background processing for the profiler
func (lp *LockProfiler) processingLoop(ctx context.Context) {
	defer lp.wg.Done()

	ticker := time.NewTicker(lp.processingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			lp.processPeriodicStats()
			lp.cleanupOldData()
		case <-lp.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// processPeriodicStats processes and stores periodic statistics
func (lp *LockProfiler) processPeriodicStats() {
	now := time.Now()

	// Calculate stats for the current period
	period := StatsPeriod{
		StartTime:        now.Add(-lp.processingInterval),
		EndTime:          now,
		LockRequests:     atomic.LoadInt64(&lp.globalStats.TotalLockRequests),
		LockAcquisitions: atomic.LoadInt64(&lp.globalStats.TotalLockAcquisitions),
		LockTimeouts:     atomic.LoadInt64(&lp.globalStats.TotalLockTimeouts),
		ContentionEvents: atomic.LoadInt64(&lp.globalStats.TotalContentionEvents),
	}

	// Calculate average wait time
	totalWait := atomic.LoadInt64(&lp.globalStats.TotalWaitTime)
	totalRequests := atomic.LoadInt64(&lp.globalStats.TotalLockRequests)
	if totalRequests > 0 {
		period.AvgWaitTime = time.Duration(totalWait / totalRequests)
	}

	period.MaxWaitTime = time.Duration(atomic.LoadInt64(&lp.globalStats.MaxWaitTime))

	// Calculate throughput (locks per second)
	if lp.processingInterval.Seconds() > 0 {
		period.ThroughputLPS = float64(period.LockAcquisitions) / lp.processingInterval.Seconds()
	}

	// Store the period
	lp.periodicStats.mutex.Lock()
	lp.periodicStats.periods = append(lp.periodicStats.periods, period)

	// Remove old periods if we exceed the limit
	if len(lp.periodicStats.periods) > lp.periodicStats.maxPeriods {
		lp.periodicStats.periods = lp.periodicStats.periods[1:]
	}
	lp.periodicStats.mutex.Unlock()
}

// cleanupOldData removes old profiling data to prevent memory leaks
func (lp *LockProfiler) cleanupOldData() {
	cutoff := time.Now().Add(-time.Hour) // Keep data for 1 hour

	// Clean up event histories
	lp.lockEvents.Range(func(key, value interface{}) bool {
		history := value.(*LockEventHistory)
		history.mutex.Lock()

		// Remove events older than cutoff
		newEvents := make([]LockEvent, 0, len(history.Events))
		for _, event := range history.Events {
			if event.Timestamp.After(cutoff) {
				newEvents = append(newEvents, event)
			}
		}
		history.Events = newEvents

		history.mutex.Unlock()
		return true
	})
}

// GetResourceProfile returns profiling information for a specific resource
func (lp *LockProfiler) GetResourceProfile(resource string) *ResourceProfile {
	profile := &ResourceProfile{
		Resource: resource,
	}

	// Get event history
	if value, exists := lp.lockEvents.Load(resource); exists {
		history := value.(*LockEventHistory)
		history.mutex.RLock()
		profile.Events = make([]LockEvent, len(history.Events))
		copy(profile.Events, history.Events)
		profile.TotalEvents = atomic.LoadInt64(&history.TotalEvents)
		history.mutex.RUnlock()
	}

	// Get contention info
	if value, exists := lp.contentionMap.Load(resource); exists {
		info := value.(*ContentionInfo)
		profile.ContentionInfo = &ContentionInfo{
			Resource:           info.Resource,
			ContentionCount:    atomic.LoadInt64(&info.ContentionCount),
			TotalWaitTime:      atomic.LoadInt64(&info.TotalWaitTime),
			MaxWaitTime:        atomic.LoadInt64(&info.MaxWaitTime),
			AvgWaitTime:        atomic.LoadInt64(&info.AvgWaitTime),
			MaxQueueDepth:      atomic.LoadInt32(&info.MaxQueueDepth),
			LastContentionTime: info.LastContentionTime,
		}
	}

	// Get hotspot info
	if value, exists := lp.hotspots.Load(resource); exists {
		info := value.(*HotspotInfo)
		info.mutex.RLock()
		profile.HotspotInfo = &HotspotInfo{
			Resource:       info.Resource,
			AccessCount:    atomic.LoadInt64(&info.AccessCount),
			ContentionRate: info.ContentionRate,
			AvgHoldTime:    info.AvgHoldTime,
			LastAccessTime: info.LastAccessTime,
			HotspotScore:   info.HotspotScore,
		}
		info.mutex.RUnlock()
	}

	return profile
}

// GetTopHotspots returns the top N lock hotspots
func (lp *LockProfiler) GetTopHotspots(n int) []*HotspotInfo {
	hotspots := make([]*HotspotInfo, 0)

	lp.hotspots.Range(func(key, value interface{}) bool {
		info := value.(*HotspotInfo)
		info.mutex.RLock()
		hotspotCopy := &HotspotInfo{
			Resource:       info.Resource,
			AccessCount:    atomic.LoadInt64(&info.AccessCount),
			ContentionRate: info.ContentionRate,
			AvgHoldTime:    info.AvgHoldTime,
			LastAccessTime: info.LastAccessTime,
			HotspotScore:   info.HotspotScore,
		}
		info.mutex.RUnlock()
		hotspots = append(hotspots, hotspotCopy)
		return true
	})

	// Sort by hotspot score (descending)
	sort.Slice(hotspots, func(i, j int) bool {
		return hotspots[i].HotspotScore > hotspots[j].HotspotScore
	})

	if len(hotspots) > n {
		hotspots = hotspots[:n]
	}

	return hotspots
}

// GetGlobalStats returns current global lock statistics
func (lp *LockProfiler) GetGlobalStats() *GlobalLockStats {
	lp.globalStats.mutex.RLock()
	defer lp.globalStats.mutex.RUnlock()

	return &GlobalLockStats{
		TotalLockRequests:     atomic.LoadInt64(&lp.globalStats.TotalLockRequests),
		TotalLockAcquisitions: atomic.LoadInt64(&lp.globalStats.TotalLockAcquisitions),
		TotalLockReleases:     atomic.LoadInt64(&lp.globalStats.TotalLockReleases),
		TotalLockTimeouts:     atomic.LoadInt64(&lp.globalStats.TotalLockTimeouts),
		TotalContentionEvents: atomic.LoadInt64(&lp.globalStats.TotalContentionEvents),
		TotalWaitTime:         atomic.LoadInt64(&lp.globalStats.TotalWaitTime),
		TotalHoldTime:         atomic.LoadInt64(&lp.globalStats.TotalHoldTime),
		MaxWaitTime:           atomic.LoadInt64(&lp.globalStats.MaxWaitTime),
		MaxHoldTime:           atomic.LoadInt64(&lp.globalStats.MaxHoldTime),
		MaxConcurrentLocks:    atomic.LoadInt32(&lp.globalStats.MaxConcurrentLocks),
		CurrentActiveLocks:    atomic.LoadInt32(&lp.globalStats.CurrentActiveLocks),
		AvgAcquisitionTime:    atomic.LoadInt64(&lp.globalStats.AvgAcquisitionTime),
		AvgContentionTime:     atomic.LoadInt64(&lp.globalStats.AvgContentionTime),
		LastUpdateTime:        lp.globalStats.LastUpdateTime,
	}
}

// GetPeriodicStats returns periodic statistics
func (lp *LockProfiler) GetPeriodicStats() []StatsPeriod {
	lp.periodicStats.mutex.RLock()
	defer lp.periodicStats.mutex.RUnlock()

	result := make([]StatsPeriod, len(lp.periodicStats.periods))
	copy(result, lp.periodicStats.periods)
	return result
}

// ResourceProfile contains comprehensive profiling information for a resource
type ResourceProfile struct {
	Resource       string
	Events         []LockEvent
	TotalEvents    int64
	ContentionInfo *ContentionInfo
	HotspotInfo    *HotspotInfo
}

// Enable enables the lock profiler
func (lp *LockProfiler) Enable() {
	atomic.StoreInt32(&lp.enabled, 1)
}

// Disable disables the lock profiler
func (lp *LockProfiler) Disable() {
	atomic.StoreInt32(&lp.enabled, 0)
}

// IsEnabled returns whether the profiler is enabled
func (lp *LockProfiler) IsEnabled() bool {
	return atomic.LoadInt32(&lp.enabled) == 1
}

// SetSamplingRate sets the sampling rate for event recording
func (lp *LockProfiler) SetSamplingRate(rate float64) {
	if rate < 0 {
		rate = 0
	} else if rate > 1 {
		rate = 1
	}
	lp.samplingRate = rate
}
