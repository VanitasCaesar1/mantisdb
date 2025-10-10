.PHONY: build test bench clean install run-api

# Build the Rust core library
build:
	cargo build --release

# Build with all features
build-all:
	cargo build --release --all-features

# Run tests
test:
	cargo test --release

# Run benchmarks
bench:
	cargo bench

# Run the REST API server example
run-api:
	cargo run --example rest_api_server --release

# Clean build artifacts
clean:
	cargo clean

# Install to system
install: build
	mkdir -p ../lib
	cp target/release/libmantisdb_core.a ../lib/
	cp target/release/libmantisdb_core.so ../lib/ 2>/dev/null || true
	cp target/release/libmantisdb_core.dylib ../lib/ 2>/dev/null || true

# Format code
fmt:
	cargo fmt

# Run clippy lints
lint:
	cargo clippy --all-targets --all-features -- -D warnings

# Check code without building
check:
	cargo check --all-targets --all-features

# Build documentation
docs:
	cargo doc --no-deps --open

# Run all checks (fmt, lint, test)
ci: fmt lint test

# Development build (faster compilation)
dev:
	cargo build

# Watch and rebuild on changes
watch:
	cargo watch -x build

# Profile the application
profile:
	cargo build --release --profile bench
	cargo flamegraph --example rest_api_server

help:
	@echo "MantisDB Rust Core - Available targets:"
	@echo "  build      - Build release version"
	@echo "  build-all  - Build with all features"
	@echo "  test       - Run tests"
	@echo "  bench      - Run benchmarks"
	@echo "  run-api    - Run REST API server"
	@echo "  clean      - Clean build artifacts"
	@echo "  install    - Install library to ../lib"
	@echo "  fmt        - Format code"
	@echo "  lint       - Run clippy lints"
	@echo "  check      - Check code without building"
	@echo "  docs       - Build and open documentation"
	@echo "  ci         - Run all CI checks"
	@echo "  dev        - Development build"
	@echo "  watch      - Watch and rebuild on changes"
	@echo "  profile    - Profile the application"
