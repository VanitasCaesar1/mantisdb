#!/bin/bash

# MantisDB Cleanup Script
# Removes development artifacts and prepares for production

set -e

echo "🧹 MantisDB Cleanup Script"
echo "========================="

# Remove development data directories
echo "Removing development data directories..."
rm -rf data/
rm -rf test_data/
rm -rf tmp/
rm -rf .tmp/

# Remove build artifacts
echo "Removing build artifacts..."
rm -rf dist/
rm -rf build/
rm -f mantisdb
rm -f mantisdb.exe
rm -f admin-server
rm -f admin-server.exe

# Remove log files
echo "Removing log files..."
rm -f *.log
rm -rf logs/

# Remove cache files
echo "Removing cache files..."
rm -rf cache/
rm -rf .cache/

# Remove benchmark results
echo "Removing benchmark results..."
rm -f benchmark_results.json
rm -f *.bench
rm -f *.prof
rm -f *.pprof

# Remove coverage files
echo "Removing coverage files..."
rm -f coverage.out
rm -f coverage.html

# Remove node_modules if they exist
echo "Removing node_modules..."
find . -name "node_modules" -type d -exec rm -rf {} + 2>/dev/null || true

# Remove Python cache
echo "Removing Python cache..."
find . -name "__pycache__" -type d -exec rm -rf {} + 2>/dev/null || true
find . -name "*.pyc" -delete 2>/dev/null || true
find . -name "*.pyo" -delete 2>/dev/null || true

# Remove Go build cache
echo "Cleaning Go build cache..."
go clean -cache -modcache -testcache 2>/dev/null || true

# Remove IDE files
echo "Removing IDE files..."
rm -rf .vscode/settings.json 2>/dev/null || true
rm -rf .idea/ 2>/dev/null || true

# Remove OS files
echo "Removing OS files..."
find . -name ".DS_Store" -delete 2>/dev/null || true
find . -name "Thumbs.db" -delete 2>/dev/null || true

echo "✅ Cleanup complete!"
echo ""
echo "Repository is now clean and ready for:"
echo "  - Production builds: make production"
echo "  - Docker builds: docker build ."
echo "  - Development: make build && ./mantisdb"