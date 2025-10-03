# MantisDB Client Libraries

This directory contains the official client libraries for MantisDB in multiple programming languages.

## Available Clients

- **Go** (`./go/`) - Native Go client with full feature support
- **Python** (`./python/`) - Python client with both sync and async support  
- **JavaScript/TypeScript** (`./javascript/`) - Node.js and browser compatible client

## Quick Start

### Go Client

```go
package main

import (
    "context"
    "log"
    
    mantisdb "github.com/mantisdb/mantisdb/clients/go"
)

func main() {
    config := mantisdb.DefaultConfig()
    config.Host = "localhost"
    config.Port = 8080
    config.Username = "admin"
    config.Password = "password"

    client, err := mantisdb.NewClient(config)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    ctx := context.Background()
    result, err := client.Query(ctx, "SELECT * FROM users")
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Found %d rows", result.RowCount)
}
```

### Python Client

```python
from mantisdb import Client, MantisConfig

# Synchronous client
config = MantisConfig(
    host="localhost",
    port=8080,
    username="admin",
    password="password"
)

client = Client(config)
try:
    result = client.query("SELECT * FROM users")
    print(f"Found {result.row_count} rows")
finally:
    client.close()

# Asynchronous client
import asyncio
from mantisdb import AsyncClient

async def main():
    client = AsyncClient(config)
    try:
        result = await client.query("SELECT * FROM users")
        print(f"Found {result.row_count} rows")
    finally:
        await client.close()

asyncio.run(main())
```

### JavaScript/TypeScript Client

```javascript
import { MantisClient } from 'mantisdb-js';

const client = new MantisClient({
  host: 'localhost',
  port: 8080,
  username: 'admin',
  password: 'password'
});

try {
  const result = await client.query('SELECT * FROM users');
  console.log(`Found ${result.rowCount} rows`);
} finally {
  await client.close();
}
```

## Features

All client libraries support:

- **CRUD Operations** - Insert, update, delete, and query data
- **Transactions** - ACID transactions with commit/rollback
- **Authentication** - Basic auth, API keys, and JWT tokens
- **Connection Pooling** - Efficient connection management
- **Retry Logic** - Automatic retry on transient failures
- **Failover Support** - Automatic failover to backup servers
- **Error Handling** - Comprehensive error reporting
- **Type Safety** - Strong typing (Go, TypeScript) and type hints (Python)

## Testing

### Prerequisites

1. **MantisDB Server**: Ensure MantisDB is running on `localhost:8080` (or configure test environment variables)
2. **Test Dependencies**: Each client has its own test dependencies

### Environment Variables

Configure test settings using environment variables:

```bash
export MANTISDB_TEST_HOST=localhost
export MANTISDB_TEST_PORT=8080
export MANTISDB_TEST_USERNAME=admin
export MANTISDB_TEST_PASSWORD=password
export MANTISDB_TEST_API_KEY=your-api-key  # Optional
```

### Running All Tests

Use the comprehensive test runner:

```bash
# Run all client tests
./run_tests.sh

# Run specific client tests
./run_tests.sh --go-only
./run_tests.sh --python-only
./run_tests.sh --js-only

# Include performance tests
./run_tests.sh --performance

# Include load tests (slow)
./run_tests.sh --load-tests

# Run tests in parallel
./run_tests.sh --parallel

# Verbose output with coverage
./run_tests.sh --verbose
```

### Running Individual Client Tests

#### Go Tests

```bash
cd go
go test -v ./...

# Run benchmarks
go test -bench=. -benchmem ./...

# Run with race detection
go test -race -v ./...
```

#### Python Tests

```bash
cd python
python -m venv venv
source venv/bin/activate
pip install -e .
pip install pytest pytest-asyncio pytest-cov

# Run all tests
pytest -v tests/

# Run with coverage
pytest --cov=mantisdb --cov-report=html tests/

# Run performance tests
pytest -v tests/test_performance.py

# Run load tests
pytest --runslow tests/
```

#### JavaScript Tests

```bash
cd javascript
npm install
npm run build

# Run all tests
npm test

# Run with coverage
npm test -- --coverage

# Run specific test files
npm test -- tests/integration.test.ts
```

## Test Coverage

The test suites include:

### Integration Tests
- **Connection Testing** - Server connectivity and authentication
- **CRUD Operations** - Create, read, update, delete operations
- **Transaction Management** - Transaction lifecycle and rollback
- **Error Handling** - Error scenarios and recovery
- **Concurrent Operations** - Multi-threaded/async operations

### Performance Tests
- **Query Performance** - Single and batch query performance
- **Insert Performance** - Bulk insert operations
- **Transaction Performance** - Transaction throughput
- **Concurrent Performance** - Multi-client performance

### Load Tests
- **High Concurrency** - Many simultaneous connections
- **Sustained Load** - Long-running operations
- **Memory Usage** - Memory leak detection
- **Connection Pool Stress** - Pool exhaustion scenarios

### Cross-Platform Tests
- **Unicode Support** - International character handling
- **Large Data** - Large payload handling
- **Data Types** - All supported data types
- **Platform Compatibility** - Different OS and runtime versions

## Performance Benchmarks

Typical performance characteristics (on modern hardware):

| Operation | Go Client | Python Client | JavaScript Client |
|-----------|-----------|---------------|-------------------|
| Simple Query | 1000+ ops/sec | 500+ ops/sec | 800+ ops/sec |
| Insert | 500+ ops/sec | 200+ ops/sec | 400+ ops/sec |
| Transaction | 100+ ops/sec | 50+ ops/sec | 80+ ops/sec |
| Concurrent Queries | 2000+ ops/sec | 1000+ ops/sec | 1500+ ops/sec |

*Performance varies based on hardware, network latency, and query complexity.*

## Authentication

All clients support multiple authentication methods:

### Basic Authentication
```go
// Go
config.Username = "admin"
config.Password = "password"
```

```python
# Python
config = MantisConfig(username="admin", password="password")
```

```javascript
// JavaScript
const client = new MantisClient({
  username: 'admin',
  password: 'password'
});
```

### API Key Authentication
```go
// Go
config.APIKey = "your-api-key"
```

```python
# Python
config = MantisConfig(api_key="your-api-key")
```

```javascript
// JavaScript
const client = new MantisClient({
  apiKey: 'your-api-key'
});
```

### JWT Authentication
```go
// Go
config.ClientID = "client-id"
config.ClientSecret = "client-secret"
config.TokenURL = "https://auth.example.com/token"
```

```python
# Python
from mantisdb.auth import JWTAuthProvider
provider = JWTAuthProvider("client-id", "client-secret", "token-url")
config = MantisConfig(auth_provider=provider)
```

```javascript
// JavaScript
import { JWTAuthProvider } from 'mantisdb-js';
const authProvider = new JWTAuthProvider('client-id', 'client-secret');
const client = new MantisClient({ authProvider });
```

## Error Handling

All clients provide structured error handling:

```go
// Go
if err != nil {
    if mantisErr, ok := err.(*mantisdb.MantisError); ok {
        log.Printf("MantisDB error [%s]: %s", mantisErr.Code, mantisErr.Message)
    }
}
```

```python
# Python
try:
    result = client.query("SELECT * FROM users")
except MantisError as e:
    print(f"MantisDB error [{e.code}]: {e.message}")
```

```javascript
// JavaScript
try {
  const result = await client.query('SELECT * FROM users');
} catch (error) {
  if (error instanceof MantisError) {
    console.log(`MantisDB error [${error.code}]: ${error.message}`);
  }
}
```

## Contributing

When contributing to client libraries:

1. **Add Tests** - All new features must include comprehensive tests
2. **Update Documentation** - Update README and inline documentation
3. **Performance Testing** - Include performance benchmarks for new features
4. **Cross-Platform Testing** - Test on multiple platforms and versions
5. **Error Handling** - Ensure proper error handling and reporting

## Support

For issues and questions:

- **GitHub Issues** - Report bugs and feature requests
- **Documentation** - Check the main MantisDB documentation
- **Examples** - See the `examples/` directory in each client
- **Tests** - Review test files for usage examples