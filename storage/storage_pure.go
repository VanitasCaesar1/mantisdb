package storage

import (
	"context"
	"errors"
	"sync"
)

// PureGoStorageEngine implements StorageEngine in pure Go
type PureGoStorageEngine struct {
	data   map[string]string
	mutex  sync.RWMutex
	config StorageConfig
}

// NewPureGoStorageEngine creates a new pure Go storage engine
func NewPureGoStorageEngine(config StorageConfig) *PureGoStorageEngine {
	return &PureGoStorageEngine{
		data:   make(map[string]string),
		config: config,
	}
}

func (s *PureGoStorageEngine) Init(dataDir string) error {
	// Pure Go implementation doesn't need initialization
	return nil
}

func (s *PureGoStorageEngine) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.data = nil
	return nil
}

func (s *PureGoStorageEngine) Put(ctx context.Context, key, value string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.data == nil {
		return errors.New("storage engine not initialized")
	}

	s.data[key] = value
	return nil
}

func (s *PureGoStorageEngine) Get(ctx context.Context, key string) (string, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.data == nil {
		return "", errors.New("storage engine not initialized")
	}

	value, exists := s.data[key]
	if !exists {
		return "", errors.New("key not found")
	}

	return value, nil
}

func (s *PureGoStorageEngine) Delete(ctx context.Context, key string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.data == nil {
		return errors.New("storage engine not initialized")
	}

	delete(s.data, key)
	return nil
}

func (s *PureGoStorageEngine) BatchPut(ctx context.Context, kvPairs map[string]string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.data == nil {
		return errors.New("storage engine not initialized")
	}

	for k, v := range kvPairs {
		s.data[k] = v
	}
	return nil
}

func (s *PureGoStorageEngine) BatchGet(ctx context.Context, keys []string) (map[string]string, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

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

func (s *PureGoStorageEngine) BatchDelete(ctx context.Context, keys []string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.data == nil {
		return errors.New("storage engine not initialized")
	}

	for _, key := range keys {
		delete(s.data, key)
	}
	return nil
}

func (s *PureGoStorageEngine) NewIterator(ctx context.Context, prefix string) (Iterator, error) {
	return &pureGoIterator{
		storage: s,
		prefix:  prefix,
		keys:    make([]string, 0),
		index:   -1,
	}, nil
}

func (s *PureGoStorageEngine) BeginTransaction(ctx context.Context) (Transaction, error) {
	return &pureGoTransaction{
		storage: s,
		ops:     make(map[string]transactionOp),
	}, nil
}

func (s *PureGoStorageEngine) HealthCheck(ctx context.Context) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.data == nil {
		return errors.New("storage engine not initialized")
	}
	return nil
}

// pureGoIterator implements Iterator for pure Go storage
type pureGoIterator struct {
	storage *PureGoStorageEngine
	prefix  string
	keys    []string
	index   int
	err     error
}

func (it *pureGoIterator) Next() bool {
	if it.index == -1 {
		// First call - populate keys
		it.storage.mutex.RLock()
		for key := range it.storage.data {
			if it.prefix == "" || len(key) >= len(it.prefix) && key[:len(it.prefix)] == it.prefix {
				it.keys = append(it.keys, key)
			}
		}
		it.storage.mutex.RUnlock()
		it.index = 0
	} else {
		it.index++
	}

	return it.index < len(it.keys)
}

func (it *pureGoIterator) Key() string {
	if it.index >= 0 && it.index < len(it.keys) {
		return it.keys[it.index]
	}
	return ""
}

func (it *pureGoIterator) Value() string {
	if it.index >= 0 && it.index < len(it.keys) {
		key := it.keys[it.index]
		it.storage.mutex.RLock()
		value := it.storage.data[key]
		it.storage.mutex.RUnlock()
		return value
	}
	return ""
}

func (it *pureGoIterator) Error() error {
	return it.err
}

func (it *pureGoIterator) Close() error {
	it.keys = nil
	return nil
}

// pureGoTransaction implements Transaction for pure Go storage
type transactionOp struct {
	opType string // "put", "delete"
	value  string
}

type pureGoTransaction struct {
	storage *PureGoStorageEngine
	ops     map[string]transactionOp
}

func (tx *pureGoTransaction) Put(key, value string) error {
	tx.ops[key] = transactionOp{opType: "put", value: value}
	return nil
}

func (tx *pureGoTransaction) Get(key string) (string, error) {
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

func (tx *pureGoTransaction) Delete(key string) error {
	tx.ops[key] = transactionOp{opType: "delete"}
	return nil
}

func (tx *pureGoTransaction) Commit() error {
	tx.storage.mutex.Lock()
	defer tx.storage.mutex.Unlock()

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

func (tx *pureGoTransaction) Rollback() error {
	tx.ops = make(map[string]transactionOp)
	return nil
}
