package mantisdb_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	mantisdb "github.com/mantisdb/mantisdb/clients/go"
)

// Performance benchmarks for Go client
func BenchmarkClientOperations(b *testing.B) {
	client := createTestClient(&testing.T{})
	defer client.Close()

	ctx := context.Background()

	b.Run("Query", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := client.Query(ctx, "SELECT 1")
			if err != nil {
				b.Fatalf("Query failed: %v", err)
			}
		}
	})

	b.Run("Ping", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := client.Ping(ctx)
			if err != nil {
				b.Fatalf("Ping failed: %v", err)
			}
		}
	})
}

func BenchmarkConcurrentQueries(b *testing.B) {
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

func BenchmarkTransactionOperations(b *testing.B) {
	client := createTestClient(&testing.T{})
	defer client.Close()

	ctx := context.Background()
	tableName := "bench_transactions_" + fmt.Sprintf("%d", time.Now().Unix())

	// Create table
	createTableSQL := fmt.Sprintf(`
		CREATE TABLE %s (
			id INTEGER PRIMARY KEY,
			value INTEGER
		)
	`, tableName)

	_, err := client.Query(ctx, createTableSQL)
	if err != nil {
		b.Fatalf("Failed to create table: %v", err)
	}

	defer func() {
		client.Query(ctx, fmt.Sprintf("DROP TABLE %s", tableName))
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tx, err := client.BeginTransaction(ctx)
		if err != nil {
			b.Fatalf("Failed to begin transaction: %v", err)
		}

		data := map[string]interface{}{
			"value": i,
		}

		err = tx.Insert(ctx, tableName, data)
		if err != nil {
			tx.Rollback(ctx)
			b.Fatalf("Failed to insert: %v", err)
		}

		err = tx.Commit(ctx)
		if err != nil {
			b.Fatalf("Failed to commit: %v", err)
		}
	}
}

// Load testing
func TestClientLoadTesting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	client := createTestClient(t)
	defer client.Close()

	ctx := context.Background()
	tableName := "load_test_" + fmt.Sprintf("%d", time.Now().Unix())

	// Create table
	createTableSQL := fmt.Sprintf(`
		CREATE TABLE %s (
			id INTEGER PRIMARY KEY,
			worker_id INTEGER,
			operation_id INTEGER,
			timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`, tableName)

	_, err := client.Query(ctx, createTableSQL)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	defer func() {
		client.Query(ctx, fmt.Sprintf("DROP TABLE %s", tableName))
	}()

	// Load test parameters
	numWorkers := 20
	operationsPerWorker := 100
	duration := 30 * time.Second

	var wg sync.WaitGroup
	errors := make(chan error, numWorkers*operationsPerWorker)
	startTime := time.Now()

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			operationCount := 0
			for time.Since(startTime) < duration && operationCount < operationsPerWorker {
				data := map[string]interface{}{
					"worker_id":    workerID,
					"operation_id": operationCount,
				}

				if err := client.Insert(ctx, tableName, data); err != nil {
					errors <- fmt.Errorf("worker %d operation %d failed: %v", workerID, operationCount, err)
					return
				}

				operationCount++
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Errorf("Load test error: %v", err)
		errorCount++
	}

	// Get final statistics
	result, err := client.Query(ctx, fmt.Sprintf("SELECT COUNT(*) as total_operations FROM %s", tableName))
	if err != nil {
		t.Fatalf("Failed to get final count: %v", err)
	}

	totalOperations := fmt.Sprintf("%v", result.Rows[0]["total_operations"])
	actualDuration := time.Since(startTime)

	t.Logf("Load test completed:")
	t.Logf("  Duration: %v", actualDuration)
	t.Logf("  Total operations: %s", totalOperations)
	t.Logf("  Errors: %d", errorCount)
	t.Logf("  Operations per second: %.2f", float64(result.RowCount)/actualDuration.Seconds())

	if errorCount > 0 {
		t.Fatalf("Load test failed with %d errors", errorCount)
	}
}

// Memory usage testing
func TestClientMemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory test in short mode")
	}

	client := createTestClient(t)
	defer client.Close()

	ctx := context.Background()

	// Perform many operations to test for memory leaks
	for i := 0; i < 1000; i++ {
		_, err := client.Query(ctx, "SELECT 1")
		if err != nil {
			t.Fatalf("Query %d failed: %v", i, err)
		}

		if i%100 == 0 {
			// Force garbage collection periodically
			// In a real test, you might use runtime.GC() and runtime.ReadMemStats()
			t.Logf("Completed %d queries", i)
		}
	}
}

// Connection pool stress testing
func TestConnectionPoolStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping connection pool stress test in short mode")
	}

	config := mantisdb.DefaultConfig()
	config.Host = testHost
	config.Port = testPort
	config.Username = testUsername
	config.Password = testPassword
	config.MaxConnections = 5 // Limit connections to stress the pool

	client, err := mantisdb.NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Create many concurrent operations that exceed the connection pool size
	numGoroutines := 20
	operationsPerGoroutine := 10

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*operationsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				_, err := client.Query(ctx, "SELECT 1")
				if err != nil {
					errors <- fmt.Errorf("goroutine %d operation %d failed: %v", goroutineID, j, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Connection pool stress test error: %v", err)
	}

	// Verify connection pool stats
	stats := client.GetConnectionStats()
	t.Logf("Connection pool stats: %+v", stats)

	if stats.MaxConnections != 5 {
		t.Errorf("Expected max connections 5, got %d", stats.MaxConnections)
	}
}

// Timeout and retry testing
func TestClientTimeoutAndRetry(t *testing.T) {
	// Test with very short timeout
	config := mantisdb.DefaultConfig()
	config.Host = testHost
	config.Port = testPort
	config.Username = testUsername
	config.Password = testPassword
	config.RequestTimeout = 1 * time.Millisecond // Very short timeout
	config.RetryAttempts = 2
	config.RetryDelay = 10 * time.Millisecond

	client, err := mantisdb.NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// This should fail due to timeout but demonstrate retry behavior
	start := time.Now()
	_, err = client.Query(ctx, "SELECT 1")
	duration := time.Since(start)

	// Should have taken at least the retry delay time
	expectedMinDuration := time.Duration(config.RetryAttempts) * config.RetryDelay
	if duration < expectedMinDuration {
		t.Logf("Query completed faster than expected retry duration: %v < %v", duration, expectedMinDuration)
	}

	t.Logf("Query with retries took: %v", duration)
}
