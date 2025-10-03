package transaction

import (
	"sync"
	"time"
)

// TxnStatus represents the current status of a transaction
type TxnStatus int

const (
	TxnActive TxnStatus = iota
	TxnCommitted
	TxnAborted
	TxnPrepared
)

func (s TxnStatus) String() string {
	switch s {
	case TxnActive:
		return "ACTIVE"
	case TxnCommitted:
		return "COMMITTED"
	case TxnAborted:
		return "ABORTED"
	case TxnPrepared:
		return "PREPARED"
	default:
		return "UNKNOWN"
	}
}

// IsolationLevel defines the isolation level for transactions
type IsolationLevel int

const (
	ReadUncommitted IsolationLevel = iota
	ReadCommitted
	RepeatableRead
	Serializable
)

func (l IsolationLevel) String() string {
	switch l {
	case ReadUncommitted:
		return "READ_UNCOMMITTED"
	case ReadCommitted:
		return "READ_COMMITTED"
	case RepeatableRead:
		return "REPEATABLE_READ"
	case Serializable:
		return "SERIALIZABLE"
	default:
		return "UNKNOWN"
	}
}

// LockType represents the type of lock
type LockType int

const (
	SharedLock LockType = iota
	ExclusiveLock
)

func (l LockType) String() string {
	switch l {
	case SharedLock:
		return "SHARED"
	case ExclusiveLock:
		return "EXCLUSIVE"
	default:
		return "UNKNOWN"
	}
}

// Operation represents a database operation within a transaction
type Operation struct {
	Type     OperationType
	Key      string
	Value    []byte
	OldValue []byte // For rollback purposes
}

// OperationType defines the type of operation
type OperationType int

const (
	OpInsert OperationType = iota
	OpUpdate
	OpDelete
	OpRead
)

func (o OperationType) String() string {
	switch o {
	case OpInsert:
		return "INSERT"
	case OpUpdate:
		return "UPDATE"
	case OpDelete:
		return "DELETE"
	case OpRead:
		return "READ"
	default:
		return "UNKNOWN"
	}
}

// Transaction represents a database transaction
type Transaction struct {
	ID         uint64
	StartTime  time.Time
	Status     TxnStatus
	Isolation  IsolationLevel
	Operations []Operation
	Locks      []Lock
	mutex      sync.RWMutex
}

// Lock represents a lock held by a transaction
type Lock struct {
	Resource    string
	TxnID       uint64
	Type        LockType
	AcquiredAt  time.Time
	WaitingTxns []uint64
}

// LockInfo provides information about locks on a resource
type LockInfo struct {
	Resource    string
	HolderTxnID uint64
	LockType    LockType
	WaitingTxns []uint64
	AcquiredAt  time.Time
}

// DeadlockInfo contains information about a detected deadlock
type DeadlockInfo struct {
	Cycle       []uint64 // Transaction IDs in the deadlock cycle
	VictimTxnID uint64   // Transaction chosen to be aborted
	DetectedAt  time.Time
}

// WaitForGraph represents the wait-for graph for deadlock detection
type WaitForGraph struct {
	Edges map[uint64][]uint64 // txnID -> list of txnIDs it's waiting for
	mutex sync.RWMutex
}

// TransactionManager interface defines the contract for transaction management
type TransactionManager interface {
	// Transaction lifecycle
	Begin(isolation IsolationLevel) (*Transaction, error)
	Commit(txn *Transaction) error
	Abort(txn *Transaction) error
	GetTransaction(txnID uint64) (*Transaction, error)

	// Lock management
	AcquireLock(txn *Transaction, key string, lockType LockType) error
	ReleaseLocks(txn *Transaction) error

	// Deadlock detection
	DetectDeadlocks() []DeadlockInfo
	ResolveDeadlock(deadlock DeadlockInfo) error

	// Status and monitoring
	GetActiveTransactions() []*Transaction
	GetTransactionCount() int
	Close() error
}

// LockManager interface defines the contract for lock management
type LockManager interface {
	// Lock operations
	AcquireLock(txnID uint64, resource string, lockType LockType) error
	ReleaseLock(txnID uint64, resource string) error
	ReleaseAllLocks(txnID uint64) error

	// Deadlock detection
	DetectDeadlocks() []DeadlockInfo
	BuildWaitForGraph() *WaitForGraph

	// Lock information
	GetLockInfo(resource string) *LockInfo
	GetBlockedTransactions() []uint64

	// Cleanup
	Close() error
}
