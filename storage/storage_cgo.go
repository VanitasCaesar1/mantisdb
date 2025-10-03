package storage

import (
	"context"
	"errors"
	"fmt"
)

// CGOStorageEngine implements StorageEngine using CGO
type CGOStorageEngine struct {
	config StorageConfig
	// In a real implementation, this would interface with C code
	// For now, we'll use a simple map as a placeholder
	data map[string]string
}

// NewCGOStorageEngine creates a new CGO storage engine
func NewCGOStorageEngine(config StorageConfig) *CGOStorageEngine {
	return &CGOStorageEngine{
		config: config,
		data:   make(map[string]string),
	}
}

func (s *CGOStorageEngine) Init(dataDir string) error {
	// Initialize CGO storage engine
	// In a real implementation, this would call C functions
	fmt.Printf("Initializing CGO storage engine in %s\n", dataDir)
	return nil
}

func (s *CGOStorageEngine) Close() error {
	// Close CGO storage engine
	s.data = nil
	return nil
}

func (s *CGOStorageEngine) Put(ctx context.Context, key, value string) error {
	if s.data == nil {
		return errors.New("storage engine not initialized")
	}
	s.data[key] = value
	return nil
}

func (s *CGOStorageEngine) Get(ctx context.Context, key string) (string, error) {
	if s.data == nil {
		return "", errors.New("storage engine not initialized")
	}

	value, exists := s.data[key]
	if !exists {
		return "", errors.New("key not found")
	}

	return value, nil
}

func (s *CGOStorageEngine) Delete(ctx context.Context, key string) error {
	if s.data == nil {
		return errors.New("storage engine not initialized")
	}

	delete(s.data, key)
	return nil
}

func (s *CGOStorageEngine) BatchPut(ctx context.Context, kvPairs map[string]string) error {
	if s.data == nil {
		return errors.New("storage engine not initialized")
	}

	for k, v := range kvPairs {
		s.data[k] = v
	}
	return nil
}

func (s *CGOStorageEngine) BatchGet(ctx context.Context, keys []string) (map[string]string, error) {
	if s.data == nil {
		return nil, errors.New("storage engine not initialized")
	}

	result := make(map[string]string)
	for _, key := range keys {
		if value, exists := s.data[key]; exists {
			result[key] = value
		}
	}

	return result, nil
}

func (s *CGOStorageEngine) BatchDelete(ctx context.Context, keys []string) error {
	if s.data == nil {
		return errors.New("storage engine not initialized")
	}

	for _, key := range keys {
		delete(s.data, key)
	}
	return nil
}

func (s *CGOStorageEngine) NewIterator(ctx context.Context, prefix string) (Iterator, error) {
	return &cgoIterator{
		storage: s,
		prefix:  prefix,
		keys:    make([]string, 0),
		index:   -1,
	}, nil
}

func (s *CGOStorageEngine) BeginTransaction(ctx context.Context) (Transaction, error) {
	return &cgoTransaction{
		storage: s,
		ops:     make(map[string]transactionOp),
	}, nil
}

func (s *CGOStorageEngine) HealthCheck(ctx context.Context) error {
	if s.data == nil {
		return errors.New("storage engine not initialized")
	}
	return nil
}

// cgoIterator implements Iterator for CGO storage
type cgoIterator struct {
	storage *CGOStorageEngine
	prefix  string
	keys    []string
	index   int
	err     error
}

func (it *cgoIterator) Next() bool {
	if it.index == -1 {
		// First call - populate keys
		for key := range it.storage.data {
			if it.prefix == "" || len(key) >= len(it.prefix) && key[:len(it.prefix)] == it.prefix {
				it.keys = append(it.keys, key)
			}
		}
		it.index = 0
	} else {
		it.index++
	}

	return it.index < len(it.keys)
}

func (it *cgoIterator) Key() string {
	if it.index >= 0 && it.index < len(it.keys) {
		return it.keys[it.index]
	}
	return ""
}

func (it *cgoIterator) Value() string {
	if it.index >= 0 && it.index < len(it.keys) {
		key := it.keys[it.index]
		return it.storage.data[key]
	}
	return ""
}

func (it *cgoIterator) Error() error {
	return it.err
}

func (it *cgoIterator) Close() error {
	it.keys = nil
	return nil
}

// cgoTransaction implements Transaction for CGO storage
type cgoTransaction struct {
	storage *CGOStorageEngine
	ops     map[string]transactionOp
}

func (tx *cgoTransaction) Put(key, value string) error {
	tx.ops[key] = transactionOp{opType: "put", value: value}
	return nil
}

func (tx *cgoTransaction) Get(key string) (string, error) {
	// Check transaction ops first
	if op, exists := tx.ops[key]; exists {
		if op.opType == "delete" {
			return "", errors.New("key not found")
		}
		return op.value, nil
	}

	// Fall back to storage
	return tx.storage.Get(context.Background(), key)
}

func (tx *cgoTransaction) Delete(key string) error {
	tx.ops[key] = transactionOp{opType: "delete"}
	return nil
}

func (tx *cgoTransaction) Commit() error {
	for key, op := range tx.ops {
		switch op.opType {
		case "put":
			tx.storage.data[key] = op.value
		case "delete":
			delete(tx.storage.data, key)
		}
	}

	tx.ops = make(map[string]transactionOp)
	return nil
}

func (tx *cgoTransaction) Rollback() error {
	tx.ops = make(map[string]transactionOp)
	return nil
}
