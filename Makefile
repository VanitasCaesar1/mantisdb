# MantisDB Makefile

.PHONY: build run test benchmark clean help build-admin build-frontend build-clients build-all cross-platform production release install

# Build configuration
BINARY_NAME=mantisdb
ADMIN_PORT=8081
FRONTEND_DIR=admin/frontend
ASSETS_DIR=admin/assets/dist

# Platform targets for cross-compilation
PLATFORMS=linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

# Default target
help:
	@echo "MantisDB - Multi-Model Database with Admin Dashboard"
	@echo ""
	@echo "Available targets:"
	@echo "  build           - Build the MantisDB binary with embedded admin dashboard"
	@echo "  build-frontend  - Build the React frontend"
	@echo "  build-admin     - Build the admin API server"
	@echo "  build-clients   - Build all client libraries"
	@echo "  build-all       - Build everything (frontend, admin, clients, main binary)"
	@echo "  cross-platform  - Build for all supported platforms"
	@echo "  production      - Build production-ready binaries with installers"
	@echo "  release         - Create GitHub release with all artifacts"
	@echo "  install         - Install MantisDB locally"
	@echo "  run             - Build and run MantisDB with admin dashboard"
	@echo "  run-dev         - Run in development mode with hot reload"
	@echo "  test            - Run the test suite"
	@echo "  benchmark       - Run benchmarks only"
	@echo "  clean           - Clean all build artifacts"
	@echo "  help            - Show this help message"

# Build the complete binary with embedded admin dashboard
build: build-frontend
	@echo "Building MantisDB with embedded admin dashboard..."
	@VERSION=$$(git describe --tags --abbrev=0 2>/dev/null || echo "dev"); \
	BUILD_TIME=$$(date -u +%Y-%m-%dT%H:%M:%SZ); \
	GIT_COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	go build -ldflags="-s -w -X main.Version=$$VERSION -X main.BuildTime=$$BUILD_TIME -X main.GitCommit=$$GIT_COMMIT" \
		-o $(BINARY_NAME) cmd/mantisDB/main.go
	@echo "Build complete: ./$(BINARY_NAME)"

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
	@mkdir -p dist
	@for platform in $(PLATFORMS); do \
		platform_split=($${platform//\// }); \
		GOOS=$${platform_split[0]}; \
		GOARCH=$${platform_split[1]}; \
		output_name=$(BINARY_NAME)-$$GOOS-$$GOARCH; \
		if [ $$GOOS = "windows" ]; then output_name+='.exe'; fi; \
		echo "Building for $$GOOS/$$GOARCH..."; \
		env GOOS=$$GOOS GOARCH=$$GOARCH CGO_ENABLED=0 go build -ldflags="-s -w" -o dist/$$output_name cmd/mantisDB/main.go; \
	done
	@echo "Cross-platform builds complete in ./dist/"

# Production builds with installers
production:
	@echo "Building production release..."
	@./scripts/build-production.sh

# Create GitHub release
release: production
	@echo "Creating GitHub release..."
	@./scripts/release.sh

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
	rm -f $(BINARY_NAME) admin-server
	rm -rf data/ test_data/ dist/
	rm -rf $(ASSETS_DIR)/*
	rm -rf $(FRONTEND_DIR)/node_modules $(FRONTEND_DIR)/dist
	rm -rf clients/javascript/node_modules clients/javascript/dist
	rm -rf clients/python/build clients/python/dist clients/python/*.egg-info
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