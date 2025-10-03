package transaction

import (
	"fmt"
	"sync"
	"time"
)

// ReadView represents a consistent snapshot for transaction isolation
type ReadView struct {
	TxnID         uint64
	StartTime     time.Time
	ActiveTxns    map[uint64]bool      // Set of active transaction IDs at snapshot time
	CommittedTxns map[uint64]time.Time // Committed transactions and their commit times
}

// VersionedValue represents a versioned value for MVCC
type VersionedValue struct {
	Value     []byte
	TxnID     uint64    // Transaction that created this version
	Timestamp time.Time // When this version was created
	Deleted   bool      // Whether this version represents a deletion
}

// MVCCStorage provides multi-version concurrency control storage
type MVCCStorage struct {
	data  map[string][]*VersionedValue // key -> list of versions (newest first)
	mutex sync.RWMutex
}

// NewMVCCStorage creates a new MVCC storage
func NewMVCCStorage() *MVCCStorage {
	return &MVCCStorage{
		data: make(map[string][]*VersionedValue),
	}
}

// IsolationManager handles transaction isolation levels
type IsolationManager struct {
	mvccStorage *MVCCStorage
	readViews   map[uint64]*ReadView // txnID -> ReadView
	mutex       sync.RWMutex
}

// NewIsolationManager creates a new isolation manager
func NewIsolationManager() *IsolationManager {
	return &IsolationManager{
		mvccStorage: NewMVCCStorage(),
		readViews:   make(map[uint64]*ReadView),
	}
}

// CreateReadView creates a read view for a transaction
func (im *IsolationManager) CreateReadView(txn *Transaction, activeTxns []*Transaction) *ReadView {
	im.mutex.Lock()
	defer im.mutex.Unlock()

	readView := &ReadView{
		TxnID:         txn.ID,
		StartTime:     txn.StartTime,
		ActiveTxns:    make(map[uint64]bool),
		CommittedTxns: make(map[uint64]time.Time),
	}

	// Record active transactions at the time of snapshot
	for _, activeTxn := range activeTxns {
		if activeTxn.ID != txn.ID {
			readView.ActiveTxns[activeTxn.ID] = true
		}
	}

	im.readViews[txn.ID] = readView
	return readView
}

// RemoveReadView removes a read view when transaction completes
func (im *IsolationManager) RemoveReadView(txnID uint64) {
	im.mutex.Lock()
	defer im.mutex.Unlock()

	delete(im.readViews, txnID)
}

// Read performs a read operation with appropriate isolation level
func (im *IsolationManager) Read(txn *Transaction, key string) ([]byte, error) {
	switch txn.Isolation {
	case ReadUncommitted:
		return im.readUncommitted(key)
	case ReadCommitted:
		return im.readCommitted(txn, key)
	case RepeatableRead:
		return im.repeatableRead(txn, key)
	case Serializable:
		return im.serializableRead(txn, key)
	default:
		return nil, fmt.Errorf("unsupported isolation level: %s", txn.Isolation)
	}
}

// Write performs a write operation with appropriate isolation level
func (im *IsolationManager) Write(txn *Transaction, key string, value []byte) error {
	switch txn.Isolation {
	case ReadUncommitted, ReadCommitted:
		return im.writeWithLocking(txn, key, value)
	case RepeatableRead:
		return im.writeWithMVCC(txn, key, value)
	case Serializable:
		return im.serializableWrite(txn, key, value)
	default:
		return fmt.Errorf("unsupported isolation level: %s", txn.Isolation)
	}
}

// readUncommitted reads the latest version without any isolation
func (im *IsolationManager) readUncommitted(key string) ([]byte, error) {
	im.mvccStorage.mutex.RLock()
	defer im.mvccStorage.mutex.RUnlock()

	versions, exists := im.mvccStorage.data[key]
	if !exists || len(versions) == 0 {
		return nil, fmt.Errorf("key not found: %s", key)
	}

	// Return the latest version
	latest := versions[0]
	if latest.Deleted {
		return nil, fmt.Errorf("key not found: %s", key)
	}

	return latest.Value, nil
}

// readCommitted reads committed data only
func (im *IsolationManager) readCommitted(txn *Transaction, key string) ([]byte, error) {
	im.mvccStorage.mutex.RLock()
	defer im.mvccStorage.mutex.RUnlock()

	versions, exists := im.mvccStorage.data[key]
	if !exists || len(versions) == 0 {
		return nil, fmt.Errorf("key not found: %s", key)
	}

	// Find the latest committed version
	for _, version := range versions {
		if version.TxnID == txn.ID {
			// Own transaction's changes are visible
			if version.Deleted {
				return nil, fmt.Errorf("key not found: %s", key)
			}
			return version.Value, nil
		}

		// Check if this version is from a committed transaction
		if im.isCommitted(version.TxnID) {
			if version.Deleted {
				return nil, fmt.Errorf("key not found: %s", key)
			}
			return version.Value, nil
		}
	}

	return nil, fmt.Errorf("key not found: %s", key)
}

// repeatableRead provides repeatable read isolation using MVCC
func (im *IsolationManager) repeatableRead(txn *Transaction, key string) ([]byte, error) {
	im.mutex.RLock()
	readView, exists := im.readViews[txn.ID]
	im.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no read view found for transaction %d", txn.ID)
	}

	im.mvccStorage.mutex.RLock()
	defer im.mvccStorage.mutex.RUnlock()

	versions, exists := im.mvccStorage.data[key]
	if !exists || len(versions) == 0 {
		return nil, fmt.Errorf("key not found: %s", key)
	}

	// Find the appropriate version based on read view
	for _, version := range versions {
		if version.TxnID == txn.ID {
			// Own transaction's changes are visible
			if version.Deleted {
				return nil, fmt.Errorf("key not found: %s", key)
			}
			return version.Value, nil
		}

		// Check if this version is visible in the read view
		if im.isVisibleInReadView(version, readView) {
			if version.Deleted {
				return nil, fmt.Errorf("key not found: %s", key)
			}
			return version.Value, nil
		}
	}

	return nil, fmt.Errorf("key not found: %s", key)
}

// serializableRead provides serializable isolation with conflict detection
func (im *IsolationManager) serializableRead(txn *Transaction, key string) ([]byte, error) {
	// First perform repeatable read
	value, err := im.repeatableRead(txn, key)
	if err != nil {
		return nil, err
	}

	// Record read for conflict detection
	im.recordRead(txn, key)

	return value, nil
}

// writeWithLocking performs write with locking (for READ_UNCOMMITTED and READ_COMMITTED)
func (im *IsolationManager) writeWithLocking(txn *Transaction, key string, value []byte) error {
	im.mvccStorage.mutex.Lock()
	defer im.mvccStorage.mutex.Unlock()

	// Create new version
	version := &VersionedValue{
		Value:     value,
		TxnID:     txn.ID,
		Timestamp: time.Now(),
		Deleted:   false,
	}

	// Add to versions list (newest first)
	if im.mvccStorage.data[key] == nil {
		im.mvccStorage.data[key] = make([]*VersionedValue, 0)
	}

	im.mvccStorage.data[key] = append([]*VersionedValue{version}, im.mvccStorage.data[key]...)

	return nil
}

// writeWithMVCC performs write with MVCC (for REPEATABLE_READ)
func (im *IsolationManager) writeWithMVCC(txn *Transaction, key string, value []byte) error {
	// Check for write-write conflicts
	if err := im.checkWriteConflict(txn, key); err != nil {
		return err
	}

	return im.writeWithLocking(txn, key, value)
}

// serializableWrite performs write with serializable isolation
func (im *IsolationManager) serializableWrite(txn *Transaction, key string, value []byte) error {
	// Check for read-write conflicts
	if err := im.checkReadWriteConflict(txn, key); err != nil {
		return err
	}

	// Check for write-write conflicts
	if err := im.checkWriteConflict(txn, key); err != nil {
		return err
	}

	// Record write for conflict detection
	im.recordWrite(txn, key)

	return im.writeWithLocking(txn, key, value)
}

// Delete performs a delete operation
func (im *IsolationManager) Delete(txn *Transaction, key string) error {
	im.mvccStorage.mutex.Lock()
	defer im.mvccStorage.mutex.Unlock()

	// Create delete marker
	version := &VersionedValue{
		Value:     nil,
		TxnID:     txn.ID,
		Timestamp: time.Now(),
		Deleted:   true,
	}

	// Add to versions list (newest first)
	if im.mvccStorage.data[key] == nil {
		im.mvccStorage.data[key] = make([]*VersionedValue, 0)
	}

	im.mvccStorage.data[key] = append([]*VersionedValue{version}, im.mvccStorage.data[key]...)

	return nil
}

// CommitTransaction commits all changes made by a transaction
func (im *IsolationManager) CommitTransaction(txn *Transaction) error {
	im.mutex.Lock()
	defer im.mutex.Unlock()

	// Mark all versions created by this transaction as committed
	commitTime := time.Now()

	im.mvccStorage.mutex.Lock()
	for _, versions := range im.mvccStorage.data {
		for _, version := range versions {
			if version.TxnID == txn.ID {
				// Update commit time in read views
				for _, readView := range im.readViews {
					readView.CommittedTxns[txn.ID] = commitTime
				}
				break
			}
		}
	}
	im.mvccStorage.mutex.Unlock()

	// Remove read view
	delete(im.readViews, txn.ID)

	return nil
}

// AbortTransaction aborts all changes made by a transaction
func (im *IsolationManager) AbortTransaction(txn *Transaction) error {
	im.mvccStorage.mutex.Lock()
	defer im.mvccStorage.mutex.Unlock()

	// Remove all versions created by this transaction
	for key, versions := range im.mvccStorage.data {
		filteredVersions := make([]*VersionedValue, 0)
		for _, version := range versions {
			if version.TxnID != txn.ID {
				filteredVersions = append(filteredVersions, version)
			}
		}

		if len(filteredVersions) == 0 {
			delete(im.mvccStorage.data, key)
		} else {
			im.mvccStorage.data[key] = filteredVersions
		}
	}

	// Remove read view
	im.mutex.Lock()
	delete(im.readViews, txn.ID)
	im.mutex.Unlock()

	return nil
}

// Helper methods

func (im *IsolationManager) isCommitted(txnID uint64) bool {
	// In a real implementation, this would check the transaction status
	// For now, we'll assume all transactions except the current one are committed
	return true
}

func (im *IsolationManager) isVisibleInReadView(version *VersionedValue, readView *ReadView) bool {
	// Version is visible if:
	// 1. It was created before the read view started, OR
	// 2. It was created by a transaction that committed before the read view started

	if version.Timestamp.Before(readView.StartTime) {
		return true
	}

	// Check if the creating transaction was active when read view was created
	if readView.ActiveTxns[version.TxnID] {
		return false // Was active, so not visible
	}

	// Check if it was committed before read view
	if commitTime, exists := readView.CommittedTxns[version.TxnID]; exists {
		return commitTime.Before(readView.StartTime)
	}

	return true
}

func (im *IsolationManager) checkWriteConflict(txn *Transaction, key string) error {
	im.mvccStorage.mutex.RLock()
	defer im.mvccStorage.mutex.RUnlock()

	versions, exists := im.mvccStorage.data[key]
	if !exists {
		return nil
	}

	// Check for concurrent writes
	for _, version := range versions {
		if version.TxnID != txn.ID && version.Timestamp.After(txn.StartTime) {
			// Another transaction wrote to this key after we started
			if !im.isCommitted(version.TxnID) {
				return fmt.Errorf("write-write conflict detected for key %s", key)
			}
		}
	}

	return nil
}

func (im *IsolationManager) checkReadWriteConflict(txn *Transaction, key string) error {
	// In a full implementation, this would check if any transaction
	// that read this key is still active and started before us
	return nil
}

func (im *IsolationManager) recordRead(txn *Transaction, key string) {
	// Record read for conflict detection
	// In a full implementation, this would maintain read sets
}

func (im *IsolationManager) recordWrite(txn *Transaction, key string) {
	// Record write for conflict detection
	// In a full implementation, this would maintain write sets
}
