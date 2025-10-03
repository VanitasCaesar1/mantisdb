package wal

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestRecoveryEngineBasicFunctionality(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "recovery_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create recovery engine
	engine, err := NewRecoveryEngine(tempDir)
	if err != nil {
		t.Fatalf("Failed to create recovery engine: %v", err)
	}

	// Test crash detection on clean system
	crashed, err := engine.DetectCrash()
	if err != nil {
		t.Fatalf("Failed to detect crash: %v", err)
	}
	if crashed {
		t.Error("Expected no crash on clean system")
	}

	// Test creating crash detection file
	if err := engine.CreateCrashDetectionFile(); err != nil {
		t.Fatalf("Failed to create crash detection file: %v", err)
	}

	// Now crash should be detected
	crashed, err = engine.DetectCrash()
	if err != nil {
		t.Fatalf("Failed to detect crash: %v", err)
	}
	if !crashed {
		t.Error("Expected crash to be detected after creating detection file")
	}

	// Test removing crash detection file
	if err := engine.RemoveCrashDetectionFile(); err != nil {
		t.Fatalf("Failed to remove crash detection file: %v", err)
	}

	// Crash should no longer be detected
	crashed, err = engine.DetectCrash()
	if err != nil {
		t.Fatalf("Failed to detect crash: %v", err)
	}
	if crashed {
		t.Error("Expected no crash after removing detection file")
	}
}

func TestRecoveryStateManagement(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "recovery_state_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	engine, err := NewRecoveryEngine(tempDir)
	if err != nil {
		t.Fatalf("Failed to create recovery engine: %v", err)
	}

	// Check initial state
	state := engine.GetRecoveryState()
	if state.Status != RecoveryStatusIdle {
		t.Errorf("Expected initial status to be idle, got %v", state.Status)
	}

	// Test progress reporting
	if engine.config.ProgressReporting {
		progressChan := engine.GetProgressChannel()
		if progressChan == nil {
			t.Error("Expected progress channel to be available")
		}
	}
}

func TestSafeModeOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "safe_mode_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	engine, err := NewRecoveryEngine(tempDir)
	if err != nil {
		t.Fatalf("Failed to create recovery engine: %v", err)
	}

	// Initially not in safe mode
	if engine.IsInSafeMode() {
		t.Error("Expected system not to be in safe mode initially")
	}

	// Enter safe mode
	validationResult := &ValidationResult{
		Success: false,
		Errors: []ValidationError{
			{
				Type:    ValidationErrorCorruption,
				Message: "Test corruption error",
			},
		},
	}

	if err := engine.EnterSafeMode("Test safe mode", validationResult); err != nil {
		t.Fatalf("Failed to enter safe mode: %v", err)
	}

	// Should now be in safe mode
	if !engine.IsInSafeMode() {
		t.Error("Expected system to be in safe mode")
	}

	// Get safe mode info
	info, err := engine.GetSafeModeInfo()
	if err != nil {
		t.Fatalf("Failed to get safe mode info: %v", err)
	}
	if !info.Enabled {
		t.Error("Expected safe mode to be enabled")
	}
	if info.Reason != "Test safe mode" {
		t.Errorf("Expected reason 'Test safe mode', got '%s'", info.Reason)
	}
}

func TestValidationResult(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "validation_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	engine, err := NewRecoveryEngine(tempDir)
	if err != nil {
		t.Fatalf("Failed to create recovery engine: %v", err)
	}

	// Test validation with empty WAL
	result, err := engine.ValidateRecoveryWithDetails()
	if err != nil {
		t.Fatalf("Failed to validate recovery: %v", err)
	}

	if !result.Success {
		t.Error("Expected validation to succeed with empty WAL")
	}

	if result.ValidationTime == 0 {
		t.Error("Expected validation time to be recorded")
	}
}

func TestRecoveryConfiguration(t *testing.T) {
	config := DefaultRecoveryConfig()

	// Test default values
	if config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries to be 3, got %d", config.MaxRetries)
	}

	if config.ValidationMode != ValidationModeStrict {
		t.Errorf("Expected ValidationMode to be strict, got %v", config.ValidationMode)
	}

	if !config.SafeModeOnFailure {
		t.Error("Expected SafeModeOnFailure to be true by default")
	}

	if !config.ConsistencyChecks {
		t.Error("Expected ConsistencyChecks to be true by default")
	}

	if !config.DataIntegrityChecks {
		t.Error("Expected DataIntegrityChecks to be true by default")
	}
}

func TestTransactionStateReconstruction(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "txn_reconstruction_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	engine, err := NewRecoveryEngine(tempDir)
	if err != nil {
		t.Fatalf("Failed to create recovery engine: %v", err)
	}

	// Create a simple recovery plan with transactions
	plan := &RecoveryPlan{
		StartLSN: 1,
		EndLSN:   3,
		Operations: []*WALEntry{
			{
				LSN:   1,
				TxnID: 100,
				Operation: Operation{
					Type:  OpInsert,
					Key:   "key1",
					Value: []byte("value1"),
				},
				Timestamp: time.Now(),
			},
			{
				LSN:   2,
				TxnID: 100,
				Operation: Operation{
					Type: OpCommit,
				},
				Timestamp: time.Now(),
			},
		},
		Transactions: make(map[uint64]*TransactionState),
	}

	// Test replay with transaction reconstruction
	replayFunc := func(entry *WALEntry, context *ReplayContext) error {
		// Simple validation
		if entry.LSN == 0 {
			return fmt.Errorf("invalid LSN")
		}
		return nil
	}

	if err := engine.ReplayWithTransactionReconstruction(plan, replayFunc); err != nil {
		t.Fatalf("Failed to replay with transaction reconstruction: %v", err)
	}
}
