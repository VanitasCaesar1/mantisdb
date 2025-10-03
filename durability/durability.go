package durability

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DurabilityManager is the main interface for durability operations
type DurabilityManager struct {
	config *DurabilityConfig
	mutex  sync.RWMutex

	// Components
	policyEnforcer *PolicyEnforcer
	syncWriter     *SyncWriter
	asyncWriter    *AsyncWriter
	flushManager   *FlushManager
	syncOptimizer  *SyncOptimizer

	// State
	initialized bool
}

// NewDurabilityManager creates a new durability manager
func NewDurabilityManager(config *DurabilityConfig) (*DurabilityManager, error) {
	if config == nil {
		config = DefaultDurabilityConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid durability config: %w", err)
	}

	dm := &DurabilityManager{
		config: config,
	}

	// Initialize components
	if err := dm.initializeComponents(); err != nil {
		return nil, fmt.Errorf("failed to initialize durability components: %w", err)
	}

	return dm, nil
}

// Write performs a write operation according to the configured durability level
func (dm *DurabilityManager) Write(ctx context.Context, filePath string, data []byte, offset int64) error {
	if !dm.initialized {
		return fmt.Errorf("durability manager not initialized")
	}

	// Enforce write policy
	if err := dm.policyEnforcer.EnforceWritePolicy(ctx); err != nil {
		return fmt.Errorf("policy enforcement failed: %w", err)
	}

	// Route to appropriate writer based on configuration
	switch dm.config.WriteMode {
	case WriteModeSync:
		if dm.config.Level == DurabilityStrict {
			// Use direct sync writer for strict durability
			return dm.syncWriter.WriteSync(ctx, filePath, data, offset)
		} else {
			// Use optimized sync writer
			return dm.syncOptimizer.OptimizeWrite(ctx, filePath, data, offset)
		}

	case WriteModeAsync:
		return dm.asyncWriter.WriteAsync(ctx, filePath, data, offset)

	case WriteModeBatch:
		// For batch mode, we still use async writer but with different flush behavior
		return dm.asyncWriter.WriteAsync(ctx, filePath, data, offset)

	default:
		return fmt.Errorf("unknown write mode: %v", dm.config.WriteMode)
	}
}

// BatchWrite performs a batch of write operations
func (dm *DurabilityManager) BatchWrite(ctx context.Context, writes []WriteOperation) error {
	if !dm.initialized {
		return fmt.Errorf("durability manager not initialized")
	}

	// Convert to appropriate format based on write mode
	switch dm.config.WriteMode {
	case WriteModeSync:
		syncWrites := make([]SyncWrite, len(writes))
		for i, w := range writes {
			syncWrites[i] = SyncWrite{
				FilePath: w.FilePath,
				Data:     w.Data,
				Offset:   w.Offset,
			}
		}
		return dm.syncWriter.BatchWriteSync(ctx, syncWrites)

	case WriteModeAsync, WriteModeBatch:
		asyncWrites := make([]AsyncWrite, len(writes))
		for i, w := range writes {
			asyncWrites[i] = AsyncWrite{
				FilePath: w.FilePath,
				Data:     w.Data,
				Offset:   w.Offset,
			}
		}
		return dm.asyncWriter.BatchWriteAsync(ctx, asyncWrites)

	default:
		return fmt.Errorf("unknown write mode: %v", dm.config.WriteMode)
	}
}

// Flush forces a flush of all pending writes
func (dm *DurabilityManager) Flush(ctx context.Context) error {
	if !dm.initialized {
		return fmt.Errorf("durability manager not initialized")
	}

	return dm.flushManager.ForceFlush(ctx)
}

// FlushFile forces a flush of pending writes for a specific file
func (dm *DurabilityManager) FlushFile(ctx context.Context, filePath string) error {
	if !dm.initialized {
		return fmt.Errorf("durability manager not initialized")
	}

	return dm.flushManager.RequestFlush([]string{filePath}, FlushPriorityHigh)
}

// Sync forces a sync operation for all files
func (dm *DurabilityManager) Sync(ctx context.Context) error {
	if !dm.initialized {
		return fmt.Errorf("durability manager not initialized")
	}

	// First flush any pending async writes
	if err := dm.Flush(ctx); err != nil {
		return fmt.Errorf("flush failed during sync: %w", err)
	}

	// Then force sync
	return dm.syncWriter.FsyncAll(ctx)
}

// SyncFile forces a sync operation for a specific file
func (dm *DurabilityManager) SyncFile(ctx context.Context, filePath string) error {
	if !dm.initialized {
		return fmt.Errorf("durability manager not initialized")
	}

	// First flush any pending async writes for this file
	if err := dm.FlushFile(ctx, filePath); err != nil {
		return fmt.Errorf("flush failed during sync: %w", err)
	}

	// Then force sync
	return dm.syncWriter.FsyncFile(ctx, filePath)
}

// UpdateConfig updates the durability configuration
func (dm *DurabilityManager) UpdateConfig(config *DurabilityConfig) error {
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid durability config: %w", err)
	}

	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	dm.config = config

	// Update component configurations
	if err := dm.policyEnforcer.UpdateConfig(config); err != nil {
		return fmt.Errorf("failed to update policy enforcer config: %w", err)
	}

	return nil
}

// GetConfig returns the current durability configuration
func (dm *DurabilityManager) GetConfig() *DurabilityConfig {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	// Return a copy
	configCopy := *dm.config
	return &configCopy
}

// GetMetrics returns comprehensive durability metrics
func (dm *DurabilityManager) GetMetrics() DurabilityMetrics {
	if !dm.initialized {
		return DurabilityMetrics{}
	}

	// Get sync writer metrics - either from direct sync writer or from sync optimizer's sync writer
	var syncWriterMetrics SyncWriterMetrics
	if dm.config.Level == DurabilityStrict {
		syncWriterMetrics = dm.syncWriter.GetMetrics()
	} else {
		// When using sync optimizer, get metrics from its internal sync writer
		syncWriterMetrics = dm.syncOptimizer.GetSyncWriterMetrics()
	}

	return DurabilityMetrics{
		Policy:        dm.policyEnforcer.GetMetrics(),
		SyncWriter:    syncWriterMetrics,
		AsyncWriter:   dm.asyncWriter.GetMetrics(),
		FlushManager:  dm.flushManager.GetMetrics(),
		SyncOptimizer: dm.syncOptimizer.GetMetrics(),
	}
}

// GetStatus returns the current durability status
func (dm *DurabilityManager) GetStatus() DurabilityStatus {
	if !dm.initialized {
		return DurabilityStatus{
			Initialized: false,
		}
	}

	flushStatus := dm.flushManager.GetFlushStatus()

	return DurabilityStatus{
		Initialized:     true,
		Config:          *dm.config,
		UnflushedWrites: flushStatus.UnflushedWrites,
		LastFlush:       flushStatus.LastFlush,
		FlushInProgress: flushStatus.FlushInProgress,
	}
}

// Close shuts down the durability manager and flushes all pending operations
func (dm *DurabilityManager) Close(ctx context.Context) error {
	if !dm.initialized {
		return nil
	}

	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	var lastErr error

	// Close components in reverse order of initialization
	if dm.flushManager != nil {
		if err := dm.flushManager.Close(ctx); err != nil {
			lastErr = fmt.Errorf("failed to close flush manager: %w", err)
		}
	}

	if dm.syncOptimizer != nil {
		if err := dm.syncOptimizer.Close(ctx); err != nil {
			lastErr = fmt.Errorf("failed to close sync optimizer: %w", err)
		}
	}

	if dm.asyncWriter != nil {
		if err := dm.asyncWriter.Close(ctx); err != nil {
			lastErr = fmt.Errorf("failed to close async writer: %w", err)
		}
	}

	if dm.syncWriter != nil {
		if err := dm.syncWriter.Close(); err != nil {
			lastErr = fmt.Errorf("failed to close sync writer: %w", err)
		}
	}

	dm.initialized = false
	return lastErr
}

// initializeComponents initializes all durability components
func (dm *DurabilityManager) initializeComponents() error {
	// Initialize policy enforcer
	dm.policyEnforcer = NewPolicyEnforcer(dm.config)

	// Initialize writers
	dm.syncWriter = NewSyncWriter(dm.config)
	dm.asyncWriter = NewAsyncWriter(dm.config)
	dm.syncOptimizer = NewSyncOptimizer(dm.config)

	// Initialize flush manager
	dm.flushManager = NewFlushManager(dm.config)

	// Register writers with flush manager
	dm.flushManager.RegisterAsyncWriter(dm.asyncWriter)
	dm.flushManager.RegisterSyncWriter(dm.syncWriter)

	// Set up policy enforcer callbacks
	dm.policyEnforcer.SetCallbacks(
		func() error { return dm.syncWriter.FsyncAll(context.Background()) },
		func() error { return dm.asyncWriter.FlushAll(context.Background()) },
		func() error { return nil }, // Barrier callback - not file-specific
		func() error { return nil }, // Verification callback
	)

	dm.initialized = true
	return nil
}

// WriteOperation represents a single write operation
type WriteOperation struct {
	FilePath string
	Data     []byte
	Offset   int64
}

// DurabilityMetrics holds comprehensive durability metrics
type DurabilityMetrics struct {
	Policy        PolicyMetrics        `json:"policy"`
	SyncWriter    SyncWriterMetrics    `json:"sync_writer"`
	AsyncWriter   AsyncWriterMetrics   `json:"async_writer"`
	FlushManager  FlushManagerMetrics  `json:"flush_manager"`
	SyncOptimizer SyncOptimizerMetrics `json:"sync_optimizer"`
}

// DurabilityStatus represents the current status of the durability system
type DurabilityStatus struct {
	Initialized     bool             `json:"initialized"`
	Config          DurabilityConfig `json:"config"`
	UnflushedWrites int64            `json:"unflushed_writes"`
	LastFlush       time.Time        `json:"last_flush"`
	FlushInProgress bool             `json:"flush_in_progress"`
}
