#!/bin/bash

# MantisDB Production Startup Script
# Starts the Rust admin server and frontend in production mode

set -e

echo "🚀 Starting MantisDB in Production Mode"
echo "========================================"

# Check if admin-server binary exists
if [ ! -f "rust-core/target/release/admin-server" ]; then
    echo "❌ Admin server binary not found!"
    echo "Please run ./build-optimized.sh first"
    exit 1
fi

# Start admin server
echo "🦀 Starting Rust Admin Server..."
./rust-core/target/release/admin-server &
SERVER_PID=$!

# Wait for server to start
echo "⏳ Waiting for server to start..."
sleep 2

# Check if server is running
if ! curl -s http://localhost:8081/api/health > /dev/null; then
    echo "❌ Server failed to start!"
    kill $SERVER_PID 2>/dev/null || true
    exit 1
fi

echo "✅ Server started successfully (PID: $SERVER_PID)"

# Build and serve frontend
echo "📦 Building frontend..."
cd admin/frontend
npm run build

echo "🌐 Starting frontend server..."
npm run preview &
FRONTEND_PID=$!

cd ../..

echo ""
echo "✅ MantisDB is running in production mode!"
echo "============================================"
echo "Backend:  http://localhost:8081"
echo "Frontend: http://localhost:4173"
echo ""
echo "Server PID: $SERVER_PID"
echo "Frontend PID: $FRONTEND_PID"
echo ""
echo "To stop all services:"
echo "  kill $SERVER_PID $FRONTEND_PID"
echo ""
echo "Press Ctrl+C to stop all services"

# Trap Ctrl+C
trap "echo ''; echo '🛑 Stopping services...'; kill $SERVER_PID $FRONTEND_PID 2>/dev/null; echo '✅ Services stopped'; exit 0" INT

# Wait for processes
wait
