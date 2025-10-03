package errors

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// MemoryMonitor monitors memory usage and handles exhaustion scenarios
type MemoryMonitor struct {
	mu               sync.RWMutex
	config           *MemoryMonitorConfig
	alertCallbacks   []MemoryAlertCallback
	isMonitoring     bool
	stopChan         chan struct{}
	currentStats     *MemoryStats
	degradationLevel DegradationLevel
}

// MemoryMonitorConfig contains configuration for memory monitoring
type MemoryMonitorConfig struct {
	CheckInterval         time.Duration `json:"check_interval"`
	WarningThreshold      float64       `json:"warning_threshold"`   // Percentage of max memory (e.g., 0.70 for 70%)
	CriticalThreshold     float64       `json:"critical_threshold"`  // Percentage of max memory (e.g., 0.85 for 85%)
	EmergencyThreshold    float64       `json:"emergency_threshold"` // Percentage of max memory (e.g., 0.95 for 95%)
	MaxMemoryBytes        uint64        `json:"max_memory_bytes"`    // Maximum allowed memory usage (0 = auto-detect)
	EnableAlerts          bool          `json:"enable_alerts"`
	EnableGracefulDegrade bool          `json:"enable_graceful_degrade"`
	GCThreshold           float64       `json:"gc_threshold"`       // Trigger GC at this memory usage %
	ForceGCThreshold      float64       `json:"force_gc_threshold"` // Force aggressive GC at this %
}

// MemoryStats contains current memory usage statistics
type MemoryStats struct {
	AllocBytes      uint64       `json:"alloc_bytes"`       // Currently allocated bytes
	TotalAllocBytes uint64       `json:"total_alloc_bytes"` // Total allocated bytes (cumulative)
	SysBytes        uint64       `json:"sys_bytes"`         // System memory obtained from OS
	HeapBytes       uint64       `json:"heap_bytes"`        // Heap memory
	StackBytes      uint64       `json:"stack_bytes"`       // Stack memory
	GCCycles        uint32       `json:"gc_cycles"`         // Number of GC cycles
	LastGC          time.Time    `json:"last_gc"`           // Last GC time
	UsagePercent    float64      `json:"usage_percent"`     // Memory usage percentage
	Status          MemoryStatus `json:"status"`            // Current memory status
	Timestamp       time.Time    `json:"timestamp"`         // When stats were collected
}

// MemoryStatus represents the current memory status
type MemoryStatus int

const (
	MemoryStatusOK MemoryStatus = iota
	MemoryStatusWarning
	MemoryStatusCritical
	MemoryStatusEmergency
	MemoryStatusExhausted
)

func (s MemoryStatus) String() string {
	switch s {
	case MemoryStatusOK:
		return "OK"
	case MemoryStatusWarning:
		return "WARNING"
	case MemoryStatusCritical:
		return "CRITICAL"
	case MemoryStatusEmergency:
		return "EMERGENCY"
	case MemoryStatusExhausted:
		return "EXHAUSTED"
	default:
		return "UNKNOWN"
	}
}

// DegradationLevel represents the current level of graceful degradation
type DegradationLevel int

const (
	DegradationNone DegradationLevel = iota
	DegradationLight
	DegradationModerate
	DegradationSevere
	DegradationCritical
)

func (d DegradationLevel) String() string {
	switch d {
	case DegradationNone:
		return "NONE"
	case DegradationLight:
		return "LIGHT"
	case DegradationModerate:
		return "MODERATE"
	case DegradationSevere:
		return "SEVERE"
	case DegradationCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// MemoryAlertCallback is called when memory alerts are triggered
type MemoryAlertCallback func(stats *MemoryStats, previousStatus MemoryStatus)

// NewMemoryMonitor creates a new memory monitor
func NewMemoryMonitor(config *MemoryMonitorConfig) *MemoryMonitor {
	if config == nil {
		config = &MemoryMonitorConfig{
			CheckInterval:         10 * time.Second,
			WarningThreshold:      0.70, // 70%
			CriticalThreshold:     0.85, // 85%
			EmergencyThreshold:    0.95, // 95%
			MaxMemoryBytes:        0,    // Auto-detect
			EnableAlerts:          true,
			EnableGracefulDegrade: true,
			GCThreshold:           0.75, // 75%
			ForceGCThreshold:      0.90, // 90%
		}
	}

	// Auto-detect max memory if not specified
	if config.MaxMemoryBytes == 0 {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		// Use system memory as a reasonable default
		config.MaxMemoryBytes = m.Sys * 2 // Allow up to 2x current system memory
	}

	return &MemoryMonitor{
		config:           config,
		alertCallbacks:   make([]MemoryAlertCallback, 0),
		stopChan:         make(chan struct{}),
		degradationLevel: DegradationNone,
	}
}

// AddAlertCallback adds a callback function for memory alerts
func (m *MemoryMonitor) AddAlertCallback(callback MemoryAlertCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.alertCallbacks = append(m.alertCallbacks, callback)
}

// StartMonitoring starts the memory monitoring loop
func (m *MemoryMonitor) StartMonitoring() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isMonitoring {
		return fmt.Errorf("monitoring is already running")
	}

	m.isMonitoring = true
	m.stopChan = make(chan struct{})

	go m.monitoringLoop()

	return nil
}

// StopMonitoring stops the memory monitoring loop
func (m *MemoryMonitor) StopMonitoring() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isMonitoring {
		return
	}

	m.isMonitoring = false
	close(m.stopChan)
}

// GetMemoryStats returns current memory statistics
func (m *MemoryMonitor) GetMemoryStats() *MemoryStats {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	stats := &MemoryStats{
		AllocBytes:      memStats.Alloc,
		TotalAllocBytes: memStats.TotalAlloc,
		SysBytes:        memStats.Sys,
		HeapBytes:       memStats.HeapAlloc,
		StackBytes:      memStats.StackSys,
		GCCycles:        memStats.NumGC,
		Timestamp:       time.Now(),
	}

	// Calculate last GC time
	if memStats.NumGC > 0 {
		stats.LastGC = time.Unix(0, int64(memStats.LastGC))
	}

	// Calculate usage percentage
	stats.UsagePercent = float64(stats.AllocBytes) / float64(m.config.MaxMemoryBytes)

	// Determine status
	stats.Status = m.determineStatus(stats.UsagePercent)

	// Update current stats
	m.mu.Lock()
	previousStatus := MemoryStatusOK
	if m.currentStats != nil {
		previousStatus = m.currentStats.Status
	}
	m.currentStats = stats
	m.mu.Unlock()

	// Trigger alerts if status changed
	if m.config.EnableAlerts && stats.Status != previousStatus {
		m.triggerAlerts(stats, previousStatus)
	}

	return stats
}

// CheckMemoryPressure checks if the system is under memory pressure
func (m *MemoryMonitor) CheckMemoryPressure() bool {
	stats := m.GetMemoryStats()
	return stats.Status >= MemoryStatusWarning
}

// CanAllocate checks if a memory allocation of the given size can proceed
func (m *MemoryMonitor) CanAllocate(size uint64) error {
	if !m.config.EnableGracefulDegrade {
		return nil
	}

	stats := m.GetMemoryStats()

	// Check if allocation would exceed memory limits
	projectedUsage := float64(stats.AllocBytes+size) / float64(m.config.MaxMemoryBytes)

	if projectedUsage >= 1.0 {
		return &MemoryExhaustionError{
			RequestedBytes: size,
			AvailableBytes: m.config.MaxMemoryBytes - stats.AllocBytes,
			CurrentUsage:   stats.UsagePercent,
			Status:         stats.Status,
		}
	}

	// Check if we're in emergency status
	if stats.Status >= MemoryStatusEmergency {
		return &MemoryExhaustionError{
			RequestedBytes: size,
			AvailableBytes: m.config.MaxMemoryBytes - stats.AllocBytes,
			CurrentUsage:   stats.UsagePercent,
			Status:         stats.Status,
		}
	}

	return nil
}

// TriggerGC triggers garbage collection if needed
func (m *MemoryMonitor) TriggerGC(force bool) {
	stats := m.GetMemoryStats()

	shouldGC := force ||
		stats.UsagePercent >= m.config.GCThreshold ||
		stats.Status >= MemoryStatusWarning

	if shouldGC {
		if force || stats.UsagePercent >= m.config.ForceGCThreshold {
			// Force aggressive GC
			runtime.GC()
			runtime.GC() // Run twice for more aggressive cleanup
		} else {
			// Normal GC
			runtime.GC()
		}
	}
}

// GetDegradationLevel returns the current degradation level
func (m *MemoryMonitor) GetDegradationLevel() DegradationLevel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.degradationLevel
}

// ApplyGracefulDegradation applies graceful degradation based on memory pressure
func (m *MemoryMonitor) ApplyGracefulDegradation() error {
	stats := m.GetMemoryStats()

	if !m.config.EnableGracefulDegrade {
		return nil
	}

	// Determine appropriate degradation level
	newLevel := m.calculateDegradationLevel(stats)

	m.mu.Lock()
	oldLevel := m.degradationLevel
	m.degradationLevel = newLevel
	m.mu.Unlock()

	// Apply degradation if level changed
	if newLevel != oldLevel {
		return m.applyDegradationLevel(newLevel, oldLevel)
	}

	return nil
}

// MemoryExhaustionError represents a memory exhaustion error
type MemoryExhaustionError struct {
	RequestedBytes uint64       `json:"requested_bytes"`
	AvailableBytes uint64       `json:"available_bytes"`
	CurrentUsage   float64      `json:"current_usage"`
	Status         MemoryStatus `json:"status"`
}

func (e *MemoryExhaustionError) Error() string {
	return fmt.Sprintf("insufficient memory: requested %d bytes, available %d bytes, current usage %.2f%%, status: %s",
		e.RequestedBytes, e.AvailableBytes, e.CurrentUsage*100, e.Status)
}

// Private methods

func (m *MemoryMonitor) monitoringLoop() {
	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.GetMemoryStats()
			m.ApplyGracefulDegradation()

			// Trigger GC if needed
			stats := m.currentStats
			if stats != nil && stats.UsagePercent >= m.config.GCThreshold {
				m.TriggerGC(false)
			}

		case <-m.stopChan:
			return
		}
	}
}

func (m *MemoryMonitor) determineStatus(usagePercent float64) MemoryStatus {
	if usagePercent >= 1.0 {
		return MemoryStatusExhausted
	} else if usagePercent >= m.config.EmergencyThreshold {
		return MemoryStatusEmergency
	} else if usagePercent >= m.config.CriticalThreshold {
		return MemoryStatusCritical
	} else if usagePercent >= m.config.WarningThreshold {
		return MemoryStatusWarning
	}

	return MemoryStatusOK
}

func (m *MemoryMonitor) triggerAlerts(stats *MemoryStats, previousStatus MemoryStatus) {
	for _, callback := range m.alertCallbacks {
		go callback(stats, previousStatus)
	}
}

func (m *MemoryMonitor) calculateDegradationLevel(stats *MemoryStats) DegradationLevel {
	switch stats.Status {
	case MemoryStatusExhausted:
		return DegradationCritical
	case MemoryStatusEmergency:
		return DegradationSevere
	case MemoryStatusCritical:
		return DegradationModerate
	case MemoryStatusWarning:
		return DegradationLight
	default:
		return DegradationNone
	}
}

func (m *MemoryMonitor) applyDegradationLevel(newLevel, oldLevel DegradationLevel) error {
	// Log degradation level change
	fmt.Printf("[MEMORY] Degradation level changed from %s to %s\n", oldLevel, newLevel)

	switch newLevel {
	case DegradationNone:
		return m.restoreNormalOperation()
	case DegradationLight:
		return m.applyLightDegradation()
	case DegradationModerate:
		return m.applyModerateDegradation()
	case DegradationSevere:
		return m.applySevereDegradation()
	case DegradationCritical:
		return m.applyCriticalDegradation()
	default:
		return nil
	}
}

func (m *MemoryMonitor) restoreNormalOperation() error {
	// Restore normal operation
	// This could include:
	// - Re-enabling caches
	// - Restoring buffer sizes
	// - Re-enabling background tasks
	return nil
}

func (m *MemoryMonitor) applyLightDegradation() error {
	// Light degradation measures:
	// - Trigger garbage collection
	// - Reduce cache sizes slightly
	m.TriggerGC(false)
	return nil
}

func (m *MemoryMonitor) applyModerateDegradation() error {
	// Moderate degradation measures:
	// - Force garbage collection
	// - Reduce buffer sizes
	// - Clear non-essential caches
	m.TriggerGC(true)
	return nil
}

func (m *MemoryMonitor) applySevereDegradation() error {
	// Severe degradation measures:
	// - Aggressive garbage collection
	// - Disable non-essential features
	// - Reduce connection pools
	m.TriggerGC(true)
	runtime.GC() // Additional GC cycle
	return nil
}

func (m *MemoryMonitor) applyCriticalDegradation() error {
	// Critical degradation measures:
	// - Emergency garbage collection
	// - Reject new operations
	// - Enable emergency mode
	m.TriggerGC(true)
	runtime.GC()
	runtime.GC() // Multiple GC cycles

	// In a real implementation, this might:
	// - Switch to read-only mode
	// - Reject new connections
	// - Flush all caches

	return nil
}
