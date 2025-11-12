# WARP.md

This file provides guidance to WARP (warp.dev) when working with code in this repository.

## Project Overview

MantisDB is a high-performance multi-model database with a hybrid architecture:
- **Go Core**: Main database engine, API server, and coordination (Go 1.22+)
- **Rust Core**: Performance-critical components (connection pooling, RLS engine, admin API)
- **React Admin UI**: TypeScript/React dashboard with Vite and Tailwind CSS

**Key Capabilities:**
- 100K+ req/s throughput with sub-millisecond latency
- 4 data models: Key-Value (Redis-like), Document (MongoDB-style), Columnar (Cassandra/ScyllaDB-style), SQL
- PostgreSQL-compatible Row Level Security (RLS) with <10μs checks
- ACID transactions, WAL, point-in-time recovery

## Development Commands

### Building

```powershell
# Full production build (Rust + Go + Admin UI)
./build-unified.sh release

# Backend only (skip UI build - faster for development)
make build-backend-only

# Build Rust components only
make build-rust
cd rust-core
cargo build --release

# Build Go components only
go build -o mantisdb.exe cmd/mantisDB/main.go
```

### Running

**Recommended: Two-terminal approach (fastest for development)**

Terminal 1 (Backend):
```powershell
make run-backend
# Or: go run cmd/mantisDB/main.go
```

Terminal 2 (Frontend dev server):
```powershell
make run-dashboard
# Or: cd admin/frontend && npm run dev
```

**Alternative: Single command (development)**
```powershell
make dev
```

**Production:**
```powershell
./start-production.sh
# Or with docker: docker-compose up -d
```

**Access points:**
- Database API: http://localhost:8080
- Admin Dashboard: http://localhost:5173 (dev) or http://localhost:8081 (production)
- API Docs (Swagger): http://localhost:8081/api/docs

### Testing

```powershell
# All tests (Go + Rust)
make test

# Go tests only
go test ./...

# Run specific Go test
go test ./transaction -run TestTransactionManager -v

# Rust tests
cd rust-core
cargo test --release

# Integration tests
./test_integration.sh

# Benchmarks
make bench
# Or Go benchmarks: go test -bench=. -benchmem ./...
# Or Rust benchmarks: cd rust-core && cargo bench
```

### Linting and Formatting

```powershell
# Format all code
make fmt

# Lint
make lint

# Go formatting only
go fmt ./...
gofmt -s -w .

# Rust formatting only
cd rust-core
cargo fmt
cargo clippy --all-targets --all-features -- -D warnings

# TypeScript/React linting (Admin UI)
cd admin/frontend
npm run lint
npm run type-check
```

### Cleaning

```powershell
make clean  # Removes data/, wal/, backups/, lib/, dist/, and build artifacts
```

## Architecture

### High-Level Structure

```
mantisDB/
├── cmd/mantisDB/          # Main Go entry point
├── rust-core/             # Rust performance layer
│   ├── src/
│   │   ├── pool.rs        # Connection pooling (100k+ ops/sec)
│   │   ├── rls.rs         # Row Level Security engine (<10μs)
│   │   ├── admin_api.rs   # Admin REST API
│   │   ├── wal.rs         # Write-Ahead Log
│   │   ├── cache.rs       # Lock-free cache
│   │   └── bin/admin-server.rs
├── admin/frontend/        # React admin dashboard
├── api/                   # Go HTTP API layer
├── storage/               # Storage engines (KV, Document, Columnar, SQL)
├── transaction/           # ACID transaction system
├── wal/                   # WAL implementation
├── query/                 # Query parser, optimizer, executor
├── config/                # Configuration management
└── pkg/                   # Shared utilities
```

### Critical Architecture Concepts

**1. Hybrid Language Design:**
- Go handles I/O, concurrency, and business logic
- Rust handles memory-critical operations via CGO/FFI
- Communication via FFI bindings (`rust-core/src/ffi.rs`, `rust-core/src/pool_ffi.rs`, `rust-core/src/rls_ffi.rs`)

**2. Storage Engine Abstraction:**
- Interface: `storage/storage_interface.go`
- Implementations: `storage_pure.go` (Go), `storage_rust.go` (Rust via CGO)
- Multi-model stores in `store/` package coordinate between storage engines

**3. Transaction System:**
- Manager: `transaction/manager.go`
- Isolation levels: Read Committed, Repeatable Read, Serializable
- MVCC-based with optimistic locking
- Deadlock detection: `transaction/deadlock_detector.go`
- Integrates with WAL for durability

**4. WAL (Write-Ahead Log):**
- Core: `wal/manager.go`, `wal/entry.go`
- Recovery: `wal/recovery.go`
- Dual implementation: Go and Rust versions for different use cases
- Point-in-time recovery support

**5. Admin Architecture:**
- Backend: Rust Axum server (`rust-core/src/bin/admin-server.rs`)
- Frontend: React SPA with Vite (`admin/frontend/`)
- Proxy: Vite dev server proxies `/api` to `http://localhost:8080`
- Build output: `admin/api/assets/dist/`

**6. Multi-Model Data Access:**
- Key-Value: `models/kv.go` - Redis-like with TTL, prefix search
- Document: `models/document.go` - MongoDB-style with aggregation pipelines
- Columnar: `models/columnar.go` - Cassandra/ScyllaDB-style with CQL support
- SQL: `query/parser.go`, `query/optimizer.go`, `query/executor.go`

### Component Relationships

**Startup Flow:**
1. `cmd/mantisDB/main.go` initializes configuration
2. Creates `MantisDB` instance with storage engine selection
3. Initializes Rust components via FFI (pool, RLS)
4. Starts API server (`api/server.go`)
5. Optionally spawns Rust admin-server process
6. Registers shutdown handlers (`shutdown/manager.go`)

**Request Flow (API):**
1. HTTP request → `api/server.go`
2. Route to handler in `api/handlers.go` or `internal/api/`
3. Business logic in `store/` package
4. Storage operation through `storage/` interface
5. Transaction coordination via `transaction/manager.go`
6. WAL write for durability
7. Response back through layers

**Query Flow (SQL):**
1. SQL query → `query/parser.go` (or C parser via `pkg/sql/c_parser.go`)
2. AST generation
3. Optimization → `query/optimizer.go`
4. Execution plan → `query/executor.go`
5. Storage access through appropriate model
6. Results aggregation and return

## Development Workflows

### Adding a New API Endpoint

1. Define handler in `api/handlers.go` or `internal/api/handlers.go`
2. Add route in `api/server.go` (Go server) or `rust-core/src/admin_api.rs` (Rust admin API)
3. Update models in `models/` if needed
4. Add tests in corresponding `*_test.go` files
5. Update OpenAPI/Swagger docs if in admin API

### Modifying Storage Layer

1. Update interface in `storage/storage_interface.go`
2. Implement in `storage/storage_pure.go` (Go) or `storage/storage_rust.go` (Rust)
3. Update Rust implementation in `rust-core/src/storage.rs` if using Rust
4. Add/update FFI bindings in `rust-core/src/ffi.rs` if Rust changes
5. Update CGO bindings in relevant Go files
6. Run tests: `go test ./storage/... -v`

### Working with Transactions

- Transaction manager is in `transaction/manager.go`
- Begin transaction: `manager.Begin(isolationLevel)`
- Use transaction context for all operations
- Commit or rollback through manager
- Test transaction scenarios in `transaction/manager_test.go`

### Frontend Development

```powershell
cd admin/frontend
npm install
npm run dev  # Hot reload on http://localhost:5173
```

- Main entry: `admin/frontend/src/main.tsx`
- Routing: React Router in `App.tsx`
- API client: `admin/frontend/src/api/` (using axios)
- State management: React Query for server state
- Styling: Tailwind CSS + custom components in `components/`

### Adding a Rust Component

1. Add module in `rust-core/src/`
2. Export in `rust-core/src/lib.rs`
3. If FFI needed, add C-compatible functions with `#[no_mangle]` and `extern "C"`
4. Create corresponding Go binding in `cgo/` or inline in relevant Go file
5. Build: `cd rust-core && cargo build --release`
6. Test: `cargo test --release`

## Configuration

**Environment Variables:** Use `MANTIS_*` prefix (see `config/config.go`)
- `MANTIS_PORT` - API server port (default: 8080)
- `MANTIS_ADMIN_PORT` - Admin server port (default: 8081)
- `MANTIS_DATA_DIR` - Data directory (default: ./data)
- `MANTIS_LOG_LEVEL` - Log level (debug, info, warn, error)

**Config File:** `configs/production.yaml` or set via flags
- See `config/config.go` for all configuration options

## Testing Strategy

- **Unit tests:** Test individual components in isolation
- **Integration tests:** Test component interactions (`integration_test.go`, `wal_transaction_integration_test.go`, `durability_integrity_integration_test.go`)
- **Stress tests:** High-load scenarios (`stress_test.go`, `cmd/stress-benchmark/`)
- **Chaos tests:** Fault injection (`chaos_engineering_test.go`)
- **Edge cases:** Boundary conditions (`cmd/edge-case-tests/`)

## Common Issues

**UI Build Timeout:**
- Use `make run-backend` and `make run-dashboard` in separate terminals
- Or set `SKIP_UI=true` when running build scripts
- Reason: TypeScript + Vite build can timeout on some systems

**Port Conflicts:**
- Check if ports 8080, 8081, or 5173 are in use
- Kill with: `Stop-Process -Id (Get-NetTCPConnection -LocalPort 8080).OwningProcess` (PowerShell)

**Rust Build Errors:**
- Ensure Rust 1.75+ is installed: `rustc --version`
- Clean rebuild: `cd rust-core && cargo clean && cargo build --release`

**CGO Errors:**
- Ensure Rust library is built: `make build-rust`
- Check library path in `lib/` directory
- On Windows, ensure proper C toolchain (MinGW or MSVC)

## Key Files to Understand

**Entry Points:**
- `cmd/mantisDB/main.go` - Main application entry
- `rust-core/src/lib.rs` - Rust core library entry
- `admin/frontend/src/main.tsx` - Admin UI entry

**Core Interfaces:**
- `storage/storage_interface.go` - Storage engine contract
- `transaction/types.go` - Transaction system types
- `query/parser.go` - SQL parsing interface
- `api/server.go` - HTTP API structure

**Configuration:**
- `config/config.go` - All configuration structures
- `Makefile` - Build and development commands
- `build-unified.sh` - Unified build script

**Documentation:**
- `docs/` - Comprehensive documentation by topic
- `MULTI_MODEL_FEATURES.md` - Data model capabilities
- `README.md` - Quick start and overview

## Performance Considerations

- MantisDB uses `mimalloc` (Rust) for optimized memory allocation
- Lock-free data structures in Rust core (`crossbeam`, `parking_lot`)
- Zero-copy I/O where possible
- Connection pooling for efficient resource management
- WAL uses batched writes to reduce I/O overhead
- Query optimizer uses cost-based planning
- Vectorized execution for analytical queries

## Notes for AI Assistants

- When making changes, preserve the hybrid Go/Rust architecture
- Always run tests after storage layer modifications
- For FFI changes, update both Rust and Go sides
- Keep transaction boundaries clear when modifying data paths
- Admin UI uses Vite proxy in dev, ensure API endpoints match
- Windows development: use PowerShell commands where shell scripts exist
- Build system handles cross-compilation and FFI library placement
