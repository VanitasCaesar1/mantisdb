package errors

import (
	"fmt"
	"time"
)

// ErrorSeverity represents the severity level of an error
type ErrorSeverity int

const (
	ErrorSeverityLow ErrorSeverity = iota
	ErrorSeverityMedium
	ErrorSeverityHigh
	ErrorSeverityCritical
)

func (s ErrorSeverity) String() string {
	switch s {
	case ErrorSeverityLow:
		return "LOW"
	case ErrorSeverityMedium:
		return "MEDIUM"
	case ErrorSeverityHigh:
		return "HIGH"
	case ErrorSeverityCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// ErrorCategory represents the category of an error
type ErrorCategory int

const (
	ErrorCategoryIO ErrorCategory = iota
	ErrorCategoryMemory
	ErrorCategoryDisk
	ErrorCategoryCorruption
	ErrorCategoryTransaction
	ErrorCategoryWAL
	ErrorCategoryNetwork
	ErrorCategorySystem
)

func (c ErrorCategory) String() string {
	switch c {
	case ErrorCategoryIO:
		return "IO"
	case ErrorCategoryMemory:
		return "MEMORY"
	case ErrorCategoryDisk:
		return "DISK"
	case ErrorCategoryCorruption:
		return "CORRUPTION"
	case ErrorCategoryTransaction:
		return "TRANSACTION"
	case ErrorCategoryWAL:
		return "WAL"
	case ErrorCategoryNetwork:
		return "NETWORK"
	case ErrorCategorySystem:
		return "SYSTEM"
	default:
		return "UNKNOWN"
	}
}

// ErrorAction represents the action to take when handling an error
type ErrorAction int

const (
	ErrorActionRetry ErrorAction = iota
	ErrorActionFail
	ErrorActionDegrade
	ErrorActionRecover
	ErrorActionShutdown
)

func (a ErrorAction) String() string {
	switch a {
	case ErrorActionRetry:
		return "RETRY"
	case ErrorActionFail:
		return "FAIL"
	case ErrorActionDegrade:
		return "DEGRADE"
	case ErrorActionRecover:
		return "RECOVER"
	case ErrorActionShutdown:
		return "SHUTDOWN"
	default:
		return "UNKNOWN"
	}
}

// ErrorContext provides context information about an error
type ErrorContext struct {
	Operation   string                 `json:"operation"`
	Resource    string                 `json:"resource"`
	Severity    ErrorSeverity          `json:"severity"`
	Category    ErrorCategory          `json:"category"`
	Recoverable bool                   `json:"recoverable"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// MantisError represents a classified error with context
type MantisError struct {
	Err     error        `json:"error"`
	Context ErrorContext `json:"context"`
}

func (e *MantisError) Error() string {
	return fmt.Sprintf("[%s:%s] %s - %s",
		e.Context.Category,
		e.Context.Severity,
		e.Context.Operation,
		e.Err.Error())
}

func (e *MantisError) Unwrap() error {
	return e.Err
}

// ErrorHandler interface defines the contract for error handling
type ErrorHandler interface {
	// Error handling
	HandleError(err error, context ErrorContext) ErrorAction
	RecoverFromError(err error, context ErrorContext) error

	// Resource exhaustion
	HandleDiskFull(operation string) error
	HandleMemoryExhaustion(operation string) error
	HandleIOError(err error, retryCount int) error

	// Corruption handling
	HandleCorruption(corruptionInfo CorruptionInfo) error
	IsolateCorruptedData(location DataLocation) error

	// Classification
	ClassifyError(err error) ErrorContext

	// Recovery strategies
	GetRecoveryStrategy(context ErrorContext) RecoveryStrategy
}

// CorruptionInfo contains information about detected corruption
type CorruptionInfo struct {
	Location    DataLocation `json:"location"`
	Type        string       `json:"type"`
	Description string       `json:"description"`
	Timestamp   time.Time    `json:"timestamp"`
	Checksum    uint32       `json:"checksum,omitempty"`
}

// DataLocation represents the location of corrupted data
type DataLocation struct {
	File   string `json:"file"`
	Offset int64  `json:"offset"`
	Size   int64  `json:"size"`
}

// RecoveryStrategy defines how to recover from an error
type RecoveryStrategy struct {
	Action      ErrorAction   `json:"action"`
	MaxRetries  int           `json:"max_retries"`
	RetryDelay  time.Duration `json:"retry_delay"`
	Timeout     time.Duration `json:"timeout"`
	Recoverable bool          `json:"recoverable"`
}
