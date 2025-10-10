#!/bin/bash

# MantisDB Optimized Build Script
# Builds the Rust admin server with maximum optimizations

set -e

echo "🦀 Building MantisDB Admin Server (Optimized)"
echo "=============================================="

cd rust-core

# Clean previous builds
echo "🧹 Cleaning previous builds..."
cargo clean

# Build with release optimizations
echo "🔨 Building with release optimizations..."
RUSTFLAGS="-C target-cpu=native -C opt-level=3" \
  cargo build --release --bin admin-server

# Check binary size
BINARY_SIZE=$(du -h target/release/admin-server | cut -f1)
echo "📦 Binary size: $BINARY_SIZE"

# Run tests
echo "🧪 Running tests..."
cargo test --release

echo ""
echo "✅ Build complete!"
echo "📍 Binary location: rust-core/target/release/admin-server"
echo ""
echo "To run the server:"
echo "  ./rust-core/target/release/admin-server"
echo ""
echo "Performance tips:"
echo "  - Binary is optimized for your CPU architecture"
echo "  - LTO (Link Time Optimization) enabled"
echo "  - Debug symbols stripped"
echo "  - Panic=abort for smaller binary"
