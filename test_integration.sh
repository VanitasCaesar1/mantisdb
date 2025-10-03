#!/bin/bash

echo "Testing MantisDB Production Integration..."

# Start MantisDB in background
echo "Starting MantisDB..."
go run cmd/mantisDB/main.go --enable-admin --admin-port=8081 --port=8080 &
MANTIS_PID=$!

# Wait for startup
sleep 5

echo "Testing API endpoints..."

# Test health endpoint
echo "Testing health endpoint..."
HEALTH_RESPONSE=$(curl -s http://localhost:8081/api/health)
if [[ $? -eq 0 ]]; then
    echo "✓ Health endpoint working"
    echo "Response: $HEALTH_RESPONSE"
else
    echo "✗ Health endpoint failed"
fi

# Test metrics endpoint
echo "Testing metrics endpoint..."
METRICS_RESPONSE=$(curl -s http://localhost:8081/api/metrics)
if [[ $? -eq 0 ]]; then
    echo "✓ Metrics endpoint working"
else
    echo "✗ Metrics endpoint failed"
fi

# Test tables endpoint
echo "Testing tables endpoint..."
TABLES_RESPONSE=$(curl -s http://localhost:8081/api/tables)
if [[ $? -eq 0 ]]; then
    echo "✓ Tables endpoint working"
    echo "Response: $TABLES_RESPONSE"
else
    echo "✗ Tables endpoint failed"
fi

# Test static files (admin dashboard)
echo "Testing admin dashboard..."
DASHBOARD_RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8081/)
if [[ "$DASHBOARD_RESPONSE" == "200" ]]; then
    echo "✓ Admin dashboard accessible"
else
    echo "✗ Admin dashboard failed (HTTP $DASHBOARD_RESPONSE)"
fi

# Test with authentication token
echo "Testing with authentication token..."
export MANTIS_ADMIN_TOKEN="test-token-123"
AUTH_RESPONSE=$(curl -s -H "Authorization: Bearer test-token-123" http://localhost:8081/api/health)
if [[ $? -eq 0 ]]; then
    echo "✓ Authentication working"
else
    echo "✗ Authentication failed"
fi

# Cleanup
echo "Stopping MantisDB..."
kill $MANTIS_PID
wait $MANTIS_PID 2>/dev/null

echo "Integration test completed!"