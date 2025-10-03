package wal

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestWALReader_ReadFromLSN(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "wal_reader_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create WAL file manager and write some test entries
	config := DefaultWALFileManagerConfig()
	config.WALDir = tempDir
	config.MaxFileSize = 1024 // Small size to force rotation

	manager, err := NewWALFileManager(config)
	if err != nil {
		t.Fatalf("Failed to create WAL manager: %v", err)
	}
	defer manager.Close()

	// Write test entries
	testEntries := []*WALEntry{
		{TxnID: 1, Operation: Operation{Type: OpInsert, Key: "key1", Value: []byte("value1")}, Timestamp: time.Now()},
		{TxnID: 1, Operation: Operation{Type: OpCommit}, Timestamp: time.Now()},
		{TxnID: 2, Operation: Operation{Type: OpUpdate, Key: "key1", Value: []byte("value2"), OldValue: []byte("value1")}, Timestamp: time.Now()},
		{TxnID: 2, Operation: Operation{Type: OpCommit}, Timestamp: time.Now()},
		{TxnID: 3, Operation: Operation{Type: OpDelete, Key: "key1", OldValue: []byte("value2")}, Timestamp: time.Now()},
		{TxnID: 3, Operation: Operation{Type: OpAbort}, Timestamp: time.Now()},
	}

	for _, entry := range testEntries {
		if err := manager.WriteEntry(entry); err != nil {
			t.Fatalf("Failed to write entry: %v", err)
		}
	}

	// Force sync to ensure data is written
	if err := manager.Sync(); err != nil {
		t.Fatalf("Failed to sync: %v", err)
	}

	// Create WAL reader
	reader, err := NewWALReader(tempDir)
	if err != nil {
		t.Fatalf("Failed to create WAL reader: %v", err)
	}

	// Test reading from LSN 1
	entries, err := reader.ReadFromLSN(1)
	if err != nil {
		t.Fatalf("Failed to read from LSN 1: %v", err)
	}

	if len(entries) != len(testEntries) {
		t.Errorf("Expected %d entries, got %d", len(testEntries), len(entries))
	}

	// Verify entries are in correct order
	for i, entry := range entries {
		if entry.LSN != uint64(i+1) {
			t.Errorf("Expected LSN %d, got %d", i+1, entry.LSN)
		}
		if entry.TxnID != testEntries[i].TxnID {
			t.Errorf("Expected TxnID %d, got %d", testEntries[i].TxnID, entry.TxnID)
		}
	}

	// Test reading from LSN 3
	entries, err = reader.ReadFromLSN(3)
	if err != nil {
		t.Fatalf("Failed to read from LSN 3: %v", err)
	}

	if len(entries) != 4 { // Should get entries 3, 4, 5, 6
		t.Errorf("Expected 4 entries from LSN 3, got %d", len(entries))
	}

	if entries[0].LSN != 3 {
		t.Errorf("Expected first entry LSN 3, got %d", entries[0].LSN)
	}
}

func TestWALReader_ReadRange(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "wal_reader_range_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create WAL file manager and write test entries
	config := DefaultWALFileManagerConfig()
	config.WALDir = tempDir

	manager, err := NewWALFileManager(config)
	if err != nil {
		t.Fatalf("Failed to create WAL manager: %v", err)
	}
	defer manager.Close()

	// Write 10 test entries
	for i := 1; i <= 10; i++ {
		entry := &WALEntry{
			TxnID: uint64(i),
			Operation: Operation{
				Type:  OpInsert,
				Key:   fmt.Sprintf("key%d", i),
				Value: []byte(fmt.Sprintf("value%d", i)),
			},
			Timestamp: time.Now(),
		}
		if err := manager.WriteEntry(entry); err != nil {
			t.Fatalf("Failed to write entry %d: %v", i, err)
		}
	}

	if err := manager.Sync(); err != nil {
		t.Fatalf("Failed to sync: %v", err)
	}

	// Create WAL reader
	reader, err := NewWALReader(tempDir)
	if err != nil {
		t.Fatalf("Failed to create WAL reader: %v", err)
	}

	// Test reading range 3-7
	entries, err := reader.ReadRange(3, 7)
	if err != nil {
		t.Fatalf("Failed to read range 3-7: %v", err)
	}

	if len(entries) != 5 {
		t.Errorf("Expected 5 entries in range 3-7, got %d", len(entries))
	}

	for i, entry := range entries {
		expectedLSN := uint64(i + 3)
		if entry.LSN != expectedLSN {
			t.Errorf("Expected LSN %d, got %d", expectedLSN, entry.LSN)
		}
	}
}

func TestRecoveryEngine_AnalyzeWAL(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "recovery_analyze_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create WAL file manager and write test scenario
	config := DefaultWALFileManagerConfig()
	config.WALDir = tempDir

	manager, err := NewWALFileManager(config)
	if err != nil {
		t.Fatalf("Failed to create WAL manager: %v", err)
	}
	defer manager.Close()

	// Write test scenario:
	// - Transaction 1: Insert + Commit (should be replayed)
	// - Transaction 2: Update + Abort (should not be replayed)
	// - Transaction 3: Delete + Commit (should be replayed)
	// - Transaction 4: Insert (incomplete, should not be replayed)
	testEntries := []*WALEntry{
		{TxnID: 1, Operation: Operation{Type: OpInsert, Key: "key1", Value: []byte("value1")}, Timestamp: time.Now()},
		{TxnID: 1, Operation: Operation{Type: OpCommit}, Timestamp: time.Now()},
		{TxnID: 2, Operation: Operation{Type: OpUpdate, Key: "key1", Value: []byte("value2"), OldValue: []byte("value1")}, Timestamp: time.Now()},
		{TxnID: 2, Operation: Operation{Type: OpAbort}, Timestamp: time.Now()},
		{TxnID: 3, Operation: Operation{Type: OpDelete, Key: "key1", OldValue: []byte("value1")}, Timestamp: time.Now()},
		{TxnID: 3, Operation: Operation{Type: OpCommit}, Timestamp: time.Now()},
		{TxnID: 4, Operation: Operation{Type: OpInsert, Key: "key2", Value: []byte("value2")}, Timestamp: time.Now()},
	}

	for _, entry := range testEntries {
		if err := manager.WriteEntry(entry); err != nil {
			t.Fatalf("Failed to write entry: %v", err)
		}
	}

	if err := manager.Sync(); err != nil {
		t.Fatalf("Failed to sync: %v", err)
	}

	// Create recovery engine
	engine, err := NewRecoveryEngine(tempDir)
	if err != nil {
		t.Fatalf("Failed to create recovery engine: %v", err)
	}

	// Analyze WAL
	plan, err := engine.AnalyzeWAL()
	if err != nil {
		t.Fatalf("Failed to analyze WAL: %v", err)
	}

	// Verify recovery plan
	if plan.StartLSN != 1 {
		t.Errorf("Expected StartLSN 1, got %d", plan.StartLSN)
	}

	if plan.EndLSN != 7 {
		t.Errorf("Expected EndLSN 7, got %d", plan.EndLSN)
	}

	// Should have 3 transactions
	if len(plan.Transactions) != 4 {
		t.Errorf("Expected 4 transactions, got %d", len(plan.Transactions))
	}

	// Check transaction states
	txn1 := plan.Transactions[1]
	if txn1.Status != TxnStatusCommitted {
		t.Errorf("Expected transaction 1 to be committed, got %v", txn1.Status)
	}

	txn2 := plan.Transactions[2]
	if txn2.Status != TxnStatusAborted {
		t.Errorf("Expected transaction 2 to be aborted, got %v", txn2.Status)
	}

	txn3 := plan.Transactions[3]
	if txn3.Status != TxnStatusCommitted {
		t.Errorf("Expected transaction 3 to be committed, got %v", txn3.Status)
	}

	txn4 := plan.Transactions[4]
	if txn4.Status != TxnStatusActive {
		t.Errorf("Expected transaction 4 to be active, got %v", txn4.Status)
	}

	// Should have 2 operations to replay (from committed transactions, excluding commits)
	if len(plan.Operations) != 2 {
		t.Errorf("Expected 2 operations to replay, got %d", len(plan.Operations))
	}

	// Verify operations are from committed transactions only
	for _, op := range plan.Operations {
		if op.TxnID == 2 || op.TxnID == 4 {
			t.Errorf("Operation from transaction %d should not be in replay plan", op.TxnID)
		}
		if op.Operation.Type == OpCommit || op.Operation.Type == OpAbort {
			t.Errorf("Commit/Abort operations should not be in replay plan")
		}
	}
}

func TestRecoveryEngine_ReplayOperations(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "recovery_replay_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create recovery engine
	engine, err := NewRecoveryEngine(tempDir)
	if err != nil {
		t.Fatalf("Failed to create recovery engine: %v", err)
	}

	// Create a simple recovery plan
	plan := &RecoveryPlan{
		Operations: []*WALEntry{
			{LSN: 1, TxnID: 1, Operation: Operation{Type: OpInsert, Key: "key1", Value: []byte("value1")}},
			{LSN: 3, TxnID: 2, Operation: Operation{Type: OpUpdate, Key: "key1", Value: []byte("value2"), OldValue: []byte("value1")}},
		},
	}

	// Track replayed operations
	var replayedOps []*WALEntry
	replayFunc := func(entry *WALEntry) error {
		replayedOps = append(replayedOps, entry)
		return nil
	}

	// Replay operations
	if err := engine.ReplayOperations(plan, replayFunc); err != nil {
		t.Fatalf("Failed to replay operations: %v", err)
	}

	// Verify all operations were replayed
	if len(replayedOps) != 2 {
		t.Errorf("Expected 2 replayed operations, got %d", len(replayedOps))
	}

	// Verify operations were replayed in correct order
	if replayedOps[0].LSN != 1 || replayedOps[1].LSN != 3 {
		t.Errorf("Operations not replayed in correct order")
	}
}

func TestWALValidator_ValidateEntry(t *testing.T) {
	validator := NewWALValidator(true) // Strict mode

	// Test valid entry
	validEntry := &WALEntry{
		LSN:   1,
		TxnID: 1,
		Operation: Operation{
			Type:  OpInsert,
			Key:   "key1",
			Value: []byte("value1"),
		},
		Timestamp: time.Now(),
	}

	if err := validator.ValidateEntry(validEntry); err != nil {
		t.Errorf("Valid entry failed validation: %v", err)
	}

	// Test invalid LSN
	invalidLSNEntry := &WALEntry{
		LSN:       0, // Invalid
		TxnID:     1,
		Operation: Operation{Type: OpInsert, Key: "key1", Value: []byte("value1")},
	}

	if err := validator.ValidateEntry(invalidLSNEntry); err == nil {
		t.Error("Expected validation error for invalid LSN")
	}

	// Test invalid transaction ID
	invalidTxnEntry := &WALEntry{
		LSN:       1,
		TxnID:     0, // Invalid
		Operation: Operation{Type: OpInsert, Key: "key1", Value: []byte("value1")},
	}

	if err := validator.ValidateEntry(invalidTxnEntry); err == nil {
		t.Error("Expected validation error for invalid transaction ID")
	}

	// Test invalid operation type
	invalidOpEntry := &WALEntry{
		LSN:       1,
		TxnID:     1,
		Operation: Operation{Type: OperationType(999), Key: "key1", Value: []byte("value1")}, // Invalid type
	}

	if err := validator.ValidateEntry(invalidOpEntry); err == nil {
		t.Error("Expected validation error for invalid operation type")
	}

	// Test empty key for data operation
	emptyKeyEntry := &WALEntry{
		LSN:       1,
		TxnID:     1,
		Operation: Operation{Type: OpInsert, Key: "", Value: []byte("value1")}, // Empty key
	}

	if err := validator.ValidateEntry(emptyKeyEntry); err == nil {
		t.Error("Expected validation error for empty key")
	}

	// Test commit operation (should be valid without key/value)
	commitEntry := &WALEntry{
		LSN:       1,
		TxnID:     1,
		Operation: Operation{Type: OpCommit},
	}

	if err := validator.ValidateEntry(commitEntry); err != nil {
		t.Errorf("Commit entry failed validation: %v", err)
	}
}

func TestWALValidator_ValidateSequence(t *testing.T) {
	validator := NewWALValidator(true)

	// Test valid sequence
	validEntries := []*WALEntry{
		{LSN: 1, TxnID: 1, Operation: Operation{Type: OpInsert, Key: "key1", Value: []byte("value1")}},
		{LSN: 2, TxnID: 1, Operation: Operation{Type: OpCommit}},
		{LSN: 3, TxnID: 2, Operation: Operation{Type: OpUpdate, Key: "key1", Value: []byte("value2")}},
	}

	if err := validator.ValidateSequence(validEntries); err != nil {
		t.Errorf("Valid sequence failed validation: %v", err)
	}

	// Test invalid sequence (LSN goes backwards)
	invalidEntries := []*WALEntry{
		{LSN: 1, TxnID: 1, Operation: Operation{Type: OpInsert, Key: "key1", Value: []byte("value1")}},
		{LSN: 3, TxnID: 1, Operation: Operation{Type: OpCommit}},
		{LSN: 2, TxnID: 2, Operation: Operation{Type: OpUpdate, Key: "key1", Value: []byte("value2")}}, // Out of order
	}

	if err := validator.ValidateSequence(invalidEntries); err == nil {
		t.Error("Expected validation error for invalid sequence")
	}

	// Test duplicate LSN
	duplicateEntries := []*WALEntry{
		{LSN: 1, TxnID: 1, Operation: Operation{Type: OpInsert, Key: "key1", Value: []byte("value1")}},
		{LSN: 1, TxnID: 2, Operation: Operation{Type: OpInsert, Key: "key2", Value: []byte("value2")}}, // Duplicate LSN
	}

	if err := validator.ValidateSequence(duplicateEntries); err == nil {
		t.Error("Expected validation error for duplicate LSN")
	}
}

func TestRecoveryEngine_GetLastCommittedLSN(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "last_committed_lsn_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create WAL file manager and write test scenario
	config := DefaultWALFileManagerConfig()
	config.WALDir = tempDir

	manager, err := NewWALFileManager(config)
	if err != nil {
		t.Fatalf("Failed to create WAL manager: %v", err)
	}
	defer manager.Close()

	// Write test scenario with multiple transactions
	testEntries := []*WALEntry{
		{TxnID: 1, Operation: Operation{Type: OpInsert, Key: "key1", Value: []byte("value1")}, Timestamp: time.Now()},
		{TxnID: 1, Operation: Operation{Type: OpCommit}, Timestamp: time.Now()}, // LSN 2 - committed
		{TxnID: 2, Operation: Operation{Type: OpUpdate, Key: "key1", Value: []byte("value2")}, Timestamp: time.Now()},
		{TxnID: 2, Operation: Operation{Type: OpAbort}, Timestamp: time.Now()}, // LSN 4 - aborted
		{TxnID: 3, Operation: Operation{Type: OpDelete, Key: "key1"}, Timestamp: time.Now()},
		{TxnID: 3, Operation: Operation{Type: OpCommit}, Timestamp: time.Now()},                                       // LSN 6 - committed (latest)
		{TxnID: 4, Operation: Operation{Type: OpInsert, Key: "key2", Value: []byte("value2")}, Timestamp: time.Now()}, // LSN 7 - incomplete
	}

	for _, entry := range testEntries {
		if err := manager.WriteEntry(entry); err != nil {
			t.Fatalf("Failed to write entry: %v", err)
		}
	}

	if err := manager.Sync(); err != nil {
		t.Fatalf("Failed to sync: %v", err)
	}

	// Create recovery engine
	engine, err := NewRecoveryEngine(tempDir)
	if err != nil {
		t.Fatalf("Failed to create recovery engine: %v", err)
	}

	// Get last committed LSN
	lastLSN, err := engine.GetLastCommittedLSN()
	if err != nil {
		t.Fatalf("Failed to get last committed LSN: %v", err)
	}

	// Should be LSN 6 (transaction 3 commit)
	if lastLSN != 6 {
		t.Errorf("Expected last committed LSN 6, got %d", lastLSN)
	}
}

func TestRecoveryEngine_ValidateWALIntegrity(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "wal_integrity_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create WAL file manager and write valid entries
	config := DefaultWALFileManagerConfig()
	config.WALDir = tempDir

	manager, err := NewWALFileManager(config)
	if err != nil {
		t.Fatalf("Failed to create WAL manager: %v", err)
	}
	defer manager.Close()

	// Write valid test entries
	testEntries := []*WALEntry{
		{TxnID: 1, Operation: Operation{Type: OpInsert, Key: "key1", Value: []byte("value1")}, Timestamp: time.Now()},
		{TxnID: 1, Operation: Operation{Type: OpCommit}, Timestamp: time.Now()},
	}

	for _, entry := range testEntries {
		if err := manager.WriteEntry(entry); err != nil {
			t.Fatalf("Failed to write entry: %v", err)
		}
	}

	if err := manager.Sync(); err != nil {
		t.Fatalf("Failed to sync: %v", err)
	}

	// Create recovery engine
	engine, err := NewRecoveryEngine(tempDir)
	if err != nil {
		t.Fatalf("Failed to create recovery engine: %v", err)
	}

	// Validate WAL integrity (should pass)
	if err := engine.ValidateWALIntegrity(); err != nil {
		t.Errorf("WAL integrity validation failed unexpectedly: %v", err)
	}
}
