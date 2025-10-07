# MantisDB Go Client

Official Go client library for MantisDB with connection pooling, transactions, and comprehensive error handling.

> **Full Documentation**: See [Go Client Documentation](../../docs/clients/go.md) for complete API reference and examples.

## Installation

```bash
go get github.com/mantisdb/mantisdb/clients/go
```

## Features

- Connection pooling and retry logic
- Full ACID transaction support
- Context-aware operations
- Type-safe interfaces
- Comprehensive error handling

## Quick Start

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

    client, err := mantisdb.NewClient(config)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    ctx := context.Background()

    // Insert data
    user := map[string]interface{}{
        "name":  "John Doe",
        "email": "john@example.com",
    }
    
    if err := client.Insert(ctx, "users", user); err != nil {
        log.Fatal(err)
    }

    // Query data
    result, err := client.Query(ctx, "SELECT * FROM users")
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Found %d users\n", result.RowCount)
}
```

## Documentation

For complete documentation including:
- Configuration options
- CRUD operations
- Transaction handling
- Error handling
- Best practices

See the [Go Client Documentation](../../docs/clients/go.md).

## License

MIT License - Part of the MantisDB project.