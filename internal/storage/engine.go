// Package storage provides internal storage engine implementations
package storage

import (
	"context"
	"fmt"
	"sync"

	"mantisDB/pkg/storage"
)

// BaseEngine provides common functionality for storage engines
type BaseEngine struct {
	path   string
	mu     sync.RWMutex
	closed bool
}

// NewBaseEngine creates a new base engine
func NewBaseEngine() *BaseEngine {
	return &BaseEngine{}
}

// Open opens the storage engine
func (e *BaseEngine) Open(ctx context.Context, path string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.path != "" {
		return fmt.Errorf("engine is already open")
	}

	e.path = path
	e.closed = false
	return nil
}

// Close closes the storage engine
func (e *BaseEngine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return nil
	}

	e.closed = true
	return nil
}

// Sync syncs the storage engine
func (e *BaseEngine) Sync() error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.closed {
		return fmt.Errorf("engine is closed")
	}

	return nil
}

// isOpen checks if the engine is open
func (e *BaseEngine) isOpen() bool {
	return e.path != "" && !e.closed
}

// MemoryEngine implements an in-memory storage engine
type MemoryEngine struct {
	*BaseEngine
	data map[string][]byte
}

// NewMemoryEngine creates a new memory-based storage engine
func NewMemoryEngine() storage.Engine {
	return &MemoryEngine{
		BaseEngine: NewBaseEngine(),
		data:       make(map[string][]byte),
	}
}

// Get retrieves a value by key
func (e *MemoryEngine) Get(ctx context.Context, key []byte) ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.isOpen() {
		return nil, fmt.Errorf("engine is not open")
	}

	value, exists := e.data[string(key)]
	if !exists {
		return nil, fmt.Errorf("key not found")
	}

	// Return a copy to prevent external modification
	result := make([]byte, len(value))
	copy(result, value)
	return result, nil
}

// Put stores a key-value pair
func (e *MemoryEngine) Put(ctx context.Context, key, value []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.isOpen() {
		return fmt.Errorf("engine is not open")
	}

	// Store a copy to prevent external modification
	valueCopy := make([]byte, len(value))
	copy(valueCopy, value)
	e.data[string(key)] = valueCopy

	return nil
}

// Delete removes a key-value pair
func (e *MemoryEngine) Delete(ctx context.Context, key []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.isOpen() {
		return fmt.Errorf("engine is not open")
	}

	delete(e.data, string(key))
	return nil
}

// Batch executes multiple operations atomically
func (e *MemoryEngine) Batch(ctx context.Context, ops []storage.Operation) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.isOpen() {
		return fmt.Errorf("engine is not open")
	}

	// Create a backup in case we need to rollback
	backup := make(map[string][]byte)
	for k, v := range e.data {
		valueCopy := make([]byte, len(v))
		copy(valueCopy, v)
		backup[k] = valueCopy
	}

	// Execute operations
	for _, op := range ops {
		switch op.Type {
		case storage.OpGet:
			// Get operations don't modify state
		case storage.OpPut:
			valueCopy := make([]byte, len(op.Value))
			copy(valueCopy, op.Value)
			e.data[string(op.Key)] = valueCopy
		case storage.OpDelete:
			delete(e.data, string(op.Key))
		default:
			// Rollback on unknown operation
			e.data = backup
			return fmt.Errorf("unknown operation type: %v", op.Type)
		}
	}

	return nil
}

// Iterator returns an iterator for the storage engine
func (e *MemoryEngine) Iterator(ctx context.Context, prefix []byte) (storage.Iterator, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.isOpen() {
		return nil, fmt.Errorf("engine is not open")
	}

	return NewMemoryIterator(e.data, prefix), nil
}

// MemoryIterator implements storage.Iterator for in-memory data
type MemoryIterator struct {
	data    map[string][]byte
	prefix  string
	keys    []string
	current int
	err     error
}

// NewMemoryIterator creates a new memory iterator
func NewMemoryIterator(data map[string][]byte, prefix []byte) *MemoryIterator {
	prefixStr := string(prefix)
	var keys []string

	for k := range data {
		if len(prefix) == 0 || len(k) >= len(prefixStr) && k[:len(prefixStr)] == prefixStr {
			keys = append(keys, k)
		}
	}

	return &MemoryIterator{
		data:    data,
		prefix:  prefixStr,
		keys:    keys,
		current: -1,
	}
}

// Next advances the iterator to the next item
func (it *MemoryIterator) Next() bool {
	it.current++
	return it.current < len(it.keys)
}

// Key returns the current key
func (it *MemoryIterator) Key() []byte {
	if it.current < 0 || it.current >= len(it.keys) {
		return nil
	}
	return []byte(it.keys[it.current])
}

// Value returns the current value
func (it *MemoryIterator) Value() []byte {
	if it.current < 0 || it.current >= len(it.keys) {
		return nil
	}

	key := it.keys[it.current]
	value := it.data[key]

	// Return a copy to prevent external modification
	result := make([]byte, len(value))
	copy(result, value)
	return result
}

// Error returns any error that occurred during iteration
func (it *MemoryIterator) Error() error {
	return it.err
}

// Close closes the iterator
func (it *MemoryIterator) Close() error {
	it.keys = nil
	it.data = nil
	return nil
}
