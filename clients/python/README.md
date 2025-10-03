# MantisDB Python Client

The official Python client library for MantisDB, providing both synchronous and asynchronous interfaces with comprehensive error handling, connection pooling, and full transaction support.

## Features

- **Dual Interface**: Both synchronous and asynchronous client implementations
- **Type Safety**: Full type hints and Pydantic models for data validation
- **Connection Pooling**: Efficient connection management with configurable pool sizes
- **Error Handling**: Comprehensive error handling with detailed error information
- **Transaction Support**: Full ACID transaction support with context managers
- **CRUD Operations**: Complete support for Create, Read, Update, Delete operations
- **Query Interface**: Execute raw SQL queries with structured results
- **Authentication**: Built-in support for basic authentication
- **Retry Logic**: Configurable retry mechanisms for resilient operations
- **Async/Await**: Native async/await support for high-performance applications

## Installation

```bash
pip install mantisdb-python
```

### Development Installation

```bash
git clone https://github.com/mantisdb/mantisdb.git
cd mantisdb/clients/python
pip install -e ".[dev]"
```

## Quick Start

### Synchronous Client

```python
import mantisdb

# Create client with connection string
client = mantisdb.Client("mantisdb://admin:password@localhost:8080")

# Or with configuration object
config = mantisdb.MantisConfig(
    host="localhost",
    port=8080,
    username="admin",
    password="password"
)
client = mantisdb.Client(config)

try:
    # Test connection
    client.ping()
    
    # Insert data
    user = {
        "name": "John Doe",
        "email": "john@example.com",
        "age": 30
    }
    client.insert("users", user)
    
    # Query data
    result = client.query("SELECT * FROM users WHERE age > 25")
    print(f"Found {result.row_count} users")
    
    for row in result.rows:
        print(f"User: {row['name']} ({row['email']})")

finally:
    client.close()
```

### Asynchronous Client

```python
import asyncio
import mantisdb

async def main():
    # Create async client
    client = mantisdb.AsyncClient("mantisdb://admin:password@localhost:8080")
    
    try:
        # Test connection
        await client.ping()
        
        # Insert data
        user = {
            "name": "Jane Doe",
            "email": "jane@example.com",
            "age": 28
        }
        await client.insert("users", user)
        
        # Query data
        result = await client.query("SELECT * FROM users WHERE age > 25")
        print(f"Found {result.row_count} users")
        
        for row in result.rows:
            print(f"User: {row['name']} ({row['email']})")
    
    finally:
        await client.close()

asyncio.run(main())
```

## Configuration

The client supports extensive configuration through the `MantisConfig` class:

```python
from mantisdb import MantisConfig

config = MantisConfig(
    host="localhost",
    port=8080,
    username="admin",
    password="password",
    max_connections=10,
    connection_timeout=30.0,
    request_timeout=60.0,
    retry_attempts=3,
    retry_delay=1.0,
    enable_compression=True,
    tls_enabled=False
)
```

### Configuration Options

- `host`: Database server hostname (default: "localhost")
- `port`: Database server port (default: 8080)
- `username`: Authentication username
- `password`: Authentication password
- `max_connections`: Maximum number of connections in pool (default: 10)
- `connection_timeout`: Connection timeout in seconds (default: 30.0)
- `request_timeout`: Request timeout in seconds (default: 60.0)
- `retry_attempts`: Number of retry attempts (default: 3)
- `retry_delay`: Delay between retries in seconds (default: 1.0)
- `enable_compression`: Enable HTTP compression (default: True)
- `tls_enabled`: Enable TLS/SSL (default: False)

## CRUD Operations

### Insert

```python
# Insert single record
user = {
    "name": "Alice Smith",
    "email": "alice@example.com",
    "age": 25,
    "active": True
}
client.insert("users", user)

# Async version
await async_client.insert("users", user)
```

### Query

```python
# Execute SQL query
result = client.query("SELECT * FROM users WHERE age > 25 ORDER BY name")

print(f"Columns: {result.columns}")
print(f"Row count: {result.row_count}")

for row in result.rows:
    print(f"User: {row['name']} - Age: {row['age']}")

# Async version
result = await async_client.query("SELECT * FROM users WHERE age > 25")
```

### Get with Filters

```python
# Get data with filters
filters = {
    "age": 25,
    "active": True
}
result = client.get("users", filters)

# Async version
result = await async_client.get("users", filters)
```

### Update

```python
# Update record
updates = {
    "age": 26,
    "last_login": "2025-01-03T10:30:00Z"
}
client.update("users", "user-id-123", updates)

# Async version
await async_client.update("users", "user-id-123", updates)
```

### Delete

```python
# Delete record
client.delete("users", "user-id-123")

# Async version
await async_client.delete("users", "user-id-123")
```

## Transactions

### Synchronous Transactions

```python
# Using context manager (recommended)
with client.begin_transaction() as tx:
    tx.insert("users", {"name": "User 1", "email": "user1@example.com"})
    tx.insert("users", {"name": "User 2", "email": "user2@example.com"})
    
    # Query within transaction
    result = tx.query("SELECT COUNT(*) as count FROM users")
    print(f"Total users: {result.rows[0]['count']}")
    
    # Transaction automatically commits on successful exit
    # or rolls back on exception

# Manual transaction management
tx = client.begin_transaction()
try:
    tx.insert("users", {"name": "User 3", "email": "user3@example.com"})
    tx.commit()
except Exception:
    tx.rollback()
    raise
```

### Asynchronous Transactions

```python
# Using async context manager (recommended)
async with await async_client.begin_transaction() as tx:
    await tx.insert("users", {"name": "User 1", "email": "user1@example.com"})
    await tx.insert("users", {"name": "User 2", "email": "user2@example.com"})
    
    # Query within transaction
    result = await tx.query("SELECT COUNT(*) as count FROM users")
    print(f"Total users: {result.rows[0]['count']}")
    
    # Transaction automatically commits on successful exit

# Manual async transaction management
tx = await async_client.begin_transaction()
try:
    await tx.insert("users", {"name": "User 3", "email": "user3@example.com"})
    await tx.commit()
except Exception:
    await tx.rollback()
    raise
```

## Error Handling

The client provides detailed error information through custom exception classes:

```python
import mantisdb

try:
    result = client.query("INVALID SQL SYNTAX")
except mantisdb.MantisError as e:
    print(f"Error Code: {e.code}")
    print(f"Message: {e.message}")
    print(f"Request ID: {e.request_id}")
    print(f"Details: {e.details}")
except mantisdb.ConnectionError as e:
    print(f"Connection failed: {e}")
except mantisdb.QueryError as e:
    print(f"Query failed: {e}")
except mantisdb.TransactionError as e:
    print(f"Transaction failed: {e}")
```

### Exception Hierarchy

- `MantisDBError`: Base exception class
  - `ConnectionError`: Connection-related errors
  - `QueryError`: Query execution errors
  - `TransactionError`: Transaction-related errors
  - `AuthenticationError`: Authentication failures
  - `ValidationError`: Data validation errors
  - `TimeoutError`: Request timeout errors
  - `RetryExhaustedError`: All retry attempts failed

## Data Models

### QueryResult

```python
class QueryResult:
    rows: List[Dict[str, Any]]      # Query result rows
    columns: List[str]              # Column names
    row_count: int                  # Number of rows returned
    metadata: Optional[Dict[str, Any]]  # Additional metadata
```

### MantisConfig

```python
class MantisConfig:
    host: str = "localhost"
    port: int = 8080
    username: Optional[str] = None
    password: Optional[str] = None
    max_connections: int = 10
    connection_timeout: float = 30.0
    request_timeout: float = 60.0
    retry_attempts: int = 3
    retry_delay: float = 1.0
    enable_compression: bool = True
    tls_enabled: bool = False
```

## Connection Pooling

Both sync and async clients automatically manage connection pools:

### Synchronous Client
- Uses `requests.Session` with `HTTPAdapter`
- Configurable pool size via `max_connections`
- Automatic connection reuse and cleanup

### Asynchronous Client
- Uses `aiohttp.ClientSession` with `TCPConnector`
- Configurable connection limits and timeouts
- Efficient async connection management

## Retry Logic

Built-in retry logic for resilient operations:

- Configurable retry attempts and delays
- Exponential backoff for retry delays
- Automatic retry on server errors (5xx status codes)
- Connection error handling with retries

## Concurrent Operations

### Async Concurrent Operations

```python
import asyncio

async def insert_users_concurrently():
    client = mantisdb.AsyncClient(config)
    
    # Create multiple insert tasks
    tasks = []
    for i in range(100):
        user = {
            "name": f"User {i}",
            "email": f"user{i}@example.com",
            "age": 20 + (i % 50)
        }
        tasks.append(client.insert("users", user))
    
    # Execute all inserts concurrently
    await asyncio.gather(*tasks)
    
    await client.close()

asyncio.run(insert_users_concurrently())
```

## Best Practices

1. **Reuse Client Instances**: Create one client instance and reuse it across your application
2. **Use Context Managers**: Always use context managers for transactions
3. **Handle Errors**: Implement comprehensive error handling
4. **Close Resources**: Always close clients when done
5. **Use Async for High Concurrency**: Use async client for high-throughput applications
6. **Configure Timeouts**: Set appropriate timeouts for your use case
7. **Connection Pooling**: Configure connection pools based on your concurrency needs

## Type Safety

The client is fully typed with comprehensive type hints:

```python
from typing import Dict, List, Any
import mantisdb

client: mantisdb.Client = mantisdb.Client(config)
result: mantisdb.QueryResult = client.query("SELECT * FROM users")
rows: List[Dict[str, Any]] = result.rows
```

## Testing

Run the test suite:

```bash
# Install development dependencies
pip install -e ".[dev]"

# Run tests
pytest

# Run async tests
pytest -v tests/test_async_client.py

# Run with coverage
pytest --cov=mantisdb --cov-report=html
```

## License

This client library is part of the MantisDB project and follows the same license terms.