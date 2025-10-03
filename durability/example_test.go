package durability

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// TestDurabilityManagerBasic tests basic durability manager functionality
func TestDurabilityManagerBasic(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "durability_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test with async configuration
	config := DefaultDurabilityConfig()
	config.Level = DurabilityAsync
	config.WriteMode = WriteModeAsync
	config.FlushInterval = 100 * time.Millisecond

	dm, err := NewDurabilityManager(config)
	if err != nil {
		t.Fatalf("Failed to create durability manager: %v", err)
	}
	defer dm.Close(context.Background())

	// Test write operation
	ctx := context.Background()
	testFile := tempDir + "/test.dat"
	testData := []byte("Hello, durability!")

	err = dm.Write(ctx, testFile, testData, 0)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Test flush
	err = dm.Flush(ctx)
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// Verify metrics
	metrics := dm.GetMetrics()
	if metrics.AsyncWriter.AsyncWrites == 0 {
		t.Error("Expected async writes to be recorded")
	}

	// Test status
	status := dm.GetStatus()
	if !status.Initialized {
		t.Error("Expected durability manager to be initialized")
	}
}

// TestDurabilityManagerSync tests sync durability
func TestDurabilityManagerSync(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "durability_sync_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test with sync configuration
	config := SyncDurabilityConfig()

	dm, err := NewDurabilityManager(config)
	if err != nil {
		t.Fatalf("Failed to create durability manager: %v", err)
	}
	defer dm.Close(context.Background())

	// Test sync write
	ctx := context.Background()
	testFile := tempDir + "/sync_test.dat"
	testData := []byte("Sync write test")

	err = dm.Write(ctx, testFile, testData, 0)
	if err != nil {
		t.Fatalf("Sync write failed: %v", err)
	}

	// Test sync operation
	err = dm.Sync(ctx)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// Verify metrics
	metrics := dm.GetMetrics()
	if metrics.SyncWriter.SyncOperations == 0 {
		t.Error("Expected sync operations to be recorded")
	}
}

// TestDurabilityManagerBatch tests batch operations
func TestDurabilityManagerBatch(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "durability_batch_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultDurabilityConfig()
	config.WriteMode = WriteModeBatch
	config.BatchSize = 3

	dm, err := NewDurabilityManager(config)
	if err != nil {
		t.Fatalf("Failed to create durability manager: %v", err)
	}
	defer dm.Close(context.Background())

	// Test batch write
	ctx := context.Background()
	writes := []WriteOperation{
		{FilePath: tempDir + "/batch1.dat", Data: []byte("batch write 1"), Offset: 0},
		{FilePath: tempDir + "/batch2.dat", Data: []byte("batch write 2"), Offset: 0},
		{FilePath: tempDir + "/batch3.dat", Data: []byte("batch write 3"), Offset: 0},
	}

	err = dm.BatchWrite(ctx, writes)
	if err != nil {
		t.Fatalf("Batch write failed: %v", err)
	}

	// Flush to ensure writes are persisted
	err = dm.Flush(ctx)
	if err != nil {
		t.Fatalf("Flush after batch write failed: %v", err)
	}
}

// TestDurabilityConfigValidation tests configuration validation
func TestDurabilityConfigValidation(t *testing.T) {
	// Test valid config
	config := DefaultDurabilityConfig()
	if err := config.Validate(); err != nil {
		t.Errorf("Valid config should not fail validation: %v", err)
	}

	// Test invalid durability level
	config.Level = DurabilityLevel(999)
	if err := config.Validate(); err == nil {
		t.Error("Invalid durability level should fail validation")
	}

	// Test invalid write mode
	config = DefaultDurabilityConfig()
	config.WriteMode = WriteMode(999)
	if err := config.Validate(); err == nil {
		t.Error("Invalid write mode should fail validation")
	}

	// Test inconsistent configuration
	config = DefaultDurabilityConfig()
	config.Level = DurabilitySync
	config.SyncWrites = false
	if err := config.Validate(); err == nil {
		t.Error("Inconsistent sync config should fail validation")
	}
}

// ExampleDurabilityManager demonstrates basic usage
func ExampleDurabilityManager() {
	// Create durability manager with async configuration
	config := DefaultDurabilityConfig()
	config.Level = DurabilityAsync
	config.FlushInterval = 1 * time.Second

	dm, err := NewDurabilityManager(config)
	if err != nil {
		fmt.Printf("Failed to create durability manager: %v\n", err)
		return
	}
	defer dm.Close(context.Background())

	// Perform write operations
	ctx := context.Background()

	// Single write
	err = dm.Write(ctx, "/tmp/example.dat", []byte("Hello World"), 0)
	if err != nil {
		fmt.Printf("Write failed: %v\n", err)
		return
	}

	// Batch write
	writes := []WriteOperation{
		{FilePath: "/tmp/batch1.dat", Data: []byte("Batch 1"), Offset: 0},
		{FilePath: "/tmp/batch2.dat", Data: []byte("Batch 2"), Offset: 0},
	}

	err = dm.BatchWrite(ctx, writes)
	if err != nil {
		fmt.Printf("Batch write failed: %v\n", err)
		return
	}

	// Force flush
	err = dm.Flush(ctx)
	if err != nil {
		fmt.Printf("Flush failed: %v\n", err)
		return
	}

	// Get metrics
	metrics := dm.GetMetrics()
	fmt.Printf("Async writes: %d\n", metrics.AsyncWriter.AsyncWrites)
	fmt.Printf("Flush operations: %d\n", metrics.FlushManager.ForcedFlushes)

	// Output:
	// Async writes: 3
	// Flush operations: 1
}

// BenchmarkDurabilityManagerWrite benchmarks write performance
func BenchmarkDurabilityManagerWrite(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "durability_bench")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultDurabilityConfig()
	config.Level = DurabilityAsync

	dm, err := NewDurabilityManager(config)
	if err != nil {
		b.Fatalf("Failed to create durability manager: %v", err)
	}
	defer dm.Close(context.Background())

	ctx := context.Background()
	testData := []byte("benchmark test data")
	testFile := tempDir + "/bench.dat"

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := dm.Write(ctx, testFile, testData, 0)
		if err != nil {
			b.Fatalf("Write failed: %v", err)
		}
	}
}
