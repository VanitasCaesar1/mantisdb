// +build rust

package storage

/*
#cgo LDFLAGS: -L../rust-core/target/release -lmantisdb_core
#include <stdlib.h>
#include <stdint.h>

// Storage functions
extern uintptr_t storage_new();
extern void storage_free(uintptr_t handle);
extern int storage_put(uintptr_t handle, const char* key, size_t key_len, const uint8_t* value, size_t value_len);
extern int storage_get(uintptr_t handle, const char* key, size_t key_len, uint8_t** value_out, size_t* value_len_out);
extern int storage_delete(uintptr_t handle, const char* key, size_t key_len);
extern int storage_stats(uintptr_t handle, uint64_t* reads_out, uint64_t* writes_out, uint64_t* deletes_out);
extern void rust_free(uint8_t* ptr);

// Cache functions
extern uintptr_t cache_new(size_t max_size);
extern void cache_free(uintptr_t handle);
extern int cache_put(uintptr_t handle, const char* key, size_t key_len, const uint8_t* value, size_t value_len, uint64_t ttl);
extern int cache_get(uintptr_t handle, const char* key, size_t key_len, uint8_t** value_out, size_t* value_len_out);
extern void cache_delete(uintptr_t handle, const char* key, size_t key_len);
extern int cache_stats(uintptr_t handle, uint64_t* hits_out, uint64_t* misses_out, uint64_t* evictions_out);
*/
import "C"
import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"unsafe"
)

// RustStorageEngine implements StorageEngine using Rust core
type RustStorageEngine struct {
	config       StorageConfig
	storageHandle C.uintptr_t
	cacheHandle   C.uintptr_t
}

// NewRustStorageEngine creates a new Rust-based storage engine
func NewRustStorageEngine(config StorageConfig) *RustStorageEngine {
	engine := &RustStorageEngine{
		config:        config,
		storageHandle: C.storage_new(),
		cacheHandle:   C.cache_new(C.size_t(config.CacheSize)),
	}
	
	runtime.SetFinalizer(engine, func(e *RustStorageEngine) {
		C.storage_free(e.storageHandle)
		C.cache_free(e.cacheHandle)
	})
	
	return engine
}

func (s *RustStorageEngine) Init(dataDir string) error {
	// Rust storage is already initialized
	return nil
}

func (s *RustStorageEngine) Close() error {
	C.storage_free(s.storageHandle)
	C.cache_free(s.cacheHandle)
	return nil
}

func (s *RustStorageEngine) Put(ctx context.Context, key, value string) error {
	keyBytes := []byte(key)
	valueBytes := []byte(value)
	
	// Try cache first
	cKey := (*C.char)(unsafe.Pointer(&keyBytes[0]))
	cValue := (*C.uint8_t)(unsafe.Pointer(&valueBytes[0]))
	
	result := C.cache_put(
		s.cacheHandle,
		cKey,
		C.size_t(len(keyBytes)),
		cValue,
		C.size_t(len(valueBytes)),
		0, // No TTL
	)
	
	// Always write to storage
	result = C.storage_put(
		s.storageHandle,
		cKey,
		C.size_t(len(keyBytes)),
		cValue,
		C.size_t(len(valueBytes)),
	)
	
	if result != 0 {
		return errors.New("failed to put key-value")
	}
	
	return nil
}

func (s *RustStorageEngine) Get(ctx context.Context, key string) (string, error) {
	keyBytes := []byte(key)
	cKey := (*C.char)(unsafe.Pointer(&keyBytes[0]))
	
	// Try cache first
	var cValue *C.uint8_t
	var cValueLen C.size_t
	
	result := C.cache_get(
		s.cacheHandle,
		cKey,
		C.size_t(len(keyBytes)),
		&cValue,
		&cValueLen,
	)
	
	if result == 0 && cValue != nil {
		// Cache hit
		value := C.GoBytes(unsafe.Pointer(cValue), C.int(cValueLen))
		C.rust_free(cValue)
		return string(value), nil
	}
	
	// Cache miss - get from storage
	result = C.storage_get(
		s.storageHandle,
		cKey,
		C.size_t(len(keyBytes)),
		&cValue,
		&cValueLen,
	)
	
	if result != 0 || cValue == nil {
		return "", errors.New("key not found")
	}
	
	value := C.GoBytes(unsafe.Pointer(cValue), C.int(cValueLen))
	C.rust_free(cValue)
	
	// Update cache
	cValuePtr := (*C.uint8_t)(unsafe.Pointer(&value[0]))
	C.cache_put(
		s.cacheHandle,
		cKey,
		C.size_t(len(keyBytes)),
		cValuePtr,
		C.size_t(len(value)),
		3600, // 1 hour TTL
	)
	
	return string(value), nil
}

func (s *RustStorageEngine) Delete(ctx context.Context, key string) error {
	keyBytes := []byte(key)
	cKey := (*C.char)(unsafe.Pointer(&keyBytes[0]))
	
	// Delete from cache
	C.cache_delete(s.cacheHandle, cKey, C.size_t(len(keyBytes)))
	
	// Delete from storage
	result := C.storage_delete(
		s.storageHandle,
		cKey,
		C.size_t(len(keyBytes)),
	)
	
	if result != 0 {
		return errors.New("failed to delete key")
	}
	
	return nil
}

func (s *RustStorageEngine) BatchPut(ctx context.Context, kvPairs map[string]string) error {
	for key, value := range kvPairs {
		if err := s.Put(ctx, key, value); err != nil {
			return err
		}
	}
	return nil
}

func (s *RustStorageEngine) BatchGet(ctx context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string)
	for _, key := range keys {
		if value, err := s.Get(ctx, key); err == nil {
			result[key] = value
		}
	}
	return result, nil
}

func (s *RustStorageEngine) BatchDelete(ctx context.Context, keys []string) error {
	for _, key := range keys {
		if err := s.Delete(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

func (s *RustStorageEngine) NewIterator(ctx context.Context, prefix string) (Iterator, error) {
	return &rustIterator{
		engine: s,
		prefix: prefix,
		ctx:    ctx,
	}, nil
}

func (s *RustStorageEngine) BeginTransaction(ctx context.Context) (Transaction, error) {
	return &rustTransaction{
		engine: s,
		ops:    make(map[string]transactionOp),
	}, nil
}

func (s *RustStorageEngine) HealthCheck(ctx context.Context) error {
	// Simple health check - try to read stats
	var reads, writes, deletes C.uint64_t
	result := C.storage_stats(s.storageHandle, &reads, &writes, &deletes)
	if result != 0 {
		return errors.New("storage engine unhealthy")
	}
	return nil
}

// GetStats returns storage and cache statistics
func (s *RustStorageEngine) GetStats() map[string]interface{} {
	var reads, writes, deletes C.uint64_t
	C.storage_stats(s.storageHandle, &reads, &writes, &deletes)
	
	var hits, misses, evictions C.uint64_t
	C.cache_stats(s.cacheHandle, &hits, &misses, &evictions)
	
	hitRate := float64(0)
	total := uint64(hits) + uint64(misses)
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}
	
	return map[string]interface{}{
		"storage": map[string]interface{}{
			"reads":   uint64(reads),
			"writes":  uint64(writes),
			"deletes": uint64(deletes),
		},
		"cache": map[string]interface{}{
			"hits":      uint64(hits),
			"misses":    uint64(misses),
			"evictions": uint64(evictions),
			"hit_rate":  hitRate,
		},
	}
}

// rustIterator implements Iterator for Rust storage
type rustIterator struct {
	engine *RustStorageEngine
	prefix string
	ctx    context.Context
	keys   []string
	index  int
	err    error
}

func (it *rustIterator) Next() bool {
	// Simplified implementation - in production, use Rust iterator
	it.index++
	return it.index < len(it.keys)
}

func (it *rustIterator) Key() string {
	if it.index >= 0 && it.index < len(it.keys) {
		return it.keys[it.index]
	}
	return ""
}

func (it *rustIterator) Value() string {
	key := it.Key()
	if key == "" {
		return ""
	}
	value, _ := it.engine.Get(it.ctx, key)
	return value
}

func (it *rustIterator) Error() error {
	return it.err
}

func (it *rustIterator) Close() error {
	it.keys = nil
	return nil
}

// rustTransaction implements Transaction for Rust storage
type rustTransaction struct {
	engine *RustStorageEngine
	ops    map[string]transactionOp
}

func (tx *rustTransaction) Put(key, value string) error {
	tx.ops[key] = transactionOp{opType: "put", value: value}
	return nil
}

func (tx *rustTransaction) Get(key string) (string, error) {
	if op, exists := tx.ops[key]; exists {
		if op.opType == "delete" {
			return "", errors.New("key not found")
		}
		return op.value, nil
	}
	return tx.engine.Get(context.Background(), key)
}

func (tx *rustTransaction) Delete(key string) error {
	tx.ops[key] = transactionOp{opType: "delete"}
	return nil
}

func (tx *rustTransaction) Commit() error {
	ctx := context.Background()
	for key, op := range tx.ops {
		switch op.opType {
		case "put":
			if err := tx.engine.Put(ctx, key, op.value); err != nil {
				return err
			}
		case "delete":
			if err := tx.engine.Delete(ctx, key); err != nil {
				return err
			}
		}
	}
	tx.ops = make(map[string]transactionOp)
	return nil
}

func (tx *rustTransaction) Rollback() error {
	tx.ops = make(map[string]transactionOp)
	return nil
}

type transactionOp struct {
	opType string
	value  string
}
