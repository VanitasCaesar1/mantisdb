package mantisdb_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	mantisdb "github.com/mantisdb/mantisdb/clients/go"
)

// Test configuration
var (
	testHost     = getEnv("MANTISDB_TEST_HOST", "localhost")
	testPort     = getEnvInt("MANTISDB_TEST_PORT", 8080)
	testUsername = getEnv("MANTISDB_TEST_USERNAME", "admin")
	testPassword = getEnv("MANTISDB_TEST_PASSWORD", "password")
	testAPIKey   = getEnv("MANTISDB_TEST_API_KEY", "")
)

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func createTestClient(t *testing.T) *mantisdb.Client {
	config := mantisdb.DefaultConfig()
	config.Host = testHost
	config.Port = testPort
	config.Username = testUsername
	config.Password = testPassword
	config.RequestTimeout = 10 * time.Second
	config.ConnectionTimeout = 5 * time.Second

	client, err := mantisdb.NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	return client
}

func TestClientConnection(t *testing.T) {
	client := createTestClient(t)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Ping(ctx)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestClientBasicOperations(t *testing.T) {
	client := createTestClient(t)
	defer client.Close()

	ctx := context.Background()
	tableName := "test_users_" + fmt.Sprintf("%d", time.Now().Unix())

	// Create table
	createTableSQL := fmt.Sprintf(`
		CREATE TABLE %s (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT UNIQUE,
			age INTEGER,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`, tableName)

	_, err := client.Query(ctx, createTableSQL)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert data
	userData := map[string]interface{}{
		"name":  "John Doe",
		"email": "john@example.com",
		"age":   30,
	}

	err = client.Insert(ctx, tableName, userData)
	if err != nil {
		t.Fatalf("Failed to insert data: %v", err)
	}

	// Query data
	result, err := client.Query(ctx, fmt.Sprintf("SELECT * FROM %s WHERE name = 'John Doe'", tableName))
	if err != nil {
		t.Fatalf("Failed to query data: %v", err)
	}

	if result.RowCount != 1 {
		t.Fatalf("Expected 1 row, got %d", result.RowCount)
	}

	if result.Rows[0]["name"] != "John Doe" {
		t.Fatalf("Expected name 'John Doe', got %v", result.Rows[0]["name"])
	}

	// Get data with filters
	filters := map[string]interface{}{
		"age": 30,
	}

	result, err = client.Get(ctx, tableName, filters)
	if err != nil {
		t.Fatalf("Failed to get data with filters: %v", err)
	}

	if result.RowCount != 1 {
		t.Fatalf("Expected 1 row with filters, got %d", result.RowCount)
	}

	// Update data
	userID := fmt.Sprintf("%v", result.Rows[0]["id"])
	updateData := map[string]interface{}{
		"age": 31,
	}

	err = client.Update(ctx, tableName, userID, updateData)
	if err != nil {
		t.Fatalf("Failed to update data: %v", err)
	}

	// Verify update
	result, err = client.Query(ctx, fmt.Sprintf("SELECT age FROM %s WHERE id = %s", tableName, userID))
	if err != nil {
		t.Fatalf("Failed to verify update: %v", err)
	}

	if fmt.Sprintf("%v", result.Rows[0]["age"]) != "31" {
		t.Fatalf("Expected age 31, got %v", result.Rows[0]["age"])
	}

	// Delete data
	err = client.Delete(ctx, tableName, userID)
	if err != nil {
		t.Fatalf("Failed to delete data: %v", err)
	}

	// Verify deletion
	result, err = client.Query(ctx, fmt.Sprintf("SELECT COUNT(*) as count FROM %s", tableName))
	if err != nil {
		t.Fatalf("Failed to verify deletion: %v", err)
	}

	if fmt.Sprintf("%v", result.Rows[0]["count"]) != "0" {
		t.Fatalf("Expected 0 rows after deletion, got %v", result.Rows[0]["count"])
	}

	// Clean up
	_, err = client.Query(ctx, fmt.Sprintf("DROP TABLE %s", tableName))
	if err != nil {
		t.Logf("Warning: Failed to drop test table: %v", err)
	}
}

func TestClientTransactions(t *testing.T) {
	client := createTestClient(t)
	defer client.Close()

	ctx := context.Background()
	tableName := "test_transactions_" + fmt.Sprintf("%d", time.Now().Unix())

	// Create table
	createTableSQL := fmt.Sprintf(`
		CREATE TABLE %s (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			balance INTEGER DEFAULT 0
		)
	`, tableName)

	_, err := client.Query(ctx, createTableSQL)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Test successful transaction
	tx, err := client.BeginTransaction(ctx)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Insert data in transaction
	userData1 := map[string]interface{}{
		"name":    "Alice",
		"balance": 1000,
	}
	userData2 := map[string]interface{}{
		"name":    "Bob",
		"balance": 500,
	}

	err = tx.Insert(ctx, tableName, userData1)
	if err != nil {
		tx.Rollback(ctx)
		t.Fatalf("Failed to insert in transaction: %v", err)
	}

	err = tx.Insert(ctx, tableName, userData2)
	if err != nil {
		tx.Rollback(ctx)
		t.Fatalf("Failed to insert in transaction: %v", err)
	}

	// Query within transaction
	result, err := tx.Query(ctx, fmt.Sprintf("SELECT COUNT(*) as count FROM %s", tableName))
	if err != nil {
		tx.Rollback(ctx)
		t.Fatalf("Failed to query in transaction: %v", err)
	}

	if fmt.Sprintf("%v", result.Rows[0]["count"]) != "2" {
		tx.Rollback(ctx)
		t.Fatalf("Expected 2 rows in transaction, got %v", result.Rows[0]["count"])
	}

	// Commit transaction
	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	// Verify data persisted
	result, err = client.Query(ctx, fmt.Sprintf("SELECT COUNT(*) as count FROM %s", tableName))
	if err != nil {
		t.Fatalf("Failed to verify committed data: %v", err)
	}

	if fmt.Sprintf("%v", result.Rows[0]["count"]) != "2" {
		t.Fatalf("Expected 2 rows after commit, got %v", result.Rows[0]["count"])
	}

	// Test rollback transaction
	tx2, err := client.BeginTransaction(ctx)
	if err != nil {
		t.Fatalf("Failed to begin second transaction: %v", err)
	}

	userData3 := map[string]interface{}{
		"name":    "Charlie",
		"balance": 750,
	}

	err = tx2.Insert(ctx, tableName, userData3)
	if err != nil {
		tx2.Rollback(ctx)
		t.Fatalf("Failed to insert in second transaction: %v", err)
	}

	// Rollback transaction
	err = tx2.Rollback(ctx)
	if err != nil {
		t.Fatalf("Failed to rollback transaction: %v", err)
	}

	// Verify data was not persisted
	result, err = client.Query(ctx, fmt.Sprintf("SELECT COUNT(*) as count FROM %s", tableName))
	if err != nil {
		t.Fatalf("Failed to verify rollback: %v", err)
	}

	if fmt.Sprintf("%v", result.Rows[0]["count"]) != "2" {
		t.Fatalf("Expected 2 rows after rollback, got %v", result.Rows[0]["count"])
	}

	// Clean up
	_, err = client.Query(ctx, fmt.Sprintf("DROP TABLE %s", tableName))
	if err != nil {
		t.Logf("Warning: Failed to drop test table: %v", err)
	}
}

func TestClientAuthentication(t *testing.T) {
	// Test Basic Auth
	t.Run("BasicAuth", func(t *testing.T) {
		config := mantisdb.DefaultConfig()
		config.Host = testHost
		config.Port = testPort
		config.Username = testUsername
		config.Password = testPassword

		client, err := mantisdb.NewClient(config)
		if err != nil {
			t.Fatalf("Failed to create client with basic auth: %v", err)
		}
		defer client.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = client.Ping(ctx)
		if err != nil {
			t.Fatalf("Basic auth ping failed: %v", err)
		}
	})

	// Test API Key Auth (if API key is provided)
	if testAPIKey != "" {
		t.Run("APIKeyAuth", func(t *testing.T) {
			config := mantisdb.DefaultConfig()
			config.Host = testHost
			config.Port = testPort
			config.APIKey = testAPIKey

			client, err := mantisdb.NewClient(config)
			if err != nil {
				t.Fatalf("Failed to create client with API key auth: %v", err)
			}
			defer client.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err = client.Ping(ctx)
			if err != nil {
				t.Fatalf("API key auth ping failed: %v", err)
			}
		})
	}
}

func TestClientConcurrency(t *testing.T) {
	client := createTestClient(t)
	defer client.Close()

	ctx := context.Background()
	tableName := "test_concurrency_" + fmt.Sprintf("%d", time.Now().Unix())

	// Create table
	createTableSQL := fmt.Sprintf(`
		CREATE TABLE %s (
			id INTEGER PRIMARY KEY,
			worker_id INTEGER,
			value INTEGER
		)
	`, tableName)

	_, err := client.Query(ctx, createTableSQL)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Test concurrent operations
	numWorkers := 10
	numOperationsPerWorker := 5
	var wg sync.WaitGroup
	errors := make(chan error, numWorkers*numOperationsPerWorker)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < numOperationsPerWorker; j++ {
				data := map[string]interface{}{
					"worker_id": workerID,
					"value":     j,
				}

				if err := client.Insert(ctx, tableName, data); err != nil {
					errors <- fmt.Errorf("worker %d operation %d failed: %v", workerID, j, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent operation error: %v", err)
	}

	// Verify all data was inserted
	result, err := client.Query(ctx, fmt.Sprintf("SELECT COUNT(*) as count FROM %s", tableName))
	if err != nil {
		t.Fatalf("Failed to count inserted rows: %v", err)
	}

	expectedCount := numWorkers * numOperationsPerWorker
	actualCount := fmt.Sprintf("%v", result.Rows[0]["count"])
	if actualCount != fmt.Sprintf("%d", expectedCount) {
		t.Fatalf("Expected %d rows, got %s", expectedCount, actualCount)
	}

	// Clean up
	_, err = client.Query(ctx, fmt.Sprintf("DROP TABLE %s", tableName))
	if err != nil {
		t.Logf("Warning: Failed to drop test table: %v", err)
	}
}

func TestClientErrorHandling(t *testing.T) {
	client := createTestClient(t)
	defer client.Close()

	ctx := context.Background()

	// Test invalid SQL
	_, err := client.Query(ctx, "INVALID SQL STATEMENT")
	if err == nil {
		t.Fatal("Expected error for invalid SQL, got nil")
	}

	mantisErr, ok := err.(*mantisdb.MantisError)
	if !ok {
		t.Fatalf("Expected MantisError, got %T", err)
	}

	if mantisErr.Code == "" {
		t.Fatal("Expected error code, got empty string")
	}

	// Test non-existent table
	_, err = client.Query(ctx, "SELECT * FROM non_existent_table")
	if err == nil {
		t.Fatal("Expected error for non-existent table, got nil")
	}

	// Test invalid insert
	err = client.Insert(ctx, "non_existent_table", map[string]interface{}{"field": "value"})
	if err == nil {
		t.Fatal("Expected error for insert to non-existent table, got nil")
	}
}

func TestClientRetryMechanism(t *testing.T) {
	// Create client with retry configuration
	config := mantisdb.DefaultConfig()
	config.Host = testHost
	config.Port = testPort
	config.Username = testUsername
	config.Password = testPassword
	config.RetryAttempts = 3
	config.RetryDelay = 100 * time.Millisecond

	client, err := mantisdb.NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Test that normal operations work with retry enabled
	err = client.Ping(ctx)
	if err != nil {
		t.Fatalf("Ping failed with retry enabled: %v", err)
	}
}

func TestClientConnectionPooling(t *testing.T) {
	config := mantisdb.DefaultConfig()
	config.Host = testHost
	config.Port = testPort
	config.Username = testUsername
	config.Password = testPassword
	config.MaxConnections = 5

	client, err := mantisdb.NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Get connection stats
	stats := client.GetConnectionStats()
	if stats.MaxConnections != 5 {
		t.Fatalf("Expected max connections 5, got %d", stats.MaxConnections)
	}
}

func TestClientHealthCheck(t *testing.T) {
	client := createTestClient(t)
	defer client.Close()

	ctx := context.Background()

	healthResult, err := client.HealthCheck(ctx)
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}

	if healthResult.Status != "healthy" && healthResult.Status != "degraded" {
		t.Fatalf("Expected healthy or degraded status, got %s", healthResult.Status)
	}

	if healthResult.Host != testHost {
		t.Fatalf("Expected host %s, got %s", testHost, healthResult.Host)
	}

	if healthResult.Port != testPort {
		t.Fatalf("Expected port %d, got %d", testPort, healthResult.Port)
	}

	if healthResult.Duration <= 0 {
		t.Fatal("Expected positive duration")
	}
}

func TestClientFailover(t *testing.T) {
	// Only test if failover hosts are configured
	if len(os.Getenv("MANTISDB_TEST_FAILOVER_HOSTS")) == 0 {
		t.Skip("Skipping failover test - no failover hosts configured")
	}

	config := mantisdb.DefaultConfig()
	config.Host = testHost
	config.Port = testPort
	config.Username = testUsername
	config.Password = testPassword
	config.EnableFailover = true
	config.FailoverHosts = []string{"localhost:8081", "localhost:8082"}

	client, err := mantisdb.NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client with failover: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Test normal operation
	err = client.Ping(ctx)
	if err != nil {
		t.Fatalf("Initial ping failed: %v", err)
	}

	// Test failover (this will fail if failover hosts are not available)
	err = client.Failover(ctx)
	if err != nil {
		t.Logf("Failover failed as expected: %v", err)
	}
}

// Benchmark tests
func BenchmarkClientQuery(b *testing.B) {
	client := createTestClient(&testing.T{})
	defer client.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.Query(ctx, "SELECT 1")
		if err != nil {
			b.Fatalf("Query failed: %v", err)
		}
	}
}

func BenchmarkClientInsert(b *testing.B) {
	client := createTestClient(&testing.T{})
	defer client.Close()

	ctx := context.Background()
	tableName := "bench_test_" + fmt.Sprintf("%d", time.Now().Unix())

	// Create table
	createTableSQL := fmt.Sprintf(`
		CREATE TABLE %s (
			id INTEGER PRIMARY KEY,
			name TEXT,
			value INTEGER
		)
	`, tableName)

	_, err := client.Query(ctx, createTableSQL)
	if err != nil {
		b.Fatalf("Failed to create table: %v", err)
	}

	data := map[string]interface{}{
		"name":  "test",
		"value": 42,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := client.Insert(ctx, tableName, data)
		if err != nil {
			b.Fatalf("Insert failed: %v", err)
		}
	}

	// Clean up
	client.Query(ctx, fmt.Sprintf("DROP TABLE %s", tableName))
}

func BenchmarkClientConcurrentQueries(b *testing.B) {
	client := createTestClient(&testing.T{})
	defer client.Close()

	ctx := context.Background()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := client.Query(ctx, "SELECT 1")
			if err != nil {
				b.Fatalf("Concurrent query failed: %v", err)
			}
		}
	})
}
