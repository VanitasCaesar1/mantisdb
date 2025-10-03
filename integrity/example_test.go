package integrity

import (
	"fmt"
	"os"
	"testing"
)

// TestIntegritySystemBasicUsage demonstrates basic usage of the integrity system
func TestIntegritySystemBasicUsage(t *testing.T) {
	// Create a test configuration
	config := DefaultIntegrityConfig()
	config.EnableBackgroundScan = false // Disable for testing

	// Create the integrity system
	system := NewIntegritySystem(config)

	// Start the system
	if err := system.Start(); err != nil {
		t.Fatalf("Failed to start integrity system: %v", err)
	}
	defer system.Stop()

	// Test checksum calculation
	testData := []byte("Hello, World! This is test data for integrity verification.")
	checksum, err := system.CalculateAndVerifyChecksum(testData, "test_location")
	if err != nil {
		t.Fatalf("Failed to calculate checksum: %v", err)
	}

	fmt.Printf("Calculated checksum: %08x\n", checksum)

	// Test data verification
	if err := system.VerifyData(testData, "test_location", checksum); err != nil {
		t.Fatalf("Data verification failed: %v", err)
	}

	// Test with corrupted data
	corruptedData := []byte("Hello, World! This is CORRUPTED data for integrity verification.")
	if err := system.VerifyData(corruptedData, "test_location", checksum); err == nil {
		t.Fatal("Expected verification to fail for corrupted data")
	}

	// Get health status
	health := system.GetHealthStatus()
	fmt.Printf("System health: %s\n", health.Status)

	// Get metrics
	metrics := system.GetMetrics()
	fmt.Printf("Total checksum operations: %d\n", metrics.ChecksumOperations.TotalOperations)
	fmt.Printf("Successful operations: %d\n", metrics.ChecksumOperations.SuccessfulOps)
	fmt.Printf("Failed operations: %d\n", metrics.ChecksumOperations.FailedOps)
}

// TestChecksumEngine demonstrates checksum engine functionality
func TestChecksumEngine(t *testing.T) {
	engine := NewChecksumEngine(ChecksumCRC32)

	testData := []byte("Test data for checksum calculation")

	// Test basic checksum calculation
	checksum := engine.Calculate(testData)
	if checksum == 0 {
		t.Fatal("Checksum should not be zero")
	}

	// Test checksum verification
	if err := engine.Verify(testData, checksum); err != nil {
		t.Fatalf("Checksum verification failed: %v", err)
	}

	// Test with wrong checksum
	if err := engine.Verify(testData, checksum+1); err == nil {
		t.Fatal("Expected verification to fail with wrong checksum")
	}

	// Test batch operations
	dataBlocks := [][]byte{
		[]byte("Block 1"),
		[]byte("Block 2"),
		[]byte("Block 3"),
	}

	checksums := engine.CalculateBatch(dataBlocks)
	if len(checksums) != len(dataBlocks) {
		t.Fatalf("Expected %d checksums, got %d", len(dataBlocks), len(checksums))
	}

	errors := engine.VerifyBatch(dataBlocks, checksums)
	for i, err := range errors {
		if err != nil {
			t.Fatalf("Batch verification failed for block %d: %v", i, err)
		}
	}
}

// TestCorruptionDetector demonstrates corruption detection functionality
func TestCorruptionDetector(t *testing.T) {
	config := DefaultIntegrityConfig()
	detector := NewCorruptionDetector(config)

	testData := []byte("Test data for corruption detection")
	engine := NewChecksumEngine(ChecksumCRC32)
	correctChecksum := engine.Calculate(testData)

	// Test with correct checksum (should not detect corruption)
	event := detector.DetectCorruption(testData, correctChecksum)
	if event != nil {
		t.Fatal("Should not detect corruption with correct checksum")
	}

	// Test with incorrect checksum (should detect corruption)
	event = detector.DetectCorruption(testData, correctChecksum+1)
	if event == nil {
		t.Fatal("Should detect corruption with incorrect checksum")
	}

	if event.Type != CorruptionTypeChecksum {
		t.Fatalf("Expected corruption type %s, got %s", CorruptionTypeChecksum, event.Type)
	}

	// Test data validation
	event = detector.ValidateData(testData, "test_location")
	// Should not detect corruption for valid data without cached checksum
	if event != nil {
		t.Fatalf("Unexpected corruption detected: %v", event)
	}

	// Test with empty data
	event = detector.ValidateData([]byte{}, "empty_location")
	if event == nil {
		t.Fatal("Should detect corruption for empty data")
	}

	if event.Type != CorruptionTypeSize {
		t.Fatalf("Expected corruption type %s, got %s", CorruptionTypeSize, event.Type)
	}
}

// TestFileIntegrity demonstrates file integrity verification
func TestFileIntegrity(t *testing.T) {
	// Create a temporary test file
	testFile := "test_integrity_file.txt"
	testData := []byte("This is test data for file integrity verification.")

	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(testFile)

	// Create integrity system
	config := DefaultIntegrityConfig()
	system := NewIntegritySystem(config)

	if err := system.Start(); err != nil {
		t.Fatalf("Failed to start integrity system: %v", err)
	}
	defer system.Stop()

	// Verify file integrity
	if err := system.VerifyFileIntegrity(testFile); err != nil {
		t.Fatalf("File integrity verification failed: %v", err)
	}

	// Test batch file verification
	testFiles := []string{testFile}
	results := system.BatchVerifyFiles(testFiles)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	if results[testFile] != nil {
		t.Fatalf("File verification should have succeeded: %v", results[testFile])
	}
}

// TestAlertHandlers demonstrates alert handling functionality
func TestAlertHandlers(t *testing.T) {
	// Test log alert handler
	logHandler := NewLogAlertHandler()

	err := logHandler.HandleAlert(AlertLevelWarning, "Test alert message", map[string]interface{}{
		"test_key": "test_value",
		"number":   42,
	})

	if err != nil {
		t.Fatalf("Log alert handler failed: %v", err)
	}

	// Test file alert handler
	alertFile := "test_alerts.log"
	fileHandler, err := NewFileAlertHandler(alertFile)
	if err != nil {
		t.Fatalf("Failed to create file alert handler: %v", err)
	}
	defer fileHandler.Close()
	defer os.Remove(alertFile)

	err = fileHandler.HandleAlert(AlertLevelError, "Test file alert", map[string]interface{}{
		"component": "test",
	})

	if err != nil {
		t.Fatalf("File alert handler failed: %v", err)
	}

	// Verify file was written
	if _, err := os.Stat(alertFile); os.IsNotExist(err) {
		t.Fatal("Alert file was not created")
	}

	// Test multi alert handler
	multiHandler := NewMultiAlertHandler(logHandler, fileHandler)

	err = multiHandler.HandleAlert(AlertLevelCritical, "Test multi alert", nil)
	if err != nil {
		t.Fatalf("Multi alert handler failed: %v", err)
	}
}

// BenchmarkChecksumCalculation benchmarks checksum calculation performance
func BenchmarkChecksumCalculation(b *testing.B) {
	engine := NewChecksumEngine(ChecksumCRC32)
	testData := make([]byte, 1024) // 1KB test data

	for i := range testData {
		testData[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Calculate(testData)
	}
}

// BenchmarkBatchChecksumCalculation benchmarks batch checksum calculation
func BenchmarkBatchChecksumCalculation(b *testing.B) {
	engine := NewChecksumEngine(ChecksumCRC32)

	// Create 100 blocks of 1KB each
	dataBlocks := make([][]byte, 100)
	for i := range dataBlocks {
		dataBlocks[i] = make([]byte, 1024)
		for j := range dataBlocks[i] {
			dataBlocks[i][j] = byte((i + j) % 256)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.CalculateBatch(dataBlocks)
	}
}

// ExampleIntegritySystem demonstrates how to use the integrity system
func ExampleIntegritySystem() {
	// Create configuration
	config := DefaultIntegrityConfig()
	config.ChecksumAlgorithm = ChecksumCRC32
	config.EnableBackgroundScan = false

	// Create and start the integrity system
	system := NewIntegritySystem(config)
	if err := system.Start(); err != nil {
		fmt.Printf("Failed to start system: %v\n", err)
		return
	}
	defer system.Stop()

	// Register an alert handler
	logHandler := NewLogAlertHandler()
	system.RegisterAlertHandler(logHandler)

	// Calculate checksum for some data
	data := []byte("Important data that needs integrity verification")
	checksum, err := system.CalculateAndVerifyChecksum(data, "important_data")
	if err != nil {
		fmt.Printf("Checksum calculation failed: %v\n", err)
		return
	}

	fmt.Printf("Data checksum: %08x\n", checksum)

	// Verify the data
	if err := system.VerifyData(data, "important_data", checksum); err != nil {
		fmt.Printf("Data verification failed: %v\n", err)
		return
	}

	fmt.Println("Data verification successful")

	// Get system health
	health := system.GetHealthStatus()
	fmt.Printf("System health: %s\n", health.Status)

	// Output:
	// Data checksum: [some hex value]
	// Data verification successful
	// System health: healthy
}
