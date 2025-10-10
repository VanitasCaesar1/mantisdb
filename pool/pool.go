package pool

/*
#cgo LDFLAGS: -L../lib -lmantisdb_core -ldl -lm
#include <stdlib.h>

// Connection pool FFI declarations
typedef struct PoolHandle PoolHandle;
typedef struct ConnectionHandle ConnectionHandle;

typedef struct {
    size_t total_connections;
    size_t active_connections;
    size_t idle_connections;
    unsigned long long wait_count;
    unsigned long long avg_wait_time_ms;
    unsigned long long connections_created;
    unsigned long long connections_closed;
    unsigned long long health_check_failures;
} CPoolStats;

PoolHandle* mantis_pool_new(int min_connections, int max_connections, int max_idle_seconds, int connection_timeout_seconds);
ConnectionHandle* mantis_pool_acquire(PoolHandle* pool);
void mantis_pool_release(ConnectionHandle* conn);
int mantis_conn_get(ConnectionHandle* conn, const char* key, unsigned char** value_out, size_t* value_len_out);
int mantis_conn_put(ConnectionHandle* conn, const char* key, const unsigned char* value, size_t value_len);
int mantis_conn_delete(ConnectionHandle* conn, const char* key);
int mantis_pool_stats(PoolHandle* pool, CPoolStats* stats_out);
void mantis_pool_destroy(PoolHandle* pool);
void mantis_free_value(unsigned char* ptr, size_t len);
*/
import "C"
import (
	"context"
	"errors"
	"fmt"
	"time"
	"unsafe"
)

var (
	// ErrPoolClosed is returned when the pool is closed
	ErrPoolClosed = errors.New("connection pool is closed")
	// ErrPoolExhausted is returned when no connections are available
	ErrPoolExhausted = errors.New("connection pool exhausted")
	// ErrConnectionFailed is returned when connection operations fail
	ErrConnectionFailed = errors.New("connection operation failed")
)

// PoolConfig holds connection pool configuration
type PoolConfig struct {
	// MinConnections is the minimum number of connections to maintain
	MinConnections int
	// MaxConnections is the maximum number of connections allowed
	MaxConnections int
	// MaxIdleTime is the maximum time a connection can be idle before being closed
	MaxIdleTime time.Duration
	// ConnectionTimeout is the timeout for acquiring a connection
	ConnectionTimeout time.Duration
}

// DefaultPoolConfig returns a default pool configuration
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		MinConnections:    10,
		MaxConnections:    1000,
		MaxIdleTime:       5 * time.Minute,
		ConnectionTimeout: 10 * time.Second,
	}
}

// PoolStats holds connection pool statistics
type PoolStats struct {
	TotalConnections     int
	ActiveConnections    int
	IdleConnections      int
	WaitCount            uint64
	AvgWaitTimeMs        uint64
	ConnectionsCreated   uint64
	ConnectionsClosed    uint64
	HealthCheckFailures  uint64
}

// ConnectionPool manages a pool of database connections using Rust backend
type ConnectionPool struct {
	handle *C.PoolHandle
	config *PoolConfig
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(config *PoolConfig) (*ConnectionPool, error) {
	if config == nil {
		config = DefaultPoolConfig()
	}

	handle := C.mantis_pool_new(
		C.int(config.MinConnections),
		C.int(config.MaxConnections),
		C.int(config.MaxIdleTime.Seconds()),
		C.int(config.ConnectionTimeout.Seconds()),
	)

	if handle == nil {
		return nil, fmt.Errorf("failed to create connection pool")
	}

	return &ConnectionPool{
		handle: handle,
		config: config,
	}, nil
}

// Acquire acquires a connection from the pool
func (p *ConnectionPool) Acquire(ctx context.Context) (*PooledConnection, error) {
	if p.handle == nil {
		return nil, ErrPoolClosed
	}

	// Create a channel for the result
	resultCh := make(chan *C.ConnectionHandle, 1)
	errCh := make(chan error, 1)

	// Acquire connection in a goroutine
	go func() {
		handle := C.mantis_pool_acquire(p.handle)
		if handle == nil {
			errCh <- ErrPoolExhausted
		} else {
			resultCh <- handle
		}
	}()

	// Wait for result or timeout
	select {
	case handle := <-resultCh:
		return &PooledConnection{
			handle: handle,
			pool:   p,
		}, nil
	case err := <-errCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Stats returns current pool statistics
func (p *ConnectionPool) Stats() (*PoolStats, error) {
	if p.handle == nil {
		return nil, ErrPoolClosed
	}

	var cStats C.CPoolStats
	result := C.mantis_pool_stats(p.handle, &cStats)
	if result != 0 {
		return nil, fmt.Errorf("failed to get pool stats")
	}

	return &PoolStats{
		TotalConnections:    int(cStats.total_connections),
		ActiveConnections:   int(cStats.active_connections),
		IdleConnections:     int(cStats.idle_connections),
		WaitCount:           uint64(cStats.wait_count),
		AvgWaitTimeMs:       uint64(cStats.avg_wait_time_ms),
		ConnectionsCreated:  uint64(cStats.connections_created),
		ConnectionsClosed:   uint64(cStats.connections_closed),
		HealthCheckFailures: uint64(cStats.health_check_failures),
	}, nil
}

// Close closes the connection pool
func (p *ConnectionPool) Close() error {
	if p.handle == nil {
		return nil
	}

	C.mantis_pool_destroy(p.handle)
	p.handle = nil
	return nil
}

// PooledConnection represents a connection from the pool
type PooledConnection struct {
	handle *C.ConnectionHandle
	pool   *ConnectionPool
}

// Get retrieves a value by key
func (c *PooledConnection) Get(ctx context.Context, key string) ([]byte, error) {
	if c.handle == nil {
		return nil, ErrConnectionFailed
	}

	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))

	var valuePtr *C.uchar
	var valueLen C.size_t

	result := C.mantis_conn_get(c.handle, cKey, &valuePtr, &valueLen)
	if result != 0 {
		return nil, fmt.Errorf("key not found: %s", key)
	}

	// Copy the data
	value := C.GoBytes(unsafe.Pointer(valuePtr), C.int(valueLen))

	// Free the Rust-allocated memory
	C.mantis_free_value(valuePtr, valueLen)

	return value, nil
}

// Set stores a key-value pair
func (c *PooledConnection) Set(ctx context.Context, key string, value []byte) error {
	if c.handle == nil {
		return ErrConnectionFailed
	}

	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))

	var valuePtr *C.uchar
	if len(value) > 0 {
		valuePtr = (*C.uchar)(unsafe.Pointer(&value[0]))
	}

	result := C.mantis_conn_put(c.handle, cKey, valuePtr, C.size_t(len(value)))
	if result != 0 {
		return fmt.Errorf("failed to set key: %s", key)
	}

	return nil
}

// Delete removes a key-value pair
func (c *PooledConnection) Delete(ctx context.Context, key string) error {
	if c.handle == nil {
		return ErrConnectionFailed
	}

	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))

	result := C.mantis_conn_delete(c.handle, cKey)
	if result != 0 {
		return fmt.Errorf("failed to delete key: %s", key)
	}

	return nil
}

// Release returns the connection to the pool
func (c *PooledConnection) Release() {
	if c.handle != nil {
		C.mantis_pool_release(c.handle)
		c.handle = nil
	}
}

// Close is an alias for Release
func (c *PooledConnection) Close() error {
	c.Release()
	return nil
}
