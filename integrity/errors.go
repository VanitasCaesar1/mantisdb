package integrity

import "fmt"

// ChecksumMismatchError represents a checksum verification failure
type ChecksumMismatchError struct {
	Expected    uint32 `json:"expected"`
	Actual      uint32 `json:"actual"`
	ExpectedStr string `json:"expected_str,omitempty"`
	ActualStr   string `json:"actual_str,omitempty"`
	Size        int64  `json:"size"`
}

func (e *ChecksumMismatchError) Error() string {
	if e.ExpectedStr != "" && e.ActualStr != "" {
		return fmt.Sprintf("checksum mismatch: expected %s, got %s (size: %d bytes)",
			e.ExpectedStr, e.ActualStr, e.Size)
	}
	return fmt.Sprintf("checksum mismatch: expected %08x, got %08x (size: %d bytes)",
		e.Expected, e.Actual, e.Size)
}

// FileChecksumMismatchError represents a file checksum verification failure
type FileChecksumMismatchError struct {
	FilePath string `json:"file_path"`
	Expected uint32 `json:"expected"`
	Actual   uint32 `json:"actual"`
}

func (e *FileChecksumMismatchError) Error() string {
	return fmt.Sprintf("file checksum mismatch for %s: expected %08x, got %08x",
		e.FilePath, e.Expected, e.Actual)
}

// CorruptionDetectedError represents detected data corruption
type CorruptionDetectedError struct {
	Location    string `json:"location"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Checksum    uint32 `json:"checksum,omitempty"`
}

func (e *CorruptionDetectedError) Error() string {
	return fmt.Sprintf("corruption detected at %s (%s): %s",
		e.Location, e.Type, e.Description)
}

// IntegrityViolationError represents a general integrity violation
type IntegrityViolationError struct {
	Component   string                 `json:"component"`
	Operation   string                 `json:"operation"`
	Description string                 `json:"description"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

func (e *IntegrityViolationError) Error() string {
	return fmt.Sprintf("integrity violation in %s during %s: %s",
		e.Component, e.Operation, e.Description)
}
