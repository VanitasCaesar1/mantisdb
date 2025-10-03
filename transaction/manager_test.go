package transaction

import (
	"testing"
	"time"
)

func TestTransactionManager_BasicOperations(t *testing.T) {
	lockManager := NewLockManager(5 * time.Second)
	defer lockManager.Close()

	tm := NewTransactionManager(lockManager)
	defer tm.Close()

	// Test Begin
	txn, err := tm.Begin(ReadCommitted)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	if txn.ID == 0 {
		t.Error("Transaction ID should not be zero")
	}

	if txn.Status != TxnActive {
		t.Errorf("Expected transaction status %v, got %v", TxnActive, txn.Status)
	}

	if txn.Isolation != ReadCommitted {
		t.Errorf("Expected isolation level %v, got %v", ReadCommitted, txn.Isolation)
	}

	// Test GetTransaction
	retrievedTxn, err := tm.GetTransaction(txn.ID)
	if err != nil {
		t.Fatalf("Failed to get transaction: %v", err)
	}

	if retrievedTxn.ID != txn.ID {
		t.Errorf("Retrieved transaction ID mismatch: expected %d, got %d", txn.ID, retrievedTxn.ID)
	}

	// Test Commit
	err = tm.Commit(txn)
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	if txn.Status != TxnCommitted {
		t.Errorf("Expected transaction status %v, got %v", TxnCommitted, txn.Status)
	}

	// Verify transaction is removed from active transactions
	_, err = tm.GetTransaction(txn.ID)
	if err == nil {
		t.Error("Expected error when getting committed transaction")
	}
}

func TestTransactionManager_Abort(t *testing.T) {
	lockManager := NewLockManager(5 * time.Second)
	defer lockManager.Close()

	tm := NewTransactionManager(lockManager)
	defer tm.Close()

	// Begin transaction
	txn, err := tm.Begin(ReadCommitted)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Add some operations
	op := Operation{
		Type:  OpInsert,
		Key:   "test_key",
		Value: []byte("test_value"),
	}
	err = tm.AddOperation(txn, op)
	if err != nil {
		t.Fatalf("Failed to add operation: %v", err)
	}

	// Abort transaction
	err = tm.Abort(txn)
	if err != nil {
		t.Fatalf("Failed to abort transaction: %v", err)
	}

	if txn.Status != TxnAborted {
		t.Errorf("Expected transaction status %v, got %v", TxnAborted, txn.Status)
	}

	// Verify transaction is removed from active transactions
	_, err = tm.GetTransaction(txn.ID)
	if err == nil {
		t.Error("Expected error when getting aborted transaction")
	}
}

func TestTransactionManager_LockAcquisition(t *testing.T) {
	lockManager := NewLockManager(5 * time.Second)
	defer lockManager.Close()

	tm := NewTransactionManager(lockManager)
	defer tm.Close()

	// Begin transaction
	txn, err := tm.Begin(ReadCommitted)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Acquire shared lock
	err = tm.AcquireLock(txn, "resource1", SharedLock)
	if err != nil {
		t.Fatalf("Failed to acquire shared lock: %v", err)
	}

	// Verify lock was added to transaction
	if len(txn.Locks) != 1 {
		t.Errorf("Expected 1 lock, got %d", len(txn.Locks))
	}

	if txn.Locks[0].Resource != "resource1" {
		t.Errorf("Expected resource 'resource1', got '%s'", txn.Locks[0].Resource)
	}

	if txn.Locks[0].Type != SharedLock {
		t.Errorf("Expected lock type %v, got %v", SharedLock, txn.Locks[0].Type)
	}

	// Acquire exclusive lock on different resource
	err = tm.AcquireLock(txn, "resource2", ExclusiveLock)
	if err != nil {
		t.Fatalf("Failed to acquire exclusive lock: %v", err)
	}

	if len(txn.Locks) != 2 {
		t.Errorf("Expected 2 locks, got %d", len(txn.Locks))
	}

	// Commit transaction (should release locks)
	err = tm.Commit(txn)
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}
}

func TestTransactionManager_ConcurrentTransactions(t *testing.T) {
	lockManager := NewLockManager(5 * time.Second)
	defer lockManager.Close()

	tm := NewTransactionManager(lockManager)
	defer tm.Close()

	// Begin multiple transactions
	txn1, err := tm.Begin(ReadCommitted)
	if err != nil {
		t.Fatalf("Failed to begin transaction 1: %v", err)
	}

	txn2, err := tm.Begin(ReadCommitted)
	if err != nil {
		t.Fatalf("Failed to begin transaction 2: %v", err)
	}

	txn3, err := tm.Begin(Serializable)
	if err != nil {
		t.Fatalf("Failed to begin transaction 3: %v", err)
	}

	// Verify transaction count
	activeTransactions := tm.GetActiveTransactions()
	if len(activeTransactions) != 3 {
		t.Errorf("Expected 3 active transactions, got %d", len(activeTransactions))
	}

	count := tm.GetTransactionCount()
	if count != 3 {
		t.Errorf("Expected transaction count 3, got %d", count)
	}

	// Commit transactions
	err = tm.Commit(txn1)
	if err != nil {
		t.Fatalf("Failed to commit transaction 1: %v", err)
	}

	err = tm.Abort(txn2)
	if err != nil {
		t.Fatalf("Failed to abort transaction 2: %v", err)
	}

	err = tm.Commit(txn3)
	if err != nil {
		t.Fatalf("Failed to commit transaction 3: %v", err)
	}

	// Verify all transactions are cleaned up
	if tm.GetTransactionCount() != 0 {
		t.Errorf("Expected 0 active transactions, got %d", tm.GetTransactionCount())
	}
}

func TestTransactionManager_IsolationLevels(t *testing.T) {
	lockManager := NewLockManager(5 * time.Second)
	defer lockManager.Close()

	tm := NewTransactionManager(lockManager)
	defer tm.Close()

	isolationLevels := []IsolationLevel{
		ReadUncommitted,
		ReadCommitted,
		RepeatableRead,
		Serializable,
	}

	for _, isolation := range isolationLevels {
		t.Run(isolation.String(), func(t *testing.T) {
			txn, err := tm.Begin(isolation)
			if err != nil {
				t.Fatalf("Failed to begin transaction with %s: %v", isolation, err)
			}

			if txn.Isolation != isolation {
				t.Errorf("Expected isolation %v, got %v", isolation, txn.Isolation)
			}

			err = tm.Commit(txn)
			if err != nil {
				t.Fatalf("Failed to commit transaction with %s: %v", isolation, err)
			}
		})
	}
}

func TestTransactionManager_ErrorHandling(t *testing.T) {
	lockManager := NewLockManager(5 * time.Second)
	defer lockManager.Close()

	tm := NewTransactionManager(lockManager)
	defer tm.Close()

	// Test operations with nil transaction
	err := tm.Commit(nil)
	if err == nil {
		t.Error("Expected error when committing nil transaction")
	}

	err = tm.Abort(nil)
	if err == nil {
		t.Error("Expected error when aborting nil transaction")
	}

	err = tm.AcquireLock(nil, "resource", SharedLock)
	if err == nil {
		t.Error("Expected error when acquiring lock for nil transaction")
	}

	err = tm.AddOperation(nil, Operation{})
	if err == nil {
		t.Error("Expected error when adding operation to nil transaction")
	}

	// Test operations on non-existent transaction
	_, err = tm.GetTransaction(999999)
	if err == nil {
		t.Error("Expected error when getting non-existent transaction")
	}

	// Test double commit
	txn, err := tm.Begin(ReadCommitted)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	err = tm.Commit(txn)
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	err = tm.Commit(txn)
	if err == nil {
		t.Error("Expected error when committing already committed transaction")
	}

	// Test abort after commit
	txn2, err := tm.Begin(ReadCommitted)
	if err != nil {
		t.Fatalf("Failed to begin transaction 2: %v", err)
	}

	err = tm.Commit(txn2)
	if err != nil {
		t.Fatalf("Failed to commit transaction 2: %v", err)
	}

	err = tm.Abort(txn2)
	if err == nil {
		t.Error("Expected error when aborting committed transaction")
	}
}

func TestTransactionManager_Operations(t *testing.T) {
	lockManager := NewLockManager(5 * time.Second)
	defer lockManager.Close()

	tm := NewTransactionManager(lockManager)
	defer tm.Close()

	txn, err := tm.Begin(ReadCommitted)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Add various operations
	operations := []Operation{
		{Type: OpInsert, Key: "key1", Value: []byte("value1")},
		{Type: OpUpdate, Key: "key1", Value: []byte("value2"), OldValue: []byte("value1")},
		{Type: OpDelete, Key: "key2", OldValue: []byte("old_value")},
	}

	for _, op := range operations {
		err = tm.AddOperation(txn, op)
		if err != nil {
			t.Fatalf("Failed to add operation: %v", err)
		}
	}

	// Verify operations were added
	if len(txn.Operations) != len(operations) {
		t.Errorf("Expected %d operations, got %d", len(operations), len(txn.Operations))
	}

	for i, op := range txn.Operations {
		if op.Type != operations[i].Type {
			t.Errorf("Operation %d type mismatch: expected %v, got %v", i, operations[i].Type, op.Type)
		}
		if op.Key != operations[i].Key {
			t.Errorf("Operation %d key mismatch: expected %s, got %s", i, operations[i].Key, op.Key)
		}
	}

	err = tm.Commit(txn)
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}
}

func TestTransactionManager_DeadlockDetection(t *testing.T) {
	lockManager := NewLockManager(5 * time.Second)
	defer lockManager.Close()

	tm := NewTransactionManager(lockManager)
	defer tm.Close()

	// Test deadlock detection (basic functionality)
	deadlocks := tm.DetectDeadlocks()
	if deadlocks == nil {
		t.Error("DetectDeadlocks should return empty slice, not nil")
	}

	// Test deadlock resolution with non-existent deadlock
	deadlock := DeadlockInfo{
		VictimTxnID: 999999,
		Cycle:       []uint64{999999},
	}

	err := tm.ResolveDeadlock(deadlock)
	if err == nil {
		t.Error("Expected error when resolving deadlock with non-existent transaction")
	}
}

func TestTransactionManager_Close(t *testing.T) {
	lockManager := NewLockManager(5 * time.Second)
	defer lockManager.Close()

	tm := NewTransactionManager(lockManager)

	// Begin some transactions
	txn1, err := tm.Begin(ReadCommitted)
	if err != nil {
		t.Fatalf("Failed to begin transaction 1: %v", err)
	}

	txn2, err := tm.Begin(ReadCommitted)
	if err != nil {
		t.Fatalf("Failed to begin transaction 2: %v", err)
	}

	// Verify transactions are active
	if tm.GetTransactionCount() != 2 {
		t.Errorf("Expected 2 active transactions, got %d", tm.GetTransactionCount())
	}

	// Close transaction manager
	err = tm.Close()
	if err != nil {
		t.Fatalf("Failed to close transaction manager: %v", err)
	}

	// Verify all transactions were aborted
	if txn1.Status != TxnAborted {
		t.Errorf("Expected transaction 1 to be aborted, got %v", txn1.Status)
	}

	if txn2.Status != TxnAborted {
		t.Errorf("Expected transaction 2 to be aborted, got %v", txn2.Status)
	}

	// Test operations after close
	_, err = tm.Begin(ReadCommitted)
	if err == nil {
		t.Error("Expected error when beginning transaction after close")
	}
}
