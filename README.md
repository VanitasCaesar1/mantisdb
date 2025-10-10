# MantisDB

High-performance multi-model database with **Rust-powered admin backend** and **PostgreSQL-compatible Row Level Security**.

**Performance**: 100K+ req/s | Sub-millisecond latency | Zero-copy I/O ⚡

## ✨ Features

### Multi-Model Database
- **Key-Value Store**: Redis-like with TTL, batch operations, and prefix search
- **Document Store**: MongoDB-style with aggregation pipelines and flexible schema
- **Columnar Store**: Cassandra/ScyllaDB-style with CQL support and partitioning
- **SQL Store**: Traditional relational with advanced query features

### Performance & Scalability
- **Rust-Powered Core**: 100K+ req/s throughput, sub-millisecond latency
- **Zero-Copy I/O**: Optimized memory management with mimalloc
- **Lock-Free Operations**: Concurrent access without bottlenecks
- **Connection Pooling**: Efficient resource management

### Enterprise Features
- **Row Level Security (RLS)**: PostgreSQL-compatible policy engine (<10μs checks)
- **ACID Transactions**: Full transaction support across all models
- **Backup & Recovery**: Automated backup with point-in-time recovery
- **Monitoring & Observability**: Prometheus metrics, real-time dashboards
- **Authentication & Authorization**: JWT-based with role management

### Developer Experience
- **Professional Admin UI**: React-based with model-specific interfaces
- **Enhanced SQL Editor**: Autocomplete, query history, explain plans
- **RESTful API**: 60+ endpoints with OpenAPI/Swagger documentation
- **Client Libraries**: Go, Python, JavaScript/TypeScript
- **Hot Reload**: Development mode with instant updates

## 🚀 Quick Start

### Prerequisites
- **Rust 1.75+** (admin backend)
- **Node.js 18+** (frontend dashboard)
- **Go 1.20+** (database core)

### Build & Run

```bash
# Complete production build (Rust + Go + Admin UI)
./scripts/build-all.sh

# Or use unified build system
./build-unified.sh release

# Or use Make
make build
make run

# Production deployment
./start-production.sh
```

### Quick Install (Binary)

```bash
# Download latest release
wget https://github.com/mantisdb/mantisdb/releases/latest/download/mantisdb-linux-amd64.tar.gz
tar -xzf mantisdb-linux-amd64.tar.gz
cd mantisdb-linux-amd64
./mantisdb --version
```

### Access Points

- **Admin Dashboard**: http://localhost:5173
- **Admin API**: http://localhost:8081/api/docs (OpenAPI docs)
- **Database API**: http://localhost:8080
- **Default Login**: admin@mantisdb.io / admin123

## 🐳 Docker

```bash
# Quick start
docker-compose up -d

# Or build manually
docker build -t mantisdb .
docker run -p 8080:8080 -p 8081:8081 mantisdb
```

## 📚 Documentation

Comprehensive documentation available in [`docs/`](docs/):

- **[Multi-Model Features](MULTI_MODEL_FEATURES.md)** - ⭐ NEW: Complete guide to all data models
- **[Getting Started](docs/getting-started/)** - Installation and setup
- **[Production Release Guide](PRODUCTION_RELEASE.md)** - Complete production deployment guide
- **[Deployment Guide](DEPLOYMENT_GUIDE.md)** - Detailed deployment strategies
- **[Release Checklist](RELEASE_CHECKLIST.md)** - Pre-release verification
- **[Architecture](docs/architecture/)** - System design and components  
- **[API Reference](docs/api/)** - REST API documentation
- **[Client Libraries](docs/clients/)** - Language-specific guides
- **[Admin Dashboard](docs/components/admin.md)** - UI features and usage
- **[Row Level Security](docs/components/rls.md)** - RLS implementation
- **[Performance](docs/performance/)** - Benchmarks and tuning

### API Documentation

Interactive API documentation with Swagger UI:
- **OpenAPI Spec**: http://localhost:8081/api/docs/openapi.yaml
- **Swagger UI**: http://localhost:8081/api/docs

## 🛠️ Development

```bash
# Build
make build              # Full build with Rust + Go
make build-rust         # Rust components only

# Run
make run                # Start with admin dashboard
make run-api            # Standalone API server

# Test
make test               # All tests
make bench              # Benchmarks

# Clean
make clean              # Remove build artifacts
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Add tests for your changes
4. Run tests: `make test`
5. Submit a pull request

See [docs/development/](docs/development/) for detailed guidelines.

## 🆘 Support

- **Documentation**: [docs/](docs/)
- **API Docs**: http://localhost:8081/api/docs (Swagger UI)
- **Issues**: [GitHub Issues](https://github.com/mantisdb/mantisdb/issues)
- **Discussions**: [GitHub Discussions](https://github.com/mantisdb/mantisdb/discussions)

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

---

**Ready to get started?** Run `./build-unified.sh release` and see the [Quick Start Guide](docs/getting-started/quickstart.md).