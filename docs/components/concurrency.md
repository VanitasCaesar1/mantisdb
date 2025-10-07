# Enhanced Concurrency System

This package provides a comprehensive, production-ready concurrency control system for MantisDB with advanced features including hierarchical locking, deadlock detection, performance monitoring, and goroutine lifecycle management.

## Features

### 🔒 Enhanced Lock Manager
- **Hierarchical Locking**: Prevents deadlocks through ordered lock acquisition
- **Fast Path Optimization**: Optimized path for uncontended locks
- **Lock Pooling**: Reduces allocation overhead through object reuse
- **Adaptive Timeouts**: Dynamic timeout adjustment based on system load
- **Comprehensive Metrics**: Detailed performance tracking and analysis

### 🕸️ Advanced Deadlock Detection
- **Multiple Detection Algorithms**: DFS, BFS, Tarjan's SCC, and adaptive strategies
- **Smart Victim Selection**: Multiple strategies including cost-based selection
- **Adaptive Timeout Management**: Dynamic timeout adjustment based on system load
- **Performance Monitoring**: Detailed metrics on detection performance

### 📊 Lock Profiling and Monitoring
- **Event History Tracking**: Detailed history of lock events per resource
- **Contention Analysis**: Comprehensive contention statistics and hotspot detection
- **Performance Metrics**: Wait times, hold times, queue depths, and throughput
- **Real-time Monitoring**: Live performance data with configurable sampling

### 📈 Metrics Export
- **JSON Metrics API**: RESTful endpoint for metrics consumption
- **Resource-level Metrics**: Per-resource performance statistics
- **Global System Metrics**: System-wide performance indicators
- **Health Monitoring**: System health checks and status reporting

### 🧵 Goroutine Lifecycle Management
- **Controlled Goroutine Pools**: Managed worker pools with automatic scaling
- **Leak Detection**: Automatic detection and cleanup of leaked goroutines
- **Performance Monitoring**: Goroutine performance and resource usage tracking
- **Graceful Shutdown**: Proper cleanup of all managed goroutines

## Quick Start

```go
package main

import (
    "log"
    "github.com/mantisdb/pkg/concurrency"
)

func main() {
    // Create and start the concurrency system
    system := concurrency.NewEnhancedConcurrencySystem(nil) // Uses default config
    
    if err := system.Start(); err != nil {
        log.Fatal(err)
    }
    defer system.Stop()
    
    // Use the system for lock management
    txnID := uint64(1)
    resource := "user:123"
    
    // Acquire a lock with full monitoring
    if err := system.AcquireLock(txnID, resource, concurrency.ReadLock); err != nil {
        log.Printf("Failed to acquire lock: %v", err)
        return
    }
    
    // Do work...
    
    // Release the lock
    if err := system.ReleaseLock(txnID, resource); err != nil {
        log.Printf("Failed to release lock: %v", err)
    }
}
```

For complete documentation, see the [concurrency package documentation](../pkg/concurrency/).