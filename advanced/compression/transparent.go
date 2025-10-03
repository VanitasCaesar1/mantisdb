package compression

import (
	"encoding/binary"
	"fmt"
	"sync"
	"time"
)

// TransparentCompression provides transparent compression/decompression for storage
type TransparentCompression struct {
	engine        *CompressionEngine
	accessTracker *AccessTracker
	config        *TransparentConfig
	mutex         sync.RWMutex
}

// TransparentConfig configures transparent compression behavior
type TransparentConfig struct {
	Enabled            bool          `json:"enabled"`
	MinSize            int64         `json:"min_size"`
	ColdThreshold      time.Duration `json:"cold_threshold"`
	DefaultAlgorithm   string        `json:"default_algorithm"`
	CompressionLevel   int           `json:"compression_level"`
	BackgroundCompress bool          `json:"background_compress"`
}

// AccessTracker tracks data access patterns for cold data detection
type AccessTracker struct {
	accessTimes map[string]time.Time
	accessCount map[string]int64
	mutex       sync.RWMutex
}

// CompressedData represents compressed data with metadata
type CompressedData struct {
	Algorithm      string
	OriginalSize   int64
	CompressedSize int64
	Timestamp      time.Time
	Data           []byte
}

// CompressionHeader represents the header for compressed data
type CompressionHeader struct {
	Magic          [4]byte // "CMPR"
	Version        uint8
	Algorithm      uint8
	Reserved       uint16
	OriginalSize   uint64
	CompressedSize uint64
	Timestamp      uint64
}

const (
	CompressionMagic = "CMPR"
	HeaderSize       = 32

	// Algorithm IDs
	AlgoNone   = 0
	AlgoLZ4    = 1
	AlgoSnappy = 2
	AlgoZSTD   = 3
)

var algorithmMap = map[string]uint8{
	"none":   AlgoNone,
	"lz4":    AlgoLZ4,
	"snappy": AlgoSnappy,
	"zstd":   AlgoZSTD,
}

var algorithmNames = map[uint8]string{
	AlgoNone:   "none",
	AlgoLZ4:    "lz4",
	AlgoSnappy: "snappy",
	AlgoZSTD:   "zstd",
}

// NewTransparentCompression creates a new transparent compression layer
func NewTransparentCompression(config *TransparentConfig) *TransparentCompression {
	if config == nil {
		config = &TransparentConfig{
			Enabled:            true,
			MinSize:            1024,
			ColdThreshold:      24 * time.Hour,
			DefaultAlgorithm:   "lz4",
			CompressionLevel:   1,
			BackgroundCompress: true,
		}
	}

	return &TransparentCompression{
		engine:        NewCompressionEngine(),
		accessTracker: NewAccessTracker(),
		config:        config,
	}
}

// NewAccessTracker creates a new access tracker
func NewAccessTracker() *AccessTracker {
	return &AccessTracker{
		accessTimes: make(map[string]time.Time),
		accessCount: make(map[string]int64),
	}
}

// Write compresses data if it meets compression criteria
func (tc *TransparentCompression) Write(key string, data []byte) ([]byte, error) {
	if !tc.config.Enabled {
		return data, nil
	}

	// Track access
	tc.accessTracker.RecordAccess(key)

	// Create metadata for compression decision
	metadata := &DataMetadata{
		Size:         int64(len(data)),
		LastAccessed: time.Now(),
		AccessCount:  tc.accessTracker.GetAccessCount(key),
		DataType:     "binary",
	}

	// Compress data
	compressed, algorithm, err := tc.engine.Compress(data, metadata)
	if err != nil {
		return nil, fmt.Errorf("compression failed: %w", err)
	}

	// If no compression was applied, return original data
	if algorithm == "none" {
		return data, nil
	}

	// Create compressed data with header
	return tc.createCompressedData(compressed, algorithm, int64(len(data)))
}

// Read decompresses data if it's compressed
func (tc *TransparentCompression) Read(key string, data []byte) ([]byte, error) {
	// Track access
	tc.accessTracker.RecordAccess(key)

	// Check if data is compressed
	if !tc.isCompressed(data) {
		return data, nil
	}

	// Parse compressed data
	compressedData, err := tc.parseCompressedData(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse compressed data: %w", err)
	}

	// Decompress
	decompressed, err := tc.engine.Decompress(compressedData.Data, compressedData.Algorithm)
	if err != nil {
		return nil, fmt.Errorf("decompression failed: %w", err)
	}

	return decompressed, nil
}

// createCompressedData creates a compressed data block with header
func (tc *TransparentCompression) createCompressedData(compressed []byte, algorithm string, originalSize int64) ([]byte, error) {
	algoID, exists := algorithmMap[algorithm]
	if !exists {
		return nil, fmt.Errorf("unknown algorithm: %s", algorithm)
	}

	header := CompressionHeader{
		Magic:          [4]byte{'C', 'M', 'P', 'R'},
		Version:        1,
		Algorithm:      algoID,
		Reserved:       0,
		OriginalSize:   uint64(originalSize),
		CompressedSize: uint64(len(compressed)),
		Timestamp:      uint64(time.Now().Unix()),
	}

	// Serialize header
	headerBytes := make([]byte, HeaderSize)
	copy(headerBytes[0:4], header.Magic[:])
	headerBytes[4] = header.Version
	headerBytes[5] = header.Algorithm
	binary.LittleEndian.PutUint16(headerBytes[6:8], header.Reserved)
	binary.LittleEndian.PutUint64(headerBytes[8:16], header.OriginalSize)
	binary.LittleEndian.PutUint64(headerBytes[16:24], header.CompressedSize)
	binary.LittleEndian.PutUint64(headerBytes[24:32], header.Timestamp)

	// Combine header and compressed data
	result := make([]byte, HeaderSize+len(compressed))
	copy(result[0:HeaderSize], headerBytes)
	copy(result[HeaderSize:], compressed)

	return result, nil
}

// parseCompressedData parses compressed data block
func (tc *TransparentCompression) parseCompressedData(data []byte) (*CompressedData, error) {
	if len(data) < HeaderSize {
		return nil, fmt.Errorf("data too small for compression header")
	}

	// Parse header
	header := CompressionHeader{}
	copy(header.Magic[:], data[0:4])
	header.Version = data[4]
	header.Algorithm = data[5]
	header.Reserved = binary.LittleEndian.Uint16(data[6:8])
	header.OriginalSize = binary.LittleEndian.Uint64(data[8:16])
	header.CompressedSize = binary.LittleEndian.Uint64(data[16:24])
	header.Timestamp = binary.LittleEndian.Uint64(data[24:32])

	// Validate magic
	if string(header.Magic[:]) != CompressionMagic {
		return nil, fmt.Errorf("invalid compression magic")
	}

	// Get algorithm name
	algorithmName, exists := algorithmNames[header.Algorithm]
	if !exists {
		return nil, fmt.Errorf("unknown algorithm ID: %d", header.Algorithm)
	}

	// Extract compressed data
	compressedData := data[HeaderSize:]
	if len(compressedData) != int(header.CompressedSize) {
		return nil, fmt.Errorf("compressed data size mismatch")
	}

	return &CompressedData{
		Algorithm:      algorithmName,
		OriginalSize:   int64(header.OriginalSize),
		CompressedSize: int64(header.CompressedSize),
		Timestamp:      time.Unix(int64(header.Timestamp), 0),
		Data:           compressedData,
	}, nil
}

// isCompressed checks if data is compressed by looking for the magic header
func (tc *TransparentCompression) isCompressed(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	return string(data[0:4]) == CompressionMagic
}

// RecordAccess records access to a key for cold data detection
func (at *AccessTracker) RecordAccess(key string) {
	at.mutex.Lock()
	defer at.mutex.Unlock()

	at.accessTimes[key] = time.Now()
	at.accessCount[key]++
}

// GetLastAccess returns the last access time for a key
func (at *AccessTracker) GetLastAccess(key string) time.Time {
	at.mutex.RLock()
	defer at.mutex.RUnlock()

	if lastAccess, exists := at.accessTimes[key]; exists {
		return lastAccess
	}
	return time.Time{}
}

// GetAccessCount returns the access count for a key
func (at *AccessTracker) GetAccessCount(key string) int64 {
	at.mutex.RLock()
	defer at.mutex.RUnlock()

	return at.accessCount[key]
}

// IsCold checks if data is considered cold based on access patterns
func (at *AccessTracker) IsCold(key string, threshold time.Duration) bool {
	lastAccess := at.GetLastAccess(key)
	if lastAccess.IsZero() {
		return true // Never accessed
	}
	return time.Since(lastAccess) > threshold
}

// GetColdKeys returns keys that are considered cold
func (at *AccessTracker) GetColdKeys(threshold time.Duration) []string {
	at.mutex.RLock()
	defer at.mutex.RUnlock()

	var coldKeys []string
	now := time.Now()

	for key, lastAccess := range at.accessTimes {
		if now.Sub(lastAccess) > threshold {
			coldKeys = append(coldKeys, key)
		}
	}

	return coldKeys
}

// Cleanup removes old access records
func (at *AccessTracker) Cleanup(maxAge time.Duration) {
	at.mutex.Lock()
	defer at.mutex.Unlock()

	cutoff := time.Now().Add(-maxAge)

	for key, lastAccess := range at.accessTimes {
		if lastAccess.Before(cutoff) {
			delete(at.accessTimes, key)
			delete(at.accessCount, key)
		}
	}
}

// GetConfig returns the current configuration
func (tc *TransparentCompression) GetConfig() *TransparentConfig {
	tc.mutex.RLock()
	defer tc.mutex.RUnlock()

	// Return a copy to prevent external modification
	config := *tc.config
	return &config
}

// UpdateConfig updates the compression configuration
func (tc *TransparentCompression) UpdateConfig(config *TransparentConfig) {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	tc.config = config
}

// GetStats returns compression statistics
func (tc *TransparentCompression) GetStats() CompressionStats {
	return tc.engine.GetStats()
}

// GetMonitor returns the compression monitor
func (tc *TransparentCompression) GetMonitor() *CompressionMonitor {
	return tc.engine.monitor
}
