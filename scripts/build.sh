#!/bin/bash

# MantisDB Build Script
# This script handles the complete build process including frontend compilation

set -e

# Configuration
VERSION=${VERSION:-"1.0.0"}
BUILD_INSTALLERS=${BUILD_INSTALLERS:-false}
BUILD_CROSS_PLATFORM=${BUILD_CROSS_PLATFORM:-false}

echo "🔨 MantisDB Build Script"
echo "======================="
echo "Version: $VERSION"
echo "Build Installers: $BUILD_INSTALLERS"
echo "Cross Platform: $BUILD_CROSS_PLATFORM"
echo ""

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
    
    if [ "$BUILD_CROSS_PLATFORM" = true ]; then
        build_cross_platform
    else
        # Build for current platform
        go build $BUILD_FLAGS -o mantisdb cmd/mantisDB/main.go
        echo "✅ Binary build complete: ./mantisdb"
    fi
}

# Build cross-platform binaries
build_cross_platform() {
    echo "🌍 Building cross-platform binaries..."
    
    mkdir -p dist
    
    # Define target platforms
    declare -A platforms=(
        ["linux/amd64"]="mantisdb-linux-amd64"
        ["linux/arm64"]="mantisdb-linux-arm64"
        ["darwin/amd64"]="mantisdb-darwin-amd64"
        ["darwin/arm64"]="mantisdb-darwin-arm64"
        ["windows/amd64"]="mantisdb-windows-amd64.exe"
    )
    
    for platform in "${!platforms[@]}"; do
        IFS='/' read -r GOOS GOARCH <<< "$platform"
        output="${platforms[$platform]}"
        
        echo "  📦 Building for $GOOS/$GOARCH..."
        
        GOOS=$GOOS GOARCH=$GOARCH go build $BUILD_FLAGS -o "dist/$output" cmd/mantisDB/main.go
        
        # Create archives for distribution
        if [[ "$GOOS" == "windows" ]]; then
            # Windows: keep as .exe
            echo "    ✅ Created: dist/$output"
        else
            # Unix: create tar.gz
            tar -czf "dist/${output}.tar.gz" -C dist "$output"
            echo "    ✅ Created: dist/${output}.tar.gz"
        fi
    done
    
    echo "✅ Cross-platform build complete"
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

# Build installers
build_installers() {
    if [ "$BUILD_INSTALLERS" != true ]; then
        return 0
    fi
    
    echo "📦 Building installers..."
    
    # Ensure we have cross-platform binaries
    if [ ! -d "dist" ] || [ -z "$(ls -A dist/ 2>/dev/null)" ]; then
        echo "  ⚠️  No binaries found in dist/, building cross-platform first..."
        BUILD_CROSS_PLATFORM=true build_binary
    fi
    
    # Create Windows installer
    if [ -f "dist/mantisdb-windows-amd64.exe" ]; then
        echo "  🪟 Creating Windows installer..."
        ./scripts/create-installers.sh --version="$VERSION" --platform=windows
    fi
    
    # Create macOS DMG (only on macOS)
    if [[ "$OSTYPE" == "darwin"* ]] && ([ -f "dist/mantisdb-darwin-amd64" ] || [ -f "dist/mantisdb-darwin-arm64" ]); then
        echo "  🍎 Creating macOS DMG..."
        ./scripts/create-dmg.sh --version="$VERSION"
    fi
    
    # Create Homebrew formula
    if [ -f "dist/mantisdb-darwin-amd64" ] || [ -f "dist/mantisdb-darwin-arm64" ]; then
        echo "  🍺 Creating Homebrew formula..."
        ./scripts/create-homebrew.sh --version="$VERSION"
    fi
    
    # Create Linux packages
    if [ -f "dist/mantisdb-linux-amd64" ]; then
        echo "  🐧 Creating Linux packages..."
        ./scripts/create-installers.sh --version="$VERSION" --platform=linux
    fi
    
    echo "✅ Installer creation complete"
}

# Main build process
main() {
    check_dependencies
    build_frontend
    build_clients
    build_binary
    build_installers
    
    echo ""
    echo "🎉 Build complete!"
    
    if [ "$BUILD_CROSS_PLATFORM" = true ]; then
        echo "   Binaries in: ./dist/"
        ls -la dist/ | grep mantisdb || true
    else
        echo "   Binary: ./mantisdb"
        echo "   Run with: ./mantisdb --admin-port=8081"
    fi
    
    if [ "$BUILD_INSTALLERS" = true ]; then
        echo ""
        echo "📦 Installers created:"
        find dist/installers -name "*.dmg" -o -name "*.exe" -o -name "*.zip" -o -name "*.deb" -o -name "*.rpm" 2>/dev/null | while read -r file; do
            echo "   $(basename "$file")"
        done
    fi
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --version)
            VERSION="$2"
            shift 2
            ;;
        --version=*)
            VERSION="${1#*=}"
            shift
            ;;
        --installers)
            BUILD_INSTALLERS=true
            shift
            ;;
        --cross-platform)
            BUILD_CROSS_PLATFORM=true
            shift
            ;;
        --all)
            BUILD_INSTALLERS=true
            BUILD_CROSS_PLATFORM=true
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --version VERSION     Set build version (default: 1.0.0)"
            echo "  --cross-platform      Build for all platforms"
            echo "  --installers          Create platform installers"
            echo "  --all                 Build everything (cross-platform + installers)"
            echo "  --help, -h            Show this help"
            echo ""
            echo "Examples:"
            echo "  $0                           # Build for current platform only"
            echo "  $0 --cross-platform          # Build for all platforms"
            echo "  $0 --all --version=1.2.0     # Full release build"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Run main function
main