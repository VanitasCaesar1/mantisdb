// storage_interface.go - Core storage abstraction layer
package storage

import "context"

// StorageEngine defines the interface for storage operations.
// We use an interface here (not concrete types) because we have multiple
// implementations: pure Go, CGO-based, and Rust FFI. The interface lets
// us swap engines at runtime without changing application code.
type StorageEngine interface {
	// Init creates/opens the database at dataDir.
	// Must be called before any other operations - we enforce this at
	// the application layer, not with locks, to avoid initialization overhead.
	Init(dataDir string) error

	// Close flushes pending writes and releases file handles.
	// After Close, the engine is unusable - calling other methods will panic.
	Close() error

	// Basic key-value operations
	Put(ctx context.Context, key, value string) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error

	// Batch operations reduce write amplification by grouping ops.
	// These are atomic within a single batch - either all succeed or all fail.
	// We use maps/slices (not variadic args) because benchmarks showed better
	// performance with pre-allocated collections for large batches.
	BatchPut(ctx context.Context, kvPairs map[string]string) error
	BatchGet(ctx context.Context, keys []string) (map[string]string, error)
	BatchDelete(ctx context.Context, keys []string) error

	// NewIterator creates a forward-only cursor for range scans.
	// Prefix filtering happens at the storage layer (not in Go) to avoid
	// unnecessary data transfer across FFI boundaries.
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

// StorageConfig holds configuration for storage engines.
type StorageConfig struct {
	DataDir    string
	BufferSize int64 // Write buffer - larger = better throughput, more memory
	CacheSize  int64 // Read cache - keep hot data in RAM
	UseCGO     bool  // Use C-based engine (faster but less portable)
	// SyncWrites forces fsync on every write. Slower but safer.
	// We default to false and rely on periodic checkpoints for durability
	// because most workloads can tolerate losing a few seconds of data.
	SyncWrites bool
}
