# Quick Start Guide

Get MantisDB up and running in just a few minutes with this quick start guide.

## 🚀 Installation

### Option 1: Docker (Recommended)

```bash
# Start MantisDB with Docker
docker run -d \
  --name mantisdb \
  -p 8080:8080 \
  -p 8081:8081 \
  -v mantisdb_data:/var/lib/mantisdb \
  mantisdb/mantisdb:latest

# Check if it's running
docker ps | grep mantisdb
```

### Option 2: Binary Installation

```bash
# Linux/macOS one-liner
curl -L https://github.com/mantisdb/mantisdb/releases/latest/download/mantisdb-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m).tar.gz | tar xz
cd mantisdb-*
sudo ./install.sh

# Start MantisDB
mantisdb
```

### Option 3: Build from Source

```bash
git clone https://github.com/mantisdb/mantisdb.git
cd mantisdb
make build
./mantisdb
```

## ✅ Verify Installation

```bash
# Health check
curl http://localhost:8080/health

# Expected response:
# {"status":"healthy","timestamp":1640995200}
```

## 🌐 Access Admin Dashboard

Open your browser to **http://localhost:8081**

The admin dashboard provides:
- Real-time database monitoring
- SQL query interface
- Data browser and editor
- Configuration management
- Backup and restore tools

## 🔑 First Steps

### 1. Set an Admin Token (Recommended)

```bash
# Generate a secure token
export MANTIS_ADMIN_TOKEN="$(openssl rand -hex 32)"

# Restart MantisDB with the token
mantisdb --admin-token="$MANTIS_ADMIN_TOKEN"
```

### 2. Basic Key-Value Operations

```bash
# Set a key-value pair
curl -X PUT http://localhost:8080/api/v1/kv/greeting \
  -H "Content-Type: application/json" \
  -d '{"value": "Hello, MantisDB!"}'

# Get the value
curl http://localhost:8080/api/v1/kv/greeting

# Response:
# {"key":"greeting","value":"Hello, MantisDB!"}

# Set with TTL (expires in 1 hour)
curl -X PUT http://localhost:8080/api/v1/kv/session \
  -H "Content-Type: application/json" \
  -d '{"value": "user_session_data", "ttl": 3600}'
```

### 3. Document Operations

```bash
# Create a document
curl -X POST http://localhost:8080/api/v1/docs/users \
  -H "Content-Type: application/json" \
  -d '{
    "id": "user_001",
    "data": {
      "name": "John Doe",
      "email": "john@example.com",
      "role": "admin",
      "created_at": "2024-01-01T00:00:00Z"
    }
  }'

# Get the document
curl http://localhost:8080/api/v1/docs/users/user_001

# Query documents
curl -X POST http://localhost:8080/api/v1/docs/query \
  -H "Content-Type: application/json" \
  -d '{
    "collection": "users",
    "filter": {"role": "admin"},
    "limit": 10
  }'
```

### 4. Batch Operations

```bash
# Execute multiple operations atomically
curl -X POST http://localhost:8080/api/v1/kv/batch \
  -H "Content-Type: application/json" \
  -d '{
    "atomic": true,
    "operations": [
      {"type": "set", "key": "counter", "value": "1"},
      {"type": "set", "key": "timestamp", "value": "2024-01-01T00:00:00Z"},
      {"type": "get", "key": "greeting"}
    ]
  }'
```

## 📊 Monitor Your Database

### View Statistics

```bash
# Get database statistics
curl http://localhost:8080/api/v1/stats

# Response includes:
# - Memory usage
# - Operation counts
# - Cache hit rates
# - Active connections
```

### Prometheus Metrics

```bash
# Get Prometheus-compatible metrics
curl http://localhost:8080/metrics
```

## 🔧 Configuration

### Basic Configuration File

Create `/etc/mantisdb/config.yaml`:

```yaml
server:
  port: 8080
  admin_port: 8081
  host: "0.0.0.0"

database:
  data_dir: "/var/lib/mantisdb"
  cache_size: "512MB"
  buffer_size: "128MB"
  sync_writes: true

security:
  admin_token: "${MANTIS_ADMIN_TOKEN}"
  enable_cors: false

logging:
  level: "info"
  format: "json"
  output: "stdout"
```

### Environment Variables

```bash
# Essential settings
export MANTIS_ADMIN_TOKEN="your-secure-token"
export MANTIS_DATA_DIR="/var/lib/mantisdb"
export MANTIS_LOG_LEVEL="info"
export MANTIS_CACHE_SIZE="1GB"
```

## 🐳 Docker Compose Setup

Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  mantisdb:
    image: mantisdb/mantisdb:latest
    container_name: mantisdb
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

Start with:

```bash
export MANTIS_ADMIN_TOKEN="$(openssl rand -hex 32)"
docker-compose up -d
```

## 📚 Client Libraries

### Go Client

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/mantisdb/mantisdb/clients/go"
)

func main() {
    client := mantisdb.NewClient("http://localhost:8080")
    
    // Key-Value operations
    ctx := context.Background()
    err := client.KV().Set(ctx, "hello", []byte("world"), time.Hour)
    if err != nil {
        panic(err)
    }
    
    value, err := client.KV().Get(ctx, "hello")
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Value: %s\n", value)
}
```

### Python Client

```python
import asyncio
from mantisdb import MantisClient

async def main():
    client = MantisClient("http://localhost:8080")
    
    # Key-Value operations
    await client.kv.set("hello", b"world", ttl=3600)
    value = await client.kv.get("hello")
    print(f"Value: {value.decode()}")
    
    # Document operations
    doc = {"name": "Alice", "age": 30}
    await client.documents.create("users", doc)

if __name__ == "__main__":
    asyncio.run(main())
```

### JavaScript Client

```javascript
import { MantisClient } from 'mantisdb-js';

const client = new MantisClient('http://localhost:8080');

// Key-Value operations
await client.kv.set('hello', 'world', { ttl: 3600 });
const value = await client.kv.get('hello');
console.log(`Value: ${value}`);

// Document operations
const doc = { name: 'Alice', age: 30 };
await client.documents.create('users', doc);
```

## 🔍 Next Steps

Now that you have MantisDB running, explore these topics:

1. **[Configuration Guide](configuration.md)** - Detailed configuration options
2. **[API Reference](../api/rest.md)** - Complete API documentation
3. **[Admin Dashboard](../admin/dashboard.md)** - Web interface features
4. **[Production Deployment](../deployment/production.md)** - Production setup
5. **[Performance Tuning](../deployment/performance.md)** - Optimization tips

## 🆘 Troubleshooting

### Common Issues

**Port already in use:**
```bash
# Check what's using the port
sudo lsof -i :8080
sudo lsof -i :8081

# Use different ports
mantisdb --port=8082 --admin-port=8083
```

**Permission denied:**
```bash
# Fix data directory permissions
sudo chown -R $USER:$USER /var/lib/mantisdb
```

**Docker container won't start:**
```bash
# Check container logs
docker logs mantisdb

# Check container status
docker inspect mantisdb
```

### Getting Help

- **Documentation**: [Full documentation](../README.md)
- **GitHub Issues**: [Report bugs](https://github.com/mantisdb/mantisdb/issues)
- **Community**: [GitHub Discussions](https://github.com/mantisdb/mantisdb/discussions)

---

**Congratulations!** You now have MantisDB running. Continue with the [Configuration Guide](configuration.md) to customize your setup.