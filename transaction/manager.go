// manager.go - ACID transaction coordinator with deadlock detection
package transaction

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultTransactionManager implements the TransactionManager interface.
// This is the core of our ACID guarantee - it coordinates locks, detects deadlocks,
// and ensures atomic commit/abort. We use a simple 2PL (two-phase locking) protocol
// because it's proven and easy to reason about, not fancy MVCC which adds complexity.
type DefaultTransactionManager struct {
	// nextTxnID is atomic - no mutex needed for ID generation.
	// Atomic increment is faster than mutex for this hot path.
	nextTxnID    uint64
	transactions map[uint64]*Transaction // Active transactions only
	lockManager  LockManager
	// mutex protects the transactions map, not individual transaction state.
	// Each Transaction has its own mutex to reduce contention.
	mutex  sync.RWMutex
	closed bool
}

// NewTransactionManager creates a new transaction manager
func NewTransactionManager(lockManager LockManager) *DefaultTransactionManager {
	return &DefaultTransactionManager{
		nextTxnID:    1,
		transactions: make(map[uint64]*Transaction),
		lockManager:  lockManager,
	}
}

// Begin starts a new transaction with the specified isolation level
func (tm *DefaultTransactionManager) Begin(isolation IsolationLevel) (*Transaction, error) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	if tm.closed {
		return nil, fmt.Errorf("transaction manager is closed")
	}

	// Atomic ID generation - thread-safe without mutex overhead.
	// We never reuse IDs even after wraparound (uint64 is big enough).
	txnID := atomic.AddUint64(&tm.nextTxnID, 1)

	txn := &Transaction{
		ID:         txnID,
		StartTime:  time.Now(),
		Status:     TxnActive,
		Isolation:  isolation,
		Operations: make([]Operation, 0),
		Locks:      make([]Lock, 0),
	}

	tm.transactions[txnID] = txn

	return txn, nil
}

// Commit commits a transaction
func (tm *DefaultTransactionManager) Commit(txn *Transaction) error {
	if txn == nil {
		return fmt.Errorf("transaction is nil")
	}

	txn.mutex.Lock()
	defer txn.mutex.Unlock()

	if txn.Status != TxnActive {
		return fmt.Errorf("transaction %d is not active (status: %s)", txn.ID, txn.Status)
	}

	// TODO: Write operations to WAL *before* marking committed.
	// This is the critical path for durability - if we crash after marking
	// committed but before WAL flush, we lose data. The correct order is:
	// 1. Write to WAL, 2. Fsync WAL, 3. Mark committed, 4. Release locks.

	// Mark as committed only after WAL flush (currently skipped).
	txn.Status = TxnCommitted

	// Release locks immediately after commit - holding them longer hurts concurrency.
	if err := tm.lockManager.ReleaseAllLocks(txn.ID); err != nil {
		// Don't fail commit if lock release fails - the transaction is already
		// committed (durable). Failing here would violate atomicity.
		// Worst case: locks leak until deadlock detector or timeout cleans them.
		fmt.Printf("Warning: failed to release locks for transaction %d: %v\n", txn.ID, err)
	}

	// Remove from active map to free memory.
	// We hold the outer mutex briefly - delete is O(1) so no contention risk.
	tm.mutex.Lock()
	delete(tm.transactions, txn.ID)
	tm.mutex.Unlock()

	return nil
}

// Abort aborts a transaction
func (tm *DefaultTransactionManager) Abort(txn *Transaction) error {
	if txn == nil {
		return fmt.Errorf("transaction is nil")
	}

	txn.mutex.Lock()
	defer txn.mutex.Unlock()

	if txn.Status == TxnCommitted {
		return fmt.Errorf("cannot abort committed transaction %d", txn.ID)
	}

	if txn.Status == TxnAborted {
		return nil // Already aborted
	}

	// TODO: Rollback operations using WAL
	// This will be implemented when WAL system is available

	// Mark transaction as aborted
	txn.Status = TxnAborted

	// Release all locks held by this transaction
	if err := tm.lockManager.ReleaseAllLocks(txn.ID); err != nil {
		// Log error but don't fail the abort
		fmt.Printf("Warning: failed to release locks for transaction %d: %v\n", txn.ID, err)
	}

	// Remove transaction from active transactions
	tm.mutex.Lock()
	delete(tm.transactions, txn.ID)
	tm.mutex.Unlock()

	return nil
}

// GetTransaction retrieves a transaction by ID
func (tm *DefaultTransactionManager) GetTransaction(txnID uint64) (*Transaction, error) {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	txn, exists := tm.transactions[txnID]
	if !exists {
		return nil, fmt.Errorf("transaction %d not found", txnID)
	}

	return txn, nil
}

// AcquireLock acquires a lock for a transaction
func (tm *DefaultTransactionManager) AcquireLock(txn *Transaction, key string, lockType LockType) error {
	if txn == nil {
		return fmt.Errorf("transaction is nil")
	}

	txn.mutex.RLock()
	if txn.Status != TxnActive {
		txn.mutex.RUnlock()
		return fmt.Errorf("transaction %d is not active", txn.ID)
	}
	txn.mutex.RUnlock()

	// Delegate to lock manager which handles deadlock detection.
	// This may block if the lock is held - the lock manager uses timeouts
	// to prevent indefinite waiting and trigger deadlock resolution.
	if err := tm.lockManager.AcquireLock(txn.ID, key, lockType); err != nil {
		return fmt.Errorf("failed to acquire %s lock on %s for transaction %d: %w",
			lockType, key, txn.ID, err)
	}

	// Add lock to transaction's lock list
	txn.mutex.Lock()
	lock := Lock{
		Resource:   key,
		TxnID:      txn.ID,
		Type:       lockType,
		AcquiredAt: time.Now(),
	}
	txn.Locks = append(txn.Locks, lock)
	txn.mutex.Unlock()

	return nil
}

// ReleaseLocks releases all locks held by a transaction
func (tm *DefaultTransactionManager) ReleaseLocks(txn *Transaction) error {
	if txn == nil {
		return fmt.Errorf("transaction is nil")
	}

	return tm.lockManager.ReleaseAllLocks(txn.ID)
}

// DetectDeadlocks detects deadlocks in the system
func (tm *DefaultTransactionManager) DetectDeadlocks() []DeadlockInfo {
	return tm.lockManager.DetectDeadlocks()
}

// ResolveDeadlock resolves a deadlock by aborting the victim transaction
func (tm *DefaultTransactionManager) ResolveDeadlock(deadlock DeadlockInfo) error {
	// Get the victim transaction
	victimTxn, err := tm.GetTransaction(deadlock.VictimTxnID)
	if err != nil {
		return fmt.Errorf("failed to get victim transaction %d: %w", deadlock.VictimTxnID, err)
	}

	// Abort the victim transaction
	if err := tm.Abort(victimTxn); err != nil {
		return fmt.Errorf("failed to abort victim transaction %d: %w", deadlock.VictimTxnID, err)
	}

	return nil
}

// GetActiveTransactions returns all active transactions
func (tm *DefaultTransactionManager) GetActiveTransactions() []*Transaction {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	transactions := make([]*Transaction, 0, len(tm.transactions))
	for _, txn := range tm.transactions {
		transactions = append(transactions, txn)
	}

	return transactions
}

// GetTransactionCount returns the number of active transactions
func (tm *DefaultTransactionManager) GetTransactionCount() int {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	return len(tm.transactions)
}

// Close closes the transaction manager
func (tm *DefaultTransactionManager) Close() error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	if tm.closed {
		return nil
	}

	// Abort all active transactions
	for _, txn := range tm.transactions {
		if err := tm.Abort(txn); err != nil {
			fmt.Printf("Warning: failed to abort transaction %d during shutdown: %v\n", txn.ID, err)
		}
	}

	// Close lock manager
	if err := tm.lockManager.Close(); err != nil {
		return fmt.Errorf("failed to close lock manager: %w", err)
	}

	tm.closed = true
	return nil
}

// AddOperation adds an operation to a transaction
func (tm *DefaultTransactionManager) AddOperation(txn *Transaction, op Operation) error {
	if txn == nil {
		return fmt.Errorf("transaction is nil")
	}

	txn.mutex.Lock()
	defer txn.mutex.Unlock()

	if txn.Status != TxnActive {
		return fmt.Errorf("transaction %d is not active", txn.ID)
	}

	txn.Operations = append(txn.Operations, op)
	return nil
}
