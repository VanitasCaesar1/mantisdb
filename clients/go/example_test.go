package mantisdb_test

import (
	"context"
	"fmt"
	"log"
	"time"

	mantisdb "github.com/mantisdb/mantisdb/clients/go"
)

func ExampleClient_basic() {
	// Create a client with default configuration
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

	// Test connection
	if err := client.Ping(ctx); err != nil {
		log.Fatal(err)
	}

	// Insert data
	userData := map[string]interface{}{
		"name":  "John Doe",
		"email": "john@example.com",
		"age":   30,
	}

	if err := client.Insert(ctx, "users", userData); err != nil {
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

	// Output:
	// Found 1 users
	// User: John Doe (john@example.com)
}

func ExampleClient_transaction() {
	config := mantisdb.DefaultConfig()
	client, err := mantisdb.NewClient(config)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()

	// Begin transaction
	tx, err := client.BeginTransaction(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Insert multiple records in transaction
	users := []map[string]interface{}{
		{"name": "Alice", "email": "alice@example.com", "age": 25},
		{"name": "Bob", "email": "bob@example.com", "age": 35},
	}

	for _, user := range users {
		if err := tx.Insert(ctx, "users", user); err != nil {
			tx.Rollback(ctx)
			log.Fatal(err)
		}
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Transaction completed successfully")
	// Output: Transaction completed successfully
}

func ExampleClient_withRetry() {
	config := mantisdb.DefaultConfig()
	config.RetryAttempts = 5
	config.RetryDelay = 2 * time.Second
	config.RequestTimeout = 30 * time.Second

	client, err := mantisdb.NewClient(config)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()

	// This will automatically retry on failure
	result, err := client.Query(ctx, "SELECT COUNT(*) as total FROM users")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Total users: %v\n", result.Rows[0]["total"])
}

func ExampleClient_crud() {
	config := mantisdb.DefaultConfig()
	client, err := mantisdb.NewClient(config)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()

	// Create
	user := map[string]interface{}{
		"name":  "Jane Smith",
		"email": "jane@example.com",
		"age":   28,
	}

	if err := client.Insert(ctx, "users", user); err != nil {
		log.Fatal(err)
	}

	// Read with filters
	filters := map[string]interface{}{
		"name": "Jane Smith",
	}

	result, err := client.Get(ctx, "users", filters)
	if err != nil {
		log.Fatal(err)
	}

	if len(result.Rows) > 0 {
		userID := result.Rows[0]["id"].(string)

		// Update
		updates := map[string]interface{}{
			"age": 29,
		}

		if err := client.Update(ctx, "users", userID, updates); err != nil {
			log.Fatal(err)
		}

		// Delete
		if err := client.Delete(ctx, "users", userID); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Println("CRUD operations completed")
	// Output: CRUD operations completed
}
