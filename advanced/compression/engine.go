package compression

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/golang/snappy"
	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"
)

// CompressionAlgorithm defines the interface for compression algorithms
type CompressionAlgorithm interface {
	Compress(data []byte) ([]byte, error)
	Decompress(data []byte) ([]byte, error)
	Name() string
	Ratio() float64
}

// CompressionEngine manages multiple compression algorithms and policies
type CompressionEngine struct {
	algorithms map[string]CompressionAlgorithm
	policies   []CompressionPolicy
	monitor    *CompressionMonitor
	mutex      sync.RWMutex
	stats      *CompressionStats
}

// CompressionPolicy defines when and how to compress data
type CompressionPolicy interface {
	ShouldCompress(data []byte, metadata *DataMetadata) bool
	SelectAlgorithm(data []byte, metadata *DataMetadata) string
}

// DataMetadata contains information about data for compression decisions
type DataMetadata struct {
	Size         int64
	LastAccessed time.Time
	AccessCount  int64
	DataType     string
	TableName    string
}

// CompressionStats tracks compression performance metrics
type CompressionStats struct {
	TotalCompressed   int64
	TotalDecompressed int64
	CompressionRatio  float64
	CompressionTime   time.Duration
	DecompressionTime time.Duration
	mutex             sync.RWMutex
}

// NewCompressionEngine creates a new compression engine with default algorithms
func NewCompressionEngine() *CompressionEngine {
	engine := &CompressionEngine{
		algorithms: make(map[string]CompressionAlgorithm),
		policies:   make([]CompressionPolicy, 0),
		monitor:    NewCompressionMonitor(),
		stats:      &CompressionStats{},
	}

	// Register default algorithms
	engine.RegisterAlgorithm(&LZ4Algorithm{})
	engine.RegisterAlgorithm(&SnappyAlgorithm{})
	engine.RegisterAlgorithm(&ZSTDAlgorithm{})

	// Add default policies
	engine.AddPolicy(&SizeBasedPolicy{MinSize: 1024}) // Compress data > 1KB
	engine.AddPolicy(&ColdDataCompressionPolicy{ColdThreshold: 24 * time.Hour})

	return engine
}

// RegisterAlgorithm adds a compression algorithm to the engine
func (e *CompressionEngine) RegisterAlgorithm(algo CompressionAlgorithm) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.algorithms[algo.Name()] = algo
}

// AddPolicy adds a compression policy to the engine
func (e *CompressionEngine) AddPolicy(policy CompressionPolicy) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.policies = append(e.policies, policy)
}

// Compress compresses data using the best algorithm based on policies
func (e *CompressionEngine) Compress(data []byte, metadata *DataMetadata) ([]byte, string, error) {
	start := time.Now()
	defer func() {
		e.stats.mutex.Lock()
		e.stats.CompressionTime += time.Since(start)
		e.stats.mutex.Unlock()
	}()

	// Check if data should be compressed
	shouldCompress := false
	var selectedAlgo string

	e.mutex.RLock()
	for _, policy := range e.policies {
		if policy.ShouldCompress(data, metadata) {
			shouldCompress = true
			selectedAlgo = policy.SelectAlgorithm(data, metadata)
			break
		}
	}
	e.mutex.RUnlock()

	if !shouldCompress {
		return data, "none", nil
	}

	// Default to LZ4 if no algorithm selected
	if selectedAlgo == "" {
		selectedAlgo = "lz4"
	}

	e.mutex.RLock()
	algo, exists := e.algorithms[selectedAlgo]
	e.mutex.RUnlock()

	if !exists {
		return nil, "", fmt.Errorf("compression algorithm %s not found", selectedAlgo)
	}

	compressed, err := algo.Compress(data)
	if err != nil {
		return nil, "", fmt.Errorf("compression failed: %w", err)
	}

	// Update stats
	e.stats.mutex.Lock()
	e.stats.TotalCompressed += int64(len(data))
	if len(compressed) > 0 {
		ratio := float64(len(data)) / float64(len(compressed))
		e.stats.CompressionRatio = (e.stats.CompressionRatio + ratio) / 2
	}
	e.stats.mutex.Unlock()

	// Update monitoring
	e.monitor.RecordCompression(selectedAlgo, len(data), len(compressed))

	return compressed, selectedAlgo, nil
}

// Decompress decompresses data using the specified algorithm
func (e *CompressionEngine) Decompress(data []byte, algorithm string) ([]byte, error) {
	start := time.Now()
	defer func() {
		e.stats.mutex.Lock()
		e.stats.DecompressionTime += time.Since(start)
		e.stats.mutex.Unlock()
	}()

	if algorithm == "none" {
		return data, nil
	}

	e.mutex.RLock()
	algo, exists := e.algorithms[algorithm]
	e.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("compression algorithm %s not found", algorithm)
	}

	decompressed, err := algo.Decompress(data)
	if err != nil {
		return nil, fmt.Errorf("decompression failed: %w", err)
	}

	// Update stats
	e.stats.mutex.Lock()
	e.stats.TotalDecompressed += int64(len(decompressed))
	e.stats.mutex.Unlock()

	// Update monitoring
	e.monitor.RecordDecompression(algorithm, len(data), len(decompressed))

	return decompressed, nil
}

// GetStats returns current compression statistics
func (e *CompressionEngine) GetStats() CompressionStats {
	e.stats.mutex.RLock()
	defer e.stats.mutex.RUnlock()
	return CompressionStats{
		TotalCompressed:   e.stats.TotalCompressed,
		TotalDecompressed: e.stats.TotalDecompressed,
		CompressionRatio:  e.stats.CompressionRatio,
		CompressionTime:   e.stats.CompressionTime,
		DecompressionTime: e.stats.DecompressionTime,
	}
}

// LZ4Algorithm implements LZ4 compression
type LZ4Algorithm struct{}

func (a *LZ4Algorithm) Name() string { return "lz4" }

func (a *LZ4Algorithm) Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := lz4.NewWriter(&buf)

	_, err := writer.Write(data)
	if err != nil {
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (a *LZ4Algorithm) Decompress(data []byte) ([]byte, error) {
	reader := lz4.NewReader(bytes.NewReader(data))
	return io.ReadAll(reader)
}

func (a *LZ4Algorithm) Ratio() float64 { return 2.5 } // Typical LZ4 ratio

// SnappyAlgorithm implements Snappy compression
type SnappyAlgorithm struct{}

func (a *SnappyAlgorithm) Name() string { return "snappy" }

func (a *SnappyAlgorithm) Compress(data []byte) ([]byte, error) {
	return snappy.Encode(nil, data), nil
}

func (a *SnappyAlgorithm) Decompress(data []byte) ([]byte, error) {
	return snappy.Decode(nil, data)
}

func (a *SnappyAlgorithm) Ratio() float64 { return 2.0 } // Typical Snappy ratio

// ZSTDAlgorithm implements ZSTD compression
type ZSTDAlgorithm struct {
	encoder *zstd.Encoder
	decoder *zstd.Decoder
}

func (a *ZSTDAlgorithm) Name() string { return "zstd" }

func (a *ZSTDAlgorithm) Compress(data []byte) ([]byte, error) {
	if a.encoder == nil {
		var err error
		a.encoder, err = zstd.NewWriter(nil)
		if err != nil {
			return nil, err
		}
	}
	return a.encoder.EncodeAll(data, nil), nil
}

func (a *ZSTDAlgorithm) Decompress(data []byte) ([]byte, error) {
	if a.decoder == nil {
		var err error
		a.decoder, err = zstd.NewReader(nil)
		if err != nil {
			return nil, err
		}
	}
	return a.decoder.DecodeAll(data, nil)
}

func (a *ZSTDAlgorithm) Ratio() float64 { return 3.5 } // Typical ZSTD ratio

// SizeBasedPolicy compresses data based on size threshold
type SizeBasedPolicy struct {
	MinSize int64
}

func (p *SizeBasedPolicy) ShouldCompress(data []byte, metadata *DataMetadata) bool {
	return int64(len(data)) >= p.MinSize
}

func (p *SizeBasedPolicy) SelectAlgorithm(data []byte, metadata *DataMetadata) string {
	// Use LZ4 for fast compression on smaller data
	if len(data) < 10*1024 {
		return "lz4"
	}
	// Use ZSTD for better compression on larger data
	return "zstd"
}

// ColdDataCompressionPolicy compresses data that hasn't been accessed recently
type ColdDataCompressionPolicy struct {
	ColdThreshold time.Duration
}

func (p *ColdDataCompressionPolicy) ShouldCompress(data []byte, metadata *DataMetadata) bool {
	if metadata == nil {
		return false
	}
	return time.Since(metadata.LastAccessed) > p.ColdThreshold
}

func (p *ColdDataCompressionPolicy) SelectAlgorithm(data []byte, metadata *DataMetadata) string {
	// Use ZSTD for cold data since we prioritize compression ratio over speed
	return "zstd"
}
