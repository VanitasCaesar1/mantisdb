package transaction

import (
	"fmt"
	"sync"
	"time"
)

// TransactionSystem integrates all transaction components
type TransactionSystem struct {
	txnManager       *DefaultTransactionManager
	lockManager      *DefaultLockManager
	isolationManager *IsolationManager
	deadlockDetector *DeadlockDetector
	victimSelector   *VictimSelector
	mutex            sync.RWMutex
	closed           bool
}

// TransactionSystemConfig holds configuration for the transaction system
type TransactionSystemConfig struct {
	LockTimeout               time.Duration
	DeadlockDetectionInterval time.Duration
	VictimSelectionStrategy   VictimSelectionStrategy
}

// DefaultTransactionSystemConfig returns default configuration
func DefaultTransactionSystemConfig() *TransactionSystemConfig {
	return &TransactionSystemConfig{
		LockTimeout:               30 * time.Second,
		DeadlockDetectionInterval: 5 * time.Second,
		VictimSelectionStrategy:   YoungestTransaction,
	}
}

// NewTransactionSystem creates a new integrated transaction system
func NewTransactionSystem(config *TransactionSystemConfig) *TransactionSystem {
	if config == nil {
		config = DefaultTransactionSystemConfig()
	}

	// Create components
	lockManager := NewLockManager(config.LockTimeout)
	isolationManager := NewIsolationManager()
	txnManager := NewTransactionManager(lockManager)

	victimSelector := NewVictimSelector(config.VictimSelectionStrategy, txnManager, lockManager)
	deadlockDetector := NewDeadlockDetector(lockManager, txnManager, config.DeadlockDetectionInterval)

	system := &TransactionSystem{
		txnManager:       txnManager,
		lockManager:      lockManager,
		isolationManager: isolationManager,
		deadlockDetector: deadlockDetector,
		victimSelector:   victimSelector,
	}

	return system
}

// Start starts the transaction system
func (ts *TransactionSystem) Start() error {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	if ts.closed {
		return fmt.Errorf("transaction system is closed")
	}

	// Start deadlock detector
	if err := ts.deadlockDetector.Start(); err != nil {
		return fmt.Errorf("failed to start deadlock detector: %w", err)
	}

	return nil
}

// Stop stops the transaction system
func (ts *TransactionSystem) Stop() error {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	if ts.closed {
		return nil
	}

	// Stop deadlock detector
	if err := ts.deadlockDetector.Stop(); err != nil {
		return fmt.Errorf("failed to stop deadlock detector: %w", err)
	}

	// Close transaction manager (which will close lock manager)
	if err := ts.txnManager.Close(); err != nil {
		return fmt.Errorf("failed to close transaction manager: %w", err)
	}

	ts.closed = true
	return nil
}

// BeginTransaction starts a new transaction
func (ts *TransactionSystem) BeginTransaction(isolation IsolationLevel) (*Transaction, error) {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	if ts.closed {
		return nil, fmt.Errorf("transaction system is closed")
	}

	// Begin transaction
	txn, err := ts.txnManager.Begin(isolation)
	if err != nil {
		return nil, err
	}

	// Create read view for isolation levels that need it
	if isolation == RepeatableRead || isolation == Serializable {
		activeTxns := ts.txnManager.GetActiveTransactions()
		ts.isolationManager.CreateReadView(txn, activeTxns)
	}

	return txn, nil
}

// CommitTransaction commits a transaction
func (ts *TransactionSystem) CommitTransaction(txn *Transaction) error {
	if txn == nil {
		return fmt.Errorf("transaction is nil")
	}

	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	if ts.closed {
		return fmt.Errorf("transaction system is closed")
	}

	// Commit in isolation manager first
	if err := ts.isolationManager.CommitTransaction(txn); err != nil {
		return fmt.Errorf("failed to commit in isolation manager: %w", err)
	}

	// Then commit in transaction manager
	if err := ts.txnManager.Commit(txn); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// AbortTransaction aborts a transaction
func (ts *TransactionSystem) AbortTransaction(txn *Transaction) error {
	if txn == nil {
		return fmt.Errorf("transaction is nil")
	}

	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	if ts.closed {
		return fmt.Errorf("transaction system is closed")
	}

	// Abort in isolation manager first
	if err := ts.isolationManager.AbortTransaction(txn); err != nil {
		return fmt.Errorf("failed to abort in isolation manager: %w", err)
	}

	// Then abort in transaction manager
	if err := ts.txnManager.Abort(txn); err != nil {
		return fmt.Errorf("failed to abort transaction: %w", err)
	}

	return nil
}

// Read performs a read operation within a transaction
func (ts *TransactionSystem) Read(txn *Transaction, key string) ([]byte, error) {
	if txn == nil {
		return nil, fmt.Errorf("transaction is nil")
	}

	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	if ts.closed {
		return nil, fmt.Errorf("transaction system is closed")
	}

	// Acquire appropriate lock based on isolation level
	var lockType LockType
	switch txn.Isolation {
	case ReadUncommitted:
		// No locking needed
	case ReadCommitted, RepeatableRead:
		lockType = SharedLock
	case Serializable:
		lockType = SharedLock
	}

	if lockType != 0 {
		if err := ts.txnManager.AcquireLock(txn, key, lockType); err != nil {
			return nil, fmt.Errorf("failed to acquire read lock: %w", err)
		}
	}

	// Perform read with appropriate isolation
	value, err := ts.isolationManager.Read(txn, key)
	if err != nil {
		return nil, err
	}

	// Record operation
	op := Operation{
		Type: OpRead,
		Key:  key,
	}
	ts.txnManager.AddOperation(txn, op)

	return value, nil
}

// Write performs a write operation within a transaction
func (ts *TransactionSystem) Write(txn *Transaction, key string, value []byte) error {
	if txn == nil {
		return fmt.Errorf("transaction is nil")
	}

	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	if ts.closed {
		return fmt.Errorf("transaction system is closed")
	}

	// Acquire exclusive lock for write
	if err := ts.txnManager.AcquireLock(txn, key, ExclusiveLock); err != nil {
		return fmt.Errorf("failed to acquire write lock: %w", err)
	}

	// Get old value for rollback
	oldValue, _ := ts.isolationManager.Read(txn, key)

	// Perform write with appropriate isolation
	if err := ts.isolationManager.Write(txn, key, value); err != nil {
		return err
	}

	// Record operation
	op := Operation{
		Type:     OpUpdate,
		Key:      key,
		Value:    value,
		OldValue: oldValue,
	}
	ts.txnManager.AddOperation(txn, op)

	return nil
}

// Insert performs an insert operation within a transaction
func (ts *TransactionSystem) Insert(txn *Transaction, key string, value []byte) error {
	if txn == nil {
		return fmt.Errorf("transaction is nil")
	}

	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	if ts.closed {
		return fmt.Errorf("transaction system is closed")
	}

	// Check if key already exists
	_, err := ts.isolationManager.Read(txn, key)
	if err == nil {
		return fmt.Errorf("key already exists: %s", key)
	}

	// Acquire exclusive lock for insert
	if err := ts.txnManager.AcquireLock(txn, key, ExclusiveLock); err != nil {
		return fmt.Errorf("failed to acquire insert lock: %w", err)
	}

	// Perform write
	if err := ts.isolationManager.Write(txn, key, value); err != nil {
		return err
	}

	// Record operation
	op := Operation{
		Type:  OpInsert,
		Key:   key,
		Value: value,
	}
	ts.txnManager.AddOperation(txn, op)

	return nil
}

// Delete performs a delete operation within a transaction
func (ts *TransactionSystem) Delete(txn *Transaction, key string) error {
	if txn == nil {
		return fmt.Errorf("transaction is nil")
	}

	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	if ts.closed {
		return fmt.Errorf("transaction system is closed")
	}

	// Acquire exclusive lock for delete
	if err := ts.txnManager.AcquireLock(txn, key, ExclusiveLock); err != nil {
		return fmt.Errorf("failed to acquire delete lock: %w", err)
	}

	// Get old value for rollback
	oldValue, err := ts.isolationManager.Read(txn, key)
	if err != nil {
		return fmt.Errorf("key not found: %s", key)
	}

	// Perform delete
	if err := ts.isolationManager.Delete(txn, key); err != nil {
		return err
	}

	// Record operation
	op := Operation{
		Type:     OpDelete,
		Key:      key,
		OldValue: oldValue,
	}
	ts.txnManager.AddOperation(txn, op)

	return nil
}

// GetTransaction retrieves a transaction by ID
func (ts *TransactionSystem) GetTransaction(txnID uint64) (*Transaction, error) {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	if ts.closed {
		return nil, fmt.Errorf("transaction system is closed")
	}

	return ts.txnManager.GetTransaction(txnID)
}

// GetActiveTransactions returns all active transactions
func (ts *TransactionSystem) GetActiveTransactions() []*Transaction {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	if ts.closed {
		return nil
	}

	return ts.txnManager.GetActiveTransactions()
}

// GetTransactionCount returns the number of active transactions
func (ts *TransactionSystem) GetTransactionCount() int {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	if ts.closed {
		return 0
	}

	return ts.txnManager.GetTransactionCount()
}

// GetSystemStats returns transaction system statistics
func (ts *TransactionSystem) GetSystemStats() *TransactionSystemStats {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	if ts.closed {
		return nil
	}

	activeTxns := ts.txnManager.GetActiveTransactions()
	blockedTxns := ts.lockManager.GetBlockedTransactions()
	deadlocks := ts.lockManager.DetectDeadlocks()

	return &TransactionSystemStats{
		ActiveTransactions:  len(activeTxns),
		BlockedTransactions: len(blockedTxns),
		DetectedDeadlocks:   len(deadlocks),
		Timestamp:           time.Now(),
	}
}

// TransactionSystemStats holds statistics about the transaction system
type TransactionSystemStats struct {
	ActiveTransactions  int
	BlockedTransactions int
	DetectedDeadlocks   int
	Timestamp           time.Time
}
