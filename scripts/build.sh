#!/bin/bash

# MantisDB Build Script
# This script handles the complete build process including frontend compilation

set -e

echo "🔨 MantisDB Build Script"
echo "======================="

# Check for required tools
check_dependencies() {
    echo "📋 Checking dependencies..."
    
    if ! command -v go &> /dev/null; then
        echo "❌ Go is required but not installed"
        exit 1
    fi
    
    if ! command -v node &> /dev/null; then
        echo "❌ Node.js is required but not installed"
        exit 1
    fi
    
    if ! command -v npm &> /dev/null; then
        echo "❌ npm is required but not installed"
        exit 1
    fi
    
    echo "✅ All dependencies found"
}

# Build frontend assets
build_frontend() {
    echo "🎨 Building frontend assets..."
    cd admin/frontend
    
    if [ ! -d "node_modules" ]; then
        echo "📦 Installing frontend dependencies..."
        npm install
    fi
    
    echo "🏗️  Building React application..."
    npm run build
    
    cd ../..
    echo "✅ Frontend build complete"
}

# Build Go binary with embedded assets
build_binary() {
    echo "🚀 Building MantisDB binary..."
    
    # Set build flags
    BUILD_FLAGS="-ldflags=-s -w"
    
    # Add version information if available
    if [ -n "$VERSION" ]; then
        BUILD_FLAGS="$BUILD_FLAGS -X main.version=$VERSION"
    fi
    
    # Build the binary
    go build -ldflags="-s -w" -o mantisdb main.go
    
    echo "✅ Binary build complete: ./mantisdb"
}

# Build client libraries
build_clients() {
    echo "📚 Building client libraries..."
    
    # Go client
    echo "  🐹 Building Go client..."
    cd clients/go && go build ./... && cd ../..
    
    # JavaScript client
    echo "  🟨 Building JavaScript client..."
    cd clients/javascript
    if [ ! -d "node_modules" ]; then
        npm install
    fi
    npm run build
    cd ../..
    
    # Python client (if Python is available)
    if command -v python3 &> /dev/null; then
        echo "  🐍 Building Python client..."
        cd clients/python && python3 -m pip install -e . --break-system-packages 2>/dev/null || echo "  ⚠️  Python client build skipped (environment restrictions)" && cd ../..
    else
        echo "  ⚠️  Python3 not found, skipping Python client"
    fi
    
    echo "✅ Client libraries build complete"
}

# Main build process
main() {
    check_dependencies
    build_frontend
    build_clients
    build_binary
    
    echo ""
    echo "🎉 Build complete!"
    echo "   Binary: ./mantisdb"
    echo "   Run with: ./mantisdb --admin-port=8081"
}

# Run main function
main "$@"