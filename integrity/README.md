# Data Integrity System

The integrity package provides comprehensive data integrity verification, corruption detection, and monitoring capabilities for MantisDB. This system is designed to ensure data safety and detect corruption in real-time.

## Features

- **Checksum Engine**: Multiple checksum algorithms (CRC32, MD5, SHA256)
- **Corruption Detection**: Real-time and background corruption detection
- **WAL Integrity**: Write-Ahead Log entry verification and repair
- **Monitoring**: Comprehensive metrics, health checks, and alerting
- **Alert System**: Configurable alert handlers for different notification methods

## Components

### 1. ChecksumEngine

Provides data integrity verification through checksums:

```go
engine := integrity.NewChecksumEngine(integrity.ChecksumCRC32)

// Calculate checksum
data := []byte("important data")
checksum := engine.Calculate(data)

// Verify data integrity
err := engine.Verify(data, checksum)
if err != nil {
    // Handle corruption
}

// Batch operations
dataBlocks := [][]byte{data1, data2, data3}
checksums := engine.CalculateBatch(dataBlocks)
errors := engine.VerifyBatch(dataBlocks, checksums)
```

### 2. CorruptionDetector

Detects data corruption in real-time and through background scanning:

```go
config := integrity.DefaultIntegrityConfig()
detector := integrity.NewCorruptionDetector(config)

// Real-time corruption detection
event := detector.DetectCorruption(data, expectedChecksum)
if event != nil {
    // Handle corruption event
    fmt.Printf("Corruption detected: %s\n", event.Description)
}

// Background scanning
err := detector.StartBackgroundScan("/data/directory")
if err != nil {
    // Handle error
}

// Get corruption statistics
stats := detector.GetCorruptionStats()
fmt.Printf("Total corruption events: %d\n", stats.TotalEvents)
```

### 3. WALIntegrityVerifier

Verifies Write-Ahead Log integrity and provides repair capabilities:

```go
verifier := integrity.NewWALIntegrityVerifier(config)

// Verify a single WAL file
result, err := verifier.VerifyWALFile("/path/to/wal/file.log")
if err != nil {
    // Handle error
}

if result.Status == integrity.WALStatusCorrupted {
    // Attempt repair
    repairResult, err := verifier.RepairWALFile("/path/to/wal/file.log")
    if err != nil {
        // Handle repair failure
    }
    
    if repairResult.Success {
        fmt.Printf("Repaired %d entries\n", repairResult.RepairedEntries)
    }
}

// Verify entire WAL directory
results, err := verifier.VerifyWALDirectory("/path/to/wal/directory")
```

### 4. IntegrityMonitor

Provides comprehensive monitoring, metrics, and alerting:

```go
monitor := integrity.NewIntegrityMonitor(config)

// Register alert handlers
logHandler := integrity.NewLogAlertHandler()
fileHandler, _ := integrity.NewFileAlertHandler("/var/log/integrity.log")
monitor.RegisterAlertHandler(logHandler)
monitor.RegisterAlertHandler(fileHandler)

// Record operations
monitor.RecordChecksumOperation("verify_data", duration, success)

// Get health status
health := monitor.PerformHealthCheck()
fmt.Printf("System health: %s\n", health.Status)

// Get metrics
metrics := monitor.GetIntegrityMetrics()
fmt.Printf("Operations: %d\n", metrics.ChecksumOperations.TotalOperations)
```

### 5. IntegritySystem

The main system that coordinates all components:

```go
// Create and configure the system
config := integrity.DefaultIntegrityConfig()
config.ChecksumAlgorithm = integrity.ChecksumCRC32
config.EnableBackgroundScan = true

system := integrity.NewIntegritySystem(config)

// Start the system
err := system.Start()
if err != nil {
    // Handle startup error
}
defer system.Stop()

// Register alert handlers
logHandler := integrity.NewLogAlertHandler()
system.RegisterAlertHandler(logHandler)

// Use the system
data := []byte("important data")
checksum, err := system.CalculateAndVerifyChecksum(data, "data_location")
if err != nil {
    // Handle error
}

// Verify data
err = system.VerifyData(data, "data_location", checksum)
if err != nil {
    // Handle corruption
}

// Start background scanning
err = system.StartBackgroundScan("/data/directory")
if err != nil {
    // Handle error
}

// Get system status
health := system.GetHealthStatus()
metrics := system.GetMetrics()
```

## Configuration

The system is configured using `IntegrityConfig`:

```go
config := &integrity.IntegrityConfig{
    ChecksumAlgorithm:       integrity.ChecksumCRC32,
    EnableBackgroundScan:    true,
    ScanInterval:            1 * time.Hour,
    MaxConcurrentScans:      2,
    EnableRealTimeDetection: true,
    EnableAutoRecovery:      false,
    AlertThresholds: integrity.AlertThresholds{
        CorruptionRate: 0.01, // 1%
        FailureRate:    0.05, // 5%
        ResponseTime:   5 * time.Second,
        MemoryUsage:    1024 * 1024 * 1024, // 1GB
        DiskUsage:      0.90, // 90%
    },
    RetentionPeriod: 30 * 24 * time.Hour, // 30 days
}
```

## Alert Handlers

The system supports multiple alert handlers:

### LogAlertHandler
Logs alerts to stdout:
```go
handler := integrity.NewLogAlertHandler()
```

### FileAlertHandler
Writes alerts to a file:
```go
handler, err := integrity.NewFileAlertHandler("/var/log/integrity-alerts.log")
```

### MultiAlertHandler
Forwards alerts to multiple handlers:
```go
multiHandler := integrity.NewMultiAlertHandler(logHandler, fileHandler)
```

### ThresholdAlertHandler
Filters alerts by severity level:
```go
criticalHandler := integrity.NewThresholdAlertHandler(
    integrity.AlertLevelCritical, 
    fileHandler,
)
```

## Error Types

The system defines several error types for different integrity violations:

- `ChecksumMismatchError`: Checksum verification failure
- `FileChecksumMismatchError`: File checksum verification failure
- `CorruptionDetectedError`: General corruption detection
- `IntegrityViolationError`: General integrity violation

## Metrics and Monitoring

The system provides comprehensive metrics:

- **Operation Metrics**: Success/failure rates, latency, throughput
- **Corruption Stats**: Event counts by type and severity
- **Health Status**: Component health and overall system status
- **Performance Metrics**: Memory usage, CPU usage, I/O operations

## Integration with MantisDB

The integrity system integrates with MantisDB components:

1. **WAL Integration**: Verifies WAL entry integrity during writes and recovery
2. **Storage Integration**: Validates data integrity during reads and writes
3. **Transaction Integration**: Ensures transaction data integrity
4. **Error Handling**: Integrates with the error handling system

## Best Practices

1. **Choose Appropriate Algorithm**: CRC32 for performance, SHA256 for security
2. **Configure Thresholds**: Set appropriate alert thresholds for your environment
3. **Monitor Regularly**: Use health checks and metrics to monitor system health
4. **Handle Alerts**: Implement proper alert handling for your operational needs
5. **Background Scanning**: Enable background scanning for proactive corruption detection
6. **Backup Before Repair**: Always create backups before attempting WAL repairs

## Performance Considerations

- CRC32 is fastest but less secure than cryptographic hashes
- Background scanning can impact I/O performance
- Batch operations are more efficient than individual operations
- Monitor memory usage with large datasets
- Consider scan frequency based on your data change rate

## Testing

Run the test suite to verify functionality:

```bash
go test ./integrity/...
```

Run benchmarks to measure performance:

```bash
go test -bench=. ./integrity/...
```

## Example Usage

See `example_test.go` for comprehensive usage examples and test cases.