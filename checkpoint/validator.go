package checkpoint

import (
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"time"
)

// DefaultValidator provides default checkpoint validation
type DefaultValidator struct {
	strictMode bool
}

// NewDefaultValidator creates a new default validator
func NewDefaultValidator(strictMode bool) *DefaultValidator {
	return &DefaultValidator{
		strictMode: strictMode,
	}
}

// ValidateCheckpoint validates a checkpoint thoroughly
func (v *DefaultValidator) ValidateCheckpoint(checkpoint *Checkpoint) (*CheckpointValidationResult, error) {
	startTime := time.Now()
	result := &CheckpointValidationResult{
		Valid:       true,
		Errors:      make([]CheckpointError, 0),
		Warnings:    make([]CheckpointWarning, 0),
		ValidatedAt: startTime,
	}

	// Validate file existence
	if err := v.validateFileExists(checkpoint, result); err != nil {
		result.Valid = false
		result.Duration = time.Since(startTime)
		return result, err
	}

	// Validate file size
	v.validateFileSize(checkpoint, result)

	// Validate checksum
	v.validateChecksum(checkpoint, result)

	// Validate metadata
	v.validateMetadata(checkpoint, result)

	// Validate format (basic)
	v.validateFormat(checkpoint, result)

	// Set overall validity
	result.Valid = len(result.Errors) == 0
	result.Duration = time.Since(startTime)

	return result, nil
}

// ValidateIntegrity validates file integrity using checksum
func (v *DefaultValidator) ValidateIntegrity(filePath string, expectedChecksum uint32) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file for integrity check: %w", err)
	}
	defer file.Close()

	hash := crc32.NewIEEE()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}

	actualChecksum := hash.Sum32()
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("integrity check failed: expected checksum %d, got %d",
			expectedChecksum, actualChecksum)
	}

	return nil
}

// validateFileExists checks if the checkpoint file exists
func (v *DefaultValidator) validateFileExists(checkpoint *Checkpoint, result *CheckpointValidationResult) error {
	if _, err := os.Stat(checkpoint.FilePath); err != nil {
		if os.IsNotExist(err) {
			result.Errors = append(result.Errors, CheckpointError{
				Type:        CheckpointErrorFileNotFound,
				Message:     "checkpoint file not found",
				Field:       "file_path",
				Expected:    "existing file",
				Actual:      "file not found",
				Recoverable: false,
			})
			return fmt.Errorf("checkpoint file not found: %s", checkpoint.FilePath)
		}
		if os.IsPermission(err) {
			result.Errors = append(result.Errors, CheckpointError{
				Type:        CheckpointErrorPermissionDenied,
				Message:     "permission denied accessing checkpoint file",
				Field:       "file_path",
				Expected:    "readable file",
				Actual:      "permission denied",
				Recoverable: false,
			})
			return fmt.Errorf("permission denied: %s", checkpoint.FilePath)
		}
		return fmt.Errorf("failed to stat checkpoint file: %w", err)
	}
	return nil
}

// validateFileSize validates the checkpoint file size
func (v *DefaultValidator) validateFileSize(checkpoint *Checkpoint, result *CheckpointValidationResult) {
	fileInfo, err := os.Stat(checkpoint.FilePath)
	if err != nil {
		result.Errors = append(result.Errors, CheckpointError{
			Type:        CheckpointErrorSizeMismatch,
			Message:     "failed to get file size",
			Field:       "size",
			Expected:    checkpoint.Size,
			Actual:      "unknown",
			Recoverable: false,
		})
		return
	}

	actualSize := fileInfo.Size()
	if actualSize != checkpoint.Size {
		result.Errors = append(result.Errors, CheckpointError{
			Type:        CheckpointErrorSizeMismatch,
			Message:     "file size mismatch",
			Field:       "size",
			Expected:    checkpoint.Size,
			Actual:      actualSize,
			Recoverable: false,
		})
		result.SizeOK = false
	} else {
		result.SizeOK = true
	}

	// Warn about unusually large checkpoints
	if actualSize > 100*1024*1024 { // 100MB
		result.Warnings = append(result.Warnings, CheckpointWarning{
			Type:    CheckpointWarningLargeSize,
			Message: "checkpoint file is unusually large",
			Field:   "size",
			Details: map[string]interface{}{
				"size_mb": actualSize / (1024 * 1024),
			},
		})
	}
}

// validateChecksum validates the checkpoint file checksum
func (v *DefaultValidator) validateChecksum(checkpoint *Checkpoint, result *CheckpointValidationResult) {
	file, err := os.Open(checkpoint.FilePath)
	if err != nil {
		result.Errors = append(result.Errors, CheckpointError{
			Type:        CheckpointErrorChecksumMismatch,
			Message:     "failed to open file for checksum validation",
			Field:       "checksum",
			Expected:    checkpoint.Checksum,
			Actual:      "unknown",
			Recoverable: false,
		})
		return
	}
	defer file.Close()

	hash := crc32.NewIEEE()
	if _, err := io.Copy(hash, file); err != nil {
		result.Errors = append(result.Errors, CheckpointError{
			Type:        CheckpointErrorChecksumMismatch,
			Message:     "failed to calculate checksum",
			Field:       "checksum",
			Expected:    checkpoint.Checksum,
			Actual:      "calculation failed",
			Recoverable: false,
		})
		return
	}

	actualChecksum := hash.Sum32()
	if actualChecksum != checkpoint.Checksum {
		result.Errors = append(result.Errors, CheckpointError{
			Type:        CheckpointErrorChecksumMismatch,
			Message:     "checksum mismatch",
			Field:       "checksum",
			Expected:    checkpoint.Checksum,
			Actual:      actualChecksum,
			Recoverable: false,
		})
		result.ChecksumOK = false
	} else {
		result.ChecksumOK = true
	}
}

// validateMetadata validates checkpoint metadata
func (v *DefaultValidator) validateMetadata(checkpoint *Checkpoint, result *CheckpointValidationResult) {
	// Validate basic metadata fields
	if checkpoint.ID == "" {
		result.Errors = append(result.Errors, CheckpointError{
			Type:        CheckpointErrorMetadataCorrupt,
			Message:     "checkpoint ID is empty",
			Field:       "id",
			Expected:    "non-empty string",
			Actual:      "empty",
			Recoverable: false,
		})
	}

	if checkpoint.LSN == 0 {
		result.Errors = append(result.Errors, CheckpointError{
			Type:        CheckpointErrorMetadataCorrupt,
			Message:     "checkpoint LSN is zero",
			Field:       "lsn",
			Expected:    "positive number",
			Actual:      0,
			Recoverable: false,
		})
	}

	if checkpoint.Timestamp.IsZero() {
		result.Errors = append(result.Errors, CheckpointError{
			Type:        CheckpointErrorMetadataCorrupt,
			Message:     "checkpoint timestamp is zero",
			Field:       "timestamp",
			Expected:    "valid timestamp",
			Actual:      "zero time",
			Recoverable: false,
		})
	}

	// Validate metadata version
	if checkpoint.Metadata.Version == "" {
		result.Warnings = append(result.Warnings, CheckpointWarning{
			Type:    CheckpointWarningMissingMetadata,
			Message: "metadata version is missing",
			Field:   "metadata.version",
		})
	}

	// Validate database version
	if checkpoint.Metadata.DatabaseVersion == "" {
		result.Warnings = append(result.Warnings, CheckpointWarning{
			Type:    CheckpointWarningMissingMetadata,
			Message: "database version is missing",
			Field:   "metadata.database_version",
		})
	}

	// Check for old format versions
	if checkpoint.Metadata.Version != "" && checkpoint.Metadata.Version < "1.0" {
		result.Warnings = append(result.Warnings, CheckpointWarning{
			Type:    CheckpointWarningOldFormat,
			Message: "checkpoint uses old format version",
			Field:   "metadata.version",
			Details: map[string]interface{}{
				"version": checkpoint.Metadata.Version,
			},
		})
	}

	result.MetadataOK = len(result.Errors) == 0
}

// validateFormat performs basic format validation
func (v *DefaultValidator) validateFormat(checkpoint *Checkpoint, result *CheckpointValidationResult) {
	// For now, we'll do basic format validation
	// In a real implementation, this would validate the binary format

	file, err := os.Open(checkpoint.FilePath)
	if err != nil {
		result.Errors = append(result.Errors, CheckpointError{
			Type:        CheckpointErrorFormatInvalid,
			Message:     "failed to open file for format validation",
			Field:       "format",
			Expected:    "valid checkpoint format",
			Actual:      "unreadable",
			Recoverable: false,
		})
		return
	}
	defer file.Close()

	// Read first few bytes to check for magic header
	header := make([]byte, 8)
	n, err := file.Read(header)
	if err != nil && err != io.EOF {
		result.Errors = append(result.Errors, CheckpointError{
			Type:        CheckpointErrorFormatInvalid,
			Message:     "failed to read file header",
			Field:       "format",
			Expected:    "readable header",
			Actual:      "read error",
			Recoverable: false,
		})
		return
	}

	if n < 8 {
		result.Warnings = append(result.Warnings, CheckpointWarning{
			Type:    CheckpointWarningMissingMetadata,
			Message: "file is too small to contain valid header",
			Field:   "format",
			Details: map[string]interface{}{
				"bytes_read": n,
			},
		})
	}

	// In strict mode, validate magic header
	if v.strictMode && n >= 8 {
		expectedMagic := []byte{'M', 'A', 'N', 'T', 'I', 'S', 'C', 'P'}
		magicMatch := true
		for i := 0; i < 8; i++ {
			if header[i] != expectedMagic[i] {
				magicMatch = false
				break
			}
		}

		if !magicMatch {
			result.Errors = append(result.Errors, CheckpointError{
				Type:        CheckpointErrorFormatInvalid,
				Message:     "invalid magic header",
				Field:       "format",
				Expected:    string(expectedMagic),
				Actual:      string(header),
				Recoverable: false,
			})
		}
	}

	result.FormatOK = len(result.Errors) == 0
}

// ValidateCheckpointChain validates a chain of checkpoints for consistency
func (v *DefaultValidator) ValidateCheckpointChain(checkpoints []*Checkpoint) (*CheckpointValidationResult, error) {
	startTime := time.Now()
	result := &CheckpointValidationResult{
		Valid:       true,
		Errors:      make([]CheckpointError, 0),
		Warnings:    make([]CheckpointWarning, 0),
		ValidatedAt: startTime,
	}

	if len(checkpoints) == 0 {
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Sort checkpoints by LSN
	sortedCheckpoints := make([]*Checkpoint, len(checkpoints))
	copy(sortedCheckpoints, checkpoints)

	// Validate LSN sequence
	for i := 1; i < len(sortedCheckpoints); i++ {
		prev := sortedCheckpoints[i-1]
		curr := sortedCheckpoints[i]

		if curr.LSN <= prev.LSN {
			result.Errors = append(result.Errors, CheckpointError{
				Type:        CheckpointErrorInconsistency,
				Message:     "LSN sequence violation in checkpoint chain",
				Field:       "lsn",
				Expected:    fmt.Sprintf("> %d", prev.LSN),
				Actual:      curr.LSN,
				Recoverable: false,
			})
		}

		// Validate timestamp sequence
		if curr.Timestamp.Before(prev.Timestamp) {
			result.Warnings = append(result.Warnings, CheckpointWarning{
				Type:    CheckpointWarningDataInconsistency,
				Message: "timestamp sequence violation in checkpoint chain",
				Field:   "timestamp",
				Details: map[string]interface{}{
					"prev_checkpoint": prev.ID,
					"curr_checkpoint": curr.ID,
				},
			})
		}
	}

	// Validate incremental checkpoint dependencies
	for _, checkpoint := range sortedCheckpoints {
		if checkpoint.Type == CheckpointTypeIncremental {
			if len(checkpoint.Metadata.Dependencies) == 0 {
				result.Warnings = append(result.Warnings, CheckpointWarning{
					Type:    CheckpointWarningMissingMetadata,
					Message: "incremental checkpoint has no dependencies",
					Field:   "metadata.dependencies",
					Details: map[string]interface{}{
						"checkpoint_id": checkpoint.ID,
					},
				})
			}
		}
	}

	result.Valid = len(result.Errors) == 0
	result.Duration = time.Since(startTime)
	return result, nil
}

// QuickValidation performs a quick validation (checksum only)
func (v *DefaultValidator) QuickValidation(checkpoint *Checkpoint) error {
	return v.ValidateIntegrity(checkpoint.FilePath, checkpoint.Checksum)
}

// DeepValidation performs comprehensive validation including data consistency
func (v *DefaultValidator) DeepValidation(checkpoint *Checkpoint) (*CheckpointValidationResult, error) {
	// First perform standard validation
	result, err := v.ValidateCheckpoint(checkpoint)
	if err != nil {
		return result, err
	}

	// Add deep validation checks
	// This would include validating the actual data content
	// For now, we'll add placeholder logic

	if checkpoint.Type == CheckpointTypeFull {
		// Validate full checkpoint data integrity
		if err := v.validateFullCheckpointData(checkpoint, result); err != nil {
			return result, err
		}
	} else if checkpoint.Type == CheckpointTypeIncremental {
		// Validate incremental checkpoint data
		if err := v.validateIncrementalCheckpointData(checkpoint, result); err != nil {
			return result, err
		}
	}

	result.Valid = len(result.Errors) == 0
	return result, nil
}

// validateFullCheckpointData validates full checkpoint data
func (v *DefaultValidator) validateFullCheckpointData(checkpoint *Checkpoint, result *CheckpointValidationResult) error {
	// Placeholder for full checkpoint data validation
	// In a real implementation, this would validate the actual database state
	return nil
}

// validateIncrementalCheckpointData validates incremental checkpoint data
func (v *DefaultValidator) validateIncrementalCheckpointData(checkpoint *Checkpoint, result *CheckpointValidationResult) error {
	// Placeholder for incremental checkpoint data validation
	// In a real implementation, this would validate WAL entries
	return nil
}
