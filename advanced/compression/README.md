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

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                 Compression Manager                         │
├─────────────────────────────────────────────────────────────┤
│  Transparent Compression    │  Cold Data Detection          │
│  ├── Algorithm Selection    │  ├── Bloom Filter             │
│  ├── Policy Engine         │  ├── Access Tracking          │
│  └── Background Worker     │  └── Configurable Policies    │
├─────────────────────────────────────────────────────────────┤
│  Compression Engine         │  Monitoring & Reporting       │
│  ├── LZ4 Algorithm         │  ├── Metrics Collection       │
│  ├── Snappy Algorithm      │  ├── Performance Tracking     │
│  ├── ZSTD Algorithm        │  ├── Alert System             │
│  └── Custom Algorithms     │  └── HTTP Endpoints           │
└─────────────────────────────────────────────────────────────┘
```

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
    
    // Get compression statistics
    stats := manager.GetCompressionStats()
    // Use stats...
}
```

### Transparent Compression

```go
// Create transparent compression layer
config := &compression.TransparentConfig{
    Enabled:           true,
    MinSize:           1024,
    ColdThreshold:     24 * time.Hour,
    DefaultAlgorithm:  "lz4",
}

tc := compression.NewTransparentCompression(config)

// Data is automatically compressed/decompressed
compressed, _ := tc.Write("key", data)
decompressed, _ := tc.Read("key", compressed)
```

### Cold Data Detection

```go
// Configure cold data detection
config := &compression.ColdDataConfig{
    Enabled:              true,
    ColdThreshold:        24 * time.Hour,
    BloomFilterSize:      1000000,
    SizeThreshold:        1024,
}

detector := compression.NewColdDataDetector(config)

// Record data access
detector.RecordAccess("my_key", 2048)

// Check if data is cold
metadata := &compression.DataMetadata{
    Size:         2048,
    LastAccessed: time.Now().Add(-25 * time.Hour),
    AccessCount:  1,
}
isCold := detector.IsCold("my_key", metadata)

// Get cold data candidates for compression
candidates := detector.GetColdDataCandidates(100)
```

## Configuration

### Compression Manager Configuration

```go
type CompressionManagerConfig struct {
    Enabled                bool          // Enable/disable compression
    BackgroundCompression  bool          // Enable background compression
    CompressionInterval    time.Duration // Interval for background compression
    MaxCandidatesPerCycle  int           // Max candidates per compression cycle
    CompressionThreshold   int64         // Minimum size for compression
    DecompressionCacheSize int           // Cache size for decompressed data
}
```

### Cold Data Detection Configuration

```go
type ColdDataConfig struct {
    Enabled              bool          // Enable cold data detection
    ColdThreshold        time.Duration // Time threshold for cold data
    BloomFilterSize      uint          // Bloom filter size (bits)
    BloomFilterHashCount uint          // Number of hash functions
    AccessTrackingWindow time.Duration // Window for tracking access
    MinAccessCount       int64         // Minimum access count threshold
    SizeThreshold        int64         // Minimum size for compression
    CleanupInterval      time.Duration // Cleanup interval
}
```

## Monitoring and Metrics

### HTTP Endpoints

The compression system exposes several HTTP endpoints for monitoring:

- `GET /metrics` - Overall compression metrics
- `GET /metrics/dashboard` - Dashboard data for visualization
- `GET /metrics/report` - Comprehensive compression report
- `GET /metrics/alerts` - Active alerts
- `GET /metrics/algorithms` - Algorithm-specific metrics
- `GET /metrics/timeseries?metric=<name>` - Time series data

### Metrics Available

- **Compression Ratio**: Overall and per-algorithm compression ratios
- **Throughput**: Compression and decompression rates (MB/s)
- **Latency**: Average compression and decompression times
- **Storage Savings**: Total bytes saved through compression
- **CPU/Memory Overhead**: Resource usage impact
- **Cold Data Statistics**: Access patterns and cold data identification

### Example Dashboard Data

```json
{
  "overview": {
    "total_data_processed": "1.2 GB",
    "storage_savings": "456.7 MB",
    "average_compression_ratio": 2.85,
    "compression_efficiency": "38.1%",
    "active_alerts": 0,
    "system_health": "Excellent"
  },
  "algorithm_stats": {
    "lz4": {
      "compression_count": 1250,
      "average_ratio": 2.1,
      "throughput_mbps": 145.2,
      "recommended_use_case": "Real-time data, high-frequency operations"
    }
  }
}
```

## Algorithms

### LZ4
- **Speed**: Very fast compression and decompression
- **Ratio**: Moderate compression ratio (typically 2-3:1)
- **Use Case**: Real-time applications, high-frequency operations
- **CPU Impact**: Low

### Snappy
- **Speed**: Fast compression and decompression
- **Ratio**: Good compression ratio (typically 2-4:1)
- **Use Case**: Network compression, streaming data
- **CPU Impact**: Low to moderate

### ZSTD
- **Speed**: Slower compression, fast decompression
- **Ratio**: High compression ratio (typically 3-5:1)
- **Use Case**: Cold data, archival storage, batch processing
- **CPU Impact**: Moderate to high

## Best Practices

### 1. Algorithm Selection
- Use **LZ4** for frequently accessed data requiring fast access
- Use **Snappy** for network transmission and moderate compression needs
- Use **ZSTD** for cold data and when storage space is critical

### 2. Threshold Configuration
- Set `CompressionThreshold` based on your data patterns (typically 1KB-4KB)
- Configure `ColdThreshold` based on your access patterns (typically 1-7 days)
- Adjust `BloomFilterSize` based on your dataset size

### 3. Monitoring
- Monitor compression ratios to ensure effectiveness
- Watch CPU overhead to prevent performance impact
- Set up alerts for low compression efficiency or high resource usage

### 4. Performance Tuning
- Enable background compression for better performance
- Adjust compression intervals based on your workload
- Use appropriate cache sizes for your memory constraints

## Examples

Run the included examples to see the compression system in action:

```go
// Run all examples
compression.RunAllExamples()

// Or run individual examples
compression.Example()                    // Full system example
compression.ExampleTransparentCompression() // Transparent compression
compression.ExampleColdDataDetection()      // Cold data detection
```

## Integration with MantisDB

The compression system integrates seamlessly with MantisDB's storage layer:

1. **Storage Interface**: Transparent compression at the storage level
2. **WAL Integration**: Compressed write-ahead log entries
3. **Cache Integration**: Compressed data in memory caches
4. **Backup Integration**: Compressed backup data
5. **Monitoring Integration**: Unified metrics with database monitoring

## Performance Characteristics

### Compression Ratios (Typical)
- Text data: 3-8:1
- JSON data: 4-10:1
- Binary data: 1.5-3:1
- Log files: 5-15:1

### Throughput (Typical)
- LZ4: 100-200 MB/s compression, 300-500 MB/s decompression
- Snappy: 80-150 MB/s compression, 200-400 MB/s decompression
- ZSTD: 20-80 MB/s compression, 200-400 MB/s decompression

### Memory Usage
- Bloom filter: ~1MB per million tracked items
- Compression buffers: ~64KB per active compression
- Metrics storage: ~10MB for 24 hours of data

## Troubleshooting

### Low Compression Ratios
- Check data types (already compressed data won't compress further)
- Verify compression thresholds are appropriate
- Consider different algorithms for different data types

### High CPU Usage
- Reduce compression frequency
- Switch to faster algorithms (LZ4 instead of ZSTD)
- Adjust background compression settings

### Memory Issues
- Reduce bloom filter size
- Decrease metrics retention period
- Adjust cache sizes

### Performance Impact
- Enable background compression
- Increase compression thresholds
- Monitor and tune based on metrics