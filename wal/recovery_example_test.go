package wal

import (
	"fmt"
	"os"
	"time"
)

// ExampleRecoveryEngine_AnalyzeWAL demonstrates how to use the recovery engine
// to analyze WAL files and create a recovery plan
func ExampleRecoveryEngine_AnalyzeWAL() {
	// Create temporary directory for example
	tempDir, err := os.MkdirTemp("", "recovery_example")
	if err != nil {
		fmt.Printf("Failed to create temp dir: %v\n", err)
		return
	}
	defer os.RemoveAll(tempDir)

	// Create WAL file manager and write some test data
	config := DefaultWALFileManagerConfig()
	config.WALDir = tempDir

	manager, err := NewWALFileManager(config)
	if err != nil {
		fmt.Printf("Failed to create WAL manager: %v\n", err)
		return
	}
	defer manager.Close()

	// Simulate a crash scenario with mixed transaction states
	entries := []*WALEntry{
		// Transaction 1: Complete (Insert + Commit)
		{TxnID: 1, Operation: Operation{Type: OpInsert, Key: "user:1", Value: []byte("John Doe")}, Timestamp: time.Now()},
		{TxnID: 1, Operation: Operation{Type: OpCommit}, Timestamp: time.Now()},

		// Transaction 2: Aborted (Update + Abort)
		{TxnID: 2, Operation: Operation{Type: OpUpdate, Key: "user:1", Value: []byte("Jane Doe"), OldValue: []byte("John Doe")}, Timestamp: time.Now()},
		{TxnID: 2, Operation: Operation{Type: OpAbort}, Timestamp: time.Now()},

		// Transaction 3: Complete (Delete + Commit)
		{TxnID: 3, Operation: Operation{Type: OpDelete, Key: "user:1", OldValue: []byte("John Doe")}, Timestamp: time.Now()},
		{TxnID: 3, Operation: Operation{Type: OpCommit}, Timestamp: time.Now()},

		// Transaction 4: Incomplete (Insert only, no commit/abort)
		{TxnID: 4, Operation: Operation{Type: OpInsert, Key: "user:2", Value: []byte("Bob Smith")}, Timestamp: time.Now()},
	}

	// Write entries to WAL
	for _, entry := range entries {
		if err := manager.WriteEntry(entry); err != nil {
			fmt.Printf("Failed to write entry: %v\n", err)
			return
		}
	}

	if err := manager.Sync(); err != nil {
		fmt.Printf("Failed to sync: %v\n", err)
		return
	}

	// Create recovery engine
	engine, err := NewRecoveryEngine(tempDir)
	if err != nil {
		fmt.Printf("Failed to create recovery engine: %v\n", err)
		return
	}

	// Analyze WAL to create recovery plan
	plan, err := engine.AnalyzeWAL()
	if err != nil {
		fmt.Printf("Failed to analyze WAL: %v\n", err)
		return
	}

	// Display recovery plan
	fmt.Printf("Recovery Plan:\n")
	fmt.Printf("  LSN Range: %d - %d\n", plan.StartLSN, plan.EndLSN)
	fmt.Printf("  Transactions: %d\n", len(plan.Transactions))
	fmt.Printf("  Operations to replay: %d\n", len(plan.Operations))
	fmt.Printf("  Corrupted entries: %d\n", len(plan.CorruptedEntries))

	// Show transaction states (sorted by transaction ID for consistent output)
	fmt.Printf("\nTransaction States:\n")
	for txnID := uint64(1); txnID <= 4; txnID++ {
		if txnState, exists := plan.Transactions[txnID]; exists {
			status := "ACTIVE"
			switch txnState.Status {
			case TxnStatusCommitted:
				status = "COMMITTED"
			case TxnStatusAborted:
				status = "ABORTED"
			}
			fmt.Printf("  Txn %d: %s (%d operations)\n", txnID, status, len(txnState.Operations))
		}
	}

	// Show operations that will be replayed
	fmt.Printf("\nOperations to replay:\n")
	for _, op := range plan.Operations {
		fmt.Printf("  LSN %d: Txn %d - %v on key '%s'\n",
			op.LSN, op.TxnID, op.Operation.Type, op.Operation.Key)
	}

	// Output:
	// Recovery Plan:
	//   LSN Range: 1 - 7
	//   Transactions: 4
	//   Operations to replay: 2
	//   Corrupted entries: 0
	//
	// Transaction States:
	//   Txn 1: COMMITTED (2 operations)
	//   Txn 2: ABORTED (2 operations)
	//   Txn 3: COMMITTED (2 operations)
	//   Txn 4: ACTIVE (1 operations)
	//
	// Operations to replay:
	//   LSN 1: Txn 1 - OpInsert on key 'user:1'
	//   LSN 5: Txn 3 - OpDelete on key 'user:1'
}

// ExampleRecoveryEngine_ReplayOperations demonstrates how to replay operations
// from a recovery plan
func ExampleRecoveryEngine_ReplayOperations() {
	// Create a simple recovery plan
	plan := &RecoveryPlan{
		Operations: []*WALEntry{
			{LSN: 1, TxnID: 1, Operation: Operation{Type: OpInsert, Key: "key1", Value: []byte("value1")}},
			{LSN: 3, TxnID: 2, Operation: Operation{Type: OpUpdate, Key: "key1", Value: []byte("value2"), OldValue: []byte("value1")}},
			{LSN: 5, TxnID: 3, Operation: Operation{Type: OpDelete, Key: "key1", OldValue: []byte("value2")}},
		},
	}

	// Create recovery engine (using empty directory for this example)
	tempDir, _ := os.MkdirTemp("", "replay_example")
	defer os.RemoveAll(tempDir)

	engine, err := NewRecoveryEngine(tempDir)
	if err != nil {
		fmt.Printf("Failed to create recovery engine: %v\n", err)
		return
	}

	// Simulate a storage system
	storage := make(map[string][]byte)

	// Define replay function
	replayFunc := func(entry *WALEntry) error {
		switch entry.Operation.Type {
		case OpInsert:
			storage[entry.Operation.Key] = entry.Operation.Value
			fmt.Printf("Replayed INSERT: %s = %s\n", entry.Operation.Key, string(entry.Operation.Value))
		case OpUpdate:
			storage[entry.Operation.Key] = entry.Operation.Value
			fmt.Printf("Replayed UPDATE: %s = %s (was %s)\n",
				entry.Operation.Key, string(entry.Operation.Value), string(entry.Operation.OldValue))
		case OpDelete:
			delete(storage, entry.Operation.Key)
			fmt.Printf("Replayed DELETE: %s (was %s)\n",
				entry.Operation.Key, string(entry.Operation.OldValue))
		}
		return nil
	}

	// Replay operations
	if err := engine.ReplayOperations(plan, replayFunc); err != nil {
		fmt.Printf("Failed to replay operations: %v\n", err)
		return
	}

	fmt.Printf("\nFinal storage state: %v\n", storage)

	// Output:
	// Replayed INSERT: key1 = value1
	// Replayed UPDATE: key1 = value2 (was value1)
	// Replayed DELETE: key1 (was value2)
	//
	// Final storage state: map[]
}

// ExampleWALValidator_ValidateEntry demonstrates WAL entry validation
func ExampleWALValidator_ValidateEntry() {
	validator := NewWALValidator(true) // Strict mode

	// Valid entry
	validEntry := &WALEntry{
		LSN:   1,
		TxnID: 1,
		Operation: Operation{
			Type:  OpInsert,
			Key:   "user:123",
			Value: []byte("John Doe"),
		},
		Timestamp: time.Now(),
	}

	if err := validator.ValidateEntry(validEntry); err != nil {
		fmt.Printf("Validation failed: %v\n", err)
	} else {
		fmt.Printf("Entry is valid\n")
	}

	// Invalid entry (empty key)
	invalidEntry := &WALEntry{
		LSN:   2,
		TxnID: 1,
		Operation: Operation{
			Type:  OpInsert,
			Key:   "", // Invalid: empty key
			Value: []byte("Jane Doe"),
		},
		Timestamp: time.Now(),
	}

	if err := validator.ValidateEntry(invalidEntry); err != nil {
		fmt.Printf("Validation failed: %v\n", err)
	} else {
		fmt.Printf("Entry is valid\n")
	}

	// Output:
	// Entry is valid
	// Validation failed: operation key cannot be empty for OpInsert operation
}
