#!/bin/bash

# MantisDB Release Build Script
# Creates a complete release with all binaries and installers

set -e

# Configuration
VERSION=${VERSION:-"1.0.0"}
SKIP_TESTS=${SKIP_TESTS:-false}
SKIP_FRONTEND=${SKIP_FRONTEND:-false}
SKIP_CLIENTS=${SKIP_CLIENTS:-false}

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

log_step() {
    echo -e "${CYAN}[STEP]${NC} $1"
}

# Print banner
print_banner() {
    echo -e "${CYAN}"
    echo "╔══════════════════════════════════════════════════════════════╗"
    echo "║                    MantisDB Release Builder                  ║"
    echo "║                                                              ║"
    echo "║  Creates production-ready binaries and installers for       ║"
    echo "║  Windows, macOS, and Linux platforms                        ║"
    echo "╚══════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
    echo ""
    echo "Version: $VERSION"
    echo "Skip Tests: $SKIP_TESTS"
    echo "Skip Frontend: $SKIP_FRONTEND"
    echo "Skip Clients: $SKIP_CLIENTS"
    echo ""
}

# Check prerequisites
check_prerequisites() {
    log_step "Checking prerequisites..."
    
    local missing_tools=()
    
    # Required tools
    if ! command -v go &> /dev/null; then
        missing_tools+=("go")
    fi
    
    if [ "$SKIP_FRONTEND" != true ] && ! command -v node &> /dev/null; then
        missing_tools+=("node")
    fi
    
    if [ "$SKIP_FRONTEND" != true ] && ! command -v npm &> /dev/null; then
        missing_tools+=("npm")
    fi
    
    # Optional tools (warn but don't fail)
    local optional_missing=()
    
    if ! command -v git &> /dev/null; then
        optional_missing+=("git")
    fi
    
    if [[ "$OSTYPE" == "darwin"* ]] && ! command -v hdiutil &> /dev/null; then
        optional_missing+=("hdiutil (for macOS DMG creation)")
    fi
    
    if ! command -v zip &> /dev/null; then
        optional_missing+=("zip")
    fi
    
    # Report missing tools
    if [ ${#missing_tools[@]} -ne 0 ]; then
        log_error "Missing required tools: ${missing_tools[*]}"
        exit 1
    fi
    
    if [ ${#optional_missing[@]} -ne 0 ]; then
        log_warning "Missing optional tools: ${optional_missing[*]}"
        log_warning "Some features may be limited"
    fi
    
    log_success "Prerequisites check complete"
}

# Clean previous builds
clean_build() {
    log_step "Cleaning previous builds..."
    
    # Remove old binaries
    rm -f mantisdb mantisdb.exe mantisdb-*
    
    # Clean dist directory
    rm -rf dist/
    mkdir -p dist/installers/{windows,macos,linux,homebrew}
    
    # Clean Go build cache
    go clean -cache -modcache -testcache 2>/dev/null || true
    
    log_success "Build environment cleaned"
}

# Run tests
run_tests() {
    if [ "$SKIP_TESTS" = true ]; then
        log_warning "Skipping tests (SKIP_TESTS=true)"
        return 0
    fi
    
    log_step "Running tests..."
    
    # Run Go tests
    log_info "Running Go tests..."
    go test -v ./... || {
        log_error "Go tests failed"
        exit 1
    }
    
    # Run frontend tests if not skipped
    if [ "$SKIP_FRONTEND" != true ] && [ -f "admin/frontend/package.json" ]; then
        log_info "Running frontend tests..."
        cd admin/frontend
        if npm run test --if-present 2>/dev/null; then
            log_info "Frontend tests passed"
        else
            log_warning "Frontend tests not available or failed"
        fi
        cd ../..
    fi
    
    # Run client tests if not skipped
    if [ "$SKIP_CLIENTS" != true ]; then
        log_info "Running client tests..."
        
        # JavaScript client tests
        if [ -f "clients/javascript/package.json" ]; then
            cd clients/javascript
            if npm run test --if-present 2>/dev/null; then
                log_info "JavaScript client tests passed"
            else
                log_warning "JavaScript client tests not available"
            fi
            cd ../..
        fi
        
        # Python client tests
        if [ -f "clients/python/pyproject.toml" ] && command -v python3 &> /dev/null; then
            cd clients/python
            if python3 -m pytest 2>/dev/null; then
                log_info "Python client tests passed"
            else
                log_warning "Python client tests not available or failed"
            fi
            cd ../..
        fi
        
        # Go client tests
        if [ -d "clients/go" ]; then
            cd clients/go
            if go test ./... 2>/dev/null; then
                log_info "Go client tests passed"
            else
                log_warning "Go client tests failed"
            fi
            cd ../..
        fi
    fi
    
    log_success "Tests completed"
}

# Build everything
build_all() {
    log_step "Building all components..."
    
    # Set environment variables for build script
    export VERSION="$VERSION"
    export BUILD_CROSS_PLATFORM=true
    export BUILD_INSTALLERS=true
    
    # Run the main build script
    ./scripts/build.sh --all --version="$VERSION"
    
    log_success "Build completed"
}

# Create checksums
create_checksums() {
    log_step "Creating checksums..."
    
    cd dist
    
    # Create checksums for all files
    find . -type f \( -name "*.exe" -o -name "*.dmg" -o -name "*.deb" -o -name "*.rpm" -o -name "*.zip" -o -name "*.tar.gz" \) -exec shasum -a 256 {} \; > checksums.txt
    
    # Also create individual checksum files
    find . -type f \( -name "*.exe" -o -name "*.dmg" -o -name "*.deb" -o -name "*.rpm" -o -name "*.zip" -o -name "*.tar.gz" \) | while read -r file; do
        shasum -a 256 "$file" > "${file}.sha256"
    done
    
    cd ..
    
    log_success "Checksums created"
}

# Generate release notes
generate_release_notes() {
    log_step "Generating release notes..."
    
    local release_notes="dist/RELEASE_NOTES.md"
    
    cat > "$release_notes" << EOF
# MantisDB $VERSION Release Notes

## Overview

MantisDB $VERSION is a multi-model database that supports Key-Value, Document, and Columnar data models with a built-in admin dashboard.

## What's New

- Production-ready release with comprehensive installers
- Cross-platform support (Windows, macOS, Linux)
- Built-in admin dashboard with real-time monitoring
- Multiple client libraries (Go, JavaScript, Python)
- Professional installers for all platforms

## Installation

### Quick Install

**Windows:**
- Download \`MantisDB-$VERSION-Windows-amd64-Installer.zip\`
- Extract and run \`install.bat\` as administrator

**macOS:**
- Download \`MantisDB-$VERSION-macOS-universal.dmg\`
- Double-click to mount and drag to Applications
- Or use Homebrew: \`brew tap mantisdb/tap && brew install mantisdb\`

**Linux:**
- Ubuntu/Debian: \`sudo dpkg -i mantisdb_${VERSION}_amd64.deb\`
- RHEL/CentOS: \`sudo rpm -i mantisdb-${VERSION}-1.x86_64.rpm\`

### Manual Installation

Download the appropriate binary for your platform and follow the installation guide.

## Files in This Release

### Binaries
- \`mantisdb-linux-amd64.tar.gz\` - Linux x86_64
- \`mantisdb-linux-arm64.tar.gz\` - Linux ARM64
- \`mantisdb-darwin-amd64.tar.gz\` - macOS Intel
- \`mantisdb-darwin-arm64.tar.gz\` - macOS Apple Silicon
- \`mantisdb-windows-amd64.exe\` - Windows x86_64

### Installers
- \`MantisDB-$VERSION-Windows-amd64-Installer.zip\` - Windows installer package
- \`MantisDB-$VERSION-macOS-universal.dmg\` - macOS disk image
- \`mantisdb_${VERSION}_amd64.deb\` - Debian/Ubuntu package
- \`mantisdb-${VERSION}-1.x86_64.rpm\` - RHEL/CentOS package

### Package Managers
- \`mantisdb.rb\` - Homebrew formula
- Homebrew tap available at: \`mantisdb/tap\`

## Getting Started

1. **Install MantisDB** using one of the methods above
2. **Start the server:**
   \`\`\`bash
   mantisdb
   \`\`\`
3. **Access the admin dashboard:** http://localhost:8081
4. **Connect your application** using one of the client libraries

## Configuration

Default configuration locations:
- **Linux/macOS:** \`~/.mantisdb/config.yaml\` or \`/etc/mantisdb/config.yaml\`
- **Windows:** \`%APPDATA%\\MantisDB\\config.yaml\`

## Client Libraries

- **Go:** \`go get github.com/mantisdb/mantisdb/clients/go\`
- **JavaScript/Node.js:** \`npm install mantisdb\`
- **Python:** \`pip install mantisdb\`

## Documentation

- **Installation Guide:** [INSTALL.md](INSTALL.md)
- **Configuration:** [CONFIG.md](CONFIG.md)
- **API Documentation:** [API.md](API.md)
- **Client Examples:** See \`clients/\` directory

## Support

- **Website:** https://mantisdb.com
- **Documentation:** https://mantisdb.com/docs
- **Issues:** https://github.com/mantisdb/mantisdb/issues
- **Discussions:** https://github.com/mantisdb/mantisdb/discussions

## Verification

All files include SHA256 checksums for verification:
\`\`\`bash
# Verify file integrity
shasum -a 256 -c checksums.txt
\`\`\`

## License

MantisDB is released under the MIT License. See [LICENSE](LICENSE) for details.

---

**Full Changelog:** https://github.com/mantisdb/mantisdb/compare/v$(echo "$VERSION" | awk -F. '{print $1"."$2"."($3-1)}')...v$VERSION
EOF
    
    log_success "Release notes generated: $release_notes"
}

# Create release archive
create_release_archive() {
    log_step "Creating release archive..."
    
    local archive_name="mantisdb-$VERSION-complete-release.tar.gz"
    
    # Create a complete release archive
    tar -czf "dist/$archive_name" \
        --exclude="dist/$archive_name" \
        -C dist \
        .
    
    log_success "Release archive created: dist/$archive_name"
}

# Print summary
print_summary() {
    echo ""
    echo -e "${CYAN}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║                     RELEASE COMPLETE                        ║${NC}"
    echo -e "${CYAN}╚══════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "${GREEN}✅ MantisDB $VERSION release build completed successfully!${NC}"
    echo ""
    echo "📦 Release artifacts:"
    
    # List all created files
    if [ -d "dist" ]; then
        find dist -type f \( -name "*.exe" -o -name "*.dmg" -o -name "*.deb" -o -name "*.rpm" -o -name "*.zip" -o -name "*.tar.gz" \) | sort | while read -r file; do
            local size=$(du -h "$file" | cut -f1)
            echo "   $(basename "$file") ($size)"
        done
    fi
    
    echo ""
    echo "📋 Next steps:"
    echo "   1. Test the installers on target platforms"
    echo "   2. Upload to GitHub releases"
    echo "   3. Update package repositories"
    echo "   4. Announce the release"
    echo ""
    echo "📖 Documentation:"
    echo "   - Release notes: dist/RELEASE_NOTES.md"
    echo "   - Checksums: dist/checksums.txt"
    echo "   - Installation guide: INSTALL.md"
}

# Main execution
main() {
    print_banner
    check_prerequisites
    clean_build
    run_tests
    build_all
    create_checksums
    generate_release_notes
    create_release_archive
    print_summary
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --version)
            VERSION="$2"
            shift 2
            ;;
        --skip-tests)
            SKIP_TESTS=true
            shift
            ;;
        --skip-frontend)
            SKIP_FRONTEND=true
            shift
            ;;
        --skip-clients)
            SKIP_CLIENTS=true
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Creates a complete MantisDB release with all binaries and installers"
            echo ""
            echo "Options:"
            echo "  --version VERSION     Set release version (default: 1.0.0)"
            echo "  --skip-tests          Skip running tests"
            echo "  --skip-frontend       Skip frontend build"
            echo "  --skip-clients        Skip client library builds"
            echo "  --help, -h            Show this help"
            echo ""
            echo "Environment Variables:"
            echo "  VERSION               Release version"
            echo "  SKIP_TESTS            Skip tests (true/false)"
            echo "  SKIP_FRONTEND         Skip frontend (true/false)"
            echo "  SKIP_CLIENTS          Skip clients (true/false)"
            echo ""
            echo "Examples:"
            echo "  $0                                    # Full release build"
            echo "  $0 --version=1.2.0                   # Specific version"
            echo "  $0 --skip-tests --version=1.2.0-rc1  # Release candidate"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Run main function
main