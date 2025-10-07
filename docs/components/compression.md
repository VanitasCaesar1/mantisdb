# MantisDB Compression System

The MantisDB compression system provides transparent data compression with multiple algorithms, cold data detection, and comprehensive monitoring capabilities.

## Features

### 🗜️ Multiple Compression Algorithms
- **LZ4**: Fast compression/decompression, ideal for real-time operations
- **Snappy**: Balanced speed and compression ratio, good for streaming data
- **ZSTD**: High compression ratio, perfect for cold data and archival storage

### 🧊 Cold Data Detection
- Bloom filter-based access pattern tracking
- Configurable policies for identifying cold data
- Automatic background compression of infrequently accessed data

### 📊 Comprehensive Monitoring
- Real-time compression metrics and performance tracking
- HTTP endpoints for metrics visualization
- Configurable alerts and recommendations
- Time-series data collection for trend analysis

### 🔄 Transparent Operation
- Automatic compression/decompression based on policies
- No application code changes required
- Configurable compression thresholds and algorithms

## Quick Start

### Basic Usage

```go
package main

import (
    "mantisDB/advanced/compression"
    "time"
)

func main() {
    // Create compression manager
    config := &compression.CompressionManagerConfig{
        Enabled:                true,
        BackgroundCompression:  true,
        CompressionInterval:    5 * time.Minute,
        CompressionThreshold:   1024, // 1KB
    }
    
    manager := compression.NewCompressionManager(config)
    defer manager.Shutdown()
    
    // Write data (automatically compressed if it meets criteria)
    data := []byte("Your data here...")
    compressed, err := manager.Write("my_key", data)
    if err != nil {
        panic(err)
    }
    
    // Read data (automatically decompressed)
    decompressed, err := manager.Read("my_key", compressed)
    if err != nil {
        panic(err)
    }
}
```

For complete documentation, see the [compression package documentation](../advanced/compression/).