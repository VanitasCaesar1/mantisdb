package memory

import (
	"runtime"
	"sync"
	"time"
)

// MemoryMonitor tracks system memory usage and provides alerts
type MemoryMonitor struct {
	interval   time.Duration
	thresholds map[string]float64
	callbacks  map[string][]MemoryCallback
	stats      *MemoryStats
	mutex      sync.RWMutex
	stopCh     chan struct{}
	running    bool
}

// MemoryCallback is called when memory thresholds are exceeded
type MemoryCallback func(stats *MemoryStats, threshold float64)

// MemoryStats holds current memory statistics
type MemoryStats struct {
	// Go runtime memory stats
	Alloc         uint64    // Bytes allocated and in use
	TotalAlloc    uint64    // Total bytes allocated (cumulative)
	Sys           uint64    // Bytes obtained from system
	Lookups       uint64    // Number of pointer lookups
	Mallocs       uint64    // Number of mallocs
	Frees         uint64    // Number of frees
	HeapAlloc     uint64    // Bytes allocated and in use in heap
	HeapSys       uint64    // Bytes obtained from system for heap
	HeapIdle      uint64    // Bytes in idle spans
	HeapInuse     uint64    // Bytes in non-idle spans
	HeapReleased  uint64    // Bytes released to the OS
	HeapObjects   uint64    // Number of allocated objects
	StackInuse    uint64    // Bytes used by stack spans
	StackSys      uint64    // Bytes obtained from system for stack
	MSpanInuse    uint64    // Bytes used by mspan structures
	MSpanSys      uint64    // Bytes obtained from system for mspan
	MCacheInuse   uint64    // Bytes used by mcache structures
	MCacheSys     uint64    // Bytes obtained from system for mcache
	BuckHashSys   uint64    // Bytes used by profiling bucket hash table
	GCSys         uint64    // Bytes used for garbage collection metadata
	OtherSys      uint64    // Bytes used for other system allocations
	NextGC        uint64    // Target heap size for next GC
	LastGC        time.Time // Time of last garbage collection
	PauseTotalNs  uint64    // Total GC pause time in nanoseconds
	NumGC         uint32    // Number of completed GC cycles
	NumForcedGC   uint32    // Number of forced GC cycles
	GCCPUFraction float64   // Fraction of CPU time used by GC

	// Calculated metrics
	MemoryUsage float64   // Alloc / Sys ratio
	HeapUsage   float64   // HeapInuse / HeapSys ratio
	GCPressure  float64   // GC frequency indicator
	Timestamp   time.Time // When stats were collected
}

// NewMemoryMonitor creates a new memory monitor
func NewMemoryMonitor(interval time.Duration) *MemoryMonitor {
	return &MemoryMonitor{
		interval:   interval,
		thresholds: make(map[string]float64),
		callbacks:  make(map[string][]MemoryCallback),
		stats:      &MemoryStats{},
		stopCh:     make(chan struct{}),
	}
}

// Start begins memory monitoring
func (mm *MemoryMonitor) Start() {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	if mm.running {
		return
	}

	mm.running = true
	go mm.monitor()
}

// Stop stops memory monitoring
func (mm *MemoryMonitor) Stop() {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	if !mm.running {
		return
	}

	mm.running = false
	close(mm.stopCh)
}

// SetThreshold sets a memory usage threshold with a callback
func (mm *MemoryMonitor) SetThreshold(name string, threshold float64, callback MemoryCallback) {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	mm.thresholds[name] = threshold
	mm.callbacks[name] = append(mm.callbacks[name], callback)
}

// RemoveThreshold removes a memory threshold
func (mm *MemoryMonitor) RemoveThreshold(name string) {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	delete(mm.thresholds, name)
	delete(mm.callbacks, name)
}

// GetStats returns current memory statistics
func (mm *MemoryMonitor) GetStats() *MemoryStats {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()

	// Create a copy to avoid race conditions
	stats := *mm.stats
	return &stats
}

// ForceGC triggers garbage collection and updates stats
func (mm *MemoryMonitor) ForceGC() {
	runtime.GC()
	mm.updateStats()
}

// Private methods

func (mm *MemoryMonitor) monitor() {
	ticker := time.NewTicker(mm.interval)
	defer ticker.Stop()

	// Initial stats collection
	mm.updateStats()

	for {
		select {
		case <-ticker.C:
			mm.updateStats()
			mm.checkThresholds()
		case <-mm.stopCh:
			return
		}
	}
}

func (mm *MemoryMonitor) updateStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	mm.stats.Alloc = m.Alloc
	mm.stats.TotalAlloc = m.TotalAlloc
	mm.stats.Sys = m.Sys
	mm.stats.Lookups = m.Lookups
	mm.stats.Mallocs = m.Mallocs
	mm.stats.Frees = m.Frees
	mm.stats.HeapAlloc = m.HeapAlloc
	mm.stats.HeapSys = m.HeapSys
	mm.stats.HeapIdle = m.HeapIdle
	mm.stats.HeapInuse = m.HeapInuse
	mm.stats.HeapReleased = m.HeapReleased
	mm.stats.HeapObjects = m.HeapObjects
	mm.stats.StackInuse = m.StackInuse
	mm.stats.StackSys = m.StackSys
	mm.stats.MSpanInuse = m.MSpanInuse
	mm.stats.MSpanSys = m.MSpanSys
	mm.stats.MCacheInuse = m.MCacheInuse
	mm.stats.MCacheSys = m.MCacheSys
	mm.stats.BuckHashSys = m.BuckHashSys
	mm.stats.GCSys = m.GCSys
	mm.stats.OtherSys = m.OtherSys
	mm.stats.NextGC = m.NextGC
	mm.stats.LastGC = time.Unix(0, int64(m.LastGC))
	mm.stats.PauseTotalNs = m.PauseTotalNs
	mm.stats.NumGC = m.NumGC
	mm.stats.NumForcedGC = m.NumForcedGC
	mm.stats.GCCPUFraction = m.GCCPUFraction

	// Calculate derived metrics
	if mm.stats.Sys > 0 {
		mm.stats.MemoryUsage = float64(mm.stats.Alloc) / float64(mm.stats.Sys)
	}

	if mm.stats.HeapSys > 0 {
		mm.stats.HeapUsage = float64(mm.stats.HeapInuse) / float64(mm.stats.HeapSys)
	}

	// Calculate GC pressure (GCs per minute)
	if mm.stats.NumGC > 0 {
		timeSinceStart := time.Since(mm.stats.LastGC)
		if timeSinceStart > 0 {
			mm.stats.GCPressure = float64(mm.stats.NumGC) / timeSinceStart.Minutes()
		}
	}

	mm.stats.Timestamp = time.Now()
}

func (mm *MemoryMonitor) checkThresholds() {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()

	for name, threshold := range mm.thresholds {
		var currentUsage float64

		switch name {
		case "memory":
			currentUsage = mm.stats.MemoryUsage
		case "heap":
			currentUsage = mm.stats.HeapUsage
		case "gc_pressure":
			currentUsage = mm.stats.GCPressure
		default:
			continue
		}

		if currentUsage > threshold {
			if callbacks, exists := mm.callbacks[name]; exists {
				for _, callback := range callbacks {
					go callback(mm.stats, threshold)
				}
			}
		}
	}
}

// MemoryAlert represents a memory threshold alert
type MemoryAlert struct {
	Name      string
	Threshold float64
	Current   float64
	Timestamp time.Time
	Stats     *MemoryStats
}

// AlertManager manages memory alerts and notifications
type AlertManager struct {
	alerts    []MemoryAlert
	mutex     sync.RWMutex
	maxAlerts int
}

// NewAlertManager creates a new alert manager
func NewAlertManager(maxAlerts int) *AlertManager {
	return &AlertManager{
		alerts:    make([]MemoryAlert, 0),
		maxAlerts: maxAlerts,
	}
}

// AddAlert adds a new memory alert
func (am *AlertManager) AddAlert(alert MemoryAlert) {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	am.alerts = append(am.alerts, alert)

	// Keep only the most recent alerts
	if len(am.alerts) > am.maxAlerts {
		am.alerts = am.alerts[len(am.alerts)-am.maxAlerts:]
	}
}

// GetAlerts returns recent memory alerts
func (am *AlertManager) GetAlerts() []MemoryAlert {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	// Return a copy to avoid race conditions
	alerts := make([]MemoryAlert, len(am.alerts))
	copy(alerts, am.alerts)
	return alerts
}

// ClearAlerts removes all alerts
func (am *AlertManager) ClearAlerts() {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	am.alerts = am.alerts[:0]
}
