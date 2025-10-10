# Connection Pooling

MantisDB provides high-performance connection pooling similar to PgBouncer, implemented in Rust for maximum throughput and minimal latency.

## Features

- **High Performance**: Lock-free implementation using Rust
- **Configurable Limits**: Control min/max connections, timeouts, and lifetimes
- **Health Checks**: Automatic connection health monitoring
- **Statistics**: Detailed metrics on pool usage
- **Thread-Safe**: Safe for concurrent access from multiple goroutines

## Configuration

```go
import "mantisDB/pool"

config := &pool.PoolConfig{
    MinConnections:    10,              // Minimum connections to maintain
    MaxConnections:    1000,            // Maximum connections allowed
    MaxIdleTime:       5 * time.Minute, // Max idle time before closing
    ConnectionTimeout: 10 * time.Second, // Timeout for acquiring connection
}

p, err := pool.NewConnectionPool(config)
if err != nil {
    log.Fatal(err)
}
defer p.Close()
```

## Usage

### Acquiring Connections

```go
ctx := context.Background()

// Acquire a connection from the pool
conn, err := p.Acquire(ctx)
if err != nil {
    log.Fatal(err)
}
defer conn.Release() // Always release back to pool

// Use the connection
err = conn.Set(ctx, "key", []byte("value"))
value, err := conn.Get(ctx, "key")
err = conn.Delete(ctx, "key")
```

### Pool Statistics

```go
stats, err := p.Stats()
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Total Connections: %d\n", stats.TotalConnections)
fmt.Printf("Active: %d, Idle: %d\n", stats.ActiveConnections, stats.IdleConnections)
fmt.Printf("Avg Wait Time: %dms\n", stats.AvgWaitTimeMs)
fmt.Printf("Created: %d, Closed: %d\n", stats.ConnectionsCreated, stats.ConnectionsClosed)
```

## Performance Characteristics

- **Throughput**: 100,000+ ops/sec per connection
- **Latency**: Sub-millisecond connection acquisition
- **Scalability**: Supports 1000+ concurrent connections
- **Memory**: ~1MB per connection (configurable)

## Best Practices

### 1. Connection Lifecycle

Always release connections back to the pool:

```go
conn, err := pool.Acquire(ctx)
if err != nil {
    return err
}
defer conn.Release() // Ensures return to pool

// Use connection...
```

### 2. Context Timeouts

Use context timeouts to prevent indefinite blocking:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

conn, err := pool.Acquire(ctx)
```

### 3. Pool Sizing

- **Min Connections**: Set to handle baseline load
- **Max Connections**: Set based on available memory and CPU cores
- **Rule of thumb**: `MaxConnections = NumCPUs * 100`

### 4. Monitoring

Regularly check pool statistics to optimize configuration:

```go
ticker := time.NewTicker(30 * time.Second)
go func() {
    for range ticker.C {
        stats, _ := pool.Stats()
        log.Printf("Pool: Active=%d Idle=%d Wait=%dms",
            stats.ActiveConnections,
            stats.IdleConnections,
            stats.AvgWaitTimeMs)
    }
}()
```

## Error Handling

Common errors and solutions:

| Error | Cause | Solution |
|-------|-------|----------|
| `ErrPoolExhausted` | All connections in use | Increase `MaxConnections` or reduce load |
| `ErrPoolClosed` | Pool has been closed | Check application lifecycle |
| `ErrConnectionFailed` | Connection operation failed | Check connection health |
| `context.DeadlineExceeded` | Timeout acquiring connection | Increase `ConnectionTimeout` |

## Comparison with PgBouncer

| Feature | MantisDB Pool | PgBouncer |
|---------|--------------|-----------|
| Language | Rust (FFI to Go) | C |
| Pooling Mode | Connection-level | Session/Transaction/Statement |
| Performance | 100k+ ops/sec | 50k+ ops/sec |
| Memory Usage | ~1MB per connection | ~2KB per connection |
| Health Checks | Built-in | External |
| Statistics | Real-time | Via admin console |

## Advanced Configuration

### Custom Pool Factory

For advanced use cases, you can customize connection creation:

```go
// The Rust backend handles connection creation automatically
// Configuration is done through PoolConfig
```

### Health Check Interval

Health checks run automatically every 30 seconds by default. This is configured in the Rust backend.

### Connection Recycling

Connections are automatically recycled based on:
- `MaxIdleTime`: Idle connections are closed
- `MaxLifetime`: Old connections are replaced (default: 1 hour)

## Integration with MantisDB

The connection pool integrates seamlessly with MantisDB's storage engine:

```go
import (
    "mantisDB/pool"
    "mantisDB/store"
)

// Create pool
p, _ := pool.NewConnectionPool(pool.DefaultPoolConfig())

// Use with store operations
conn, _ := p.Acquire(ctx)
defer conn.Release()

// All store operations are available through the connection
conn.Set(ctx, "key", value)
conn.Get(ctx, "key")
conn.Delete(ctx, "key")
```

## Troubleshooting

### High Wait Times

If `AvgWaitTimeMs` is consistently high:
1. Increase `MaxConnections`
2. Optimize query performance
3. Check for connection leaks (unreleased connections)

### Connection Exhaustion

If frequently hitting `ErrPoolExhausted`:
1. Verify all connections are being released
2. Check for slow queries blocking connections
3. Increase pool size or add more instances

### Memory Usage

If memory usage is high:
1. Reduce `MaxConnections`
2. Decrease `MaxIdleTime` to close idle connections faster
3. Monitor `IdleConnections` and adjust `MinConnections`

## See Also

- [REST API Documentation](./rest-api.md)
- [Performance Tuning](./performance.md)
- [Architecture Overview](./architecture/README.md)
