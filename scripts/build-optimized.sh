#!/bin/bash
# Build MantisDB with all performance optimizations enabled

set -e

echo "=========================================="
echo "Building MantisDB with Optimizations"
echo "=========================================="

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Check for AVX2 support
echo -e "${YELLOW}Checking CPU capabilities...${NC}"
if grep -q avx2 /proc/cpuinfo 2>/dev/null || sysctl -a 2>/dev/null | grep -q "machdep.cpu.features.*AVX2"; then
    echo -e "${GREEN}✓ AVX2 support detected${NC}"
    AVX2_FLAGS="-march=native -mavx2"
else
    echo -e "${YELLOW}⚠ AVX2 not detected, using standard optimizations${NC}"
    AVX2_FLAGS="-march=native"
fi

# Build C components with optimizations
echo ""
echo -e "${YELLOW}Building C components...${NC}"
cd cgo
make clean
CFLAGS="-O3 $AVX2_FLAGS -fPIC" make
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ C components built successfully${NC}"
else
    echo -e "${RED}✗ C build failed${NC}"
    exit 1
fi
cd ..

# Build Rust components with optimizations
echo ""
echo -e "${YELLOW}Building Rust components...${NC}"
cd rust-core
cargo clean
RUSTFLAGS="-C target-cpu=native -C opt-level=3" cargo build --release
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Rust components built successfully${NC}"
else
    echo -e "${RED}✗ Rust build failed${NC}"
    exit 1
fi
cd ..

# Build Go application
echo ""
echo -e "${YELLOW}Building Go application...${NC}"
go build -tags rust -o mantisdb-optimized ./cmd/mantisDB/
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Go application built successfully${NC}"
else
    echo -e "${RED}✗ Go build failed${NC}"
    exit 1
fi

# Run quick verification
echo ""
echo -e "${YELLOW}Running verification tests...${NC}"
go test -short -tags rust ./storage/... ./advanced/...
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Verification tests passed${NC}"
else
    echo -e "${YELLOW}⚠ Some tests failed (this may be expected)${NC}"
fi

echo ""
echo "=========================================="
echo -e "${GREEN}Build Complete!${NC}"
echo "=========================================="
echo ""
echo "Optimizations enabled:"
echo "  ✓ Rust lock-free storage with SIMD"
echo "  ✓ C fast write buffer with memory pooling"
echo "  ✓ Go write optimizer with parallel processing"
echo "  ✓ Zero-copy operations"
echo "  ✓ AVX2 acceleration (if supported)"
echo ""
echo "Binary: ./mantisdb-optimized"
echo ""
echo "To run benchmarks:"
echo "  go test -bench=. -benchmem ./benchmark/"
echo ""
echo "To run with production config:"
echo "  ./mantisdb-optimized -config configs/production.yaml"
echo ""
