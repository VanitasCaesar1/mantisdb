package transaction

import (
	"testing"
	"time"
)

func TestBasicTransactionFlow(t *testing.T) {
	// Create transaction system with shorter timeouts for testing
	config := &TransactionSystemConfig{
		LockTimeout:               1 * time.Second,
		DeadlockDetectionInterval: 100 * time.Millisecond,
		VictimSelectionStrategy:   YoungestTransaction,
	}
	system := NewTransactionSystem(config)

	if err := system.Start(); err != nil {
		t.Fatalf("Failed to start transaction system: %v", err)
	}
	defer system.Stop()

	// Test single transaction
	txn, err := system.BeginTransaction(ReadCommitted)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Insert a key-value pair
	if err := system.Insert(txn, "test_key", []byte("test_value")); err != nil {
		t.Fatalf("Failed to insert: %v", err)
	}

	// Read the value back
	value, err := system.Read(txn, "test_key")
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	if string(value) != "test_value" {
		t.Errorf("Expected 'test_value', got '%s'", string(value))
	}

	// Commit the transaction
	if err := system.CommitTransaction(txn); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Verify no active transactions
	if count := system.GetTransactionCount(); count != 0 {
		t.Errorf("Expected 0 active transactions, got %d", count)
	}
}

func TestTransactionAbortFlow(t *testing.T) {
	config := &TransactionSystemConfig{
		LockTimeout:               1 * time.Second,
		DeadlockDetectionInterval: 100 * time.Millisecond,
		VictimSelectionStrategy:   YoungestTransaction,
	}
	system := NewTransactionSystem(config)

	if err := system.Start(); err != nil {
		t.Fatalf("Failed to start transaction system: %v", err)
	}
	defer system.Stop()

	// Begin transaction
	txn, err := system.BeginTransaction(ReadCommitted)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Insert data
	if err := system.Insert(txn, "abort_key", []byte("abort_value")); err != nil {
		t.Fatalf("Failed to insert: %v", err)
	}

	// Abort the transaction
	if err := system.AbortTransaction(txn); err != nil {
		t.Fatalf("Failed to abort transaction: %v", err)
	}

	// Verify no active transactions
	if count := system.GetTransactionCount(); count != 0 {
		t.Errorf("Expected 0 active transactions after abort, got %d", count)
	}
}

func TestLockManagerBasics(t *testing.T) {
	lockManager := NewLockManager(1 * time.Second)
	defer lockManager.Close()

	// Test shared lock acquisition
	if err := lockManager.AcquireLock(1, "resource1", SharedLock); err != nil {
		t.Fatalf("Failed to acquire shared lock: %v", err)
	}

	// Test another shared lock on same resource
	if err := lockManager.AcquireLock(2, "resource1", SharedLock); err != nil {
		t.Fatalf("Failed to acquire second shared lock: %v", err)
	}

	// Release locks
	if err := lockManager.ReleaseLock(1, "resource1"); err != nil {
		t.Fatalf("Failed to release first lock: %v", err)
	}

	if err := lockManager.ReleaseLock(2, "resource1"); err != nil {
		t.Fatalf("Failed to release second lock: %v", err)
	}

	// Test exclusive lock
	if err := lockManager.AcquireLock(3, "resource2", ExclusiveLock); err != nil {
		t.Fatalf("Failed to acquire exclusive lock: %v", err)
	}

	if err := lockManager.ReleaseLock(3, "resource2"); err != nil {
		t.Fatalf("Failed to release exclusive lock: %v", err)
	}
}
