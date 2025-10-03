# MantisDB Project Structure

This document describes the project structure for MantisDB with the integrated admin dashboard and production features.

## Directory Structure

```
mantisdb/
├── admin/                          # Admin dashboard components
│   ├── api/                        # REST API server for dashboard
│   │   ├── server.go              # Main API server with embedded assets
│   │   └── go.mod                 # Go module for admin API
│   ├── frontend/                   # React-based web interface
│   │   ├── src/                   # React source code
│   │   ├── package.json           # Frontend dependencies
│   │   ├── vite.config.ts         # Vite build configuration
│   │   ├── tailwind.config.js     # Tailwind CSS with mantis theme
│   │   └── tsconfig.json          # TypeScript configuration
│   └── assets/                     # Static assets and build output
│       └── dist/                  # Built frontend assets (embedded)
├── clients/                        # Official client libraries
│   ├── go/                        # Go SDK
│   │   ├── client.go              # Go client implementation
│   │   └── go.mod                 # Go client module
│   ├── python/                    # Python SDK
│   │   ├── mantisdb/              # Python package
│   │   │   ├── __init__.py        # Package initialization
│   │   │   ├── client.py          # Sync/async client implementation
│   │   │   └── exceptions.py      # Client exceptions
│   │   └── pyproject.toml         # Python package configuration
│   └── javascript/                # JavaScript/TypeScript SDK
│       ├── src/                   # TypeScript source code
│       │   └── index.ts           # Main client implementation
│       ├── package.json           # JavaScript package configuration
│       └── tsconfig.json          # TypeScript configuration
├── advanced/                       # Advanced production features
│   ├── backup/                    # Hot backup system
│   │   └── manager.go             # Backup manager implementation
│   ├── concurrency/               # Advanced concurrency control
│   │   └── rwlock.go              # Read-write lock manager
│   ├── memory/                    # Memory management and caching
│   │   └── cache_manager.go       # Cache manager with eviction policies
│   ├── logging/                   # Structured logging system
│   │   └── structured.go          # JSON structured logger
│   ├── metrics/                   # Prometheus metrics and observability
│   │   └── prometheus.go          # Prometheus metrics exporter
│   └── compression/               # Data compression for cold storage
│       └── engine.go              # Compression engine
├── scripts/                        # Build and development scripts
│   ├── build.sh                   # Complete build script
│   └── dev.sh                     # Development environment script
├── build.config.yaml              # Build system configuration
├── Makefile                       # Enhanced Makefile with admin dashboard support
└── PROJECT_STRUCTURE.md           # This file
```

## Build System

### Make Targets

- `make build` - Build complete MantisDB binary with embedded admin dashboard
- `make build-frontend` - Build React frontend only
- `make build-admin` - Build admin API server only
- `make build-clients` - Build all client libraries
- `make build-all` - Build everything (frontend, admin, clients, main binary)
- `make cross-platform` - Build for all supported platforms
- `make run` - Build and run MantisDB with admin dashboard
- `make run-dev` - Run in development mode with hot reload
- `make clean` - Clean all build artifacts
- `make deps` - Install all dependencies

### Scripts

- `scripts/build.sh` - Comprehensive build script with dependency checking
- `scripts/dev.sh` - Development environment with hot reload

### Configuration

- `build.config.yaml` - Centralized build configuration
- Platform targets: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)

## Development Workflow

1. **Install dependencies**: `make deps`
2. **Development mode**: `make run-dev` or `./scripts/dev.sh`
3. **Build for production**: `make build` or `./scripts/build.sh`
4. **Cross-platform builds**: `make cross-platform`

## Admin Dashboard

- **Frontend**: React with TypeScript, Tailwind CSS, mantis theme
- **API**: Go REST API server with WebSocket support
- **Assets**: Embedded using Go embed for single binary distribution
- **Development**: Hot reload with Vite dev server
- **Production**: Optimized build with asset bundling

## Client Libraries

- **Go**: Idiomatic Go interfaces with connection pooling
- **Python**: Sync/async support with type hints and Pydantic
- **JavaScript**: Node.js and browser support with TypeScript definitions

## Advanced Features

- **Hot Backup**: Snapshot-based backups without downtime
- **Concurrency**: Read-write locks with deadlock detection
- **Memory Management**: Configurable cache with LRU/LFU/TTL eviction
- **Logging**: Structured JSON logging with contextual information
- **Metrics**: Prometheus-compatible metrics and health checks
- **Compression**: Automatic compression for cold data

## Embedded Assets

The admin dashboard frontend is embedded directly into the Go binary using the `embed` package, creating a single self-contained executable with no external dependencies.