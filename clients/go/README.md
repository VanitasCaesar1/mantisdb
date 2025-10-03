# MantisDB Go Client

The official Go client library for MantisDB, providing a comprehensive SDK with connection pooling, error handling, and support for all database operations and transactions.

## Features

- **Connection Pooling**: Efficient HTTP connection management with configurable pool size
- **Error Handling**: Comprehensive error handling with retry logic and detailed error messages
- **Transaction Support**: Full ACID transaction support with commit/rollback operations
- **CRUD Operations**: Complete support for Create, Read, Update, Delete operations
- **Query Interface**: Execute raw SQL queries with structured results
- **Authentication**: Built-in support for basic authentication
- **Retry Logic**: Configurable retry mechanisms for resilient operations
- **Context Support**: Full context.Context support for cancellation and timeouts
- **Type Safety**: Strongly typed interfaces with comprehensive error handling

## Installation

```bash
go get github.com/mantisdb/mantisdb/clients/go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    mantisdb "github.com/mantisdb/mantisdb/clients/go"
)

func main() {
    // Create client configuration
    config := mantisdb.DefaultConfig()
    config.Host = "localhost"
    config.Port = 8080
    config.Username = "admin"
    config.Password = "password"

    // Create client
    client, err := mantisdb.NewClient(config)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    ctx := context.Background()

    // Test connection
    if err := client.Ping(ctx); err != nil {
        log.Fatal(err)
    }

    // Insert data
    user := map[string]interface{}{
        "name":  "John Doe",
        "email": "john@example.com",
        "age":   30,
    }
    
    if err := client.Insert(ctx, "users", user); err != nil {
        log.Fatal(err)
    }

    // Query data
    result, err := client.Query(ctx, "SELECT * FROM users WHERE age > 25")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Found %d users\n", result.RowCount)
    for _, row := range result.Rows {
        fmt.Printf("User: %s (%s)\n", row["name"], row["email"])
    }
}
```

## Configuration

The client supports extensive configuration options:

```go
config := &mantisdb.Config{
    Host:               "localhost",
    Port:               8080,
    Username:           "admin",
    Password:           "password",
    MaxConnections:     10,
    ConnectionTimeout:  30 * time.Second,
    RequestTimeout:     60 * time.Second,
    RetryAttempts:      3,
    RetryDelay:         1 * time.Second,
    EnableCompression:  true,
    TLSEnabled:         false,
}
```

## CRUD Operations

### Insert
```go
user := map[string]interface{}{
    "name":  "Jane Smith",
    "email": "jane@example.com",
    "age":   28,
}

err := client.Insert(ctx, "users", user)
```

### Query
```go
result, err := client.Query(ctx, "SELECT * FROM users WHERE age > 25")
if err != nil {
    log.Fatal(err)
}

for _, row := range result.Rows {
    fmt.Printf("User: %v\n", row)
}
```

### Get with Filters
```go
filters := map[string]interface{}{
    "age": 28,
    "active": true,
}

result, err := client.Get(ctx, "users", filters)
```

### Update
```go
updates := map[string]interface{}{
    "age": 29,
}

err := client.Update(ctx, "users", "user-id", updates)
```

### Delete
```go
err := client.Delete(ctx, "users", "user-id")
```

## Transactions

```go
// Begin transaction
tx, err := client.BeginTransaction(ctx)
if err != nil {
    log.Fatal(err)
}

// Insert data in transaction
user1 := map[string]interface{}{"name": "Alice", "age": 25}
user2 := map[string]interface{}{"name": "Bob", "age": 35}

if err := tx.Insert(ctx, "users", user1); err != nil {
    tx.Rollback(ctx)
    log.Fatal(err)
}

if err := tx.Insert(ctx, "users", user2); err != nil {
    tx.Rollback(ctx)
    log.Fatal(err)
}

// Commit transaction
if err := tx.Commit(ctx); err != nil {
    log.Fatal(err)
}
```

## Error Handling

The client provides detailed error information:

```go
result, err := client.Query(ctx, "INVALID SQL")
if err != nil {
    if mantisErr, ok := err.(*mantisdb.MantisError); ok {
        fmt.Printf("Error Code: %s\n", mantisErr.Code)
        fmt.Printf("Message: %s\n", mantisErr.Message)
        fmt.Printf("Request ID: %s\n", mantisErr.RequestID)
        fmt.Printf("Details: %v\n", mantisErr.Details)
    }
}
```

## Connection Pooling

The client automatically manages HTTP connections with configurable pooling:

- `MaxConnections`: Maximum number of idle connections
- `ConnectionTimeout`: Timeout for idle connections
- `RequestTimeout`: Timeout for individual requests

## Retry Logic

Built-in retry logic for resilient operations:

- `RetryAttempts`: Number of retry attempts (default: 3)
- `RetryDelay`: Delay between retries (default: 1 second)
- Exponential backoff for retry delays
- Automatic retry on 5xx server errors

## Thread Safety

The client is thread-safe and can be used concurrently from multiple goroutines. Connection pooling ensures efficient resource utilization across concurrent operations.

## Best Practices

1. **Reuse Client Instances**: Create one client instance and reuse it across your application
2. **Use Context**: Always pass context for cancellation and timeout control
3. **Handle Errors**: Check and handle all errors appropriately
4. **Close Resources**: Always call `client.Close()` when done
5. **Use Transactions**: Use transactions for multi-operation consistency
6. **Configure Timeouts**: Set appropriate timeouts for your use case

## License

This client library is part of the MantisDB project and follows the same license terms.