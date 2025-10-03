package checkpoint

import (
	"fmt"
	"time"
)

// DefaultCheckpointConfig returns a default checkpoint configuration
func DefaultCheckpointConfig() *CheckpointConfig {
	return &CheckpointConfig{
		CheckpointDir:        "data/checkpoints",
		MaxCheckpoints:       10,
		CheckpointInterval:   5 * time.Minute,
		LSNInterval:          1000,
		EnableCompression:    false,
		CompressionAlgorithm: "gzip",
		EnableEncryption:     false,
		ValidateOnCreate:     true,
		ValidateOnLoad:       true,
		BufferSize:           64 * 1024, // 64KB
		ParallelCreation:     false,
		MaxWorkers:           4,
		AutoCleanup:          true,
		CleanupInterval:      1 * time.Hour,
		RetentionPeriod:      24 * time.Hour,
		MinCheckpoints:       3,
		MaxDiskUsage:         1024 * 1024 * 1024, // 1GB
	}
}

// ProductionCheckpointConfig returns a configuration suitable for production
func ProductionCheckpointConfig() *CheckpointConfig {
	return &CheckpointConfig{
		CheckpointDir:        "data/checkpoints",
		MaxCheckpoints:       20,
		CheckpointInterval:   2 * time.Minute,
		LSNInterval:          500,
		EnableCompression:    true,
		CompressionAlgorithm: "gzip",
		EnableEncryption:     false,
		ValidateOnCreate:     true,
		ValidateOnLoad:       true,
		BufferSize:           256 * 1024, // 256KB
		ParallelCreation:     true,
		MaxWorkers:           8,
		AutoCleanup:          true,
		CleanupInterval:      30 * time.Minute,
		RetentionPeriod:      7 * 24 * time.Hour, // 7 days
		MinCheckpoints:       5,
		MaxDiskUsage:         10 * 1024 * 1024 * 1024, // 10GB
	}
}

// FastCheckpointConfig returns a configuration optimized for speed
func FastCheckpointConfig() *CheckpointConfig {
	return &CheckpointConfig{
		CheckpointDir:        "data/checkpoints",
		MaxCheckpoints:       5,
		CheckpointInterval:   30 * time.Second,
		LSNInterval:          100,
		EnableCompression:    false,
		CompressionAlgorithm: "",
		EnableEncryption:     false,
		ValidateOnCreate:     false,
		ValidateOnLoad:       false,
		BufferSize:           1024 * 1024, // 1MB
		ParallelCreation:     true,
		MaxWorkers:           16,
		AutoCleanup:          true,
		CleanupInterval:      5 * time.Minute,
		RetentionPeriod:      1 * time.Hour,
		MinCheckpoints:       2,
		MaxDiskUsage:         512 * 1024 * 1024, // 512MB
	}
}

// SecureCheckpointConfig returns a configuration with security features enabled
func SecureCheckpointConfig() *CheckpointConfig {
	return &CheckpointConfig{
		CheckpointDir:        "data/checkpoints",
		MaxCheckpoints:       15,
		CheckpointInterval:   3 * time.Minute,
		LSNInterval:          750,
		EnableCompression:    true,
		CompressionAlgorithm: "gzip",
		EnableEncryption:     true,
		ValidateOnCreate:     true,
		ValidateOnLoad:       true,
		BufferSize:           128 * 1024, // 128KB
		ParallelCreation:     false,      // Disabled for security
		MaxWorkers:           4,
		AutoCleanup:          true,
		CleanupInterval:      1 * time.Hour,
		RetentionPeriod:      30 * 24 * time.Hour, // 30 days
		MinCheckpoints:       10,
		MaxDiskUsage:         5 * 1024 * 1024 * 1024, // 5GB
	}
}

// TestCheckpointConfig returns a configuration suitable for testing
func TestCheckpointConfig() *CheckpointConfig {
	return &CheckpointConfig{
		CheckpointDir:        "test/checkpoints",
		MaxCheckpoints:       3,
		CheckpointInterval:   1 * time.Second,
		LSNInterval:          10,
		EnableCompression:    false,
		CompressionAlgorithm: "",
		EnableEncryption:     false,
		ValidateOnCreate:     true,
		ValidateOnLoad:       true,
		BufferSize:           4 * 1024, // 4KB
		ParallelCreation:     false,
		MaxWorkers:           2,
		AutoCleanup:          false, // Manual cleanup in tests
		CleanupInterval:      0,
		RetentionPeriod:      0,
		MinCheckpoints:       1,
		MaxDiskUsage:         10 * 1024 * 1024, // 10MB
	}
}

// Validate validates the checkpoint configuration
func (c *CheckpointConfig) Validate() error {
	if c.CheckpointDir == "" {
		return fmt.Errorf("checkpoint directory cannot be empty")
	}

	if c.MaxCheckpoints <= 0 {
		return fmt.Errorf("max checkpoints must be positive: %d", c.MaxCheckpoints)
	}

	if c.CheckpointInterval < 0 {
		return fmt.Errorf("checkpoint interval cannot be negative: %v", c.CheckpointInterval)
	}

	if c.LSNInterval == 0 {
		return fmt.Errorf("LSN interval cannot be zero")
	}

	if c.EnableCompression && c.CompressionAlgorithm == "" {
		return fmt.Errorf("compression algorithm must be specified when compression is enabled")
	}

	if c.BufferSize <= 0 {
		return fmt.Errorf("buffer size must be positive: %d", c.BufferSize)
	}

	if c.ParallelCreation && c.MaxWorkers <= 0 {
		return fmt.Errorf("max workers must be positive when parallel creation is enabled: %d", c.MaxWorkers)
	}

	if c.CleanupInterval < 0 {
		return fmt.Errorf("cleanup interval cannot be negative: %v", c.CleanupInterval)
	}

	if c.RetentionPeriod < 0 {
		return fmt.Errorf("retention period cannot be negative: %v", c.RetentionPeriod)
	}

	if c.MinCheckpoints < 0 {
		return fmt.Errorf("min checkpoints cannot be negative: %d", c.MinCheckpoints)
	}

	if c.MinCheckpoints > c.MaxCheckpoints {
		return fmt.Errorf("min checkpoints (%d) cannot be greater than max checkpoints (%d)",
			c.MinCheckpoints, c.MaxCheckpoints)
	}

	if c.MaxDiskUsage < 0 {
		return fmt.Errorf("max disk usage cannot be negative: %d", c.MaxDiskUsage)
	}

	// Validate compression algorithm
	if c.EnableCompression {
		validAlgorithms := []string{"gzip", "lz4", "zstd", "snappy"}
		valid := false
		for _, alg := range validAlgorithms {
			if c.CompressionAlgorithm == alg {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("unsupported compression algorithm: %s", c.CompressionAlgorithm)
		}
	}

	// Validate encryption settings
	if c.EnableEncryption && len(c.EncryptionKey) == 0 {
		return fmt.Errorf("encryption key must be provided when encryption is enabled")
	}

	if c.EnableEncryption && len(c.EncryptionKey) < 16 {
		return fmt.Errorf("encryption key must be at least 16 bytes")
	}

	return nil
}

// Clone creates a deep copy of the configuration
func (c *CheckpointConfig) Clone() *CheckpointConfig {
	clone := *c

	// Copy encryption key if present
	if len(c.EncryptionKey) > 0 {
		clone.EncryptionKey = make([]byte, len(c.EncryptionKey))
		copy(clone.EncryptionKey, c.EncryptionKey)
	}

	return &clone
}

// IsCompressionEnabled returns true if compression is enabled
func (c *CheckpointConfig) IsCompressionEnabled() bool {
	return c.EnableCompression && c.CompressionAlgorithm != ""
}

// IsEncryptionEnabled returns true if encryption is enabled
func (c *CheckpointConfig) IsEncryptionEnabled() bool {
	return c.EnableEncryption && len(c.EncryptionKey) > 0
}

// IsParallelCreationEnabled returns true if parallel creation is enabled
func (c *CheckpointConfig) IsParallelCreationEnabled() bool {
	return c.ParallelCreation && c.MaxWorkers > 1
}

// IsAutoCleanupEnabled returns true if automatic cleanup is enabled
func (c *CheckpointConfig) IsAutoCleanupEnabled() bool {
	return c.AutoCleanup && c.CleanupInterval > 0
}

// GetEffectiveMaxWorkers returns the effective number of workers
func (c *CheckpointConfig) GetEffectiveMaxWorkers() int {
	if !c.ParallelCreation {
		return 1
	}
	if c.MaxWorkers <= 0 {
		return 1
	}
	return c.MaxWorkers
}

// ShouldValidateOnCreate returns true if validation should be performed on creation
func (c *CheckpointConfig) ShouldValidateOnCreate() bool {
	return c.ValidateOnCreate
}

// ShouldValidateOnLoad returns true if validation should be performed on load
func (c *CheckpointConfig) ShouldValidateOnLoad() bool {
	return c.ValidateOnLoad
}

// GetBufferSize returns the buffer size for I/O operations
func (c *CheckpointConfig) GetBufferSize() int {
	if c.BufferSize <= 0 {
		return 64 * 1024 // Default 64KB
	}
	return c.BufferSize
}

// String returns a string representation of the configuration
func (c *CheckpointConfig) String() string {
	return fmt.Sprintf("CheckpointConfig{Dir:%s, Max:%d, Interval:%v, LSN:%d, Compression:%v, Encryption:%v}",
		c.CheckpointDir, c.MaxCheckpoints, c.CheckpointInterval, c.LSNInterval,
		c.IsCompressionEnabled(), c.IsEncryptionEnabled())
}
