#!/bin/bash

# MantisDB Development Script
# This script starts the development environment with hot reload

set -e

echo "🚀 MantisDB Development Environment"
echo "=================================="

# Function to cleanup background processes
cleanup() {
    echo ""
    echo "🛑 Shutting down development servers..."
    jobs -p | xargs -r kill
    exit 0
}

# Set up signal handlers
trap cleanup SIGINT SIGTERM

# Start frontend development server
start_frontend() {
    echo "🎨 Starting frontend development server..."
    cd admin/frontend
    
    if [ ! -d "node_modules" ]; then
        echo "📦 Installing frontend dependencies..."
        npm install
    fi
    
    echo "🌐 Frontend server will be available at http://localhost:3000"
    npm run dev &
    FRONTEND_PID=$!
    cd ../..
}

# Start admin API server
start_admin_api() {
    echo "🔧 Building and starting admin API server..."
    cd admin/api
    go build -o ../../admin-server .
    cd ../..
    
    echo "🌐 Admin API server will be available at http://localhost:8081"
    ./admin-server &
    ADMIN_PID=$!
}

# Main development startup
main() {
    echo "📋 Starting development environment..."
    
    start_frontend
    start_admin_api
    
    echo ""
    echo "✅ Development environment ready!"
    echo "   Frontend:  http://localhost:3000"
    echo "   Admin API: http://localhost:8081"
    echo ""
    echo "Press Ctrl+C to stop all servers"
    
    # Wait for background processes
    wait
}

# Run main function
main "$@"