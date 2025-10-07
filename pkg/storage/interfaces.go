// Package storage provides public interfaces for MantisDB storage components
package storage

import (
	"context"
	"io"
)

// Engine defines the interface for storage engines
type Engine interface {
	// Basic operations
	Get(ctx context.Context, key []byte) ([]byte, error)
	Put(ctx context.Context, key, value []byte) error
	Delete(ctx context.Context, key []byte) error

	// Batch operations
	Batch(ctx context.Context, ops []Operation) error

	// Iteration
	Iterator(ctx context.Context, prefix []byte) (Iterator, error)

	// Lifecycle
	Open(ctx context.Context, path string) error
	Close() error
	Sync() error
}

// Operation represents a storage operation
type Operation struct {
	Type  OperationType
	Key   []byte
	Value []byte
}

// OperationType defines the type of storage operation
type OperationType int

const (
	OpGet OperationType = iota
	OpPut
	OpDelete
)

// Iterator defines the interface for iterating over storage entries
type Iterator interface {
	Next() bool
	Key() []byte
	Value() []byte
	Error() error
	Close() error
}

// Backup defines the interface for backup operations
type Backup interface {
	Create(ctx context.Context, dest io.Writer) error
	Restore(ctx context.Context, src io.Reader) error
}

// Compactor defines the interface for storage compaction
type Compactor interface {
	Compact(ctx context.Context) error
	NeedsCompaction() bool
}

// Config holds configuration for storage engines
type Config struct {
	DataDir    string
	BufferSize int64
	CacheSize  int64
	UseCGO     bool
	SyncWrites bool
}
