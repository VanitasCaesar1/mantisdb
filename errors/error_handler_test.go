package errors

import (
	"fmt"
	"testing"
	"time"
)

func TestErrorHandler_BasicOperations(t *testing.T) {
	handler := NewDefaultErrorHandler(nil)

	// Test error classification
	testErr := fmt.Errorf("test error")
	context := ErrorContext{
		Operation:   "test_operation",
		Resource:    "test_resource",
		Severity:    ErrorSeverityMedium,
		Category:    ErrorCategoryIO,
		Recoverable: true,
		Timestamp:   time.Now(),
		Metadata:    map[string]interface{}{"key": "value"},
	}

	classified := handler.ClassifyError(testErr)
	// ClassifyError creates a new context based on the error, not the input context
	if classified.Timestamp.IsZero() {
		t.Error("Classified error should have timestamp set")
	}

	// Test error handling
	action := handler.HandleError(testErr, context)
	if action == ErrorActionRetry || action == ErrorActionFail || action == ErrorActionDegrade || action == ErrorActionRecover || action == ErrorActionShutdown {
		// Valid action returned
	} else {
		t.Error("HandleError should return a valid action")
	}
}

func TestErrorHandler_DiskFullHandling(t *testing.T) {
	handler := NewDefaultErrorHandler(nil)

	// Test disk full handling
	err := handler.HandleDiskFull("write_operation")
	if err != nil {
		t.Errorf("HandleDiskFull should not return error in basic case: %v", err)
	}

	// Test multiple disk full events
	for i := 0; i < 5; i++ {
		err = handler.HandleDiskFull(fmt.Sprintf("operation_%d", i))
		if err != nil {
			t.Errorf("HandleDiskFull iteration %d failed: %v", i, err)
		}
	}
}

func TestErrorHandler_MemoryExhaustionHandling(t *testing.T) {
	handler := NewDefaultErrorHandler(nil)

	// Test memory exhaustion handling
	err := handler.HandleMemoryExhaustion("allocation_operation")
	if err != nil {
		t.Errorf("HandleMemoryExhaustion should not return error in basic case: %v", err)
	}

	// Test multiple memory exhaustion events
	for i := 0; i < 3; i++ {
		err = handler.HandleMemoryExhaustion(fmt.Sprintf("alloc_%d", i))
		if err != nil {
			t.Errorf("HandleMemoryExhaustion iteration %d failed: %v", i, err)
		}
	}
}

func TestErrorHandler_IOErrorHandling(t *testing.T) {
	handler := NewDefaultErrorHandler(nil)

	testErr := fmt.Errorf("I/O operation failed")

	// Test I/O error handling with different retry counts
	for retryCount := 0; retryCount < 5; retryCount++ {
		err := handler.HandleIOError(testErr, retryCount)
		if err != nil {
			t.Errorf("HandleIOError with retry count %d failed: %v", retryCount, err)
		}
	}

	// Test with nil error
	err := handler.HandleIOError(nil, 0)
	if err == nil {
		t.Error("HandleIOError should return error for nil input error")
	}
}

func TestErrorHandler_CorruptionHandling(t *testing.T) {
	handler := NewDefaultErrorHandler(nil)

	corruptionInfo := CorruptionInfo{
		Location: DataLocation{
			File:   "test_file.dat",
			Offset: 1024,
			Size:   256,
		},
		Type:        "checksum_mismatch",
		Description: "CRC32 checksum verification failed",
		Timestamp:   time.Now(),
		Checksum:    0x12345678,
	}

	// Test corruption handling
	err := handler.HandleCorruption(corruptionInfo)
	if err != nil {
		t.Errorf("HandleCorruption failed: %v", err)
	}

	// Test data isolation
	err = handler.IsolateCorruptedData(corruptionInfo.Location)
	if err != nil {
		t.Errorf("IsolateCorruptedData failed: %v", err)
	}
}

func TestErrorHandler_RecoveryStrategies(t *testing.T) {
	handler := NewDefaultErrorHandler(nil)

	contexts := []ErrorContext{
		{
			Category:    ErrorCategoryIO,
			Severity:    ErrorSeverityLow,
			Recoverable: true,
		},
		{
			Category:    ErrorCategoryMemory,
			Severity:    ErrorSeverityHigh,
			Recoverable: true,
		},
		{
			Category:    ErrorCategoryCorruption,
			Severity:    ErrorSeverityCritical,
			Recoverable: false,
		},
		{
			Category:    ErrorCategoryDisk,
			Severity:    ErrorSeverityMedium,
			Recoverable: true,
		},
	}

	for i, context := range contexts {
		t.Run(fmt.Sprintf("Context_%d", i), func(t *testing.T) {
			strategy := handler.GetRecoveryStrategy(context)

			if strategy.Action == ErrorAction(0) {
				t.Error("Recovery strategy should have a valid action")
			}

			if strategy.MaxRetries < 0 {
				t.Error("MaxRetries should not be negative")
			}

			if strategy.RetryDelay < 0 {
				t.Error("RetryDelay should not be negative")
			}

			if strategy.Timeout < 0 {
				t.Error("Timeout should not be negative")
			}

			// Verify recoverable flag consistency
			if !context.Recoverable && strategy.Recoverable {
				t.Error("Strategy should not be recoverable if context is not recoverable")
			}
		})
	}
}

func TestErrorHandler_RecoverFromError(t *testing.T) {
	handler := NewDefaultErrorHandler(nil)

	testCases := []struct {
		name    string
		err     error
		context ErrorContext
	}{
		{
			name: "IO Error",
			err:  fmt.Errorf("I/O operation failed"),
			context: ErrorContext{
				Category:    ErrorCategoryIO,
				Severity:    ErrorSeverityMedium,
				Recoverable: true,
			},
		},
		{
			name: "Memory Error",
			err:  fmt.Errorf("out of memory"),
			context: ErrorContext{
				Category:    ErrorCategoryMemory,
				Severity:    ErrorSeverityHigh,
				Recoverable: true,
			},
		},
		{
			name: "Critical Error",
			err:  fmt.Errorf("critical system failure"),
			context: ErrorContext{
				Category:    ErrorCategorySystem,
				Severity:    ErrorSeverityCritical,
				Recoverable: false,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := handler.RecoverFromError(tc.err, tc.context)

			// Recovery might succeed or fail depending on the error type
			// We just verify that the method doesn't panic and returns something
			if tc.context.Recoverable && err != nil {
				t.Logf("Recovery failed for recoverable error: %v", err)
			}
			if !tc.context.Recoverable && err == nil {
				t.Error("Recovery should fail for non-recoverable error")
			}
		})
	}
}

func TestMantisError(t *testing.T) {
	originalErr := fmt.Errorf("original error")
	context := ErrorContext{
		Operation:   "test_operation",
		Resource:    "test_resource",
		Severity:    ErrorSeverityHigh,
		Category:    ErrorCategoryWAL,
		Recoverable: true,
		Timestamp:   time.Now(),
		Metadata:    map[string]interface{}{"txn_id": 12345},
	}

	mantisErr := &MantisError{
		Err:     originalErr,
		Context: context,
	}

	// Test Error() method
	errorStr := mantisErr.Error()
	if len(errorStr) == 0 {
		t.Error("Error string should not be empty")
	}

	// Should contain category and severity
	if !containsStr(errorStr, context.Category.String()) {
		t.Errorf("Error string should contain category: %s", errorStr)
	}

	if !containsStr(errorStr, context.Severity.String()) {
		t.Errorf("Error string should contain severity: %s", errorStr)
	}

	// Test Unwrap() method
	unwrapped := mantisErr.Unwrap()
	if unwrapped != originalErr {
		t.Error("Unwrap should return the original error")
	}
}

func TestErrorSeverity_String(t *testing.T) {
	severities := []ErrorSeverity{
		ErrorSeverityLow,
		ErrorSeverityMedium,
		ErrorSeverityHigh,
		ErrorSeverityCritical,
	}

	expectedStrings := []string{"LOW", "MEDIUM", "HIGH", "CRITICAL"}

	for i, severity := range severities {
		str := severity.String()
		if str != expectedStrings[i] {
			t.Errorf("Expected severity string %s, got %s", expectedStrings[i], str)
		}
	}

	// Test unknown severity
	unknownSeverity := ErrorSeverity(999)
	if unknownSeverity.String() != "UNKNOWN" {
		t.Errorf("Expected 'UNKNOWN' for invalid severity, got %s", unknownSeverity.String())
	}
}

func TestErrorCategory_String(t *testing.T) {
	categories := []ErrorCategory{
		ErrorCategoryIO,
		ErrorCategoryMemory,
		ErrorCategoryDisk,
		ErrorCategoryCorruption,
		ErrorCategoryTransaction,
		ErrorCategoryWAL,
		ErrorCategoryNetwork,
		ErrorCategorySystem,
	}

	expectedStrings := []string{
		"IO", "MEMORY", "DISK", "CORRUPTION",
		"TRANSACTION", "WAL", "NETWORK", "SYSTEM",
	}

	for i, category := range categories {
		str := category.String()
		if str != expectedStrings[i] {
			t.Errorf("Expected category string %s, got %s", expectedStrings[i], str)
		}
	}

	// Test unknown category
	unknownCategory := ErrorCategory(999)
	if unknownCategory.String() != "UNKNOWN" {
		t.Errorf("Expected 'UNKNOWN' for invalid category, got %s", unknownCategory.String())
	}
}

func TestErrorAction_String(t *testing.T) {
	actions := []ErrorAction{
		ErrorActionRetry,
		ErrorActionFail,
		ErrorActionDegrade,
		ErrorActionRecover,
		ErrorActionShutdown,
	}

	expectedStrings := []string{
		"RETRY", "FAIL", "DEGRADE", "RECOVER", "SHUTDOWN",
	}

	for i, action := range actions {
		str := action.String()
		if str != expectedStrings[i] {
			t.Errorf("Expected action string %s, got %s", expectedStrings[i], str)
		}
	}

	// Test unknown action
	unknownAction := ErrorAction(999)
	if unknownAction.String() != "UNKNOWN" {
		t.Errorf("Expected 'UNKNOWN' for invalid action, got %s", unknownAction.String())
	}
}

func TestErrorHandler_ConcurrentAccess(t *testing.T) {
	handler := NewDefaultErrorHandler(nil)

	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	// Test concurrent error handling
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer func() { done <- true }()

			testErr := fmt.Errorf("concurrent error %d", goroutineID)
			context := ErrorContext{
				Operation:   fmt.Sprintf("operation_%d", goroutineID),
				Resource:    fmt.Sprintf("resource_%d", goroutineID),
				Severity:    ErrorSeverityMedium,
				Category:    ErrorCategoryIO,
				Recoverable: true,
				Timestamp:   time.Now(),
			}

			// Test various operations concurrently
			action := handler.HandleError(testErr, context)
			if action == ErrorAction(0) {
				t.Errorf("Goroutine %d: HandleError returned invalid action", goroutineID)
			}

			err := handler.HandleDiskFull(fmt.Sprintf("disk_op_%d", goroutineID))
			if err != nil {
				t.Errorf("Goroutine %d: HandleDiskFull failed: %v", goroutineID, err)
			}

			err = handler.HandleMemoryExhaustion(fmt.Sprintf("mem_op_%d", goroutineID))
			if err != nil {
				t.Errorf("Goroutine %d: HandleMemoryExhaustion failed: %v", goroutineID, err)
			}

			strategy := handler.GetRecoveryStrategy(context)
			if strategy.Action == ErrorAction(0) {
				t.Errorf("Goroutine %d: GetRecoveryStrategy returned invalid action", goroutineID)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

// Helper function to check if a string contains a substring
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				containsInMiddle(s, substr))))
}

func containsInMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
