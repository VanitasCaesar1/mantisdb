#!/bin/bash

# Script to run MantisDB Admin Dashboard
# This script starts both the backend API and the frontend dev server

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}🚀 MantisDB Admin Dashboard Startup Script${NC}"
echo ""

# Check if Rust admin server is already running
if lsof -ti:8081 > /dev/null 2>&1; then
    echo -e "${YELLOW}⚠️  Admin server already running on port 8081${NC}"
else
    echo -e "${GREEN}Starting MantisDB Rust Admin Server on port 8081...${NC}"
    # Start Rust admin server in background
    cargo run -p mantisdb-core --bin admin-server > /dev/null 2>&1 &
    BACKEND_PID=$!
    echo "Admin Server PID: $BACKEND_PID"
    
    # Wait for server to start
    echo "Waiting for admin server to be ready..."
    for i in {1..60}; do
        if curl -s http://localhost:8081/api/health > /dev/null 2>&1; then
            echo -e "${GREEN}✅ Admin server is ready!${NC}"
            break
        fi
        if [ $i -eq 60 ]; then
            echo -e "${RED}❌ Admin server failed to start${NC}"
            exit 1
        fi
        sleep 1
        echo -n "."
    done
    echo ""
fi

# Check if frontend is already running
if lsof -ti:5173 > /dev/null 2>&1; then
    echo -e "${YELLOW}⚠️  Frontend already running on port 5173${NC}"
else
    echo -e "${GREEN}Starting Admin Frontend on port 5173...${NC}"
    cd admin/frontend
    
    # Install dependencies if needed
    if [ ! -d "node_modules" ]; then
        echo "Installing frontend dependencies..."
        npm install
    fi
    
    # Start frontend dev server
    npm run dev
fi
