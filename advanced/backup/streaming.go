package backup

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// BackupStreamer handles streaming backup data to various destinations
type BackupStreamer struct {
	mu sync.RWMutex

	// Configuration
	config *StreamingConfig

	// Active streams
	activeStreams map[string]*BackupStream
}

// StreamingConfig holds configuration for backup streaming
type StreamingConfig struct {
	BufferSize      int           // Buffer size for streaming
	CompressionType string        // Compression type (none, gzip, lz4)
	VerifyChecksum  bool          // Whether to verify checksums during streaming
	MaxConcurrent   int           // Maximum concurrent streams
	Timeout         time.Duration // Timeout for streaming operations
	RetryAttempts   int           // Number of retry attempts for failed streams
	RetryDelay      time.Duration // Delay between retry attempts
}

// BackupStream represents an active backup stream
type BackupStream struct {
	ID          string                 `json:"id"`
	SnapshotID  string                 `json:"snapshot_id"`
	Destination BackupDestination      `json:"destination"`
	Status      string                 `json:"status"`
	Progress    StreamProgress         `json:"progress"`
	StartTime   time.Time              `json:"start_time"`
	EndTime     *time.Time             `json:"end_time,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Checksum    string                 `json:"checksum"`
	Metadata    map[string]interface{} `json:"metadata"`

	// Internal state
	writer     io.WriteCloser
	hasher     hash.Hash
	compressor io.WriteCloser
	cancel     context.CancelFunc
	mu         sync.RWMutex
}

// BackupDestination represents where backup data should be streamed
type BackupDestination struct {
	Type     string            `json:"type"`     // file, s3, gcs, azure, etc.
	Location string            `json:"location"` // path, URL, bucket name, etc.
	Options  map[string]string `json:"options"`  // additional options
}

// StreamProgress tracks the progress of a backup stream
type StreamProgress struct {
	BytesWritten    int64      `json:"bytes_written"`
	TotalBytes      int64      `json:"total_bytes"`
	PercentComplete float64    `json:"percent_complete"`
	EstimatedETA    *time.Time `json:"estimated_eta,omitempty"`
	TransferRate    float64    `json:"transfer_rate_mbps"`
}

// BackupVerifier handles backup integrity verification
type BackupVerifier struct {
	config *VerificationConfig
}

// VerificationConfig holds configuration for backup verification
type VerificationConfig struct {
	VerifyChecksums    bool          // Whether to verify checksums
	VerifyCompression  bool          // Whether to verify compression integrity
	VerifyStructure    bool          // Whether to verify backup structure
	SampleVerification bool          // Whether to do sampling verification
	SampleRate         float64       // Rate of sampling (0.0 to 1.0)
	Timeout            time.Duration // Timeout for verification operations
}

// VerificationResult contains the results of backup verification
type VerificationResult struct {
	Valid            bool                   `json:"valid"`
	ChecksumValid    bool                   `json:"checksum_valid"`
	StructureValid   bool                   `json:"structure_valid"`
	CompressionOK    bool                   `json:"compression_ok"`
	Errors           []string               `json:"errors"`
	Warnings         []string               `json:"warnings"`
	VerifiedAt       time.Time              `json:"verified_at"`
	VerificationTime time.Duration          `json:"verification_time"`
	Metadata         map[string]interface{} `json:"metadata"`
}

// NewBackupStreamer creates a new backup streamer
func NewBackupStreamer(config *StreamingConfig) *BackupStreamer {
	if config == nil {
		config = DefaultStreamingConfig()
	}

	return &BackupStreamer{
		config:        config,
		activeStreams: make(map[string]*BackupStream),
	}
}

// DefaultStreamingConfig returns default streaming configuration
func DefaultStreamingConfig() *StreamingConfig {
	return &StreamingConfig{
		BufferSize:      1024 * 1024, // 1MB
		CompressionType: "gzip",
		VerifyChecksum:  true,
		MaxConcurrent:   5,
		Timeout:         2 * time.Hour,
		RetryAttempts:   3,
		RetryDelay:      30 * time.Second,
	}
}

// StreamBackup streams a backup to the specified destination
func (bs *BackupStreamer) StreamBackup(ctx context.Context, snapshotID string,
	destination BackupDestination) (*BackupStream, error) {

	bs.mu.Lock()
	defer bs.mu.Unlock()

	// Check concurrent stream limit
	if len(bs.activeStreams) >= bs.config.MaxConcurrent {
		return nil, fmt.Errorf("maximum concurrent streams (%d) reached", bs.config.MaxConcurrent)
	}

	// Create stream ID
	streamID := fmt.Sprintf("stream_%s_%d", snapshotID, time.Now().Unix())

	// Create backup stream
	stream := &BackupStream{
		ID:          streamID,
		SnapshotID:  snapshotID,
		Destination: destination,
		Status:      "initializing",
		StartTime:   time.Now(),
		Metadata:    make(map[string]interface{}),
		hasher:      sha256.New(),
	}

	// Add to active streams
	bs.activeStreams[streamID] = stream

	// Start streaming asynchronously
	streamCtx, cancel := context.WithTimeout(ctx, bs.config.Timeout)
	stream.cancel = cancel

	go bs.streamBackupAsync(streamCtx, stream)

	return stream, nil
}

// streamBackupAsync performs the actual backup streaming
func (bs *BackupStreamer) streamBackupAsync(ctx context.Context, stream *BackupStream) {
	defer func() {
		bs.mu.Lock()
		delete(bs.activeStreams, stream.ID)
		bs.mu.Unlock()

		if stream.cancel != nil {
			stream.cancel()
		}
	}()

	// Initialize destination writer
	if err := bs.initializeDestination(ctx, stream); err != nil {
		bs.markStreamFailed(stream, fmt.Errorf("failed to initialize destination: %w", err))
		return
	}
	defer bs.closeDestination(stream)

	// Stream the backup data
	if err := bs.streamData(ctx, stream); err != nil {
		bs.markStreamFailed(stream, fmt.Errorf("failed to stream data: %w", err))
		return
	}

	// Verify if configured
	if bs.config.VerifyChecksum {
		if err := bs.verifyStream(ctx, stream); err != nil {
			bs.markStreamFailed(stream, fmt.Errorf("stream verification failed: %w", err))
			return
		}
	}

	// Finalize stream
	bs.finalizeStream(stream)
}

// initializeDestination sets up the destination writer
func (bs *BackupStreamer) initializeDestination(ctx context.Context, stream *BackupStream) error {
	var writer io.WriteCloser
	var err error

	switch stream.Destination.Type {
	case "file":
		writer, err = bs.createFileWriter(stream.Destination.Location)
	case "s3":
		writer, err = bs.createS3Writer(ctx, stream.Destination)
	case "gcs":
		writer, err = bs.createGCSWriter(ctx, stream.Destination)
	case "azure":
		writer, err = bs.createAzureWriter(ctx, stream.Destination)
	default:
		return fmt.Errorf("unsupported destination type: %s", stream.Destination.Type)
	}

	if err != nil {
		return err
	}

	// Set up compression if configured
	if bs.config.CompressionType != "none" {
		compressor, err := bs.createCompressor(writer, bs.config.CompressionType)
		if err != nil {
			writer.Close()
			return fmt.Errorf("failed to create compressor: %w", err)
		}
		stream.compressor = compressor
		stream.writer = compressor
	} else {
		stream.writer = writer
	}

	stream.Status = "streaming"
	return nil
}

// createFileWriter creates a file writer for local file destinations
func (bs *BackupStreamer) createFileWriter(location string) (io.WriteCloser, error) {
	// Ensure directory exists
	dir := filepath.Dir(location)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.Create(location)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	return file, nil
}

// createS3Writer creates an S3 writer (placeholder implementation)
func (bs *BackupStreamer) createS3Writer(ctx context.Context, dest BackupDestination) (io.WriteCloser, error) {
	// This would integrate with AWS S3 SDK
	// For now, return a placeholder
	return &noopWriteCloser{}, nil
}

// createGCSWriter creates a Google Cloud Storage writer (placeholder implementation)
func (bs *BackupStreamer) createGCSWriter(ctx context.Context, dest BackupDestination) (io.WriteCloser, error) {
	// This would integrate with Google Cloud Storage SDK
	// For now, return a placeholder
	return &noopWriteCloser{}, nil
}

// createAzureWriter creates an Azure Blob Storage writer (placeholder implementation)
func (bs *BackupStreamer) createAzureWriter(ctx context.Context, dest BackupDestination) (io.WriteCloser, error) {
	// This would integrate with Azure SDK
	// For now, return a placeholder
	return &noopWriteCloser{}, nil
}

// createCompressor creates a compressor based on the specified type
func (bs *BackupStreamer) createCompressor(writer io.WriteCloser, compressionType string) (io.WriteCloser, error) {
	switch compressionType {
	case "gzip":
		return gzip.NewWriter(writer), nil
	case "lz4":
		// This would use LZ4 compression library
		// For now, return the original writer
		return writer, nil
	default:
		return nil, fmt.Errorf("unsupported compression type: %s", compressionType)
	}
}

// streamData streams the actual backup data
func (bs *BackupStreamer) streamData(ctx context.Context, stream *BackupStream) error {
	// Open the snapshot file
	snapshotPath := filepath.Join("data/snapshots", stream.SnapshotID+".snap")
	file, err := os.Open(snapshotPath)
	if err != nil {
		return fmt.Errorf("failed to open snapshot file: %w", err)
	}
	defer file.Close()

	// Get file size for progress tracking
	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file stats: %w", err)
	}
	stream.Progress.TotalBytes = stat.Size()

	// Create multi-writer to write to destination and calculate checksum
	multiWriter := io.MultiWriter(stream.writer, stream.hasher)

	// Stream data with progress tracking
	buffer := make([]byte, bs.config.BufferSize)
	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to read from snapshot: %w", err)
		}

		if n > 0 {
			if _, writeErr := multiWriter.Write(buffer[:n]); writeErr != nil {
				return fmt.Errorf("failed to write to destination: %w", writeErr)
			}

			// Update progress
			stream.mu.Lock()
			stream.Progress.BytesWritten += int64(n)
			stream.Progress.PercentComplete = float64(stream.Progress.BytesWritten) / float64(stream.Progress.TotalBytes) * 100

			// Calculate transfer rate
			elapsed := time.Since(startTime).Seconds()
			if elapsed > 0 {
				mbps := float64(stream.Progress.BytesWritten) / (1024 * 1024) / elapsed
				stream.Progress.TransferRate = mbps

				// Estimate ETA
				if stream.Progress.PercentComplete > 0 {
					totalTime := elapsed / (stream.Progress.PercentComplete / 100)
					eta := startTime.Add(time.Duration(totalTime) * time.Second)
					stream.Progress.EstimatedETA = &eta
				}
			}
			stream.mu.Unlock()
		}

		if err == io.EOF {
			break
		}
	}

	return nil
}

// verifyStream verifies the streamed backup
func (bs *BackupStreamer) verifyStream(ctx context.Context, stream *BackupStream) error {
	// Calculate final checksum
	stream.Checksum = hex.EncodeToString(stream.hasher.Sum(nil))

	// For file destinations, we can verify by reading back
	if stream.Destination.Type == "file" {
		return bs.verifyFileDestination(stream)
	}

	// For cloud destinations, verification would depend on the provider's capabilities
	return nil
}

// verifyFileDestination verifies a file destination
func (bs *BackupStreamer) verifyFileDestination(stream *BackupStream) error {
	file, err := os.Open(stream.Destination.Location)
	if err != nil {
		return fmt.Errorf("failed to open destination file for verification: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("failed to calculate destination checksum: %w", err)
	}

	destChecksum := hex.EncodeToString(hasher.Sum(nil))
	if destChecksum != stream.Checksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s",
			stream.Checksum, destChecksum)
	}

	return nil
}

// finalizeStream marks the stream as completed
func (bs *BackupStreamer) finalizeStream(stream *BackupStream) {
	now := time.Now()
	stream.mu.Lock()
	stream.Status = "completed"
	stream.EndTime = &now
	stream.mu.Unlock()
}

// markStreamFailed marks a stream as failed
func (bs *BackupStreamer) markStreamFailed(stream *BackupStream, err error) {
	now := time.Now()
	stream.mu.Lock()
	stream.Status = "failed"
	stream.Error = err.Error()
	stream.EndTime = &now
	stream.mu.Unlock()
}

// closeDestination closes the destination writer
func (bs *BackupStreamer) closeDestination(stream *BackupStream) {
	if stream.compressor != nil {
		stream.compressor.Close()
	}
	if stream.writer != nil {
		stream.writer.Close()
	}
}

// GetStream retrieves a stream by ID
func (bs *BackupStreamer) GetStream(id string) (*BackupStream, error) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	if stream, exists := bs.activeStreams[id]; exists {
		return stream, nil
	}

	return nil, fmt.Errorf("stream %s not found", id)
}

// ListStreams returns all active streams
func (bs *BackupStreamer) ListStreams() []*BackupStream {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	streams := make([]*BackupStream, 0, len(bs.activeStreams))
	for _, stream := range bs.activeStreams {
		streams = append(streams, stream)
	}

	return streams
}

// CancelStream cancels an active stream
func (bs *BackupStreamer) CancelStream(id string) error {
	bs.mu.RLock()
	stream, exists := bs.activeStreams[id]
	bs.mu.RUnlock()

	if !exists {
		return fmt.Errorf("stream %s not found", id)
	}

	if stream.cancel != nil {
		stream.cancel()
	}

	stream.mu.Lock()
	stream.Status = "cancelled"
	stream.mu.Unlock()

	return nil
}

// NewBackupVerifier creates a new backup verifier
func NewBackupVerifier(config *VerificationConfig) *BackupVerifier {
	if config == nil {
		config = DefaultVerificationConfig()
	}

	return &BackupVerifier{
		config: config,
	}
}

// DefaultVerificationConfig returns default verification configuration
func DefaultVerificationConfig() *VerificationConfig {
	return &VerificationConfig{
		VerifyChecksums:    true,
		VerifyCompression:  true,
		VerifyStructure:    true,
		SampleVerification: false,
		SampleRate:         0.1, // 10%
		Timeout:            30 * time.Minute,
	}
}

// VerifyBackup verifies the integrity of a backup
func (bv *BackupVerifier) VerifyBackup(ctx context.Context, backupPath string,
	expectedChecksum string) (*VerificationResult, error) {

	startTime := time.Now()
	result := &VerificationResult{
		Valid:          true,
		ChecksumValid:  true,
		StructureValid: true,
		CompressionOK:  true,
		Errors:         make([]string, 0),
		Warnings:       make([]string, 0),
		VerifiedAt:     startTime,
		Metadata:       make(map[string]interface{}),
	}

	// Set timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, bv.config.Timeout)
	defer cancel()

	// Verify checksum if configured
	if bv.config.VerifyChecksums {
		if err := bv.verifyChecksum(timeoutCtx, backupPath, expectedChecksum, result); err != nil {
			result.Valid = false
			result.ChecksumValid = false
			result.Errors = append(result.Errors, fmt.Sprintf("checksum verification failed: %v", err))
		}
	}

	// Verify structure if configured
	if bv.config.VerifyStructure {
		if err := bv.verifyStructure(timeoutCtx, backupPath, result); err != nil {
			result.Valid = false
			result.StructureValid = false
			result.Errors = append(result.Errors, fmt.Sprintf("structure verification failed: %v", err))
		}
	}

	// Verify compression if configured
	if bv.config.VerifyCompression {
		if err := bv.verifyCompression(timeoutCtx, backupPath, result); err != nil {
			result.CompressionOK = false
			result.Warnings = append(result.Warnings, fmt.Sprintf("compression verification warning: %v", err))
		}
	}

	result.VerificationTime = time.Since(startTime)
	return result, nil
}

// verifyChecksum verifies the backup checksum
func (bv *BackupVerifier) verifyChecksum(ctx context.Context, backupPath string,
	expectedChecksum string, result *VerificationResult) error {

	file, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}

	actualChecksum := hex.EncodeToString(hasher.Sum(nil))
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s",
			expectedChecksum, actualChecksum)
	}

	result.Metadata["actual_checksum"] = actualChecksum
	result.Metadata["expected_checksum"] = expectedChecksum
	return nil
}

// verifyStructure verifies the backup file structure
func (bv *BackupVerifier) verifyStructure(ctx context.Context, backupPath string,
	result *VerificationResult) error {

	file, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer file.Close()

	// Read and verify header
	header := make([]byte, 256)
	n, err := file.Read(header)
	if err != nil {
		return fmt.Errorf("failed to read header: %w", err)
	}

	headerStr := string(header[:n])
	if !contains(headerStr, "MANTIS_SNAPSHOT_V1") {
		return fmt.Errorf("invalid backup header")
	}

	result.Metadata["header_valid"] = true
	return nil
}

// verifyCompression verifies compression integrity
func (bv *BackupVerifier) verifyCompression(ctx context.Context, backupPath string,
	result *VerificationResult) error {

	// This would verify that compressed data can be decompressed successfully
	// For now, we'll just check if the file is readable
	file, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer file.Close()

	// Try to read a small portion to verify readability
	buffer := make([]byte, 1024)
	_, err = file.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read compressed data: %w", err)
	}

	return nil
}

// Helper types and functions

// noopWriteCloser is a no-op writer for placeholder implementations
type noopWriteCloser struct{}

func (nwc *noopWriteCloser) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func (nwc *noopWriteCloser) Close() error {
	return nil
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}
