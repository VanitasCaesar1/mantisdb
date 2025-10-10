#!/bin/bash
# Build the C storage engine library

set -e

echo "Building C storage engine..."
cd "$(dirname "$0")/../cgo"

# Build the shared library
make clean
make

echo "C storage engine built successfully: libstorage_engine.so"
echo "Library location: $(pwd)/libstorage_engine.so"
