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

# Output results to file
go run cmd/edge-case-tests/main.go -output results.json -format json

# Verbose output
go run cmd/edge-case-tests/main.go -verbose

# Generate default configuration
go run cmd/edge-case-tests/main.go -save-config my_config.json
```

### Reliability Testing Command Line Interface

```bash
# Run all reliability tests
go run cmd/reliability-tests/main.go

# Run specific reliability test type
go run cmd/reliability-tests/main.go -test crash-recovery
go run cmd/reliability-tests/main.go -test disk-space
go run cmd/reliability-tests/main.go -test memory-limits
go run cmd/reliability-tests/main.go -test concurrent-access

# Use custom configuration
go run cmd/reliability-tests/main.go -config configs/reliability_tests.json

# Output results to file
go run cmd/reliability-tests/main.go -output reliability_results.json -format json

# Verbose output with detailed metrics
go run cmd/reliability-tests/main.go -verbose

# Generate default configuration
go run cmd/reliability-tests/main.go -save-config my_reliability_config.json

# Run only specific test categories
go run cmd/reliability-tests/main.go -enable-crash=false -enable-disk=false
```

### Programmatic Usage

#### Edge Case Testing

```go
package main

import (
    "context"
    "mantisDB/testing"
    "mantisDB/store"
    "mantisDB/storage"
    "mantisDB/cache"
)

func main() {
    // Initialize MantisDB components
    storageEngine := storage.NewPureGoStorageEngine(storage.StorageConfig{
        DataDir: "./test_data",
        // ... other config
    })
    cacheManager := cache.NewCacheManager(cache.CacheConfig{
        MaxSize: 50 * 1024 * 1024,
        // ... other config
    })
    mantisStore := store.NewMantisStore(storageEngine, cacheManager)

    // Create and run edge case test suite
    testSuite := testing.NewEdgeCaseTestSuite(mantisStore)
    results, err := testSuite.RunAllTests(context.Background())
    if err != nil {
        panic(err)
    }

    // Process results
    for testName, result := range results.Tests {
        fmt.Printf("Test %s: %v\n", testName, result.Success)
    }
}
```

#### Reliability Testing

```go
package main

import (
    "context"
    "mantisDB/testing"
    "mantisDB/store"
    "mantisDB/storage"
    "mantisDB/cache"
)

func main() {
    // Initialize MantisDB components
    storageEngine := storage.NewPureGoStorageEngine(storage.StorageConfig{
        DataDir: "./test_data_reliability",
        // ... other config
    })
    cacheManager := cache.NewCacheManager(cache.CacheConfig{
        MaxSize: 50 * 1024 * 1024,
        // ... other config
    })
    mantisStore := store.NewMantisStore(storageEngine, cacheManager)

    // Create and run reliability test suite
    reliabilityTestSuite := testing.NewReliabilityTestSuite(mantisStore)
    results, err := reliabilityTestSuite.RunAllTests(context.Background())
    if err != nil {
        panic(err)
    }

    // Process results
    for testName, result := range results.Tests {
        fmt.Printf("Reliability Test %s: %v\n", testName, result.Success)
    }
}
```

## Configuration

The test framework uses JSON configuration files to customize test parameters:

```json
{
  "storage_config": {
    "data_dir": "./test_data_edge_cases",
    "buffer_size": 1048576,
    "cache_size": 52428800,
    "use_cgo": false,
    "sync_writes": true
  },
  "cache_config": {
    "max_size": 52428800,
    "default_ttl": 3600000000000,
    "cleanup_interval": 300000000000,
    "eviction_policy": "lru"
  },
  "test_config": {
    "large_document_sizes": [1048576, 5242880, 10485760],
    "high_ttl_values": [90000, 604800, 2592000],
    "concurrency_levels": [10, 50, 100, 500],
    "memory_pressure_levels": [0.5, 0.7, 0.8, 0.9],
    "timeout_duration": 1800000000000
  },
  "output_format": "text",
  "output_file": "",
  "verbose": false
}
```

## Test Types

### Edge Case Test Types
- `large-documents`: Large document handling tests
- `high-ttl`: High TTL value tests
- `concurrent-writes`: Concurrent write operation tests
- `memory-pressure`: Memory pressure and cache eviction tests

### Reliability Test Types
- `crash-recovery`: Crash recovery and data integrity tests
- `disk-space`: Disk space exhaustion handling tests
- `memory-limits`: Memory limit and pressure handling tests
- `concurrent-access`: High-concurrency and deadlock detection tests

### Output Formats
- `text`: Human-readable text format (default)
- `json`: Machine-readable JSON format
- `html`: HTML report with styling

## Test Results

Each test provides detailed metrics including:
- Execution duration
- Memory usage statistics
- Performance metrics
- Error counts and details
- Success/failure status

### Example Output

#### Edge Case Test Results
```
Edge Case Test Results
======================
Start Time: 2024-01-15T10:30:00Z
End Time: 2024-01-15T10:45:00Z
Total Duration: 15m0s

✅ PASS large_documents (Duration: 5m30s)
  ✅ PASS document_1MB (Duration: 30s)
  ✅ PASS document_5MB (Duration: 2m0s)
  ✅ PASS document_10MB (Duration: 3m0s)

✅ PASS high_ttl (Duration: 2m15s)
  ✅ PASS ttl_25_hours (Duration: 15s)
  ✅ PASS ttl_168_hours (Duration: 30s)
  ✅ PASS ttl_720_hours (Duration: 45s)

Test Summary
============
Total Tests: 12
Passed: 12
Failed: 0
Success Rate: 100.0%
Total Duration: 15m0s

🎉 All tests passed!
```

#### Reliability Test Results
```
Reliability Test Results
========================
Start Time: 2024-01-15T11:00:00Z
End Time: 2024-01-15T11:25:00Z
Total Duration: 25m0s

✅ PASS crash_recovery (Duration: 8m30s)
  ✅ PASS crash_during_write (Duration: 2m0s)
  ✅ PASS crash_during_transaction (Duration: 3m0s)
  ✅ PASS data_integrity_recovery (Duration: 2m30s)
  ✅ PASS transaction_rollback (Duration: 1m0s)

✅ PASS disk_space (Duration: 6m15s)
  ✅ PASS graceful_error_handling (Duration: 2m30s)
  ✅ PASS disk_space_monitoring (Duration: 1m45s)
  ✅ PASS recovery_after_restoration (Duration: 2m0s)

✅ PASS memory_limits (Duration: 5m30s)
  ✅ PASS memory_monitoring (Duration: 1m30s)
  ✅ PASS memory_pressure_handling (Duration: 2m0s)
  ✅ PASS graceful_degradation (Duration: 2m0s)

✅ PASS concurrent_access (Duration: 4m45s)
  ✅ PASS high_concurrency (Duration: 2m0s)
  ✅ PASS deadlock_detection (Duration: 1m30s)
  ✅ PASS performance_benchmarking (Duration: 1m15s)

Reliability Test Summary
========================
Total Tests: 16
Passed: 16
Failed: 0
Success Rate: 100.0%
Total Duration: 25m0s

🎉 All reliability tests passed!
```

## Requirements

- Go 1.19 or later
- MantisDB storage and cache components
- Sufficient disk space for test data
- Adequate memory for memory pressure tests

## Best Practices

1. **Run tests in isolation**: Use a separate test environment
2. **Monitor resources**: Watch memory and disk usage during tests
3. **Clean up**: Tests automatically clean up, but verify test data removal
4. **Customize configuration**: Adjust test parameters based on your system
5. **Regular testing**: Include edge case tests in your CI/CD pipeline

## Troubleshooting

### Common Issues

1. **Out of memory errors**: Reduce memory pressure levels or increase system memory
2. **Timeout errors**: Increase timeout duration in configuration
3. **Disk space errors**: Ensure sufficient disk space for test data
4. **Concurrent access errors**: These may be expected in race condition tests

### Debug Mode

Use verbose mode (`-verbose`) to get detailed metrics and debug information:

#### Edge Case Tests
```bash
go run cmd/edge-case-tests/main.go -verbose
```

#### Reliability Tests
```bash
go run cmd/reliability-tests/main.go -verbose
```

This will show detailed metrics for each test including memory usage, timing, performance statistics, and internal diagnostics.

## Test Categories

### Edge Case Tests
Focus on extreme conditions and stress scenarios:
- **Large Documents**: Tests handling of documents >1MB
- **High TTL Values**: Tests TTL values >24 hours  
- **Concurrent Writes**: Tests race conditions and data consistency
- **Memory Pressure**: Tests cache eviction under memory constraints

### Reliability Tests
Focus on failure scenarios and recovery:
- **Crash Recovery**: Tests recovery after process crashes and data integrity
- **Disk Space**: Tests behavior when disk space is exhausted
- **Memory Limits**: Tests memory pressure handling and graceful degradation
- **Concurrent Access**: Tests high-concurrency scenarios and deadlock prevention

## Integration with CI/CD

Both test suites can be integrated into CI/CD pipelines:

```yaml
# Example GitHub Actions workflow
- name: Run Edge Case Tests
  run: go run cmd/edge-case-tests/main.go -output edge_results.json -format json

- name: Run Reliability Tests  
  run: go run cmd/reliability-tests/main.go -output reliability_results.json -format json

- name: Upload Test Results
  uses: actions/upload-artifact@v2
  with:
    name: test-results
    path: |
      edge_results.json
      reliability_results.json
```