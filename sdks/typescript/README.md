# @mantisdb/client

Official TypeScript/JavaScript SDK for MantisDB multimodal database with full type safety.

## Installation

```bash
npm install @mantisdb/client
# or
yarn add @mantisdb/client
```

## Quick Start

```typescript
import { MantisDB } from '@mantisdb/client';

// Connect to MantisDB
const db = new MantisDB('http://localhost:8080');

// Key-Value operations
await db.kv.set('user:123', 'John Doe');
const name = await db.kv.get('user:123');

// Document operations  
await db.docs.insert('users', { name: 'Alice', age: 30 });
const users = await db.docs.find('users', { age: { $gt: 25 } });

// SQL operations
const results = await db.sql.execute('SELECT * FROM users WHERE age > 25');

// Vector operations
await db.vectors.insert('embeddings', [0.1, 0.2, 0.3], { text: 'hello' });
const similar = await db.vectors.search('embeddings', [0.15, 0.25, 0.35], 10);

// Query builder
const query = db.query()
  .select('name', 'email')
  .from('users')
  .where('age > 25')
  .orderBy('name')
  .limit(10)
  .execute();
```

## Features

- ✅ **Full TypeScript Support** - Complete type definitions and IntelliSense
- ✅ **Promise-Based API** - Modern async/await syntax
- ✅ **Type-Safe Query Builder** - Fluent API with compile-time safety
- ✅ **All Database Types** - KV, Document, SQL, Columnar, Vector
- ✅ **Connection Management** - Automatic retry and timeout handling
- ✅ **Zero Dependencies** - Only axios for HTTP (can be swapped)

## API Reference

### Configuration

```typescript
import { MantisDB, MantisDBConfig } from '@mantisdb/client';

const config: MantisDBConfig = {
  baseURL: 'http://localhost:8080',
  authToken: 'your-token',  // optional
  timeout: 30000            // optional, default 30s
};

const db = new MantisDB(config);
```

### Key-Value Operations

```typescript
// Set with optional TTL
await db.kv.set('key', 'value', 3600);  // expires in 1 hour
const value = await db.kv.get('key');
await db.kv.delete('key');
const exists = await db.kv.exists('key');
```

### Document Operations

```typescript
const id = await db.docs.insert('users', {
  name: 'Bob',
  email: 'bob@example.com',
  age: 25
});

const users = await db.docs.find('users', {
  age: { $gte: 18, $lt: 65 },
  status: 'active'
});

await db.docs.update('users', id, { age: 26 });
await db.docs.delete('users', id);
```

### SQL Operations

```typescript
// Execute queries
const results = await db.sql.execute(
  `SELECT u.name, o.total 
   FROM users u 
   JOIN orders o ON u.id = o.user_id 
   WHERE o.total > 100`
);

// Manage tables
await db.sql.createTable('products', {
  id: 'INTEGER PRIMARY KEY',
  name: 'TEXT',
  price: 'REAL'
});

const tables = await db.sql.listTables();
```

### Vector Operations

```typescript
// Insert vectors with metadata
const id = await db.vectors.insert(
  'embeddings',
  [0.1, 0.2, 0.3, 0.4],
  { text: 'hello world', category: 'greeting' }
);

// Search with optional filtering
const results = await db.vectors.search(
  'embeddings',
  [0.15, 0.25, 0.35, 0.45],
  10,  // k nearest neighbors
  { category: 'greeting' }  // metadata filter
);

// Results include id, score, metadata, and optionally the vector
results.forEach(result => {
  console.log(`ID: ${result.id}, Score: ${result.score}`);
  console.log(`Metadata:`, result.metadata);
});
```

### Query Builder

```typescript
const query = db.query()
  .select('name', 'email', 'created_at')
  .from('users')
  .where('age >= 18')
  .where('status = "active"')
  .orderBy('created_at', 'DESC')
  .limit(20)
  .offset(40);

// Build SQL string
const sql = query.build();

// Or execute directly
const results = await query.execute();
```

### Monitoring

```typescript
// Health check
const health = await db.health();
console.log(health.status);  // "healthy"

// Detailed metrics
const metrics = await db.metrics();
console.log(`QPS: ${metrics.overview.queries_per_second}`);
console.log(`P95 Latency: ${metrics.performance.p95_latency_ms}ms`);
console.log(`Cache Hit Ratio: ${metrics.performance.cache_hit_ratio}`);
```

## Development

```bash
# Install dependencies
npm install

# Build
npm run build

# Run tests
npm test

# Lint
npm run lint
```

## License

MIT
