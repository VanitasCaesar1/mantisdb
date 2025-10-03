package durability

import (
	"context"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"
)

// SyncWriter handles synchronous write operations with fsync
type SyncWriter struct {
	config *DurabilityConfig
	mutex  sync.RWMutex

	// File handles for sync operations
	files map[string]*os.File

	// Metrics
	syncOperations int64
	syncLatency    time.Duration
	barrierOps     int64
	fsyncErrors    int64
}

// NewSyncWriter creates a new synchronous writer
func NewSyncWriter(config *DurabilityConfig) *SyncWriter {
	return &SyncWriter{
		config: config,
		files:  make(map[string]*os.File),
	}
}

// WriteSync performs a synchronous write with fsync
func (sw *SyncWriter) WriteSync(ctx context.Context, filePath string, data []byte, offset int64) error {
	start := time.Now()
	defer func() {
		sw.mutex.Lock()
		sw.syncOperations++
		sw.syncLatency += time.Since(start)
		sw.mutex.Unlock()
	}()

	// Get or create file handle
	file, err := sw.getFileHandle(filePath)
	if err != nil {
		return fmt.Errorf("failed to get file handle: %w", err)
	}

	// Write barriers before write if required
	if sw.config.RequiresBarriers() {
		if err := sw.writeBarrier(ctx, file); err != nil {
			return fmt.Errorf("write barrier failed: %w", err)
		}
	}

	// Perform the write
	if offset >= 0 {
		_, err = file.WriteAt(data, offset)
	} else {
		_, err = file.Write(data)
	}

	if err != nil {
		return fmt.Errorf("write operation failed: %w", err)
	}

	// Force sync to disk
	if err := sw.fsyncFile(ctx, file); err != nil {
		sw.mutex.Lock()
		sw.fsyncErrors++
		sw.mutex.Unlock()
		return fmt.Errorf("fsync failed: %w", err)
	}

	// Write barriers after write if required for strict ordering
	if sw.config.RequiresBarriers() && sw.config.Level == DurabilityStrict {
		if err := sw.writeBarrier(ctx, file); err != nil {
			return fmt.Errorf("post-write barrier failed: %w", err)
		}
	}

	return nil
}

// BatchWriteSync performs a batch of synchronous writes
func (sw *SyncWriter) BatchWriteSync(ctx context.Context, writes []SyncWrite) error {
	if len(writes) == 0 {
		return nil
	}

	start := time.Now()
	defer func() {
		sw.mutex.Lock()
		sw.syncOperations += int64(len(writes))
		sw.syncLatency += time.Since(start)
		sw.mutex.Unlock()
	}()

	// Group writes by file for efficiency
	fileWrites := make(map[string][]SyncWrite)
	for _, write := range writes {
		fileWrites[write.FilePath] = append(fileWrites[write.FilePath], write)
	}

	// Process each file's writes
	for filePath, fileWriteList := range fileWrites {
		file, err := sw.getFileHandle(filePath)
		if err != nil {
			return fmt.Errorf("failed to get file handle for %s: %w", filePath, err)
		}

		// Write barriers before batch if required
		if sw.config.RequiresBarriers() {
			if err := sw.writeBarrier(ctx, file); err != nil {
				return fmt.Errorf("pre-batch barrier failed for %s: %w", filePath, err)
			}
		}

		// Perform all writes for this file
		for _, write := range fileWriteList {
			if write.Offset >= 0 {
				_, err = file.WriteAt(write.Data, write.Offset)
			} else {
				_, err = file.Write(write.Data)
			}

			if err != nil {
				return fmt.Errorf("batch write failed for %s: %w", filePath, err)
			}
		}

		// Sync this file
		if err := sw.fsyncFile(ctx, file); err != nil {
			sw.mutex.Lock()
			sw.fsyncErrors++
			sw.mutex.Unlock()
			return fmt.Errorf("batch fsync failed for %s: %w", filePath, err)
		}

		// Post-write barriers if required
		if sw.config.RequiresBarriers() && sw.config.Level == DurabilityStrict {
			if err := sw.writeBarrier(ctx, file); err != nil {
				return fmt.Errorf("post-batch barrier failed for %s: %w", filePath, err)
			}
		}
	}

	return nil
}

// FsyncFile forces a file to be synced to disk
func (sw *SyncWriter) FsyncFile(ctx context.Context, filePath string) error {
	file, err := sw.getFileHandle(filePath)
	if err != nil {
		return fmt.Errorf("failed to get file handle: %w", err)
	}

	return sw.fsyncFile(ctx, file)
}

// FsyncAll forces all open files to be synced to disk
func (sw *SyncWriter) FsyncAll(ctx context.Context) error {
	sw.mutex.RLock()
	files := make([]*os.File, 0, len(sw.files))
	for _, file := range sw.files {
		files = append(files, file)
	}
	sw.mutex.RUnlock()

	for _, file := range files {
		if err := sw.fsyncFile(ctx, file); err != nil {
			return fmt.Errorf("fsync all failed: %w", err)
		}
	}

	return nil
}

// WriteBarrier implements write barriers for ordering guarantees
func (sw *SyncWriter) WriteBarrier(ctx context.Context, filePath string) error {
	if filePath == "" {
		// For global barriers, sync all open files
		return sw.FsyncAll(ctx)
	}

	file, err := sw.getFileHandle(filePath)
	if err != nil {
		return fmt.Errorf("failed to get file handle: %w", err)
	}

	return sw.writeBarrier(ctx, file)
}

// Close closes all file handles and cleans up resources
func (sw *SyncWriter) Close() error {
	sw.mutex.Lock()
	defer sw.mutex.Unlock()

	var lastErr error
	for path, file := range sw.files {
		if err := file.Close(); err != nil {
			lastErr = fmt.Errorf("failed to close file %s: %w", path, err)
		}
	}

	sw.files = make(map[string]*os.File)
	return lastErr
}

// GetMetrics returns sync writer metrics
func (sw *SyncWriter) GetMetrics() SyncWriterMetrics {
	sw.mutex.RLock()
	defer sw.mutex.RUnlock()

	avgLatency := time.Duration(0)
	if sw.syncOperations > 0 {
		avgLatency = sw.syncLatency / time.Duration(sw.syncOperations)
	}

	return SyncWriterMetrics{
		SyncOperations: sw.syncOperations,
		AverageLatency: avgLatency,
		TotalLatency:   sw.syncLatency,
		BarrierOps:     sw.barrierOps,
		FsyncErrors:    sw.fsyncErrors,
	}
}

// getFileHandle gets or creates a file handle for the given path
func (sw *SyncWriter) getFileHandle(filePath string) (*os.File, error) {
	sw.mutex.RLock()
	if file, exists := sw.files[filePath]; exists {
		sw.mutex.RUnlock()
		return file, nil
	}
	sw.mutex.RUnlock()

	sw.mutex.Lock()
	defer sw.mutex.Unlock()

	// Double-check after acquiring write lock
	if file, exists := sw.files[filePath]; exists {
		return file, nil
	}

	// Open file with appropriate flags for sync writes
	flags := os.O_WRONLY | os.O_CREATE
	if sw.config.Level == DurabilityStrict {
		// Use O_SYNC for strict durability (writes are synchronous at OS level)
		flags |= os.O_SYNC
	}

	file, err := os.OpenFile(filePath, flags, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}

	sw.files[filePath] = file
	return file, nil
}

// fsyncFile performs fsync with retry logic and context cancellation
func (sw *SyncWriter) fsyncFile(ctx context.Context, file *os.File) error {
	// Check context before starting
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Perform fsync with retry logic
	var lastErr error
	for attempt := 0; attempt <= sw.config.MaxRetries; attempt++ {
		if err := file.Sync(); err != nil {
			lastErr = err

			// Check if this is a retryable error
			if !isRetryableError(err) {
				return fmt.Errorf("non-retryable fsync error: %w", err)
			}

			// If not the last attempt, wait and retry
			if attempt < sw.config.MaxRetries {
				delay := sw.config.RetryDelay * time.Duration(1<<uint(attempt))
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(delay):
					continue
				}
			}
		} else {
			return nil // Success
		}
	}

	return fmt.Errorf("fsync failed after %d retries: %w", sw.config.MaxRetries, lastErr)
}

// writeBarrier implements write barriers for ordering guarantees
func (sw *SyncWriter) writeBarrier(ctx context.Context, file *os.File) error {
	sw.mutex.Lock()
	sw.barrierOps++
	sw.mutex.Unlock()

	// Check context
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// On Linux, we can use fdatasync for data-only barriers
	// For full barriers, we use fsync
	if sw.config.Level == DurabilityStrict {
		return file.Sync() // Full barrier
	} else {
		// Try to use fdatasync if available (Linux-specific)
		if fd := file.Fd(); fd != 0 {
			if _, _, err := syscall.Syscall(syscall.SYS_FDATASYNC, uintptr(fd), 0, 0); err != 0 {
				// Fall back to fsync if fdatasync fails
				return file.Sync()
			}
			return nil
		}
		return file.Sync()
	}
}

// isRetryableError determines if an error is retryable
func isRetryableError(err error) bool {
	// Check for temporary errors that might be retryable
	if err == syscall.EINTR || err == syscall.EAGAIN || err == syscall.EBUSY {
		return true
	}

	// Check for temporary network/IO errors
	if pathErr, ok := err.(*os.PathError); ok {
		if pathErr.Err == syscall.EINTR || pathErr.Err == syscall.EAGAIN {
			return true
		}
	}

	return false
}

// SyncWrite represents a single synchronous write operation
type SyncWrite struct {
	FilePath string
	Data     []byte
	Offset   int64 // -1 for append
}

// SyncWriterMetrics holds metrics for sync writer operations
type SyncWriterMetrics struct {
	SyncOperations int64         `json:"sync_operations"`
	AverageLatency time.Duration `json:"average_latency"`
	TotalLatency   time.Duration `json:"total_latency"`
	BarrierOps     int64         `json:"barrier_operations"`
	FsyncErrors    int64         `json:"fsync_errors"`
}
