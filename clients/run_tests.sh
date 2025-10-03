#!/bin/bash

# Comprehensive test runner for all MantisDB client libraries
# This script runs integration tests for Go, Python, and JavaScript clients

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test configuration
export MANTISDB_TEST_HOST="${MANTISDB_TEST_HOST:-localhost}"
export MANTISDB_TEST_PORT="${MANTISDB_TEST_PORT:-8080}"
export MANTISDB_TEST_USERNAME="${MANTISDB_TEST_USERNAME:-admin}"
export MANTISDB_TEST_PASSWORD="${MANTISDB_TEST_PASSWORD:-password}"
export MANTISDB_TEST_API_KEY="${MANTISDB_TEST_API_KEY:-}"

# Flags
RUN_GO=true
RUN_PYTHON=true
RUN_JAVASCRIPT=true
RUN_PERFORMANCE=false
RUN_LOAD_TESTS=false
VERBOSE=false
PARALLEL=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --go-only)
            RUN_GO=true
            RUN_PYTHON=false
            RUN_JAVASCRIPT=false
            shift
            ;;
        --python-only)
            RUN_GO=false
            RUN_PYTHON=true
            RUN_JAVASCRIPT=false
            shift
            ;;
        --js-only)
            RUN_GO=false
            RUN_PYTHON=false
            RUN_JAVASCRIPT=true
            shift
            ;;
        --performance)
            RUN_PERFORMANCE=true
            shift
            ;;
        --load-tests)
            RUN_LOAD_TESTS=true
            shift
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        --parallel)
            PARALLEL=true
            shift
            ;;
        --help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --go-only        Run only Go client tests"
            echo "  --python-only    Run only Python client tests"
            echo "  --js-only        Run only JavaScript client tests"
            echo "  --performance    Include performance tests"
            echo "  --load-tests     Include load tests (slow)"
            echo "  --verbose        Verbose output"
            echo "  --parallel       Run tests in parallel where possible"
            echo "  --help           Show this help message"
            echo ""
            echo "Environment variables:"
            echo "  MANTISDB_TEST_HOST      Test server host (default: localhost)"
            echo "  MANTISDB_TEST_PORT      Test server port (default: 8080)"
            echo "  MANTISDB_TEST_USERNAME  Test username (default: admin)"
            echo "  MANTISDB_TEST_PASSWORD  Test password (default: password)"
            echo "  MANTISDB_TEST_API_KEY   Test API key (optional)"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Function to print colored output
print_status() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}"
}

# Function to check if MantisDB server is running
check_server() {
    print_status $BLUE "Checking MantisDB server at ${MANTISDB_TEST_HOST}:${MANTISDB_TEST_PORT}..."
    
    if command -v curl >/dev/null 2>&1; then
        if curl -s -f "http://${MANTISDB_TEST_HOST}:${MANTISDB_TEST_PORT}/api/health" >/dev/null; then
            print_status $GREEN "✓ MantisDB server is running"
            return 0
        fi
    elif command -v wget >/dev/null 2>&1; then
        if wget -q --spider "http://${MANTISDB_TEST_HOST}:${MANTISDB_TEST_PORT}/api/health" 2>/dev/null; then
            print_status $GREEN "✓ MantisDB server is running"
            return 0
        fi
    fi
    
    print_status $RED "✗ MantisDB server is not accessible"
    print_status $YELLOW "Please ensure MantisDB is running on ${MANTISDB_TEST_HOST}:${MANTISDB_TEST_PORT}"
    return 1
}

# Function to run Go tests
run_go_tests() {
    print_status $BLUE "Running Go client tests..."
    
    cd go
    
    # Check if Go is installed
    if ! command -v go >/dev/null 2>&1; then
        print_status $RED "✗ Go is not installed"
        return 1
    fi
    
    # Install dependencies
    print_status $YELLOW "Installing Go dependencies..."
    go mod tidy
    
    # Run tests
    local test_flags="-v"
    if [[ $VERBOSE == true ]]; then
        test_flags="$test_flags -race"
    fi
    
    if [[ $RUN_PERFORMANCE == true ]]; then
        print_status $YELLOW "Running Go integration and performance tests..."
        go test $test_flags ./...
        
        print_status $YELLOW "Running Go benchmarks..."
        go test -bench=. -benchmem ./...
    else
        print_status $YELLOW "Running Go integration tests..."
        go test $test_flags -short ./...
    fi
    
    if [[ $? -eq 0 ]]; then
        print_status $GREEN "✓ Go tests passed"
    else
        print_status $RED "✗ Go tests failed"
        return 1
    fi
    
    cd ..
}

# Function to run Python tests
run_python_tests() {
    print_status $BLUE "Running Python client tests..."
    
    cd python
    
    # Check if Python is installed
    if ! command -v python3 >/dev/null 2>&1; then
        print_status $RED "✗ Python 3 is not installed"
        return 1
    fi
    
    # Create virtual environment if it doesn't exist
    if [[ ! -d "venv" ]]; then
        print_status $YELLOW "Creating Python virtual environment..."
        python3 -m venv venv
    fi
    
    # Activate virtual environment
    source venv/bin/activate
    
    # Install dependencies
    print_status $YELLOW "Installing Python dependencies..."
    pip install -e .
    pip install pytest pytest-asyncio pytest-cov
    
    # Run tests
    local pytest_flags="-v"
    if [[ $VERBOSE == true ]]; then
        pytest_flags="$pytest_flags -s"
    fi
    
    if [[ $RUN_PERFORMANCE == true ]]; then
        pytest_flags="$pytest_flags tests/test_performance.py"
    fi
    
    if [[ $RUN_LOAD_TESTS == true ]]; then
        pytest_flags="$pytest_flags --runslow"
    fi
    
    print_status $YELLOW "Running Python tests..."
    pytest $pytest_flags tests/
    
    if [[ $? -eq 0 ]]; then
        print_status $GREEN "✓ Python tests passed"
    else
        print_status $RED "✗ Python tests failed"
        deactivate
        return 1
    fi
    
    # Generate coverage report if verbose
    if [[ $VERBOSE == true ]]; then
        print_status $YELLOW "Generating Python coverage report..."
        pytest --cov=mantisdb --cov-report=html tests/
    fi
    
    deactivate
    cd ..
}

# Function to run JavaScript tests
run_javascript_tests() {
    print_status $BLUE "Running JavaScript client tests..."
    
    cd javascript
    
    # Check if Node.js is installed
    if ! command -v node >/dev/null 2>&1; then
        print_status $RED "✗ Node.js is not installed"
        return 1
    fi
    
    # Check if npm is installed
    if ! command -v npm >/dev/null 2>&1; then
        print_status $RED "✗ npm is not installed"
        return 1
    fi
    
    # Install dependencies
    print_status $YELLOW "Installing JavaScript dependencies..."
    npm install
    
    # Build the project
    print_status $YELLOW "Building JavaScript project..."
    npm run build
    
    # Run tests
    local jest_flags=""
    if [[ $VERBOSE == true ]]; then
        jest_flags="--verbose --coverage"
    fi
    
    if [[ $RUN_PERFORMANCE == true ]]; then
        jest_flags="$jest_flags --testNamePattern='Performance|performance'"
    fi
    
    print_status $YELLOW "Running JavaScript tests..."
    npm test -- $jest_flags
    
    if [[ $? -eq 0 ]]; then
        print_status $GREEN "✓ JavaScript tests passed"
    else
        print_status $RED "✗ JavaScript tests failed"
        return 1
    fi
    
    cd ..
}

# Function to run tests in parallel
run_tests_parallel() {
    local pids=()
    local results=()
    
    if [[ $RUN_GO == true ]]; then
        run_go_tests &
        pids+=($!)
    fi
    
    if [[ $RUN_PYTHON == true ]]; then
        run_python_tests &
        pids+=($!)
    fi
    
    if [[ $RUN_JAVASCRIPT == true ]]; then
        run_javascript_tests &
        pids+=($!)
    fi
    
    # Wait for all tests to complete
    for pid in "${pids[@]}"; do
        wait $pid
        results+=($?)
    done
    
    # Check results
    local failed=false
    for result in "${results[@]}"; do
        if [[ $result -ne 0 ]]; then
            failed=true
            break
        fi
    done
    
    if [[ $failed == true ]]; then
        return 1
    else
        return 0
    fi
}

# Function to run tests sequentially
run_tests_sequential() {
    local failed=false
    
    if [[ $RUN_GO == true ]]; then
        if ! run_go_tests; then
            failed=true
        fi
    fi
    
    if [[ $RUN_PYTHON == true ]]; then
        if ! run_python_tests; then
            failed=true
        fi
    fi
    
    if [[ $RUN_JAVASCRIPT == true ]]; then
        if ! run_javascript_tests; then
            failed=true
        fi
    fi
    
    if [[ $failed == true ]]; then
        return 1
    else
        return 0
    fi
}

# Main execution
main() {
    print_status $BLUE "MantisDB Client Library Test Runner"
    print_status $BLUE "=================================="
    
    # Check server availability
    if ! check_server; then
        exit 1
    fi
    
    # Show configuration
    print_status $YELLOW "Test Configuration:"
    print_status $YELLOW "  Host: ${MANTISDB_TEST_HOST}"
    print_status $YELLOW "  Port: ${MANTISDB_TEST_PORT}"
    print_status $YELLOW "  Username: ${MANTISDB_TEST_USERNAME}"
    print_status $YELLOW "  Go tests: ${RUN_GO}"
    print_status $YELLOW "  Python tests: ${RUN_PYTHON}"
    print_status $YELLOW "  JavaScript tests: ${RUN_JAVASCRIPT}"
    print_status $YELLOW "  Performance tests: ${RUN_PERFORMANCE}"
    print_status $YELLOW "  Load tests: ${RUN_LOAD_TESTS}"
    print_status $YELLOW "  Parallel execution: ${PARALLEL}"
    echo ""
    
    # Change to clients directory
    cd "$(dirname "$0")"
    
    # Run tests
    local start_time=$(date +%s)
    
    if [[ $PARALLEL == true ]]; then
        print_status $YELLOW "Running tests in parallel..."
        run_tests_parallel
    else
        print_status $YELLOW "Running tests sequentially..."
        run_tests_sequential
    fi
    
    local exit_code=$?
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    echo ""
    print_status $BLUE "Test Summary"
    print_status $BLUE "============"
    print_status $YELLOW "Total duration: ${duration} seconds"
    
    if [[ $exit_code -eq 0 ]]; then
        print_status $GREEN "✓ All tests passed!"
    else
        print_status $RED "✗ Some tests failed!"
    fi
    
    exit $exit_code
}

# Run main function
main "$@"