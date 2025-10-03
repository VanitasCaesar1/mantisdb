package integrity

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"mantisDB/wal"
)

// WALIntegrityVerifier provides integrity verification for WAL entries and files
type WALIntegrityVerifier struct {
	engine          *ChecksumEngine
	detector        *CorruptionDetector
	config          *IntegrityConfig
	mutex           sync.RWMutex
	verificationLog map[string]*WALVerificationResult
	repairAttempts  map[string]int
}

// WALVerificationResult contains the result of WAL integrity verification
type WALVerificationResult struct {
	FilePath         string                 `json:"file_path"`
	TotalEntries     int64                  `json:"total_entries"`
	ValidEntries     int64                  `json:"valid_entries"`
	CorruptEntries   int64                  `json:"corrupt_entries"`
	CorruptionEvents []CorruptionEvent      `json:"corruption_events"`
	VerificationTime time.Time              `json:"verification_time"`
	Duration         time.Duration          `json:"duration"`
	Status           WALVerificationStatus  `json:"status"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// WALVerificationStatus represents the status of WAL verification
type WALVerificationStatus string

const (
	WALStatusHealthy   WALVerificationStatus = "healthy"
	WALStatusCorrupted WALVerificationStatus = "corrupted"
	WALStatusRepaired  WALVerificationStatus = "repaired"
	WALStatusFailed    WALVerificationStatus = "failed"
)

// WALRepairResult contains the result of WAL repair operations
type WALRepairResult struct {
	FilePath            string    `json:"file_path"`
	RepairedEntries     int64     `json:"repaired_entries"`
	UnrepairableEntries int64     `json:"unrepairable_entries"`
	BackupCreated       bool      `json:"backup_created"`
	BackupPath          string    `json:"backup_path,omitempty"`
	RepairTime          time.Time `json:"repair_time"`
	Success             bool      `json:"success"`
	Error               string    `json:"error,omitempty"`
}

// NewWALIntegrityVerifier creates a new WAL integrity verifier
func NewWALIntegrityVerifier(config *IntegrityConfig) *WALIntegrityVerifier {
	if config == nil {
		config = DefaultIntegrityConfig()
	}

	return &WALIntegrityVerifier{
		engine:          NewChecksumEngine(config.ChecksumAlgorithm),
		detector:        NewCorruptionDetector(config),
		config:          config,
		verificationLog: make(map[string]*WALVerificationResult),
		repairAttempts:  make(map[string]int),
	}
}

// VerifyWALEntry verifies the integrity of a single WAL entry
func (wiv *WALIntegrityVerifier) VerifyWALEntry(entry *wal.WALEntry) *CorruptionEvent {
	// Serialize the entry to get the data for verification
	data, err := entry.Serialize()
	if err != nil {
		return &CorruptionEvent{
			ID:          wiv.generateEventID(),
			Timestamp:   time.Now(),
			Location:    fmt.Sprintf("WAL_Entry_LSN_%d", entry.LSN),
			Type:        CorruptionTypeFormat,
			Severity:    SeverityHigh,
			Description: fmt.Sprintf("Failed to serialize WAL entry: %v", err),
			Size:        0,
			Metadata: map[string]interface{}{
				"lsn":    entry.LSN,
				"txn_id": entry.TxnID,
				"error":  err.Error(),
			},
		}
	}

	// Verify the entry's internal checksum
	if !wiv.verifyEntryChecksum(data, entry.Checksum) {
		return &CorruptionEvent{
			ID:          wiv.generateEventID(),
			Timestamp:   time.Now(),
			Location:    fmt.Sprintf("WAL_Entry_LSN_%d", entry.LSN),
			Type:        CorruptionTypeChecksum,
			Severity:    SeverityHigh,
			Description: "WAL entry checksum mismatch",
			Expected:    entry.Checksum,
			Actual:      wiv.engine.Calculate(data),
			Size:        int64(len(data)),
			Metadata: map[string]interface{}{
				"lsn":    entry.LSN,
				"txn_id": entry.TxnID,
			},
		}
	}

	return nil
}

// VerifyWALFile verifies the integrity of an entire WAL file
func (wiv *WALIntegrityVerifier) VerifyWALFile(filePath string) (*WALVerificationResult, error) {
	startTime := time.Now()

	result := &WALVerificationResult{
		FilePath:         filePath,
		VerificationTime: startTime,
		Status:           WALStatusHealthy,
		Metadata:         make(map[string]interface{}),
	}

	// Open and read the WAL file
	file, err := os.Open(filePath)
	if err != nil {
		result.Status = WALStatusFailed
		return result, fmt.Errorf("failed to open WAL file: %w", err)
	}
	defer file.Close()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		result.Status = WALStatusFailed
		return result, fmt.Errorf("failed to get file info: %w", err)
	}

	result.Metadata["file_size"] = fileInfo.Size()
	result.Metadata["mod_time"] = fileInfo.ModTime()

	// Read and verify each entry
	reader := &walFileReader{file: file}

	for {
		entry, entryData, err := reader.readNextEntry()
		if err != nil {
			if err.Error() == "EOF" {
				break // End of file reached
			}

			// Record corruption event
			corruptionEvent := &CorruptionEvent{
				ID:          wiv.generateEventID(),
				Timestamp:   time.Now(),
				Location:    fmt.Sprintf("%s:offset_%d", filePath, reader.offset),
				Type:        CorruptionTypeFormat,
				Severity:    SeverityHigh,
				Description: fmt.Sprintf("Failed to read WAL entry: %v", err),
				Size:        int64(len(entryData)),
				Metadata: map[string]interface{}{
					"file_offset": reader.offset,
					"error":       err.Error(),
				},
			}

			result.CorruptionEvents = append(result.CorruptionEvents, *corruptionEvent)
			result.CorruptEntries++
			result.Status = WALStatusCorrupted
			continue
		}

		result.TotalEntries++

		// Verify the entry
		if corruptionEvent := wiv.VerifyWALEntry(entry); corruptionEvent != nil {
			corruptionEvent.Location = fmt.Sprintf("%s:LSN_%d", filePath, entry.LSN)
			result.CorruptionEvents = append(result.CorruptionEvents, *corruptionEvent)
			result.CorruptEntries++
			result.Status = WALStatusCorrupted
		} else {
			result.ValidEntries++
		}
	}

	result.Duration = time.Since(startTime)

	// Store verification result
	wiv.mutex.Lock()
	wiv.verificationLog[filePath] = result
	wiv.mutex.Unlock()

	return result, nil
}

// VerifyWALDirectory verifies all WAL files in a directory
func (wiv *WALIntegrityVerifier) VerifyWALDirectory(walDir string) ([]*WALVerificationResult, error) {
	// Find all WAL files
	walFiles, err := filepath.Glob(filepath.Join(walDir, "wal-*.log"))
	if err != nil {
		return nil, fmt.Errorf("failed to find WAL files: %w", err)
	}

	var results []*WALVerificationResult
	var verificationErrors []error

	for _, filePath := range walFiles {
		result, err := wiv.VerifyWALFile(filePath)
		if err != nil {
			verificationErrors = append(verificationErrors, fmt.Errorf("failed to verify %s: %w", filePath, err))
			continue
		}
		results = append(results, result)
	}

	if len(verificationErrors) > 0 {
		return results, fmt.Errorf("verification completed with %d errors", len(verificationErrors))
	}

	return results, nil
}

// RepairWALFile attempts to repair a corrupted WAL file
func (wiv *WALIntegrityVerifier) RepairWALFile(filePath string) (*WALRepairResult, error) {
	wiv.mutex.Lock()
	attempts := wiv.repairAttempts[filePath]
	wiv.repairAttempts[filePath] = attempts + 1
	wiv.mutex.Unlock()

	result := &WALRepairResult{
		FilePath:   filePath,
		RepairTime: time.Now(),
	}

	// Create backup before attempting repair
	backupPath := fmt.Sprintf("%s.backup.%d", filePath, time.Now().Unix())
	if err := wiv.createBackup(filePath, backupPath); err != nil {
		result.Error = fmt.Sprintf("failed to create backup: %v", err)
		return result, err
	}

	result.BackupCreated = true
	result.BackupPath = backupPath

	// Attempt to repair by reading valid entries and rewriting the file
	validEntries, err := wiv.extractValidEntries(filePath)
	if err != nil {
		result.Error = fmt.Sprintf("failed to extract valid entries: %v", err)
		return result, err
	}

	// Rewrite the file with only valid entries
	if err := wiv.rewriteWALFile(filePath, validEntries); err != nil {
		result.Error = fmt.Sprintf("failed to rewrite WAL file: %v", err)
		return result, err
	}

	result.RepairedEntries = int64(len(validEntries))
	result.Success = true

	return result, nil
}

// GetVerificationHistory returns the verification history for a file
func (wiv *WALIntegrityVerifier) GetVerificationHistory(filePath string) *WALVerificationResult {
	wiv.mutex.RLock()
	defer wiv.mutex.RUnlock()

	if result, exists := wiv.verificationLog[filePath]; exists {
		// Return a copy to avoid race conditions
		resultCopy := *result
		return &resultCopy
	}

	return nil
}

// GetAllVerificationResults returns all verification results
func (wiv *WALIntegrityVerifier) GetAllVerificationResults() map[string]*WALVerificationResult {
	wiv.mutex.RLock()
	defer wiv.mutex.RUnlock()

	results := make(map[string]*WALVerificationResult)
	for path, result := range wiv.verificationLog {
		resultCopy := *result
		results[path] = &resultCopy
	}

	return results
}

// ClearVerificationHistory clears the verification history
func (wiv *WALIntegrityVerifier) ClearVerificationHistory() {
	wiv.mutex.Lock()
	defer wiv.mutex.Unlock()

	wiv.verificationLog = make(map[string]*WALVerificationResult)
	wiv.repairAttempts = make(map[string]int)
}

// Private helper methods

func (wiv *WALIntegrityVerifier) verifyEntryChecksum(data []byte, expectedChecksum uint32) bool {
	// The WAL entry serialization includes the checksum in the data
	// We need to verify using the same method as the WAL entry deserialization
	return wal.VerifyChecksum(data, expectedChecksum)
}

func (wiv *WALIntegrityVerifier) generateEventID() string {
	return fmt.Sprintf("wal_integrity_%d_%d", time.Now().UnixNano(), wiv.detector.stats.TotalEvents)
}

func (wiv *WALIntegrityVerifier) createBackup(sourcePath, backupPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	backupFile, err := os.Create(backupPath)
	if err != nil {
		return err
	}
	defer backupFile.Close()

	_, err = io.Copy(backupFile, sourceFile)
	return err
}

func (wiv *WALIntegrityVerifier) extractValidEntries(filePath string) ([]*wal.WALEntry, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var validEntries []*wal.WALEntry
	reader := &walFileReader{file: file}

	for {
		entry, _, err := reader.readNextEntry()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			continue // Skip corrupted entries
		}

		// Verify the entry
		if wiv.VerifyWALEntry(entry) == nil {
			validEntries = append(validEntries, entry)
		}
	}

	return validEntries, nil
}

func (wiv *WALIntegrityVerifier) rewriteWALFile(filePath string, entries []*wal.WALEntry) error {
	// Create temporary file
	tempPath := filePath + ".tmp"
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return err
	}
	defer tempFile.Close()

	// Write all valid entries to temporary file
	for _, entry := range entries {
		data, err := entry.Serialize()
		if err != nil {
			return fmt.Errorf("failed to serialize entry LSN %d: %w", entry.LSN, err)
		}

		if _, err := tempFile.Write(data); err != nil {
			return fmt.Errorf("failed to write entry LSN %d: %w", entry.LSN, err)
		}
	}

	// Sync and close temporary file
	if err := tempFile.Sync(); err != nil {
		return err
	}
	tempFile.Close()

	// Replace original file with temporary file
	return os.Rename(tempPath, filePath)
}

// walFileReader is a helper for reading WAL entries from a file
type walFileReader struct {
	file   *os.File
	offset int64
}

func (r *walFileReader) readNextEntry() (*wal.WALEntry, []byte, error) {
	// Read entry header first
	headerData := make([]byte, wal.WALEntryHeaderSize)
	n, err := r.file.Read(headerData)
	if err != nil {
		return nil, nil, err
	}
	if n < wal.WALEntryHeaderSize {
		return nil, nil, fmt.Errorf("EOF")
	}

	r.offset += int64(n)

	// Parse header to get payload length
	header, err := parseWALHeader(headerData)
	if err != nil {
		return nil, headerData, err
	}

	// Read payload
	payloadData := make([]byte, header.PayloadLen)
	if header.PayloadLen > 0 {
		n, err := r.file.Read(payloadData)
		if err != nil {
			return nil, append(headerData, payloadData[:n]...), err
		}
		if n < int(header.PayloadLen) {
			return nil, append(headerData, payloadData[:n]...), fmt.Errorf("incomplete payload")
		}
		r.offset += int64(n)
	}

	// Combine header and payload
	entryData := append(headerData, payloadData...)

	// Deserialize the complete entry
	entry, err := wal.DeserializeWALEntry(entryData)
	if err != nil {
		return nil, entryData, err
	}

	return entry, entryData, nil
}

// parseWALHeader parses a WAL entry header
func parseWALHeader(data []byte) (*walHeaderInfo, error) {
	if len(data) < wal.WALEntryHeaderSize {
		return nil, fmt.Errorf("invalid header size")
	}

	// This is a simplified header parser - in practice, you'd use the same
	// binary parsing as in the WAL package
	header := &walHeaderInfo{
		PayloadLen: uint32(data[28]) | uint32(data[29])<<8 | uint32(data[30])<<16 | uint32(data[31])<<24,
	}

	return header, nil
}

type walHeaderInfo struct {
	PayloadLen uint32
}
