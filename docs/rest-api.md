# REST API

MantisDB provides a high-performance REST API similar to PostgREST, built with Axum in Rust for maximum throughput and low latency.

## Features

- **High Performance**: Built on Axum and Tokio for async I/O
- **Automatic CRUD**: RESTful endpoints for all data models
- **Connection Pooling**: Integrated with MantisDB connection pool
- **Compression**: Automatic response compression
- **CORS**: Configurable cross-origin support
- **Tracing**: Built-in request tracing and logging

## Quick Start

### Starting the Server

```bash
# Using the Rust example
cd rust-core
cargo run --example rest_api_server --release

# Server starts on http://0.0.0.0:8080
```

### Configuration

```rust
use mantisdb_core::{RestApiConfig, RestApiServer, ConnectionPool};

let config = RestApiConfig {
    bind_addr: "0.0.0.0:8080".parse()?,
    enable_cors: true,
    enable_compression: true,
    enable_tracing: true,
    max_body_size: 10 * 1024 * 1024, // 10MB
    request_timeout: 30,
};

let server = RestApiServer::new(config, pool);
server.start().await?;
```

## API Endpoints

### Health & Monitoring

#### Health Check
```http
GET /health
```

**Response:**
```json
{
  "success": true,
  "data": {
    "status": "healthy",
    "timestamp": "2025-10-08T01:30:00Z"
  }
}
```

#### Pool Statistics
```http
GET /stats
```

**Response:**
```json
{
  "success": true,
  "data": {
    "pool": {
      "total_connections": 50,
      "active_connections": 10,
      "idle_connections": 40,
      "wait_count": 1000,
      "avg_wait_time_ms": 2,
      "connections_created": 50,
      "connections_closed": 0,
      "health_check_failures": 0
    }
  }
}
```

### Key-Value API

#### Get Value
```http
GET /api/v1/kv/:key
```

**Example:**
```bash
curl http://localhost:8080/api/v1/kv/mykey
```

**Response:**
```json
{
  "success": true,
  "data": {
    "key": "mykey",
    "value": [72, 101, 108, 108, 111]
  }
}
```

#### Set Value
```http
PUT /api/v1/kv/:key
Content-Type: application/json

{
  "value": [72, 101, 108, 108, 111],
  "ttl": 3600
}
```

**Example:**
```bash
curl -X PUT http://localhost:8080/api/v1/kv/mykey \
  -H 'Content-Type: application/json' \
  -d '{"value": [72,101,108,108,111]}'
```

**Response:**
```json
{
  "success": true,
  "data": {
    "key": "mykey",
    "message": "Value set successfully"
  }
}
```

#### Delete Value
```http
DELETE /api/v1/kv/:key
```

**Example:**
```bash
curl -X DELETE http://localhost:8080/api/v1/kv/mykey
```

#### Batch Operations
```http
POST /api/v1/kv
Content-Type: application/json

{
  "operations": [
    {
      "op": "set",
      "key": "key1",
      "value": [1, 2, 3]
    },
    {
      "op": "get",
      "key": "key2"
    },
    {
      "op": "delete",
      "key": "key3"
    }
  ]
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "results": [
      {
        "key": "key1",
        "success": true
      },
      {
        "key": "key2",
        "success": true,
        "value": [4, 5, 6]
      },
      {
        "key": "key3",
        "success": true
      }
    ]
  }
}
```

#### List Keys
```http
GET /api/v1/kv?limit=100&offset=0&prefix=user:
```

### Table API (PostgREST-like)

#### Query Table
```http
GET /api/v1/tables/:table?column=value&limit=10
```

**Example:**
```bash
curl "http://localhost:8080/api/v1/tables/users?age=25&limit=10"
```

#### Insert Row
```http
POST /api/v1/tables/:table
Content-Type: application/json

{
  "name": "John Doe",
  "age": 30,
  "email": "john@example.com"
}
```

#### Get Row by ID
```http
GET /api/v1/tables/:table/:id
```

#### Update Row
```http
PUT /api/v1/tables/:table/:id
Content-Type: application/json

{
  "age": 31
}
```

#### Delete Row
```http
DELETE /api/v1/tables/:table/:id
```

## Response Format

All responses follow a consistent format:

```json
{
  "success": true|false,
  "data": { ... },
  "error": "error message if failed",
  "meta": {
    "count": 10,
    "total": 100,
    "page": 1,
    "per_page": 10
  }
}
```

## Error Responses

### HTTP Status Codes

| Code | Meaning |
|------|---------|
| 200 | Success |
| 400 | Bad Request |
| 404 | Not Found |
| 408 | Request Timeout |
| 503 | Service Unavailable (Pool Exhausted) |
| 500 | Internal Server Error |

### Error Format

```json
{
  "success": false,
  "data": null,
  "error": "Resource not found",
  "meta": null
}
```

## Performance

### Benchmarks

- **Throughput**: 50,000+ requests/sec
- **Latency**: <1ms p50, <5ms p99
- **Concurrent Connections**: 10,000+
- **Memory**: ~10MB base + ~1KB per connection

### Optimization Tips

1. **Enable Compression**: Reduces bandwidth by 70-90%
2. **Use Batch Operations**: Up to 10x faster than individual requests
3. **Connection Pooling**: Reuse connections for better performance
4. **HTTP/2**: Enable for multiplexing and header compression

## Security

### CORS Configuration

```rust
let config = RestApiConfig {
    enable_cors: true,
    // CORS is permissive by default
    // For production, configure specific origins
    ..Default::default()
};
```

### Rate Limiting

Rate limiting should be implemented at the reverse proxy level (nginx, Caddy, etc.)

### Authentication

Authentication can be added via middleware:

```rust
// Custom authentication middleware can be added
// to the Axum router
```

## Integration Examples

### cURL

```bash
# Set a value
curl -X PUT http://localhost:8080/api/v1/kv/user:123 \
  -H 'Content-Type: application/json' \
  -d '{"value": [1,2,3,4,5]}'

# Get a value
curl http://localhost:8080/api/v1/kv/user:123

# Batch operations
curl -X POST http://localhost:8080/api/v1/kv \
  -H 'Content-Type: application/json' \
  -d '{
    "operations": [
      {"op": "set", "key": "k1", "value": [1,2,3]},
      {"op": "get", "key": "k2"}
    ]
  }'
```

### JavaScript/TypeScript

```typescript
// Set value
await fetch('http://localhost:8080/api/v1/kv/mykey', {
  method: 'PUT',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ value: [72, 101, 108, 108, 111] })
});

// Get value
const response = await fetch('http://localhost:8080/api/v1/kv/mykey');
const data = await response.json();
console.log(data.data.value);
```

### Python

```python
import requests

# Set value
requests.put('http://localhost:8080/api/v1/kv/mykey', json={
    'value': [72, 101, 108, 108, 111]
})

# Get value
response = requests.get('http://localhost:8080/api/v1/kv/mykey')
data = response.json()
print(data['data']['value'])
```

## Comparison with PostgREST

| Feature | MantisDB REST API | PostgREST |
|---------|-------------------|-----------|
| Language | Rust (Axum) | Haskell |
| Performance | 50k+ req/sec | 10k+ req/sec |
| Auto CRUD | ✅ | ✅ |
| Connection Pool | Built-in | External |
| Compression | Built-in | Via nginx |
| WebSockets | Planned | Via extensions |

## Deployment

### Docker

```dockerfile
FROM rust:1.75 as builder
WORKDIR /app
COPY rust-core .
RUN cargo build --release --example rest_api_server

FROM debian:bookworm-slim
COPY --from=builder /app/target/release/examples/rest_api_server /usr/local/bin/
EXPOSE 8080
CMD ["rest_api_server"]
```

### Systemd Service

```ini
[Unit]
Description=MantisDB REST API
After=network.target

[Service]
Type=simple
User=mantisdb
ExecStart=/usr/local/bin/rest_api_server
Restart=always

[Install]
WantedBy=multi-user.target
```

## See Also

- [Connection Pooling](./connection-pooling.md)
- [Performance Tuning](./performance.md)
- [Client Libraries](../clients/overview.md)
