# MantisDB Documentation Index

## 📚 Quick Navigation

### Getting Started
- **[Quick Start](setup/QUICK_START.md)** - Get up and running in 5 minutes
- **[Installation Guide](setup/INSTALL.md)** - Detailed installation instructions
- **[Build Guide](setup/BUILD.md)** - Building from source

### Rust High-Performance Core
- **[Quick Start with Rust](rust/QUICK_START_RUST.md)** - 50K+ ops/sec in 5 minutes
- **[Rust Performance Guide](rust/RUST_PERFORMANCE.md)** - Comprehensive performance documentation
- **[Upgrade to Rust Core](rust/README_RUST_UPGRADE.md)** - Migration guide
- **[Performance Optimization Summary](rust/PERFORMANCE_OPTIMIZATION_SUMMARY.md)** - Technical deep dive
- **[Rust Core Summary](rust/RUST_CORE_SUMMARY.txt)** - Quick reference

### Architecture & Components
- **[Architecture Overview](architecture/sql-engine.md)** - System architecture
- **[Performance Architecture](performance/rust-core-architecture.md)** - Rust core internals
- **[Components Overview](components/README.md)** - All system components

### Setup & Configuration
- **[Port Management](setup/PORT_MANAGEMENT.md)** - Port configuration
- **[WASM Admin](setup/WASM_ADMIN.md)** - WebAssembly admin dashboard

### Client Libraries
- **[Client Overview](clients/overview.md)** - All available clients
- Go, Python, JavaScript/TypeScript clients

### Project Summaries
- **[Implementation Complete](summaries/IMPLEMENTATION_COMPLETE.md)** - Rust core implementation
- **[Admin Dashboard Status](summaries/ADMIN_DASHBOARD_STATUS.md)** - Dashboard features
- **[App Fix Summary](summaries/APP_FIX_SUMMARY.md)** - Bug fixes
- **[Installer Summary](summaries/INSTALLER_SUMMARY.md)** - Installation system
- **[SDK Publishing](summaries/SDK_PUBLISHING_SUMMARY.md)** - Client library publishing
- **[Startup Fix](summaries/STARTUP_FIX_SUMMARY.md)** - Startup improvements

## 🎯 By Use Case

### I want to...

**Get started quickly**
→ [Quick Start](setup/QUICK_START.md)

**Achieve maximum performance (50K+ ops/sec)**
→ [Rust Quick Start](rust/QUICK_START_RUST.md)

**Understand the architecture**
→ [Architecture Overview](architecture/sql-engine.md)
→ [Rust Core Architecture](performance/rust-core-architecture.md)

**Build from source**
→ [Build Guide](setup/BUILD.md)

**Deploy to production**
→ [Installation Guide](setup/INSTALL.md)
→ [Rust Upgrade Guide](rust/README_RUST_UPGRADE.md)

**Integrate with my application**
→ [Client Overview](clients/overview.md)

**Troubleshoot issues**
→ [Port Management](setup/PORT_MANAGEMENT.md)
→ [Summaries](summaries/)

## 📊 Performance Documentation

### Benchmarks & Optimization
- [Rust Performance Guide](rust/RUST_PERFORMANCE.md) - Comprehensive benchmarks
- [Performance Optimization](rust/PERFORMANCE_OPTIMIZATION_SUMMARY.md) - Technical details
- [Rust Core Architecture](performance/rust-core-architecture.md) - Lock-free algorithms

### Performance Comparison

| Metric | Pure Go | Rust Core | Guide |
|--------|---------|-----------|-------|
| Throughput | 11K ops/s | 50K+ ops/s | [Rust Performance](rust/RUST_PERFORMANCE.md) |
| P99 Latency | 8.5s | <100ms | [Optimization Summary](rust/PERFORMANCE_OPTIMIZATION_SUMMARY.md) |
| Lock Contention | High | None | [Architecture](performance/rust-core-architecture.md) |

## 🔧 Component Documentation

### Core Components
- [Admin Dashboard](components/admin.md)
- [Advanced Features](components/advanced.md)
- [API Server](components/api.md)
- [Benchmarking](components/benchmark.md)
- [Cache System](components/cache.md)
- [Checkpointing](components/checkpoint.md)
- [Configuration](components/config.md)
- [Durability](components/durability.md)
- [Health Checks](components/health.md)
- [Query Engine](components/query.md)
- [Storage Engine](components/storage.md)

## 📖 Additional Resources

### External Links
- GitHub Repository
- Issue Tracker
- Discussions
- Discord Community

### Contributing
- See individual component READMEs
- Check [rust-core/README.md](../rust-core/README.md) for Rust contributions

## 🆘 Getting Help

1. **Check documentation** - Start with relevant guide above
2. **Search issues** - Someone may have had the same problem
3. **Ask in discussions** - Community support
4. **Join Discord** - Real-time help

---

**Last Updated**: 2025-10-07
**Version**: 0.2.0 (Rust Core)
