package storage

import "context"

// StorageEngine defines the interface for storage operations
type StorageEngine interface {
	// Initialize the storage engine
	Init(dataDir string) error

	// Close the storage engine
	Close() error

	// Basic key-value operations
	Put(ctx context.Context, key, value string) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error

	// Batch operations
	BatchPut(ctx context.Context, kvPairs map[string]string) error
	BatchGet(ctx context.Context, keys []string) (map[string]string, error)
	BatchDelete(ctx context.Context, keys []string) error

	// Iterator support
	NewIterator(ctx context.Context, prefix string) (Iterator, error)

	// Transaction support
	BeginTransaction(ctx context.Context) (Transaction, error)

	// Health check
	HealthCheck(ctx context.Context) error
}

// Iterator provides sequential access to key-value pairs
type Iterator interface {
	// Next advances the iterator and returns true if a key-value pair is available
	Next() bool

	// Key returns the current key
	Key() string

	// Value returns the current value
	Value() string

	// Error returns any error encountered during iteration
	Error() error

	// Close releases iterator resources
	Close() error
}

// Transaction provides atomic operations
type Transaction interface {
	// Put adds a key-value pair to the transaction
	Put(key, value string) error

	// Get retrieves a value within the transaction context
	Get(key string) (string, error)

	// Delete marks a key for deletion in the transaction
	Delete(key string) error

	// Commit applies all transaction operations atomically
	Commit() error

	// Rollback discards all transaction operations
	Rollback() error
}

// StorageConfig holds configuration for storage engines
type StorageConfig struct {
	DataDir    string
	BufferSize int64
	CacheSize  int64
	UseCGO     bool
	SyncWrites bool
}
