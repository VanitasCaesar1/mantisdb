# Data Compression System

MantisDB includes a sophisticated data compression system optimized for different workloads and storage patterns.

## Overview

The compression system provides:
- **Multiple Algorithms**: LZ4, Snappy, ZSTD, and custom algorithms
- **Transparent Compression**: Automatic compression/decompression
- **Cold Data Optimization**: Specialized compression for infrequently accessed data
- **Performance Monitoring**: Compression ratio and performance metrics
- **Adaptive Selection**: Algorithm selection based on data characteristics

## Supported Algorithms

| Algorithm | Compression Ratio | Speed | Use Case |
|-----------|------------------|-------|----------|
| LZ4 | 2.5:1 | Very Fast | Hot data, OLTP workloads |
| Snappy | 3.2:1 | Fast | General purpose |
| ZSTD | 4.8:1 | Medium | Cold data, archival |
| Custom | 5.2:1 | Medium | MantisDB-optimized |

## Configuration

```yaml
compression:
  enabled: true
  default_algorithm: "lz4"
  cold_data_algorithm: "zstd"
  cold_data_threshold: "7d"
  compression_level: 3
  min_compression_ratio: 1.1
  enable_adaptive: true
  monitor_performance: true
```

## Usage Examples

### Basic Compression
```go
engine := compression.NewEngine(compression.Config{
    Algorithm: compression.LZ4,
    Level:     3,
})

// Compress data
compressed, err := engine.Compress(data)
if err != nil {
    return err
}

// Decompress data
original, err := engine.Decompress(compressed)
if err != nil {
    return err
}
```

### Transparent Compression
```go
// Enable transparent compression for a table
err := db.SetTableCompression("users", compression.Config{
    Algorithm: compression.Snappy,
    Threshold: 1024, // Compress data > 1KB
})
```

### Cold Data Compression
```go
// Configure cold data compression
coldConfig := compression.ColdDataConfig{
    Algorithm:        compression.ZSTD,
    Level:           9,
    AgeThreshold:    7 * 24 * time.Hour,
    AccessThreshold: 30 * 24 * time.Hour,
}

err := db.EnableColdDataCompression(coldConfig)
```

## Performance Characteristics

### Compression Ratios by Data Type

| Data Type | LZ4 | Snappy | ZSTD | Custom |
|-----------|-----|--------|------|--------|
| JSON Documents | 2.8:1 | 3.5:1 | 5.1:1 | 5.8:1 |
| Log Data | 3.2:1 | 4.1:1 | 6.2:1 | 6.8:1 |
| Time Series | 4.5:1 | 5.2:1 | 7.8:1 | 8.1:1 |
| Binary Data | 1.8:1 | 2.1:1 | 2.8:1 | 3.1:1 |

### Performance Metrics

| Algorithm | Compression (MB/s) | Decompression (MB/s) | CPU Usage |
|-----------|-------------------|---------------------|-----------|
| LZ4 | 850 | 2,100 | Low |
| Snappy | 650 | 1,800 | Low |
| ZSTD | 280 | 950 | Medium |
| Custom | 320 | 1,200 | Medium |

## Integration

The compression system integrates with all MantisDB storage engines:

### Key-Value Storage
- Automatic value compression above threshold
- Key compression for long keys
- Batch compression for bulk operations

### Document Storage
- Document-level compression
- Field-level compression for large fields
- Index compression for better performance

### Columnar Storage
- Column-wise compression
- Dictionary encoding
- Run-length encoding
- Delta compression for time series

## Monitoring

### Metrics
- Compression ratios by algorithm and table
- Compression/decompression throughput
- CPU usage and memory consumption
- Storage space savings

### Alerts
- Low compression ratio warnings
- Performance degradation alerts
- Storage space threshold alerts
- Algorithm failure notifications

## Best Practices

1. **Algorithm Selection**:
   - Use LZ4 for hot data requiring fast access
   - Use ZSTD for cold data prioritizing space savings
   - Use Snappy for balanced performance/compression

2. **Threshold Configuration**:
   - Set minimum size thresholds to avoid overhead
   - Configure cold data thresholds based on access patterns
   - Monitor compression ratios and adjust accordingly

3. **Performance Optimization**:
   - Enable adaptive algorithm selection
   - Monitor CPU usage and adjust compression levels
   - Use batch operations for better throughput

4. **Storage Management**:
   - Implement proper retention policies
   - Monitor storage space savings
   - Plan for decompression overhead during queries