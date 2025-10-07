# MantisDB Testing Framework

This framework provides comprehensive testing for MantisDB, including edge case testing and reliability testing to ensure robust operation under extreme conditions and failure scenarios.

## Features

## Edge Case Testing

### 1. Large Document Testing
- Tests documents larger than 1MB (up to 10MB)
- Validates document integrity and storage/retrieval performance
- Monitors memory usage during operations
- Includes checksum validation for data integrity

### 2. High TTL Value Testing
- Tests TTL values greater than 24 hours (up to 1 year)
- Validates TTL overflow detection and handling
- Tests TTL precision and accuracy
- Ensures proper TTL management for long-lived data

### 3. Concurrent Write Testing
- Tests multiple goroutines writing to the same key simultaneously
- Validates data consistency after concurrent operations
- Includes race condition detection mechanisms
- Tests various concurrency levels (10-500 workers)

### 4. Memory Pressure Testing
- Simulates memory pressure scenarios
- Tests cache eviction policies (LRU, LFU)
- Validates cache consistency during eviction
- Monitors system behavior under memory constraints

## Reliability Testing

### 1. Crash Recovery Testing
- Tests system recovery after simulated process crashes
- Validates data integrity after unexpected shutdowns
- Tests transaction rollback verification
- Includes process management utilities for controlled crashes

### 2. Disk Space Exhaustion Testing
- Simulates disk full conditions and tests graceful error handling
- Monitors disk space usage and validates error responses
- Tests recovery after disk space restoration
- Includes disk space monitoring and simulation tools

### 3. Memory Limit Testing
- Tests memory pressure handling and graceful degradation
- Monitors memory usage and implements pressure detection
- Validates system behavior under memory constraints
- Includes memory usage monitoring and limiting tools

### 4. Concurrent Access Pattern Testing
- Tests high-concurrency scenarios with deadlock detection
- Validates performance under concurrent load
- Implements deadlock prevention mechanisms
- Includes performance benchmarking under concurrent access

## Usage

### Edge Case Testing Command Line Interface

```bash
# Run all edge case tests
go run cmd/edge-case-tests/main.go

# Run specific test type
go run cmd/edge-case-tests/main.go -test large-documents

# Use custom configuration
go run cmd/edge-case-tests/main.go -config configs/edge_case_tests.json
```

For complete documentation, see the [testing package documentation](../testing/).