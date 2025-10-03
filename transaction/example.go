package transaction

import (
	"fmt"
	"log"
	"time"
)

// Example demonstrates the ACID transaction system usage
func Example() {
	// Create transaction system with custom configuration
	config := &TransactionSystemConfig{
		LockTimeout:               10 * time.Second,
		DeadlockDetectionInterval: 2 * time.Second,
		VictimSelectionStrategy:   YoungestTransaction,
	}

	system := NewTransactionSystem(config)

	// Start the system
	if err := system.Start(); err != nil {
		log.Fatalf("Failed to start transaction system: %v", err)
	}
	defer system.Stop()

	fmt.Println("=== ACID Transaction System Example ===")

	// Example 1: Basic CRUD operations
	fmt.Println("\n1. Basic CRUD Operations:")
	demonstrateBasicOperations(system)

	// Example 2: Transaction isolation levels
	fmt.Println("\n2. Transaction Isolation Levels:")
	demonstrateIsolationLevels(system)

	// Example 3: Concurrent transactions
	fmt.Println("\n3. Concurrent Transactions:")
	demonstrateConcurrentTransactions(system)

	// Example 4: Transaction rollback
	fmt.Println("\n4. Transaction Rollback:")
	demonstrateRollback(system)

	// Example 5: Lock management
	fmt.Println("\n5. Lock Management:")
	demonstrateLockManagement(system)

	fmt.Println("\n=== Example Complete ===")
}

func demonstrateBasicOperations(system *TransactionSystem) {
	// Begin a transaction with READ_COMMITTED isolation
	txn, err := system.BeginTransaction(ReadCommitted)
	if err != nil {
		log.Printf("Failed to begin transaction: %v", err)
		return
	}

	fmt.Printf("Started transaction %d with %s isolation\n", txn.ID, txn.Isolation)

	// Insert operation
	if err := system.Insert(txn, "user:1", []byte(`{"name":"Alice","age":30}`)); err != nil {
		log.Printf("Insert failed: %v", err)
		system.AbortTransaction(txn)
		return
	}
	fmt.Println("Inserted user:1")

	// Read operation
	value, err := system.Read(txn, "user:1")
	if err != nil {
		log.Printf("Read failed: %v", err)
		system.AbortTransaction(txn)
		return
	}
	fmt.Printf("Read user:1: %s\n", string(value))

	// Update operation
	if err := system.Write(txn, "user:1", []byte(`{"name":"Alice","age":31}`)); err != nil {
		log.Printf("Update failed: %v", err)
		system.AbortTransaction(txn)
		return
	}
	fmt.Println("Updated user:1")

	// Verify update
	value, err = system.Read(txn, "user:1")
	if err != nil {
		log.Printf("Read after update failed: %v", err)
		system.AbortTransaction(txn)
		return
	}
	fmt.Printf("Read updated user:1: %s\n", string(value))

	// Commit transaction
	if err := system.CommitTransaction(txn); err != nil {
		log.Printf("Commit failed: %v", err)
		return
	}
	fmt.Printf("Transaction %d committed successfully\n", txn.ID)
}

func demonstrateIsolationLevels(system *TransactionSystem) {
	isolationLevels := []IsolationLevel{
		ReadUncommitted,
		ReadCommitted,
		RepeatableRead,
		Serializable,
	}

	for _, isolation := range isolationLevels {
		fmt.Printf("Testing %s isolation level:\n", isolation)

		txn, err := system.BeginTransaction(isolation)
		if err != nil {
			log.Printf("Failed to begin transaction: %v", err)
			continue
		}

		key := fmt.Sprintf("isolation_test_%s", isolation)
		value := []byte(fmt.Sprintf("value_for_%s", isolation))

		if err := system.Insert(txn, key, value); err != nil {
			log.Printf("Insert failed for %s: %v", isolation, err)
			system.AbortTransaction(txn)
			continue
		}

		readValue, err := system.Read(txn, key)
		if err != nil {
			log.Printf("Read failed for %s: %v", isolation, err)
			system.AbortTransaction(txn)
			continue
		}

		fmt.Printf("  Successfully read: %s\n", string(readValue))

		if err := system.CommitTransaction(txn); err != nil {
			log.Printf("Commit failed for %s: %v", isolation, err)
		} else {
			fmt.Printf("  Transaction committed\n")
		}
	}
}

func demonstrateConcurrentTransactions(system *TransactionSystem) {
	// Start two concurrent transactions
	txn1, err := system.BeginTransaction(ReadCommitted)
	if err != nil {
		log.Printf("Failed to begin transaction 1: %v", err)
		return
	}

	txn2, err := system.BeginTransaction(ReadCommitted)
	if err != nil {
		log.Printf("Failed to begin transaction 2: %v", err)
		system.AbortTransaction(txn1)
		return
	}

	fmt.Printf("Started concurrent transactions: %d and %d\n", txn1.ID, txn2.ID)

	// Each transaction works on different keys to avoid conflicts
	if err := system.Insert(txn1, "concurrent:1", []byte("data from txn1")); err != nil {
		log.Printf("Insert in txn1 failed: %v", err)
	} else {
		fmt.Println("Transaction 1 inserted concurrent:1")
	}

	if err := system.Insert(txn2, "concurrent:2", []byte("data from txn2")); err != nil {
		log.Printf("Insert in txn2 failed: %v", err)
	} else {
		fmt.Println("Transaction 2 inserted concurrent:2")
	}

	// Commit both transactions
	if err := system.CommitTransaction(txn1); err != nil {
		log.Printf("Commit txn1 failed: %v", err)
	} else {
		fmt.Printf("Transaction %d committed\n", txn1.ID)
	}

	if err := system.CommitTransaction(txn2); err != nil {
		log.Printf("Commit txn2 failed: %v", err)
	} else {
		fmt.Printf("Transaction %d committed\n", txn2.ID)
	}

	fmt.Printf("Active transactions: %d\n", system.GetTransactionCount())
}

func demonstrateRollback(system *TransactionSystem) {
	txn, err := system.BeginTransaction(ReadCommitted)
	if err != nil {
		log.Printf("Failed to begin transaction: %v", err)
		return
	}

	fmt.Printf("Started transaction %d for rollback demo\n", txn.ID)

	// Insert some data
	if err := system.Insert(txn, "rollback_test", []byte("this will be rolled back")); err != nil {
		log.Printf("Insert failed: %v", err)
		system.AbortTransaction(txn)
		return
	}
	fmt.Println("Inserted data that will be rolled back")

	// Simulate an error condition and rollback
	fmt.Println("Simulating error condition - rolling back transaction")
	if err := system.AbortTransaction(txn); err != nil {
		log.Printf("Rollback failed: %v", err)
		return
	}

	fmt.Printf("Transaction %d rolled back successfully\n", txn.ID)

	// Verify data was rolled back by starting a new transaction
	newTxn, err := system.BeginTransaction(ReadCommitted)
	if err != nil {
		log.Printf("Failed to begin verification transaction: %v", err)
		return
	}

	_, err = system.Read(newTxn, "rollback_test")
	if err != nil {
		fmt.Println("Confirmed: rolled back data is not accessible")
	} else {
		fmt.Println("Warning: rolled back data is still accessible")
	}

	system.AbortTransaction(newTxn)
}

func demonstrateLockManagement(system *TransactionSystem) {
	// Get system statistics
	stats := system.GetSystemStats()
	fmt.Printf("System stats - Active: %d, Blocked: %d, Deadlocks: %d\n",
		stats.ActiveTransactions, stats.BlockedTransactions, stats.DetectedDeadlocks)

	// Demonstrate lock acquisition
	txn, err := system.BeginTransaction(ReadCommitted)
	if err != nil {
		log.Printf("Failed to begin transaction: %v", err)
		return
	}

	fmt.Printf("Demonstrating lock management with transaction %d\n", txn.ID)

	// Insert will acquire exclusive lock
	if err := system.Insert(txn, "lock_demo", []byte("locked data")); err != nil {
		log.Printf("Insert failed: %v", err)
		system.AbortTransaction(txn)
		return
	}
	fmt.Println("Acquired exclusive lock on 'lock_demo'")

	// Read will acquire shared lock (but we already have exclusive)
	_, err = system.Read(txn, "lock_demo")
	if err != nil {
		log.Printf("Read failed: %v", err)
	} else {
		fmt.Println("Read with existing exclusive lock")
	}

	// Show transaction locks
	fmt.Printf("Transaction %d holds %d locks\n", txn.ID, len(txn.Locks))

	// Commit to release locks
	if err := system.CommitTransaction(txn); err != nil {
		log.Printf("Commit failed: %v", err)
	} else {
		fmt.Println("Transaction committed, locks released")
	}
}
