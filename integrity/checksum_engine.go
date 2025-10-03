package integrity

import (
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"sync"
)

// ChecksumAlgorithm represents different checksum algorithms
type ChecksumAlgorithm int

const (
	ChecksumCRC32 ChecksumAlgorithm = iota
	ChecksumMD5
	ChecksumSHA256
)

// ChecksumEngine provides data integrity verification through checksums
type ChecksumEngine struct {
	algorithm ChecksumAlgorithm
	mutex     sync.RWMutex
}

// ChecksumResult contains checksum calculation results
type ChecksumResult struct {
	Algorithm ChecksumAlgorithm `json:"algorithm"`
	Value     string            `json:"value"`
	Size      int64             `json:"size"`
}

// BatchChecksumResult contains results for batch checksum operations
type BatchChecksumResult struct {
	Results []ChecksumResult `json:"results"`
	Errors  []error          `json:"errors"`
}

// NewChecksumEngine creates a new checksum engine with the specified algorithm
func NewChecksumEngine(algorithm ChecksumAlgorithm) *ChecksumEngine {
	return &ChecksumEngine{
		algorithm: algorithm,
	}
}

// Calculate computes checksum for the given data
func (ce *ChecksumEngine) Calculate(data []byte) uint32 {
	ce.mutex.RLock()
	defer ce.mutex.RUnlock()

	switch ce.algorithm {
	case ChecksumCRC32:
		return crc32.ChecksumIEEE(data)
	case ChecksumMD5:
		hash := md5.Sum(data)
		// Convert first 4 bytes to uint32 for compatibility
		return uint32(hash[0])<<24 | uint32(hash[1])<<16 | uint32(hash[2])<<8 | uint32(hash[3])
	case ChecksumSHA256:
		hash := sha256.Sum256(data)
		// Convert first 4 bytes to uint32 for compatibility
		return uint32(hash[0])<<24 | uint32(hash[1])<<16 | uint32(hash[2])<<8 | uint32(hash[3])
	default:
		return crc32.ChecksumIEEE(data)
	}
}

// CalculateString computes checksum and returns as hex string
func (ce *ChecksumEngine) CalculateString(data []byte) string {
	ce.mutex.RLock()
	defer ce.mutex.RUnlock()

	switch ce.algorithm {
	case ChecksumCRC32:
		return fmt.Sprintf("%08x", crc32.ChecksumIEEE(data))
	case ChecksumMD5:
		hash := md5.Sum(data)
		return fmt.Sprintf("%x", hash)
	case ChecksumSHA256:
		hash := sha256.Sum256(data)
		return fmt.Sprintf("%x", hash)
	default:
		return fmt.Sprintf("%08x", crc32.ChecksumIEEE(data))
	}
}

// Verify checks if data matches the expected checksum
func (ce *ChecksumEngine) Verify(data []byte, expectedChecksum uint32) error {
	actualChecksum := ce.Calculate(data)
	if actualChecksum != expectedChecksum {
		return &ChecksumMismatchError{
			Expected: expectedChecksum,
			Actual:   actualChecksum,
			Size:     int64(len(data)),
		}
	}
	return nil
}

// VerifyString checks if data matches the expected checksum string
func (ce *ChecksumEngine) VerifyString(data []byte, expectedChecksum string) error {
	actualChecksum := ce.CalculateString(data)
	if actualChecksum != expectedChecksum {
		return &ChecksumMismatchError{
			ExpectedStr: expectedChecksum,
			ActualStr:   actualChecksum,
			Size:        int64(len(data)),
		}
	}
	return nil
}

// CalculateBatch computes checksums for multiple data blocks
func (ce *ChecksumEngine) CalculateBatch(dataBlocks [][]byte) []uint32 {
	ce.mutex.RLock()
	defer ce.mutex.RUnlock()

	results := make([]uint32, len(dataBlocks))
	for i, data := range dataBlocks {
		results[i] = ce.Calculate(data)
	}
	return results
}

// VerifyBatch verifies multiple data blocks against their expected checksums
func (ce *ChecksumEngine) VerifyBatch(dataBlocks [][]byte, checksums []uint32) []error {
	if len(dataBlocks) != len(checksums) {
		return []error{fmt.Errorf("data blocks and checksums length mismatch: %d vs %d", len(dataBlocks), len(checksums))}
	}

	ce.mutex.RLock()
	defer ce.mutex.RUnlock()

	errors := make([]error, len(dataBlocks))
	for i, data := range dataBlocks {
		errors[i] = ce.Verify(data, checksums[i])
	}
	return errors
}

// CalculateFileChecksum computes checksum for a file
func (ce *ChecksumEngine) CalculateFileChecksum(filePath string) (uint32, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	ce.mutex.RLock()
	defer ce.mutex.RUnlock()

	switch ce.algorithm {
	case ChecksumCRC32:
		hash := crc32.NewIEEE()
		if _, err := io.Copy(hash, file); err != nil {
			return 0, fmt.Errorf("failed to calculate CRC32 for file %s: %w", filePath, err)
		}
		return hash.Sum32(), nil

	case ChecksumMD5:
		hash := md5.New()
		if _, err := io.Copy(hash, file); err != nil {
			return 0, fmt.Errorf("failed to calculate MD5 for file %s: %w", filePath, err)
		}
		sum := hash.Sum(nil)
		return uint32(sum[0])<<24 | uint32(sum[1])<<16 | uint32(sum[2])<<8 | uint32(sum[3]), nil

	case ChecksumSHA256:
		hash := sha256.New()
		if _, err := io.Copy(hash, file); err != nil {
			return 0, fmt.Errorf("failed to calculate SHA256 for file %s: %w", filePath, err)
		}
		sum := hash.Sum(nil)
		return uint32(sum[0])<<24 | uint32(sum[1])<<16 | uint32(sum[2])<<8 | uint32(sum[3]), nil

	default:
		hash := crc32.NewIEEE()
		if _, err := io.Copy(hash, file); err != nil {
			return 0, fmt.Errorf("failed to calculate checksum for file %s: %w", filePath, err)
		}
		return hash.Sum32(), nil
	}
}

// VerifyFileChecksum verifies a file against expected checksum
func (ce *ChecksumEngine) VerifyFileChecksum(filePath string, expectedChecksum uint32) error {
	actualChecksum, err := ce.CalculateFileChecksum(filePath)
	if err != nil {
		return fmt.Errorf("failed to calculate file checksum: %w", err)
	}

	if actualChecksum != expectedChecksum {
		return &FileChecksumMismatchError{
			FilePath: filePath,
			Expected: expectedChecksum,
			Actual:   actualChecksum,
		}
	}
	return nil
}

// SetAlgorithm changes the checksum algorithm
func (ce *ChecksumEngine) SetAlgorithm(algorithm ChecksumAlgorithm) {
	ce.mutex.Lock()
	defer ce.mutex.Unlock()
	ce.algorithm = algorithm
}

// GetAlgorithm returns the current checksum algorithm
func (ce *ChecksumEngine) GetAlgorithm() ChecksumAlgorithm {
	ce.mutex.RLock()
	defer ce.mutex.RUnlock()
	return ce.algorithm
}

// GetAlgorithmName returns the name of the current algorithm
func (ce *ChecksumEngine) GetAlgorithmName() string {
	ce.mutex.RLock()
	defer ce.mutex.RUnlock()

	switch ce.algorithm {
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
