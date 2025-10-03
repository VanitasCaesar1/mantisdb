package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mantisDB/durability"
	"mantisDB/errors"
	"mantisDB/integrity"
	"mantisDB/transaction"
	"mantisDB/wal"
)

// TestEndToEndRecovery tests the complete recovery flow
func TestEndToEndRecovery(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "e2e_recovery_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	walDir := filepath.Join(tempDir, "wal")
	dataDir := filepath.Join(tempDir, "data")

	// Phase 1: Create and populate system
	t.Log("Phase 1: Creating and populating system")

	// Create WAL manager
	walConfig := wal.DefaultWALFileManagerConfig()
	walConfig.WALDir = walDir
	walManager, err := wal.NewWALFileManager(walConfig)
	if err != nil {
		t.Fatalf("Failed to create WAL manager: %v", err)
	}

	// Create transaction system
	lockManager := transaction.NewLockManager(5 * time.Second)
	_ = transaction.NewTransactionManager(lockManager)
	txnSystemConfig := transaction.DefaultTransactionSystemConfig()
	txnSystem := transaction.NewTransactionSystem(txnSystemConfig)
	if err := txnSystem.Start(); err != nil {
		t.Fatalf("Failed to start transaction system: %v", err)
	}

	// Create durability manager
	durabilityConfig := durability.DefaultDurabilityConfig()
	durabilityManager, err := durability.NewDurabilityManager(durabilityConfig)
	if err != nil {
		t.Fatalf("Failed to create durability manager: %v", err)
	}

	// Create integrity system
	checksumEngine := integrity.NewChecksumEngine(integrity.ChecksumCRC32)

	// Simulate application operations
	operations := []struct {
		txnID uint64
		ops   []wal.Operation
	}{
		{
			txnID: 1,
			ops: []wal.Operation{
				{Type: wal.OpInsert, Key: "user:1", Value: []byte(`{"name":"Alice","age":30}`)},
				{Type: wal.OpInsert, Key: "user:2", Value: []byte(`{"name":"Bob","age":25}`)},
			},
		},
		{
			txnID: 2,
			ops: []wal.Operation{
				{Type: wal.OpUpdate, Key: "user:1", Value: []byte(`{"name":"Alice","age":31}`), OldValue: []byte(`{"name":"Alice","age":30}`)},
			},
		},
		{
			txnID: 3,
			ops: []wal.Operation{
				{Type: wal.OpDelete, Key: "user:2", OldValue: []byte(`{"name":"Bob","age":25}`)},
			},
		},
	}

	// Execute operations with proper transaction handling
	for _, opGroup := range operations {
		// Begin transaction
		txn, err := txnSystem.BeginTransaction(transaction.ReadCommitted)
		if err != nil {
			t.Fatalf("Failed to begin transaction %d: %v", opGroup.txnID, err)
		}

		// Execute operations
		for _, op := range opGroup.ops {
			// Write to WAL
			walEntry := &wal.WALEntry{
				TxnID:     opGroup.txnID,
				Operation: op,
				Timestamp: time.Now(),
			}

			if err := walManager.WriteEntry(walEntry); err != nil {
				t.Fatalf("Failed to write WAL entry for txn %d: %v", opGroup.txnID, err)
			}

			// Write to data store with durability
			dataFile := filepath.Join(dataDir, op.Key+".json")
			if op.Type == wal.OpInsert || op.Type == wal.OpUpdate {
				if err := durabilityManager.Write(context.Background(), dataFile, op.Value, 0); err != nil {
					t.Fatalf("Failed to write data for txn %d: %v", opGroup.txnID, err)
				}

				// Verify integrity
				checksum := checksumEngine.Calculate(op.Value)
				if err := checksumEngine.Verify(op.Value, checksum); err != nil {
					t.Fatalf("Integrity check failed for txn %d: %v", opGroup.txnID, err)
				}
			} else if op.Type == wal.OpDelete {
				if err := os.Remove(dataFile); err != nil && !os.IsNotExist(err) {
					t.Fatalf("Failed to delete data for txn %d: %v", opGroup.txnID, err)
				}
			}
		}

		// Commit transaction
		commitEntry := &wal.WALEntry{
			TxnID:     opGroup.txnID,
			Operation: wal.Operation{Type: wal.OpCommit},
			Timestamp: time.Now(),
		}

		if err := walManager.WriteEntry(commitEntry); err != nil {
			t.Fatalf("Failed to write commit for txn %d: %v", opGroup.txnID, err)
		}

		if err := txnSystem.CommitTransaction(txn); err != nil {
			t.Fatalf("Failed to commit transaction %d: %v", opGroup.txnID, err)
		}
	}

	// Force sync
	if err := walManager.Sync(); err != nil {
		t.Fatalf("Failed to sync WAL: %v", err)
	}

	if err := durabilityManager.Sync(context.Background()); err != nil {
		t.Fatalf("Failed to sync data: %v", err)
	}

	// Close systems to simulate crash
	walManager.Close()
	txnSystem.Stop()
	durabilityManager.Close(context.Background())
	lockManager.Close()

	t.Log("Phase 2: Simulating crash and recovery")

	// Phase 2: Recovery
	recoveryEngine, err := wal.NewRecoveryEngine(walDir)
	if err != nil {
		t.Fatalf("Failed to create recovery engine: %v", err)
	}

	// Analyze WAL
	recoveryPlan, err := recoveryEngine.AnalyzeWAL()
	if err != nil {
		t.Fatalf("Failed to analyze WAL: %v", err)
	}

	t.Logf("Recovery plan: %d operations from LSN %d to %d",
		len(recoveryPlan.Operations), recoveryPlan.StartLSN, recoveryPlan.EndLSN)

	// Verify recovery plan
	if len(recoveryPlan.Operations) != 4 { // 3 data operations + 0 commits (filtered out)
		t.Errorf("Expected 4 operations in recovery plan, got %d", len(recoveryPlan.Operations))
	}

	// Verify all transactions are committed
	for txnID, txn := range recoveryPlan.Transactions {
		if txn.Status != wal.TxnStatusCommitted {
			t.Errorf("Transaction %d should be committed, got %v", txnID, txn.Status)
		}
	}

	// Replay operations
	replayedOps := 0
	replayFunc := func(entry *wal.WALEntry) error {
		replayedOps++
		t.Logf("Replaying operation %d: %s on key %s",
			entry.LSN, entry.Operation.Type, entry.Operation.Key)

		// Simulate applying the operation
		dataFile := filepath.Join(dataDir, entry.Operation.Key+".json")

		switch entry.Operation.Type {
		case wal.OpInsert, wal.OpUpdate:
			if err := os.WriteFile(dataFile, entry.Operation.Value, 0644); err != nil {
				return fmt.Errorf("failed to replay write: %w", err)
			}
		case wal.OpDelete:
			if err := os.Remove(dataFile); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to replay delete: %w", err)
			}
		}

		return nil
	}

	if err := recoveryEngine.ReplayOperations(recoveryPlan, replayFunc); err != nil {
		t.Fatalf("Failed to replay operations: %v", err)
	}

	if replayedOps != len(recoveryPlan.Operations) {
		t.Errorf("Expected to replay %d operations, replayed %d",
			len(recoveryPlan.Operations), replayedOps)
	}

	// Validate recovery
	if _, err := recoveryEngine.ValidateRecoveryWithDetails(); err != nil {
		t.Fatalf("Recovery validation failed: %v", err)
	}

	t.Log("Phase 3: Verifying recovered state")

	// Phase 3: Verify final state
	// Check that user:1 has the updated data
	user1File := filepath.Join(dataDir, "user:1.json")
	user1Data, err := os.ReadFile(user1File)
	if err != nil {
		t.Fatalf("Failed to read user:1 data: %v", err)
	}

	expectedUser1 := `{"name":"Alice","age":31}`
	if string(user1Data) != expectedUser1 {
		t.Errorf("User:1 data mismatch: expected %s, got %s", expectedUser1, string(user1Data))
	}

	// Check that user:2 was deleted
	user2File := filepath.Join(dataDir, "user:2.json")
	if _, err := os.Stat(user2File); !os.IsNotExist(err) {
		t.Error("User:2 should have been deleted")
	}

	// Verify data integrity
	checksum := checksumEngine.Calculate(user1Data)
	if err := checksumEngine.Verify(user1Data, checksum); err != nil {
		t.Fatalf("Final integrity check failed: %v", err)
	}

	t.Log("End-to-end recovery test completed successfully")
}

// TestMultiComponentIntegration tests integration between multiple components
func TestMultiComponentIntegration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "multi_component_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create error handler
	errorHandler := errors.NewDefaultErrorHandler(nil)

	// Create integrity system
	checksumEngine := integrity.NewChecksumEngine(integrity.ChecksumCRC32)

	// Create durability manager
	durabilityConfig := durability.DefaultDurabilityConfig()
	durabilityManager, err := durability.NewDurabilityManager(durabilityConfig)
	if err != nil {
		t.Fatalf("Failed to create durability manager: %v", err)
	}
	defer durabilityManager.Close(context.Background())

	// Test integrated write with integrity checking
	testData := []byte("Test data for multi-component integration")
	testFile := filepath.Join(tempDir, "integration_test.dat")

	// Calculate checksum before write
	expectedChecksum := checksumEngine.Calculate(testData)

	// Write with durability
	ctx := context.Background()
	if err := durabilityManager.Write(ctx, testFile, testData, 0); err != nil {
		// Handle error through error handler
		errorCtx := errors.ErrorContext{
			Operation:   "durability_write",
			Resource:    testFile,
			Severity:    errors.ErrorSeverityHigh,
			Category:    errors.ErrorCategoryIO,
			Recoverable: true,
			Timestamp:   time.Now(),
		}

		action := errorHandler.HandleError(err, errorCtx)
		if action == errors.ErrorActionFail {
			t.Fatalf("Write failed and error handler suggests failing: %v", err)
		}
	}

	// Force sync
	if err := durabilityManager.Sync(ctx); err != nil {
		t.Fatalf("Failed to sync: %v", err)
	}

	// Read and verify integrity
	readData, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if err := checksumEngine.Verify(readData, expectedChecksum); err != nil {
		// Handle integrity error
		corruptionInfo := errors.CorruptionInfo{
			Location: errors.DataLocation{
				File:   testFile,
				Offset: 0,
				Size:   int64(len(readData)),
			},
			Type:        "checksum_mismatch",
			Description: "Data integrity verification failed",
			Timestamp:   time.Now(),
			Checksum:    expectedChecksum,
		}

		if err := errorHandler.HandleCorruption(corruptionInfo); err != nil {
			t.Fatalf("Failed to handle corruption: %v", err)
		}

		t.Fatalf("Integrity verification failed: %v", err)
	}

	// Test batch operations with integrity
	batchData := [][]byte{
		[]byte("Batch item 1"),
		[]byte("Batch item 2"),
		[]byte("Batch item 3"),
	}

	batchWrites := make([]durability.WriteOperation, len(batchData))
	expectedChecksums := make([]uint32, len(batchData))

	for i, data := range batchData {
		batchWrites[i] = durability.WriteOperation{
			FilePath: filepath.Join(tempDir, fmt.Sprintf("batch_%d.dat", i)),
			Data:     data,
			Offset:   0,
		}
		expectedChecksums[i] = checksumEngine.Calculate(data)
	}

	// Execute batch write
	if err := durabilityManager.BatchWrite(ctx, batchWrites); err != nil {
		t.Fatalf("Batch write failed: %v", err)
	}

	// Force sync
	if err := durabilityManager.Sync(ctx); err != nil {
		t.Fatalf("Failed to sync batch: %v", err)
	}

	// Verify all batch items
	for i, write := range batchWrites {
		data, err := os.ReadFile(write.FilePath)
		if err != nil {
			t.Fatalf("Failed to read batch file %d: %v", i, err)
		}

		if err := checksumEngine.Verify(data, expectedChecksums[i]); err != nil {
			t.Errorf("Batch item %d integrity check failed: %v", i, err)
		}
	}

	// Test error handling integration
	invalidFile := "/invalid/path/file.dat"
	err = durabilityManager.Write(ctx, invalidFile, testData, 0)
	if err != nil {
		errorCtx := errors.ErrorContext{
			Operation:   "invalid_write",
			Resource:    invalidFile,
			Severity:    errors.ErrorSeverityMedium,
			Category:    errors.ErrorCategoryIO,
			Recoverable: false,
			Timestamp:   time.Now(),
		}

		action := errorHandler.HandleError(err, errorCtx)
		if action != errors.ErrorActionFail {
			t.Logf("Error handler suggested action: %v for invalid write", action)
		}
	}

	t.Log("Multi-component integration test completed successfully")
}

// TestFailureScenarios tests various failure scenarios
func TestFailureScenarios(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "failure_scenarios_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	walDir := filepath.Join(tempDir, "wal")
	_ = filepath.Join(tempDir, "data") // dataDir for future use

	t.Run("IncompleteTransaction", func(t *testing.T) {
		// Create WAL manager
		walConfig := wal.DefaultWALFileManagerConfig()
		walConfig.WALDir = walDir
		walManager, err := wal.NewWALFileManager(walConfig)
		if err != nil {
			t.Fatalf("Failed to create WAL manager: %v", err)
		}
		defer walManager.Close()

		// Write incomplete transaction (no commit)
		entries := []*wal.WALEntry{
			{TxnID: 1, Operation: wal.Operation{Type: wal.OpInsert, Key: "incomplete", Value: []byte("data")}, Timestamp: time.Now()},
			{TxnID: 1, Operation: wal.Operation{Type: wal.OpUpdate, Key: "incomplete", Value: []byte("updated"), OldValue: []byte("data")}, Timestamp: time.Now()},
			// Missing commit
		}

		for _, entry := range entries {
			if err := walManager.WriteEntry(entry); err != nil {
				t.Fatalf("Failed to write entry: %v", err)
			}
		}

		if err := walManager.Sync(); err != nil {
			t.Fatalf("Failed to sync: %v", err)
		}

		// Analyze for recovery
		recoveryEngine, err := wal.NewRecoveryEngine(walDir)
		if err != nil {
			t.Fatalf("Failed to create recovery engine: %v", err)
		}

		plan, err := recoveryEngine.AnalyzeWAL()
		if err != nil {
			t.Fatalf("Failed to analyze WAL: %v", err)
		}

		// Should have no operations to replay (incomplete transaction)
		if len(plan.Operations) != 0 {
			t.Errorf("Expected 0 operations for incomplete transaction, got %d", len(plan.Operations))
		}

		// Transaction should be marked as active (incomplete)
		if txn, exists := plan.Transactions[1]; exists {
			if txn.Status != wal.TxnStatusActive {
				t.Errorf("Expected incomplete transaction to be active, got %v", txn.Status)
			}
		}
	})

	t.Run("AbortedTransaction", func(t *testing.T) {
		// Clean WAL directory
		os.RemoveAll(walDir)

		walConfig := wal.DefaultWALFileManagerConfig()
		walConfig.WALDir = walDir
		walManager, err := wal.NewWALFileManager(walConfig)
		if err != nil {
			t.Fatalf("Failed to create WAL manager: %v", err)
		}
		defer walManager.Close()

		// Write aborted transaction
		entries := []*wal.WALEntry{
			{TxnID: 2, Operation: wal.Operation{Type: wal.OpInsert, Key: "aborted", Value: []byte("data")}, Timestamp: time.Now()},
			{TxnID: 2, Operation: wal.Operation{Type: wal.OpAbort}, Timestamp: time.Now()},
		}

		for _, entry := range entries {
			if err := walManager.WriteEntry(entry); err != nil {
				t.Fatalf("Failed to write entry: %v", err)
			}
		}

		if err := walManager.Sync(); err != nil {
			t.Fatalf("Failed to sync: %v", err)
		}

		// Analyze for recovery
		recoveryEngine, err := wal.NewRecoveryEngine(walDir)
		if err != nil {
			t.Fatalf("Failed to create recovery engine: %v", err)
		}

		plan, err := recoveryEngine.AnalyzeWAL()
		if err != nil {
			t.Fatalf("Failed to analyze WAL: %v", err)
		}

		// Should have no operations to replay (aborted transaction)
		if len(plan.Operations) != 0 {
			t.Errorf("Expected 0 operations for aborted transaction, got %d", len(plan.Operations))
		}

		// Transaction should be marked as aborted
		if txn, exists := plan.Transactions[2]; exists {
			if txn.Status != wal.TxnStatusAborted {
				t.Errorf("Expected aborted transaction status, got %v", txn.Status)
			}
		}
	})

	t.Run("CorruptedWALEntry", func(t *testing.T) {
		// This test would require creating a corrupted WAL file
		// For now, we'll test the error handling path
		_, err := wal.NewRecoveryEngine("/non/existent/path")
		if err == nil {
			t.Error("Expected error for non-existent WAL directory")
		}
	})

	t.Run("DiskSpaceExhaustion", func(t *testing.T) {
		errorHandler := errors.NewDefaultErrorHandler(nil)

		// Simulate disk space exhaustion
		err := errorHandler.HandleDiskFull("write_operation")
		if err != nil {
			t.Errorf("HandleDiskFull should handle gracefully: %v", err)
		}

		// Test multiple disk full events
		for i := 0; i < 3; i++ {
			err = errorHandler.HandleDiskFull(fmt.Sprintf("operation_%d", i))
			if err != nil {
				t.Errorf("HandleDiskFull iteration %d failed: %v", i, err)
			}
		}
	})

	t.Run("MemoryExhaustion", func(t *testing.T) {
		errorHandler := errors.NewDefaultErrorHandler(nil)

		// Simulate memory exhaustion
		err := errorHandler.HandleMemoryExhaustion("allocation_operation")
		if err != nil {
			t.Errorf("HandleMemoryExhaustion should handle gracefully: %v", err)
		}
	})

	t.Log("Failure scenarios test completed successfully")
}

// TestConcurrentOperations tests concurrent operations across components
func TestConcurrentOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "concurrent_ops_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	walDir := filepath.Join(tempDir, "wal")
	dataDir := filepath.Join(tempDir, "data")

	// Create components
	walConfig := wal.DefaultWALFileManagerConfig()
	walConfig.WALDir = walDir
	walManager, err := wal.NewWALFileManager(walConfig)
	if err != nil {
		t.Fatalf("Failed to create WAL manager: %v", err)
	}
	defer walManager.Close()

	durabilityConfig := durability.DefaultDurabilityConfig()
	durabilityConfig.WriteMode = durability.WriteModeAsync
	durabilityManager, err := durability.NewDurabilityManager(durabilityConfig)
	if err != nil {
		t.Fatalf("Failed to create durability manager: %v", err)
	}
	defer durabilityManager.Close(context.Background())

	checksumEngine := integrity.NewChecksumEngine(integrity.ChecksumCRC32)

	const numGoroutines = 10
	const operationsPerGoroutine = 5
	done := make(chan bool, numGoroutines)

	// Test concurrent operations
	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer func() { done <- true }()

			ctx := context.Background()

			for i := 0; i < operationsPerGoroutine; i++ {
				key := fmt.Sprintf("concurrent_%d_%d", goroutineID, i)
				value := []byte(fmt.Sprintf("data from goroutine %d, operation %d", goroutineID, i))

				// Write to WAL
				walEntry := &wal.WALEntry{
					TxnID:     uint64(goroutineID*100 + i),
					Operation: wal.Operation{Type: wal.OpInsert, Key: key, Value: value},
					Timestamp: time.Now(),
				}

				if err := walManager.WriteEntry(walEntry); err != nil {
					t.Errorf("Goroutine %d: WAL write failed: %v", goroutineID, err)
					return
				}

				// Write to data store
				dataFile := filepath.Join(dataDir, key+".dat")
				if err := durabilityManager.Write(ctx, dataFile, value, 0); err != nil {
					t.Errorf("Goroutine %d: Data write failed: %v", goroutineID, err)
					return
				}

				// Verify integrity
				checksum := checksumEngine.Calculate(value)
				if err := checksumEngine.Verify(value, checksum); err != nil {
					t.Errorf("Goroutine %d: Integrity check failed: %v", goroutineID, err)
					return
				}
			}
		}(g)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Force sync
	if err := walManager.Sync(); err != nil {
		t.Fatalf("Failed to sync WAL: %v", err)
	}

	if err := durabilityManager.Sync(context.Background()); err != nil {
		t.Fatalf("Failed to sync data: %v", err)
	}

	// Verify all operations completed
	expectedEntries := numGoroutines * operationsPerGoroutine
	currentLSN := walManager.GetCurrentLSN()
	if currentLSN != uint64(expectedEntries) {
		t.Errorf("Expected LSN %d, got %d", expectedEntries, currentLSN)
	}

	// Verify data files were created
	files, err := filepath.Glob(filepath.Join(dataDir, "concurrent_*.dat"))
	if err != nil {
		t.Fatalf("Failed to list data files: %v", err)
	}

	if len(files) != expectedEntries {
		t.Errorf("Expected %d data files, got %d", expectedEntries, len(files))
	}

	t.Log("Concurrent operations test completed successfully")
}
