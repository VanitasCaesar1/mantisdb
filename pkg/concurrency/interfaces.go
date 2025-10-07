// Package concurrency provides public interfaces for MantisDB concurrency components
package concurrency

import (
	"context"
	"time"
)

// LockManager defines the interface for lock management
type LockManager interface {
	// Lock operations
	Lock(ctx context.Context, key string, lockType LockType) (Lock, error)
	TryLock(ctx context.Context, key string, lockType LockType, timeout time.Duration) (Lock, bool, error)

	// Lock information
	IsLocked(key string) bool
	GetLockInfo(key string) (*LockInfo, error)

	// Deadlock detection
	DetectDeadlocks(ctx context.Context) ([]DeadlockInfo, error)

	// Lifecycle
	Close() error
}

// Lock represents a held lock
type Lock interface {
	Unlock() error
	Key() string
	Type() LockType
	Owner() string
	AcquiredAt() time.Time
}

// LockType defines the type of lock
type LockType int

const (
	ReadLock LockType = iota
	WriteLock
	ExclusiveLock
)

// LockInfo contains information about a lock
type LockInfo struct {
	Key        string
	Type       LockType
	Owner      string
	AcquiredAt time.Time
	WaitQueue  []string
}

// DeadlockInfo contains information about a detected deadlock
type DeadlockInfo struct {
	Cycle      []string
	Victims    []string
	DetectedAt time.Time
}

// TransactionManager defines the interface for transaction management
type TransactionManager interface {
	Begin(ctx context.Context) (Transaction, error)
	Get(ctx context.Context, txnID string) (Transaction, error)
	Commit(ctx context.Context, txnID string) error
	Rollback(ctx context.Context, txnID string) error

	// Transaction information
	ActiveTransactions() []string
	GetTransactionInfo(txnID string) (*TransactionInfo, error)
}

// Transaction represents a database transaction
type Transaction interface {
	ID() string
	Begin() time.Time
	Commit() error
	Rollback() error
	IsActive() bool

	// Operations within transaction
	Get(key []byte) ([]byte, error)
	Put(key, value []byte) error
	Delete(key []byte) error
}

// TransactionInfo contains information about a transaction
type TransactionInfo struct {
	ID         string
	BeginTime  time.Time
	Status     TransactionStatus
	Operations int
}

// TransactionStatus defines the status of a transaction
type TransactionStatus int

const (
	TxnActive TransactionStatus = iota
	TxnCommitted
	TxnRolledBack
	TxnAborted
)
