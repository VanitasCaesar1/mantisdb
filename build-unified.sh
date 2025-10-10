#!/bin/bash
# MantisDB Unified Build Script
# Builds Rust core, Admin UI, and optional Go components

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
RUST_CORE_DIR="rust-core"
ADMIN_UI_DIR="admin/frontend"
BUILD_CONFIG="build.config.yaml"

# Parse command line arguments
BUILD_TYPE="${1:-dev}"
SKIP_TESTS="${SKIP_TESTS:-false}"
SKIP_UI="${SKIP_UI:-false}"

print_header() {
    echo -e "${BLUE}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║           MantisDB Unified Build System                      ║${NC}"
    echo -e "${BLUE}╚══════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

print_step() {
    echo -e "${GREEN}▶ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ Error: $1${NC}"
    exit 1
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

# Check prerequisites
check_prerequisites() {
    print_step "Checking prerequisites..."
    
    # Check Rust
    if ! command -v cargo &> /dev/null; then
        print_error "Rust/Cargo not found. Install from https://rustup.rs/"
    fi
    print_success "Rust installed: $(rustc --version)"
    
    # Check Node.js (for admin UI)
    if [ "$SKIP_UI" != "true" ]; then
        if ! command -v node &> /dev/null; then
            print_warning "Node.js not found. Skipping admin UI build"
            SKIP_UI="true"
        else
            print_success "Node.js installed: $(node --version)"
        fi
    fi
    
    echo ""
}

# Build Rust core
build_rust_core() {
    print_step "Building Rust core (Admin API + RLS Engine)..."
    
    cd "$RUST_CORE_DIR"
    
    if [ "$BUILD_TYPE" == "release" ]; then
        cargo build --release --lib
        cargo build --release --bin admin-server
        print_success "Rust core built in release mode"
        
        # Copy artifacts
        mkdir -p ../lib
        cp target/release/libmantisdb_core.* ../lib/ 2>/dev/null || true
        cp target/release/admin-server ../bin/admin-server 2>/dev/null || true
    else
        cargo build --lib
        cargo build --bin admin-server
        print_success "Rust core built in debug mode"
        
        # Copy artifacts
        mkdir -p ../lib
        cp target/debug/libmantisdb_core.* ../lib/ 2>/dev/null || true
        cp target/debug/admin-server ../bin/admin-server 2>/dev/null || true
    fi
    
    cd ..
    echo ""
}

# Run Rust tests
test_rust_core() {
    if [ "$SKIP_TESTS" != "true" ]; then
        print_step "Running Rust tests..."
        cd "$RUST_CORE_DIR"
        cargo test --lib
        print_success "Rust tests passed"
        cd ..
        echo ""
    fi
}

# Build Admin UI
build_admin_ui() {
    if [ "$SKIP_UI" != "true" ]; then
        print_step "Building Admin UI..."
        
        if [ -d "$ADMIN_UI_DIR" ]; then
            cd "$ADMIN_UI_DIR"
            
            # Install dependencies if needed
            if [ ! -d "node_modules" ]; then
                print_step "Installing npm dependencies..."
                npm install
            fi
            
            # Build
            npm run build
            print_success "Admin UI built successfully"
            cd ../../..
        else
            print_warning "Admin UI directory not found, skipping"
        fi
        echo ""
    fi
}

# Create bin directory
prepare_directories() {
    print_step "Preparing directories..."
    mkdir -p bin lib data logs
    print_success "Directories created"
    echo ""
}

# Display build summary
show_summary() {
    echo -e "${GREEN}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║                  Build Completed Successfully                ║${NC}"
    echo -e "${GREEN}╚══════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo "📦 Artifacts:"
    echo "  • Rust Admin API:     bin/admin-server"
    echo "  • Rust Core Library:  lib/libmantisdb_core.*"
    if [ "$SKIP_UI" != "true" ]; then
        echo "  • Admin UI:           admin/frontend/dist/"
    fi
    echo ""
    echo "🚀 Run MantisDB:"
    echo "  # Start admin server"
    echo "  ./bin/admin-server"
    echo ""
    echo "  # Or use Docker"
    echo "  docker-compose up"
    echo ""
    echo "📊 Admin Dashboard: http://localhost:8081"
    echo "📚 Documentation:   ./docs/README.md"
    echo ""
}

# Main build process
main() {
    print_header
    
    case "$BUILD_TYPE" in
        release|prod|production)
            BUILD_TYPE="release"
            echo "Build mode: RELEASE (optimized)"
            ;;
        dev|development|debug)
            BUILD_TYPE="dev"
            echo "Build mode: DEVELOPMENT (debug symbols)"
            ;;
        *)
            print_error "Invalid build type. Use: dev, release"
            ;;
    esac
    echo ""
    
    check_prerequisites
    prepare_directories
    build_rust_core
    test_rust_core
    build_admin_ui
    show_summary
}

# Run main
main
