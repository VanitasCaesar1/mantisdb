# MantisDB

A high-performance, production-ready multi-model database system that combines Key-Value, Document, and Columnar storage in a single, unified platform.

## 🚀 Features

- **Multi-Model Support**: Key-Value, Document, and Columnar data models in one database
- **High Performance**: Optimized storage engines with both CGO and Pure Go implementations
- **ACID Transactions**: Full transaction support across all data models
- **Admin Dashboard**: Professional web-based administration interface
- **RESTful API**: Complete HTTP API for all operations
- **Client Libraries**: Official clients for Go, Python, and JavaScript/TypeScript
- **Production Ready**: Docker support, systemd integration, monitoring, and logging
- **Cross-Platform**: Native binaries for Linux, macOS, and Windows
- **Enterprise Features**: Backup/restore, monitoring, health checks, and security

## 📦 Installation

### Quick Install (Recommended)

**Linux/macOS:**
```bash
curl -L https://github.com/mantisdb/mantisdb/releases/latest/download/mantisdb-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m).tar.gz | tar xz
cd mantisdb-*
sudo ./install.sh
```

**Windows:**
```powershell
# Download from releases page and run install.ps1 as Administrator
```

**Docker:**
```bash
docker run -d \
  --name mantisdb \
  -p 8080:8080 \
  -p 8081:8081 \
  -v mantisdb_data:/var/lib/mantisdb \
  mantisdb/mantisdb:latest
```

> **Full Installation Guide**: See [docs/getting-started/installation.md](docs/getting-started/installation.md) for complete installation instructions including Kubernetes, package managers, and more.

## 🎯 Quick Start

### 1. Start MantisDB

```bash
# Start with default configuration
mantisdb

# Start with custom config
mantisdb --config=/etc/mantisdb/config.yaml

# Docker Compose (recommended for production)
docker-compose up -d
```

### 2. Access Admin Dashboard

Open your browser to **http://localhost:8081**

- Database browser and query interface
- Real-time monitoring and metrics
- Configuration management
- Backup and restore operations

### 3. Use the API

```bash
# Health check
curl http://localhost:8080/health

# Set a key-value pair
curl -X PUT http://localhost:8080/api/v1/kv/mykey \
  -H "Content-Type: application/json" \
  -d '{"value": "Hello, MantisDB!"}'

# Get the value
curl http://localhost:8080/api/v1/kv/mykey
```

## 🔧 Configuration

### Production Configuration

```yaml
# /etc/mantisdb/config.yaml
server:
  port: 8080
  admin_port: 8081
  host: "0.0.0.0"

database:
  data_dir: "/var/lib/mantisdb"
  cache_size: "512MB"
  buffer_size: "128MB"
  use_cgo: false
  sync_writes: true

security:
  admin_token: "${MANTIS_ADMIN_TOKEN}"
  enable_cors: false

logging:
  level: "info"
  format: "json"
  output: "stdout"

backup:
  enabled: true
  schedule: "0 2 * * *"  # Daily at 2 AM
  retention_days: 30
```

### Environment Variables

```bash
export MANTIS_ADMIN_TOKEN="your-secure-token"
export MANTIS_DATA_DIR="/var/lib/mantisdb"
export MANTIS_LOG_LEVEL="info"
```

## 📚 API Reference

### Key-Value Store

```bash
# Set value with TTL
curl -X PUT http://localhost:8080/api/v1/kv/session:123 \
  -H "Content-Type: application/json" \
  -d '{"value": "user_data", "ttl": 3600}'

# Get value
curl http://localhost:8080/api/v1/kv/session:123

# Delete key
curl -X DELETE http://localhost:8080/api/v1/kv/session:123
```

### Document Store

```bash
# Create document
curl -X POST http://localhost:8080/api/v1/docs/users \
  -H "Content-Type: application/json" \
  -d '{"name": "John Doe", "email": "john@example.com", "role": "admin"}'

# Query documents
curl "http://localhost:8080/api/v1/docs/users?filter=role:admin&limit=10"

# Update document
curl -X PUT http://localhost:8080/api/v1/docs/users/user_123 \
  -H "Content-Type: application/json" \
  -d '{"name": "John Smith", "email": "john.smith@example.com"}'
```

### Columnar Store

```bash
# Create table
curl -X POST http://localhost:8080/api/v1/tables/analytics \
  -H "Content-Type: application/json" \
  -d '{
    "columns": [
      {"name": "timestamp", "type": "datetime"},
      {"name": "event", "type": "string"},
      {"name": "user_id", "type": "string"},
      {"name": "value", "type": "float64"}
    ]
  }'

# Insert batch data
curl -X POST http://localhost:8080/api/v1/tables/analytics/batch \
  -H "Content-Type: application/json" \
  -d '{"rows": [...]}'

# Query with aggregation
curl "http://localhost:8080/api/v1/tables/analytics/query?select=COUNT(*),AVG(value)&group_by=event"
```

## 💻 Client Libraries

### Go

```go
import "github.com/mantisdb/mantisdb/clients/go"

client := mantisdb.NewClient("http://localhost:8080")

// Key-Value
client.KV().Set("key", []byte("value"), time.Hour)
value, _ := client.KV().Get("key")

// Documents
doc := map[string]interface{}{"name": "John", "age": 30}
client.Documents().Create("users", doc)

// Transactions
tx := client.BeginTransaction()
tx.KV().Set("key1", []byte("value1"))
tx.Documents().Create("users", doc)
tx.Commit()
```

### Python

```python
from mantisdb import MantisClient

client = MantisClient("http://localhost:8080")

# Key-Value
client.kv.set("key", b"value", ttl=3600)
value = client.kv.get("key")

# Documents with async support
async with client.documents.transaction() as tx:
    await tx.create("users", {"name": "John", "age": 30})
    await tx.commit()
```

### JavaScript/TypeScript

```typescript
import { MantisClient } from 'mantisdb-js';

const client = new MantisClient('http://localhost:8080');

// Modern async/await API
await client.kv.set('key', 'value', { ttl: 3600 });
const value = await client.kv.get('key');

// Streaming queries
const stream = client.columnar.queryStream('SELECT * FROM events');
for await (const row of stream) {
  console.log(row);
}
```

## � Production Deployment

### Docker Compose (Recommended)

```yaml
version: '3.8'
services:
  mantisdb:
    image: mantisdb/mantisdb:latest
    restart: unless-stopped
    ports:
      - "8080:8080"
      - "8081:8081"
    volumes:
      - mantisdb_data:/var/lib/mantisdb
      - ./config.yaml:/etc/mantisdb/config.yaml:ro
    environment:
      - MANTIS_ADMIN_TOKEN=${MANTIS_ADMIN_TOKEN}
    healthcheck:
      test: ["CMD", "wget", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3

volumes:
  mantisdb_data:
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: mantisdb
spec:
  serviceName: mantisdb
  replicas: 1
  selector:
    matchLabels:
      app: mantisdb
  template:
    metadata:
      labels:
        app: mantisdb
    spec:
      containers:
      - name: mantisdb
        image: mantisdb/mantisdb:latest
        ports:
        - containerPort: 8080
        - containerPort: 8081
        env:
        - name: MANTIS_ADMIN_TOKEN
          valueFrom:
            secretKeyRef:
              name: mantisdb-secret
              key: admin-token
        volumeMounts:
        - name: data
          mountPath: /var/lib/mantisdb
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
  volumeClaimTemplates:
  - metadata:
      name: data
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 10Gi
```

### Systemd Service

```bash
# Install
sudo ./install.sh

# Configure
sudo systemctl enable mantisdb
sudo systemctl start mantisdb

# Monitor
sudo systemctl status mantisdb
sudo journalctl -u mantisdb -f
```

## 📊 Monitoring & Observability

### Built-in Metrics

```bash
# Health check
curl http://localhost:8080/health

# Detailed metrics
curl http://localhost:8080/api/v1/stats

# Prometheus metrics
curl http://localhost:8080/metrics
```

### Grafana Dashboard

Import the official MantisDB dashboard from `monitoring/grafana/dashboards/`

### Alerting

Configure alerts in `monitoring/alerts.yaml`:

```yaml
alerts:
  - name: high_memory_usage
    condition: memory_usage > 80%
    action: email
  - name: disk_space_low
    condition: disk_free < 10%
    action: webhook
```

## 🔒 Security

### Authentication

```bash
# Set admin token
export MANTIS_ADMIN_TOKEN="$(openssl rand -hex 32)"

# Enable TLS
mantisdb --tls-cert=/path/to/cert.pem --tls-key=/path/to/key.pem
```

### Network Security

```yaml
security:
  admin_token: "${MANTIS_ADMIN_TOKEN}"
  enable_cors: false
  rate_limit: 1000
  allowed_ips: ["10.0.0.0/8", "192.168.0.0/16"]
```

## ⚡ Performance Tuning

### Hardware Recommendations

- **CPU**: 4+ cores for production workloads
- **Memory**: 8GB+ RAM (4GB for cache, 4GB for system)
- **Storage**: SSD recommended for data directory
- **Network**: 1Gbps+ for distributed deployments

### Configuration Tuning

```yaml
database:
  cache_size: "2GB"        # 25-50% of available RAM
  buffer_size: "512MB"     # 10-25% of cache size
  use_cgo: true           # Enable for maximum performance
  sync_writes: false      # Disable for higher throughput
  max_connections: 1000   # Adjust based on workload
```

### Benchmarks

```bash
# Run performance benchmarks
make benchmark

# Custom benchmark
mantisdb --benchmark-only --benchmark-duration=60s
```

## 🛠️ Development

### Building

```bash
# Development build
make build

# Production build with all platforms
make production

# Run tests
make test

# Development server with hot reload
make run-dev
```

### Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make your changes and add tests
4. Run tests: `make test`
5. Submit a pull request

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

## 📚 Documentation

Complete documentation is available in the [docs/](docs/) directory:

- **[Getting Started](docs/getting-started/)** - Installation and quick start guides
- **[Architecture](docs/architecture/)** - System design and components
- **[API Reference](docs/api/)** - REST API documentation
- **[Client Libraries](docs/clients/)** - Go, Python, and JavaScript clients
- **[Performance](docs/performance/)** - Benchmarks and optimization
- **[Development](docs/development/)** - Building, testing, and contributing

## 🆘 Support

- **Documentation**: [docs/](docs/)
- **Issues**: [GitHub Issues](https://github.com/mantisdb/mantisdb/issues)
- **Discussions**: [GitHub Discussions](https://github.com/mantisdb/mantisdb/discussions)
- **Enterprise Support**: enterprise@mantisdb.com

---

**Ready to get started?** See the [Installation Guide](docs/getting-started/installation.md) and [Quick Start Guide](docs/getting-started/quickstart.md).