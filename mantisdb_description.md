# MantisDB: A Comprehensive Overview

MantisDB is a high-performance, multi-model database designed for modern applications. It combines the power of a Rust-powered admin backend with PostgreSQL-compatible Row Level Security to deliver exceptional speed, scalability, and security. With a throughput of over 100,000 requests per second, sub-millisecond latency, and zero-copy I/O, MantisDB is built to handle the most demanding workloads.

## Key Features

### Multi-Model Database

MantisDB offers a versatile multi-model architecture, allowing you to use the right data model for the right job, all within a single database:

- **Key-Value Store**: A Redis-like store with support for Time-to-Live (TTL), batch operations, and prefix searching.
- **Document Store**: A MongoDB-style store with flexible schemas and powerful aggregation pipelines.
- **Columnar Store**: A Cassandra/ScyllaDB-style store with CQL support and efficient data partitioning.
- **SQL Store**: A traditional relational store with advanced query features and ACID compliance.

### Performance and Scalability

Performance is at the core of MantisDB's design:

- **Rust-Powered Core**: The admin backend is built in Rust, ensuring memory safety, concurrency, and high performance.
- **Zero-Copy I/O**: Optimized memory management with `mimalloc` minimizes data copying and reduces latency.
- **Lock-Free Operations**: Concurrent data access is handled without bottlenecks, ensuring smooth performance under heavy load.
- **Connection Pooling**: Efficiently manages database connections to conserve resources and improve response times.

### Enterprise-Grade Features

MantisDB is packed with features that make it ready for enterprise environments:

- **Row Level Security (RLS)**: A PostgreSQL-compatible policy engine that performs security checks in under 10 microseconds.
- **ACID Transactions**: Full support for ACID transactions across all data models, ensuring data integrity.
- **Backup & Recovery**: Automated backups with point-in-time recovery to protect against data loss.
- **Monitoring & Observability**: Integration with Prometheus for metrics and real-time dashboards for monitoring.
- **Authentication & Authorization**: JWT-based authentication with role management for secure access control.

### Developer Experience

MantisDB is designed to be developer-friendly:

- **Professional Admin UI**: A React-based admin dashboard with model-specific interfaces for easy management.
- **Enhanced SQL Editor**: An advanced SQL editor with features like autocomplete, query history, and explain plans.
- **RESTful API**: Over 60 endpoints with comprehensive OpenAPI/Swagger documentation.
- **Client Libraries**: Official client libraries for Go, Python, and JavaScript/TypeScript.
- **Hot Reload**: A development mode with instant updates for a faster development cycle.

## Getting Started

To get started with MantisDB, you'll need the following prerequisites:

- **Rust 1.75+**
- **Node.js 18+**
- **Go 1.20+**

You can build and run MantisDB using the following commands:

```bash
# Unified build system
./build-unified.sh release

# Or use Make
make build
make run
```

## Documentation and Support

- **Comprehensive Docs**: [docs/](docs/)
- **API Reference**: [http://localhost:8081/api/docs](http://localhost:8081/api/docs)
- **GitHub Issues**: [https://github.com/mantisdb/mantisdb/issues](https://github.com/mantisdb/mantisdb/issues)
- **GitHub Discussions**: [https://github.com/mantisdb/mantisdb/discussions](https://github.com/mantisdb/mantisdb/discussions)
