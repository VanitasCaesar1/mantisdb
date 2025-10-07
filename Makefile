# MantisDB Makefile
# Unified build system for all platforms

.PHONY: build run test benchmark clean help build-admin build-frontend build-clients build-all cross-platform production release install installers

# Build configuration
BINARY_NAME=mantisdb
VERSION?=$(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
ADMIN_PORT=8081
FRONTEND_DIR=admin/frontend
ASSETS_DIR=admin/assets/dist
RUST_CORE_DIR=rust-core

# Build mode: go (pure Go) or rust (with Rust core)
BUILD_MODE?=go

# Platform targets for cross-compilation
PLATFORMS=linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

# Build flags
LDFLAGS=-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)

# Default target
help:
	@echo "╔══════════════════════════════════════════════════════════════╗"
	@echo "║         MantisDB - Multi-Model Database Builder             ║"
	@echo "╚══════════════════════════════════════════════════════════════╝"
	@echo ""
	@echo "QUICK START:"
	@echo "  make build          Build for current platform (Pure Go)"
	@echo "  make build-rust     Build with Rust high-performance core"
	@echo "  make run            Build and run with admin dashboard"
	@echo "  make production     Full production build with installers"
	@echo ""
	@echo "BUILD TARGETS:"
	@echo "  build              Build MantisDB binary (current platform)"
	@echo "  build-rust         Build with Rust core for 5000+ ops/sec"
	@echo "  build-all          Build everything (frontend + admin + clients + binary)"
	@echo "  build-frontend     Build React admin dashboard"
	@echo "  build-admin        Build standalone admin API server"
	@echo "  build-clients      Build all client libraries (Go, Python, JS)"
	@echo "  cross-platform     Build for all platforms (Linux, macOS, Windows)"
	@echo "  installers         Create platform-specific installers"
	@echo "  production         Full production build (cross-platform + installers)"
	@echo "  release            Create GitHub release with all artifacts"
	@echo ""
	@echo "DEVELOPMENT:"
	@echo "  run                Build and run with admin dashboard"
	@echo "  run-dev            Run in development mode with hot reload"
	@echo "  test               Run test suite"
	@echo "  benchmark          Run benchmarks"
	@echo "  fmt                Format code"
	@echo "  lint               Run linter"
	@echo ""
	@echo "INSTALLATION:"
	@echo "  install            Install to ~/.local/bin (user)"
	@echo "  deps               Install all build dependencies"
	@echo ""
	@echo "MAINTENANCE:"
	@echo "  clean              Clean all build artifacts"
	@echo "  stats              Show project statistics"
	@echo ""
	@echo "EXAMPLES:"
	@echo "  make build VERSION=1.2.3           Build with specific version"
	@echo "  make cross-platform                Build for all platforms"
	@echo "  make production VERSION=1.2.3      Full release build"
	@echo ""
	@echo "DOCUMENTATION:"
	@echo "  See BUILD.md for detailed build instructions"
	@echo "  See INSTALL.md for installation guide"
	@echo ""
	@echo "Current version: $(VERSION)"
	@echo "Supported platforms: $(PLATFORMS)"

# Build the complete binary with embedded admin dashboard
build: build-frontend
	@echo "Building MantisDB with embedded admin dashboard (Pure Go)..."
	@echo "Version: $(VERSION)"
	go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) cmd/mantisDB/main.go
	@echo "Build complete: ./$(BINARY_NAME)"

# Build with Rust high-performance core
build-rust: build-rust-core build-frontend
	@echo "Building MantisDB with Rust core (High Performance)..."
	@echo "Version: $(VERSION)"
	@echo "This build includes:"
	@echo "  - Lock-free storage engine"
	@echo "  - Lock-free LRU cache"
	@echo "  - Target: 5000+ ops/sec throughput"
	go build -tags rust -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)-rust cmd/mantisDB/main.go
	@echo "Build complete: ./$(BINARY_NAME)-rust"
	@echo ""
	@echo "Performance comparison:"
	@echo "  Pure Go:    ~11K ops/sec (high throughput)"
	@echo "  Rust Core:  50K+ ops/sec (target)"

# Build Rust core library
build-rust-core:
	@echo "Building Rust core library..."
	@cd $(RUST_CORE_DIR) && cargo build --release
	@echo "Rust core build complete"

# Run Rust benchmarks
bench-rust:
	@echo "Running Rust core benchmarks..."
	@cd $(RUST_CORE_DIR) && cargo bench
	@echo "Benchmark results saved to rust-core/target/criterion/"

# Build the React frontend
build-frontend:
	@echo "Building admin dashboard frontend..."
	@if [ ! -d "$(FRONTEND_DIR)/node_modules" ]; then \
		echo "Installing frontend dependencies..."; \
		cd $(FRONTEND_DIR) && npm install; \
	fi
	@cd $(FRONTEND_DIR) && npm run build
	@echo "Frontend build complete"

# Build the admin API server (standalone)
build-admin:
	@echo "Building admin API server..."
	cd admin/api && go build -o ../../admin-server .
	@echo "Admin server build complete: ./admin-server"

# Build all client libraries
build-clients: build-client-go build-client-python build-client-js

# Build Go client library
build-client-go:
	@echo "Building Go client library..."
	cd clients/go && go build ./...
	@echo "Go client build complete"

# Build Python client library
build-client-python:
	@echo "Building Python client library..."
	@if command -v python3 >/dev/null 2>&1; then \
		cd clients/python && python3 -m pip install -e .; \
	else \
		echo "Python3 not found, skipping Python client build"; \
	fi

# Build JavaScript client library
build-client-js:
	@echo "Building JavaScript client library..."
	@if [ ! -d "clients/javascript/node_modules" ]; then \
		echo "Installing JavaScript client dependencies..."; \
		cd clients/javascript && npm install; \
	fi
	@cd clients/javascript && npm run build
	@echo "JavaScript client build complete"

# Build everything
build-all: build-frontend build-admin build-clients build
	@echo "All components built successfully"

# Cross-platform builds
cross-platform: build-frontend
	@echo "Building for multiple platforms..."
	@echo "Version: $(VERSION)"
	@mkdir -p dist
	@for platform in $(PLATFORMS); do \
		platform_split=($${platform//\// }); \
		GOOS=$${platform_split[0]}; \
		GOARCH=$${platform_split[1]}; \
		output_name=$(BINARY_NAME)-$$GOOS-$$GOARCH; \
		if [ $$GOOS = "windows" ]; then output_name+='.exe'; fi; \
		echo "Building for $$GOOS/$$GOARCH..."; \
		env GOOS=$$GOOS GOARCH=$$GOARCH CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o dist/$$output_name cmd/mantisDB/main.go; \
	done
	@echo "Cross-platform builds complete in ./dist/"

# Create installers for all platforms
installers: cross-platform
	@echo "Creating installers for all platforms..."
	@./scripts/create-installers.sh --version=$(VERSION)
	@echo "Installers created in ./dist/installers/"

# Production builds with installers
production: installers
	@echo "Production build complete!"
	@echo "Version: $(VERSION)"
	@echo "Artifacts in: ./dist/"

# Create GitHub release
release: production
	@echo "Creating GitHub release..."
	@./scripts/build-release.sh --version=$(VERSION)

# Install locally
install: build
	@echo "Installing MantisDB locally..."
	@mkdir -p ~/.local/bin
	@cp $(BINARY_NAME) ~/.local/bin/
	@echo "MantisDB installed to ~/.local/bin/$(BINARY_NAME)"
	@echo "Make sure ~/.local/bin is in your PATH"

# Build and run with admin dashboard
run: build
	@echo "Starting MantisDB with admin dashboard..."
	@echo "Database will be available on port 8080"
	@echo "Admin dashboard will be available on port $(ADMIN_PORT)"
	./$(BINARY_NAME) --admin-port=$(ADMIN_PORT)

# Development mode with hot reload
run-dev:
	@echo "Starting development servers..."
	@echo "Starting frontend dev server..."
	@cd $(FRONTEND_DIR) && npm run dev &
	@echo "Building and starting MantisDB..."
	@$(MAKE) build-admin
	@./admin-server &
	@echo "Development servers started:"
	@echo "  Frontend: http://localhost:3000"
	@echo "  Admin API: http://localhost:$(ADMIN_PORT)"
	@echo "Press Ctrl+C to stop all servers"

# Run the test suite
test:
	@echo "Running Go tests..."
	go test ./...
	@echo "Running integration tests..."
	go run test_mantisdb.go

# Run benchmarks only
benchmark: build
	@echo "Running benchmarks..."
	./mantisdb --benchmark-only

# Run with benchmarks after startup
run-with-benchmark: build
	@echo "Starting MantisDB with benchmarks..."
	./mantisdb --benchmark

# Clean all build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY_NAME) $(BINARY_NAME)-rust admin-server
	rm -rf data/ test_data/ dist/
	rm -rf $(ASSETS_DIR)/*
	rm -rf $(FRONTEND_DIR)/node_modules $(FRONTEND_DIR)/dist
	rm -rf clients/javascript/node_modules clients/javascript/dist
	rm -rf clients/python/build clients/python/dist clients/python/*.egg-info
	@if [ -d "$(RUST_CORE_DIR)" ]; then \
		echo "Cleaning Rust artifacts..."; \
		cd $(RUST_CORE_DIR) && cargo clean; \
	fi
	@echo "Clean complete"

# Development targets
dev-run: build
	./mantisdb --log-level=debug --cache-size=52428800

dev-benchmark: build
	./mantisdb --benchmark-only --log-level=debug

# Production-like settings
prod-run: build
	./mantisdb --port=8080 --cache-size=268435456 --data-dir=/tmp/mantisdb --log-level=info

# Check code formatting
fmt:
	go fmt ./...

# Run linter (if available)
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping..."; \
	fi

# Install all dependencies
deps:
	@echo "Installing Go dependencies..."
	go mod tidy
	go mod download
	@echo "Installing frontend dependencies..."
	@cd $(FRONTEND_DIR) && npm install
	@echo "Installing JavaScript client dependencies..."
	@cd clients/javascript && npm install
	@echo "Installing Python client dependencies..."
	@if command -v python3 >/dev/null 2>&1; then \
		cd clients/python && python3 -m pip install -e .[dev]; \
	else \
		echo "Python3 not found, skipping Python dependencies"; \
	fi
	@echo "All dependencies installed"

# Show project statistics
stats:
	@echo "Project Statistics:"
	@echo "==================="
	@find . -name "*.go" -not -path "./vendor/*" | wc -l | xargs echo "Go files:"
	@find . -name "*.go" -not -path "./vendor/*" -exec cat {} \; | wc -l | xargs echo "Lines of code:"
	@du -sh . | xargs echo "Project size:"