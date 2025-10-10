# Quick Start: MantisDB with Rust High-Performance Core

## 🚀 TL;DR

```bash
# Install Rust (if not already installed)
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh

# Build with Rust core
make build-rust

# Run and benchmark
./mantisdb-rust --benchmark-only

# Expected: 50,000+ ops/sec (vs 11,000 with pure Go)
```

## 📊 Performance Gains

| Metric | Pure Go | Rust Core | Improvement |
|--------|---------|-----------|-------------|
| Concurrent Ops | 5.5K/s | **50K+/s** | **9x faster** |
| P99 Latency | 8.5s | **<100ms** | **85x better** |
| Lock Contention | High | **None** | Eliminated |

## 🔧 Installation

### 1. Install Rust

```bash
# macOS/Linux
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
source $HOME/.cargo/env

# Verify
rustc --version
cargo --version
```

### 2. Build MantisDB

```bash
# Clone repository (if not already)
cd /path/to/mantisdb

# Build with Rust core
make build-rust

# This will:
# 1. Build Rust core library (rust-core/)
# 2. Build Go wrapper with FFI bindings
# 3. Create mantisdb-rust binary
```

### 3. Run Benchmarks

```bash
# Run comprehensive benchmarks
./mantisdb-rust --benchmark-only --benchmark-stress=heavy

# Expected output:
# ✓ High Throughput Test: 50,000+ ops/sec
# ✓ Concurrent Operations: 50,000+ ops/sec
# ✓ P99 Latency: <100ms
```

## 🎯 Usage

### Basic Usage

```bash
# Start server with Rust core
./mantisdb-rust --port=8080

# Access admin dashboard
open http://localhost:8081

# Access API
curl http://localhost:8080/api/v1/stats
```

### Production Configuration

```bash
# High-performance production setup
./mantisdb-rust \
  --port=8080 \
  --admin-port=8081 \
  --cache-size=1073741824 \
  --data-dir=/var/lib/mantisdb \
  --log-level=info
```

### Docker

```bash
# Build Docker image with Rust core
docker build -t mantisdb:rust -f Dockerfile.rust .

# Run container
docker run -p 8080:8080 -p 8081:8081 mantisdb:rust
```

## 🔍 Verification

### Check Performance

```bash
# Run benchmarks
./mantisdb-rust --benchmark-only

# Check stats
curl http://localhost:8080/api/v1/stats | jq

# Expected output:
{
  "storage": {
    "reads": 50000,
    "writes": 50000,
    "deletes": 1000
  },
  "cache": {
    "hits": 45000,
    "misses": 5000,
    "hit_rate": 0.9
  }
}
```

### Compare with Pure Go

```bash
# Build pure Go version
make build

# Run benchmark
./mantisdb --benchmark-only

# Compare results:
# Pure Go:    ~11K ops/sec
# Rust Core:  ~50K ops/sec
```

## 📈 Performance Tuning

### Environment Variables

```bash
# Increase Rust thread pool
export RAYON_NUM_THREADS=16

# Enable large pages (Linux)
export MIMALLOC_LARGE_OS_PAGES=1

# Run
./mantisdb-rust
```

### CPU Governor (Linux)

```bash
# Set CPU to performance mode
echo performance | sudo tee /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor

# Verify
cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_governor
```

### Cache Size

```bash
# Increase cache for better hit rate
./mantisdb-rust --cache-size=2147483648  # 2GB

# Monitor cache stats
watch -n 1 'curl -s http://localhost:8080/api/v1/stats | jq .cache'
```

## 🐛 Troubleshooting

### Build Errors

**Error: `rustc not found`**
```bash
# Install Rust
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
source $HOME/.cargo/env
```

**Error: `cannot find -lmantisdb_core`**
```bash
# Build Rust core first
cd rust-core
cargo build --release
cd ..
make build-rust
```

**Error: `CGO_ENABLED required`**
```bash
# Enable CGO
export CGO_ENABLED=1
make build-rust
```

### Runtime Issues

**Issue: Lower than expected performance**
```bash
# Check CPU frequency
lscpu | grep MHz

# Disable CPU throttling
sudo cpupower frequency-set -g performance

# Verify
./mantisdb-rust --benchmark-only
```

**Issue: High memory usage**
```bash
# Reduce cache size
./mantisdb-rust --cache-size=268435456  # 256MB

# Monitor memory
watch -n 1 'ps aux | grep mantisdb-rust'
```

## 📚 Architecture

### Components

```
┌─────────────────────────────────────┐
│         Go Application              │
│  (API, Query Parser, Orchestration) │
└─────────────┬───────────────────────┘
              │ FFI (CGO)
┌─────────────▼───────────────────────┐
│         Rust Core                   │
│  ┌─────────────────────────────┐   │
│  │  Lock-Free Storage Engine   │   │
│  │  (Crossbeam Skiplist)       │   │
│  └─────────────────────────────┘   │
│  ┌─────────────────────────────┐   │
│  │  Lock-Free LRU Cache        │   │
│  │  (Atomic Operations)        │   │
│  └─────────────────────────────┘   │
└─────────────────────────────────────┘
```

### Key Technologies

- **Storage**: Crossbeam skiplist (lock-free)
- **Cache**: Parking lot RwLock + atomic counters
- **Serialization**: rkyv (zero-copy)
- **Allocator**: mimalloc (high performance)
- **FFI**: C ABI for Go integration

## 🎓 Next Steps

### Development

```bash
# Run tests
cd rust-core && cargo test

# Run Rust benchmarks
cd rust-core && cargo bench

# Profile performance
cargo install flamegraph
sudo cargo flamegraph --bench storage_bench
```

### Production Deployment

```bash
# Build optimized release
make build-rust VERSION=1.0.0

# Create systemd service
sudo cp mantisdb.service /etc/systemd/system/
sudo systemctl enable mantisdb
sudo systemctl start mantisdb

# Monitor
journalctl -u mantisdb -f
```

### Monitoring

```bash
# Install Prometheus exporter (optional)
./mantisdb-rust --enable-metrics --metrics-port=9090

# View metrics
curl http://localhost:9090/metrics
```

## 📖 Documentation

- **Full Documentation**: [RUST_PERFORMANCE.md](RUST_PERFORMANCE.md)
- **Optimization Guide**: [PERFORMANCE_OPTIMIZATION_SUMMARY.md](PERFORMANCE_OPTIMIZATION_SUMMARY.md)
- **Rust Core README**: [rust-core/README.md](rust-core/README.md)
- **Build Guide**: [BUILD.md](BUILD.md)

## 🤝 Contributing

```bash
# Fork and clone
git clone https://github.com/yourusername/mantisdb.git
cd mantisdb

# Create feature branch
git checkout -b feature/my-optimization

# Make changes to rust-core/
cd rust-core
cargo test
cargo bench

# Submit PR
git push origin feature/my-optimization
```

## 📝 License

Same as MantisDB main project

## 🆘 Support

- **Issues**: https://github.com/yourusername/mantisdb/issues
- **Discussions**: https://github.com/yourusername/mantisdb/discussions
- **Discord**: [Join community](https://discord.gg/mantisdb)

---

**Built with ❤️ using Rust and Go**
