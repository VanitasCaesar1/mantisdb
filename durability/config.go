package durability

import (
	"fmt"
	"time"
)

// DurabilityLevel defines the level of durability guarantees
type DurabilityLevel int

const (
	// DurabilityNone - No durability guarantees (fastest)
	DurabilityNone DurabilityLevel = iota

	// DurabilityAsync - Asynchronous writes with periodic flushes
	DurabilityAsync

	// DurabilitySync - Synchronous writes with fsync
	DurabilitySync

	// DurabilityStrict - Strict durability with barriers and verification
	DurabilityStrict
)

// WriteMode defines how writes are handled
type WriteMode int

const (
	// WriteModeAsync - Buffered writes with periodic flush
	WriteModeAsync WriteMode = iota

	// WriteModeSync - Immediate sync to disk
	WriteModeSync

	// WriteModeBatch - Batch writes with configurable sync
	WriteModeBatch
)

// DurabilityConfig holds all durability-related configuration
type DurabilityConfig struct {
	// Level defines the overall durability level
	Level DurabilityLevel `json:"level"`

	// WriteMode defines how writes are handled
	WriteMode WriteMode `json:"write_mode"`

	// SyncWrites forces fsync on every write when true
	SyncWrites bool `json:"sync_writes"`

	// FlushInterval for async writes (how often to flush buffers)
	FlushInterval time.Duration `json:"flush_interval"`

	// BatchSize for batch write mode
	BatchSize int `json:"batch_size"`

	// BatchTimeout maximum time to wait before flushing a partial batch
	BatchTimeout time.Duration `json:"batch_timeout"`

	// WriteBarriers enables write barriers for ordering guarantees
	WriteBarriers bool `json:"write_barriers"`

	// VerifyWrites enables write verification after sync
	VerifyWrites bool `json:"verify_writes"`

	// BufferSize for async write buffering
	BufferSize int `json:"buffer_size"`

	// MaxRetries for failed write operations
	MaxRetries int `json:"max_retries"`

	// RetryDelay base delay for retry operations
	RetryDelay time.Duration `json:"retry_delay"`
}

// DefaultDurabilityConfig returns a default configuration
func DefaultDurabilityConfig() *DurabilityConfig {
	return &DurabilityConfig{
		Level:         DurabilityAsync,
		WriteMode:     WriteModeAsync,
		SyncWrites:    false,
		FlushInterval: 1 * time.Second,
		BatchSize:     100,
		BatchTimeout:  100 * time.Millisecond,
		WriteBarriers: false,
		VerifyWrites:  false,
		BufferSize:    64 * 1024, // 64KB
		MaxRetries:    3,
		RetryDelay:    10 * time.Millisecond,
	}
}

// SyncDurabilityConfig returns a configuration for synchronous writes
func SyncDurabilityConfig() *DurabilityConfig {
	return &DurabilityConfig{
		Level:         DurabilitySync,
		WriteMode:     WriteModeSync,
		SyncWrites:    true,
		FlushInterval: 0, // Not used in sync mode
		BatchSize:     1,
		BatchTimeout:  0, // Not used in sync mode
		WriteBarriers: true,
		VerifyWrites:  false,
		BufferSize:    0, // No buffering in sync mode
		MaxRetries:    3,
		RetryDelay:    10 * time.Millisecond,
	}
}

// StrictDurabilityConfig returns a configuration for strict durability
func StrictDurabilityConfig() *DurabilityConfig {
	return &DurabilityConfig{
		Level:         DurabilityStrict,
		WriteMode:     WriteModeSync,
		SyncWrites:    true,
		FlushInterval: 0, // Not used in sync mode
		BatchSize:     1,
		BatchTimeout:  0, // Not used in sync mode
		WriteBarriers: true,
		VerifyWrites:  true,
		BufferSize:    0, // No buffering in strict mode
		MaxRetries:    5,
		RetryDelay:    50 * time.Millisecond,
	}
}

// Validate checks if the configuration is valid
func (c *DurabilityConfig) Validate() error {
	if c.Level < DurabilityNone || c.Level > DurabilityStrict {
		return fmt.Errorf("invalid durability level: %d", c.Level)
	}

	if c.WriteMode < WriteModeAsync || c.WriteMode > WriteModeBatch {
		return fmt.Errorf("invalid write mode: %d", c.WriteMode)
	}

	if c.FlushInterval < 0 {
		return fmt.Errorf("flush interval cannot be negative: %v", c.FlushInterval)
	}

	if c.BatchSize <= 0 {
		return fmt.Errorf("batch size must be positive: %d", c.BatchSize)
	}

	if c.BatchTimeout < 0 {
		return fmt.Errorf("batch timeout cannot be negative: %v", c.BatchTimeout)
	}

	if c.BufferSize < 0 {
		return fmt.Errorf("buffer size cannot be negative: %d", c.BufferSize)
	}

	if c.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be negative: %d", c.MaxRetries)
	}

	if c.RetryDelay < 0 {
		return fmt.Errorf("retry delay cannot be negative: %v", c.RetryDelay)
	}

	// Validate consistency between level and other settings
	switch c.Level {
	case DurabilityNone:
		// No restrictions for no durability
	case DurabilityAsync:
		if c.SyncWrites {
			return fmt.Errorf("async durability level cannot have sync writes enabled")
		}
	case DurabilitySync:
		if !c.SyncWrites {
			return fmt.Errorf("sync durability level must have sync writes enabled")
		}
	case DurabilityStrict:
		if !c.SyncWrites || !c.WriteBarriers {
			return fmt.Errorf("strict durability level must have sync writes and write barriers enabled")
		}
	}

	return nil
}

// String returns a string representation of the durability level
func (d DurabilityLevel) String() string {
	switch d {
	case DurabilityNone:
		return "none"
	case DurabilityAsync:
		return "async"
	case DurabilitySync:
		return "sync"
	case DurabilityStrict:
		return "strict"
	default:
		return fmt.Sprintf("unknown(%d)", d)
	}
}

// String returns a string representation of the write mode
func (w WriteMode) String() string {
	switch w {
	case WriteModeAsync:
		return "async"
	case WriteModeSync:
		return "sync"
	case WriteModeBatch:
		return "batch"
	default:
		return fmt.Sprintf("unknown(%d)", w)
	}
}

// IsAsync returns true if the configuration uses asynchronous writes
func (c *DurabilityConfig) IsAsync() bool {
	return c.WriteMode == WriteModeAsync && !c.SyncWrites
}

// IsSync returns true if the configuration uses synchronous writes
func (c *DurabilityConfig) IsSync() bool {
	return c.WriteMode == WriteModeSync || c.SyncWrites
}

// RequiresFlush returns true if the configuration requires periodic flushing
func (c *DurabilityConfig) RequiresFlush() bool {
	return c.IsAsync() && c.FlushInterval > 0
}

// RequiresBarriers returns true if write barriers are required
func (c *DurabilityConfig) RequiresBarriers() bool {
	return c.WriteBarriers
}

// RequiresVerification returns true if write verification is required
func (c *DurabilityConfig) RequiresVerification() bool {
	return c.VerifyWrites
}
