#!/bin/bash

# MantisDB Homebrew Formula Creator
# Creates Homebrew formula and tap repository

set -e

# Configuration
VERSION=${VERSION:-"1.0.0"}
APP_NAME="MantisDB"
DESCRIPTION="Multi-Model Database with Admin Dashboard"
WEBSITE="https://mantisdb.com"
GITHUB_REPO="mantisdb/mantisdb"

# Directories
BUILD_DIR="dist"
HOMEBREW_DIR="$BUILD_DIR/installers/homebrew"

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

# Calculate SHA256 checksums for release archives
calculate_checksums() {
    log_info "Calculating checksums for release archives..."
    
    local amd64_sha=""
    local arm64_sha=""
    local universal_sha=""
    
    # Check for tar.gz archives (for Homebrew)
    if [ -f "$BUILD_DIR/mantisdb-darwin-amd64.tar.gz" ]; then
        amd64_sha=$(shasum -a 256 "$BUILD_DIR/mantisdb-darwin-amd64.tar.gz" | cut -d' ' -f1)
        log_info "AMD64 SHA256: $amd64_sha"
    elif [ -f "$BUILD_DIR/mantisdb-darwin-amd64" ]; then
        # Create tar.gz if it doesn't exist
        log_info "Creating AMD64 archive for Homebrew..."
        tar -czf "$BUILD_DIR/mantisdb-darwin-amd64.tar.gz" -C "$BUILD_DIR" mantisdb-darwin-amd64
        amd64_sha=$(shasum -a 256 "$BUILD_DIR/mantisdb-darwin-amd64.tar.gz" | cut -d' ' -f1)
    fi
    
    if [ -f "$BUILD_DIR/mantisdb-darwin-arm64.tar.gz" ]; then
        arm64_sha=$(shasum -a 256 "$BUILD_DIR/mantisdb-darwin-arm64.tar.gz" | cut -d' ' -f1)
        log_info "ARM64 SHA256: $arm64_sha"
    elif [ -f "$BUILD_DIR/mantisdb-darwin-arm64" ]; then
        # Create tar.gz if it doesn't exist
        log_info "Creating ARM64 archive for Homebrew..."
        tar -czf "$BUILD_DIR/mantisdb-darwin-arm64.tar.gz" -C "$BUILD_DIR" mantisdb-darwin-arm64
        arm64_sha=$(shasum -a 256 "$BUILD_DIR/mantisdb-darwin-arm64.tar.gz" | cut -d' ' -f1)
    fi
    
    # Create universal archive if both architectures exist
    if [ -f "$BUILD_DIR/mantisdb-darwin-amd64" ] && [ -f "$BUILD_DIR/mantisdb-darwin-arm64" ]; then
        if [ ! -f "$BUILD_DIR/mantisdb-darwin-universal.tar.gz" ]; then
            log_info "Creating universal binary and archive..."
            lipo -create -output "$BUILD_DIR/mantisdb-darwin-universal" \
                "$BUILD_DIR/mantisdb-darwin-amd64" \
                "$BUILD_DIR/mantisdb-darwin-arm64"
            tar -czf "$BUILD_DIR/mantisdb-darwin-universal.tar.gz" -C "$BUILD_DIR" mantisdb-darwin-universal
            rm "$BUILD_DIR/mantisdb-darwin-universal"
        fi
        universal_sha=$(shasum -a 256 "$BUILD_DIR/mantisdb-darwin-universal.tar.gz" | cut -d' ' -f1)
        log_info "Universal SHA256: $universal_sha"
    fi
    
    echo "$amd64_sha|$arm64_sha|$universal_sha"
}

# Create Homebrew formula
create_formula() {
    local checksums="$1"
    local amd64_sha=$(echo "$checksums" | cut -d'|' -f1)
    local arm64_sha=$(echo "$checksums" | cut -d'|' -f2)
    local universal_sha=$(echo "$checksums" | cut -d'|' -f3)
    
    log_info "Creating Homebrew formula..."
    
    mkdir -p "$HOMEBREW_DIR"
    
    # Create the main formula
    cat > "$HOMEBREW_DIR/mantisdb.rb" << EOF
class Mantisdb < Formula
  desc "$DESCRIPTION"
  homepage "$WEBSITE"
  version "$VERSION"
  license "MIT"
  
  # Architecture-specific downloads
  if Hardware::CPU.intel?
    url "https://github.com/$GITHUB_REPO/releases/download/v#{version}/mantisdb-darwin-amd64.tar.gz"
    sha256 "$amd64_sha"
  elsif Hardware::CPU.arm?
    url "https://github.com/$GITHUB_REPO/releases/download/v#{version}/mantisdb-darwin-arm64.tar.gz"
    sha256 "$arm64_sha"
  end
  
  # Alternative: Universal binary (uncomment if preferred)
  # url "https://github.com/$GITHUB_REPO/releases/download/v#{version}/mantisdb-darwin-universal.tar.gz"
  # sha256 "$universal_sha"
  
  depends_on "go" => :build
  depends_on macos: :big_sur
  
  def install
    # Determine binary name based on architecture
    if Hardware::CPU.intel?
      binary_name = "mantisdb-darwin-amd64"
    elsif Hardware::CPU.arm?
      binary_name = "mantisdb-darwin-arm64"
    else
      binary_name = "mantisdb-darwin-universal"
    end
    
    # Install binary
    bin.install binary_name => "mantisdb"
    
    # Create config directory
    etc.mkpath "mantisdb"
    
    # Install default config
    (etc/"mantisdb/config.yaml").write <<~EOS
      # MantisDB Configuration
      server:
        port: 8080
        admin_port: 8081
        host: 127.0.0.1
      
      storage:
        data_dir: #{var}/lib/mantisdb
        engine: auto
      
      logging:
        level: info
        file: #{var}/log/mantisdb/mantisdb.log
      
      cache:
        size: 268435456  # 256MB
    EOS
    
    # Create example configs
    (etc/"mantisdb/config-production.yaml").write <<~EOS
      # MantisDB Production Configuration
      server:
        port: 8080
        admin_port: 8081
        host: 0.0.0.0
        tls:
          enabled: true
          cert_file: /etc/ssl/certs/mantisdb.crt
          key_file: /etc/ssl/private/mantisdb.key
      
      storage:
        data_dir: #{var}/lib/mantisdb
        engine: cgo
        backup:
          enabled: true
          interval: 1h
          retention: 168h  # 7 days
      
      logging:
        level: warn
        file: #{var}/log/mantisdb/mantisdb.log
        structured: true
      
      cache:
        size: 1073741824  # 1GB
      
      auth:
        enabled: true
        jwt_secret: "change-this-in-production"
    EOS
  end
  
  def post_install
    # Create data and log directories
    (var/"lib/mantisdb").mkpath
    (var/"log/mantisdb").mkpath
    
    # Set permissions
    system "chmod", "755", var/"lib/mantisdb"
    system "chmod", "755", var/"log/mantisdb"
  end
  
  service do
    run [opt_bin/"mantisdb", "--config=#{etc}/mantisdb/config.yaml"]
    keep_alive true
    log_path var/"log/mantisdb/mantisdb.log"
    error_log_path var/"log/mantisdb/mantisdb.log"
    working_dir var/"lib/mantisdb"
  end
  
  test do
    # Test version
    assert_match version.to_s, shell_output("#{bin}/mantisdb --version")
    
    # Test config validation
    system bin/"mantisdb", "--config=#{etc}/mantisdb/config.yaml", "--validate"
    
    # Test basic functionality (start and stop quickly)
    port = free_port
    admin_port = free_port
    
    # Create test config
    test_config = testpath/"test-config.yaml"
    test_config.write <<~EOS
      server:
        port: #{port}
        admin_port: #{admin_port}
        host: 127.0.0.1
      storage:
        data_dir: #{testpath}/data
        engine: pure
      logging:
        level: error
        file: #{testpath}/test.log
    EOS
    
    # Start server in background
    pid = fork do
      exec bin/"mantisdb", "--config=#{test_config}"
    end
    
    sleep 2
    
    # Test health endpoint
    system "curl", "-f", "http://127.0.0.1:#{admin_port}/health"
    
    # Clean up
    Process.kill("TERM", pid)
    Process.wait(pid)
  end
end
EOF
    
    log_success "Created Homebrew formula: mantisdb.rb"
}

# Create tap repository structure
create_tap_repository() {
    log_info "Creating Homebrew tap repository structure..."
    
    local tap_dir="$HOMEBREW_DIR/homebrew-tap"
    mkdir -p "$tap_dir/Formula"
    
    # Copy formula to tap
    cp "$HOMEBREW_DIR/mantisdb.rb" "$tap_dir/Formula/"
    
    # Create tap README
    cat > "$tap_dir/README.md" << EOF
# MantisDB Homebrew Tap

Official Homebrew tap for MantisDB - Multi-Model Database with Admin Dashboard.

## Installation

\`\`\`bash
# Add the tap
brew tap mantisdb/tap

# Install MantisDB
brew install mantisdb
\`\`\`

## Usage

\`\`\`bash
# Start MantisDB
brew services start mantisdb

# Or run directly
mantisdb --config=/usr/local/etc/mantisdb/config.yaml

# Access admin dashboard
open http://localhost:8081
\`\`\`

## Configuration

Default configuration: \`/usr/local/etc/mantisdb/config.yaml\`
Data directory: \`/usr/local/var/lib/mantisdb\`
Logs: \`/usr/local/var/log/mantisdb/mantisdb.log\`

## Updating

\`\`\`bash
brew update
brew upgrade mantisdb
\`\`\`

## Uninstalling

\`\`\`bash
# Stop service
brew services stop mantisdb

# Uninstall
brew uninstall mantisdb

# Remove tap (optional)
brew untap mantisdb/tap
\`\`\`

## Support

- Website: $WEBSITE
- Issues: https://github.com/$GITHUB_REPO/issues
- Documentation: $WEBSITE/docs
EOF
    
    # Create tap info file
    cat > "$tap_dir/.github/workflows/tests.yml" << 'EOF'
name: brew test-bot
on:
  push:
    branches:
      - main
  pull_request:
jobs:
  test-bot:
    strategy:
      matrix:
        os: [ubuntu-22.04, macos-12, macos-13]
    runs-on: ${{ matrix.os }}
    steps:
      - name: Set up Homebrew
        id: set-up-homebrew
        uses: Homebrew/actions/setup-homebrew@master

      - name: Cache Homebrew Bundler RubyGems
        id: cache
        uses: actions/cache@v3
        with:
          path: ${{ steps.set-up-homebrew.outputs.gems-path }}
          key: ${{ runner.os }}-rubygems-${{ steps.set-up-homebrew.outputs.gems-hash }}
          restore-keys: ${{ runner.os }}-rubygems-

      - name: Install Homebrew Bundler RubyGems
        if: steps.cache.outputs.cache-hit != 'true'
        run: brew install-bundler-gems

      - run: brew test-bot --only-cleanup-before

      - run: brew test-bot --only-setup

      - run: brew test-bot --only-tap-syntax

      - run: brew test-bot --only-formulae
        if: github.event_name == 'pull_request'

      - name: Upload bottles as artifact
        if: always() && github.event_name == 'pull_request'
        uses: actions/upload-artifact@main
        with:
          name: bottles
          path: '*.bottle.*'
EOF
    
    log_success "Created tap repository structure"
}

# Create installation instructions
create_install_instructions() {
    log_info "Creating installation instructions..."
    
    cat > "$HOMEBREW_DIR/INSTALL.md" << EOF
# MantisDB Homebrew Installation

## Quick Install

\`\`\`bash
# Method 1: Direct install (when formula is in Homebrew core)
brew install mantisdb

# Method 2: From our tap
brew tap mantisdb/tap
brew install mantisdb/tap/mantisdb

# Method 3: Direct formula URL
brew install https://raw.githubusercontent.com/$GITHUB_REPO/main/Formula/mantisdb.rb
\`\`\`

## Development Install

\`\`\`bash
# Install from local formula
brew install --build-from-source ./mantisdb.rb

# Or install HEAD version
brew install --HEAD mantisdb/tap/mantisdb
\`\`\`

## Service Management

\`\`\`bash
# Start as service (runs on boot)
brew services start mantisdb

# Stop service
brew services stop mantisdb

# Restart service
brew services restart mantisdb

# Check service status
brew services list | grep mantisdb
\`\`\`

## Manual Usage

\`\`\`bash
# Run with default config
mantisdb

# Run with custom config
mantisdb --config=/path/to/config.yaml

# Run with specific ports
mantisdb --port=8080 --admin-port=8081

# Validate configuration
mantisdb --config=/usr/local/etc/mantisdb/config.yaml --validate

# Show version
mantisdb --version

# Show help
mantisdb --help
\`\`\`

## Configuration

### Default Locations
- Config: \`/usr/local/etc/mantisdb/config.yaml\`
- Data: \`/usr/local/var/lib/mantisdb/\`
- Logs: \`/usr/local/var/log/mantisdb/mantisdb.log\`

### Edit Configuration
\`\`\`bash
# Edit main config
nano /usr/local/etc/mantisdb/config.yaml

# Use production config template
cp /usr/local/etc/mantisdb/config-production.yaml /usr/local/etc/mantisdb/config.yaml
\`\`\`

### Key Settings
\`\`\`yaml
server:
  port: 8080              # Main database port
  admin_port: 8081        # Admin dashboard port
  host: 127.0.0.1         # Bind address (0.0.0.0 for external access)

storage:
  data_dir: /usr/local/var/lib/mantisdb
  engine: auto            # auto, cgo, pure

logging:
  level: info             # debug, info, warn, error
  file: /usr/local/var/log/mantisdb/mantisdb.log
\`\`\`

## Admin Dashboard

After starting MantisDB:
- URL: http://localhost:8081
- No authentication by default
- Configure auth for production use

## Updating

\`\`\`bash
# Update Homebrew
brew update

# Upgrade MantisDB
brew upgrade mantisdb

# Check for outdated packages
brew outdated
\`\`\`

## Troubleshooting

### Service Won't Start
\`\`\`bash
# Check service logs
brew services info mantisdb

# Check system logs
tail -f /usr/local/var/log/mantisdb/mantisdb.log

# Validate config
mantisdb --config=/usr/local/etc/mantisdb/config.yaml --validate
\`\`\`

### Port Conflicts
\`\`\`bash
# Check what's using ports
lsof -i :8080
lsof -i :8081

# Use different ports
mantisdb --port=8082 --admin-port=8083
\`\`\`

### Permission Issues
\`\`\`bash
# Fix data directory permissions
sudo chown -R \$(whoami) /usr/local/var/lib/mantisdb
sudo chown -R \$(whoami) /usr/local/var/log/mantisdb
\`\`\`

## Uninstalling

\`\`\`bash
# Stop service
brew services stop mantisdb

# Uninstall package
brew uninstall mantisdb

# Remove data (optional)
rm -rf /usr/local/var/lib/mantisdb
rm -rf /usr/local/var/log/mantisdb

# Remove config (optional)
rm -rf /usr/local/etc/mantisdb

# Remove tap (optional)
brew untap mantisdb/tap
\`\`\`

## Support

- Documentation: $WEBSITE/docs
- Issues: https://github.com/$GITHUB_REPO/issues
- Homebrew Issues: https://github.com/$GITHUB_REPO/issues (label: homebrew)
EOF
    
    log_success "Created installation instructions"
}

# Main execution
main() {
    echo -e "${BLUE}MantisDB Homebrew Formula Creator${NC}"
    echo -e "${BLUE}=================================${NC}"
    
    # Check if we have macOS binaries
    if [ ! -f "$BUILD_DIR/mantisdb-darwin-amd64" ] && [ ! -f "$BUILD_DIR/mantisdb-darwin-arm64" ]; then
        log_error "No macOS binaries found. Please build macOS binaries first."
        exit 1
    fi
    
    # Calculate checksums
    local checksums
    checksums=$(calculate_checksums)
    
    # Create formula and supporting files
    create_formula "$checksums"
    create_tap_repository
    create_install_instructions
    
    log_success "Homebrew formula creation complete!"
    echo ""
    echo "Created files:"
    echo "  Formula: $HOMEBREW_DIR/mantisdb.rb"
    echo "  Tap: $HOMEBREW_DIR/homebrew-tap/"
    echo "  Instructions: $HOMEBREW_DIR/INSTALL.md"
    echo ""
    echo "To test locally:"
    echo "  brew install --build-from-source $HOMEBREW_DIR/mantisdb.rb"
    echo ""
    echo "To publish:"
    echo "  1. Create GitHub repository: mantisdb/homebrew-tap"
    echo "  2. Push contents of $HOMEBREW_DIR/homebrew-tap/"
    echo "  3. Users can install with: brew tap mantisdb/tap && brew install mantisdb"
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
        --help|-h)
            echo "Usage: $0 [--version VERSION]"
            echo "Creates Homebrew formula for MantisDB"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Run main function
main "$@"