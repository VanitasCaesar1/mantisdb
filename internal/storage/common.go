// Package storage provides common storage functionality
package storage

import (
	"errors"
	"sync"

	"mantisDB/pkg/storage"
)

// BaseStorageEngine provides common functionality for storage engines
type BaseStorageEngine struct {
	data   map[string]string
	mutex  sync.RWMutex
	config storage.Config
	closed bool
}

// NewBaseStorageEngine creates a new base storage engine
func NewBaseStorageEngine(config storage.Config) *BaseStorageEngine {
	return &BaseStorageEngine{
		data:   make(map[string]string),
		config: config,
		closed: false,
	}
}

// ValidateInitialized checks if the storage engine is initialized
func (s *BaseStorageEngine) ValidateInitialized() error {
	if s.closed || s.data == nil {
		return errors.New("storage engine not initialized")
	}
	return nil
}

// SafePut performs a thread-safe put operation
func (s *BaseStorageEngine) SafePut(key, value string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if err := s.ValidateInitialized(); err != nil {
		return err
	}

	s.data[key] = value
	return nil
}

// SafeGet performs a thread-safe get operation
func (s *BaseStorageEngine) SafeGet(key string) (string, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if err := s.ValidateInitialized(); err != nil {
		return "", err
	}

	value, exists := s.data[key]
	if !exists {
		return "", errors.New("key not found")
	}

	return value, nil
}

// SafeDelete performs a thread-safe delete operation
func (s *BaseStorageEngine) SafeDelete(key string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if err := s.ValidateInitialized(); err != nil {
		return err
	}

	delete(s.data, key)
	return nil
}

// SafeBatchPut performs a thread-safe batch put operation
func (s *BaseStorageEngine) SafeBatchPut(kvPairs map[string]string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if err := s.ValidateInitialized(); err != nil {
		return err
	}

	for k, v := range kvPairs {
		s.data[k] = v
	}
	return nil
}

// SafeBatchGet performs a thread-safe batch get operation
func (s *BaseStorageEngine) SafeBatchGet(keys []string) (map[string]string, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if err := s.ValidateInitialized(); err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, key := range keys {
		if value, exists := s.data[key]; exists {
			result[key] = value
		}
	}

	return result, nil
}

// SafeBatchDelete performs a thread-safe batch delete operation
func (s *BaseStorageEngine) SafeBatchDelete(keys []string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if err := s.ValidateInitialized(); err != nil {
		return err
	}

	for _, key := range keys {
		delete(s.data, key)
	}
	return nil
}

// GetKeysWithPrefix returns all keys matching the given prefix
func (s *BaseStorageEngine) GetKeysWithPrefix(prefix string) []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var keys []string
	for key := range s.data {
		if prefix == "" || (len(key) >= len(prefix) && key[:len(prefix)] == prefix) {
			keys = append(keys, key)
		}
	}
	return keys
}

// Close closes the storage engine
func (s *BaseStorageEngine) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.data = nil
	s.closed = true
	return nil
}

// HealthCheck performs a health check
func (s *BaseStorageEngine) HealthCheck() error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.closed || s.data == nil {
		return errors.New("storage engine not initialized")
	}
	return nil
}

// CommonIterator provides a common iterator implementation
type CommonIterator struct {
	storage StorageReader
	prefix  string
	keys    []string
	index   int
	err     error
}

// StorageReader defines the interface for reading from storage
type StorageReader interface {
	GetKeysWithPrefix(prefix string) []string
	SafeGet(key string) (string, error)
}

// NewCommonIterator creates a new common iterator
func NewCommonIterator(storage StorageReader, prefix string) *CommonIterator {
	return &CommonIterator{
		storage: storage,
		prefix:  prefix,
		keys:    make([]string, 0),
		index:   -1,
	}
}

// Next advances the iterator to the next item
func (it *CommonIterator) Next() bool {
	if it.index == -1 {
		// First call - populate keys
		it.keys = it.storage.GetKeysWithPrefix(it.prefix)
		it.index = 0
	} else {
		it.index++
	}

	return it.index < len(it.keys)
}

// Key returns the current key
func (it *CommonIterator) Key() string {
	if it.index >= 0 && it.index < len(it.keys) {
		return it.keys[it.index]
	}
	return ""
}

// Value returns the current value
func (it *CommonIterator) Value() string {
	if it.index >= 0 && it.index < len(it.keys) {
		key := it.keys[it.index]
		value, err := it.storage.SafeGet(key)
		if err != nil {
			it.err = err
			return ""
		}
		return value
	}
	return ""
}

// Error returns any error that occurred during iteration
func (it *CommonIterator) Error() error {
	return it.err
}

// Close closes the iterator
func (it *CommonIterator) Close() error {
	it.keys = nil
	return nil
}

// TransactionOp represents a transaction operation
type TransactionOp struct {
	OpType string // "put", "delete"
	Value  string
}

// CommonTransaction provides a common transaction implementation
type CommonTransaction struct {
	storage StorageEngine
	ops     map[string]TransactionOp
	mutex   sync.RWMutex
}

// StorageEngine defines the interface for storage operations within transactions
type StorageEngine interface {
	SafeGet(key string) (string, error)
	SafePut(key, value string) error
	SafeDelete(key string) error
}

// NewCommonTransaction creates a new common transaction
func NewCommonTransaction(storage StorageEngine) *CommonTransaction {
	return &CommonTransaction{
		storage: storage,
		ops:     make(map[string]TransactionOp),
	}
}

// Put adds a key-value pair to the transaction
func (tx *CommonTransaction) Put(key, value string) error {
	tx.mutex.Lock()
	defer tx.mutex.Unlock()

	tx.ops[key] = TransactionOp{OpType: "put", Value: value}
	return nil
}

// Get retrieves a value within the transaction context
func (tx *CommonTransaction) Get(key string) (string, error) {
	tx.mutex.RLock()
	defer tx.mutex.RUnlock()

	// Check transaction ops first
	if op, exists := tx.ops[key]; exists {
		if op.OpType == "delete" {
			return "", errors.New("key not found")
		}
		return op.Value, nil
	}

	// Fall back to storage
	return tx.storage.SafeGet(key)
}

// Delete marks a key for deletion in the transaction
func (tx *CommonTransaction) Delete(key string) error {
	tx.mutex.Lock()
	defer tx.mutex.Unlock()

	tx.ops[key] = TransactionOp{OpType: "delete"}
	return nil
}

// Commit applies all transaction operations atomically
func (tx *CommonTransaction) Commit() error {
	tx.mutex.Lock()
	defer tx.mutex.Unlock()

	for key, op := range tx.ops {
		switch op.OpType {
		case "put":
			if err := tx.storage.SafePut(key, op.Value); err != nil {
				return err
			}
		case "delete":
			if err := tx.storage.SafeDelete(key); err != nil {
				return err
			}
		}
	}

	tx.ops = make(map[string]TransactionOp)
	return nil
}

// Rollback discards all transaction operations
func (tx *CommonTransaction) Rollback() error {
	tx.mutex.Lock()
	defer tx.mutex.Unlock()

	tx.ops = make(map[string]TransactionOp)
	return nil
}
