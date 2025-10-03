#!/bin/bash

# MantisDB Release Script
# This script creates a GitHub release with all platform binaries

set -e

# Configuration
VERSION=${1:-$(git describe --tags --abbrev=0 2>/dev/null || echo "v1.0.0")}
GITHUB_REPO=${GITHUB_REPO:-"mantisdb/mantisdb"}
BUILD_DIR="dist"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}MantisDB Release Script${NC}"
echo -e "${BLUE}======================${NC}"
echo "Version: $VERSION"
echo "Repository: $GITHUB_REPO"
echo ""

# Check if gh CLI is installed
if ! command -v gh &> /dev/null; then
    echo -e "${RED}Error: GitHub CLI (gh) is not installed${NC}"
    echo "Please install it from: https://cli.github.com/"
    exit 1
fi

# Check if authenticated
if ! gh auth status &> /dev/null; then
    echo -e "${RED}Error: Not authenticated with GitHub${NC}"
    echo "Please run: gh auth login"
    exit 1
fi

# Build production binaries
echo -e "${YELLOW}Building production binaries...${NC}"
./scripts/build-production.sh

# Check if build was successful
if [ ! -d "$BUILD_DIR" ]; then
    echo -e "${RED}Error: Build directory not found${NC}"
    exit 1
fi

# Create release notes
RELEASE_NOTES_FILE="$BUILD_DIR/release-notes.md"
cat > "$RELEASE_NOTES_FILE" << EOF
# MantisDB $VERSION

## What's New

- Production-ready multi-model database
- Admin dashboard for database management
- Cross-platform support (Linux, macOS, Windows)
- High-performance storage engine with CGO and Pure Go options
- Comprehensive API for Key-Value, Document, and Columnar data models

## Installation

### Quick Install (Linux/macOS)
\`\`\`bash
# Download and extract
curl -L https://github.com/$GITHUB_REPO/releases/download/$VERSION/mantisdb-\$(uname -s | tr '[:upper:]' '[:lower:]')-\$(uname -m).tar.gz | tar xz

# Install
cd mantisdb-*
sudo ./install.sh
\`\`\`

### Manual Installation

1. Download the appropriate binary for your platform
2. Extract the archive
3. Run the installer script
4. Start MantisDB: \`mantisdb --config=/etc/mantisdb/config.yaml\`

## Platform Support

- **Linux**: x86_64, ARM64
- **macOS**: x86_64 (Intel), ARM64 (Apple Silicon)
- **Windows**: x86_64

## Admin Dashboard

Access the admin dashboard at \`http://localhost:8081\` after starting MantisDB.

## Configuration

Default configuration is created at:
- Linux/macOS: \`/etc/mantisdb/config.yaml\`
- Windows: \`%PROGRAMDATA%\MantisDB\config.yaml\`

## API Endpoints

- Health Check: \`GET /health\`
- Database Stats: \`GET /api/v1/stats\`
- Key-Value: \`/api/v1/kv/*\`
- Documents: \`/api/v1/docs/*\`
- Tables: \`/api/v1/tables/*\`

## Checksums

See \`checksums.txt\` for SHA256 checksums of all release artifacts.
EOF

# Create the release
echo -e "${YELLOW}Creating GitHub release...${NC}"
gh release create "$VERSION" \
    --repo "$GITHUB_REPO" \
    --title "MantisDB $VERSION" \
    --notes-file "$RELEASE_NOTES_FILE" \
    "$BUILD_DIR"/*.tar.gz \
    "$BUILD_DIR"/*.zip \
    "$BUILD_DIR"/checksums.txt

echo -e "${GREEN}Release created successfully!${NC}"
echo "View at: https://github.com/$GITHUB_REPO/releases/tag/$VERSION"

# Clean up
rm -f "$RELEASE_NOTES_FILE"

echo -e "${GREEN}Release process complete!${NC}"