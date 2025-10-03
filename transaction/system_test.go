package transaction

import (
	"testing"
	"time"
)

func TestTransactionSystemBasicOperations(t *testing.T) {
	// Create transaction system
	config := DefaultTransactionSystemConfig()
	system := NewTransactionSystem(config)

	// Start the system
	if err := system.Start(); err != nil {
		t.Fatalf("Failed to start transaction system: %v", err)
	}
	defer system.Stop()

	// Test basic transaction lifecycle
	txn, err := system.BeginTransaction(ReadCommitted)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Test insert operation
	if err := system.Insert(txn, "key1", []byte("value1")); err != nil {
		t.Fatalf("Failed to insert: %v", err)
	}

	// Test read operation
	value, err := system.Read(txn, "key1")
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	if string(value) != "value1" {
		t.Errorf("Expected 'value1', got '%s'", string(value))
	}

	// Test update operation
	if err := system.Write(txn, "key1", []byte("value2")); err != nil {
		t.Fatalf("Failed to update: %v", err)
	}

	// Verify update
	value, err = system.Read(txn, "key1")
	if err != nil {
		t.Fatalf("Failed to read after update: %v", err)
	}

	if string(value) != "value2" {
		t.Errorf("Expected 'value2', got '%s'", string(value))
	}

	// Test commit
	if err := system.CommitTransaction(txn); err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	// Verify transaction count
	if count := system.GetTransactionCount(); count != 0 {
		t.Errorf("Expected 0 active transactions, got %d", count)
	}
}

func TestTransactionIsolationLevels(t *testing.T) {
	config := DefaultTransactionSystemConfig()
	system := NewTransactionSystem(config)

	if err := system.Start(); err != nil {
		t.Fatalf("Failed to start transaction system: %v", err)
	}
	defer system.Stop()

	// Test different isolation levels
	isolationLevels := []IsolationLevel{
		ReadUncommitted,
		ReadCommitted,
		RepeatableRead,
		Serializable,
	}

	for _, isolation := range isolationLevels {
		t.Run(isolation.String(), func(t *testing.T) {
			txn, err := system.BeginTransaction(isolation)
			if err != nil {
				t.Fatalf("Failed to begin transaction with %s: %v", isolation, err)
			}

			// Perform basic operations
			key := "test_key_" + isolation.String()
			value := []byte("test_value")

			if err := system.Insert(txn, key, value); err != nil {
				t.Fatalf("Failed to insert with %s: %v", isolation, err)
			}

			readValue, err := system.Read(txn, key)
			if err != nil {
				t.Fatalf("Failed to read with %s: %v", isolation, err)
			}

			if string(readValue) != string(value) {
				t.Errorf("Value mismatch with %s: expected %s, got %s",
					isolation, string(value), string(readValue))
			}

			if err := system.CommitTransaction(txn); err != nil {
				t.Fatalf("Failed to commit transaction with %s: %v", isolation, err)
			}
		})
	}
}

func TestConcurrentTransactions(t *testing.T) {
	config := DefaultTransactionSystemConfig()
	system := NewTransactionSystem(config)

	if err := system.Start(); err != nil {
		t.Fatalf("Failed to start transaction system: %v", err)
	}
	defer system.Stop()

	// Create two concurrent transactions
	txn1, err := system.BeginTransaction(ReadCommitted)
	if err != nil {
		t.Fatalf("Failed to begin transaction 1: %v", err)
	}

	txn2, err := system.BeginTransaction(ReadCommitted)
	if err != nil {
		t.Fatalf("Failed to begin transaction 2: %v", err)
	}

	// Insert different keys in each transaction
	if err := system.Insert(txn1, "key1", []byte("value1")); err != nil {
		t.Fatalf("Failed to insert in txn1: %v", err)
	}

	if err := system.Insert(txn2, "key2", []byte("value2")); err != nil {
		t.Fatalf("Failed to insert in txn2: %v", err)
	}

	// Verify active transaction count
	if count := system.GetTransactionCount(); count != 2 {
		t.Errorf("Expected 2 active transactions, got %d", count)
	}

	// Commit both transactions
	if err := system.CommitTransaction(txn1); err != nil {
		t.Fatalf("Failed to commit txn1: %v", err)
	}

	if err := system.CommitTransaction(txn2); err != nil {
		t.Fatalf("Failed to commit txn2: %v", err)
	}

	// Verify no active transactions
	if count := system.GetTransactionCount(); count != 0 {
		t.Errorf("Expected 0 active transactions, got %d", count)
	}
}

func TestTransactionAbort(t *testing.T) {
	config := DefaultTransactionSystemConfig()
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
	if err := system.Insert(txn, "key1", []byte("value1")); err != nil {
		t.Fatalf("Failed to insert: %v", err)
	}

	// Abort transaction
	if err := system.AbortTransaction(txn); err != nil {
		t.Fatalf("Failed to abort transaction: %v", err)
	}

	// Verify transaction is no longer active
	if count := system.GetTransactionCount(); count != 0 {
		t.Errorf("Expected 0 active transactions after abort, got %d", count)
	}

	// Start new transaction and verify data was rolled back
	txn2, err := system.BeginTransaction(ReadCommitted)
	if err != nil {
		t.Fatalf("Failed to begin second transaction: %v", err)
	}

	_, err = system.Read(txn2, "key1")
	if err == nil {
		t.Error("Expected key to not exist after abort, but read succeeded")
	}

	system.AbortTransaction(txn2)
}

func TestLockManager(t *testing.T) {
	lockManager := NewLockManager(5 * time.Second)
	defer lockManager.Close()

	// Test basic lock acquisition
	if err := lockManager.AcquireLock(1, "resource1", SharedLock); err != nil {
		t.Fatalf("Failed to acquire shared lock: %v", err)
	}

	// Test compatible lock acquisition
	if err := lockManager.AcquireLock(2, "resource1", SharedLock); err != nil {
		t.Fatalf("Failed to acquire compatible shared lock: %v", err)
	}

	// Test lock release
	if err := lockManager.ReleaseLock(1, "resource1"); err != nil {
		t.Fatalf("Failed to release lock: %v", err)
	}

	// Test exclusive lock acquisition
	if err := lockManager.AcquireLock(3, "resource2", ExclusiveLock); err != nil {
		t.Fatalf("Failed to acquire exclusive lock: %v", err)
	}

	// Test release all locks
	if err := lockManager.ReleaseAllLocks(2); err != nil {
		t.Fatalf("Failed to release all locks for txn 2: %v", err)
	}

	if err := lockManager.ReleaseAllLocks(3); err != nil {
		t.Fatalf("Failed to release all locks for txn 3: %v", err)
	}
}

func TestDeadlockDetection(t *testing.T) {
	config := DefaultTransactionSystemConfig()
	config.DeadlockDetectionInterval = 100 * time.Millisecond
	system := NewTransactionSystem(config)

	if err := system.Start(); err != nil {
		t.Fatalf("Failed to start transaction system: %v", err)
	}
	defer system.Stop()

	// This test would require more complex setup to create actual deadlocks
	// For now, just verify the deadlock detection components work
	deadlocks := system.lockManager.DetectDeadlocks()
	if deadlocks == nil {
		t.Error("DetectDeadlocks should return empty slice, not nil")
	}

	// Test wait-for graph construction
	graph := system.lockManager.BuildWaitForGraph()
	if graph == nil {
		t.Error("BuildWaitForGraph should not return nil")
	}
}
