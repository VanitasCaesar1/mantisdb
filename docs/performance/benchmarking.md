# MantisDB Production Benchmark & Stress Testing Guide

This guide covers the comprehensive benchmark and stress testing system for MantisDB, designed to provide production-ready performance evaluation with proper scoring metrics.

## Overview

The MantisDB benchmark system provides:

- **Production-Ready Stress Testing**: Configurable stress levels from light to extreme
- **Comprehensive Scoring**: 0-100 point scoring system with letter grades (A+ to F)
- **Performance Categories**: Sequential, Random Access, Concurrency, Throughput, Memory Management
- **Detailed Metrics**: Latency percentiles (P50, P95, P99, P999), error rates, memory usage
- **Intelligent Recommendations**: Automated performance tuning suggestions
- **Continuous Testing**: Long-running stress tests for endurance evaluation

## Quick Start

### Basic Benchmark (Integrated)

Run benchmarks as part of MantisDB startup:

```bash
# Run benchmarks after startup
./mantisdb --benchmark

# Run benchmarks only and exit
./mantisdb --benchmark-only

# Use heavy stress level for benchmark-only mode
./mantisdb --benchmark-only --data-dir=/tmp/mantis_bench
```

### Standalone Stress Testing

Use the dedicated stress testing tool:

```bash
# Build the stress testing tool
go build -o stress-benchmark ./cmd/stress-benchmark

# Run medium stress test (default)
./stress-benchmark

# Run heavy stress test with CGO engine
./stress-benchmark -stress=heavy -use-cgo

# Run extreme stress test with custom settings
./stress-benchmark -stress=extreme -duration=10m -workers=16 -ops=10000

# Continuous stress testing
./stress-benchmark -continuous -interval=5m -stress=medium
```

## Stress Levels

### Light Stress
- **Duration**: 30 seconds
- **Workers**: CPU cores
- **Operations/sec**: 500
- **Data size**: 512 bytes
- **Use case**: Quick validation, CI/CD pipelines

### Medium Stress (Default)
- **Duration**: 60 seconds  
- **Workers**: 2x CPU cores
- **Operations/sec**: 1,000
- **Data size**: 1KB
- **Use case**: Regular performance testing, development

### Heavy Stress
- **Duration**: 120 seconds
- **Workers**: 4x CPU cores
- **Operations/sec**: 2,000
- **Data size**: 2KB
- **Use case**: Pre-production validation, load testing

### Extreme Stress
- **Duration**: 300 seconds (5 minutes)
- **Workers**: 8x CPU cores
- **Operations/sec**: 5,000
- **Data size**: 4KB
- **Use case**: Maximum load testing, capacity planning

## Benchmark Categories & Scoring

### Performance Categories

1. **Sequential Performance** (40 points max)
   - Sequential reads and writes
   - Measures raw I/O throughput
   - Key for bulk operations

2. **Random Access** (30 points max)
   - Random reads and writes
   - Measures cache efficiency
   - Key for OLTP workloads

3. **Concurrency** (20 points max)
   - Multi-threaded operations
   - Measures lock contention
   - Key for high-concurrency scenarios

4. **Throughput** (30 points max)
   - High-volume operations
   - Measures system limits
   - Key for high-load scenarios

5. **Memory Management** (20 points max)
   - Memory pressure tests
   - Measures GC efficiency
   - Key for long-running processes

### Scoring Algorithm

Each benchmark receives a score based on:

- **Operations/sec** (0-40 points): Normalized to 1000 ops/sec baseline
- **Latency** (0-30 points): Lower average latency = higher score
- **Error Rate** (0-20 points): Lower error rate = higher score  
- **Consistency** (0-10 points): Lower P99/P50 ratio = higher score

### Grade Scale

- **A+ (90-100)**: Exceptional performance, production-ready
- **A (85-89)**: Excellent performance, production-ready
- **A- (80-84)**: Very good performance, production-ready
- **B+ (75-79)**: Good performance, minor tuning recommended
- **B (70-74)**: Acceptable performance, some tuning needed
- **B- (65-69)**: Below average, tuning required
- **C+ (60-64)**: Poor performance, significant tuning needed
- **C (55-59)**: Very poor performance, major optimization required
- **C- (50-54)**: Unacceptable performance, system redesign needed
- **D (40-49)**: Critical performance issues
- **F (0-39)**: System failure or severe issues

## Detailed Metrics

### Latency Metrics
- **Average Latency**: Mean response time
- **P50 (Median)**: 50th percentile latency
- **P95**: 95th percentile latency  
- **P99**: 99th percentile latency
- **P999**: 99.9th percentile latency

### Throughput Metrics
- **Operations/sec**: Raw throughput
- **Data processed (MB)**: Total data volume
- **Memory efficiency**: Data/memory ratio

### Error Metrics
- **Error count**: Total failed operations
- **Error rate (%)**: Percentage of failed operations

### System Metrics
- **Memory usage (MB)**: Peak memory consumption
- **CPU usage (%)**: Average CPU utilization
- **Cache hit rate (%)**: Cache effectiveness

## Performance Recommendations

The system automatically generates recommendations based on results:

### Sequential Performance Issues
- Increase buffer sizes
- Use SSD storage
- Optimize write batching
- Consider async I/O

### Random Access Issues  
- Increase cache size
- Add more RAM
- Optimize data structures
- Consider bloom filters

### Concurrency Issues
- Reduce lock contention
- Optimize critical sections
- Consider lock-free algorithms
- Tune worker pool sizes

### Throughput Issues
- Optimize batch sizes
- Increase parallelism
- Tune buffer sizes
- Consider compression

### Memory Issues
- Tune GC settings
- Optimize data structures
- Reduce allocations
- Consider memory pools

## Advanced Usage

### Custom Configuration

```bash
# Override specific parameters
./stress-benchmark \
  -stress=heavy \
  -duration=5m \
  -workers=32 \
  -ops=15000 \
  -data-dir=/fast/ssd/path \
  -output=custom_results.json
```

### Continuous Testing

```bash
# Run continuous tests every 10 minutes
./stress-benchmark \
  -continuous \
  -interval=10m \
  -stress=medium \
  -output=continuous_results.json
```

### Comparing Engines

```bash
# Test Pure Go engine
./stress-benchmark -stress=heavy -output=pure_go_results.json

# Test CGO engine  
./stress-benchmark -stress=heavy -use-cgo -output=cgo_results.json
```

## Result Analysis

### JSON Output Structure

```json
{
  "overall_score": 85.5,
  "grade": "A",
  "category_scores": {
    "Sequential Performance": 88.2,
    "Random Access": 82.1,
    "Concurrency": 87.5,
    "Throughput": 84.8,
    "Memory Management": 85.0
  },
  "recommendations": [
    "Performance is good! System is well-tuned for current workload."
  ],
  "system_info": {
    "os": "darwin",
    "architecture": "amd64", 
    "cpus": 8,
    "memory_mb": 16384,
    "go_version": "go1.21.0"
  },
  "test_environment": {
    "stress_level": "heavy",
    "duration": "2m0s",
    "workers": 32,
    "total_operations": 240000,
    "data_processed_mb": 468.75
  },
  "results": [
    {
      "name": "KV Sequential Writes",
      "operations": 2000,
      "duration": "1.234s",
      "ops_per_second": 1620.5,
      "avg_latency": "617µs",
      "p50_latency": "580µs", 
      "p95_latency": "1.2ms",
      "p99_latency": "2.1ms",
      "p999_latency": "4.5ms",
      "error_count": 0,
      "error_rate": 0.0,
      "memory_usage_mb": 45.2,
      "score": 87.5,
      "grade": "A",
      "timestamp": "2024-01-15T10:30:00Z"
    }
  ]
}
```

### Performance Baselines

#### Excellent Performance (A+ Grade)
- **Sequential Writes**: >2000 ops/sec, <1ms P99
- **Sequential Reads**: >5000 ops/sec, <500µs P99  
- **Random Writes**: >1000 ops/sec, <2ms P99
- **Random Reads**: >3000 ops/sec, <1ms P99
- **Concurrent Ops**: >1500 ops/sec, <1.5ms P99
- **Error Rate**: <0.1%

#### Good Performance (B+ Grade)
- **Sequential Writes**: >1000 ops/sec, <2ms P99
- **Sequential Reads**: >2500 ops/sec, <1ms P99
- **Random Writes**: >500 ops/sec, <5ms P99  
- **Random Reads**: >1500 ops/sec, <2ms P99
- **Concurrent Ops**: >750 ops/sec, <3ms P99
- **Error Rate**: <1%

## Troubleshooting

### Common Issues

#### Low Sequential Performance
- Check disk I/O capacity
- Verify buffer sizes
- Consider SSD upgrade
- Check for disk contention

#### Poor Random Access
- Increase cache size
- Add more RAM
- Optimize data layout
- Consider indexing strategies

#### High Error Rates
- Check system resources
- Verify configuration
- Look for timeout issues
- Check disk space

#### Memory Issues
- Tune GC settings (GOGC environment variable)
- Increase available RAM
- Optimize data structures
- Check for memory leaks

### Performance Tuning

#### Environment Variables
```bash
# Tune garbage collector
export GOGC=200

# Increase Go max processes
export GOMAXPROCS=16

# Set memory limit
export GOMEMLIMIT=8GiB
```

#### System Tuning
```bash
# Increase file descriptor limits
ulimit -n 65536

# Tune kernel parameters (Linux)
echo 'vm.swappiness=1' >> /etc/sysctl.conf
echo 'vm.dirty_ratio=15' >> /etc/sysctl.conf
```

## Integration with CI/CD

### GitHub Actions Example

```yaml
name: Performance Benchmarks
on: [push, pull_request]

jobs:
  benchmark:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    - uses: actions/setup-go@v3
      with:
        go-version: '1.21'
    
    - name: Build MantisDB
      run: go build -o mantisdb ./cmd/mantisDB
    
    - name: Run Light Benchmarks
      run: |
        go build -o stress-benchmark ./cmd/stress-benchmark
        ./stress-benchmark -stress=light -output=ci_results.json
    
    - name: Check Performance Threshold
      run: |
        # Fail if overall score < 70
        score=$(jq '.overall_score' ci_results.json)
        if (( $(echo "$score < 70" | bc -l) )); then
          echo "Performance regression detected: $score < 70"
          exit 1
        fi
```

## Best Practices

### Development Testing
- Use **light** stress level for quick validation
- Run benchmarks on representative hardware
- Test both Pure Go and CGO engines
- Monitor trends over time

### Pre-Production Testing  
- Use **heavy** or **extreme** stress levels
- Test on production-like hardware
- Run continuous tests for stability
- Validate under various load patterns

### Production Monitoring
- Set up continuous benchmarking
- Monitor performance trends
- Set alerts for score degradation
- Regular capacity planning tests

### Performance Goals
- Target **B+ (75+)** minimum for production
- Aim for **A- (80+)** for high-performance scenarios
- Investigate any score below **70**
- Celebrate **A+ (90+)** achievements!

## Conclusion

The MantisDB benchmark system provides comprehensive, production-ready performance evaluation with actionable insights. Use it regularly to ensure optimal performance and catch regressions early in the development cycle.

For questions or issues, please refer to the main MantisDB documentation or open an issue in the repository.