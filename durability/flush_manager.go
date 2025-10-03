package durability

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// FlushManager manages periodic flush operations for async writes
type FlushManager struct {
	config *DurabilityConfig
	mutex  sync.RWMutex

	// Flush scheduling
	flushTimer    *time.Timer
	flushTicker   *time.Ticker
	stopChan      chan struct{}
	flushRequests chan FlushRequest

	// Registered writers
	asyncWriters []AsyncWriterInterface
	syncWriters  []SyncWriterInterface

	// Flush state
	lastFlush       time.Time
	flushInProgress bool

	// Metrics
	scheduledFlushes int64
	forcedFlushes    int64
	flushDuration    time.Duration
	flushErrors      int64

	// Background goroutine
	wg sync.WaitGroup
}

// FlushRequest represents a request to flush specific files or all files
type FlushRequest struct {
	FilePaths []string // Empty means flush all
	Priority  FlushPriority
	Done      chan error
}

// FlushPriority defines the priority of flush operations
type FlushPriority int

const (
	FlushPriorityLow FlushPriority = iota
	FlushPriorityNormal
	FlushPriorityHigh
	FlushPriorityCritical
)

// AsyncWriterInterface defines the interface for async writers
type AsyncWriterInterface interface {
	FlushFile(ctx context.Context, filePath string) error
	FlushAll(ctx context.Context) error
	GetUnflushedCount(filePath string) int64
	GetTotalUnflushedCount() int64
}

// SyncWriterInterface defines the interface for sync writers
type SyncWriterInterface interface {
	FsyncFile(ctx context.Context, filePath string) error
	FsyncAll(ctx context.Context) error
}

// NewFlushManager creates a new flush manager
func NewFlushManager(config *DurabilityConfig) *FlushManager {
	fm := &FlushManager{
		config:        config,
		stopChan:      make(chan struct{}),
		flushRequests: make(chan FlushRequest, 100),
		lastFlush:     time.Now(),
	}

	// Start background flush routine
	fm.startFlushRoutine()

	return fm
}

// RegisterAsyncWriter registers an async writer for periodic flushing
func (fm *FlushManager) RegisterAsyncWriter(writer AsyncWriterInterface) {
	fm.mutex.Lock()
	defer fm.mutex.Unlock()

	fm.asyncWriters = append(fm.asyncWriters, writer)
}

// RegisterSyncWriter registers a sync writer for forced sync operations
func (fm *FlushManager) RegisterSyncWriter(writer SyncWriterInterface) {
	fm.mutex.Lock()
	defer fm.mutex.Unlock()

	fm.syncWriters = append(fm.syncWriters, writer)
}

// RequestFlush requests a flush operation with specified priority
func (fm *FlushManager) RequestFlush(filePaths []string, priority FlushPriority) error {
	done := make(chan error, 1)

	request := FlushRequest{
		FilePaths: filePaths,
		Priority:  priority,
		Done:      done,
	}

	// Send request to background routine
	select {
	case fm.flushRequests <- request:
		// Wait for completion
		return <-done
	case <-time.After(30 * time.Second):
		return fmt.Errorf("flush request timed out")
	}
}

// ForceFlush forces an immediate flush of all writers
func (fm *FlushManager) ForceFlush(ctx context.Context) error {
	fm.mutex.Lock()
	if fm.flushInProgress {
		fm.mutex.Unlock()
		return fmt.Errorf("flush already in progress")
	}
	fm.flushInProgress = true
	fm.mutex.Unlock()

	defer func() {
		fm.mutex.Lock()
		fm.flushInProgress = false
		fm.forcedFlushes++
		fm.mutex.Unlock()
	}()

	return fm.performFlush(ctx, nil)
}

// ScheduleFlush schedules a flush to occur after the specified delay
func (fm *FlushManager) ScheduleFlush(delay time.Duration, filePaths []string) {
	fm.mutex.Lock()
	defer fm.mutex.Unlock()

	// Cancel existing timer if any
	if fm.flushTimer != nil {
		fm.flushTimer.Stop()
	}

	// Schedule new flush
	fm.flushTimer = time.AfterFunc(delay, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := fm.performFlush(ctx, filePaths); err != nil {
			fmt.Printf("Scheduled flush error: %v\n", err)
		}
	})
}

// GetFlushStatus returns the current flush status
func (fm *FlushManager) GetFlushStatus() FlushStatus {
	fm.mutex.RLock()
	defer fm.mutex.RUnlock()

	// Calculate total unflushed writes
	var totalUnflushed int64
	for _, writer := range fm.asyncWriters {
		totalUnflushed += writer.GetTotalUnflushedCount()
	}

	return FlushStatus{
		LastFlush:       fm.lastFlush,
		FlushInProgress: fm.flushInProgress,
		UnflushedWrites: totalUnflushed,
		NextFlush:       fm.calculateNextFlush(),
	}
}

// Close stops the flush manager and performs final flush
func (fm *FlushManager) Close(ctx context.Context) error {
	// Stop background routine
	close(fm.stopChan)
	fm.wg.Wait()

	// Cancel any pending timer
	fm.mutex.Lock()
	if fm.flushTimer != nil {
		fm.flushTimer.Stop()
	}
	if fm.flushTicker != nil {
		fm.flushTicker.Stop()
	}
	fm.mutex.Unlock()

	// Perform final flush
	return fm.ForceFlush(ctx)
}

// startFlushRoutine starts the background flush routine
func (fm *FlushManager) startFlushRoutine() {
	fm.wg.Add(1)
	go func() {
		defer fm.wg.Done()

		// Start periodic flush ticker if configured
		var tickerChan <-chan time.Time
		if fm.config.RequiresFlush() {
			fm.flushTicker = time.NewTicker(fm.config.FlushInterval)
			tickerChan = fm.flushTicker.C
		}

		for {
			select {
			case <-fm.stopChan:
				return

			case <-tickerChan:
				// Periodic flush (only if ticker is configured)
				if tickerChan != nil {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					if err := fm.performPeriodicFlush(ctx); err != nil {
						fmt.Printf("Periodic flush error: %v\n", err)
					}
					cancel()
				}

			case request := <-fm.flushRequests:
				// Handle flush request
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				err := fm.handleFlushRequest(ctx, request)
				cancel()

				// Send result back
				request.Done <- err
				close(request.Done)
			}
		}
	}()
}

// performPeriodicFlush performs a periodic flush based on configuration
func (fm *FlushManager) performPeriodicFlush(ctx context.Context) error {
	fm.mutex.Lock()
	if fm.flushInProgress {
		fm.mutex.Unlock()
		return nil // Skip if flush is already in progress
	}

	// Check if enough time has passed since last flush
	if time.Since(fm.lastFlush) < fm.config.FlushInterval {
		fm.mutex.Unlock()
		return nil
	}

	fm.flushInProgress = true
	fm.mutex.Unlock()

	defer func() {
		fm.mutex.Lock()
		fm.flushInProgress = false
		fm.scheduledFlushes++
		fm.mutex.Unlock()
	}()

	return fm.performFlush(ctx, nil)
}

// handleFlushRequest handles a specific flush request
func (fm *FlushManager) handleFlushRequest(ctx context.Context, request FlushRequest) error {
	// For critical priority, bypass normal scheduling
	if request.Priority == FlushPriorityCritical {
		return fm.performFlush(ctx, request.FilePaths)
	}

	// For other priorities, check if we should flush now
	fm.mutex.RLock()
	shouldFlush := !fm.flushInProgress && (request.Priority >= FlushPriorityHigh ||
		time.Since(fm.lastFlush) >= fm.config.FlushInterval/2)
	fm.mutex.RUnlock()

	if shouldFlush {
		return fm.performFlush(ctx, request.FilePaths)
	}

	// Schedule for later
	delay := fm.config.FlushInterval - time.Since(fm.lastFlush)
	if delay < 0 {
		delay = 0
	}

	fm.ScheduleFlush(delay, request.FilePaths)
	return nil
}

// performFlush performs the actual flush operation
func (fm *FlushManager) performFlush(ctx context.Context, filePaths []string) error {
	start := time.Now()

	// Flush async writers
	for _, writer := range fm.asyncWriters {
		var err error
		if len(filePaths) == 0 {
			err = writer.FlushAll(ctx)
		} else {
			for _, filePath := range filePaths {
				if flushErr := writer.FlushFile(ctx, filePath); flushErr != nil {
					err = flushErr
					break
				}
			}
		}

		if err != nil {
			fm.mutex.Lock()
			fm.flushErrors++
			fm.mutex.Unlock()
			return fmt.Errorf("async writer flush failed: %w", err)
		}
	}

	// Sync writers if required
	if fm.config.Level >= DurabilitySync {
		for _, writer := range fm.syncWriters {
			var err error
			if len(filePaths) == 0 {
				err = writer.FsyncAll(ctx)
			} else {
				for _, filePath := range filePaths {
					if syncErr := writer.FsyncFile(ctx, filePath); syncErr != nil {
						err = syncErr
						break
					}
				}
			}

			if err != nil {
				fm.mutex.Lock()
				fm.flushErrors++
				fm.mutex.Unlock()
				return fmt.Errorf("sync writer flush failed: %w", err)
			}
		}
	}

	// Update state
	fm.mutex.Lock()
	fm.lastFlush = time.Now()
	fm.flushDuration += time.Since(start)
	fm.mutex.Unlock()

	return nil
}

// calculateNextFlush calculates when the next flush should occur
func (fm *FlushManager) calculateNextFlush() time.Time {
	if !fm.config.RequiresFlush() {
		return time.Time{}
	}

	return fm.lastFlush.Add(fm.config.FlushInterval)
}

// GetMetrics returns flush manager metrics
func (fm *FlushManager) GetMetrics() FlushManagerMetrics {
	fm.mutex.RLock()
	defer fm.mutex.RUnlock()

	avgFlushDuration := time.Duration(0)
	totalFlushes := fm.scheduledFlushes + fm.forcedFlushes
	if totalFlushes > 0 {
		avgFlushDuration = fm.flushDuration / time.Duration(totalFlushes)
	}

	return FlushManagerMetrics{
		ScheduledFlushes:     fm.scheduledFlushes,
		ForcedFlushes:        fm.forcedFlushes,
		FlushErrors:          fm.flushErrors,
		AverageFlushDuration: avgFlushDuration,
		TotalFlushDuration:   fm.flushDuration,
		LastFlush:            fm.lastFlush,
		FlushInProgress:      fm.flushInProgress,
	}
}

// FlushStatus represents the current flush status
type FlushStatus struct {
	LastFlush       time.Time `json:"last_flush"`
	FlushInProgress bool      `json:"flush_in_progress"`
	UnflushedWrites int64     `json:"unflushed_writes"`
	NextFlush       time.Time `json:"next_flush"`
}

// FlushManagerMetrics holds metrics for flush manager operations
type FlushManagerMetrics struct {
	ScheduledFlushes     int64         `json:"scheduled_flushes"`
	ForcedFlushes        int64         `json:"forced_flushes"`
	FlushErrors          int64         `json:"flush_errors"`
	AverageFlushDuration time.Duration `json:"average_flush_duration"`
	TotalFlushDuration   time.Duration `json:"total_flush_duration"`
	LastFlush            time.Time     `json:"last_flush"`
	FlushInProgress      bool          `json:"flush_in_progress"`
}
