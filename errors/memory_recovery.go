package errors

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"time"
)

// MemoryRecoveryManager handles memory recovery procedures
type MemoryRecoveryManager struct {
	monitor       *MemoryMonitor
	config        *MemoryRecoveryConfig
	recoveryTasks []MemoryRecoveryTask
}

// MemoryRecoveryConfig contains configuration for memory recovery
type MemoryRecoveryConfig struct {
	EnableAutoRecovery    bool          `json:"enable_auto_recovery"`
	RecoveryThreshold     float64       `json:"recovery_threshold"` // Start recovery at this usage %
	TargetUsage           float64       `json:"target_usage"`       // Target usage % after recovery
	MaxRecoveryAttempts   int           `json:"max_recovery_attempts"`
	RecoveryRetryDelay    time.Duration `json:"recovery_retry_delay"`
	EmergencyRecoveryMode bool          `json:"emergency_recovery_mode"`
	GCPressureThreshold   float64       `json:"gc_pressure_threshold"` // Trigger aggressive GC at this %
}

// MemoryRecoveryTask represents a memory recovery task
type MemoryRecoveryTask struct {
	Name        string       `json:"name"`
	Priority    int          `json:"priority"`     // Lower number = higher priority
	EstimatedMB int64        `json:"estimated_mb"` // Estimated memory to free in MB
	Execute     func() error `json:"-"`
}

// MemoryRecoveryResult contains the result of a memory recovery operation
type MemoryRecoveryResult struct {
	StartTime           time.Time                  `json:"start_time"`
	EndTime             time.Time                  `json:"end_time"`
	Duration            time.Duration              `json:"duration"`
	Success             bool                       `json:"success"`
	InitialAllocBytes   uint64                     `json:"initial_alloc_bytes"`
	FinalAllocBytes     uint64                     `json:"final_alloc_bytes"`
	BytesFreed          int64                      `json:"bytes_freed"`
	InitialUsagePercent float64                    `json:"initial_usage_percent"`
	FinalUsagePercent   float64                    `json:"final_usage_percent"`
	RecoveryTaskResults []MemoryRecoveryTaskResult `json:"recovery_task_results"`
	GCStats             GCRecoveryStats            `json:"gc_stats"`
	Message             string                     `json:"message"`
	Error               error                      `json:"error,omitempty"`
}

// MemoryRecoveryTaskResult contains the result of a single recovery task
type MemoryRecoveryTaskResult struct {
	TaskName   string        `json:"task_name"`
	Success    bool          `json:"success"`
	BytesFreed int64         `json:"bytes_freed"`
	Duration   time.Duration `json:"duration"`
	Error      error         `json:"error,omitempty"`
}

// GCRecoveryStats contains garbage collection statistics during recovery
type GCRecoveryStats struct {
	InitialGCCycles uint32        `json:"initial_gc_cycles"`
	FinalGCCycles   uint32        `json:"final_gc_cycles"`
	GCCyclesRun     uint32        `json:"gc_cycles_run"`
	TotalGCTime     time.Duration `json:"total_gc_time"`
	ForcedGCCount   int           `json:"forced_gc_count"`
}

// NewMemoryRecoveryManager creates a new memory recovery manager
func NewMemoryRecoveryManager(monitor *MemoryMonitor, config *MemoryRecoveryConfig) *MemoryRecoveryManager {
	if config == nil {
		config = &MemoryRecoveryConfig{
			EnableAutoRecovery:    true,
			RecoveryThreshold:     0.80, // Start recovery at 80% usage
			TargetUsage:           0.60, // Target 60% usage after recovery
			MaxRecoveryAttempts:   3,
			RecoveryRetryDelay:    2 * time.Second,
			EmergencyRecoveryMode: false,
			GCPressureThreshold:   0.75, // 75%
		}
	}

	manager := &MemoryRecoveryManager{
		monitor:       monitor,
		config:        config,
		recoveryTasks: make([]MemoryRecoveryTask, 0),
	}

	// Register default recovery tasks
	manager.registerDefaultRecoveryTasks()

	return manager
}

// RegisterRecoveryTask registers a new memory recovery task
func (m *MemoryRecoveryManager) RegisterRecoveryTask(task MemoryRecoveryTask) {
	m.recoveryTasks = append(m.recoveryTasks, task)

	// Sort tasks by priority (lower number = higher priority)
	for i := len(m.recoveryTasks) - 1; i > 0; i-- {
		if m.recoveryTasks[i].Priority < m.recoveryTasks[i-1].Priority {
			m.recoveryTasks[i], m.recoveryTasks[i-1] = m.recoveryTasks[i-1], m.recoveryTasks[i]
		} else {
			break
		}
	}
}

// AttemptRecovery attempts to recover memory
func (m *MemoryRecoveryManager) AttemptRecovery() (*MemoryRecoveryResult, error) {
	result := &MemoryRecoveryResult{
		StartTime: time.Now(),
		Success:   false,
	}

	// Get initial memory stats
	initialStats := m.monitor.GetMemoryStats()
	result.InitialAllocBytes = initialStats.AllocBytes
	result.InitialUsagePercent = initialStats.UsagePercent
	result.GCStats.InitialGCCycles = initialStats.GCCycles

	// Check if recovery is needed
	if initialStats.UsagePercent < m.config.RecoveryThreshold {
		result.Success = true
		result.Message = "No memory recovery needed"
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, nil
	}

	// Perform recovery attempts
	for attempt := 1; attempt <= m.config.MaxRecoveryAttempts; attempt++ {
		recoveryResults := m.performRecovery(attempt)
		result.RecoveryTaskResults = append(result.RecoveryTaskResults, recoveryResults...)

		// Check if we've achieved target usage
		currentStats := m.monitor.GetMemoryStats()
		result.FinalAllocBytes = currentStats.AllocBytes
		result.FinalUsagePercent = currentStats.UsagePercent
		result.GCStats.FinalGCCycles = currentStats.GCCycles
		result.GCStats.GCCyclesRun = result.GCStats.FinalGCCycles - result.GCStats.InitialGCCycles
		result.BytesFreed = int64(result.InitialAllocBytes) - int64(result.FinalAllocBytes)

		// Check if we've reached the target
		if currentStats.UsagePercent <= m.config.TargetUsage {
			result.Success = true
			result.Message = fmt.Sprintf("Successfully freed %d bytes in %d attempts", result.BytesFreed, attempt)
			break
		}

		// Wait before next attempt
		if attempt < m.config.MaxRecoveryAttempts {
			time.Sleep(m.config.RecoveryRetryDelay)
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	if !result.Success {
		result.Error = fmt.Errorf("failed to achieve target memory usage after %d attempts", m.config.MaxRecoveryAttempts)
	}

	return result, result.Error
}

// PerformEmergencyRecovery performs aggressive memory recovery in emergency situations
func (m *MemoryRecoveryManager) PerformEmergencyRecovery() (*MemoryRecoveryResult, error) {
	// Temporarily enable emergency recovery mode
	originalMode := m.config.EmergencyRecoveryMode
	m.config.EmergencyRecoveryMode = true
	defer func() {
		m.config.EmergencyRecoveryMode = originalMode
	}()

	// Lower the target usage for emergency recovery
	originalTarget := m.config.TargetUsage
	m.config.TargetUsage = 0.40 // Target 40% usage in emergency
	defer func() {
		m.config.TargetUsage = originalTarget
	}()

	return m.AttemptRecovery()
}

// ForceGarbageCollection performs aggressive garbage collection
func (m *MemoryRecoveryManager) ForceGarbageCollection() GCRecoveryStats {
	initialStats := m.monitor.GetMemoryStats()
	startTime := time.Now()

	stats := GCRecoveryStats{
		InitialGCCycles: initialStats.GCCycles,
	}

	// Perform multiple GC cycles for aggressive cleanup
	gcCount := 1
	if m.config.EmergencyRecoveryMode {
		gcCount = 3 // More aggressive in emergency mode
	}

	for i := 0; i < gcCount; i++ {
		runtime.GC()
		stats.ForcedGCCount++
	}

	// Force finalization
	runtime.GC()
	runtime.GC()
	stats.ForcedGCCount += 2

	finalStats := m.monitor.GetMemoryStats()
	stats.FinalGCCycles = finalStats.GCCycles
	stats.GCCyclesRun = stats.FinalGCCycles - stats.InitialGCCycles
	stats.TotalGCTime = time.Since(startTime)

	return stats
}

// SetGCPercent adjusts the garbage collection target percentage
func (m *MemoryRecoveryManager) SetGCPercent(percent int) int {
	return debug.SetGCPercent(percent)
}

// GetRecoveryTasks returns the list of registered recovery tasks
func (m *MemoryRecoveryManager) GetRecoveryTasks() []MemoryRecoveryTask {
	return append([]MemoryRecoveryTask(nil), m.recoveryTasks...) // Return a copy
}

// Private methods

func (m *MemoryRecoveryManager) performRecovery(attempt int) []MemoryRecoveryTaskResult {
	var results []MemoryRecoveryTaskResult

	for _, task := range m.recoveryTasks {
		startTime := time.Now()

		result := MemoryRecoveryTaskResult{
			TaskName: task.Name,
			Success:  false,
		}

		// Get memory stats before recovery task
		beforeStats := m.monitor.GetMemoryStats()

		// Execute recovery task
		err := task.Execute()
		if err != nil {
			result.Error = fmt.Errorf("recovery task failed: %w", err)
			result.Duration = time.Since(startTime)
			results = append(results, result)
			continue
		}

		// Get memory stats after recovery task
		afterStats := m.monitor.GetMemoryStats()

		result.Success = true
		result.BytesFreed = int64(beforeStats.AllocBytes) - int64(afterStats.AllocBytes)
		result.Duration = time.Since(startTime)

		results = append(results, result)

		// If we've achieved enough memory recovery, stop
		if afterStats.UsagePercent <= m.config.TargetUsage {
			break
		}
	}

	return results
}

func (m *MemoryRecoveryManager) registerDefaultRecoveryTasks() {
	// Task 1: Force garbage collection
	m.RegisterRecoveryTask(MemoryRecoveryTask{
		Name:        "force_garbage_collection",
		Priority:    1,
		EstimatedMB: 50,
		Execute:     m.taskForceGC,
	})

	// Task 2: Clear runtime caches
	m.RegisterRecoveryTask(MemoryRecoveryTask{
		Name:        "clear_runtime_caches",
		Priority:    2,
		EstimatedMB: 20,
		Execute:     m.taskClearRuntimeCaches,
	})

	// Task 3: Reduce GC target percentage
	m.RegisterRecoveryTask(MemoryRecoveryTask{
		Name:        "reduce_gc_target",
		Priority:    3,
		EstimatedMB: 30,
		Execute:     m.taskReduceGCTarget,
	})

	// Task 4: Force finalization
	m.RegisterRecoveryTask(MemoryRecoveryTask{
		Name:        "force_finalization",
		Priority:    4,
		EstimatedMB: 10,
		Execute:     m.taskForceFinalization,
	})

	// Emergency task: Set aggressive GC (only in emergency mode)
	m.RegisterRecoveryTask(MemoryRecoveryTask{
		Name:        "emergency_gc_mode",
		Priority:    10,
		EstimatedMB: 100,
		Execute:     m.taskEmergencyGCMode,
	})
}

// Default recovery task implementations

func (m *MemoryRecoveryManager) taskForceGC() error {
	m.ForceGarbageCollection()
	return nil
}

func (m *MemoryRecoveryManager) taskClearRuntimeCaches() error {
	// Clear various runtime caches
	runtime.GC()

	// In a real implementation, this might clear:
	// - Connection pools
	// - Query caches
	// - Buffer pools
	// - Other application-specific caches

	return nil
}

func (m *MemoryRecoveryManager) taskReduceGCTarget() error {
	// Reduce GC target percentage to trigger more frequent GC
	currentPercent := debug.SetGCPercent(50) // More aggressive GC

	// Store original value for potential restoration
	_ = currentPercent

	return nil
}

func (m *MemoryRecoveryManager) taskForceFinalization() error {
	// Force finalization of objects waiting for finalization
	runtime.GC()
	runtime.GC() // Run twice to ensure finalization

	return nil
}

func (m *MemoryRecoveryManager) taskEmergencyGCMode() error {
	// Only run in emergency mode
	if !m.config.EmergencyRecoveryMode {
		return nil
	}

	// Set very aggressive GC
	debug.SetGCPercent(10) // Very frequent GC

	// Perform multiple GC cycles
	for i := 0; i < 5; i++ {
		runtime.GC()
	}

	return nil
}
