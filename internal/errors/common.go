// Package errors provides common error handling utilities
package errors

import (
	"fmt"
	"runtime"
	"time"
)

// ErrorType represents different categories of errors
type ErrorType string

const (
	// Storage errors
	ErrorTypeStorage     ErrorType = "storage"
	ErrorTypeTransaction ErrorType = "transaction"
	ErrorTypeIterator    ErrorType = "iterator"

	// Concurrency errors
	ErrorTypeLock     ErrorType = "lock"
	ErrorTypeDeadlock ErrorType = "deadlock"

	// Cache errors
	ErrorTypeCache        ErrorType = "cache"
	ErrorTypeInvalidation ErrorType = "invalidation"

	// API errors
	ErrorTypeAPI        ErrorType = "api"
	ErrorTypeValidation ErrorType = "validation"

	// System errors
	ErrorTypeSystem   ErrorType = "system"
	ErrorTypeResource ErrorType = "resource"
)

// Severity represents error severity levels
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// MantisError represents a structured error with additional context
type MantisError struct {
	Type      ErrorType              `json:"type"`
	Severity  Severity               `json:"severity"`
	Message   string                 `json:"message"`
	Cause     error                  `json:"cause,omitempty"`
	Context   map[string]interface{} `json:"context,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Stack     string                 `json:"stack,omitempty"`
}

// Error implements the error interface
func (e *MantisError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s:%s] %s: %v", e.Type, e.Severity, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s:%s] %s", e.Type, e.Severity, e.Message)
}

// Unwrap returns the underlying cause
func (e *MantisError) Unwrap() error {
	return e.Cause
}

// WithContext adds context information to the error
func (e *MantisError) WithContext(key string, value interface{}) *MantisError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// NewError creates a new MantisError
func NewError(errorType ErrorType, severity Severity, message string) *MantisError {
	return &MantisError{
		Type:      errorType,
		Severity:  severity,
		Message:   message,
		Timestamp: time.Now(),
		Stack:     getStack(),
	}
}

// WrapError wraps an existing error with additional context
func WrapError(err error, errorType ErrorType, severity Severity, message string) *MantisError {
	return &MantisError{
		Type:      errorType,
		Severity:  severity,
		Message:   message,
		Cause:     err,
		Timestamp: time.Now(),
		Stack:     getStack(),
	}
}

// getStack captures the current stack trace
func getStack() string {
	buf := make([]byte, 1024)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// Common error constructors for different components

// Storage errors
func NewStorageError(message string) *MantisError {
	return NewError(ErrorTypeStorage, SeverityMedium, message)
}

func NewTransactionError(message string) *MantisError {
	return NewError(ErrorTypeTransaction, SeverityHigh, message)
}

func NewIteratorError(message string) *MantisError {
	return NewError(ErrorTypeIterator, SeverityLow, message)
}

// Concurrency errors
func NewLockError(message string) *MantisError {
	return NewError(ErrorTypeLock, SeverityHigh, message)
}

func NewDeadlockError(message string) *MantisError {
	return NewError(ErrorTypeDeadlock, SeverityCritical, message)
}

// Cache errors
func NewCacheError(message string) *MantisError {
	return NewError(ErrorTypeCache, SeverityMedium, message)
}

func NewInvalidationError(message string) *MantisError {
	return NewError(ErrorTypeInvalidation, SeverityMedium, message)
}

// API errors
func NewAPIError(message string) *MantisError {
	return NewError(ErrorTypeAPI, SeverityMedium, message)
}

func NewValidationError(message string) *MantisError {
	return NewError(ErrorTypeValidation, SeverityLow, message)
}

// System errors
func NewSystemError(message string) *MantisError {
	return NewError(ErrorTypeSystem, SeverityCritical, message)
}

func NewResourceError(message string) *MantisError {
	return NewError(ErrorTypeResource, SeverityHigh, message)
}

// Error checking utilities

// IsStorageError checks if an error is a storage-related error
func IsStorageError(err error) bool {
	if mantisErr, ok := err.(*MantisError); ok {
		return mantisErr.Type == ErrorTypeStorage ||
			mantisErr.Type == ErrorTypeTransaction ||
			mantisErr.Type == ErrorTypeIterator
	}
	return false
}

// IsConcurrencyError checks if an error is a concurrency-related error
func IsConcurrencyError(err error) bool {
	if mantisErr, ok := err.(*MantisError); ok {
		return mantisErr.Type == ErrorTypeLock ||
			mantisErr.Type == ErrorTypeDeadlock
	}
	return false
}

// IsCacheError checks if an error is a cache-related error
func IsCacheError(err error) bool {
	if mantisErr, ok := err.(*MantisError); ok {
		return mantisErr.Type == ErrorTypeCache ||
			mantisErr.Type == ErrorTypeInvalidation
	}
	return false
}

// IsCritical checks if an error is critical
func IsCritical(err error) bool {
	if mantisErr, ok := err.(*MantisError); ok {
		return mantisErr.Severity == SeverityCritical
	}
	return false
}

// ErrorHandler provides centralized error handling
type ErrorHandler struct {
	handlers map[ErrorType]func(*MantisError)
}

// NewErrorHandler creates a new error handler
func NewErrorHandler() *ErrorHandler {
	return &ErrorHandler{
		handlers: make(map[ErrorType]func(*MantisError)),
	}
}

// RegisterHandler registers a handler for a specific error type
func (eh *ErrorHandler) RegisterHandler(errorType ErrorType, handler func(*MantisError)) {
	eh.handlers[errorType] = handler
}

// Handle processes an error using the appropriate handler
func (eh *ErrorHandler) Handle(err error) {
	if mantisErr, ok := err.(*MantisError); ok {
		if handler, exists := eh.handlers[mantisErr.Type]; exists {
			handler(mantisErr)
		}
	}
}

// RetryableError represents an error that can be retried
type RetryableError struct {
	*MantisError
	MaxRetries int
	RetryCount int
}

// NewRetryableError creates a new retryable error
func NewRetryableError(err *MantisError, maxRetries int) *RetryableError {
	return &RetryableError{
		MantisError: err,
		MaxRetries:  maxRetries,
		RetryCount:  0,
	}
}

// CanRetry checks if the error can be retried
func (re *RetryableError) CanRetry() bool {
	return re.RetryCount < re.MaxRetries
}

// IncrementRetry increments the retry count
func (re *RetryableError) IncrementRetry() {
	re.RetryCount++
}
