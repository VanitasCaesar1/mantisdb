# MantisDB Components

This directory contains documentation for all major MantisDB components. Each component is designed to be modular, testable, and production-ready.

## Core Components

### [Concurrency System](concurrency.md)
Advanced locking and deadlock detection system with hierarchical locking, performance monitoring, and goroutine lifecycle management.

**Key Features:**
- Hierarchical locking to prevent deadlocks
- Fast path optimization for uncontended locks
- Advanced deadlock detection algorithms
- Comprehensive performance monitoring
- Goroutine lifecycle management

### [Monitoring & Observability](monitoring.md)
Comprehensive monitoring system with metrics collection, health checks, alerting, operational logging, and audit trails.

**Key Features:**
- Real-time metrics collection
- System health monitoring
- Multi-level alerting system
- Structured operational logging
- Cryptographic audit trails

### [Data Integrity](integrity.md)
Data integrity verification and corruption detection system with multiple checksum algorithms and real-time monitoring.

**Key Features:**
- Multiple checksum algorithms (CRC32, MD5, SHA256)
- Real-time corruption detection
- WAL integrity verification and repair
- Comprehensive monitoring and alerting
- Background scanning capabilities

### [Testing Framework](testing.md)
Comprehensive testing framework for edge cases and reliability testing under extreme conditions and failure scenarios.

**Key Features:**
- Edge case testing (large documents, high TTL, concurrent writes)
- Reliability testing (crash recovery, disk exhaustion, memory limits)
- Concurrent access pattern testing
- Automated test execution and reporting
- Configurable test parameters

### [Backup System](backup.md)
Hot backup system that creates consistent backups without downtime using copy-on-write semantics and WAL checkpoints.

**Key Features:**
- Consistent snapshot creation
- Copy-on-write semantics for zero downtime
- Multiple destination support (file, S3, GCS, Azure)
- Compression and integrity verification
- Automated scheduling and retention policies

### [Compression](compression.md)
Transparent data compression system with multiple algorithms, cold data detection, and comprehensive monitoring.

**Key Features:**
- Multiple compression algorithms (LZ4, Snappy, ZSTD)
- Cold data detection with bloom filters
- Transparent operation with configurable policies
- Real-time monitoring and performance tracking
- HTTP endpoints for metrics visualization

### [Metrics & Alerting](metrics.md)
Prometheus-compatible metrics system with health checks and multi-channel alerting capabilities.

**Key Features:**
- Prometheus-compatible metrics export
- System and database health checks
- Load balancer integration (readiness/liveness probes)
- Multi-channel alerting (email, Slack, webhooks)
- Configurable alert rules and suppression

### [RPO System](rpo.md)
Recovery Point Objective system providing configurable data loss protection and automated recovery capabilities.

**Key Features:**
- Configurable RPO levels (Zero to High)
- Multiple checkpoint types (Full, Incremental, Snapshot)
- Point-in-time recovery capabilities
- Automated compliance monitoring
- Comprehensive alerting and reporting

## Component Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    MantisDB Core                            │
├─────────────────────────────────────────────────────────────┤
│  SQL Engine          │  Storage Engines                     │
│  ├── Parser          │  ├── Key-Value                       │
│  ├── Optimizer       │  ├── Document                        │
│  └── Executor        │  └── Columnar                        │
├─────────────────────────────────────────────────────────────┤
│  System Components                                          │
│  ├── Concurrency     │  ├── Monitoring                      │
│  ├── Integrity       │  ├── Backup                          │
│  ├── Compression     │  ├── Metrics                         │
│  ├── Testing         │  └── RPO                             │
└─────────────────────────────────────────────────────────────┘
```

## Integration Points

All components are designed to integrate seamlessly with each other:

- **Monitoring** integrates with all components for metrics collection
- **Integrity** works with storage engines and WAL for data verification
- **Backup** uses WAL checkpoints and storage engines for consistent snapshots
- **Concurrency** provides locking services to all data access operations
- **Compression** integrates transparently with storage operations
- **Testing** validates all components under extreme conditions
- **RPO** coordinates with backup and WAL systems for data protection

## Development Guidelines

When working with these components:

1. **Modularity**: Each component should be independently testable
2. **Interfaces**: Use well-defined interfaces for component interaction
3. **Configuration**: Provide comprehensive configuration options
4. **Monitoring**: Include metrics and health checks in all components
5. **Testing**: Comprehensive test coverage including edge cases
6. **Documentation**: Maintain up-to-date documentation and examples
7. **Performance**: Consider performance impact and provide benchmarks

## Getting Started

To get started with any component:

1. Read the component-specific documentation
2. Review the configuration options
3. Check the usage examples
4. Run the included tests
5. Review the integration points with other components

Each component includes comprehensive examples and test suites to help you understand its capabilities and integration patterns.