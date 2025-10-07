#!/bin/bash
set -e

echo "Building MantisDB Rust Core..."

# Build Rust library
cd "$(dirname "$0")"
cargo build --release

echo "✓ Rust core built successfully"
echo "Library location: target/release/libmantisdb_core.a"
echo ""
echo "To use in Go:"
echo "  go build -tags rust ..."
