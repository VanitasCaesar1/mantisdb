package errors

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"syscall"
	"time"
)

// IOErrorHandler handles I/O errors with retry logic and circuit breaker pattern
type IOErrorHandler struct {
	mu              sync.RWMutex
	config          *IOErrorConfig
	circuitBreakers map[string]*CircuitBreaker
	retryStats      map[string]*RetryStats
}

// IOErrorConfig contains configuration for I/O error handling
type IOErrorConfig struct {
	MaxRetries           int                   `json:"max_retries"`
	BaseRetryDelay       time.Duration         `json:"base_retry_delay"`
	MaxRetryDelay        time.Duration         `json:"max_retry_delay"`
	RetryMultiplier      float64               `json:"retry_multiplier"`
	JitterEnabled        bool                  `json:"jitter_enabled"`
	CircuitBreakerConfig *CircuitBreakerConfig `json:"circuit_breaker_config"`
	RetryableErrors      []IOErrorType         `json:"retryable_errors"`
}

// CircuitBreakerConfig contains configuration for circuit breaker
type CircuitBreakerConfig struct {
	FailureThreshold int           `json:"failure_threshold"`   // Number of failures to open circuit
	RecoveryTimeout  time.Duration `json:"recovery_timeout"`    // Time to wait before trying half-open
	SuccessThreshold int           `json:"success_threshold"`   // Successes needed to close circuit
	HalfOpenMaxCalls int           `json:"half_open_max_calls"` // Max calls allowed in half-open state
}

// IOErrorType represents different types of I/O errors
type IOErrorType int

const (
	IOErrorTypeUnknown IOErrorType = iota
	IOErrorTypeTimeout
	IOErrorTypeConnectionRefused
	IOErrorTypeConnectionReset
	IOErrorTypeNetworkUnreachable
	IOErrorTypePermissionDenied
	IOErrorTypeFileNotFound
	IOErrorTypeDiskFull
	IOErrorTypeDeviceBusy
	IOErrorTypeCorruption
	IOErrorTypeHardwareFailure
)

func (t IOErrorType) String() string {
	switch t {
	case IOErrorTypeTimeout:
		return "TIMEOUT"
	case IOErrorTypeConnectionRefused:
		return "CONNECTION_REFUSED"
	case IOErrorTypeConnectionReset:
		return "CONNECTION_RESET"
	case IOErrorTypeNetworkUnreachable:
		return "NETWORK_UNREACHABLE"
	case IOErrorTypePermissionDenied:
		return "PERMISSION_DENIED"
	case IOErrorTypeFileNotFound:
		return "FILE_NOT_FOUND"
	case IOErrorTypeDiskFull:
		return "DISK_FULL"
	case IOErrorTypeDeviceBusy:
		return "DEVICE_BUSY"
	case IOErrorTypeCorruption:
		return "CORRUPTION"
	case IOErrorTypeHardwareFailure:
		return "HARDWARE_FAILURE"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	CircuitBreakerClosed CircuitBreakerState = iota
	CircuitBreakerOpen
	CircuitBreakerHalfOpen
)

func (s CircuitBreakerState) String() string {
	switch s {
	case CircuitBreakerClosed:
		return "CLOSED"
	case CircuitBreakerOpen:
		return "OPEN"
	case CircuitBreakerHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreaker implements the circuit breaker pattern for I/O operations
type CircuitBreaker struct {
	mu              sync.RWMutex
	config          *CircuitBreakerConfig
	state           CircuitBreakerState
	failureCount    int
	successCount    int
	lastFailureTime time.Time
	halfOpenCalls   int
}

// RetryStats contains statistics about retry operations
type RetryStats struct {
	TotalAttempts     int64         `json:"total_attempts"`
	SuccessfulRetries int64         `json:"successful_retries"`
	FailedRetries     int64         `json:"failed_retries"`
	TotalRetryTime    time.Duration `json:"total_retry_time"`
	LastRetryTime     time.Time     `json:"last_retry_time"`
}

// IOOperation represents an I/O operation that can be retried
type IOOperation func() error

// IOOperationWithResult represents an I/O operation that returns a result
type IOOperationWithResult func() (interface{}, error)

// RetryResult contains the result of a retry operation
type RetryResult struct {
	Success       bool          `json:"success"`
	Attempts      int           `json:"attempts"`
	TotalDuration time.Duration `json:"total_duration"`
	LastError     error         `json:"last_error,omitempty"`
	ErrorType     IOErrorType   `json:"error_type"`
}

// NewIOErrorHandler creates a new I/O error handler
func NewIOErrorHandler(config *IOErrorConfig) *IOErrorHandler {
	if config == nil {
		config = &IOErrorConfig{
			MaxRetries:      5,
			BaseRetryDelay:  100 * time.Millisecond,
			MaxRetryDelay:   30 * time.Second,
			RetryMultiplier: 2.0,
			JitterEnabled:   true,
			CircuitBreakerConfig: &CircuitBreakerConfig{
				FailureThreshold: 5,
				RecoveryTimeout:  60 * time.Second,
				SuccessThreshold: 3,
				HalfOpenMaxCalls: 5,
			},
			RetryableErrors: []IOErrorType{
				IOErrorTypeTimeout,
				IOErrorTypeConnectionRefused,
				IOErrorTypeConnectionReset,
				IOErrorTypeNetworkUnreachable,
				IOErrorTypeDeviceBusy,
			},
		}
	}

	return &IOErrorHandler{
		config:          config,
		circuitBreakers: make(map[string]*CircuitBreaker),
		retryStats:      make(map[string]*RetryStats),
	}
}

// ExecuteWithRetry executes an I/O operation with retry logic
func (h *IOErrorHandler) ExecuteWithRetry(ctx context.Context, operation IOOperation, resource string) *RetryResult {
	result := &RetryResult{
		Success: false,
	}

	startTime := time.Now()

	for attempt := 0; attempt <= h.config.MaxRetries; attempt++ {
		result.Attempts = attempt + 1

		// Check circuit breaker
		if !h.canExecute(resource) {
			result.LastError = fmt.Errorf("circuit breaker is open for resource: %s", resource)
			result.ErrorType = IOErrorTypeUnknown
			break
		}

		// Execute operation
		err := operation()

		if err == nil {
			// Success
			result.Success = true
			h.recordSuccess(resource)
			break
		}

		// Classify error
		errorType := h.classifyIOError(err)
		result.ErrorType = errorType
		result.LastError = err

		// Check if error is retryable
		if !h.isRetryableError(errorType) {
			h.recordFailure(resource)
			break
		}

		// Record failure
		h.recordFailure(resource)

		// Check if we should retry
		if attempt >= h.config.MaxRetries {
			break
		}

		// Check context cancellation
		if ctx.Err() != nil {
			result.LastError = ctx.Err()
			break
		}

		// Calculate retry delay
		delay := h.calculateRetryDelay(attempt)

		// Wait before retry
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			result.LastError = ctx.Err()
			return result
		}
	}

	result.TotalDuration = time.Since(startTime)
	h.updateRetryStats(resource, result)

	return result
}

// ExecuteWithRetryAndResult executes an I/O operation with retry logic and returns a result
func (h *IOErrorHandler) ExecuteWithRetryAndResult(ctx context.Context, operation func() (interface{}, error), resource string) (interface{}, *RetryResult) {
	var zeroValue interface{}
	result := &RetryResult{
		Success: false,
	}

	startTime := time.Now()

	for attempt := 0; attempt <= h.config.MaxRetries; attempt++ {
		result.Attempts = attempt + 1

		// Check circuit breaker
		if !h.canExecute(resource) {
			result.LastError = fmt.Errorf("circuit breaker is open for resource: %s", resource)
			result.ErrorType = IOErrorTypeUnknown
			break
		}

		// Execute operation
		value, err := operation()

		if err == nil {
			// Success
			result.Success = true
			h.recordSuccess(resource)
			result.TotalDuration = time.Since(startTime)
			h.updateRetryStats(resource, result)
			return value, result
		}

		// Classify error
		errorType := h.classifyIOError(err)
		result.ErrorType = errorType
		result.LastError = err

		// Check if error is retryable
		if !h.isRetryableError(errorType) {
			h.recordFailure(resource)
			break
		}

		// Record failure
		h.recordFailure(resource)

		// Check if we should retry
		if attempt >= h.config.MaxRetries {
			break
		}

		// Check context cancellation
		if ctx.Err() != nil {
			result.LastError = ctx.Err()
			break
		}

		// Calculate retry delay
		delay := h.calculateRetryDelay(attempt)

		// Wait before retry
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			result.LastError = ctx.Err()
			result.TotalDuration = time.Since(startTime)
			h.updateRetryStats(resource, result)
			return zeroValue, result
		}
	}

	result.TotalDuration = time.Since(startTime)
	h.updateRetryStats(resource, result)

	return zeroValue, result
}

// GetCircuitBreakerState returns the current state of a circuit breaker
func (h *IOErrorHandler) GetCircuitBreakerState(resource string) CircuitBreakerState {
	h.mu.RLock()
	defer h.mu.RUnlock()

	cb, exists := h.circuitBreakers[resource]
	if !exists {
		return CircuitBreakerClosed
	}

	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return cb.state
}

// GetRetryStats returns retry statistics for a resource
func (h *IOErrorHandler) GetRetryStats(resource string) *RetryStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	stats, exists := h.retryStats[resource]
	if !exists {
		return &RetryStats{}
	}

	// Return a copy
	statsCopy := *stats
	return &statsCopy
}

// ResetCircuitBreaker manually resets a circuit breaker
func (h *IOErrorHandler) ResetCircuitBreaker(resource string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	cb, exists := h.circuitBreakers[resource]
	if !exists {
		return
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = CircuitBreakerClosed
	cb.failureCount = 0
	cb.successCount = 0
	cb.halfOpenCalls = 0
}

// Private methods

func (h *IOErrorHandler) getOrCreateCircuitBreaker(resource string) *CircuitBreaker {
	h.mu.Lock()
	defer h.mu.Unlock()

	cb, exists := h.circuitBreakers[resource]
	if !exists {
		cb = &CircuitBreaker{
			config: h.config.CircuitBreakerConfig,
			state:  CircuitBreakerClosed,
		}
		h.circuitBreakers[resource] = cb
	}

	return cb
}

func (h *IOErrorHandler) canExecute(resource string) bool {
	cb := h.getOrCreateCircuitBreaker(resource)

	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitBreakerClosed:
		return true
	case CircuitBreakerOpen:
		// Check if recovery timeout has passed
		if time.Since(cb.lastFailureTime) >= cb.config.RecoveryTimeout {
			cb.state = CircuitBreakerHalfOpen
			cb.halfOpenCalls = 0
			return true
		}
		return false
	case CircuitBreakerHalfOpen:
		// Allow limited calls in half-open state
		if cb.halfOpenCalls < cb.config.HalfOpenMaxCalls {
			cb.halfOpenCalls++
			return true
		}
		return false
	default:
		return false
	}
}

func (h *IOErrorHandler) recordSuccess(resource string) {
	cb := h.getOrCreateCircuitBreaker(resource)

	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitBreakerClosed:
		cb.failureCount = 0
	case CircuitBreakerHalfOpen:
		cb.successCount++
		if cb.successCount >= cb.config.SuccessThreshold {
			cb.state = CircuitBreakerClosed
			cb.failureCount = 0
			cb.successCount = 0
			cb.halfOpenCalls = 0
		}
	}
}

func (h *IOErrorHandler) recordFailure(resource string) {
	cb := h.getOrCreateCircuitBreaker(resource)

	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case CircuitBreakerClosed:
		if cb.failureCount >= cb.config.FailureThreshold {
			cb.state = CircuitBreakerOpen
		}
	case CircuitBreakerHalfOpen:
		cb.state = CircuitBreakerOpen
		cb.successCount = 0
		cb.halfOpenCalls = 0
	}
}

func (h *IOErrorHandler) classifyIOError(err error) IOErrorType {
	if err == nil {
		return IOErrorTypeUnknown
	}

	// Check for syscall errors
	if errno, ok := err.(syscall.Errno); ok {
		switch errno {
		case syscall.ETIMEDOUT:
			return IOErrorTypeTimeout
		case syscall.ECONNREFUSED:
			return IOErrorTypeConnectionRefused
		case syscall.ECONNRESET:
			return IOErrorTypeConnectionReset
		case syscall.ENETUNREACH:
			return IOErrorTypeNetworkUnreachable
		case syscall.EACCES, syscall.EPERM:
			return IOErrorTypePermissionDenied
		case syscall.ENOENT:
			return IOErrorTypeFileNotFound
		case syscall.ENOSPC:
			return IOErrorTypeDiskFull
		case syscall.EBUSY:
			return IOErrorTypeDeviceBusy
		case syscall.EIO:
			return IOErrorTypeHardwareFailure
		}
	}

	// Check error message for common patterns
	errMsg := err.Error()
	switch {
	case contains(errMsg, "timeout", "timed out"):
		return IOErrorTypeTimeout
	case contains(errMsg, "connection refused"):
		return IOErrorTypeConnectionRefused
	case contains(errMsg, "connection reset"):
		return IOErrorTypeConnectionReset
	case contains(errMsg, "network unreachable"):
		return IOErrorTypeNetworkUnreachable
	case contains(errMsg, "permission denied"):
		return IOErrorTypePermissionDenied
	case contains(errMsg, "no such file"):
		return IOErrorTypeFileNotFound
	case contains(errMsg, "no space left", "disk full"):
		return IOErrorTypeDiskFull
	case contains(errMsg, "device busy"):
		return IOErrorTypeDeviceBusy
	case contains(errMsg, "corrupt", "checksum", "invalid"):
		return IOErrorTypeCorruption
	case contains(errMsg, "i/o error", "hardware"):
		return IOErrorTypeHardwareFailure
	default:
		return IOErrorTypeUnknown
	}
}

func (h *IOErrorHandler) isRetryableError(errorType IOErrorType) bool {
	for _, retryableType := range h.config.RetryableErrors {
		if errorType == retryableType {
			return true
		}
	}
	return false
}

func (h *IOErrorHandler) calculateRetryDelay(attempt int) time.Duration {
	// Calculate exponential backoff
	delay := time.Duration(float64(h.config.BaseRetryDelay) * math.Pow(h.config.RetryMultiplier, float64(attempt)))

	// Cap at maximum delay
	if delay > h.config.MaxRetryDelay {
		delay = h.config.MaxRetryDelay
	}

	// Add jitter if enabled
	if h.config.JitterEnabled {
		jitter := time.Duration(rand.Float64() * float64(delay) * 0.1) // 10% jitter
		delay += jitter
	}

	return delay
}

func (h *IOErrorHandler) updateRetryStats(resource string, result *RetryResult) {
	h.mu.Lock()
	defer h.mu.Unlock()

	stats, exists := h.retryStats[resource]
	if !exists {
		stats = &RetryStats{}
		h.retryStats[resource] = stats
	}

	stats.TotalAttempts += int64(result.Attempts)
	stats.TotalRetryTime += result.TotalDuration
	stats.LastRetryTime = time.Now()

	if result.Success {
		stats.SuccessfulRetries++
	} else {
		stats.FailedRetries++
	}
}

// Helper function to check if error message contains any of the given substrings
func contains(s string, substrings ...string) bool {
	for _, substring := range substrings {
		if len(s) >= len(substring) {
			for i := 0; i <= len(s)-len(substring); i++ {
				if s[i:i+len(substring)] == substring {
					return true
				}
			}
		}
	}
	return false
}
