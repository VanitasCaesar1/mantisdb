package concurrency

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// EnhancedConcurrencySystem integrates all concurrency components
type EnhancedConcurrencySystem struct {
	// Core components
	lockManager      *EnhancedLockManager
	deadlockDetector *EnhancedDeadlockDetector
	profiler         *LockProfiler
	metricsExporter  *MetricsExporter
	goroutineManager *GoroutineManager

	// Configuration
	config *ConcurrencySystemConfig

	// Lifecycle
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool
	mutex   sync.RWMutex
}

// ConcurrencySystemConfig holds configuration for the entire concurrency system
type ConcurrencySystemConfig struct {
	LockManagerConfig      *LockManagerConfig
	DeadlockDetectorConfig *DeadlockDetectorConfig
	ProfilerConfig         *LockProfilerConfig
	MetricsConfig          *MetricsExporterConfig
	GoroutineManagerConfig *GoroutineManagerConfig

	// Integration settings
	EnableProfiling           bool
	EnableMetrics             bool
	EnableGoroutineManagement bool
}

// DefaultConcurrencySystemConfig returns default configuration
func DefaultConcurrencySystemConfig() *ConcurrencySystemConfig {
	return &ConcurrencySystemConfig{
		LockManagerConfig:         DefaultLockManagerConfig(),
		DeadlockDetectorConfig:    DefaultDeadlockDetectorConfig(),
		ProfilerConfig:            DefaultLockProfilerConfig(),
		MetricsConfig:             DefaultMetricsExporterConfig(),
		GoroutineManagerConfig:    DefaultGoroutineManagerConfig(),
		EnableProfiling:           true,
		EnableMetrics:             true,
		EnableGoroutineManagement: true,
	}
}

// NewEnhancedConcurrencySystem creates a new enhanced concurrency system
func NewEnhancedConcurrencySystem(config *ConcurrencySystemConfig) *EnhancedConcurrencySystem {
	if config == nil {
		config = DefaultConcurrencySystemConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Create core components
	lockManager := NewEnhancedLockManager(config.LockManagerConfig)
	deadlockDetector := NewEnhancedDeadlockDetector(lockManager, config.DeadlockDetectorConfig)

	var profiler *LockProfiler
	var metricsExporter *MetricsExporter
	var goroutineManager *GoroutineManager

	if config.EnableProfiling {
		profiler = NewLockProfiler(config.ProfilerConfig)
	}

	if config.EnableMetrics {
		metricsExporter = NewMetricsExporter(config.MetricsConfig)
		if metricsExporter != nil {
			metricsExporter.SetDataSources(lockManager, deadlockDetector, profiler)
		}
	}

	if config.EnableGoroutineManagement {
		goroutineManager = NewGoroutineManager(config.GoroutineManagerConfig)
	}

	return &EnhancedConcurrencySystem{
		lockManager:      lockManager,
		deadlockDetector: deadlockDetector,
		profiler:         profiler,
		metricsExporter:  metricsExporter,
		goroutineManager: goroutineManager,
		config:           config,
		ctx:              ctx,
		cancel:           cancel,
	}
}

// Start starts all components of the concurrency system
func (ecs *EnhancedConcurrencySystem) Start() error {
	ecs.mutex.Lock()
	defer ecs.mutex.Unlock()

	if ecs.running {
		return fmt.Errorf("concurrency system is already running")
	}

	// Start goroutine manager first
	if ecs.goroutineManager != nil {
		if err := ecs.goroutineManager.Start(); err != nil {
			return fmt.Errorf("failed to start goroutine manager: %w", err)
		}
	}

	// Start profiler
	if ecs.profiler != nil {
		if err := ecs.profiler.Start(ecs.ctx); err != nil {
			return fmt.Errorf("failed to start profiler: %w", err)
		}
	}

	// Start deadlock detector
	if err := ecs.deadlockDetector.Start(ecs.ctx); err != nil {
		return fmt.Errorf("failed to start deadlock detector: %w", err)
	}

	// Start metrics exporter
	if ecs.metricsExporter != nil {
		if err := ecs.metricsExporter.Start(ecs.ctx, ecs.config.MetricsConfig); err != nil {
			return fmt.Errorf("failed to start metrics exporter: %w", err)
		}
	}

	ecs.running = true
	return nil
}

// Stop stops all components of the concurrency system
func (ecs *EnhancedConcurrencySystem) Stop() error {
	ecs.mutex.Lock()
	defer ecs.mutex.Unlock()

	if !ecs.running {
		return nil
	}

	// Cancel context to signal shutdown
	ecs.cancel()

	// Stop components in reverse order
	var lastError error

	if ecs.metricsExporter != nil {
		if err := ecs.metricsExporter.Stop(); err != nil {
			lastError = err
		}
	}

	if err := ecs.deadlockDetector.Stop(); err != nil {
		lastError = err
	}

	if ecs.profiler != nil {
		if err := ecs.profiler.Stop(); err != nil {
			lastError = err
		}
	}

	if ecs.goroutineManager != nil {
		if err := ecs.goroutineManager.Stop(); err != nil {
			lastError = err
		}
	}

	// Close lock manager last
	if err := ecs.lockManager.Close(); err != nil {
		lastError = err
	}

	// Wait for all goroutines to finish
	ecs.wg.Wait()

	ecs.running = false
	return lastError
}

// GetLockManager returns the lock manager
func (ecs *EnhancedConcurrencySystem) GetLockManager() *EnhancedLockManager {
	return ecs.lockManager
}

// GetDeadlockDetector returns the deadlock detector
func (ecs *EnhancedConcurrencySystem) GetDeadlockDetector() *EnhancedDeadlockDetector {
	return ecs.deadlockDetector
}

// GetProfiler returns the lock profiler
func (ecs *EnhancedConcurrencySystem) GetProfiler() *LockProfiler {
	return ecs.profiler
}

// GetMetricsExporter returns the metrics exporter
func (ecs *EnhancedConcurrencySystem) GetMetricsExporter() *MetricsExporter {
	return ecs.metricsExporter
}

// GetGoroutineManager returns the goroutine manager
func (ecs *EnhancedConcurrencySystem) GetGoroutineManager() *GoroutineManager {
	return ecs.goroutineManager
}

// AcquireLock acquires a lock with full monitoring and profiling
func (ecs *EnhancedConcurrencySystem) AcquireLock(txnID uint64, resource string, lockType LockType) error {
	startTime := time.Now()

	// Record lock request
	if ecs.profiler != nil {
		ecs.profiler.RecordLockEvent(resource, txnID, EventLockRequested, lockType, 0, 0, 0)
	}
	if ecs.metricsExporter != nil {
		ecs.metricsExporter.RecordLockRequest(resource, lockType)
	}

	// Attempt to acquire lock
	err := ecs.lockManager.AcquireLock(txnID, resource, lockType)

	waitTime := time.Since(startTime)

	if err != nil {
		// Record timeout or failure
		if ecs.profiler != nil {
			ecs.profiler.RecordLockEvent(resource, txnID, EventLockTimeout, lockType, 0, waitTime, 0)
		}
		if ecs.metricsExporter != nil {
			ecs.metricsExporter.RecordLockTimeout(resource, lockType)
		}
		return err
	}

	// Record successful acquisition
	if ecs.profiler != nil {
		ecs.profiler.RecordLockEvent(resource, txnID, EventLockAcquired, lockType, 0, waitTime, 0)
	}
	if ecs.metricsExporter != nil {
		ecs.metricsExporter.RecordLockAcquisition(resource, lockType, waitTime)
	}

	return nil
}

// ReleaseLock releases a lock with full monitoring and profiling
func (ecs *EnhancedConcurrencySystem) ReleaseLock(txnID uint64, resource string) error {
	startTime := time.Now()

	// Release the lock
	err := ecs.lockManager.ReleaseLock(txnID, resource)

	if err == nil {
		holdTime := time.Since(startTime) // This is simplified; in reality we'd track from acquisition

		// Record release
		if ecs.profiler != nil {
			ecs.profiler.RecordLockEvent(resource, txnID, EventLockReleased, ReadLock, holdTime, 0, 0)
		}
		if ecs.metricsExporter != nil {
			ecs.metricsExporter.RecordLockRelease(resource, ReadLock, holdTime)
		}
	}

	return err
}

// ReleaseAllLocks releases all locks held by a transaction
func (ecs *EnhancedConcurrencySystem) ReleaseAllLocks(txnID uint64) error {
	return ecs.lockManager.ReleaseAllLocks(txnID)
}

// GetSystemStats returns comprehensive system statistics
func (ecs *EnhancedConcurrencySystem) GetSystemStats() *SystemStats {
	stats := &SystemStats{
		Timestamp: time.Now(),
		Running:   ecs.running,
	}

	// Get lock manager metrics
	if ecs.lockManager != nil {
		stats.LockMetrics = ecs.lockManager.GetMetrics()
	}

	// Get deadlock detector metrics
	if ecs.deadlockDetector != nil {
		stats.DeadlockMetrics = ecs.deadlockDetector.GetMetrics()
	}

	// Get profiler stats
	if ecs.profiler != nil {
		stats.ProfilerStats = ecs.profiler.GetGlobalStats()
	}

	// Get goroutine manager stats
	if ecs.goroutineManager != nil {
		stats.GoroutineStats = ecs.goroutineManager.GetStats()
	}

	// Get metrics snapshot
	if ecs.metricsExporter != nil {
		stats.MetricsSnapshot = ecs.metricsExporter.GetSnapshot()
	}

	return stats
}

// SystemStats contains comprehensive system statistics
type SystemStats struct {
	Timestamp       time.Time
	Running         bool
	LockMetrics     *LockMetrics
	DeadlockMetrics *DeadlockMetrics
	ProfilerStats   *GlobalLockStats
	GoroutineStats  *GoroutineStats
	MetricsSnapshot *MetricsSnapshot
}

// IsHealthy returns whether the system is healthy
func (ecs *EnhancedConcurrencySystem) IsHealthy() bool {
	ecs.mutex.RLock()
	defer ecs.mutex.RUnlock()

	if !ecs.running {
		return false
	}

	// Check if any component has excessive errors or issues
	stats := ecs.GetSystemStats()

	// Check for excessive deadlocks
	if stats.DeadlockMetrics != nil && stats.DeadlockMetrics.deadlocksFound > 100 {
		return false
	}

	// Check for excessive timeouts
	if stats.LockMetrics != nil && stats.LockMetrics.lockTimeouts > 1000 {
		return false
	}

	// Check for goroutine leaks
	if stats.GoroutineStats != nil && stats.GoroutineStats.LeaksDetected > 50 {
		return false
	}

	return true
}

// SpawnManagedGoroutine spawns a goroutine using the goroutine manager
func (ecs *EnhancedConcurrencySystem) SpawnManagedGoroutine(name string, fn func(context.Context)) (*GoroutineInfo, error) {
	if ecs.goroutineManager == nil {
		return nil, fmt.Errorf("goroutine manager is not enabled")
	}

	return ecs.goroutineManager.SpawnGoroutine(name, fn)
}

// SubmitWork submits work to the managed worker pool
func (ecs *EnhancedConcurrencySystem) SubmitWork(item WorkItem) error {
	if ecs.goroutineManager == nil {
		return fmt.Errorf("goroutine manager is not enabled")
	}

	return ecs.goroutineManager.SubmitWork(item)
}
