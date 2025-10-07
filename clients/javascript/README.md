# MantisDB JavaScript/TypeScript Client

Official JavaScript/TypeScript client library for MantisDB with full TypeScript support for Node.js and browser environments.

> **Full Documentation**: See [JavaScript Client Documentation](../../docs/clients/javascript.md) for complete API reference and examples.

## Installation

```bash
npm install mantisdb-js
```

## Features

- Universal compatibility (Node.js and browser)
- Full TypeScript definitions
- Promise-based async/await API
- Connection pooling and retry logic
- ACID transaction support
- Comprehensive error handling

## Quick Start

### JavaScript (Node.js)

```javascript
const { MantisClient } = require('mantisdb-js');

async function main() {
  const client = new MantisClient({
    host: 'localhost',
    port: 8080
  });

  try {
    // Insert data
    await client.insert('users', {
      name: 'John Doe',
      email: 'john@example.com'
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
import { MantisClient } from 'mantisdb-js';

interface User {
  name: string;
  email: string;
}

async function main(): Promise<void> {
  const client = new MantisClient({
    host: 'localhost',
    port: 8080
  });

  try {
    const user: User = {
      name: 'Jane Doe',
      email: 'jane@example.com'
    };

    await client.insert('users', user);
    const result = await client.query('SELECT * FROM users');
    console.log(`Found ${result.rowCount} users`);
  } finally {
    await client.close();
  }
}

main().catch(console.error);
```

## Documentation

For complete documentation including:
- Configuration options
- CRUD operations
- Transaction handling
- Error handling
- TypeScript support
- Connection pooling
- Best practices

See the [JavaScript Client Documentation](../../docs/clients/javascript.md).

## License

MIT License - Part of the MantisDB project.