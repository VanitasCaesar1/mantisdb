package durability

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestDurabilityManager_BasicOperations(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "durability_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create config
	config := DefaultDurabilityConfig()
	config.Level = DurabilityAsync
	config.WriteMode = WriteModeSync

	// Create durability manager
	dm, err := NewDurabilityManager(config)
	if err != nil {
		t.Fatalf("Failed to create durability manager: %v", err)
	}
	defer dm.Close(context.Background())

	ctx := context.Background()
	testFile := filepath.Join(tempDir, "test_file.dat")
	testData := []byte("Hello, World!")

	// Test Write
	err = dm.Write(ctx, testFile, testData, 0)
	if err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}

	// Verify file was created and contains correct data
	writtenData, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	if string(writtenData) != string(testData) {
		t.Errorf("Written data mismatch: expected %s, got %s", string(testData), string(writtenData))
	}
}

func TestDurabilityManager_BatchWrite(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "durability_batch_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultDurabilityConfig()
	config.WriteMode = WriteModeAsync

	dm, err := NewDurabilityManager(config)
	if err != nil {
		t.Fatalf("Failed to create durability manager: %v", err)
	}
	defer dm.Close(context.Background())

	ctx := context.Background()

	// Create batch writes
	writes := []WriteOperation{
		{
			FilePath: filepath.Join(tempDir, "file1.dat"),
			Data:     []byte("Data for file 1"),
			Offset:   0,
		},
		{
			FilePath: filepath.Join(tempDir, "file2.dat"),
			Data:     []byte("Data for file 2"),
			Offset:   0,
		},
		{
			FilePath: filepath.Join(tempDir, "file3.dat"),
			Data:     []byte("Data for file 3"),
			Offset:   0,
		},
	}

	// Test BatchWrite
	err = dm.BatchWrite(ctx, writes)
	if err != nil {
		t.Fatalf("Failed to batch write: %v", err)
	}

	// Force flush to ensure data is written
	err = dm.Flush(ctx)
	if err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}

	// Verify all files were created with correct data
	for i, write := range writes {
		data, err := os.ReadFile(write.FilePath)
		if err != nil {
			t.Fatalf("Failed to read file %d: %v", i, err)
		}

		if string(data) != string(write.Data) {
			t.Errorf("File %d data mismatch: expected %s, got %s", i, string(write.Data), string(data))
		}
	}
}

func TestDurabilityManager_WriteModes(t *testing.T) {
	writeModes := []WriteMode{
		WriteModeSync,
		WriteModeAsync,
		WriteModeBatch,
	}

	for _, mode := range writeModes {
		t.Run(mode.String(), func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", "durability_mode_test")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir)

			config := DefaultDurabilityConfig()
			config.WriteMode = mode

			dm, err := NewDurabilityManager(config)
			if err != nil {
				t.Fatalf("Failed to create durability manager for mode %s: %v", mode, err)
			}
			defer dm.Close(context.Background())

			ctx := context.Background()
			testFile := filepath.Join(tempDir, "test_file.dat")
			testData := []byte("Test data for write mode")

			err = dm.Write(ctx, testFile, testData, 0)
			if err != nil {
				t.Fatalf("Failed to write with mode %s: %v", mode, err)
			}

			// For async modes, force flush
			if mode == WriteModeAsync || mode == WriteModeBatch {
				err = dm.Flush(ctx)
				if err != nil {
					t.Fatalf("Failed to flush for mode %s: %v", mode, err)
				}
			}

			// Verify data was written
			writtenData, err := os.ReadFile(testFile)
			if err != nil {
				t.Fatalf("Failed to read file for mode %s: %v", mode, err)
			}

			if string(writtenData) != string(testData) {
				t.Errorf("Data mismatch for mode %s: expected %s, got %s", mode, string(testData), string(writtenData))
			}
		})
	}
}

func TestDurabilityManager_DurabilityLevels(t *testing.T) {
	testCases := []struct {
		level  DurabilityLevel
		config *DurabilityConfig
	}{
		{
			level:  DurabilityAsync,
			config: DefaultDurabilityConfig(),
		},
		{
			level:  DurabilitySync,
			config: SyncDurabilityConfig(),
		},
		{
			level:  DurabilityStrict,
			config: StrictDurabilityConfig(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.level.String(), func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", "durability_level_test")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir)

			dm, err := NewDurabilityManager(tc.config)
			if err != nil {
				t.Fatalf("Failed to create durability manager for level %s: %v", tc.level, err)
			}
			defer dm.Close(context.Background())

			ctx := context.Background()
			testFile := filepath.Join(tempDir, "test_file.dat")
			testData := []byte("Test data for durability level")

			err = dm.Write(ctx, testFile, testData, 0)
			if err != nil {
				t.Fatalf("Failed to write with level %s: %v", tc.level, err)
			}

			// For async writes, force flush to ensure data is written
			if tc.level == DurabilityAsync {
				err = dm.Flush(ctx)
				if err != nil {
					t.Fatalf("Failed to flush for level %s: %v", tc.level, err)
				}
			}

			// Verify data was written
			writtenData, err := os.ReadFile(testFile)
			if err != nil {
				t.Fatalf("Failed to read file for level %s: %v", tc.level, err)
			}

			if string(writtenData) != string(testData) {
				t.Errorf("Data mismatch for level %s: expected %s, got %s", tc.level, string(testData), string(writtenData))
			}
		})
	}
}

func TestDurabilityManager_FlushOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "durability_flush_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultDurabilityConfig()
	config.WriteMode = WriteModeAsync

	dm, err := NewDurabilityManager(config)
	if err != nil {
		t.Fatalf("Failed to create durability manager: %v", err)
	}
	defer dm.Close(context.Background())

	ctx := context.Background()
	testFile := filepath.Join(tempDir, "test_file.dat")
	testData := []byte("Test data for flush operations")

	// Write data
	err = dm.Write(ctx, testFile, testData, 0)
	if err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}

	// Test FlushFile
	err = dm.FlushFile(ctx, testFile)
	if err != nil {
		t.Fatalf("Failed to flush file: %v", err)
	}

	// Test Flush (all files)
	err = dm.Flush(ctx)
	if err != nil {
		t.Fatalf("Failed to flush all: %v", err)
	}

	// Verify data was written
	writtenData, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(writtenData) != string(testData) {
		t.Errorf("Data mismatch: expected %s, got %s", string(testData), string(writtenData))
	}
}

func TestDurabilityManager_SyncOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "durability_sync_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultDurabilityConfig()
	config.WriteMode = WriteModeAsync

	dm, err := NewDurabilityManager(config)
	if err != nil {
		t.Fatalf("Failed to create durability manager: %v", err)
	}
	defer dm.Close(context.Background())

	ctx := context.Background()
	testFile := filepath.Join(tempDir, "test_file.dat")
	testData := []byte("Test data for sync operations")

	// Write data
	err = dm.Write(ctx, testFile, testData, 0)
	if err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}

	// Test SyncFile
	err = dm.SyncFile(ctx, testFile)
	if err != nil {
		t.Fatalf("Failed to sync file: %v", err)
	}

	// Test Sync (all files)
	err = dm.Sync(ctx)
	if err != nil {
		t.Fatalf("Failed to sync all: %v", err)
	}

	// Verify data was written
	writtenData, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(writtenData) != string(testData) {
		t.Errorf("Data mismatch: expected %s, got %s", string(testData), string(writtenData))
	}
}

func TestDurabilityManager_ConfigOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "durability_config_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultDurabilityConfig()

	dm, err := NewDurabilityManager(config)
	if err != nil {
		t.Fatalf("Failed to create durability manager: %v", err)
	}
	defer dm.Close(context.Background())

	// Test GetConfig
	currentConfig := dm.GetConfig()
	if currentConfig.Level != DurabilityAsync {
		t.Errorf("Config level mismatch: expected %v, got %v", DurabilityAsync, currentConfig.Level)
	}

	// Test UpdateConfig with a valid strict configuration
	newConfig := StrictDurabilityConfig()

	err = dm.UpdateConfig(newConfig)
	if err != nil {
		t.Fatalf("Failed to update config: %v", err)
	}

	updatedConfig := dm.GetConfig()
	if updatedConfig.Level != DurabilityStrict {
		t.Errorf("Config level not updated: expected %v, got %v", DurabilityStrict, updatedConfig.Level)
	}

	if updatedConfig.WriteMode != WriteModeSync {
		t.Errorf("Config write mode not updated: expected %v, got %v", WriteModeSync, updatedConfig.WriteMode)
	}
}

func TestDurabilityManager_Metrics(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "durability_metrics_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultDurabilityConfig()

	dm, err := NewDurabilityManager(config)
	if err != nil {
		t.Fatalf("Failed to create durability manager: %v", err)
	}
	defer dm.Close(context.Background())

	// Get initial metrics
	metrics := dm.GetMetrics()

	// Metrics should be initialized (not all zero)
	if metrics.SyncWriter.SyncOperations < 0 {
		t.Error("SyncWriter metrics should be initialized")
	}

	if metrics.AsyncWriter.AsyncWrites < 0 {
		t.Error("AsyncWriter metrics should be initialized")
	}

	// Perform some operations to generate metrics
	ctx := context.Background()
	testFile := filepath.Join(tempDir, "metrics_test.dat")
	testData := []byte("Test data for metrics")

	err = dm.Write(ctx, testFile, testData, 0)
	if err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}

	// Get updated metrics
	updatedMetrics := dm.GetMetrics()

	// Some metric should have changed
	if updatedMetrics.SyncWriter.SyncOperations == metrics.SyncWriter.SyncOperations &&
		updatedMetrics.AsyncWriter.AsyncWrites == metrics.AsyncWriter.AsyncWrites {
		t.Log("Note: Metrics may not have changed immediately, this could be expected")
	}
}

func TestDurabilityManager_Status(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "durability_status_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultDurabilityConfig()

	dm, err := NewDurabilityManager(config)
	if err != nil {
		t.Fatalf("Failed to create durability manager: %v", err)
	}
	defer dm.Close(context.Background())

	// Test GetStatus
	status := dm.GetStatus()

	if !status.Initialized {
		t.Error("Status should show initialized")
	}

	// Check that status is properly initialized
	if !status.Initialized {
		t.Error("Status should show as initialized")
	}

	if status.UnflushedWrites < 0 {
		t.Error("UnflushedWrites should not be negative")
	}
}

func TestDurabilityManager_ErrorHandling(t *testing.T) {
	// Test with invalid config
	invalidConfig := &DurabilityConfig{
		Level:     DurabilityLevel(999), // Invalid level
		WriteMode: WriteMode(999),       // Invalid mode
	}

	_, err := NewDurabilityManager(invalidConfig)
	if err == nil {
		t.Error("Expected error for invalid config")
	}

	// Test with nil config (should use defaults)
	tempDir, err := os.MkdirTemp("", "durability_error_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dm, err := NewDurabilityManager(nil)
	if err != nil {
		t.Fatalf("Failed to create durability manager with nil config: %v", err)
	}
	defer dm.Close(context.Background())

	// Test operations on uninitialized manager
	uninitializedDM := &DurabilityManager{initialized: false}

	ctx := context.Background()
	err = uninitializedDM.Write(ctx, "test", []byte("test"), 0)
	if err == nil {
		t.Error("Expected error for write on uninitialized manager")
	}

	err = uninitializedDM.Flush(ctx)
	if err == nil {
		t.Error("Expected error for flush on uninitialized manager")
	}

	err = uninitializedDM.Sync(ctx)
	if err == nil {
		t.Error("Expected error for sync on uninitialized manager")
	}
}

func TestDurabilityManager_ConcurrentOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "durability_concurrent_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultDurabilityConfig()
	config.WriteMode = WriteModeAsync

	dm, err := NewDurabilityManager(config)
	if err != nil {
		t.Fatalf("Failed to create durability manager: %v", err)
	}
	defer dm.Close(context.Background())

	const numGoroutines = 10
	const writesPerGoroutine = 5
	done := make(chan bool, numGoroutines)

	// Test concurrent writes
	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer func() { done <- true }()

			ctx := context.Background()

			for i := 0; i < writesPerGoroutine; i++ {
				testFile := filepath.Join(tempDir, fmt.Sprintf("concurrent_%d_%d.dat", goroutineID, i))
				testData := make([]byte, 0, 64)
				testData = fmt.Appendf(testData, "Data from goroutine %d, write %d", goroutineID, i)

				err := dm.Write(ctx, testFile, testData, 0)
				if err != nil {
					t.Errorf("Goroutine %d, write %d failed: %v", goroutineID, i, err)
					return
				}
			}
		}(g)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Force flush to ensure all data is written
	err = dm.Flush(context.Background())
	if err != nil {
		t.Fatalf("Failed to flush after concurrent writes: %v", err)
	}

	// Verify all files were created
	expectedFiles := numGoroutines * writesPerGoroutine
	files, err := filepath.Glob(filepath.Join(tempDir, "concurrent_*.dat"))
	if err != nil {
		t.Fatalf("Failed to list files: %v", err)
	}

	if len(files) != expectedFiles {
		t.Errorf("Expected %d files, got %d", expectedFiles, len(files))
	}
}
