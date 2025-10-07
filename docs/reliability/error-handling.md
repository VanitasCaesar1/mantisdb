# Comprehensive Error Handling System

This package implements a comprehensive error handling system for MantisDB that addresses all critical data safety requirements related to error handling, resource exhaustion, and corruption detection.

## Components

### 1. Error Handler Framework (`error_handler.go`, `error_handler_impl.go`)

The core error handling framework provides:
- **Error Classification**: Automatically classifies errors by category (IO, Memory, Disk, Corruption, etc.) and severity
- **Error Context Tracking**: Maintains detailed context information for each error
- **Recovery Strategy Selection**: Determines appropriate recovery actions based on error characteristics
- **Configurable Policies**: Supports different error handling policies and thresholds

**Key Features:**
- Automatic error classification based on error types and messages
- Configurable retry policies with exponential backoff
- Circuit breaker pattern for failing operations
- Graceful degradation under adverse conditions

### 2. Disk Space Monitoring (`disk_monitor.go`, `disk_recovery.go`)

Comprehensive disk space monitoring and recovery system:
- **Real-time Monitoring**: Continuously monitors disk space usage across multiple paths
- **Threshold-based Alerts**: Configurable warning, critical, and emergency thresholds
- **Graceful Write Rejection**: Prevents writes when disk space is insufficient
- **Automatic Cleanup**: Performs cleanup tasks to recover disk space

**Key Features:**
- Multi-path disk space monitoring with individual thresholds
- Automatic cleanup tasks (temp files, logs, WAL compaction, old checkpoints)
- Emergency cleanup mode for critical situations
- Detailed disk space statistics and alerting

### 3. Memory Monitoring (`memory_monitor.go`, `memory_recovery.go`)

Advanced memory monitoring and recovery system:
- **Memory Pressure Detection**: Monitors memory usage and detects pressure conditions
- **Graceful Degradation**: Applies different levels of degradation based on memory pressure
- **Automatic Recovery**: Performs memory recovery tasks including aggressive garbage collection
- **Allocation Control**: Prevents allocations that would exceed memory limits

**Key Features:**
- Real-time memory statistics collection
- Multiple degradation levels (Light, Moderate, Severe, Critical)
- Automatic garbage collection tuning
- Memory recovery tasks with priority-based execution

### 4. I/O Error Handling (`io_handler.go`, `io_operations.go`)

Robust I/O error handling with retry logic and circuit breaker pattern:
- **Exponential Backoff Retry**: Configurable retry logic with exponential backoff and jitter
- **Circuit Breaker Pattern**: Prevents cascading failures by opening circuits for failing resources
- **Error Classification**: Classifies I/O errors by type (timeout, connection issues, hardware failures)
- **High-level Operations**: Provides retry-enabled file operations (read, write, copy, delete, etc.)

**Key Features:**
- Automatic retry with exponential backoff and jitter
- Circuit breaker with configurable failure thresholds
- Comprehensive I/O error classification
- Batch file operations with individual retry logic

### 5. Corruption Detection (`corruption_detector.go`, `corruption_recovery.go`)

Advanced corruption detection and recovery system:
- **Checksum Verification**: Supports multiple checksum algorithms (CRC32, MD5, SHA256)
- **Real-time Detection**: Verifies data integrity during read/write operations
- **Background Scanning**: Periodic integrity scans of data directories
- **Automatic Isolation**: Isolates corrupted data to prevent further damage
- **Recovery Procedures**: Multiple recovery strategies including backup restore

**Key Features:**
- Multiple checksum algorithms with configurable selection
- Real-time and background corruption detection
- Automatic corruption event logging and alerting
- Priority-based recovery task execution
- Backup creation and restoration

## Usage Examples

### Basic Error Handler Setup

```go
// Create error handler with default configuration
config := &ErrorHandlerConfig{
    MaxRetries:        5,
    BaseRetryDelay:    100 * time.Millisecond,
    MaxRetryDelay:     30 * time.Second,
    EnableGracefulDegradation: true,
}
handler := NewDefaultErrorHandler(config)

// Handle an error
context := ErrorContext{
    Operation: "database_write",
    Resource:  "data_file",
    Severity:  ErrorSeverityHigh,
    Category:  ErrorCategoryIO,
}

action := handler.HandleError(err, context)
switch action {
case ErrorActionRetry:
    // Retry the operation
case ErrorActionFail:
    // Operation failed permanently
case ErrorActionDegrade:
    // Apply graceful degradation
}
```

### Disk Space Monitoring

```go
// Create disk monitor
config := &DiskMonitorConfig{
    CheckInterval:     30 * time.Second,
    WarningThreshold:  0.80, // 80%
    CriticalThreshold: 0.90, // 90%
    EmergencyThreshold: 0.95, // 95%
}
monitor := NewDiskSpaceMonitor(config)

// Add paths to monitor
monitor.AddPath("/var/lib/mantisdb/data")
monitor.AddPath("/var/lib/mantisdb/wal")

// Start monitoring
monitor.StartMonitoring()

// Check if write can proceed
err := monitor.CanWrite("/var/lib/mantisdb/data", 1024*1024) // 1MB
if err != nil {
    // Handle insufficient disk space
}
```

### Memory Monitoring

```go
// Create memory monitor
config := &MemoryMonitorConfig{
    CheckInterval:     10 * time.Second,
    WarningThreshold:  0.70, // 70%
    CriticalThreshold: 0.85, // 85%
    EmergencyThreshold: 0.95, // 95%
}
monitor := NewMemoryMonitor(config)

// Start monitoring
monitor.StartMonitoring()

// Check memory pressure
if monitor.CheckMemoryPressure() {
    // Apply graceful degradation
    monitor.ApplyGracefulDegradation()
}

// Check if allocation can proceed
err := monitor.CanAllocate(1024 * 1024) // 1MB
if err != nil {
    // Handle memory exhaustion
}
```

### I/O Operations with Retry

```go
// Create I/O error handler
config := &IOErrorConfig{
    MaxRetries:      5,
    BaseRetryDelay:  100 * time.Millisecond,
    MaxRetryDelay:   30 * time.Second,
    JitterEnabled:   true,
}
handler := NewIOErrorHandler(config)

// Create I/O operation manager
manager := NewIOOperationManager(handler)

// Perform file operations with retry
ctx := context.Background()
data, op := manager.ReadFile(ctx, "/path/to/file")
if !op.Success {
    // Handle read failure
    fmt.Printf("Read failed: %v\n", op.Error)
}

writeOp := manager.WriteFile(ctx, "/path/to/file", data, 0644)
if !writeOp.Success {
    // Handle write failure
    fmt.Printf("Write failed: %v\n", writeOp.Error)
}
```

### Corruption Detection

```go
// Create corruption detector
config := &CorruptionDetectorConfig{
    EnableRealTimeChecking: true,
    EnableBackgroundScan:   true,
    ScanInterval:          1 * time.Hour,
    ChecksumAlgorithm:     ChecksumCRC32,
    EnableAutoIsolation:   true,
}
detector := NewCorruptionDetector(config)

// Verify data integrity
data := []byte("important data")
expectedChecksum := detector.CalculateChecksum(data)

// Later, verify the data
event := detector.VerifyData(data, expectedChecksum)
if event != nil {
    // Corruption detected
    fmt.Printf("Corruption detected: %s\n", event.Description)
}

// Scan directory for corruption
result := detector.ScanDirectory("/var/lib/mantisdb/data")
if result.CorruptionsFound > 0 {
    // Handle detected corruptions
    for _, event := range result.Events {
        fmt.Printf("Corruption: %s at %s\n", event.Description, event.Location.File)
    }
}
```

## Integration with MantisDB

This error handling system integrates with MantisDB's core components:

1. **WAL System**: Uses I/O error handling for WAL file operations and corruption detection for WAL integrity
2. **Transaction Manager**: Uses error classification for transaction failures and memory monitoring for transaction limits
3. **Storage Engine**: Uses disk monitoring for storage operations and corruption detection for data integrity
4. **Recovery System**: Uses all error handling components for robust recovery procedures

## Configuration

All components support comprehensive configuration through their respective config structures:
- Error handling policies and thresholds
- Monitoring intervals and alert thresholds
- Retry policies and circuit breaker settings
- Recovery strategies and cleanup procedures

## Monitoring and Observability

The system provides extensive monitoring capabilities:
- Real-time metrics for all error handling components
- Detailed event logging and audit trails
- Configurable alerting for critical conditions
- Statistics and reporting for operational insights

## Thread Safety

All components are designed to be thread-safe and can be used concurrently across multiple goroutines without additional synchronization.