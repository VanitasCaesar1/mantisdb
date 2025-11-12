# MantisDB Python SDK

Official Python client library for MantisDB multimodal database.

## Installation

```bash
pip install mantisdb
```

## Quick Start

```python
from mantisdb import MantisDB

# Connect to MantisDB
db = MantisDB("http://localhost:8080")

# Key-Value operations
db.kv.set("user:123", "John Doe")
name = db.kv.get("user:123")

# Document operations
db.docs.insert("users", {"name": "Alice", "age": 30, "email": "alice@example.com"})
users = db.docs.find("users", {"age": {"$gt": 25}})

# SQL operations
results = db.sql.execute("SELECT * FROM users WHERE age > 25")

# Vector operations
embedding = [0.1, 0.2, 0.3, 0.4]  # Your vector
db.vectors.insert("embeddings", embedding, {"text": "hello world"})
similar = db.vectors.search("embeddings", embedding, k=10)

# Query builder
from mantisdb import QueryBuilder
query = QueryBuilder(db) \
    .select("name", "email") \
    .from_table("users") \
    .where("age > 25") \
    .order_by("name") \
    .limit(10) \
    .execute()
```

## Features

- **Key-Value Store**: Fast key-value operations with TTL support
- **Document Store**: JSON document storage with rich queries
- **SQL Database**: Full SQL support with JOINs and transactions
- **Vector Database**: Similarity search with metadata filtering
- **Connection Pooling**: Automatic connection management
- **Type Hints**: Full type hint coverage for IDE support
- **Context Manager**: Clean resource management with `with` statement

## API Reference

### MantisDB Client

```python
db = MantisDB(base_url="http://localhost:8080", auth_token="your-token", timeout=30)
```

### Key-Value Operations

- `db.kv.get(key)` - Get value
- `db.kv.set(key, value, ttl=None)` - Set value with optional TTL
- `db.kv.delete(key)` - Delete key
- `db.kv.exists(key)` - Check if key exists

### Document Operations

- `db.docs.insert(collection, document)` - Insert document
- `db.docs.find(collection, query)` - Find documents
- `db.docs.update(collection, doc_id, update)` - Update document
- `db.docs.delete(collection, doc_id)` - Delete document

### SQL Operations

- `db.sql.execute(query)` - Execute SQL query
- `db.sql.create_table(table, schema)` - Create table
- `db.sql.list_tables()` - List all tables

### Vector Operations

- `db.vectors.insert(collection, vector, metadata)` - Insert vector
- `db.vectors.search(collection, query_vector, k, filter)` - Search vectors
- `db.vectors.delete(collection, vector_id)` - Delete vector

## Development

```bash
# Install development dependencies
pip install -e ".[dev]"

# Run tests
pytest

# Format code
black mantisdb/

# Type check
mypy mantisdb/
```

## License

MIT
