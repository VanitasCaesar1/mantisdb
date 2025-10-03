package durability

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// PolicyEnforcer enforces durability policies for write operations
type PolicyEnforcer struct {
	config *DurabilityConfig
	mutex  sync.RWMutex

	// Metrics for monitoring
	writeCount int64
	syncCount  int64
	flushCount int64
	errorCount int64
	retryCount int64

	// State for async operations
	lastFlush time.Time

	// Callbacks for actual I/O operations
	syncCallback    func() error
	flushCallback   func() error
	barrierCallback func() error
	verifyCallback  func() error
}

// NewPolicyEnforcer creates a new policy enforcer
func NewPolicyEnforcer(config *DurabilityConfig) *PolicyEnforcer {
	if config == nil {
		config = DefaultDurabilityConfig()
	}

	return &PolicyEnforcer{
		config:    config,
		lastFlush: time.Now(),
	}
}

// SetCallbacks sets the I/O operation callbacks
func (p *PolicyEnforcer) SetCallbacks(
	syncCallback func() error,
	flushCallback func() error,
	barrierCallback func() error,
	verifyCallback func() error,
) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.syncCallback = syncCallback
	p.flushCallback = flushCallback
	p.barrierCallback = barrierCallback
	p.verifyCallback = verifyCallback
}

// UpdateConfig updates the durability configuration
func (p *PolicyEnforcer) UpdateConfig(config *DurabilityConfig) error {
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid durability config: %w", err)
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.config = config
	return nil
}

// GetConfig returns the current durability configuration
func (p *PolicyEnforcer) GetConfig() *DurabilityConfig {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	// Return a copy to prevent external modification
	configCopy := *p.config
	return &configCopy
}

// EnforceWritePolicy enforces the durability policy for a write operation
func (p *PolicyEnforcer) EnforceWritePolicy(ctx context.Context) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.writeCount++

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Apply write barriers if required
	if p.config.RequiresBarriers() {
		if err := p.executeWithRetry(ctx, p.barrierCallback, "barrier"); err != nil {
			return fmt.Errorf("write barrier failed: %w", err)
		}
	}

	// Handle different write modes
	switch p.config.WriteMode {
	case WriteModeSync:
		return p.enforceSyncWrite(ctx)
	case WriteModeAsync:
		return p.enforceAsyncWrite(ctx)
	case WriteModeBatch:
		return p.enforceBatchWrite(ctx)
	default:
		return fmt.Errorf("unknown write mode: %v", p.config.WriteMode)
	}
}

// enforceSyncWrite handles synchronous write policy
func (p *PolicyEnforcer) enforceSyncWrite(ctx context.Context) error {
	// Perform sync operation
	if err := p.executeWithRetry(ctx, p.syncCallback, "sync"); err != nil {
		return fmt.Errorf("sync write failed: %w", err)
	}

	p.syncCount++

	// Verify write if required
	if p.config.RequiresVerification() {
		if err := p.executeWithRetry(ctx, p.verifyCallback, "verify"); err != nil {
			return fmt.Errorf("write verification failed: %w", err)
		}
	}

	return nil
}

// enforceAsyncWrite handles asynchronous write policy
func (p *PolicyEnforcer) enforceAsyncWrite(ctx context.Context) error {
	// Check if we need to flush based on time interval
	if p.config.RequiresFlush() {
		now := time.Now()
		if now.Sub(p.lastFlush) >= p.config.FlushInterval {
			if err := p.executeWithRetry(ctx, p.flushCallback, "flush"); err != nil {
				return fmt.Errorf("async flush failed: %w", err)
			}
			p.flushCount++
			p.lastFlush = now
		}
	}

	// For async writes, we don't sync immediately
	// The actual write will be handled by the buffer management
	return nil
}

// enforceBatchWrite handles batch write policy
func (p *PolicyEnforcer) enforceBatchWrite(ctx context.Context) error {
	// For batch writes, we rely on the caller to manage batching
	// This method ensures that when a batch is ready, it's handled correctly

	// If sync writes are enabled for batch mode, sync now
	if p.config.SyncWrites {
		if err := p.executeWithRetry(ctx, p.syncCallback, "batch_sync"); err != nil {
			return fmt.Errorf("batch sync failed: %w", err)
		}
		p.syncCount++
	} else {
		// Otherwise, just flush
		if err := p.executeWithRetry(ctx, p.flushCallback, "batch_flush"); err != nil {
			return fmt.Errorf("batch flush failed: %w", err)
		}
		p.flushCount++
	}

	return nil
}

// ForceFlush forces a flush operation regardless of configuration
func (p *PolicyEnforcer) ForceFlush(ctx context.Context) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if err := p.executeWithRetry(ctx, p.flushCallback, "force_flush"); err != nil {
		return fmt.Errorf("force flush failed: %w", err)
	}

	p.flushCount++
	p.lastFlush = time.Now()
	return nil
}

// ForceSync forces a sync operation regardless of configuration
func (p *PolicyEnforcer) ForceSync(ctx context.Context) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if err := p.executeWithRetry(ctx, p.syncCallback, "force_sync"); err != nil {
		return fmt.Errorf("force sync failed: %w", err)
	}

	p.syncCount++
	return nil
}

// executeWithRetry executes a callback with retry logic
func (p *PolicyEnforcer) executeWithRetry(ctx context.Context, callback func() error, operation string) error {
	if callback == nil {
		return fmt.Errorf("no callback configured for %s operation", operation)
	}

	var lastErr error
	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Execute the callback
		if err := callback(); err != nil {
			lastErr = err
			p.errorCount++

			// If this is not the last attempt, wait and retry
			if attempt < p.config.MaxRetries {
				p.retryCount++

				// Calculate retry delay with exponential backoff
				delay := p.config.RetryDelay * time.Duration(1<<uint(attempt))

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(delay):
					continue
				}
			}
		} else {
			// Success
			return nil
		}
	}

	return fmt.Errorf("%s operation failed after %d retries: %w", operation, p.config.MaxRetries, lastErr)
}

// GetMetrics returns current metrics
func (p *PolicyEnforcer) GetMetrics() PolicyMetrics {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return PolicyMetrics{
		WriteCount: p.writeCount,
		SyncCount:  p.syncCount,
		FlushCount: p.flushCount,
		ErrorCount: p.errorCount,
		RetryCount: p.retryCount,
		LastFlush:  p.lastFlush,
	}
}

// ResetMetrics resets all metrics to zero
func (p *PolicyEnforcer) ResetMetrics() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.writeCount = 0
	p.syncCount = 0
	p.flushCount = 0
	p.errorCount = 0
	p.retryCount = 0
}

// PolicyMetrics holds metrics about policy enforcement
type PolicyMetrics struct {
	WriteCount int64     `json:"write_count"`
	SyncCount  int64     `json:"sync_count"`
	FlushCount int64     `json:"flush_count"`
	ErrorCount int64     `json:"error_count"`
	RetryCount int64     `json:"retry_count"`
	LastFlush  time.Time `json:"last_flush"`
}

// ShouldFlush returns true if a flush should be performed based on current state
func (p *PolicyEnforcer) ShouldFlush() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if !p.config.RequiresFlush() {
		return false
	}

	return time.Since(p.lastFlush) >= p.config.FlushInterval
}

// GetTimeSinceLastFlush returns the time since the last flush
func (p *PolicyEnforcer) GetTimeSinceLastFlush() time.Duration {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return time.Since(p.lastFlush)
}
