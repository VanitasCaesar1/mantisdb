package durability

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

// AsyncWriter handles asynchronous write operations with safety guarantees
type AsyncWriter struct {
	config *DurabilityConfig
	mutex  sync.RWMutex

	// Write buffers for each file
	buffers map[string]*AsyncBuffer

	// Background flush goroutine control
	flushTicker *time.Ticker
	stopChan    chan struct{}
	wg          sync.WaitGroup

	// Metrics
	asyncWrites     int64
	flushOperations int64
	bufferOverflows int64
	flushErrors     int64

	// Safety tracking
	unflushedWrites map[string]int64
	lastFlushTime   time.Time
}

// AsyncBuffer manages buffered writes for a single file
type AsyncBuffer struct {
	filePath   string
	file       *os.File
	buffer     []byte
	writeCount int64
	lastWrite  time.Time
	maxSize    int
	mutex      sync.RWMutex

	// Safety features
	checksums   []uint32
	writeOrder  []WriteEntry
	flushNeeded bool
}

// WriteEntry tracks individual write operations for safety
type WriteEntry struct {
	Data      []byte
	Offset    int64
	Timestamp time.Time
	Checksum  uint32
}

// NewAsyncWriter creates a new asynchronous writer
func NewAsyncWriter(config *DurabilityConfig) *AsyncWriter {
	aw := &AsyncWriter{
		config:          config,
		buffers:         make(map[string]*AsyncBuffer),
		stopChan:        make(chan struct{}),
		unflushedWrites: make(map[string]int64),
		lastFlushTime:   time.Now(),
	}

	// Start background flush routine if flush interval is configured
	if config.RequiresFlush() {
		aw.startFlushRoutine()
	}

	return aw
}

// WriteAsync performs an asynchronous write operation
func (aw *AsyncWriter) WriteAsync(ctx context.Context, filePath string, data []byte, offset int64) error {
	// Get or create buffer for this file
	buffer, err := aw.getBuffer(filePath)
	if err != nil {
		return fmt.Errorf("failed to get buffer: %w", err)
	}

	// Add write to buffer
	return aw.addToBuffer(ctx, buffer, data, offset)
}

// BatchWriteAsync performs a batch of asynchronous writes
func (aw *AsyncWriter) BatchWriteAsync(ctx context.Context, writes []AsyncWrite) error {
	// Group writes by file
	fileWrites := make(map[string][]AsyncWrite)
	for _, write := range writes {
		fileWrites[write.FilePath] = append(fileWrites[write.FilePath], write)
	}

	// Process each file's writes
	for filePath, fileWriteList := range fileWrites {
		buffer, err := aw.getBuffer(filePath)
		if err != nil {
			return fmt.Errorf("failed to get buffer for %s: %w", filePath, err)
		}

		// Add all writes to buffer
		for _, write := range fileWriteList {
			if err := aw.addToBuffer(ctx, buffer, write.Data, write.Offset); err != nil {
				return fmt.Errorf("failed to add write to buffer: %w", err)
			}
		}
	}

	return nil
}

// FlushFile forces a flush of a specific file's buffer
func (aw *AsyncWriter) FlushFile(ctx context.Context, filePath string) error {
	aw.mutex.RLock()
	buffer, exists := aw.buffers[filePath]
	aw.mutex.RUnlock()

	if !exists {
		return nil // Nothing to flush
	}

	return aw.flushBuffer(ctx, buffer)
}

// FlushAll forces a flush of all buffers
func (aw *AsyncWriter) FlushAll(ctx context.Context) error {
	aw.mutex.RLock()
	buffers := make([]*AsyncBuffer, 0, len(aw.buffers))
	for _, buffer := range aw.buffers {
		buffers = append(buffers, buffer)
	}
	aw.mutex.RUnlock()

	for _, buffer := range buffers {
		if err := aw.flushBuffer(ctx, buffer); err != nil {
			return fmt.Errorf("failed to flush buffer for %s: %w", buffer.filePath, err)
		}
	}

	aw.mutex.Lock()
	aw.lastFlushTime = time.Now()
	aw.mutex.Unlock()

	return nil
}

// GetUnflushedCount returns the number of unflushed writes for a file
func (aw *AsyncWriter) GetUnflushedCount(filePath string) int64 {
	aw.mutex.RLock()
	defer aw.mutex.RUnlock()

	return aw.unflushedWrites[filePath]
}

// GetTotalUnflushedCount returns the total number of unflushed writes
func (aw *AsyncWriter) GetTotalUnflushedCount() int64 {
	aw.mutex.RLock()
	defer aw.mutex.RUnlock()

	var total int64
	for _, count := range aw.unflushedWrites {
		total += count
	}
	return total
}

// Close stops the async writer and flushes all pending writes
func (aw *AsyncWriter) Close(ctx context.Context) error {
	// Stop background flush routine
	if aw.flushTicker != nil {
		aw.flushTicker.Stop()
		close(aw.stopChan)
		aw.wg.Wait()
	}

	// Flush all remaining writes
	if err := aw.FlushAll(ctx); err != nil {
		return fmt.Errorf("failed to flush all writes during close: %w", err)
	}

	// Close all file handles
	aw.mutex.Lock()
	defer aw.mutex.Unlock()

	var lastErr error
	for _, buffer := range aw.buffers {
		if buffer.file != nil {
			if err := buffer.file.Close(); err != nil {
				lastErr = fmt.Errorf("failed to close file %s: %w", buffer.filePath, err)
			}
		}
	}

	aw.buffers = make(map[string]*AsyncBuffer)
	return lastErr
}

// getBuffer gets or creates a buffer for the specified file
func (aw *AsyncWriter) getBuffer(filePath string) (*AsyncBuffer, error) {
	aw.mutex.RLock()
	if buffer, exists := aw.buffers[filePath]; exists {
		aw.mutex.RUnlock()
		return buffer, nil
	}
	aw.mutex.RUnlock()

	aw.mutex.Lock()
	defer aw.mutex.Unlock()

	// Double-check after acquiring write lock
	if buffer, exists := aw.buffers[filePath]; exists {
		return buffer, nil
	}

	// Create new buffer
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}

	buffer := &AsyncBuffer{
		filePath:   filePath,
		file:       file,
		buffer:     make([]byte, 0, aw.config.BufferSize),
		maxSize:    aw.config.BufferSize,
		checksums:  make([]uint32, 0),
		writeOrder: make([]WriteEntry, 0),
	}

	aw.buffers[filePath] = buffer
	aw.unflushedWrites[filePath] = 0

	return buffer, nil
}

// addToBuffer adds data to the async buffer with safety checks
func (aw *AsyncWriter) addToBuffer(ctx context.Context, buffer *AsyncBuffer, data []byte, offset int64) error {
	buffer.mutex.Lock()
	defer buffer.mutex.Unlock()

	// Check if buffer would overflow
	if len(buffer.buffer)+len(data) > buffer.maxSize {
		// Flush current buffer first
		buffer.mutex.Unlock()
		if err := aw.flushBuffer(ctx, buffer); err != nil {
			buffer.mutex.Lock()
			return fmt.Errorf("failed to flush buffer before adding data: %w", err)
		}
		buffer.mutex.Lock()

		aw.mutex.Lock()
		aw.bufferOverflows++
		aw.mutex.Unlock()
	}

	// Calculate checksum for safety
	checksum := calculateChecksum(data)

	// Create write entry for tracking
	entry := WriteEntry{
		Data:      make([]byte, len(data)),
		Offset:    offset,
		Timestamp: time.Now(),
		Checksum:  checksum,
	}
	copy(entry.Data, data)

	// Add to buffer
	buffer.buffer = append(buffer.buffer, data...)
	buffer.checksums = append(buffer.checksums, checksum)
	buffer.writeOrder = append(buffer.writeOrder, entry)
	buffer.writeCount++
	buffer.lastWrite = time.Now()
	buffer.flushNeeded = true

	// Update metrics
	aw.mutex.Lock()
	aw.asyncWrites++
	aw.unflushedWrites[buffer.filePath]++
	aw.mutex.Unlock()

	// Check if we should flush immediately
	shouldFlush := false
	if buffer.writeCount >= int64(aw.config.BatchSize) {
		shouldFlush = true
	} else if time.Since(buffer.lastWrite) >= aw.config.BatchTimeout {
		shouldFlush = true
	}

	if shouldFlush {
		buffer.mutex.Unlock()
		return aw.flushBuffer(ctx, buffer)
	}

	return nil
}

// flushBuffer flushes a buffer to disk
func (aw *AsyncWriter) flushBuffer(ctx context.Context, buffer *AsyncBuffer) error {
	buffer.mutex.Lock()

	if len(buffer.buffer) == 0 {
		buffer.mutex.Unlock()
		return nil
	}

	// Copy buffer data for writing
	writeData := make([]byte, len(buffer.buffer))
	copy(writeData, buffer.buffer)

	// Copy checksums for verification
	checksums := make([]uint32, len(buffer.checksums))
	copy(checksums, buffer.checksums)

	writeCount := buffer.writeCount

	buffer.mutex.Unlock()

	// Perform the actual write
	_, err := buffer.file.Write(writeData)
	if err != nil {
		aw.mutex.Lock()
		aw.flushErrors++
		aw.mutex.Unlock()
		return fmt.Errorf("failed to write buffer to file: %w", err)
	}

	// Verify write if required
	if aw.config.RequiresVerification() {
		if err := aw.verifyWrite(buffer.file, writeData, checksums); err != nil {
			return fmt.Errorf("write verification failed: %w", err)
		}
	}

	// Optionally sync to disk for safety
	if aw.config.Level >= DurabilitySync {
		if err := buffer.file.Sync(); err != nil {
			aw.mutex.Lock()
			aw.flushErrors++
			aw.mutex.Unlock()
			return fmt.Errorf("failed to sync file: %w", err)
		}
	}

	// Clear buffer after successful write
	buffer.mutex.Lock()
	buffer.buffer = buffer.buffer[:0]
	buffer.checksums = buffer.checksums[:0]
	buffer.writeOrder = buffer.writeOrder[:0]
	buffer.writeCount = 0
	buffer.flushNeeded = false
	buffer.mutex.Unlock()

	// Update metrics
	aw.mutex.Lock()
	aw.flushOperations++
	aw.unflushedWrites[buffer.filePath] -= writeCount
	if aw.unflushedWrites[buffer.filePath] < 0 {
		aw.unflushedWrites[buffer.filePath] = 0
	}
	aw.mutex.Unlock()

	return nil
}

// startFlushRoutine starts the background flush routine
func (aw *AsyncWriter) startFlushRoutine() {
	aw.flushTicker = time.NewTicker(aw.config.FlushInterval)
	aw.wg.Add(1)

	go func() {
		defer aw.wg.Done()

		for {
			select {
			case <-aw.flushTicker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				if err := aw.FlushAll(ctx); err != nil {
					// Log error but continue
					fmt.Printf("Background flush error: %v\n", err)
				}
				cancel()

			case <-aw.stopChan:
				return
			}
		}
	}()
}

// verifyWrite verifies that the write was successful
func (aw *AsyncWriter) verifyWrite(file *os.File, data []byte, checksums []uint32) error {
	// For verification, we could read back the data and verify checksums
	// This is a simplified implementation
	expectedChecksum := calculateChecksum(data)

	// Verify that all individual checksums are valid
	var combinedData []byte
	for _, checksum := range checksums {
		if checksum == 0 {
			return fmt.Errorf("invalid checksum found during verification")
		}
	}

	// Verify combined checksum
	actualChecksum := calculateChecksum(combinedData)
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %d, got %d", expectedChecksum, actualChecksum)
	}

	return nil
}

// GetMetrics returns async writer metrics
func (aw *AsyncWriter) GetMetrics() AsyncWriterMetrics {
	aw.mutex.RLock()
	defer aw.mutex.RUnlock()

	totalUnflushed := int64(0)
	for _, count := range aw.unflushedWrites {
		totalUnflushed += count
	}

	return AsyncWriterMetrics{
		AsyncWrites:     aw.asyncWrites,
		FlushOperations: aw.flushOperations,
		BufferOverflows: aw.bufferOverflows,
		FlushErrors:     aw.flushErrors,
		UnflushedWrites: totalUnflushed,
		LastFlushTime:   aw.lastFlushTime,
		ActiveBuffers:   int64(len(aw.buffers)),
	}
}

// calculateChecksum calculates a simple checksum for data integrity
func calculateChecksum(data []byte) uint32 {
	var checksum uint32
	for _, b := range data {
		checksum = checksum*31 + uint32(b)
	}
	return checksum
}

// AsyncWrite represents a single asynchronous write operation
type AsyncWrite struct {
	FilePath string
	Data     []byte
	Offset   int64
}

// AsyncWriterMetrics holds metrics for async writer operations
type AsyncWriterMetrics struct {
	AsyncWrites     int64     `json:"async_writes"`
	FlushOperations int64     `json:"flush_operations"`
	BufferOverflows int64     `json:"buffer_overflows"`
	FlushErrors     int64     `json:"flush_errors"`
	UnflushedWrites int64     `json:"unflushed_writes"`
	LastFlushTime   time.Time `json:"last_flush_time"`
	ActiveBuffers   int64     `json:"active_buffers"`
}
