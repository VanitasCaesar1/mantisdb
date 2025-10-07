#!/bin/bash

# MantisDB Client SDK Publishing Script
# Publishes all client libraries to their respective registries

set -e

# Configuration
VERSION=${VERSION:-"1.0.0"}
DRY_RUN=${DRY_RUN:-false}
SKIP_TESTS=${SKIP_TESTS:-false}

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
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

print_banner() {
    echo -e "${BLUE}"
    echo "╔══════════════════════════════════════════════════════════════╗"
    echo "║           MantisDB Client SDK Publisher                     ║"
    echo "╚══════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
    echo "Version: $VERSION"
    echo "Dry Run: $DRY_RUN"
    echo ""
}

# Publish JavaScript/TypeScript client to npm
publish_javascript() {
    log_info "Publishing JavaScript client to npm..."
    
    cd clients/javascript
    
    # Install dependencies
    log_info "Installing dependencies..."
    npm install
    
    # Build
    log_info "Building package..."
    npm run build
    
    # Run tests
    if [ "$SKIP_TESTS" != "true" ]; then
        log_info "Running tests..."
        npm test || log_warning "Tests failed, continuing anyway"
    fi
    
    # Lint
    log_info "Linting code..."
    npm run lint || log_warning "Linting issues found, continuing anyway"
    
    # Update version
    log_info "Updating version to $VERSION..."
    npm version "$VERSION" --no-git-tag-version
    
    if [ "$DRY_RUN" = "true" ]; then
        log_info "Dry run - would publish to npm"
        npm publish --dry-run --access public
    else
        log_info "Publishing to npm..."
        npm publish --access public
        log_success "Published @mantisdb/client@$VERSION to npm"
    fi
    
    cd ../..
}

# Publish Python client to PyPI
publish_python() {
    log_info "Publishing Python client to PyPI..."
    
    cd clients/python
    
    # Check if build tools are installed
    if ! command -v twine &> /dev/null; then
        log_error "twine not found. Install with: pip install twine"
        cd ../..
        return 1
    fi
    
    # Install dependencies
    log_info "Installing dependencies..."
    pip install -e .[dev] --quiet
    
    # Run tests
    if [ "$SKIP_TESTS" != "true" ]; then
        log_info "Running tests..."
        pytest || log_warning "Tests failed, continuing anyway"
    fi
    
    # Type checking
    log_info "Type checking..."
    mypy . || log_warning "Type checking issues found, continuing anyway"
    
    # Format code
    log_info "Formatting code..."
    black .
    isort .
    
    # Update version in pyproject.toml
    log_info "Updating version to $VERSION..."
    sed -i.bak "s/version = \".*\"/version = \"$VERSION\"/" pyproject.toml
    rm pyproject.toml.bak
    
    # Clean previous builds
    rm -rf dist/ build/ *.egg-info
    
    # Build package
    log_info "Building package..."
    python -m build
    
    # Check package
    log_info "Checking package..."
    python -m twine check dist/*
    
    if [ "$DRY_RUN" = "true" ]; then
        log_info "Dry run - would publish to PyPI"
        log_info "Files that would be uploaded:"
        ls -lh dist/
    else
        log_info "Publishing to PyPI..."
        python -m twine upload dist/*
        log_success "Published mantisdb@$VERSION to PyPI"
    fi
    
    cd ../..
}

# Publish Go client (create git tag)
publish_go() {
    log_info "Publishing Go client..."
    
    cd clients/go
    
    # Format code
    log_info "Formatting code..."
    go fmt ./...
    
    # Vet code
    log_info "Vetting code..."
    go vet ./...
    
    # Run tests
    if [ "$SKIP_TESTS" != "true" ]; then
        log_info "Running tests..."
        go test ./... || log_warning "Tests failed, continuing anyway"
    fi
    
    # Tidy dependencies
    log_info "Tidying dependencies..."
    go mod tidy
    
    cd ../..
    
    # Create git tag
    local tag="clients/go/v$VERSION"
    
    if [ "$DRY_RUN" = "true" ]; then
        log_info "Dry run - would create tag: $tag"
    else
        log_info "Creating git tag: $tag..."
        git tag "$tag"
        git push origin "$tag"
        
        # Trigger pkg.go.dev indexing
        log_info "Triggering pkg.go.dev indexing..."
        curl -s "https://proxy.golang.org/github.com/mantisdb/mantisdb/clients/go/@v/v${VERSION}.info" > /dev/null || true
        
        log_success "Published Go client v$VERSION"
        log_info "View at: https://pkg.go.dev/github.com/mantisdb/mantisdb/clients/go@v$VERSION"
    fi
}

# Main publishing function
main() {
    print_banner
    
    # Check if we're in the right directory
    if [ ! -d "clients" ]; then
        log_error "Must be run from the mantisdb root directory"
        exit 1
    fi
    
    # Confirm version
    if [ "$DRY_RUN" != "true" ]; then
        echo -e "${YELLOW}About to publish version $VERSION to all registries.${NC}"
        read -p "Continue? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log_info "Aborted"
            exit 0
        fi
    fi
    
    # Publish each client
    local failed=()
    
    # JavaScript
    if publish_javascript; then
        log_success "✓ JavaScript client published"
    else
        log_error "✗ JavaScript client failed"
        failed+=("JavaScript")
    fi
    
    echo ""
    
    # Python
    if publish_python; then
        log_success "✓ Python client published"
    else
        log_error "✗ Python client failed"
        failed+=("Python")
    fi
    
    echo ""
    
    # Go
    if publish_go; then
        log_success "✓ Go client published"
    else
        log_error "✗ Go client failed"
        failed+=("Go")
    fi
    
    # Summary
    echo ""
    echo -e "${BLUE}════════════════════════════════════════${NC}"
    echo -e "${BLUE}Publishing Summary${NC}"
    echo -e "${BLUE}════════════════════════════════════════${NC}"
    echo "Version: $VERSION"
    echo ""
    
    if [ ${#failed[@]} -eq 0 ]; then
        echo -e "${GREEN}✓ All clients published successfully!${NC}"
        echo ""
        echo "Installation commands:"
        echo "  npm install @mantisdb/client@$VERSION"
        echo "  pip install mantisdb==$VERSION"
        echo "  go get github.com/mantisdb/mantisdb/clients/go@v$VERSION"
    else
        echo -e "${RED}✗ Some clients failed to publish:${NC}"
        for client in "${failed[@]}"; do
            echo "  - $client"
        done
        exit 1
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
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --skip-tests)
            SKIP_TESTS=true
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Publishes MantisDB client libraries to package registries"
            echo ""
            echo "Options:"
            echo "  --version VERSION    Set version to publish (default: 1.0.0)"
            echo "  --dry-run            Show what would be published without actually publishing"
            echo "  --skip-tests         Skip running tests"
            echo "  --help, -h           Show this help"
            echo ""
            echo "Environment Variables:"
            echo "  VERSION              Version to publish"
            echo "  DRY_RUN              Set to 'true' for dry run"
            echo "  SKIP_TESTS           Set to 'true' to skip tests"
            echo ""
            echo "Examples:"
            echo "  $0 --version=1.2.0                # Publish version 1.2.0"
            echo "  $0 --version=1.2.0 --dry-run      # Test publishing without actually doing it"
            echo "  $0 --skip-tests                   # Publish without running tests"
            echo ""
            echo "Prerequisites:"
            echo "  - npm login (for JavaScript)"
            echo "  - PyPI credentials in ~/.pypirc (for Python)"
            echo "  - Git push access (for Go)"
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
main "$@"
