package checkpoint

import (
	"time"
)

// CheckpointID represents a unique identifier for a checkpoint
type CheckpointID string

// CheckpointStatus represents the status of a checkpoint
type CheckpointStatus int

const (
	CheckpointStatusCreating CheckpointStatus = iota
	CheckpointStatusCompleted
	CheckpointStatusFailed
	CheckpointStatusValidating
	CheckpointStatusCorrupted
)

// CheckpointType defines the type of checkpoint
type CheckpointType int

const (
	CheckpointTypeFull CheckpointType = iota
	CheckpointTypeIncremental
	CheckpointTypeSnapshot
)

// Checkpoint represents a checkpoint with metadata
type Checkpoint struct {
	ID          CheckpointID     `json:"id"`
	Type        CheckpointType   `json:"type"`
	Status      CheckpointStatus `json:"status"`
	LSN         uint64           `json:"lsn"`          // Last LSN included in checkpoint
	Timestamp   time.Time        `json:"timestamp"`    // When checkpoint was created
	Size        int64            `json:"size"`         // Size in bytes
	FilePath    string           `json:"file_path"`    // Path to checkpoint file
	Metadata    CheckpointMeta   `json:"metadata"`     // Additional metadata
	Checksum    uint32           `json:"checksum"`     // Integrity checksum
	CreatedBy   string           `json:"created_by"`   // Component that created checkpoint
	ValidatedAt *time.Time       `json:"validated_at"` // When checkpoint was last validated
}

// CheckpointMeta contains additional checkpoint metadata
type CheckpointMeta struct {
	Version          string            `json:"version"`           // Checkpoint format version
	DatabaseVersion  string            `json:"database_version"`  // Database version when created
	TransactionCount int               `json:"transaction_count"` // Number of transactions
	OperationCount   int               `json:"operation_count"`   // Number of operations
	CompressionType  string            `json:"compression_type"`  // Compression used
	EncryptionType   string            `json:"encryption_type"`   // Encryption used
	Tags             map[string]string `json:"tags"`              // Custom tags
	Dependencies     []CheckpointID    `json:"dependencies"`      // Dependent checkpoints
}

// CheckpointIndex maintains an index of all checkpoints
type CheckpointIndex struct {
	Checkpoints map[CheckpointID]*Checkpoint `json:"checkpoints"`
	LatestLSN   uint64                       `json:"latest_lsn"`
	CreatedAt   time.Time                    `json:"created_at"`
	UpdatedAt   time.Time                    `json:"updated_at"`
	Version     string                       `json:"version"`
}

// CheckpointConfig holds configuration for checkpoint operations
type CheckpointConfig struct {
	// Directory where checkpoints are stored
	CheckpointDir string `json:"checkpoint_dir"`

	// Maximum number of checkpoints to retain
	MaxCheckpoints int `json:"max_checkpoints"`

	// Frequency of automatic checkpoints
	CheckpointInterval time.Duration `json:"checkpoint_interval"`

	// LSN interval for checkpoints (create checkpoint every N LSNs)
	LSNInterval uint64 `json:"lsn_interval"`

	// Enable compression for checkpoints
	EnableCompression bool `json:"enable_compression"`

	// Compression algorithm to use
	CompressionAlgorithm string `json:"compression_algorithm"`

	// Enable encryption for checkpoints
	EnableEncryption bool `json:"enable_encryption"`

	// Encryption key for checkpoints
	EncryptionKey []byte `json:"-"` // Not serialized for security

	// Validation settings
	ValidateOnCreate bool `json:"validate_on_create"`
	ValidateOnLoad   bool `json:"validate_on_load"`

	// Performance settings
	BufferSize       int  `json:"buffer_size"`
	ParallelCreation bool `json:"parallel_creation"`
	MaxWorkers       int  `json:"max_workers"`

	// Cleanup settings
	AutoCleanup     bool          `json:"auto_cleanup"`
	CleanupInterval time.Duration `json:"cleanup_interval"`
	RetentionPeriod time.Duration `json:"retention_period"`
	MinCheckpoints  int           `json:"min_checkpoints"`
	MaxDiskUsage    int64         `json:"max_disk_usage"`
}

// CheckpointStats contains statistics about checkpoint operations
type CheckpointStats struct {
	TotalCheckpoints    int           `json:"total_checkpoints"`
	SuccessfulCreations int           `json:"successful_creations"`
	FailedCreations     int           `json:"failed_creations"`
	TotalSize           int64         `json:"total_size"`
	AverageSize         int64         `json:"average_size"`
	LastCheckpointTime  time.Time     `json:"last_checkpoint_time"`
	AverageCreateTime   time.Duration `json:"average_create_time"`
	ValidationFailures  int           `json:"validation_failures"`
	CorruptedCount      int           `json:"corrupted_count"`
	CleanupCount        int           `json:"cleanup_count"`
}

// CheckpointValidationResult represents the result of checkpoint validation
type CheckpointValidationResult struct {
	Valid       bool                `json:"valid"`
	Errors      []CheckpointError   `json:"errors"`
	Warnings    []CheckpointWarning `json:"warnings"`
	ChecksumOK  bool                `json:"checksum_ok"`
	SizeOK      bool                `json:"size_ok"`
	FormatOK    bool                `json:"format_ok"`
	MetadataOK  bool                `json:"metadata_ok"`
	ValidatedAt time.Time           `json:"validated_at"`
	Duration    time.Duration       `json:"duration"`
}

// CheckpointError represents an error found during validation
type CheckpointError struct {
	Type        CheckpointErrorType `json:"type"`
	Message     string              `json:"message"`
	Field       string              `json:"field"`
	Expected    interface{}         `json:"expected"`
	Actual      interface{}         `json:"actual"`
	Recoverable bool                `json:"recoverable"`
}

// CheckpointWarning represents a warning found during validation
type CheckpointWarning struct {
	Type    CheckpointWarningType  `json:"type"`
	Message string                 `json:"message"`
	Field   string                 `json:"field"`
	Details map[string]interface{} `json:"details"`
}

// CheckpointErrorType defines types of checkpoint errors
type CheckpointErrorType int

const (
	CheckpointErrorChecksumMismatch CheckpointErrorType = iota
	CheckpointErrorSizeMismatch
	CheckpointErrorFormatInvalid
	CheckpointErrorMetadataCorrupt
	CheckpointErrorFileNotFound
	CheckpointErrorPermissionDenied
	CheckpointErrorDiskFull
	CheckpointErrorCorrupted
	CheckpointErrorInconsistency
)

// CheckpointWarningType defines types of checkpoint warnings
type CheckpointWarningType int

const (
	CheckpointWarningOldFormat CheckpointWarningType = iota
	CheckpointWarningLargeSize
	CheckpointWarningSlowCreation
	CheckpointWarningMissingMetadata
	CheckpointWarningDeprecatedFeature
	CheckpointWarningDataInconsistency
)

// String returns string representation of checkpoint status
func (s CheckpointStatus) String() string {
	switch s {
	case CheckpointStatusCreating:
		return "creating"
	case CheckpointStatusCompleted:
		return "completed"
	case CheckpointStatusFailed:
		return "failed"
	case CheckpointStatusValidating:
		return "validating"
	case CheckpointStatusCorrupted:
		return "corrupted"
	default:
		return "unknown"
	}
}

// String returns string representation of checkpoint type
func (t CheckpointType) String() string {
	switch t {
	case CheckpointTypeFull:
		return "full"
	case CheckpointTypeIncremental:
		return "incremental"
	case CheckpointTypeSnapshot:
		return "snapshot"
	default:
		return "unknown"
	}
}

// String returns string representation of checkpoint error type
func (e CheckpointErrorType) String() string {
	switch e {
	case CheckpointErrorChecksumMismatch:
		return "checksum_mismatch"
	case CheckpointErrorSizeMismatch:
		return "size_mismatch"
	case CheckpointErrorFormatInvalid:
		return "format_invalid"
	case CheckpointErrorMetadataCorrupt:
		return "metadata_corrupt"
	case CheckpointErrorFileNotFound:
		return "file_not_found"
	case CheckpointErrorPermissionDenied:
		return "permission_denied"
	case CheckpointErrorDiskFull:
		return "disk_full"
	case CheckpointErrorCorrupted:
		return "corrupted"
	default:
		return "unknown"
	}
}

// String returns string representation of checkpoint warning type
func (w CheckpointWarningType) String() string {
	switch w {
	case CheckpointWarningOldFormat:
		return "old_format"
	case CheckpointWarningLargeSize:
		return "large_size"
	case CheckpointWarningSlowCreation:
		return "slow_creation"
	case CheckpointWarningMissingMetadata:
		return "missing_metadata"
	case CheckpointWarningDeprecatedFeature:
		return "deprecated_feature"
	default:
		return "unknown"
	}
}

// IsCompleted returns true if checkpoint is completed successfully
func (c *Checkpoint) IsCompleted() bool {
	return c.Status == CheckpointStatusCompleted
}

// IsFailed returns true if checkpoint creation failed
func (c *Checkpoint) IsFailed() bool {
	return c.Status == CheckpointStatusFailed
}

// IsCorrupted returns true if checkpoint is corrupted
func (c *Checkpoint) IsCorrupted() bool {
	return c.Status == CheckpointStatusCorrupted
}

// Age returns the age of the checkpoint
func (c *Checkpoint) Age() time.Duration {
	return time.Since(c.Timestamp)
}
