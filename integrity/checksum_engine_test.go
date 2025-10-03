package integrity

import (
	"os"
	"testing"
)

func TestChecksumEngine_BasicOperations(t *testing.T) {
	engine := NewChecksumEngine(ChecksumCRC32)

	testData := []byte("Hello, World!")

	// Test Calculate
	checksum1 := engine.Calculate(testData)
	checksum2 := engine.Calculate(testData)

	if checksum1 != checksum2 {
		t.Errorf("Checksum calculation not deterministic: %d != %d", checksum1, checksum2)
	}

	if checksum1 == 0 {
		t.Error("Checksum should not be zero for non-empty data")
	}

	// Test CalculateString
	checksumStr1 := engine.CalculateString(testData)
	checksumStr2 := engine.CalculateString(testData)

	if checksumStr1 != checksumStr2 {
		t.Errorf("String checksum calculation not deterministic: %s != %s", checksumStr1, checksumStr2)
	}

	if len(checksumStr1) == 0 {
		t.Error("String checksum should not be empty")
	}
}

func TestChecksumEngine_Verify(t *testing.T) {
	engine := NewChecksumEngine(ChecksumCRC32)

	testData := []byte("Test data for verification")
	checksum := engine.Calculate(testData)

	// Test successful verification
	err := engine.Verify(testData, checksum)
	if err != nil {
		t.Errorf("Verification failed for correct checksum: %v", err)
	}

	// Test failed verification
	err = engine.Verify(testData, checksum+1)
	if err == nil {
		t.Error("Expected verification to fail for incorrect checksum")
	}

	// Verify error type
	if _, ok := err.(*ChecksumMismatchError); !ok {
		t.Errorf("Expected ChecksumMismatchError, got %T", err)
	}
}

func TestChecksumEngine_VerifyString(t *testing.T) {
	engine := NewChecksumEngine(ChecksumCRC32)

	testData := []byte("Test data for string verification")
	checksumStr := engine.CalculateString(testData)

	// Test successful verification
	err := engine.VerifyString(testData, checksumStr)
	if err != nil {
		t.Errorf("String verification failed for correct checksum: %v", err)
	}

	// Test failed verification
	err = engine.VerifyString(testData, "invalid_checksum")
	if err == nil {
		t.Error("Expected string verification to fail for incorrect checksum")
	}
}

func TestChecksumEngine_BatchOperations(t *testing.T) {
	engine := NewChecksumEngine(ChecksumCRC32)

	testData := [][]byte{
		[]byte("First block"),
		[]byte("Second block"),
		[]byte("Third block"),
		[]byte(""),
		[]byte("Last block with more data"),
	}

	// Test CalculateBatch
	checksums := engine.CalculateBatch(testData)

	if len(checksums) != len(testData) {
		t.Errorf("Expected %d checksums, got %d", len(testData), len(checksums))
	}

	// Verify each checksum matches individual calculation
	for i, data := range testData {
		expectedChecksum := engine.Calculate(data)
		if checksums[i] != expectedChecksum {
			t.Errorf("Batch checksum %d mismatch: expected %d, got %d", i, expectedChecksum, checksums[i])
		}
	}

	// Test VerifyBatch with correct checksums
	errors := engine.VerifyBatch(testData, checksums)
	if len(errors) != len(testData) {
		t.Errorf("Expected %d verification results, got %d", len(testData), len(errors))
	}

	for i, err := range errors {
		if err != nil {
			t.Errorf("Batch verification %d failed: %v", i, err)
		}
	}

	// Test VerifyBatch with incorrect checksums
	incorrectChecksums := make([]uint32, len(checksums))
	for i, checksum := range checksums {
		incorrectChecksums[i] = checksum + 1
	}

	errors = engine.VerifyBatch(testData, incorrectChecksums)
	for i, err := range errors {
		if len(testData[i]) > 0 && err == nil {
			t.Errorf("Expected batch verification %d to fail for incorrect checksum", i)
		}
	}

	// Test VerifyBatch with mismatched lengths
	shortChecksums := checksums[:len(checksums)-1]
	errors = engine.VerifyBatch(testData, shortChecksums)
	if len(errors) != 1 || errors[0] == nil {
		t.Error("Expected single error for mismatched batch lengths")
	}
}

func TestChecksumEngine_FileOperations(t *testing.T) {
	engine := NewChecksumEngine(ChecksumCRC32)

	// Create temporary file
	tempFile, err := os.CreateTemp("", "checksum_test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	testData := []byte("This is test data for file checksum calculation")
	_, err = tempFile.Write(testData)
	if err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}
	tempFile.Close()

	// Test CalculateFileChecksum
	fileChecksum, err := engine.CalculateFileChecksum(tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to calculate file checksum: %v", err)
	}

	// Verify it matches data checksum
	dataChecksum := engine.Calculate(testData)
	if fileChecksum != dataChecksum {
		t.Errorf("File checksum mismatch: expected %d, got %d", dataChecksum, fileChecksum)
	}

	// Test VerifyFileChecksum with correct checksum
	err = engine.VerifyFileChecksum(tempFile.Name(), fileChecksum)
	if err != nil {
		t.Errorf("File verification failed for correct checksum: %v", err)
	}

	// Test VerifyFileChecksum with incorrect checksum
	err = engine.VerifyFileChecksum(tempFile.Name(), fileChecksum+1)
	if err == nil {
		t.Error("Expected file verification to fail for incorrect checksum")
	}

	// Verify error type
	if _, ok := err.(*FileChecksumMismatchError); !ok {
		t.Errorf("Expected FileChecksumMismatchError, got %T", err)
	}

	// Test with non-existent file
	_, err = engine.CalculateFileChecksum("/non/existent/file")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestChecksumEngine_Algorithms(t *testing.T) {
	algorithms := []ChecksumAlgorithm{
		ChecksumCRC32,
		ChecksumMD5,
		ChecksumSHA256,
	}

	testData := []byte("Test data for algorithm comparison")

	for _, algorithm := range algorithms {
		t.Run(algorithm.String(), func(t *testing.T) {
			engine := NewChecksumEngine(algorithm)

			// Test algorithm getter
			if engine.GetAlgorithm() != algorithm {
				t.Errorf("Algorithm mismatch: expected %v, got %v", algorithm, engine.GetAlgorithm())
			}

			// Test algorithm name
			name := engine.GetAlgorithmName()
			if len(name) == 0 {
				t.Error("Algorithm name should not be empty")
			}

			// Test checksum calculation
			checksum := engine.Calculate(testData)
			if checksum == 0 && len(testData) > 0 {
				t.Error("Checksum should not be zero for non-empty data")
			}

			// Test string checksum calculation
			checksumStr := engine.CalculateString(testData)
			if len(checksumStr) == 0 {
				t.Error("String checksum should not be empty")
			}

			// Test verification
			err := engine.Verify(testData, checksum)
			if err != nil {
				t.Errorf("Verification failed: %v", err)
			}

			err = engine.VerifyString(testData, checksumStr)
			if err != nil {
				t.Errorf("String verification failed: %v", err)
			}
		})
	}
}

func TestChecksumEngine_SetAlgorithm(t *testing.T) {
	engine := NewChecksumEngine(ChecksumCRC32)

	testData := []byte("Test data for algorithm switching")

	// Calculate with CRC32
	crc32Checksum := engine.Calculate(testData)

	// Switch to MD5
	engine.SetAlgorithm(ChecksumMD5)
	if engine.GetAlgorithm() != ChecksumMD5 {
		t.Error("Algorithm not switched to MD5")
	}

	md5Checksum := engine.Calculate(testData)

	// Switch to SHA256
	engine.SetAlgorithm(ChecksumSHA256)
	if engine.GetAlgorithm() != ChecksumSHA256 {
		t.Error("Algorithm not switched to SHA256")
	}

	sha256Checksum := engine.Calculate(testData)

	// Verify checksums are different (they should be for different algorithms)
	if crc32Checksum == md5Checksum || md5Checksum == sha256Checksum || crc32Checksum == sha256Checksum {
		t.Error("Different algorithms should produce different checksums")
	}

	// Switch back to CRC32 and verify consistency
	engine.SetAlgorithm(ChecksumCRC32)
	newCrc32Checksum := engine.Calculate(testData)
	if crc32Checksum != newCrc32Checksum {
		t.Error("CRC32 checksum not consistent after algorithm switching")
	}
}

func TestChecksumEngine_EmptyData(t *testing.T) {
	engine := NewChecksumEngine(ChecksumCRC32)

	emptyData := []byte{}

	// Test with empty data
	checksum := engine.Calculate(emptyData)
	checksumStr := engine.CalculateString(emptyData)

	// Verify verification works with empty data
	err := engine.Verify(emptyData, checksum)
	if err != nil {
		t.Errorf("Verification failed for empty data: %v", err)
	}

	err = engine.VerifyString(emptyData, checksumStr)
	if err != nil {
		t.Errorf("String verification failed for empty data: %v", err)
	}

	// Test batch operations with empty data
	batchData := [][]byte{emptyData, []byte("non-empty"), emptyData}
	checksums := engine.CalculateBatch(batchData)

	if len(checksums) != 3 {
		t.Errorf("Expected 3 checksums, got %d", len(checksums))
	}

	errors := engine.VerifyBatch(batchData, checksums)
	for i, err := range errors {
		if err != nil {
			t.Errorf("Batch verification %d failed: %v", i, err)
		}
	}
}

func TestChecksumEngine_ConcurrentAccess(t *testing.T) {
	engine := NewChecksumEngine(ChecksumCRC32)

	testData := []byte("Concurrent access test data")
	const numGoroutines = 10

	done := make(chan bool, numGoroutines)

	// Test concurrent reads
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()

			checksum := engine.Calculate(testData)
			err := engine.Verify(testData, checksum)
			if err != nil {
				t.Errorf("Concurrent verification failed: %v", err)
			}
		}()
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Test concurrent algorithm switching
	algorithms := []ChecksumAlgorithm{ChecksumCRC32, ChecksumMD5, ChecksumSHA256}

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer func() { done <- true }()

			algorithm := algorithms[goroutineID%len(algorithms)]
			engine.SetAlgorithm(algorithm)

			checksum := engine.Calculate(testData)
			if checksum == 0 && len(testData) > 0 {
				t.Error("Checksum should not be zero for non-empty data")
			}
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

// Helper method for ChecksumAlgorithm.String() for testing
func (ca ChecksumAlgorithm) String() string {
	switch ca {
	case ChecksumCRC32:
		return "CRC32"
	case ChecksumMD5:
		return "MD5"
	case ChecksumSHA256:
		return "SHA256"
	default:
		return "Unknown"
	}
}
