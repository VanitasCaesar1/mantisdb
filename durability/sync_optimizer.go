package durability

import (
	"context"
	"sync"
	"time"
)

// SyncOptimizer provides performance optimizations for sync write operations
type SyncOptimizer struct {
	config *DurabilityConfig
	mutex  sync.RWMutex

	// Delegate to actual sync writer
	syncWriter *SyncWriter

	// Write coalescing for batching small writes
	pendingWrites map[string][]PendingWrite
	writeTimer    *time.Timer

	// Write-ahead buffer for reducing fsync frequency
	writeBuffer map[string]*WriteBuffer

	// Metrics
	coalescedWrites int64
	bufferedWrites  int64
	optimizedSyncs  int64
}

// PendingWrite represents a write operation waiting to be coalesced
type PendingWrite struct {
	Data      []byte
	Offset    int64
	Timestamp time.Time
	Done      chan error
}

// WriteBuffer manages buffered writes for a file
type WriteBuffer struct {
	Data         []byte
	LastWrite    time.Time
	PendingCount int
	MaxSize      int
}

// NewSyncOptimizer creates a new sync optimizer
func NewSyncOptimizer(config *DurabilityConfig) *SyncOptimizer {
	return &SyncOptimizer{
		config:        config,
		syncWriter:    NewSyncWriter(config),
		pendingWrites: make(map[string][]PendingWrite),
		writeBuffer:   make(map[string]*WriteBuffer),
	}
}

// OptimizeWrite optimizes a write operation based on configuration
func (so *SyncOptimizer) OptimizeWrite(ctx context.Context, filePath string, data []byte, offset int64) error {
	// For strict durability, no optimization - write immediately
	if so.config.Level == DurabilityStrict {
		return so.writeImmediate(ctx, filePath, data, offset)
	}

	// For sync mode with optimization enabled, try to coalesce writes
	if so.config.WriteMode == WriteModeSync && so.canCoalesce(filePath, data) {
		return so.coalesceWrite(ctx, filePath, data, offset)
	}

	// For buffered sync writes
	if so.canBuffer(filePath, data) {
		return so.bufferWrite(ctx, filePath, data, offset)
	}

	// Default to immediate write
	return so.writeImmediate(ctx, filePath, data, offset)
}

// coalesceWrite attempts to coalesce multiple small writes
func (so *SyncOptimizer) coalesceWrite(ctx context.Context, filePath string, data []byte, offset int64) error {
	so.mutex.Lock()

	// Create done channel for this write
	done := make(chan error, 1)

	// Add to pending writes
	pending := PendingWrite{
		Data:      data,
		Offset:    offset,
		Timestamp: time.Now(),
		Done:      done,
	}

	so.pendingWrites[filePath] = append(so.pendingWrites[filePath], pending)

	// Start timer if this is the first pending write for this file
	if len(so.pendingWrites[filePath]) == 1 {
		so.writeTimer = time.AfterFunc(so.config.BatchTimeout, func() {
			so.flushPendingWrites(filePath)
		})
	}

	// Check if we should flush immediately due to batch size
	shouldFlushNow := len(so.pendingWrites[filePath]) >= so.config.BatchSize

	// Release lock before potentially flushing
	so.mutex.Unlock()

	if shouldFlushNow {
		so.flushPendingWrites(filePath)
	}

	// Wait for the write to complete
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// bufferWrite buffers writes to reduce fsync frequency
func (so *SyncOptimizer) bufferWrite(ctx context.Context, filePath string, data []byte, offset int64) error {
	so.mutex.Lock()
	defer so.mutex.Unlock()

	// Get or create buffer for this file
	buffer, exists := so.writeBuffer[filePath]
	if !exists {
		buffer = &WriteBuffer{
			Data:    make([]byte, 0, so.config.BufferSize),
			MaxSize: so.config.BufferSize,
		}
		so.writeBuffer[filePath] = buffer
	}

	// Check if adding this write would exceed buffer size
	if len(buffer.Data)+len(data) > buffer.MaxSize {
		// Flush current buffer first
		if err := so.flushBuffer(ctx, filePath, buffer); err != nil {
			return err
		}
		// Reset buffer
		buffer.Data = buffer.Data[:0]
		buffer.PendingCount = 0
	}

	// Add data to buffer
	buffer.Data = append(buffer.Data, data...)
	buffer.LastWrite = time.Now()
	buffer.PendingCount++
	so.bufferedWrites++

	// Check if we should flush based on time or count
	shouldFlush := false
	if buffer.PendingCount >= so.config.BatchSize {
		shouldFlush = true
	} else if time.Since(buffer.LastWrite) >= so.config.BatchTimeout {
		shouldFlush = true
	}

	if shouldFlush {
		return so.flushBuffer(ctx, filePath, buffer)
	}

	return nil
}

// writeImmediate performs an immediate write without optimization
func (so *SyncOptimizer) writeImmediate(ctx context.Context, filePath string, data []byte, offset int64) error {
	// Delegate to the actual sync writer
	return so.syncWriter.WriteSync(ctx, filePath, data, offset)
}

// flushPendingWrites flushes all pending writes for a file
func (so *SyncOptimizer) flushPendingWrites(filePath string) {
	so.mutex.Lock()
	pending := so.pendingWrites[filePath]
	delete(so.pendingWrites, filePath)
	so.mutex.Unlock()

	if len(pending) == 0 {
		return
	}

	// Cancel timer if it exists
	if so.writeTimer != nil {
		so.writeTimer.Stop()
		so.writeTimer = nil
	}

	// Combine all writes into a batch
	var combinedData []byte
	for _, write := range pending {
		combinedData = append(combinedData, write.Data...)
	}

	// Perform the actual write (simulated)
	err := so.performBatchWrite(filePath, combinedData)

	// Notify all waiting goroutines
	for _, write := range pending {
		write.Done <- err
		close(write.Done)
	}

	so.mutex.Lock()
	so.coalescedWrites += int64(len(pending))
	so.optimizedSyncs++
	so.mutex.Unlock()
}

// flushBuffer flushes a write buffer
func (so *SyncOptimizer) flushBuffer(ctx context.Context, filePath string, buffer *WriteBuffer) error {
	if len(buffer.Data) == 0 {
		return nil
	}

	// Perform the actual write
	err := so.performBatchWrite(filePath, buffer.Data)

	// Reset buffer on success
	if err == nil {
		buffer.Data = buffer.Data[:0]
		buffer.PendingCount = 0
		so.optimizedSyncs++
	}

	return err
}

// performBatchWrite performs the actual batch write operation
func (so *SyncOptimizer) performBatchWrite(filePath string, data []byte) error {
	// Delegate to the actual sync writer
	return so.syncWriter.WriteSync(context.Background(), filePath, data, -1)
}

// canCoalesce determines if a write can be coalesced
func (so *SyncOptimizer) canCoalesce(filePath string, data []byte) bool {
	// Don't coalesce very large writes
	if len(data) > so.config.BufferSize/4 {
		return false
	}

	// Don't coalesce if we're in strict mode
	if so.config.Level == DurabilityStrict {
		return false
	}

	return true
}

// canBuffer determines if a write can be buffered
func (so *SyncOptimizer) canBuffer(filePath string, data []byte) bool {
	// Don't buffer in strict mode
	if so.config.Level == DurabilityStrict {
		return false
	}

	// Don't buffer very large writes
	if len(data) > so.config.BufferSize {
		return false
	}

	return so.config.BufferSize > 0
}

// FlushAll flushes all pending writes and buffers
func (so *SyncOptimizer) FlushAll(ctx context.Context) error {
	so.mutex.Lock()

	// Flush all pending writes
	for filePath := range so.pendingWrites {
		go so.flushPendingWrites(filePath)
	}

	// Flush all buffers
	for filePath, buffer := range so.writeBuffer {
		if err := so.flushBuffer(ctx, filePath, buffer); err != nil {
			so.mutex.Unlock()
			return err
		}
	}

	so.mutex.Unlock()
	return nil
}

// GetMetrics returns optimizer metrics
func (so *SyncOptimizer) GetMetrics() SyncOptimizerMetrics {
	so.mutex.RLock()
	defer so.mutex.RUnlock()

	pendingCount := 0
	for _, pending := range so.pendingWrites {
		pendingCount += len(pending)
	}

	bufferedBytes := 0
	for _, buffer := range so.writeBuffer {
		bufferedBytes += len(buffer.Data)
	}

	return SyncOptimizerMetrics{
		CoalescedWrites: so.coalescedWrites,
		BufferedWrites:  so.bufferedWrites,
		OptimizedSyncs:  so.optimizedSyncs,
		PendingWrites:   int64(pendingCount),
		BufferedBytes:   int64(bufferedBytes),
	}
}

// GetSyncWriterMetrics returns metrics from the underlying sync writer
func (so *SyncOptimizer) GetSyncWriterMetrics() SyncWriterMetrics {
	return so.syncWriter.GetMetrics()
}

// Close cleans up the optimizer
func (so *SyncOptimizer) Close(ctx context.Context) error {
	// Flush all pending operations
	if err := so.FlushAll(ctx); err != nil {
		return err
	}

	// Close the underlying sync writer
	return so.syncWriter.Close()
}

// SyncOptimizerMetrics holds metrics for sync optimization
type SyncOptimizerMetrics struct {
	CoalescedWrites int64 `json:"coalesced_writes"`
	BufferedWrites  int64 `json:"buffered_writes"`
	OptimizedSyncs  int64 `json:"optimized_syncs"`
	PendingWrites   int64 `json:"pending_writes"`
	BufferedBytes   int64 `json:"buffered_bytes"`
}
