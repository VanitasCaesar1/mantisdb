package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mantisDB/durability"
	"mantisDB/errors"
	"mantisDB/integrity"
)

// TestDurabilityIntegrityIntegration tests integration between durability and integrity systems
func TestDurabilityIntegrityIntegration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "durability_integrity_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create durability manager
	durabilityConfig := durability.DefaultDurabilityConfig()
	durabilityManager, err := durability.NewDurabilityManager(durabilityConfig)
	if err != nil {
		t.Fatalf("Failed to create durability manager: %v", err)
	}
	defer durabilityManager.Close(context.Background())

	// Create integrity system
	checksumEngine := integrity.NewChecksumEngine(integrity.ChecksumCRC32)
	integritySystem := integrity.NewIntegritySystem(&integrity.IntegrityConfig{
		ChecksumAlgorithm:    integrity.ChecksumCRC32,
		ScanInterval:         100 * time.Millisecond,
		EnableBackgroundScan: true,
	})

	// Create error handler
	errorHandler := errors.NewDefaultErrorHandler(nil)

	t.Run("WriteWithIntegrityVerification", func(t *testing.T) {
		ctx := context.Background()
		testFile := filepath.Join(tempDir, "integrity_test.dat")
		testData := []byte("Test data for integrity verification")

		// Calculate expected checksum
		expectedChecksum := checksumEngine.Calculate(testData)

		// Write data with durability
		if err := durabilityManager.Write(ctx, testFile, testData, 0); err != nil {
			t.Fatalf("Failed to write data: %v", err)
		}

		// Force sync to ensure data is written
		if err := durabilityManager.Sync(ctx); err != nil {
			t.Fatalf("Failed to sync data: %v", err)
		}

		// Read and verify integrity
		readData, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}

		if err := checksumEngine.Verify(readData, expectedChecksum); err != nil {
			t.Fatalf("Integrity verification failed: %v", err)
		}

		// Verify with integrity system
		if err := integritySystem.VerifyFileIntegrity(testFile); err != nil {
			t.Fatalf("Integrity system verification failed: %v", err)
		}
	})

	t.Run("BatchWriteWithIntegrityChecking", func(t *testing.T) {
		ctx := context.Background()

		// Prepare batch data
		batchData := []struct {
			filename string
			data     []byte
		}{
			{"batch1.dat", []byte("Batch data item 1")},
			{"batch2.dat", []byte("Batch data item 2")},
			{"batch3.dat", []byte("Batch data item 3")},
		}

		// Calculate checksums
		expectedChecksums := make([]uint32, len(batchData))
		writes := make([]durability.WriteOperation, len(batchData))

		for i, item := range batchData {
			expectedChecksums[i] = checksumEngine.Calculate(item.data)
			writes[i] = durability.WriteOperation{
				FilePath: filepath.Join(tempDir, item.filename),
				Data:     item.data,
				Offset:   0,
			}
		}

		// Execute batch write
		if err := durabilityManager.BatchWrite(ctx, writes); err != nil {
			t.Fatalf("Batch write failed: %v", err)
		}

		// Force sync
		if err := durabilityManager.Sync(ctx); err != nil {
			t.Fatalf("Failed to sync batch: %v", err)
		}

		// Verify integrity of all files
		for i, write := range writes {
			readData, err := os.ReadFile(write.FilePath)
			if err != nil {
				t.Fatalf("Failed to read batch file %d: %v", i, err)
			}

			if err := checksumEngine.Verify(readData, expectedChecksums[i]); err != nil {
				t.Fatalf("Batch item %d integrity check failed: %v", i, err)
			}

			// Verify with integrity system
			if err := integritySystem.VerifyFileIntegrity(write.FilePath); err != nil {
				t.Fatalf("Integrity system verification failed for batch item %d: %v", i, err)
			}
		}
	})

	t.Run("CorruptionDetectionAndHandling", func(t *testing.T) {
		ctx := context.Background()
		testFile := filepath.Join(tempDir, "corruption_test.dat")
		originalData := []byte("Original data that will be corrupted")

		// Write original data
		if err := durabilityManager.Write(ctx, testFile, originalData, 0); err != nil {
			t.Fatalf("Failed to write original data: %v", err)
		}

		if err := durabilityManager.Sync(ctx); err != nil {
			t.Fatalf("Failed to sync original data: %v", err)
		}

		// Calculate original checksum
		originalChecksum := checksumEngine.Calculate(originalData)

		// Simulate corruption by modifying the file directly
		corruptedData := []byte("Corrupted data that doesn't match checksum")
		if err := os.WriteFile(testFile, corruptedData, 0644); err != nil {
			t.Fatalf("Failed to corrupt file: %v", err)
		}

		// Try to verify integrity (should fail)
		readData, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("Failed to read corrupted file: %v", err)
		}

		err = checksumEngine.Verify(readData, originalChecksum)
		if err == nil {
			t.Error("Expected integrity verification to fail for corrupted data")
		}

		// Handle corruption through error handler
		corruptionInfo := errors.CorruptionInfo{
			Location: errors.DataLocation{
				File:   testFile,
				Offset: 0,
				Size:   int64(len(readData)),
			},
			Type:        "checksum_mismatch",
			Description: "Data corruption detected during integrity check",
			Timestamp:   time.Now(),
			Checksum:    originalChecksum,
		}

		if err := errorHandler.HandleCorruption(corruptionInfo); err != nil {
			t.Fatalf("Failed to handle corruption: %v", err)
		}

		// Isolate corrupted data
		if err := errorHandler.IsolateCorruptedData(corruptionInfo.Location); err != nil {
			t.Fatalf("Failed to isolate corrupted data: %v", err)
		}
	})

	t.Run("IntegritySystemMonitoring", func(t *testing.T) {
		ctx := context.Background()

		// Create multiple files for monitoring
		monitorFiles := []string{"monitor1.dat", "monitor2.dat", "monitor3.dat"}

		for i, filename := range monitorFiles {
			filePath := filepath.Join(tempDir, filename)
			data := []byte(fmt.Sprintf("Monitor data %d", i))

			// Write file
			if err := durabilityManager.Write(ctx, filePath, data, 0); err != nil {
				t.Fatalf("Failed to write monitor file %d: %v", i, err)
			}

			// Start background scan for the directory
			if err := integritySystem.StartBackgroundScan(tempDir); err != nil {
				t.Logf("Background scan already started or failed: %v", err)
			}
		}

		// Force sync
		if err := durabilityManager.Sync(ctx); err != nil {
			t.Fatalf("Failed to sync monitor files: %v", err)
		}

		// Wait for background integrity checks
		time.Sleep(200 * time.Millisecond)

		// Get integrity metrics
		metrics := integritySystem.GetMetrics()
		if metrics.BackgroundScans != nil && metrics.BackgroundScans.FilesScanned < int64(len(monitorFiles)) {
			t.Logf("Background scans may not have completed yet. Files scanned: %d, expected: %d",
				metrics.BackgroundScans.FilesScanned, len(monitorFiles))
		}

		// Verify all files
		for _, filename := range monitorFiles {
			filePath := filepath.Join(tempDir, filename)
			if err := integritySystem.VerifyFileIntegrity(filePath); err != nil {
				t.Errorf("Integrity verification failed for %s: %v", filename, err)
			}
		}
	})

	t.Run("DifferentChecksumAlgorithms", func(t *testing.T) {
		ctx := context.Background()
		algorithms := []integrity.ChecksumAlgorithm{
			integrity.ChecksumCRC32,
			integrity.ChecksumMD5,
			integrity.ChecksumSHA256,
		}

		testData := []byte("Test data for different checksum algorithms")

		for _, algorithm := range algorithms {
			t.Run(algorithmName(algorithm), func(t *testing.T) {
				// Create engine with specific algorithm
				engine := integrity.NewChecksumEngine(algorithm)

				filename := fmt.Sprintf("checksum_%s.dat", algorithmName(algorithm))
				filePath := filepath.Join(tempDir, filename)

				// Calculate checksum
				expectedChecksum := engine.Calculate(testData)

				// Write data
				if err := durabilityManager.Write(ctx, filePath, testData, 0); err != nil {
					t.Fatalf("Failed to write data for %s: %v", algorithmName(algorithm), err)
				}

				if err := durabilityManager.Sync(ctx); err != nil {
					t.Fatalf("Failed to sync data for %s: %v", algorithmName(algorithm), err)
				}

				// Read and verify
				readData, err := os.ReadFile(filePath)
				if err != nil {
					t.Fatalf("Failed to read file for %s: %v", algorithmName(algorithm), err)
				}

				if err := engine.Verify(readData, expectedChecksum); err != nil {
					t.Fatalf("Integrity verification failed for %s: %v", algorithmName(algorithm), err)
				}

				// Test file-level checksum
				fileChecksum, err := engine.CalculateFileChecksum(filePath)
				if err != nil {
					t.Fatalf("Failed to calculate file checksum for %s: %v", algorithmName(algorithm), err)
				}

				if fileChecksum != expectedChecksum {
					t.Errorf("File checksum mismatch for %s: expected %d, got %d",
						algorithmName(algorithm), expectedChecksum, fileChecksum)
				}
			})
		}
	})

	t.Run("ConcurrentIntegrityOperations", func(t *testing.T) {
		ctx := context.Background()
		const numGoroutines = 5
		const filesPerGoroutine = 3

		done := make(chan bool, numGoroutines)

		for g := 0; g < numGoroutines; g++ {
			go func(goroutineID int) {
				defer func() { done <- true }()

				for i := 0; i < filesPerGoroutine; i++ {
					filename := fmt.Sprintf("concurrent_%d_%d.dat", goroutineID, i)
					filePath := filepath.Join(tempDir, filename)
					data := []byte(fmt.Sprintf("Concurrent data from goroutine %d, file %d", goroutineID, i))

					// Calculate checksum
					expectedChecksum := checksumEngine.Calculate(data)

					// Write data
					if err := durabilityManager.Write(ctx, filePath, data, 0); err != nil {
						t.Errorf("Goroutine %d: Failed to write file %d: %v", goroutineID, i, err)
						return
					}

					// Verify integrity immediately
					if err := checksumEngine.Verify(data, expectedChecksum); err != nil {
						t.Errorf("Goroutine %d: Integrity check failed for file %d: %v", goroutineID, i, err)
						return
					}
				}
			}(g)
		}

		// Wait for all goroutines
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		// Force sync all data
		if err := durabilityManager.Sync(ctx); err != nil {
			t.Fatalf("Failed to sync concurrent data: %v", err)
		}

		// Verify all files exist and have correct integrity
		expectedFiles := numGoroutines * filesPerGoroutine
		files, err := filepath.Glob(filepath.Join(tempDir, "concurrent_*.dat"))
		if err != nil {
			t.Fatalf("Failed to list concurrent files: %v", err)
		}

		if len(files) != expectedFiles {
			t.Errorf("Expected %d concurrent files, got %d", expectedFiles, len(files))
		}

		// Verify integrity of all files
		for _, file := range files {
			if err := integritySystem.VerifyFileIntegrity(file); err != nil {
				t.Errorf("Integrity verification failed for %s: %v", file, err)
			}
		}
	})

	t.Log("Durability-Integrity integration test completed successfully")
}

// TestIntegritySystemWithErrorHandling tests integrity system with comprehensive error handling
func TestIntegritySystemWithErrorHandling(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "integrity_error_handling_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create components
	checksumEngine := integrity.NewChecksumEngine(integrity.ChecksumCRC32)
	errorHandler := errors.NewDefaultErrorHandler(nil)

	integrityConfig := &integrity.IntegrityConfig{
		ChecksumAlgorithm:    integrity.ChecksumCRC32,
		ScanInterval:         50 * time.Millisecond,
		EnableBackgroundScan: true,
		MaxConcurrentScans:   3,
	}

	integritySystem := integrity.NewIntegritySystem(integrityConfig)
	defer integritySystem.Stop()

	t.Run("HandleChecksumMismatch", func(t *testing.T) {
		testFile := filepath.Join(tempDir, "checksum_mismatch.dat")
		originalData := []byte("Original data")
		corruptedData := []byte("Corrupted data")

		// Write original data
		if err := os.WriteFile(testFile, originalData, 0644); err != nil {
			t.Fatalf("Failed to write original data: %v", err)
		}

		// Calculate original checksum
		originalChecksum := checksumEngine.Calculate(originalData)

		// Corrupt the file
		if err := os.WriteFile(testFile, corruptedData, 0644); err != nil {
			t.Fatalf("Failed to corrupt file: %v", err)
		}

		// Try to verify (should fail)
		readData, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("Failed to read corrupted file: %v", err)
		}

		err = checksumEngine.Verify(readData, originalChecksum)
		if err == nil {
			t.Error("Expected checksum verification to fail")
		}

		// Handle the error
		errorCtx := errors.ErrorContext{
			Operation:   "checksum_verification",
			Resource:    testFile,
			Severity:    errors.ErrorSeverityCritical,
			Category:    errors.ErrorCategoryCorruption,
			Recoverable: false,
			Timestamp:   time.Now(),
		}

		action := errorHandler.HandleError(err, errorCtx)
		if action != errors.ErrorActionFail {
			t.Logf("Error handler suggested action: %v", action)
		}

		// Create corruption info
		corruptionInfo := errors.CorruptionInfo{
			Location: errors.DataLocation{
				File:   testFile,
				Offset: 0,
				Size:   int64(len(readData)),
			},
			Type:        "checksum_mismatch",
			Description: "CRC32 checksum verification failed",
			Timestamp:   time.Now(),
			Checksum:    originalChecksum,
		}

		// Handle corruption
		if err := errorHandler.HandleCorruption(corruptionInfo); err != nil {
			t.Fatalf("Failed to handle corruption: %v", err)
		}
	})

	t.Run("HandleFileAccessErrors", func(t *testing.T) {
		nonExistentFile := filepath.Join(tempDir, "non_existent.dat")

		// Try to verify non-existent file
		err := integritySystem.VerifyFileIntegrity(nonExistentFile)
		if err == nil {
			t.Error("Expected error for non-existent file")
		}

		// Handle the error
		errorCtx := errors.ErrorContext{
			Operation:   "file_verification",
			Resource:    nonExistentFile,
			Severity:    errors.ErrorSeverityMedium,
			Category:    errors.ErrorCategoryIO,
			Recoverable: true,
			Timestamp:   time.Now(),
		}

		action := errorHandler.HandleError(err, errorCtx)
		if action == errors.ErrorActionRetry {
			t.Log("Error handler suggests retry for file access error")
		}
	})

	t.Run("HandleBatchVerificationErrors", func(t *testing.T) {
		// Create test files with mixed validity
		testFiles := []struct {
			name    string
			data    []byte
			corrupt bool
		}{
			{"batch_valid1.dat", []byte("Valid data 1"), false},
			{"batch_valid2.dat", []byte("Valid data 2"), false},
			{"batch_corrupt.dat", []byte("Original data"), true},
			{"batch_valid3.dat", []byte("Valid data 3"), false},
		}

		checksums := make([]uint32, len(testFiles))
		dataBlocks := make([][]byte, len(testFiles))

		// Create files and calculate checksums
		for i, file := range testFiles {
			filePath := filepath.Join(tempDir, file.name)
			checksums[i] = checksumEngine.Calculate(file.data)
			dataBlocks[i] = file.data

			// Write file
			if err := os.WriteFile(filePath, file.data, 0644); err != nil {
				t.Fatalf("Failed to write test file %s: %v", file.name, err)
			}

			// Corrupt if needed
			if file.corrupt {
				corruptedData := []byte("Corrupted version")
				if err := os.WriteFile(filePath, corruptedData, 0644); err != nil {
					t.Fatalf("Failed to corrupt file %s: %v", file.name, err)
				}
				// Update data block to reflect corruption
				dataBlocks[i] = corruptedData
			}
		}

		// Perform batch verification
		errors := checksumEngine.VerifyBatch(dataBlocks, checksums)

		// Check results
		corruptionCount := 0
		for i, err := range errors {
			if err != nil {
				corruptionCount++
				if !testFiles[i].corrupt {
					t.Errorf("Unexpected verification error for file %s: %v", testFiles[i].name, err)
				}
			} else {
				if testFiles[i].corrupt {
					t.Errorf("Expected verification error for corrupted file %s", testFiles[i].name)
				}
			}
		}

		if corruptionCount == 0 {
			t.Error("Expected at least one corruption to be detected")
		}

		t.Logf("Detected %d corrupted files out of %d", corruptionCount, len(testFiles))
	})

	t.Log("Integrity system with error handling test completed successfully")
}

// Helper function for algorithm names
func algorithmName(ca integrity.ChecksumAlgorithm) string {
	switch ca {
	case integrity.ChecksumCRC32:
		return "CRC32"
	case integrity.ChecksumMD5:
		return "MD5"
	case integrity.ChecksumSHA256:
		return "SHA256"
	default:
		return "Unknown"
	}
}
