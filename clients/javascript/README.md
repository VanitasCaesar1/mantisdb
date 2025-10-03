# MantisDB JavaScript/TypeScript Client

The official JavaScript/TypeScript client library for MantisDB, providing a comprehensive SDK that works in both Node.js and browser environments with full TypeScript support.

## Features

- **Universal Compatibility**: Works in Node.js and browser environments
- **TypeScript Support**: Full TypeScript definitions with comprehensive type safety
- **Promise-based API**: Modern async/await support with Promise-based operations
- **Connection Pooling**: Efficient HTTP connection management with configurable pool sizes
- **Error Handling**: Comprehensive error handling with detailed error information
- **Transaction Support**: Full ACID transaction support with automatic rollback
- **CRUD Operations**: Complete support for Create, Read, Update, Delete operations
- **Query Interface**: Execute raw SQL queries with structured results
- **Authentication**: Built-in support for basic authentication
- **Retry Logic**: Configurable retry mechanisms for resilient operations
- **Connection Strings**: Support for connection string parsing and building

## Installation

```bash
npm install mantisdb-js
```

### For TypeScript projects:
```bash
npm install mantisdb-js
# TypeScript definitions are included
```

## Quick Start

### JavaScript (Node.js)

```javascript
const { MantisClient } = require('mantisdb-js');

async function main() {
  const client = new MantisClient({
    host: 'localhost',
    port: 8080,
    username: 'admin',
    password: 'password'
  });

  try {
    // Test connection
    await client.ping();
    console.log('Connected!');

    // Insert data
    await client.insert('users', {
      name: 'John Doe',
      email: 'john@example.com',
      age: 30
    });

    // Query data
    const result = await client.query('SELECT * FROM users');
    console.log(`Found ${result.rowCount} users`);

  } finally {
    await client.close();
  }
}

main().catch(console.error);
```

### TypeScript

```typescript
import { MantisClient, ClientConfig, QueryResult } from 'mantisdb-js';

interface User {
  id?: string;
  name: string;
  email: string;
  age: number;
}

async function main(): Promise<void> {
  const config: ClientConfig = {
    host: 'localhost',
    port: 8080,
    username: 'admin',
    password: 'password',
    timeout: 30000
  };

  const client = new MantisClient(config);

  try {
    await client.ping();
    
    const user: Omit<User, 'id'> = {
      name: 'Jane Doe',
      email: 'jane@example.com',
      age: 28
    };

    await client.insert('users', user);

    const result: QueryResult = await client.query('SELECT * FROM users');
    const users: User[] = result.rows as User[];
    
    console.log(`Found ${users.length} users`);

  } finally {
    await client.close();
  }
}
```

### Browser Usage

```html
<!DOCTYPE html>
<html>
<head>
  <script src="https://unpkg.com/mantisdb-js/dist/index.js"></script>
</head>
<body>
  <script>
    async function main() {
      const client = new MantisDB.MantisClient({
        host: 'localhost',
        port: 8080,
        username: 'admin',
        password: 'password'
      });

      try {
        await client.ping();
        console.log('Connected from browser!');
        
        const result = await client.query('SELECT * FROM users');
        console.log('Users:', result.rows);
      } finally {
        await client.close();
      }
    }

    main().catch(console.error);
  </script>
</body>
</html>
```

## Configuration

### ClientConfig Interface

```typescript
interface ClientConfig {
  host: string;                    // Database server hostname
  port: number;                    // Database server port
  username?: string;               // Authentication username
  password?: string;               // Authentication password
  maxConnections?: number;         // Max connections in pool (default: 10)
  timeout?: number;                // Request timeout in ms (default: 60000)
  retryAttempts?: number;          // Number of retry attempts (default: 3)
  retryDelay?: number;             // Delay between retries in ms (default: 1000)
  enableCompression?: boolean;     // Enable HTTP compression (default: true)
  tlsEnabled?: boolean;            // Enable TLS/SSL (default: false)
}
```

### Example Configuration

```javascript
const config = {
  host: 'localhost',
  port: 8080,
  username: 'admin',
  password: 'password',
  maxConnections: 20,
  timeout: 45000,
  retryAttempts: 5,
  retryDelay: 2000,
  enableCompression: true,
  tlsEnabled: false
};

const client = new MantisClient(config);
```

## Connection Strings

You can also use connection strings for configuration:

```javascript
const { MantisClient, parseConnectionString } = require('mantisdb-js');

// Parse connection string
const config = parseConnectionString('mantisdb://admin:password@localhost:8080?timeout=30000');
const client = new MantisClient(config);

// Or create connection string from config
const { buildConnectionString } = require('mantisdb-js');
const connectionString = buildConnectionString(config);
```

### Connection String Format

```
mantisdb://username:password@host:port?param=value&param2=value2
mantisdbs://username:password@host:port  // For TLS connections
```

## CRUD Operations

### Insert

```javascript
// Insert single record
const user = {
  name: 'Alice Smith',
  email: 'alice@example.com',
  age: 25,
  active: true
};

await client.insert('users', user);
```

### Query

```javascript
// Execute SQL query
const result = await client.query('SELECT * FROM users WHERE age > 25 ORDER BY name');

console.log(`Columns: ${result.columns}`);
console.log(`Row count: ${result.rowCount}`);

result.rows.forEach(row => {
  console.log(`User: ${row.name} - Age: ${row.age}`);
});
```

### Get with Filters

```javascript
// Get data with filters (converted to WHERE clause)
const filters = {
  age: 25,
  active: true
};

const result = await client.get('users', filters);
```

### Update

```javascript
// Update record
const updates = {
  age: 26,
  lastLogin: new Date().toISOString()
};

await client.update('users', 'user-id-123', updates);
```

### Delete

```javascript
// Delete record
await client.delete('users', 'user-id-123');
```

## Transactions

### Basic Transaction Usage

```javascript
// Begin transaction
const tx = await client.beginTransaction();

try {
  await tx.insert('users', { name: 'User 1', email: 'user1@example.com' });
  await tx.insert('users', { name: 'User 2', email: 'user2@example.com' });
  
  // Query within transaction
  const result = await tx.query('SELECT COUNT(*) as count FROM users');
  console.log(`Total users: ${result.rows[0].count}`);
  
  await tx.commit();
  console.log('Transaction committed');
  
} catch (error) {
  if (!tx.isClosed) {
    await tx.rollback();
    console.log('Transaction rolled back');
  }
  throw error;
}
```

### Transaction Properties

```javascript
const tx = await client.beginTransaction();

console.log(`Transaction ID: ${tx.transactionId}`);
console.log(`Is closed: ${tx.isClosed}`);

// Available methods
await tx.query(sql);
await tx.insert(table, data);
await tx.update(table, id, data);
await tx.delete(table, id);
await tx.commit();
await tx.rollback();
```

## Error Handling

The client provides detailed error information through the `MantisError` class:

```javascript
const { MantisError } = require('mantisdb-js');

try {
  await client.query('INVALID SQL SYNTAX');
} catch (error) {
  if (error instanceof MantisError) {
    console.error(`Error Code: ${error.code}`);
    console.error(`Message: ${error.message}`);
    console.error(`Request ID: ${error.requestId}`);
    console.error(`Details:`, error.details);
  } else {
    console.error('Unexpected error:', error.message);
  }
}
```

### MantisError Properties

```typescript
class MantisError extends Error {
  readonly code: string;           // Error code (e.g., 'INVALID_QUERY')
  readonly details?: object;       // Additional error details
  readonly requestId?: string;     // Request ID for tracking
}
```

## TypeScript Support

### Full Type Safety

```typescript
import { MantisClient, QueryResult, Transaction } from 'mantisdb-js';

// Define your data interfaces
interface User {
  id: string;
  name: string;
  email: string;
  age: number;
  active: boolean;
}

// Type-safe operations
const client = new MantisClient(config);
const result: QueryResult = await client.query('SELECT * FROM users');
const users: User[] = result.rows as User[];

// Type-safe transactions
const tx: Transaction = await client.beginTransaction();
await tx.insert('users', userData);
```

### Generic Repository Pattern

```typescript
class Repository<T extends Record<string, any>> {
  constructor(
    private client: MantisClient,
    private tableName: string
  ) {}

  async create(data: Omit<T, 'id'>): Promise<void> {
    await this.client.insert(this.tableName, data);
  }

  async findAll(): Promise<T[]> {
    const result = await this.client.query(`SELECT * FROM ${this.tableName}`);
    return result.rows as T[];
  }

  async findById(id: string): Promise<T | null> {
    const result = await this.client.query(
      `SELECT * FROM ${this.tableName} WHERE id = '${id}'`
    );
    return result.rows.length > 0 ? result.rows[0] as T : null;
  }
}

// Usage
const userRepo = new Repository<User>(client, 'users');
const users: User[] = await userRepo.findAll();
```

## Utility Functions

### Configuration Validation

```javascript
const { validateConfig } = require('mantisdb-js');

const validation = validateConfig(config);
if (!validation.valid) {
  console.error('Configuration errors:', validation.errors);
}
if (validation.warnings.length > 0) {
  console.warn('Configuration warnings:', validation.warnings);
}
```

### Connection String Utilities

```javascript
const { parseConnectionString, buildConnectionString } = require('mantisdb-js');

// Parse connection string
const config = parseConnectionString('mantisdb://user:pass@localhost:8080');

// Build connection string
const connectionString = buildConnectionString(config);
```

### Environment Detection

```javascript
const { isNodeEnvironment, isBrowserEnvironment } = require('mantisdb-js');

if (isNodeEnvironment()) {
  console.log('Running in Node.js');
} else if (isBrowserEnvironment()) {
  console.log('Running in browser');
}
```

## Advanced Features

### Retry Logic with Exponential Backoff

```javascript
const { retryWithBackoff } = require('mantisdb-js');

const result = await retryWithBackoff(
  () => client.query('SELECT * FROM users'),
  3,    // max attempts
  1000, // base delay
  5000  // max delay
);
```

### Request Timeout

```javascript
const { withTimeout } = require('mantisdb-js');

try {
  const result = await withTimeout(
    client.query('LONG_RUNNING_QUERY'),
    10000 // 10 second timeout
  );
} catch (error) {
  console.error('Query timed out');
}
```

### Performance Measurement

```javascript
const { measureTime } = require('mantisdb-js');

const { result, duration } = await measureTime(async () => {
  return await client.query('SELECT * FROM large_table');
});

console.log(`Query completed in ${duration}ms`);
```

## Best Practices

1. **Reuse Client Instances**: Create one client instance and reuse it
2. **Handle Errors Properly**: Always implement comprehensive error handling
3. **Use Transactions**: Use transactions for multi-operation consistency
4. **Close Resources**: Always close clients when done
5. **Configure Timeouts**: Set appropriate timeouts for your use case
6. **Type Safety**: Use TypeScript for better development experience
7. **Connection Pooling**: Configure connection pools based on your needs

## Examples

Check out the `examples/` directory for comprehensive usage examples:

- `basic-usage.js` - Basic CRUD operations
- `transactions.js` - Transaction management examples
- `typescript-example.ts` - TypeScript usage patterns

## Browser Compatibility

The client works in modern browsers that support:
- ES2017+ (async/await)
- Fetch API or XMLHttpRequest
- Promise support

For older browsers, you may need polyfills.

## Node.js Compatibility

- Node.js 16.0.0 or higher
- Built-in support for HTTP/HTTPS
- Full TypeScript support

## Building from Source

```bash
git clone https://github.com/mantisdb/mantisdb.git
cd mantisdb/clients/javascript
npm install
npm run build
```

## Testing

```bash
npm test
```

## License

This client library is part of the MantisDB project and follows the same license terms.