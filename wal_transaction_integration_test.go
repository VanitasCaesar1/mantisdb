package main

import (
	"fmt"
	"os"
	"testing"
	"time"

	"mantisDB/transaction"
	"mantisDB/wal"
)

// TestWALTransactionIntegration tests integration between WAL and transaction systems
func TestWALTransactionIntegration(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "wal_txn_integration_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create WAL manager
	walConfig := wal.DefaultWALFileManagerConfig()
	walConfig.WALDir = tempDir
	walManager, err := wal.NewWALFileManager(walConfig)
	if err != nil {
		t.Fatalf("Failed to create WAL manager: %v", err)
	}
	defer walManager.Close()

	// Create transaction system
	lockManager := transaction.NewLockManager(5 * time.Second)
	defer lockManager.Close()

	txnManager := transaction.NewTransactionManager(lockManager)
	defer txnManager.Close()

	txnSystemConfig := transaction.DefaultTransactionSystemConfig()
	txnSystem := transaction.NewTransactionSystem(txnSystemConfig)
	if err := txnSystem.Start(); err != nil {
		t.Fatalf("Failed to start transaction system: %v", err)
	}
	defer txnSystem.Stop()

	t.Run("SingleTransactionFlow", func(t *testing.T) {
		// Begin transaction
		txn, err := txnSystem.BeginTransaction(transaction.ReadCommitted)
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v", err)
		}

		// Perform operations with WAL logging
		operations := []wal.Operation{
			{Type: wal.OpInsert, Key: "key1", Value: []byte("value1")},
			{Type: wal.OpUpdate, Key: "key1", Value: []byte("value1_updated"), OldValue: []byte("value1")},
			{Type: wal.OpInsert, Key: "key2", Value: []byte("value2")},
		}

		for _, op := range operations {
			// Log operation to WAL
			walEntry := &wal.WALEntry{
				TxnID:     txn.ID,
				Operation: op,
				Timestamp: time.Now(),
			}

			if err := walManager.WriteEntry(walEntry); err != nil {
				t.Fatalf("Failed to write WAL entry: %v", err)
			}

			// Convert WAL operation to transaction operation
			txnOp := transaction.Operation{
				Type:     transaction.OperationType(op.Type - 1), // WAL ops start at 1, txn ops start at 0
				Key:      op.Key,
				Value:    op.Value,
				OldValue: op.OldValue,
			}

			// Add operation to transaction
			if err := txnManager.AddOperation(txn, txnOp); err != nil {
				t.Fatalf("Failed to add operation to transaction: %v", err)
			}

			// Acquire appropriate locks
			lockType := transaction.ExclusiveLock
			if op.Type == wal.OpInsert {
				lockType = transaction.ExclusiveLock
			}

			if err := txnManager.AcquireLock(txn, op.Key, lockType); err != nil {
				t.Fatalf("Failed to acquire lock: %v", err)
			}
		}

		// Log commit to WAL
		commitEntry := &wal.WALEntry{
			TxnID:     txn.ID,
			Operation: wal.Operation{Type: wal.OpCommit},
			Timestamp: time.Now(),
		}

		if err := walManager.WriteEntry(commitEntry); err != nil {
			t.Fatalf("Failed to write commit entry: %v", err)
		}

		// Commit transaction
		if err := txnManager.Commit(txn); err != nil {
			t.Fatalf("Failed to commit transaction: %v", err)
		}

		// Verify WAL entries
		if walManager.GetCurrentLSN() != 4 { // 3 operations + 1 commit
			t.Errorf("Expected LSN 4, got %d", walManager.GetCurrentLSN())
		}

		// Verify transaction operations
		if len(txn.Operations) != 3 {
			t.Errorf("Expected 3 operations in transaction, got %d", len(txn.Operations))
		}
	})

	t.Run("MultipleTransactionsWithConflicts", func(t *testing.T) {
		// Begin two transactions
		txn1, err := txnSystem.BeginTransaction(transaction.ReadCommitted)
		if err != nil {
			t.Fatalf("Failed to begin transaction 1: %v", err)
		}

		txn2, err := txnSystem.BeginTransaction(transaction.ReadCommitted)
		if err != nil {
			t.Fatalf("Failed to begin transaction 2: %v", err)
		}

		// Transaction 1: Update key3
		op1 := wal.Operation{Type: wal.OpUpdate, Key: "key3", Value: []byte("value3_txn1"), OldValue: []byte("original")}
		walEntry1 := &wal.WALEntry{
			TxnID:     txn1.ID,
			Operation: op1,
			Timestamp: time.Now(),
		}

		if err := walManager.WriteEntry(walEntry1); err != nil {
			t.Fatalf("Failed to write WAL entry for txn1: %v", err)
		}

		if err := txnManager.AcquireLock(txn1, "key3", transaction.ExclusiveLock); err != nil {
			t.Fatalf("Failed to acquire lock for txn1: %v", err)
		}

		// Transaction 2: Try to update same key (should be blocked or fail)
		op2 := wal.Operation{Type: wal.OpUpdate, Key: "key3", Value: []byte("value3_txn2"), OldValue: []byte("original")}
		walEntry2 := &wal.WALEntry{
			TxnID:     txn2.ID,
			Operation: op2,
			Timestamp: time.Now(),
		}

		if err := walManager.WriteEntry(walEntry2); err != nil {
			t.Fatalf("Failed to write WAL entry for txn2: %v", err)
		}

		// This should fail due to lock conflict
		err = txnManager.AcquireLock(txn2, "key3", transaction.ExclusiveLock)
		if err == nil {
			t.Error("Expected lock acquisition to fail due to conflict")
		}

		// Commit transaction 1
		commitEntry1 := &wal.WALEntry{
			TxnID:     txn1.ID,
			Operation: wal.Operation{Type: wal.OpCommit},
			Timestamp: time.Now(),
		}

		if err := walManager.WriteEntry(commitEntry1); err != nil {
			t.Fatalf("Failed to write commit entry for txn1: %v", err)
		}

		if err := txnManager.Commit(txn1); err != nil {
			t.Fatalf("Failed to commit transaction 1: %v", err)
		}

		// Abort transaction 2
		abortEntry2 := &wal.WALEntry{
			TxnID:     txn2.ID,
			Operation: wal.Operation{Type: wal.OpAbort},
			Timestamp: time.Now(),
		}

		if err := walManager.WriteEntry(abortEntry2); err != nil {
			t.Fatalf("Failed to write abort entry for txn2: %v", err)
		}

		if err := txnManager.Abort(txn2); err != nil {
			t.Fatalf("Failed to abort transaction 2: %v", err)
		}
	})

	t.Run("TransactionRecoveryScenario", func(t *testing.T) {
		// Begin transaction but don't commit (simulate crash)
		txn, err := txnSystem.BeginTransaction(transaction.ReadCommitted)
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v", err)
		}

		// Perform operations
		operations := []wal.Operation{
			{Type: wal.OpInsert, Key: "recovery_key1", Value: []byte("recovery_value1")},
			{Type: wal.OpInsert, Key: "recovery_key2", Value: []byte("recovery_value2")},
		}

		for _, op := range operations {
			walEntry := &wal.WALEntry{
				TxnID:     txn.ID,
				Operation: op,
				Timestamp: time.Now(),
			}

			if err := walManager.WriteEntry(walEntry); err != nil {
				t.Fatalf("Failed to write WAL entry: %v", err)
			}

			// Convert WAL operation to transaction operation
			txnOp := transaction.Operation{
				Type:     transaction.OperationType(op.Type - 1), // WAL ops start at 1, txn ops start at 0
				Key:      op.Key,
				Value:    op.Value,
				OldValue: op.OldValue,
			}

			if err := txnManager.AddOperation(txn, txnOp); err != nil {
				t.Fatalf("Failed to add operation to transaction: %v", err)
			}
		}

		// Force sync WAL
		if err := walManager.Sync(); err != nil {
			t.Fatalf("Failed to sync WAL: %v", err)
		}

		// Simulate crash - don't commit transaction
		initialLSN := walManager.GetCurrentLSN()

		// Create recovery engine
		recoveryEngine, err := wal.NewRecoveryEngine(tempDir)
		if err != nil {
			t.Fatalf("Failed to create recovery engine: %v", err)
		}

		// Analyze WAL for recovery
		plan, err := recoveryEngine.AnalyzeWAL()
		if err != nil {
			t.Fatalf("Failed to analyze WAL: %v", err)
		}

		// Should find incomplete transaction
		if txnInfo, exists := plan.Transactions[txn.ID]; exists {
			if txnInfo.Status != wal.TxnStatusActive {
				t.Errorf("Expected incomplete transaction to be active, got %v", txnInfo.Status)
			}
		} else {
			t.Error("Incomplete transaction not found in recovery plan")
		}

		// Should have no operations to replay (incomplete transaction)
		replayableOps := 0
		for _, op := range plan.Operations {
			if op.TxnID == txn.ID {
				replayableOps++
			}
		}

		if replayableOps != 0 {
			t.Errorf("Expected 0 replayable operations for incomplete transaction, got %d", replayableOps)
		}

		// Verify WAL integrity
		if err := recoveryEngine.ValidateWALIntegrity(); err != nil {
			t.Fatalf("WAL integrity validation failed: %v", err)
		}

		// Verify LSN consistency
		if plan.EndLSN != initialLSN {
			t.Errorf("Expected end LSN %d, got %d", initialLSN, plan.EndLSN)
		}
	})

	t.Run("ConcurrentTransactionsWithWAL", func(t *testing.T) {
		const numTransactions = 5
		const operationsPerTransaction = 3

		done := make(chan bool, numTransactions)

		// Start multiple concurrent transactions
		for i := 0; i < numTransactions; i++ {
			go func(txnIndex int) {
				defer func() { done <- true }()

				// Begin transaction
				txn, err := txnSystem.BeginTransaction(transaction.ReadCommitted)
				if err != nil {
					t.Errorf("Transaction %d: Failed to begin: %v", txnIndex, err)
					return
				}

				// Perform operations
				for j := 0; j < operationsPerTransaction; j++ {
					key := fmt.Sprintf("concurrent_key_%d_%d", txnIndex, j)
					value := []byte(fmt.Sprintf("concurrent_value_%d_%d", txnIndex, j))

					op := wal.Operation{Type: wal.OpInsert, Key: key, Value: value}
					walEntry := &wal.WALEntry{
						TxnID:     txn.ID,
						Operation: op,
						Timestamp: time.Now(),
					}

					if err := walManager.WriteEntry(walEntry); err != nil {
						t.Errorf("Transaction %d: Failed to write WAL entry: %v", txnIndex, err)
						return
					}

					// Convert WAL operation to transaction operation
					txnOp := transaction.Operation{
						Type:     transaction.OperationType(op.Type - 1), // WAL ops start at 1, txn ops start at 0
						Key:      op.Key,
						Value:    op.Value,
						OldValue: op.OldValue,
					}

					if err := txnManager.AddOperation(txn, txnOp); err != nil {
						t.Errorf("Transaction %d: Failed to add operation: %v", txnIndex, err)
						return
					}

					if err := txnManager.AcquireLock(txn, key, transaction.ExclusiveLock); err != nil {
						t.Errorf("Transaction %d: Failed to acquire lock: %v", txnIndex, err)
						return
					}
				}

				// Commit transaction
				commitEntry := &wal.WALEntry{
					TxnID:     txn.ID,
					Operation: wal.Operation{Type: wal.OpCommit},
					Timestamp: time.Now(),
				}

				if err := walManager.WriteEntry(commitEntry); err != nil {
					t.Errorf("Transaction %d: Failed to write commit entry: %v", txnIndex, err)
					return
				}

				if err := txnManager.Commit(txn); err != nil {
					t.Errorf("Transaction %d: Failed to commit: %v", txnIndex, err)
					return
				}
			}(i)
		}

		// Wait for all transactions to complete
		for i := 0; i < numTransactions; i++ {
			<-done
		}

		// Verify final state
		expectedLSN := uint64(numTransactions * (operationsPerTransaction + 1)) // +1 for commit
		actualLSN := walManager.GetCurrentLSN()

		// Allow for some variance due to the previous tests
		if actualLSN < expectedLSN {
			t.Errorf("Expected at least LSN %d, got %d", expectedLSN, actualLSN)
		}

		// Verify no active transactions
		if txnSystem.GetTransactionCount() != 0 {
			t.Errorf("Expected 0 active transactions, got %d", txnSystem.GetTransactionCount())
		}
	})

	t.Log("WAL-Transaction integration test completed successfully")
}

// TestTransactionIsolationWithWAL tests transaction isolation with WAL logging
func TestTransactionIsolationWithWAL(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "txn_isolation_wal_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create WAL manager
	walConfig := wal.DefaultWALFileManagerConfig()
	walConfig.WALDir = tempDir
	walManager, err := wal.NewWALFileManager(walConfig)
	if err != nil {
		t.Fatalf("Failed to create WAL manager: %v", err)
	}
	defer walManager.Close()

	// Create transaction system
	lockManager := transaction.NewLockManager(5 * time.Second)
	defer lockManager.Close()

	txnSystemConfig := transaction.DefaultTransactionSystemConfig()
	txnSystem := transaction.NewTransactionSystem(txnSystemConfig)
	if err := txnSystem.Start(); err != nil {
		t.Fatalf("Failed to start transaction system: %v", err)
	}
	defer txnSystem.Stop()

	isolationLevels := []transaction.IsolationLevel{
		transaction.ReadCommitted,
		transaction.RepeatableRead,
		transaction.Serializable,
	}

	for _, isolation := range isolationLevels {
		t.Run(isolation.String(), func(t *testing.T) {
			// Begin transaction with specific isolation level
			txn, err := txnSystem.BeginTransaction(isolation)
			if err != nil {
				t.Fatalf("Failed to begin transaction with %s: %v", isolation, err)
			}

			// Perform operations
			key := fmt.Sprintf("isolation_test_%s", isolation.String())
			value := []byte(fmt.Sprintf("value_for_%s", isolation.String()))

			op := wal.Operation{Type: wal.OpInsert, Key: key, Value: value}
			walEntry := &wal.WALEntry{
				TxnID:     txn.ID,
				Operation: op,
				Timestamp: time.Now(),
			}

			if err := walManager.WriteEntry(walEntry); err != nil {
				t.Fatalf("Failed to write WAL entry for %s: %v", isolation, err)
			}

			// Verify isolation level is preserved
			if txn.Isolation != isolation {
				t.Errorf("Transaction isolation level mismatch: expected %s, got %s",
					isolation, txn.Isolation)
			}

			// Commit transaction
			commitEntry := &wal.WALEntry{
				TxnID:     txn.ID,
				Operation: wal.Operation{Type: wal.OpCommit},
				Timestamp: time.Now(),
			}

			if err := walManager.WriteEntry(commitEntry); err != nil {
				t.Fatalf("Failed to write commit entry for %s: %v", isolation, err)
			}

			if err := txnSystem.CommitTransaction(txn); err != nil {
				t.Fatalf("Failed to commit transaction with %s: %v", isolation, err)
			}
		})
	}

	t.Log("Transaction isolation with WAL test completed successfully")
}
