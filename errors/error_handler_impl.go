package errors

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// DefaultErrorHandler implements the ErrorHandler interface
type DefaultErrorHandler struct {
	config *ErrorHandlerConfig
}

// ErrorHandlerConfig contains configuration for error handling
type ErrorHandlerConfig struct {
	MaxRetries                int           `json:"max_retries"`
	BaseRetryDelay            time.Duration `json:"base_retry_delay"`
	MaxRetryDelay             time.Duration `json:"max_retry_delay"`
	CircuitBreakerThreshold   int           `json:"circuit_breaker_threshold"`
	RecoveryTimeout           time.Duration `json:"recovery_timeout"`
	EnableGracefulDegradation bool          `json:"enable_graceful_degradation"`
}

// NewDefaultErrorHandler creates a new default error handler
func NewDefaultErrorHandler(config *ErrorHandlerConfig) *DefaultErrorHandler {
	if config == nil {
		config = &ErrorHandlerConfig{
			MaxRetries:                5,
			BaseRetryDelay:            100 * time.Millisecond,
			MaxRetryDelay:             30 * time.Second,
			CircuitBreakerThreshold:   5,
			RecoveryTimeout:           60 * time.Second,
			EnableGracefulDegradation: true,
		}
	}

	return &DefaultErrorHandler{
		config: config,
	}
}

// HandleError handles an error based on its context and returns the appropriate action
func (h *DefaultErrorHandler) HandleError(err error, context ErrorContext) ErrorAction {
	if err == nil {
		return ErrorActionRetry
	}

	// Classify the error if not already classified
	if context.Category == 0 && context.Severity == 0 {
		context = h.ClassifyError(err)
	}

	// Get recovery strategy based on error context
	strategy := h.GetRecoveryStrategy(context)

	// Log the error with context
	h.logError(err, context, strategy.Action)

	return strategy.Action
}

// RecoverFromError attempts to recover from an error
func (h *DefaultErrorHandler) RecoverFromError(err error, context ErrorContext) error {
	strategy := h.GetRecoveryStrategy(context)

	if !strategy.Recoverable {
		return fmt.Errorf("error is not recoverable: %w", err)
	}

	switch strategy.Action {
	case ErrorActionRetry:
		return h.retryWithBackoff(err, context, strategy)
	case ErrorActionRecover:
		return h.attemptRecovery(err, context)
	case ErrorActionDegrade:
		return h.gracefulDegrade(err, context)
	default:
		return fmt.Errorf("no recovery strategy for action %s: %w", strategy.Action, err)
	}
}

// HandleDiskFull handles disk space exhaustion
func (h *DefaultErrorHandler) HandleDiskFull(operation string) error {
	context := ErrorContext{
		Operation:   operation,
		Resource:    "disk",
		Severity:    ErrorSeverityCritical,
		Category:    ErrorCategoryDisk,
		Recoverable: false,
		Timestamp:   time.Now(),
		Metadata: map[string]interface{}{
			"disk_full": true,
		},
	}

	h.logError(errors.New("disk space exhausted"), context, ErrorActionFail)

	// Attempt cleanup if graceful degradation is enabled
	if h.config.EnableGracefulDegradation {
		return h.attemptDiskCleanup()
	}

	return fmt.Errorf("disk space exhausted during operation: %s", operation)
}

// HandleMemoryExhaustion handles memory exhaustion
func (h *DefaultErrorHandler) HandleMemoryExhaustion(operation string) error {
	context := ErrorContext{
		Operation:   operation,
		Resource:    "memory",
		Severity:    ErrorSeverityHigh,
		Category:    ErrorCategoryMemory,
		Recoverable: true,
		Timestamp:   time.Now(),
		Metadata: map[string]interface{}{
			"memory_exhausted": true,
		},
	}

	h.logError(errors.New("memory exhausted"), context, ErrorActionDegrade)

	// Attempt memory cleanup
	return h.attemptMemoryCleanup()
}

// HandleIOError handles I/O errors with retry logic
func (h *DefaultErrorHandler) HandleIOError(err error, retryCount int) error {
	context := h.ClassifyError(err)
	context.Metadata["retry_count"] = retryCount

	if retryCount >= h.config.MaxRetries {
		return fmt.Errorf("max retries exceeded for I/O operation: %w", err)
	}

	strategy := h.GetRecoveryStrategy(context)
	return h.retryWithBackoff(err, context, strategy)
}

// HandleCorruption handles data corruption
func (h *DefaultErrorHandler) HandleCorruption(corruptionInfo CorruptionInfo) error {
	context := ErrorContext{
		Operation:   "corruption_detected",
		Resource:    corruptionInfo.Location.File,
		Severity:    ErrorSeverityCritical,
		Category:    ErrorCategoryCorruption,
		Recoverable: false,
		Timestamp:   time.Now(),
		Metadata: map[string]interface{}{
			"corruption_type": corruptionInfo.Type,
			"location":        corruptionInfo.Location,
			"checksum":        corruptionInfo.Checksum,
		},
	}

	h.logError(errors.New(corruptionInfo.Description), context, ErrorActionRecover)

	// Isolate corrupted data
	if err := h.IsolateCorruptedData(corruptionInfo.Location); err != nil {
		return fmt.Errorf("failed to isolate corrupted data: %w", err)
	}

	return fmt.Errorf("data corruption detected: %s", corruptionInfo.Description)
}

// IsolateCorruptedData isolates corrupted data to prevent further damage
func (h *DefaultErrorHandler) IsolateCorruptedData(location DataLocation) error {
	// Mark the file or region as corrupted
	corruptedPath := location.File + ".corrupted"

	// Move corrupted file to isolation
	if err := os.Rename(location.File, corruptedPath); err != nil {
		return fmt.Errorf("failed to isolate corrupted file %s: %w", location.File, err)
	}

	return nil
}

// ClassifyError classifies an error based on its type and characteristics
func (h *DefaultErrorHandler) ClassifyError(err error) ErrorContext {
	context := ErrorContext{
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	// Check for specific error types
	if isIOError(err) {
		context.Category = ErrorCategoryIO
		context.Severity = ErrorSeverityMedium
		context.Recoverable = true
	} else if isMemoryError(err) {
		context.Category = ErrorCategoryMemory
		context.Severity = ErrorSeverityHigh
		context.Recoverable = true
	} else if isDiskError(err) {
		context.Category = ErrorCategoryDisk
		context.Severity = ErrorSeverityHigh
		context.Recoverable = false
	} else if isCorruptionError(err) {
		context.Category = ErrorCategoryCorruption
		context.Severity = ErrorSeverityCritical
		context.Recoverable = false
	} else {
		context.Category = ErrorCategorySystem
		context.Severity = ErrorSeverityMedium
		context.Recoverable = true
	}

	return context
}

// GetRecoveryStrategy returns the appropriate recovery strategy for an error context
func (h *DefaultErrorHandler) GetRecoveryStrategy(context ErrorContext) RecoveryStrategy {
	strategy := RecoveryStrategy{
		MaxRetries: h.config.MaxRetries,
		RetryDelay: h.config.BaseRetryDelay,
		Timeout:    h.config.RecoveryTimeout,
	}

	switch context.Category {
	case ErrorCategoryIO:
		strategy.Action = ErrorActionRetry
		strategy.Recoverable = true
	case ErrorCategoryMemory:
		if context.Severity >= ErrorSeverityHigh {
			strategy.Action = ErrorActionDegrade
		} else {
			strategy.Action = ErrorActionRetry
		}
		strategy.Recoverable = true
	case ErrorCategoryDisk:
		if context.Severity >= ErrorSeverityCritical {
			strategy.Action = ErrorActionFail
			strategy.Recoverable = false
		} else {
			strategy.Action = ErrorActionDegrade
			strategy.Recoverable = true
		}
	case ErrorCategoryCorruption:
		strategy.Action = ErrorActionRecover
		strategy.Recoverable = false
	case ErrorCategoryTransaction:
		strategy.Action = ErrorActionRetry
		strategy.Recoverable = true
	case ErrorCategoryWAL:
		strategy.Action = ErrorActionRecover
		strategy.Recoverable = true
	default:
		strategy.Action = ErrorActionRetry
		strategy.Recoverable = true
	}

	return strategy
}

// Helper functions for error classification
func isIOError(err error) bool {
	if err == nil {
		return false
	}

	// Check for syscall errors
	if errno, ok := err.(syscall.Errno); ok {
		switch errno {
		case syscall.EIO, syscall.ENODEV, syscall.ENXIO:
			return true
		}
	}

	// Check error message for I/O related keywords
	errMsg := strings.ToLower(err.Error())
	ioKeywords := []string{"i/o", "input/output", "read", "write", "connection"}
	for _, keyword := range ioKeywords {
		if strings.Contains(errMsg, keyword) {
			return true
		}
	}

	return false
}

func isMemoryError(err error) bool {
	if err == nil {
		return false
	}

	// Check for syscall errors
	if errno, ok := err.(syscall.Errno); ok {
		if errno == syscall.ENOMEM {
			return true
		}
	}

	// Check error message for memory related keywords
	errMsg := strings.ToLower(err.Error())
	memoryKeywords := []string{"memory", "out of memory", "oom", "allocation"}
	for _, keyword := range memoryKeywords {
		if strings.Contains(errMsg, keyword) {
			return true
		}
	}

	return false
}

func isDiskError(err error) bool {
	if err == nil {
		return false
	}

	// Check for syscall errors
	if errno, ok := err.(syscall.Errno); ok {
		switch errno {
		case syscall.ENOSPC, syscall.EDQUOT:
			return true
		}
	}

	// Check error message for disk related keywords
	errMsg := strings.ToLower(err.Error())
	diskKeywords := []string{"disk", "space", "full", "quota"}
	for _, keyword := range diskKeywords {
		if strings.Contains(errMsg, keyword) {
			return true
		}
	}

	return false
}

func isCorruptionError(err error) bool {
	if err == nil {
		return false
	}

	// Check error message for corruption related keywords
	errMsg := strings.ToLower(err.Error())
	corruptionKeywords := []string{"corrupt", "checksum", "invalid", "malformed"}
	for _, keyword := range corruptionKeywords {
		if strings.Contains(errMsg, keyword) {
			return true
		}
	}

	return false
}

// Recovery helper functions
func (h *DefaultErrorHandler) retryWithBackoff(err error, context ErrorContext, strategy RecoveryStrategy) error {
	retryCount := 0
	if count, ok := context.Metadata["retry_count"].(int); ok {
		retryCount = count
	}

	if retryCount >= strategy.MaxRetries {
		return fmt.Errorf("max retries exceeded: %w", err)
	}

	// Calculate exponential backoff delay
	delay := strategy.RetryDelay * time.Duration(1<<uint(retryCount))
	if delay > h.config.MaxRetryDelay {
		delay = h.config.MaxRetryDelay
	}

	time.Sleep(delay)
	return nil
}

func (h *DefaultErrorHandler) attemptRecovery(err error, context ErrorContext) error {
	// Implement specific recovery logic based on error category
	switch context.Category {
	case ErrorCategoryWAL:
		return h.recoverWAL(context)
	case ErrorCategoryCorruption:
		return h.recoverFromCorruption(context)
	default:
		return fmt.Errorf("no recovery mechanism for category %s: %w", context.Category, err)
	}
}

func (h *DefaultErrorHandler) gracefulDegrade(err error, context ErrorContext) error {
	// Implement graceful degradation based on error category
	switch context.Category {
	case ErrorCategoryMemory:
		return h.reduceMemoryUsage()
	case ErrorCategoryDisk:
		return h.enableReadOnlyMode()
	default:
		return fmt.Errorf("no degradation strategy for category %s: %w", context.Category, err)
	}
}

func (h *DefaultErrorHandler) attemptDiskCleanup() error {
	// Implement disk cleanup logic
	// This could include:
	// - Cleaning up temporary files
	// - Compacting WAL files
	// - Removing old checkpoints
	return nil
}

func (h *DefaultErrorHandler) attemptMemoryCleanup() error {
	// Force garbage collection
	runtime.GC()

	// Additional memory cleanup could include:
	// - Clearing caches
	// - Reducing buffer sizes
	// - Closing unused connections
	return nil
}

func (h *DefaultErrorHandler) recoverWAL(context ErrorContext) error {
	// Implement WAL recovery logic
	return fmt.Errorf("WAL recovery not implemented")
}

func (h *DefaultErrorHandler) recoverFromCorruption(context ErrorContext) error {
	// Implement corruption recovery logic
	return fmt.Errorf("corruption recovery not implemented")
}

func (h *DefaultErrorHandler) reduceMemoryUsage() error {
	// Implement memory usage reduction
	runtime.GC()
	return nil
}

func (h *DefaultErrorHandler) enableReadOnlyMode() error {
	// Implement read-only mode
	return fmt.Errorf("read-only mode not implemented")
}

func (h *DefaultErrorHandler) logError(err error, context ErrorContext, action ErrorAction) {
	// In a real implementation, this would use a proper logging framework
	fmt.Printf("[ERROR] %s | %s:%s | %s | Action: %s | %v\n",
		context.Timestamp.Format(time.RFC3339),
		context.Category,
		context.Severity,
		context.Operation,
		action,
		err)
}
