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

# Check if backend is already running
if lsof -ti:8080 > /dev/null 2>&1; then
    echo -e "${YELLOW}⚠️  Backend already running on port 8080${NC}"
else
    echo -e "${GREEN}Starting MantisDB Backend API on port 8080...${NC}"
    # Start backend in background
    go run cmd/mantisDB/main.go > /dev/null 2>&1 &
    BACKEND_PID=$!
    echo "Backend PID: $BACKEND_PID"
    
    # Wait for backend to start
    echo "Waiting for backend to be ready..."
    for i in {1..30}; do
        if curl -s http://localhost:8080/health > /dev/null 2>&1; then
            echo -e "${GREEN}✅ Backend API is ready!${NC}"
            break
        fi
        if [ $i -eq 30 ]; then
            echo -e "${RED}❌ Backend failed to start${NC}"
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
