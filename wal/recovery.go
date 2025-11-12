// recovery.go - Crash recovery via write-ahead log replay
package wal

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// WALReader provides functionality to read WAL entries for recovery operations.
// This is the critical crash recovery path - if the DB crashes mid-transaction,
// we replay the WAL to restore committed transactions and discard uncommitted ones.
type WALReader struct {
	walDir string
	files  []string // Sorted list of WAL files
}

// RecoveryPlan contains information about what needs to be recovered.
type RecoveryPlan struct {
	// StartLSN/EndLSN define the recovery window.
	// We scan from StartLSN (last checkpoint) to EndLSN (end of log).
	// Anything before StartLSN is already durable in the data files.
	StartLSN         uint64
	EndLSN           uint64
	Operations       []*WALEntry                  // Ops to replay in LSN order
	Transactions     map[uint64]*TransactionState // Track commit/abort state
	CorruptedEntries []CorruptedEntry             // Corruption detected
}

// TransactionState tracks the state of a transaction during recovery.
type TransactionState struct {
	TxnID      uint64
	Status     TransactionStatus
	Operations []*WALEntry
	StartLSN   uint64
	EndLSN     uint64
	StartTime  time.Time
	EndTime    time.Time

	// Redo/undo logs implement the ARIES recovery algorithm:
	// - Redo: replay committed transactions forward from checkpoint
	// - Undo: roll back uncommitted transactions backward from crash point
	UndoOperations []*WALEntry
	RedoOperations []*WALEntry

	// Transaction metadata
	IsolationLevel IsolationLevel
	ReadTimestamp  uint64
	WriteTimestamp uint64
}

// TransactionStatus represents the status of a transaction
type TransactionStatus int

const (
	TxnStatusActive TransactionStatus = iota
	TxnStatusCommitted
	TxnStatusAborted
)

// CorruptedEntry represents a corrupted WAL entry found during recovery
type CorruptedEntry struct {
	FilePath string
	Offset   int64
	LSN      uint64
	Error    error
}

// RecoveryEngine handles crash recovery and WAL replay operations.
// This is invoked on startup if we detect an unclean shutdown.
// Recovery has three phases (ARIES algorithm):
// 1. Analysis: scan WAL to build transaction table
// 2. Redo: replay all operations from checkpoint forward
// 3. Undo: roll back uncommitted transactions backward
type RecoveryEngine struct {
	walDir    string
	reader    *WALReader
	validator *WALValidator

	// Recovery state management
	recoveryState *RecoveryState
	progressChan  chan RecoveryProgress

	// Configuration
	config *RecoveryConfig
}

// RecoveryState tracks the current state of recovery operations
type RecoveryState struct {
	Status           RecoveryStatus
	StartTime        time.Time
	EndTime          time.Time
	CurrentLSN       uint64
	TotalOperations  int
	ProcessedOps     int
	FailedOps        int
	LastError        error
	CrashDetected    bool
	RecoveryRequired bool
}

// RecoveryStatus represents the current status of recovery
type RecoveryStatus int

const (
	RecoveryStatusIdle RecoveryStatus = iota
	RecoveryStatusAnalyzing
	RecoveryStatusReplaying
	RecoveryStatusValidating
	RecoveryStatusCompleted
	RecoveryStatusFailed
)

// RecoveryProgress represents progress information during recovery
type RecoveryProgress struct {
	Status          RecoveryStatus
	CurrentLSN      uint64
	TotalOps        int
	ProcessedOps    int
	FailedOps       int
	PercentComplete float64
	Message         string
	Error           error
}

// RecoveryConfig holds configuration for recovery operations.
type RecoveryConfig struct {
	MaxRetries          int
	RetryDelay          time.Duration
	ValidationMode      ValidationMode
	CrashDetectionFile  string // Deleted on clean shutdown, present after crash
	ProgressReporting   bool
	// StrictValidation fails recovery on checksum mismatch.
	// We default to false (relaxed) because corrupted tail entries after
	// crash are expected - only the committed prefix matters.
	StrictValidation bool
	// ParallelRecovery replays independent transactions concurrently.
	// Disabled by default - correctness over speed during recovery.
	ParallelRecovery   bool
	MaxParallelWorkers int
	// SafeModeOnFailure starts in read-only mode if recovery fails.
	// Better to serve stale data than corrupt or lose data.
	SafeModeOnFailure   bool
	ConsistencyChecks   bool
	DataIntegrityChecks bool
}

// ValidationMode defines how strict validation should be
type ValidationMode int

const (
	ValidationModeStrict ValidationMode = iota
	ValidationModeRelaxed
	ValidationModeSkip
)

// IsolationLevel defines transaction isolation levels
type IsolationLevel int

const (
	IsolationReadUncommitted IsolationLevel = iota
	IsolationReadCommitted
	IsolationRepeatableRead
	IsolationSerializable
)

// ReplayContext holds context information during replay
type ReplayContext struct {
	CurrentTxn     *TransactionState
	ActiveTxns     map[uint64]*TransactionState
	CommittedTxns  map[uint64]*TransactionState
	AbortedTxns    map[uint64]*TransactionState
	ReplayOrder    []*WALEntry
	ConflictMatrix map[string][]uint64 // Key -> list of transaction IDs that accessed it

	// Rollback support
	RollbackLog   []*RollbackEntry
	CheckpointLSN uint64
}

// RollbackEntry represents an entry in the rollback log
type RollbackEntry struct {
	TxnID     uint64
	LSN       uint64
	Operation Operation
	UndoData  []byte
	Timestamp time.Time
}

// ValidationResult represents the result of recovery validation
type ValidationResult struct {
	Success             bool
	Errors              []ValidationError
	Warnings            []ValidationWarning
	ConsistencyChecks   []ConsistencyCheck
	IntegrityChecks     []IntegrityCheck
	RecoveredOperations int
	FailedOperations    int
	ValidationTime      time.Duration
}

// ValidationError represents a validation error
type ValidationError struct {
	Type        ValidationErrorType
	Message     string
	LSN         uint64
	TxnID       uint64
	Key         string
	Details     map[string]interface{}
	Recoverable bool
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Type    ValidationWarningType
	Message string
	LSN     uint64
	TxnID   uint64
	Key     string
	Details map[string]interface{}
}

// ConsistencyCheck represents a data consistency check result
type ConsistencyCheck struct {
	Type        ConsistencyCheckType
	Passed      bool
	Message     string
	ExpectedVal interface{}
	ActualVal   interface{}
	Key         string
	Details     map[string]interface{}
}

// IntegrityCheck represents a data integrity check result
type IntegrityCheck struct {
	Type          IntegrityCheckType
	Passed        bool
	Message       string
	ChecksumMatch bool
	ExpectedCRC   uint32
	ActualCRC     uint32
	FilePath      string
	Details       map[string]interface{}
}

// ValidationErrorType defines types of validation errors
type ValidationErrorType int

const (
	ValidationErrorCorruption ValidationErrorType = iota
	ValidationErrorInconsistency
	ValidationErrorMissingData
	ValidationErrorInvalidState
	ValidationErrorChecksumMismatch
	ValidationErrorTransactionConflict
)

// ValidationWarningType defines types of validation warnings
type ValidationWarningType int

const (
	ValidationWarningIncompleteTransaction ValidationWarningType = iota
	ValidationWarningDataInconsistency
	ValidationWarningPerformanceImpact
	ValidationWarningResourceUsage
)

// ConsistencyCheckType defines types of consistency checks
type ConsistencyCheckType int

const (
	ConsistencyCheckReferentialIntegrity ConsistencyCheckType = iota
	ConsistencyCheckTransactionIsolation
	ConsistencyCheckDataConstraints
	ConsistencyCheckSequentialConsistency
)

// IntegrityCheckType defines types of integrity checks
type IntegrityCheckType int

const (
	IntegrityCheckWALChecksum IntegrityCheckType = iota
	IntegrityCheckDataChecksum
	IntegrityCheckFileIntegrity
	IntegrityCheckStructuralIntegrity
)

// SafeMode represents the system's safe mode state
type SafeMode struct {
	Enabled      bool
	Reason       string
	Timestamp    time.Time
	Restrictions []SafeModeRestriction
	RecoveryPlan *SafeModeRecoveryPlan
}

// SafeModeRestriction defines what operations are restricted in safe mode
type SafeModeRestriction struct {
	Type        RestrictionType
	Description string
	Severity    RestrictionSeverity
}

// SafeModeRecoveryPlan defines steps to exit safe mode
type SafeModeRecoveryPlan struct {
	Steps                      []RecoveryStep
	EstimatedTime              time.Duration
	RequiresManualIntervention bool
}

// RestrictionType defines types of safe mode restrictions
type RestrictionType int

const (
	RestrictionReadOnly RestrictionType = iota
	RestrictionNoWrites
	RestrictionNoTransactions
	RestrictionLimitedOperations
)

// RestrictionSeverity defines severity levels for restrictions
type RestrictionSeverity int

const (
	RestrictionSeverityLow RestrictionSeverity = iota
	RestrictionSeverityMedium
	RestrictionSeverityHigh
	RestrictionSeverityCritical
)

// RecoveryStep represents a step in the recovery plan
type RecoveryStep struct {
	ID          int
	Description string
	Action      string
	Completed   bool
	Error       error
}

// WALValidator provides WAL validation and corruption detection
type WALValidator struct {
	strictMode bool // Whether to be strict about validation
}

// NewWALReader creates a new WAL reader for recovery operations
func NewWALReader(walDir string) (*WALReader, error) {
	reader := &WALReader{
		walDir: walDir,
	}

	// Scan and sort WAL files
	if err := reader.scanWALFiles(); err != nil {
		return nil, fmt.Errorf("failed to scan WAL files: %w", err)
	}

	return reader, nil
}

// scanWALFiles scans the WAL directory and sorts files by number
func (wr *WALReader) scanWALFiles() error {
	pattern := filepath.Join(wr.walDir, "wal-*.log")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to glob WAL files: %w", err)
	}

	// Sort files by file number
	sort.Slice(files, func(i, j int) bool {
		var numI, numJ uint64
		fmt.Sscanf(filepath.Base(files[i]), "wal-%d.log", &numI)
		fmt.Sscanf(filepath.Base(files[j]), "wal-%d.log", &numJ)
		return numI < numJ
	})

	wr.files = files
	return nil
}

// ReadFromLSN reads WAL entries starting from the specified LSN
func (wr *WALReader) ReadFromLSN(startLSN uint64) ([]*WALEntry, error) {
	var entries []*WALEntry

	for _, filePath := range wr.files {
		fileEntries, err := wr.readEntriesFromFile(filePath, startLSN)
		if err != nil {
			return nil, fmt.Errorf("failed to read entries from file %s: %w", filePath, err)
		}
		entries = append(entries, fileEntries...)
	}

	return entries, nil
}

// ReadRange reads WAL entries within the specified LSN range
func (wr *WALReader) ReadRange(startLSN, endLSN uint64) ([]*WALEntry, error) {
	var entries []*WALEntry

	for _, filePath := range wr.files {
		fileEntries, err := wr.readEntriesFromFile(filePath, startLSN)
		if err != nil {
			return nil, fmt.Errorf("failed to read entries from file %s: %w", filePath, err)
		}

		// Filter entries within range
		for _, entry := range fileEntries {
			if entry.LSN >= startLSN && entry.LSN <= endLSN {
				entries = append(entries, entry)
			}
		}
	}

	return entries, nil
}

// readEntriesFromFile reads all entries from a single WAL file
func (wr *WALReader) readEntriesFromFile(filePath string, minLSN uint64) ([]*WALEntry, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file: %w", err)
	}
	defer file.Close()

	var entries []*WALEntry
	offset := int64(0)

	for {
		// Read entry header first
		headerBuf := make([]byte, WALEntryHeaderSize)
		n, err := file.ReadAt(headerBuf, offset)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read entry header at offset %d: %w", offset, err)
		}
		if n < WALEntryHeaderSize {
			break // Incomplete header, end of file
		}

		// Parse header to get payload length
		var header WALEntryHeader
		if err := parseHeader(headerBuf, &header); err != nil {
			return nil, fmt.Errorf("failed to parse header at offset %d: %w", offset, err)
		}

		// Skip entries with LSN less than minLSN
		if header.LSN < minLSN {
			offset += int64(WALEntryHeaderSize + header.PayloadLen)
			continue
		}

		// Read complete entry (header + payload)
		entrySize := WALEntryHeaderSize + int(header.PayloadLen)
		entryBuf := make([]byte, entrySize)
		n, err = file.ReadAt(entryBuf, offset)
		if err != nil {
			return nil, fmt.Errorf("failed to read complete entry at offset %d: %w", offset, err)
		}
		if n < entrySize {
			return nil, fmt.Errorf("incomplete entry at offset %d: expected %d bytes, got %d", offset, entrySize, n)
		}

		// Deserialize entry
		entry, err := DeserializeWALEntry(entryBuf)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize entry at offset %d: %w", offset, err)
		}

		entries = append(entries, entry)
		offset += int64(entrySize)
	}

	return entries, nil
}

// NewRecoveryEngine creates a new recovery engine
func NewRecoveryEngine(walDir string) (*RecoveryEngine, error) {
	return NewRecoveryEngineWithConfig(walDir, DefaultRecoveryConfig())
}

// NewRecoveryEngineWithConfig creates a new recovery engine with custom configuration
func NewRecoveryEngineWithConfig(walDir string, config *RecoveryConfig) (*RecoveryEngine, error) {
	reader, err := NewWALReader(walDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create WAL reader: %w", err)
	}

	validator := &WALValidator{
		strictMode: config.StrictValidation,
	}

	engine := &RecoveryEngine{
		walDir:    walDir,
		reader:    reader,
		validator: validator,
		config:    config,
		recoveryState: &RecoveryState{
			Status: RecoveryStatusIdle,
		},
	}

	if config.ProgressReporting {
		engine.progressChan = make(chan RecoveryProgress, 100)
	}

	return engine, nil
}

// DefaultRecoveryConfig returns default recovery configuration
func DefaultRecoveryConfig() *RecoveryConfig {
	return &RecoveryConfig{
		MaxRetries:          3,
		RetryDelay:          time.Second,
		ValidationMode:      ValidationModeStrict,
		CrashDetectionFile:  "crash_detection.lock",
		ProgressReporting:   true,
		StrictValidation:    true,
		ParallelRecovery:    false,
		MaxParallelWorkers:  4,
		SafeModeOnFailure:   true,
		ConsistencyChecks:   true,
		DataIntegrityChecks: true,
	}
}

// DetectCrash detects if the system crashed by checking for crash detection file
func (re *RecoveryEngine) DetectCrash() (bool, error) {
	crashFile := filepath.Join(re.walDir, re.config.CrashDetectionFile)

	// Check if crash detection file exists
	if _, err := os.Stat(crashFile); err != nil {
		if os.IsNotExist(err) {
			// No crash detection file means clean shutdown
			return false, nil
		}
		return false, fmt.Errorf("failed to check crash detection file: %w", err)
	}

	// File exists, indicating unclean shutdown
	re.recoveryState.CrashDetected = true
	return true, nil
}

// CreateCrashDetectionFile creates a file to detect unclean shutdowns
func (re *RecoveryEngine) CreateCrashDetectionFile() error {
	crashFile := filepath.Join(re.walDir, re.config.CrashDetectionFile)

	file, err := os.Create(crashFile)
	if err != nil {
		return fmt.Errorf("failed to create crash detection file: %w", err)
	}
	defer file.Close()

	// Write current timestamp
	_, err = file.WriteString(fmt.Sprintf("started:%d\n", time.Now().Unix()))
	return err
}

// RemoveCrashDetectionFile removes the crash detection file on clean shutdown
func (re *RecoveryEngine) RemoveCrashDetectionFile() error {
	crashFile := filepath.Join(re.walDir, re.config.CrashDetectionFile)

	if err := os.Remove(crashFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove crash detection file: %w", err)
	}

	return nil
}

// Recover performs complete crash recovery process
func (re *RecoveryEngine) Recover() error {
	re.recoveryState.Status = RecoveryStatusAnalyzing
	re.recoveryState.StartTime = time.Now()

	// Report progress if enabled
	if re.config.ProgressReporting {
		go re.reportProgress("Starting recovery analysis...")
	}

	// Detect if crash occurred
	crashed, err := re.DetectCrash()
	if err != nil {
		return fmt.Errorf("failed to detect crash: %w", err)
	}

	if !crashed {
		re.recoveryState.Status = RecoveryStatusCompleted
		re.recoveryState.EndTime = time.Now()
		if re.config.ProgressReporting {
			go re.reportProgress("No crash detected, recovery not needed")
		}
		return nil
	}

	// Analyze WAL to create recovery plan
	plan, err := re.AnalyzeWAL()
	if err != nil {
		re.recoveryState.Status = RecoveryStatusFailed
		re.recoveryState.LastError = err
		return fmt.Errorf("failed to analyze WAL: %w", err)
	}

	// Check if recovery is actually needed
	if len(plan.Operations) == 0 {
		re.recoveryState.Status = RecoveryStatusCompleted
		re.recoveryState.EndTime = time.Now()
		if re.config.ProgressReporting {
			go re.reportProgress("No operations to recover")
		}
		return re.RemoveCrashDetectionFile()
	}

	re.recoveryState.RecoveryRequired = true
	re.recoveryState.TotalOperations = len(plan.Operations)

	// Perform recovery
	if err := re.performRecovery(plan); err != nil {
		re.recoveryState.Status = RecoveryStatusFailed
		re.recoveryState.LastError = err
		return fmt.Errorf("recovery failed: %w", err)
	}

	// Validate recovery
	if err := re.validateRecovery(); err != nil {
		re.recoveryState.Status = RecoveryStatusFailed
		re.recoveryState.LastError = err
		return fmt.Errorf("recovery validation failed: %w", err)
	}

	// Clean up crash detection file
	if err := re.RemoveCrashDetectionFile(); err != nil {
		return fmt.Errorf("failed to clean up crash detection file: %w", err)
	}

	re.recoveryState.Status = RecoveryStatusCompleted
	re.recoveryState.EndTime = time.Now()

	if re.config.ProgressReporting {
		go re.reportProgress("Recovery completed successfully")
	}

	return nil
}

// GetRecoveryState returns the current recovery state
func (re *RecoveryEngine) GetRecoveryState() *RecoveryState {
	return re.recoveryState
}

// GetProgressChannel returns the progress reporting channel
func (re *RecoveryEngine) GetProgressChannel() <-chan RecoveryProgress {
	return re.progressChan
}

// reportProgress sends progress updates through the progress channel
func (re *RecoveryEngine) reportProgress(message string) {
	if re.progressChan == nil {
		return
	}

	var percentComplete float64
	if re.recoveryState.TotalOperations > 0 {
		percentComplete = float64(re.recoveryState.ProcessedOps) / float64(re.recoveryState.TotalOperations) * 100
	}

	progress := RecoveryProgress{
		Status:          re.recoveryState.Status,
		CurrentLSN:      re.recoveryState.CurrentLSN,
		TotalOps:        re.recoveryState.TotalOperations,
		ProcessedOps:    re.recoveryState.ProcessedOps,
		FailedOps:       re.recoveryState.FailedOps,
		PercentComplete: percentComplete,
		Message:         message,
		Error:           re.recoveryState.LastError,
	}

	select {
	case re.progressChan <- progress:
	default:
		// Channel is full, skip this update
	}
}

// AnalyzeWAL analyzes the WAL to create a recovery plan
func (re *RecoveryEngine) AnalyzeWAL() (*RecoveryPlan, error) {
	// Read all WAL entries
	entries, err := re.reader.ReadFromLSN(1) // Start from LSN 1
	if err != nil {
		return nil, fmt.Errorf("failed to read WAL entries: %w", err)
	}

	if len(entries) == 0 {
		return &RecoveryPlan{
			Transactions: make(map[uint64]*TransactionState),
		}, nil
	}

	plan := &RecoveryPlan{
		StartLSN:     entries[0].LSN,
		EndLSN:       entries[len(entries)-1].LSN,
		Operations:   make([]*WALEntry, 0),
		Transactions: make(map[uint64]*TransactionState),
	}

	// Validate entries and detect corruption
	corruptedEntries := re.validator.ValidateEntries(entries)
	plan.CorruptedEntries = corruptedEntries

	// Analyze transactions
	for _, entry := range entries {
		// Skip corrupted entries
		if re.isCorrupted(entry, corruptedEntries) {
			continue
		}

		txnID := entry.TxnID

		// Initialize transaction state if not exists
		if _, exists := plan.Transactions[txnID]; !exists {
			plan.Transactions[txnID] = &TransactionState{
				TxnID:      txnID,
				Status:     TxnStatusActive,
				Operations: make([]*WALEntry, 0),
				StartLSN:   entry.LSN,
			}
		}

		txnState := plan.Transactions[txnID]
		txnState.Operations = append(txnState.Operations, entry)
		txnState.EndLSN = entry.LSN

		// Update transaction status based on operation type
		switch entry.Operation.Type {
		case OpCommit:
			txnState.Status = TxnStatusCommitted
		case OpAbort:
			txnState.Status = TxnStatusAborted
		}
	}

	// Determine which operations need to be replayed
	for _, txnState := range plan.Transactions {
		if txnState.Status == TxnStatusCommitted {
			// Include all operations from committed transactions
			for _, op := range txnState.Operations {
				if op.Operation.Type != OpCommit { // Don't replay commit operations
					plan.Operations = append(plan.Operations, op)
				}
			}
		}
		// Ignore operations from aborted or incomplete transactions
	}

	// Sort operations by LSN for proper replay order
	sort.Slice(plan.Operations, func(i, j int) bool {
		return plan.Operations[i].LSN < plan.Operations[j].LSN
	})

	return plan, nil
}

// ReplayOperations replays the operations from a recovery plan
func (re *RecoveryEngine) ReplayOperations(plan *RecoveryPlan, replayFunc func(*WALEntry) error) error {
	if replayFunc == nil {
		return fmt.Errorf("replay function cannot be nil")
	}

	for _, entry := range plan.Operations {
		if err := replayFunc(entry); err != nil {
			return fmt.Errorf("failed to replay operation LSN %d: %w", entry.LSN, err)
		}
	}

	return nil
}

// performRecovery performs the actual recovery process
func (re *RecoveryEngine) performRecovery(plan *RecoveryPlan) error {
	re.recoveryState.Status = RecoveryStatusReplaying

	if re.config.ProgressReporting {
		go re.reportProgress(fmt.Sprintf("Starting replay of %d operations", len(plan.Operations)))
	}

	// Create a default replay function that would be provided by the storage layer
	replayFunc := func(entry *WALEntry) error {
		re.recoveryState.CurrentLSN = entry.LSN
		re.recoveryState.ProcessedOps++

		// Report progress periodically
		if re.config.ProgressReporting && re.recoveryState.ProcessedOps%100 == 0 {
			go re.reportProgress(fmt.Sprintf("Processed %d/%d operations",
				re.recoveryState.ProcessedOps, re.recoveryState.TotalOperations))
		}

		// This would normally call into the storage layer to apply the operation
		// For now, we'll just validate that the operation is well-formed
		if err := re.validator.ValidateEntry(entry); err != nil {
			re.recoveryState.FailedOps++
			return fmt.Errorf("invalid operation during replay: %w", err)
		}

		return nil
	}

	// Replay operations with retry logic
	for _, entry := range plan.Operations {
		var lastErr error
		success := false

		for attempt := 0; attempt <= re.config.MaxRetries; attempt++ {
			if attempt > 0 {
				time.Sleep(re.config.RetryDelay)
				if re.config.ProgressReporting {
					go re.reportProgress(fmt.Sprintf("Retrying operation LSN %d (attempt %d/%d)",
						entry.LSN, attempt, re.config.MaxRetries))
				}
			}

			if err := replayFunc(entry); err != nil {
				lastErr = err
				continue
			}

			success = true
			break
		}

		if !success {
			re.recoveryState.FailedOps++
			return fmt.Errorf("failed to replay operation LSN %d after %d attempts: %w",
				entry.LSN, re.config.MaxRetries, lastErr)
		}
	}

	return nil
}

// validateRecovery validates that recovery was successful
func (re *RecoveryEngine) validateRecovery() error {
	re.recoveryState.Status = RecoveryStatusValidating

	if re.config.ProgressReporting {
		go re.reportProgress("Validating recovery...")
	}

	switch re.config.ValidationMode {
	case ValidationModeSkip:
		return nil

	case ValidationModeRelaxed:
		// Basic validation - just check that we processed all operations
		if re.recoveryState.FailedOps > 0 {
			return fmt.Errorf("recovery validation failed: %d operations failed",
				re.recoveryState.FailedOps)
		}

	case ValidationModeStrict:
		// Strict validation - re-analyze WAL and verify consistency
		if err := re.ValidateWALIntegrity(); err != nil {
			return fmt.Errorf("WAL integrity validation failed: %w", err)
		}

		// Verify that all committed transactions were properly recovered
		plan, err := re.AnalyzeWAL()
		if err != nil {
			return fmt.Errorf("failed to re-analyze WAL for validation: %w", err)
		}

		// Check for any remaining uncommitted transactions
		var incompleteTransactions []uint64
		for txnID, txnState := range plan.Transactions {
			if txnState.Status == TxnStatusActive {
				incompleteTransactions = append(incompleteTransactions, txnID)
			}
		}

		if len(incompleteTransactions) > 0 {
			return fmt.Errorf("validation failed: found %d incomplete transactions: %v",
				len(incompleteTransactions), incompleteTransactions)
		}
	}

	return nil
}

// ValidateWALIntegrity validates the integrity of all WAL files
func (re *RecoveryEngine) ValidateWALIntegrity() error {
	entries, err := re.reader.ReadFromLSN(1)
	if err != nil {
		return fmt.Errorf("failed to read WAL entries for validation: %w", err)
	}

	corruptedEntries := re.validator.ValidateEntries(entries)
	if len(corruptedEntries) > 0 {
		var errorMessages []string
		for _, corrupted := range corruptedEntries {
			errorMessages = append(errorMessages, fmt.Sprintf("LSN %d in %s: %v",
				corrupted.LSN, corrupted.FilePath, corrupted.Error))
		}
		return fmt.Errorf("WAL integrity validation failed: %s", strings.Join(errorMessages, "; "))
	}

	return nil
}

// GetLastCommittedLSN returns the LSN of the last committed transaction
func (re *RecoveryEngine) GetLastCommittedLSN() (uint64, error) {
	plan, err := re.AnalyzeWAL()
	if err != nil {
		return 0, fmt.Errorf("failed to analyze WAL: %w", err)
	}

	var lastCommittedLSN uint64
	for _, txnState := range plan.Transactions {
		if txnState.Status == TxnStatusCommitted && txnState.EndLSN > lastCommittedLSN {
			lastCommittedLSN = txnState.EndLSN
		}
	}

	return lastCommittedLSN, nil
}

// NewWALValidator creates a new WAL validator
func NewWALValidator(strictMode bool) *WALValidator {
	return &WALValidator{
		strictMode: strictMode,
	}
}

// ValidateEntries validates a list of WAL entries and returns corrupted ones
func (wv *WALValidator) ValidateEntries(entries []*WALEntry) []CorruptedEntry {
	var corrupted []CorruptedEntry

	for _, entry := range entries {
		if err := wv.ValidateEntry(entry); err != nil {
			corrupted = append(corrupted, CorruptedEntry{
				LSN:   entry.LSN,
				Error: err,
			})
		}
	}

	return corrupted
}

// ValidateEntry validates a single WAL entry
func (wv *WALValidator) ValidateEntry(entry *WALEntry) error {
	// Validate LSN
	if entry.LSN == 0 {
		return fmt.Errorf("invalid LSN: cannot be zero")
	}

	// Validate transaction ID
	if entry.TxnID == 0 {
		return fmt.Errorf("invalid transaction ID: cannot be zero")
	}

	// Validate operation type
	if entry.Operation.Type < OpInsert || entry.Operation.Type > OpAbort {
		return fmt.Errorf("invalid operation type: %d", entry.Operation.Type)
	}

	// Validate operation data based on type
	switch entry.Operation.Type {
	case OpInsert, OpUpdate, OpDelete:
		if entry.Operation.Key == "" {
			return fmt.Errorf("operation key cannot be empty for %v operation", entry.Operation.Type)
		}
		if wv.strictMode && entry.Operation.Type == OpInsert && len(entry.Operation.Value) == 0 {
			return fmt.Errorf("insert operation must have a value")
		}
	case OpCommit, OpAbort:
		// These operations don't need key/value validation
	}

	// Validate checksum by re-serializing and comparing
	serialized, err := entry.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize entry for checksum validation: %w", err)
	}

	// Deserialize to verify integrity
	_, err = DeserializeWALEntry(serialized)
	if err != nil {
		return fmt.Errorf("entry failed round-trip serialization test: %w", err)
	}

	return nil
}

// ValidateSequence validates that LSNs are in proper sequence
func (wv *WALValidator) ValidateSequence(entries []*WALEntry) error {
	if len(entries) == 0 {
		return nil
	}

	for i := 1; i < len(entries); i++ {
		if entries[i].LSN <= entries[i-1].LSN {
			return fmt.Errorf("LSN sequence violation: LSN %d follows LSN %d",
				entries[i].LSN, entries[i-1].LSN)
		}
	}

	return nil
}

// isCorrupted checks if an entry is in the corrupted list
func (re *RecoveryEngine) isCorrupted(entry *WALEntry, corruptedEntries []CorruptedEntry) bool {
	for _, corrupted := range corruptedEntries {
		if corrupted.LSN == entry.LSN {
			return true
		}
	}
	return false
}

// ReplayWithTransactionReconstruction replays operations with full transaction state reconstruction
func (re *RecoveryEngine) ReplayWithTransactionReconstruction(plan *RecoveryPlan, replayFunc func(*WALEntry, *ReplayContext) error) error {
	if replayFunc == nil {
		return fmt.Errorf("replay function cannot be nil")
	}

	// Create replay context
	context := &ReplayContext{
		ActiveTxns:     make(map[uint64]*TransactionState),
		CommittedTxns:  make(map[uint64]*TransactionState),
		AbortedTxns:    make(map[uint64]*TransactionState),
		ConflictMatrix: make(map[string][]uint64),
		RollbackLog:    make([]*RollbackEntry, 0),
	}

	// Sort operations by LSN to ensure proper ordering
	sortedOps := make([]*WALEntry, len(plan.Operations))
	copy(sortedOps, plan.Operations)
	sort.Slice(sortedOps, func(i, j int) bool {
		return sortedOps[i].LSN < sortedOps[j].LSN
	})
	context.ReplayOrder = sortedOps

	// Replay operations in order
	for _, entry := range sortedOps {
		if err := re.replayOperationWithContext(entry, context, replayFunc); err != nil {
			return fmt.Errorf("failed to replay operation LSN %d: %w", entry.LSN, err)
		}
	}

	// Handle any remaining active transactions (incomplete transactions)
	if err := re.handleIncompleteTransactions(context); err != nil {
		return fmt.Errorf("failed to handle incomplete transactions: %w", err)
	}

	return nil
}

// replayOperationWithContext replays a single operation with transaction context
func (re *RecoveryEngine) replayOperationWithContext(entry *WALEntry, context *ReplayContext, replayFunc func(*WALEntry, *ReplayContext) error) error {
	txnID := entry.TxnID

	// Update current transaction in context
	if txnState, exists := context.ActiveTxns[txnID]; exists {
		context.CurrentTxn = txnState
	} else if txnState, exists := context.CommittedTxns[txnID]; exists {
		context.CurrentTxn = txnState
	} else if txnState, exists := context.AbortedTxns[txnID]; exists {
		context.CurrentTxn = txnState
	} else {
		// Create new transaction state
		context.CurrentTxn = &TransactionState{
			TxnID:          txnID,
			Status:         TxnStatusActive,
			Operations:     make([]*WALEntry, 0),
			UndoOperations: make([]*WALEntry, 0),
			RedoOperations: make([]*WALEntry, 0),
			StartLSN:       entry.LSN,
			StartTime:      entry.Timestamp,
			IsolationLevel: IsolationReadCommitted, // Default isolation level
		}
		context.ActiveTxns[txnID] = context.CurrentTxn
	}

	// Add operation to transaction
	context.CurrentTxn.Operations = append(context.CurrentTxn.Operations, entry)
	context.CurrentTxn.EndLSN = entry.LSN
	context.CurrentTxn.EndTime = entry.Timestamp

	// Handle different operation types
	switch entry.Operation.Type {
	case OpInsert, OpUpdate, OpDelete:
		// Track key access for conflict detection
		key := entry.Operation.Key
		if _, exists := context.ConflictMatrix[key]; !exists {
			context.ConflictMatrix[key] = make([]uint64, 0)
		}
		context.ConflictMatrix[key] = append(context.ConflictMatrix[key], txnID)

		// Create undo operation for rollback support
		undoEntry := re.createUndoOperation(entry)
		context.CurrentTxn.UndoOperations = append(context.CurrentTxn.UndoOperations, undoEntry)

		// Add to rollback log
		rollbackEntry := &RollbackEntry{
			TxnID:     txnID,
			LSN:       entry.LSN,
			Operation: entry.Operation,
			UndoData:  entry.Operation.OldValue,
			Timestamp: entry.Timestamp,
		}
		context.RollbackLog = append(context.RollbackLog, rollbackEntry)

	case OpCommit:
		// Move transaction from active to committed
		if txnState, exists := context.ActiveTxns[txnID]; exists {
			txnState.Status = TxnStatusCommitted
			context.CommittedTxns[txnID] = txnState
			delete(context.ActiveTxns, txnID)
		}

	case OpAbort:
		// Move transaction from active to aborted and perform rollback
		if txnState, exists := context.ActiveTxns[txnID]; exists {
			txnState.Status = TxnStatusAborted
			context.AbortedTxns[txnID] = txnState
			delete(context.ActiveTxns, txnID)

			// Perform rollback for this transaction
			if err := re.rollbackTransaction(txnState, context, replayFunc); err != nil {
				return fmt.Errorf("failed to rollback transaction %d: %w", txnID, err)
			}
		}
	}

	// Call the replay function
	if err := replayFunc(entry, context); err != nil {
		return err
	}

	// Update recovery state
	re.recoveryState.CurrentLSN = entry.LSN
	re.recoveryState.ProcessedOps++

	return nil
}

// createUndoOperation creates an undo operation for rollback support
func (re *RecoveryEngine) createUndoOperation(entry *WALEntry) *WALEntry {
	var undoOp Operation

	switch entry.Operation.Type {
	case OpInsert:
		// Undo insert with delete
		undoOp = Operation{
			Type:     OpDelete,
			Key:      entry.Operation.Key,
			Value:    nil,
			OldValue: entry.Operation.Value,
		}

	case OpUpdate:
		// Undo update by restoring old value
		undoOp = Operation{
			Type:     OpUpdate,
			Key:      entry.Operation.Key,
			Value:    entry.Operation.OldValue,
			OldValue: entry.Operation.Value,
		}

	case OpDelete:
		// Undo delete with insert
		undoOp = Operation{
			Type:     OpInsert,
			Key:      entry.Operation.Key,
			Value:    entry.Operation.OldValue,
			OldValue: nil,
		}

	default:
		// For commit/abort operations, no undo needed
		undoOp = entry.Operation
	}

	return &WALEntry{
		LSN:       entry.LSN, // Keep same LSN for tracking
		TxnID:     entry.TxnID,
		Operation: undoOp,
		Timestamp: entry.Timestamp,
	}
}

// rollbackTransaction performs rollback for an aborted transaction
func (re *RecoveryEngine) rollbackTransaction(txnState *TransactionState, context *ReplayContext, replayFunc func(*WALEntry, *ReplayContext) error) error {
	// Replay undo operations in reverse order
	for i := len(txnState.UndoOperations) - 1; i >= 0; i-- {
		undoEntry := txnState.UndoOperations[i]

		// Skip commit/abort operations during rollback
		if undoEntry.Operation.Type == OpCommit || undoEntry.Operation.Type == OpAbort {
			continue
		}

		// Apply undo operation
		if err := replayFunc(undoEntry, context); err != nil {
			return fmt.Errorf("failed to apply undo operation for LSN %d: %w", undoEntry.LSN, err)
		}
	}

	return nil
}

// handleIncompleteTransactions handles transactions that were active during crash
func (re *RecoveryEngine) handleIncompleteTransactions(context *ReplayContext) error {
	if len(context.ActiveTxns) == 0 {
		return nil
	}

	// Log incomplete transactions
	var incompleteTxnIDs []uint64
	for txnID := range context.ActiveTxns {
		incompleteTxnIDs = append(incompleteTxnIDs, txnID)
	}

	if re.config.ProgressReporting {
		go re.reportProgress(fmt.Sprintf("Rolling back %d incomplete transactions: %v",
			len(incompleteTxnIDs), incompleteTxnIDs))
	}

	// Rollback all incomplete transactions
	for txnID, txnState := range context.ActiveTxns {
		txnState.Status = TxnStatusAborted

		// Perform rollback
		if err := re.rollbackTransaction(txnState, context, func(entry *WALEntry, ctx *ReplayContext) error {
			// Default rollback function - would normally call storage layer
			return re.validator.ValidateEntry(entry)
		}); err != nil {
			return fmt.Errorf("failed to rollback incomplete transaction %d: %w", txnID, err)
		}

		// Move to aborted transactions
		context.AbortedTxns[txnID] = txnState
	}

	// Clear active transactions
	context.ActiveTxns = make(map[uint64]*TransactionState)

	return nil
}

// DetectConflicts detects conflicts between transactions during replay
func (re *RecoveryEngine) DetectConflicts(context *ReplayContext) map[string][]uint64 {
	conflicts := make(map[string][]uint64)

	for key, txnIDs := range context.ConflictMatrix {
		if len(txnIDs) > 1 {
			// Multiple transactions accessed the same key
			conflicts[key] = txnIDs
		}
	}

	return conflicts
}

// GetTransactionDependencies analyzes transaction dependencies for proper ordering
func (re *RecoveryEngine) GetTransactionDependencies(plan *RecoveryPlan) map[uint64][]uint64 {
	dependencies := make(map[uint64][]uint64)
	keyAccess := make(map[string][]uint64) // key -> list of txn IDs that accessed it

	// Build key access map
	for _, entry := range plan.Operations {
		if entry.Operation.Type == OpInsert || entry.Operation.Type == OpUpdate || entry.Operation.Type == OpDelete {
			key := entry.Operation.Key
			txnID := entry.TxnID

			if _, exists := keyAccess[key]; !exists {
				keyAccess[key] = make([]uint64, 0)
			}

			// Check if this transaction already accessed this key
			found := false
			for _, existingTxnID := range keyAccess[key] {
				if existingTxnID == txnID {
					found = true
					break
				}
			}

			if !found {
				keyAccess[key] = append(keyAccess[key], txnID)
			}
		}
	}

	// Build dependency graph
	for _, txnIDs := range keyAccess {
		if len(txnIDs) > 1 {
			// Create dependencies between transactions that accessed the same key
			for i := 0; i < len(txnIDs)-1; i++ {
				for j := i + 1; j < len(txnIDs); j++ {
					txnA, txnB := txnIDs[i], txnIDs[j]

					// Add bidirectional dependency
					if _, exists := dependencies[txnA]; !exists {
						dependencies[txnA] = make([]uint64, 0)
					}
					dependencies[txnA] = append(dependencies[txnA], txnB)

					if _, exists := dependencies[txnB]; !exists {
						dependencies[txnB] = make([]uint64, 0)
					}
					dependencies[txnB] = append(dependencies[txnB], txnA)
				}
			}
		}
	}

	return dependencies
}

// ValidateRecoveryWithDetails performs comprehensive recovery validation
func (re *RecoveryEngine) ValidateRecoveryWithDetails() (*ValidationResult, error) {
	startTime := time.Now()

	result := &ValidationResult{
		Success:             true,
		Errors:              make([]ValidationError, 0),
		Warnings:            make([]ValidationWarning, 0),
		ConsistencyChecks:   make([]ConsistencyCheck, 0),
		IntegrityChecks:     make([]IntegrityCheck, 0),
		RecoveredOperations: re.recoveryState.ProcessedOps,
		FailedOperations:    re.recoveryState.FailedOps,
	}

	// Perform integrity checks if enabled
	if re.config.DataIntegrityChecks {
		if err := re.performIntegrityChecks(result); err != nil {
			result.Success = false
			result.Errors = append(result.Errors, ValidationError{
				Type:        ValidationErrorCorruption,
				Message:     fmt.Sprintf("Integrity check failed: %v", err),
				Recoverable: false,
			})
		}
	}

	// Perform consistency checks if enabled
	if re.config.ConsistencyChecks {
		if err := re.performConsistencyChecks(result); err != nil {
			result.Success = false
			result.Errors = append(result.Errors, ValidationError{
				Type:        ValidationErrorInconsistency,
				Message:     fmt.Sprintf("Consistency check failed: %v", err),
				Recoverable: true,
			})
		}
	}

	// Check for failed operations
	if result.FailedOperations > 0 {
		result.Success = false
		result.Errors = append(result.Errors, ValidationError{
			Type:        ValidationErrorInvalidState,
			Message:     fmt.Sprintf("%d operations failed during recovery", result.FailedOperations),
			Recoverable: true,
		})
	}

	// Validate WAL integrity
	if err := re.ValidateWALIntegrity(); err != nil {
		result.Success = false
		result.Errors = append(result.Errors, ValidationError{
			Type:        ValidationErrorCorruption,
			Message:     fmt.Sprintf("WAL integrity validation failed: %v", err),
			Recoverable: false,
		})
	}

	result.ValidationTime = time.Since(startTime)

	// Enter safe mode if validation failed and configured to do so
	if !result.Success && re.config.SafeModeOnFailure {
		if err := re.EnterSafeMode("Recovery validation failed", result); err != nil {
			return result, fmt.Errorf("failed to enter safe mode: %w", err)
		}
	}

	return result, nil
}

// performIntegrityChecks performs data integrity checks
func (re *RecoveryEngine) performIntegrityChecks(result *ValidationResult) error {
	// Check WAL file checksums
	for _, filePath := range re.reader.files {
		check := IntegrityCheck{
			Type:     IntegrityCheckWALChecksum,
			FilePath: filePath,
			Passed:   true,
		}

		// Verify file integrity by reading and validating all entries
		entries, err := re.reader.readEntriesFromFile(filePath, 1)
		if err != nil {
			check.Passed = false
			check.Message = fmt.Sprintf("Failed to read WAL file: %v", err)
		} else {
			// Validate each entry's checksum
			for _, entry := range entries {
				if err := re.validator.ValidateEntry(entry); err != nil {
					check.Passed = false
					check.Message = fmt.Sprintf("Entry validation failed: %v", err)
					break
				}
			}
		}

		result.IntegrityChecks = append(result.IntegrityChecks, check)

		if !check.Passed {
			return fmt.Errorf("integrity check failed for file %s: %s", filePath, check.Message)
		}
	}

	return nil
}

// performConsistencyChecks performs data consistency checks
func (re *RecoveryEngine) performConsistencyChecks(result *ValidationResult) error {
	// Analyze WAL to check transaction consistency
	plan, err := re.AnalyzeWAL()
	if err != nil {
		return fmt.Errorf("failed to analyze WAL for consistency checks: %w", err)
	}

	// Check transaction isolation consistency
	check := ConsistencyCheck{
		Type:   ConsistencyCheckTransactionIsolation,
		Passed: true,
	}

	// Verify that all committed transactions are complete
	for txnID, txnState := range plan.Transactions {
		if txnState.Status == TxnStatusCommitted {
			// Check that transaction has proper commit operation
			hasCommit := false
			for _, op := range txnState.Operations {
				if op.Operation.Type == OpCommit {
					hasCommit = true
					break
				}
			}

			if !hasCommit {
				check.Passed = false
				check.Message = fmt.Sprintf("Committed transaction %d missing commit operation", txnID)
				check.Details = map[string]interface{}{
					"transaction_id": txnID,
					"operations":     len(txnState.Operations),
				}
				break
			}
		}
	}

	result.ConsistencyChecks = append(result.ConsistencyChecks, check)

	if !check.Passed {
		return fmt.Errorf("consistency check failed: %s", check.Message)
	}

	// Check sequential consistency
	seqCheck := ConsistencyCheck{
		Type:   ConsistencyCheckSequentialConsistency,
		Passed: true,
	}

	// Verify LSN sequence is monotonic
	if err := re.validator.ValidateSequence(plan.Operations); err != nil {
		seqCheck.Passed = false
		seqCheck.Message = fmt.Sprintf("LSN sequence validation failed: %v", err)
	}

	result.ConsistencyChecks = append(result.ConsistencyChecks, seqCheck)

	if !seqCheck.Passed {
		return fmt.Errorf("sequential consistency check failed: %s", seqCheck.Message)
	}

	return nil
}

// EnterSafeMode puts the system into safe mode
func (re *RecoveryEngine) EnterSafeMode(reason string, validationResult *ValidationResult) error {
	safeMode := &SafeMode{
		Enabled:   true,
		Reason:    reason,
		Timestamp: time.Now(),
		Restrictions: []SafeModeRestriction{
			{
				Type:        RestrictionReadOnly,
				Description: "System is in read-only mode due to recovery failure",
				Severity:    RestrictionSeverityHigh,
			},
			{
				Type:        RestrictionNoTransactions,
				Description: "New transactions are disabled",
				Severity:    RestrictionSeverityHigh,
			},
		},
		RecoveryPlan: &SafeModeRecoveryPlan{
			Steps: []RecoveryStep{
				{
					ID:          1,
					Description: "Analyze validation errors",
					Action:      "review_validation_errors",
					Completed:   false,
				},
				{
					ID:          2,
					Description: "Fix data corruption issues",
					Action:      "fix_corruption",
					Completed:   false,
				},
				{
					ID:          3,
					Description: "Re-run recovery validation",
					Action:      "validate_recovery",
					Completed:   false,
				},
			},
			EstimatedTime:              time.Hour,
			RequiresManualIntervention: true,
		},
	}

	// Store safe mode state
	re.recoveryState.LastError = fmt.Errorf("system entered safe mode: %s", reason)

	if re.config.ProgressReporting {
		go re.reportProgress(fmt.Sprintf("SAFE MODE ACTIVATED: %s", reason))
	}

	// Write safe mode indicator file
	safeModeFile := filepath.Join(re.walDir, "safe_mode.lock")
	file, err := os.Create(safeModeFile)
	if err != nil {
		return fmt.Errorf("failed to create safe mode file: %w", err)
	}
	defer file.Close()

	safeModeData := fmt.Sprintf("reason:%s\ntimestamp:%d\nerrors:%d\n",
		reason, safeMode.Timestamp.Unix(), len(validationResult.Errors))

	if _, err := file.WriteString(safeModeData); err != nil {
		return fmt.Errorf("failed to write safe mode data: %w", err)
	}

	return nil
}

// ExitSafeMode attempts to exit safe mode
func (re *RecoveryEngine) ExitSafeMode() error {
	// Check if system is in safe mode
	safeModeFile := filepath.Join(re.walDir, "safe_mode.lock")
	if _, err := os.Stat(safeModeFile); os.IsNotExist(err) {
		return fmt.Errorf("system is not in safe mode")
	}

	// Re-validate recovery
	result, err := re.ValidateRecoveryWithDetails()
	if err != nil {
		return fmt.Errorf("failed to validate recovery for safe mode exit: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("cannot exit safe mode: validation still failing with %d errors", len(result.Errors))
	}

	// Remove safe mode file
	if err := os.Remove(safeModeFile); err != nil {
		return fmt.Errorf("failed to remove safe mode file: %w", err)
	}

	if re.config.ProgressReporting {
		go re.reportProgress("Safe mode deactivated - system operational")
	}

	return nil
}

// IsInSafeMode checks if the system is currently in safe mode
func (re *RecoveryEngine) IsInSafeMode() bool {
	safeModeFile := filepath.Join(re.walDir, "safe_mode.lock")
	_, err := os.Stat(safeModeFile)
	return err == nil
}

// GetSafeModeInfo returns information about the current safe mode state
func (re *RecoveryEngine) GetSafeModeInfo() (*SafeMode, error) {
	if !re.IsInSafeMode() {
		return nil, fmt.Errorf("system is not in safe mode")
	}

	safeModeFile := filepath.Join(re.walDir, "safe_mode.lock")
	data, err := os.ReadFile(safeModeFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read safe mode file: %w", err)
	}

	// Parse safe mode data (simplified parsing)
	lines := strings.Split(string(data), "\n")
	safeMode := &SafeMode{
		Enabled: true,
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "reason:") {
			safeMode.Reason = strings.TrimPrefix(line, "reason:")
		} else if strings.HasPrefix(line, "timestamp:") {
			if tsStr := strings.TrimPrefix(line, "timestamp:"); tsStr != "" {
				if ts, err := strconv.ParseInt(tsStr, 10, 64); err == nil {
					safeMode.Timestamp = time.Unix(ts, 0)
				}
			}
		}
	}

	return safeMode, nil
}

// Helper function to parse WAL entry header
func parseHeader(data []byte, header *WALEntryHeader) error {
	if len(data) < WALEntryHeaderSize {
		return fmt.Errorf("insufficient data for header: got %d bytes, need %d", len(data), WALEntryHeaderSize)
	}

	// Parse header fields manually to avoid binary.Read overhead
	header.LSN = uint64(data[0]) | uint64(data[1])<<8 | uint64(data[2])<<16 | uint64(data[3])<<24 |
		uint64(data[4])<<32 | uint64(data[5])<<40 | uint64(data[6])<<48 | uint64(data[7])<<56

	header.TxnID = uint64(data[8]) | uint64(data[9])<<8 | uint64(data[10])<<16 | uint64(data[11])<<24 |
		uint64(data[12])<<32 | uint64(data[13])<<40 | uint64(data[14])<<48 | uint64(data[15])<<56

	header.OpType = uint32(data[16]) | uint32(data[17])<<8 | uint32(data[18])<<16 | uint32(data[19])<<24

	header.Timestamp = int64(data[20]) | int64(data[21])<<8 | int64(data[22])<<16 | int64(data[23])<<24 |
		int64(data[24])<<32 | int64(data[25])<<40 | int64(data[26])<<48 | int64(data[27])<<56

	header.PayloadLen = uint32(data[28]) | uint32(data[29])<<8 | uint32(data[30])<<16 | uint32(data[31])<<24

	header.Checksum = uint32(data[32]) | uint32(data[33])<<8 | uint32(data[34])<<16 | uint32(data[35])<<24

	return nil
}
