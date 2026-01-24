# MantisDB Project Steering

MantisDB is a high-performance multi-model database with Rust-powered admin backend and PostgreSQL-compatible Row Level Security. It supports Key-Value, Document, Columnar, and SQL data models.

> **Status:** Single-node implementation is solid. See `FEATURE_GAP_ANALYSIS.md` for production readiness assessment.

## Project Structure

```
├── cmd/mantisDB/          # Main entry point
├── config/                # Configuration management (env + YAML)
├── api/                   # REST API server (Go)
├── admin/
│   └── frontend/          # React admin dashboard
├── rust-core/             # Rust components (see Rust Core section below)
│   ├── src/
│   │   ├── pool.rs        # Lock-free connection pooling
│   │   ├── rls.rs         # Row Level Security engine
│   │   ├── vector_db.rs   # Vector similarity search
│   │   ├── rest_api.rs    # Admin REST API (Axum)
│   │   ├── cache.rs       # Lock-free cache
│   │   ├── fts.rs         # Full-text search
│   │   ├── timeseries.rs  # Time series support
│   │   ├── geospatial.rs  # Geospatial queries
│   │   ├── cdc.rs         # Change data capture
│   │   └── graphql_api.rs # GraphQL endpoint
├── storage/               # Storage engine abstraction
│   ├── storage_interface.go   # Core interface
│   ├── storage_pure.go        # Pure Go implementation
│   └── storage_rust.go        # Rust FFI implementation
├── store/                 # Unified multi-model store (MantisStore)
├── models/
│   ├── keyvalue.go        # Redis-like KV model
│   ├── document.go        # MongoDB-like document model
│   └── columnar.go        # Cassandra-like columnar model
├── cache/                 # Cache with dependency tracking
├── query/                 # SQL parser, optimizer, executor
├── transaction/           # ACID transactions, 2PL, deadlock detection
├── wal/                   # Write-ahead logging (ARIES recovery)
├── durability/            # Sync/async writers, flush management
├── integrity/             # Checksum engine, corruption detection
├── checkpoint/            # Checkpoint management, point-in-time recovery
├── health/                # Health checks
├── monitoring/            # Prometheus metrics, alerting
├── shutdown/              # Graceful shutdown management
├── advanced/              # Write optimizer, compression, concurrency
├── errors/                # Error handling, corruption recovery, disk monitoring
├── rpo/                   # Recovery Point Objective management
├── pool/                  # Go-side connection pooling
├── benchmark/             # Performance benchmarks
├── testing/               # Edge case and reliability tests
├── clients/               # Go, Python, JavaScript SDKs
├── sdks/                  # Python and TypeScript SDKs
├── scripts/               # Build and deployment scripts
└── docs/                  # Documentation
```

## Code Conventions

### Comment Philosophy (Linus Torvalds style)

1. **Comments explain WHY, not WHAT**
2. **No obvious comments** - don't comment `i++`
3. **Document trade-offs** - explain why you chose one approach over another
4. **Be direct** - no corporate speak

```go
/*
 * We use RWMutex here instead of sync.Map because benchmarks showed
 * RWMutex is 30% faster for our read-heavy workload.
 */
```

### File Headers

All files should have a descriptive header:

```go
/*
 * Component Name - Brief description
 *
 * Detailed explanation of what this component does.
 * Document any important design decisions or trade-offs.
 * Explain performance considerations if any.
 */
package packagename
```

### Error Handling

- Return errors, don't panic
- Wrap errors with context: `fmt.Errorf("failed to X: %w", err)`
- Use custom error types in `errors/` package

## Architecture

### Multi-Model Store

`MantisStore` provides unified access to all data models:

```go
store := store.NewMantisStore(storageEngine, cacheManager)

// Key-Value operations
store.KV().Set(ctx, key, value, ttl)
store.KV().Get(ctx, key)

// Document operations
store.Documents().Create(ctx, doc)
store.Documents().Query(ctx, query, cacheTTL)

// Columnar operations
store.Columnar().CreateTable(ctx, table)
store.Columnar().Query(ctx, query, cacheTTL)
```

### Storage Engine Interface

```go
type StorageEngine interface {
    Init(dataDir string) error
    Close() error
    Put(ctx context.Context, key, value string) error
    Get(ctx context.Context, key string) (string, error)
    Delete(ctx context.Context, key string) error
    BatchPut(ctx context.Context, kvPairs map[string]string) error
    BatchGet(ctx context.Context, keys []string) (map[string]string, error)
    NewIterator(ctx context.Context, prefix string) (Iterator, error)
    BeginTransaction(ctx context.Context) (Transaction, error)
    HealthCheck(ctx context.Context) error
}
```

### Transaction System

Uses 2PL (Two-Phase Locking) with deadlock detection:

```go
txn, _ := txnManager.Begin(IsolationReadCommitted)
txnManager.AcquireLock(txn, key, LockTypeExclusive)
// ... operations ...
txnManager.Commit(txn)  // or Abort(txn)
```

### Cache with Dependency Tracking

```go
// Cache with automatic invalidation when dependencies change
cache.Put(ctx, cacheKey, value, ttl, []string{"table:users"})

// When table:users changes, cacheKey is automatically invalidated
cache.InvalidateDependencies(ctx, "table:users")
```

### WAL (Write-Ahead Log)

Binary format with CRC32 checksums:
- LSN (Log Sequence Number)
- Transaction ID
- Operation type (insert/update/delete/commit/abort)
- Key, Value, OldValue (for rollback)

## Data Models

### Key-Value (Redis-like)
- TTL support with lazy expiration
- Batch operations
- Tags and metadata

### Document (MongoDB-like)
- Flexible schema (map[string]interface{})
- Collections
- Query with filters

### Columnar (Cassandra-like)
- Column-oriented storage
- Partitioning
- Aggregations (count, sum, avg, min, max)

## Configuration

Environment variables (12-factor app pattern):

```bash
MANTIS_PORT=8080
MANTIS_ADMIN_PORT=8081
MANTIS_DATA_DIR=./data
MANTIS_WAL_DIR=./wal
MANTIS_CACHE_SIZE=100MB
MANTIS_BUFFER_SIZE=64MB
MANTIS_USE_CGO=false
MANTIS_LOG_LEVEL=info
MANTIS_ADMIN_TOKEN=secret
```

## Build & Run

```bash
# Full build (Rust + Go + Admin UI)
./scripts/build-all.sh

# Or use Make
make build
make run

# Development mode
make dev

# Run benchmarks
make bench

# Tests
make test
```

## API Endpoints

- Database API: `http://localhost:8080`
- Admin Dashboard: `http://localhost:8081` (Rust server)
- Health: `http://localhost:8080/health`

### Key-Value API
- `GET /api/v1/kv/{key}` - Get value
- `PUT /api/v1/kv/{key}` - Set value
- `DELETE /api/v1/kv/{key}` - Delete key
- `POST /api/v1/kv/batch` - Batch operations

### Document API
- `POST /api/v1/docs/{collection}` - Create document
- `GET /api/v1/docs/{collection}/{id}` - Get document
- `PUT /api/v1/docs/{collection}/{id}` - Update document
- `POST /api/v1/docs/query` - Query documents

### Columnar API
- `POST /api/v1/tables/{name}` - Create table
- `GET /api/v1/tables/{name}` - Get table metadata
- `POST /api/v1/tables/{name}/insert` - Insert rows
- `POST /api/v1/tables/query` - Query data

## Testing

```bash
# All tests
make test

# Benchmarks
make bench

# Stress tests
./mantisdb --benchmark-stress=heavy
```

## Rust Core Architecture

The `rust-core/` module provides high-performance components via FFI:

### Connection Pool (`pool.rs`)
Lock-free connection pooling using crossbeam's ArrayQueue:
- Zero contention on checkout/return
- Tokio semaphore for backpressure
- Health checks with configurable intervals
- RAII connection return via Drop

### Row Level Security (`rls.rs`)
PostgreSQL-compatible RLS engine:
- Permissive/Restrictive policy types
- USING and WITH CHECK expressions
- Role-based policy filtering
- Expression compilation for fast evaluation

### Vector Database (`vector_db.rs`)
Similarity search with multiple metrics:
- Cosine, Euclidean, Dot Product distances
- Metadata filtering
- Batch insert support
- Normalized vector caching for cosine

### Other Rust Modules
- `cache.rs` - Lock-free cache with mimalloc
- `fts.rs` - Full-text search
- `timeseries.rs` - Time series data
- `geospatial.rs` - Geospatial queries
- `cdc.rs` - Change data capture
- `graphql_api.rs` - GraphQL endpoint
- `admin_api.rs` - Admin dashboard backend (Axum)

### Rust Dependencies
```toml
crossbeam = "0.8"          # Lock-free data structures
parking_lot = "0.12"       # Fast mutexes
tokio = "1"                # Async runtime
axum = "0.7"               # REST framework
mimalloc = "0.1"           # Fast allocator
rkyv = "0.7"               # Zero-copy serialization
```

## Durability System

Three write modes (`durability/`):
- **Sync**: fsync every write (safest, slowest)
- **Async**: fsync periodically (faster, small data loss window)
- **Batch**: fsync after N writes (balanced)

Components:
- `SyncWriter` - Direct fsync writes
- `AsyncWriter` - Buffered async writes
- `FlushManager` - Coordinates flush operations
- `SyncOptimizer` - Batches fsyncs for throughput

## Integrity System

Data corruption detection (`integrity/`):
- `ChecksumEngine` - CRC32/MD5/SHA256 checksums
- `CorruptionDetector` - Detects data corruption
- `WALIntegrity` - WAL entry verification
- `Monitor` - Continuous integrity monitoring

## SQL Query Engine

Hand-rolled SQL parser (`query/`):
- Single-pass tokenizer
- Recursive descent parser
- Supports: SELECT, INSERT, UPDATE, DELETE, CREATE, DROP
- WHERE, ORDER BY, LIMIT clauses
- No external parser dependencies

## Dependencies

### Go packages
- `github.com/golang/snappy` - Compression
- `github.com/klauspost/compress` - Compression
- `github.com/pierrec/lz4/v4` - LZ4 compression
- `gopkg.in/yaml.v3` - YAML config

### Rust crates
- `crossbeam` - Lock-free data structures
- `tokio` - Async runtime
- `axum` - HTTP framework
- `serde` - Serialization
- `mimalloc` - Memory allocator

## Known Gaps (see FEATURE_GAP_ANALYSIS.md)

**Critical:**
- No replication/clustering (single-node only)
- RLS not integrated with Go query executor
- Auth is weak (single admin token)

**Important:**
- No encryption at rest
- Rate limiting not enforced
- SDKs are stubs (Python, TypeScript)

**Nice to have:**
- Document aggregation pipeline incomplete
- CQL support not implemented
- Window functions missing

## License

MIT License - Copyright (c) 2025 Vanitas Caesar
