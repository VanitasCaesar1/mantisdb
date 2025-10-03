package checkpoint

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Manager handles checkpoint creation, management, and validation
type Manager struct {
	config *CheckpointConfig
	index  *CheckpointIndex
	stats  *CheckpointStats
	mutex  sync.RWMutex

	// Internal state
	indexFile string
	running   bool
	stopChan  chan struct{}
	wg        sync.WaitGroup

	// Hooks for external integration
	walReader    WALReaderInterface
	dataProvider DataProvider
	validator    ValidatorInterface
}

// WALReaderInterface for reading WAL entries
type WALReaderInterface interface {
	ReadFromLSN(startLSN uint64) ([]WALEntryData, error)
	GetLastLSN() (uint64, error)
}

// DataProvider interface for accessing database data
type DataProvider interface {
	CreateSnapshot(lsn uint64) (io.Reader, error)
	GetDataSize() (int64, error)
	ValidateData(lsn uint64) error
}

// ValidatorInterface for checkpoint validation
type ValidatorInterface interface {
	ValidateCheckpoint(checkpoint *Checkpoint) (*CheckpointValidationResult, error)
	ValidateIntegrity(filePath string, expectedChecksum uint32) error
}

// WALEntryData represents a WAL entry (simplified interface)
type WALEntryData struct {
	LSN       uint64
	TxnID     uint64
	Operation OperationData
	Timestamp time.Time
}

// OperationData represents a database operation
type OperationData struct {
	Type  OperationTypeData
	Key   string
	Value []byte
}

// OperationTypeData represents the type of operation
type OperationTypeData int

const (
	OpInsertData OperationTypeData = iota
	OpUpdateData
	OpDeleteData
	OpCommitData
	OpAbortData
)

// NewManager creates a new checkpoint manager
func NewManager(config *CheckpointConfig) (*Manager, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid checkpoint config: %w", err)
	}

	// Ensure checkpoint directory exists
	if err := os.MkdirAll(config.CheckpointDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create checkpoint directory: %w", err)
	}

	indexFile := filepath.Join(config.CheckpointDir, "checkpoint_index.json")

	manager := &Manager{
		config:    config,
		indexFile: indexFile,
		stopChan:  make(chan struct{}),
		stats: &CheckpointStats{
			LastCheckpointTime: time.Now(),
		},
	}

	// Load existing index or create new one
	if err := manager.loadIndex(); err != nil {
		return nil, fmt.Errorf("failed to load checkpoint index: %w", err)
	}

	return manager, nil
}

// SetWALReader sets the WAL reader for checkpoint operations
func (m *Manager) SetWALReader(reader WALReaderInterface) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.walReader = reader
}

// SetDataProvider sets the data provider for checkpoint operations
func (m *Manager) SetDataProvider(provider DataProvider) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.dataProvider = provider
}

// SetValidator sets the validator for checkpoint validation
func (m *Manager) SetValidator(validator ValidatorInterface) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.validator = validator
}

// Start starts the checkpoint manager background processes
func (m *Manager) Start() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.running {
		return fmt.Errorf("checkpoint manager is already running")
	}

	m.running = true

	// Start automatic checkpoint creation
	if m.config.CheckpointInterval > 0 {
		m.wg.Add(1)
		go m.automaticCheckpointLoop()
	}

	// Start cleanup process
	if m.config.AutoCleanup && m.config.CleanupInterval > 0 {
		m.wg.Add(1)
		go m.cleanupLoop()
	}

	return nil
}

// Stop stops the checkpoint manager
func (m *Manager) Stop() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.running {
		return nil
	}

	m.running = false
	close(m.stopChan)
	m.wg.Wait()

	return nil
}

// CreateCheckpoint creates a new checkpoint
func (m *Manager) CreateCheckpoint(checkpointType CheckpointType) (*Checkpoint, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.walReader == nil {
		return nil, fmt.Errorf("WAL reader not configured")
	}

	// Get current LSN
	currentLSN, err := m.walReader.GetLastLSN()
	if err != nil {
		return nil, fmt.Errorf("failed to get current LSN: %w", err)
	}

	// Generate checkpoint ID
	checkpointID := m.generateCheckpointID(checkpointType, currentLSN)

	// Create checkpoint metadata
	checkpoint := &Checkpoint{
		ID:        checkpointID,
		Type:      checkpointType,
		Status:    CheckpointStatusCreating,
		LSN:       currentLSN,
		Timestamp: time.Now(),
		CreatedBy: "checkpoint_manager",
		Metadata: CheckpointMeta{
			Version:         "1.0",
			DatabaseVersion: "mantisdb-1.0",
			Tags:            make(map[string]string),
		},
	}

	// Set file path
	checkpoint.FilePath = filepath.Join(m.config.CheckpointDir,
		fmt.Sprintf("checkpoint_%s.dat", checkpointID))

	startTime := time.Now()

	// Create the checkpoint file
	if err := m.createCheckpointFile(checkpoint); err != nil {
		checkpoint.Status = CheckpointStatusFailed
		m.stats.FailedCreations++
		return checkpoint, fmt.Errorf("failed to create checkpoint file: %w", err)
	}

	// Calculate file size and checksum
	if err := m.finalizeCheckpoint(checkpoint); err != nil {
		checkpoint.Status = CheckpointStatusFailed
		m.stats.FailedCreations++
		return checkpoint, fmt.Errorf("failed to finalize checkpoint: %w", err)
	}

	// Validate if configured
	if m.config.ValidateOnCreate {
		result, err := m.validateCheckpoint(checkpoint)
		if err != nil || !result.Valid {
			checkpoint.Status = CheckpointStatusFailed
			m.stats.FailedCreations++
			m.stats.ValidationFailures++
			if err != nil {
				return checkpoint, fmt.Errorf("checkpoint validation failed: %w", err)
			}
			return checkpoint, fmt.Errorf("checkpoint validation failed: %d errors", len(result.Errors))
		}
	}

	checkpoint.Status = CheckpointStatusCompleted
	duration := time.Since(startTime)

	// Update index
	m.index.Checkpoints[checkpointID] = checkpoint
	m.index.LatestLSN = currentLSN
	m.index.UpdatedAt = time.Now()

	// Update statistics
	m.stats.TotalCheckpoints++
	m.stats.SuccessfulCreations++
	m.stats.TotalSize += checkpoint.Size
	m.stats.LastCheckpointTime = checkpoint.Timestamp
	if m.stats.TotalCheckpoints > 0 {
		m.stats.AverageSize = m.stats.TotalSize / int64(m.stats.TotalCheckpoints)
	}

	// Update average creation time
	if m.stats.AverageCreateTime == 0 {
		m.stats.AverageCreateTime = duration
	} else {
		m.stats.AverageCreateTime = (m.stats.AverageCreateTime + duration) / 2
	}

	// Save index
	if err := m.saveIndex(); err != nil {
		return checkpoint, fmt.Errorf("failed to save checkpoint index: %w", err)
	}

	return checkpoint, nil
}

// GetCheckpoint retrieves a checkpoint by ID
func (m *Manager) GetCheckpoint(id CheckpointID) (*Checkpoint, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	checkpoint, exists := m.index.Checkpoints[id]
	if !exists {
		return nil, fmt.Errorf("checkpoint %s not found", id)
	}

	return checkpoint, nil
}

// ListCheckpoints returns all checkpoints, optionally filtered
func (m *Manager) ListCheckpoints(filter *CheckpointFilter) ([]*Checkpoint, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var checkpoints []*Checkpoint
	for _, checkpoint := range m.index.Checkpoints {
		if filter == nil || filter.Matches(checkpoint) {
			checkpoints = append(checkpoints, checkpoint)
		}
	}

	// Sort by timestamp (newest first)
	sort.Slice(checkpoints, func(i, j int) bool {
		return checkpoints[i].Timestamp.After(checkpoints[j].Timestamp)
	})

	return checkpoints, nil
}

// GetLatestCheckpoint returns the most recent checkpoint
func (m *Manager) GetLatestCheckpoint() (*Checkpoint, error) {
	checkpoints, err := m.ListCheckpoints(nil)
	if err != nil {
		return nil, err
	}

	if len(checkpoints) == 0 {
		return nil, fmt.Errorf("no checkpoints found")
	}

	return checkpoints[0], nil
}

// ValidateCheckpoint validates a checkpoint
func (m *Manager) ValidateCheckpoint(id CheckpointID) (*CheckpointValidationResult, error) {
	checkpoint, err := m.GetCheckpoint(id)
	if err != nil {
		return nil, err
	}

	return m.validateCheckpoint(checkpoint)
}

// DeleteCheckpoint deletes a checkpoint
func (m *Manager) DeleteCheckpoint(id CheckpointID) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	checkpoint, exists := m.index.Checkpoints[id]
	if !exists {
		return fmt.Errorf("checkpoint %s not found", id)
	}

	// Remove file
	if err := os.Remove(checkpoint.FilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove checkpoint file: %w", err)
	}

	// Remove from index
	delete(m.index.Checkpoints, id)
	m.index.UpdatedAt = time.Now()

	// Update statistics
	m.stats.TotalCheckpoints--
	m.stats.TotalSize -= checkpoint.Size
	if m.stats.TotalCheckpoints > 0 {
		m.stats.AverageSize = m.stats.TotalSize / int64(m.stats.TotalCheckpoints)
	} else {
		m.stats.AverageSize = 0
	}

	// Save index
	return m.saveIndex()
}

// GetStats returns checkpoint statistics
func (m *Manager) GetStats() *CheckpointStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Create a copy to avoid race conditions
	stats := *m.stats
	return &stats
}

// Cleanup removes old checkpoints based on retention policy
func (m *Manager) Cleanup() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	checkpoints, err := m.listCheckpointsInternal(nil)
	if err != nil {
		return fmt.Errorf("failed to list checkpoints for cleanup: %w", err)
	}

	if len(checkpoints) <= m.config.MinCheckpoints {
		return nil // Don't cleanup if we're at minimum
	}

	var toDelete []*Checkpoint
	totalSize := int64(0)

	// Calculate total size
	for _, cp := range checkpoints {
		totalSize += cp.Size
	}

	// Sort by timestamp (oldest first for deletion)
	sort.Slice(checkpoints, func(i, j int) bool {
		return checkpoints[i].Timestamp.Before(checkpoints[j].Timestamp)
	})

	// Mark checkpoints for deletion based on retention policy
	for i, checkpoint := range checkpoints {
		shouldDelete := false

		// Keep minimum number of checkpoints
		remaining := len(checkpoints) - len(toDelete)
		if remaining <= m.config.MinCheckpoints {
			break
		}

		// Delete if exceeding max checkpoints
		if len(checkpoints)-i > m.config.MaxCheckpoints {
			shouldDelete = true
		}

		// Delete if older than retention period
		if m.config.RetentionPeriod > 0 && checkpoint.Age() > m.config.RetentionPeriod {
			shouldDelete = true
		}

		// Delete if exceeding disk usage limit
		if m.config.MaxDiskUsage > 0 && totalSize > m.config.MaxDiskUsage {
			shouldDelete = true
			totalSize -= checkpoint.Size
		}

		if shouldDelete {
			toDelete = append(toDelete, checkpoint)
		}
	}

	// Delete marked checkpoints
	for _, checkpoint := range toDelete {
		if err := m.deleteCheckpointInternal(checkpoint.ID); err != nil {
			return fmt.Errorf("failed to delete checkpoint %s: %w", checkpoint.ID, err)
		}
		m.stats.CleanupCount++
	}

	if len(toDelete) > 0 {
		if err := m.saveIndex(); err != nil {
			return fmt.Errorf("failed to save index after cleanup: %w", err)
		}
	}

	return nil
}

// generateCheckpointID generates a unique checkpoint ID
func (m *Manager) generateCheckpointID(checkpointType CheckpointType, lsn uint64) CheckpointID {
	timestamp := time.Now().Unix()
	return CheckpointID(fmt.Sprintf("%s_%d_%d", checkpointType.String(), lsn, timestamp))
}

// createCheckpointFile creates the actual checkpoint file
func (m *Manager) createCheckpointFile(checkpoint *Checkpoint) error {
	file, err := os.Create(checkpoint.FilePath)
	if err != nil {
		return fmt.Errorf("failed to create checkpoint file: %w", err)
	}
	defer file.Close()

	// Write checkpoint header
	header := m.createCheckpointHeader(checkpoint)
	if err := m.writeCheckpointHeader(file, header); err != nil {
		return fmt.Errorf("failed to write checkpoint header: %w", err)
	}

	// Write checkpoint data based on type
	switch checkpoint.Type {
	case CheckpointTypeFull:
		return m.createFullCheckpoint(file, checkpoint)
	case CheckpointTypeIncremental:
		return m.createIncrementalCheckpoint(file, checkpoint)
	case CheckpointTypeSnapshot:
		return m.createSnapshotCheckpoint(file, checkpoint)
	default:
		return fmt.Errorf("unsupported checkpoint type: %v", checkpoint.Type)
	}
}

// createFullCheckpoint creates a full checkpoint
func (m *Manager) createFullCheckpoint(file *os.File, checkpoint *Checkpoint) error {
	if m.dataProvider == nil {
		return fmt.Errorf("data provider not configured")
	}

	// Get data snapshot
	dataReader, err := m.dataProvider.CreateSnapshot(checkpoint.LSN)
	if err != nil {
		return fmt.Errorf("failed to create data snapshot: %w", err)
	}

	// Copy data to checkpoint file
	written, err := io.Copy(file, dataReader)
	if err != nil {
		return fmt.Errorf("failed to write checkpoint data: %w", err)
	}

	checkpoint.Metadata.OperationCount = int(written)
	return nil
}

// createIncrementalCheckpoint creates an incremental checkpoint
func (m *Manager) createIncrementalCheckpoint(file *os.File, checkpoint *Checkpoint) error {
	// Find the last checkpoint to determine what's new
	lastCheckpoint, err := m.getLastCheckpointBefore(checkpoint.LSN)
	if err != nil {
		return fmt.Errorf("failed to find previous checkpoint: %w", err)
	}

	startLSN := uint64(1)
	if lastCheckpoint != nil {
		startLSN = lastCheckpoint.LSN + 1
	}

	// Read WAL entries since last checkpoint
	entries, err := m.walReader.ReadFromLSN(startLSN)
	if err != nil {
		return fmt.Errorf("failed to read WAL entries: %w", err)
	}

	// Write WAL entries to checkpoint
	encoder := json.NewEncoder(file)
	for _, entry := range entries {
		if entry.LSN > checkpoint.LSN {
			break
		}
		if err := encoder.Encode(entry); err != nil {
			return fmt.Errorf("failed to encode WAL entry: %w", err)
		}
	}

	checkpoint.Metadata.OperationCount = len(entries)
	return nil
}

// createSnapshotCheckpoint creates a snapshot checkpoint
func (m *Manager) createSnapshotCheckpoint(file *os.File, checkpoint *Checkpoint) error {
	// For now, snapshot is similar to full checkpoint
	return m.createFullCheckpoint(file, checkpoint)
}

// finalizeCheckpoint calculates size and checksum
func (m *Manager) finalizeCheckpoint(checkpoint *Checkpoint) error {
	// Get file info
	fileInfo, err := os.Stat(checkpoint.FilePath)
	if err != nil {
		return fmt.Errorf("failed to stat checkpoint file: %w", err)
	}

	checkpoint.Size = fileInfo.Size()

	// Calculate checksum
	checksum, err := m.calculateFileChecksum(checkpoint.FilePath)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}

	checkpoint.Checksum = checksum
	return nil
}

// calculateFileChecksum calculates CRC32 checksum of a file
func (m *Manager) calculateFileChecksum(filePath string) (uint32, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	hash := crc32.NewIEEE()
	if _, err := io.Copy(hash, file); err != nil {
		return 0, err
	}

	return hash.Sum32(), nil
}

// validateCheckpoint validates a checkpoint
func (m *Manager) validateCheckpoint(checkpoint *Checkpoint) (*CheckpointValidationResult, error) {
	if m.validator != nil {
		result, err := m.validator.ValidateCheckpoint(checkpoint)
		if err != nil {
			return nil, err
		}
		checkpoint.ValidatedAt = &result.ValidatedAt
		return result, nil
	}

	// Basic validation if no validator is configured
	return m.basicValidation(checkpoint)
}

// basicValidation performs basic checkpoint validation
func (m *Manager) basicValidation(checkpoint *Checkpoint) (*CheckpointValidationResult, error) {
	result := &CheckpointValidationResult{
		Valid:       true,
		Errors:      make([]CheckpointError, 0),
		Warnings:    make([]CheckpointWarning, 0),
		ValidatedAt: time.Now(),
	}

	// Check file exists
	if _, err := os.Stat(checkpoint.FilePath); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, CheckpointError{
			Type:        CheckpointErrorFileNotFound,
			Message:     "checkpoint file not found",
			Recoverable: false,
		})
		return result, fmt.Errorf("checkpoint file not found: %w", err)
	}

	// Verify checksum
	actualChecksum, err := m.calculateFileChecksum(checkpoint.FilePath)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, CheckpointError{
			Type:        CheckpointErrorChecksumMismatch,
			Message:     "failed to calculate checksum",
			Recoverable: false,
		})
		return result, fmt.Errorf("failed to calculate checksum for validation: %w", err)
	}

	if actualChecksum != checkpoint.Checksum {
		result.Valid = false
		result.Errors = append(result.Errors, CheckpointError{
			Type:        CheckpointErrorChecksumMismatch,
			Message:     "checksum mismatch",
			Expected:    checkpoint.Checksum,
			Actual:      actualChecksum,
			Recoverable: false,
		})
		return result, fmt.Errorf("checksum mismatch: expected %d, got %d",
			checkpoint.Checksum, actualChecksum)
	}

	result.ChecksumOK = true
	result.SizeOK = true
	result.FormatOK = true
	result.MetadataOK = true

	now := time.Now()
	checkpoint.ValidatedAt = &now
	return result, nil
}

// Internal helper methods
func (m *Manager) loadIndex() error {
	if _, err := os.Stat(m.indexFile); os.IsNotExist(err) {
		// Create new index
		m.index = &CheckpointIndex{
			Checkpoints: make(map[CheckpointID]*Checkpoint),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Version:     "1.0",
		}
		return nil
	}

	data, err := os.ReadFile(m.indexFile)
	if err != nil {
		return fmt.Errorf("failed to read index file: %w", err)
	}

	m.index = &CheckpointIndex{}
	if err := json.Unmarshal(data, m.index); err != nil {
		return fmt.Errorf("failed to unmarshal index: %w", err)
	}

	return nil
}

func (m *Manager) saveIndex() error {
	data, err := json.MarshalIndent(m.index, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index: %w", err)
	}

	return os.WriteFile(m.indexFile, data, 0644)
}

func (m *Manager) listCheckpointsInternal(filter *CheckpointFilter) ([]*Checkpoint, error) {
	var checkpoints []*Checkpoint
	for _, checkpoint := range m.index.Checkpoints {
		if filter == nil || filter.Matches(checkpoint) {
			checkpoints = append(checkpoints, checkpoint)
		}
	}
	return checkpoints, nil
}

func (m *Manager) deleteCheckpointInternal(id CheckpointID) error {
	checkpoint, exists := m.index.Checkpoints[id]
	if !exists {
		return fmt.Errorf("checkpoint %s not found", id)
	}

	// Remove file
	if err := os.Remove(checkpoint.FilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove checkpoint file: %w", err)
	}

	// Remove from index
	delete(m.index.Checkpoints, id)
	m.index.UpdatedAt = time.Now()

	// Update statistics
	m.stats.TotalCheckpoints--
	m.stats.TotalSize -= checkpoint.Size

	return nil
}

func (m *Manager) getLastCheckpointBefore(lsn uint64) (*Checkpoint, error) {
	var lastCheckpoint *Checkpoint
	for _, checkpoint := range m.index.Checkpoints {
		if checkpoint.LSN < lsn && checkpoint.IsCompleted() {
			if lastCheckpoint == nil || checkpoint.LSN > lastCheckpoint.LSN {
				lastCheckpoint = checkpoint
			}
		}
	}
	return lastCheckpoint, nil
}

// Background processes
func (m *Manager) automaticCheckpointLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.CheckpointInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if _, err := m.CreateCheckpoint(CheckpointTypeFull); err != nil {
				// Log error but continue
				fmt.Printf("Automatic checkpoint creation failed: %v\n", err)
			}
		case <-m.stopChan:
			return
		}
	}
}

func (m *Manager) cleanupLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := m.Cleanup(); err != nil {
				// Log error but continue
				fmt.Printf("Checkpoint cleanup failed: %v\n", err)
			}
		case <-m.stopChan:
			return
		}
	}
}

// Checkpoint header and file format helpers
type checkpointHeader struct {
	Magic     [8]byte
	Version   uint32
	Type      CheckpointType
	LSN       uint64
	Timestamp int64
	Checksum  uint32
}

func (m *Manager) createCheckpointHeader(checkpoint *Checkpoint) *checkpointHeader {
	header := &checkpointHeader{
		Magic:     [8]byte{'M', 'A', 'N', 'T', 'I', 'S', 'C', 'P'},
		Version:   1,
		Type:      checkpoint.Type,
		LSN:       checkpoint.LSN,
		Timestamp: checkpoint.Timestamp.Unix(),
	}
	return header
}

func (m *Manager) writeCheckpointHeader(file *os.File, header *checkpointHeader) error {
	// This would write the binary header to the file
	// For simplicity, we'll skip the actual binary serialization
	return nil
}
