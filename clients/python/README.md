# MantisDB Python Client

Official Python client library for MantisDB with both sync and async interfaces, type safety, and connection pooling.

> **Full Documentation**: See [Python Client Documentation](../../docs/clients/python.md) for complete API reference and examples.

## Installation

```bash
pip install mantisdb-python
```

## Features

- Synchronous and asynchronous interfaces
- Full type hints and Pydantic models
- Connection pooling and retry logic
- ACID transaction support with context managers
- Comprehensive error handling

## Quick Start

### Synchronous Client

```python
import mantisdb

client = mantisdb.Client("mantisdb://localhost:8080")

try:
    # Insert data
    user = {"name": "John Doe", "email": "john@example.com"}
    client.insert("users", user)
    
    # Query data
    result = client.query("SELECT * FROM users")
    print(f"Found {result.row_count} users")
finally:
    client.close()
```

### Asynchronous Client

```python
import asyncio
import mantisdb

async def main():
    client = mantisdb.AsyncClient("mantisdb://localhost:8080")
    
    try:
        user = {"name": "Jane Doe", "email": "jane@example.com"}
        await client.insert("users", user)
        
        result = await client.query("SELECT * FROM users")
        print(f"Found {result.row_count} users")
    finally:
        await client.close()

asyncio.run(main())
```

## Documentation

For complete documentation including:
- Configuration options
- CRUD operations
- Transaction handling
- Error handling
- Connection pooling
- Async operations
- Best practices

See the [Python Client Documentation](../../docs/clients/python.md).

## License

MIT License - Part of the MantisDB project.