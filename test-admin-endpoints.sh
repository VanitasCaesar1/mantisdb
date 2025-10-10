#!/bin/bash

# Test script for admin dashboard endpoints
# Make sure MantisDB is running on port 8080 before running this script

BASE_URL="http://localhost:8080"

echo "Testing MantisDB Admin API Endpoints"
echo "====================================="
echo ""

# Test 1: Health Check
echo "1. Testing /health endpoint..."
curl -s "$BASE_URL/health" | jq '.' 2>/dev/null || curl -s "$BASE_URL/health"
echo -e "\n"

# Test 2: API Health Check
echo "2. Testing /api/health endpoint..."
curl -s "$BASE_URL/api/health" | jq '.' 2>/dev/null || curl -s "$BASE_URL/api/health"
echo -e "\n"

# Test 3: System Stats
echo "3. Testing /api/system/stats endpoint..."
curl -s "$BASE_URL/api/system/stats" | jq '.' 2>/dev/null || curl -s "$BASE_URL/api/system/stats"
echo -e "\n"

# Test 4: Metrics
echo "4. Testing /api/metrics endpoint..."
curl -s "$BASE_URL/api/metrics" | jq '.' 2>/dev/null || curl -s "$BASE_URL/api/metrics"
echo -e "\n"

# Test 5: Tables List
echo "5. Testing /api/tables endpoint..."
curl -s "$BASE_URL/api/tables" | jq '.' 2>/dev/null || curl -s "$BASE_URL/api/tables"
echo -e "\n"

# Test 6: Columnar Tables
echo "6. Testing /api/columnar/tables endpoint..."
curl -s "$BASE_URL/api/columnar/tables" | jq '.' 2>/dev/null || curl -s "$BASE_URL/api/columnar/tables"
echo -e "\n"

# Test 7: SQL Query (should return not implemented message)
echo "7. Testing /api/query endpoint..."
curl -s -X POST "$BASE_URL/api/query" \
  -H "Content-Type: application/json" \
  -d '{"query":"SELECT * FROM users","query_type":"sql"}' | jq '.' 2>/dev/null || \
  curl -s -X POST "$BASE_URL/api/query" \
  -H "Content-Type: application/json" \
  -d '{"query":"SELECT * FROM users","query_type":"sql"}'
echo -e "\n"

# Test 8: Version
echo "8. Testing /api/v1/version endpoint..."
curl -s "$BASE_URL/api/v1/version" | jq '.' 2>/dev/null || curl -s "$BASE_URL/api/v1/version"
echo -e "\n"

# Test 9: Stats
echo "9. Testing /api/v1/stats endpoint..."
curl -s "$BASE_URL/api/v1/stats" | jq '.' 2>/dev/null || curl -s "$BASE_URL/api/v1/stats"
echo -e "\n"

echo "====================================="
echo "All tests completed!"
echo ""
echo "If you see JSON responses above, the endpoints are working correctly."
echo "If you see connection errors, make sure MantisDB is running:"
echo "  cd /Users/vanitascaesar/Documents/Projects/mantisdb"
echo "  go run cmd/mantisDB/main.go"
