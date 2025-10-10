# MantisDB Makefile
# Unified build system with Connection Pooling & REST API

.PHONY: all build test clean install run dev build-rust run-api help

# Build configuration
BINARY_NAME=mantisdb
VERSION?=$(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
ADMIN_PORT=8081
FRONTEND_DIR=admin/frontend
RUST_CORE_DIR=rust-core

LDFLAGS=-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)

# Default target
all: build

# Help target
help:
	@echo "╔══════════════════════════════════════════════════════════════╗"
	@echo "║       MantisDB - High-Performance Database with Pool         ║"
	@echo "╚══════════════════════════════════════════════════════════════╝"
	@echo ""
	@echo "QUICK START:"
	@echo "  make build          Build MantisDB (includes Rust pool & API)"
	@echo "  ./build.sh          Same as make build"
	@echo "  make run            Build and run with admin dashboard"
	@echo "  make run-api        Run standalone REST API server"
	@echo ""
	@echo "CONNECTION POOLING & REST API:"
	@echo "  make run-api        Start REST API server (http://0.0.0.0:8080)"
	@echo "  make build-rust     Build Rust core (pool + API)"
	@echo "  make test-api       Test REST API endpoints"
	@echo ""
	@echo "DEVELOPMENT:"
	@echo "  make test           Run all tests"
	@echo "  make clean          Clean build artifacts"
	@echo "  make install        Install to ~/.local/bin"
	@echo "  make bench          Run benchmarks"
	@echo ""
	@echo "📚 Documentation:"
	@echo "  docs/connection-pooling.md - Pool usage guide"
	@echo "  docs/rest-api.md          - API reference"
	@echo "  QUICK_START_POOLING.md    - 5-minute quick start"
	@echo ""
	@echo "Current version: $(VERSION)"

# Build MantisDB with Rust integration
build:
	@./build-unified.sh

# Build Rust core library (connection pool + REST API)
build-rust:
	@echo "🦀 Building Rust core (Connection Pool + REST API)..."
	@cd $(RUST_CORE_DIR) && cargo build --release
	@mkdir -p lib
	@cp $(RUST_CORE_DIR)/target/release/libmantisdb_core.a lib/ 2>/dev/null || true
	@cp $(RUST_CORE_DIR)/target/release/libmantisdb_core.so lib/ 2>/dev/null || true
	@cp $(RUST_CORE_DIR)/target/release/libmantisdb_core.dylib lib/ 2>/dev/null || true
	@echo "✅ Rust core library built successfully"

# Run standalone REST API server with connection pooling
run-api:
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "🚀 Starting MantisDB REST API Server"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "📡 Server: http://0.0.0.0:8080"
	@echo "🏥 Health: http://localhost:8080/health"
	@echo "📊 Stats:  http://localhost:8080/stats"
	@echo ""
	@echo "⚡ Features:"
	@echo "  • Connection Pool: 100k+ ops/sec"
	@echo "  • REST API: 50k+ req/sec"
	@echo "  • Sub-millisecond latency"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@cd $(RUST_CORE_DIR) && cargo run --example rest_api_server --release

# Test REST API endpoints
test-api:
	@echo "🧪 Testing REST API endpoints..."
	@echo "Health check:"
	@curl -s http://localhost:8080/health | jq || echo "Server not running. Start with: make run-api"
	@echo ""
	@echo "Pool statistics:"
	@curl -s http://localhost:8080/stats | jq || echo "Server not running"

# Run tests
test:
	@echo "Running Go tests..."
	@go test -v ./...
	@echo "Running Rust tests..."
	@cd $(RUST_CORE_DIR) && cargo test --release

# Run benchmarks
bench:
	@echo "Running Rust benchmarks..."
	@cd $(RUST_CORE_DIR) && cargo bench
	@echo "Running Go benchmarks..."
	@go test -bench=. -benchmem ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf ./data ./wal ./backups ./lib ./dist
	@rm -f $(BINARY_NAME) test
	@go clean
	@if [ -d "$(RUST_CORE_DIR)" ]; then cd $(RUST_CORE_DIR) && cargo clean; fi
	@echo "✅ Clean complete"

# Install to local bin
install: build
	@echo "Installing MantisDB..."
	@mkdir -p ~/.local/bin
	@cp $(BINARY_NAME) ~/.local/bin/
	@echo "✅ MantisDB installed to ~/.local/bin/$(BINARY_NAME)"
	@echo "Make sure ~/.local/bin is in your PATH"

# Build and run with admin dashboard
run: build
	@echo "Starting MantisDB with admin dashboard..."
	@echo "Database: http://localhost:8080"
	@echo "Admin dashboard: http://localhost:$(ADMIN_PORT)"
	./$(BINARY_NAME) --admin-port=$(ADMIN_PORT)

# Development mode
dev:
	@echo "Starting development mode..."
	@go run cmd/mantisDB/main.go

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@cd $(RUST_CORE_DIR) && cargo fmt

# Lint code
lint:
	@echo "Linting code..."
	@golangci-lint run || echo "golangci-lint not installed"
	@cd $(RUST_CORE_DIR) && cargo clippy --all-targets --all-features -- -D warnings

# Show project stats
stats:
	@echo "Project Statistics:"
	@echo "Go files: $$(find . -name '*.go' | wc -l)"
	@echo "Rust files: $$(find $(RUST_CORE_DIR)/src -name '*.rs' | wc -l)"
	@echo "Total lines (Go): $$(find . -name '*.go' -exec cat {} \; | wc -l)"
	@echo "Total lines (Rust): $$(find $(RUST_CORE_DIR)/src -name '*.rs' -exec cat {} \; | wc -l)"
