package rpo

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// Example test showing how to use the RPO system
func TestRPOSystemIntegration(t *testing.T) {
	// Create RPO configuration
	rpoConfig := ProductionRPOConfig()

	// Create managers
	rpoManager, err := NewManager(rpoConfig)
	if err != nil {
		t.Fatalf("Failed to create RPO manager: %v", err)
	}

	// Set up mock dependencies
	mockWALManager := &MockWALManager{}
	mockAlertManager := &MockAlertManager{}
	mockCheckpointManager := &MockCheckpointManager{}

	// Configure managers
	rpoManager.SetCheckpointManager(mockCheckpointManager)
	rpoManager.SetWALManager(mockWALManager)
	rpoManager.SetAlertManager(mockAlertManager)

	// Start RPO monitoring
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rpoManager.Start(ctx); err != nil {
		t.Fatalf("Failed to start RPO manager: %v", err)
	}
	defer rpoManager.Stop()

	// Simulate some operations
	time.Sleep(1 * time.Second)

	// Check compliance
	compliance, err := rpoManager.CheckCompliance()
	if err != nil {
		t.Fatalf("Failed to check compliance: %v", err)
	}

	fmt.Printf("RPO Compliance: %+v\n", compliance)

	// Get statistics
	stats := rpoManager.GetStats()
	fmt.Printf("RPO Stats: %+v\n", stats)
}

// Mock implementations for testing

type MockCheckpointManager struct {
	lastCheckpointTime time.Time
}

func (m *MockCheckpointManager) CreateCheckpoint(checkpointType CheckpointType) (*Checkpoint, error) {
	return &Checkpoint{
		ID:        "test-checkpoint-1",
		LSN:       1000,
		Timestamp: time.Now(),
		Type:      checkpointType,
	}, nil
}

func (m *MockCheckpointManager) GetLastCheckpointTime() (time.Time, error) {
	if m.lastCheckpointTime.IsZero() {
		m.lastCheckpointTime = time.Now().Add(-30 * time.Second)
	}
	return m.lastCheckpointTime, nil
}

func (m *MockCheckpointManager) GetCheckpointStats() (*CheckpointStats, error) {
	return &CheckpointStats{
		TotalCheckpoints:   5,
		LastCheckpointTime: m.lastCheckpointTime,
		AverageInterval:    2 * time.Minute,
	}, nil
}

type MockWALManager struct {
	lastSyncTime time.Time
	lastLSN      uint64
}

func (m *MockWALManager) Sync() error {
	m.lastSyncTime = time.Now()
	return nil
}

func (m *MockWALManager) GetLastSyncTime() (time.Time, error) {
	if m.lastSyncTime.IsZero() {
		m.lastSyncTime = time.Now().Add(-2 * time.Second)
	}
	return m.lastSyncTime, nil
}

func (m *MockWALManager) GetLastLSN() (uint64, error) {
	if m.lastLSN == 0 {
		m.lastLSN = 1000
	}
	return m.lastLSN, nil
}

func (m *MockWALManager) GetUncommittedDataAge() (time.Duration, error) {
	return 1 * time.Second, nil
}

type MockAlertManager struct {
	alerts []Alert
}

func (m *MockAlertManager) SendAlert(alert Alert) error {
	m.alerts = append(m.alerts, alert)
	fmt.Printf("ALERT: %s\n", alert.Message)
	return nil
}

func (m *MockAlertManager) SendCriticalAlert(alert Alert) error {
	m.alerts = append(m.alerts, alert)
	fmt.Printf("CRITICAL ALERT: %s\n", alert.Message)
	return nil
}

// Example_rpoConfigurations shows different RPO configurations
func Example_rpoConfigurations() {
	// Zero RPO for critical systems
	zeroConfig := ZeroRPOConfig()
	fmt.Printf("Zero RPO Config: %s\n", zeroConfig.String())

	// Production RPO for normal operations
	prodConfig := ProductionRPOConfig()
	fmt.Printf("Production RPO Config: %s\n", prodConfig.String())

	// Relaxed RPO for non-critical systems
	relaxedConfig := RelaxedRPOConfig()
	fmt.Printf("Relaxed RPO Config: %s\n", relaxedConfig.String())

	// Test RPO for development
	testConfig := TestRPOConfig()
	fmt.Printf("Test RPO Config: %s\n", testConfig.String())
}

// Test RPO configuration validation
func TestRPOConfigValidation(t *testing.T) {
	// Test valid configuration
	validConfig := DefaultRPOConfig()
	if err := validConfig.Validate(); err != nil {
		t.Errorf("Valid config should not fail validation: %v", err)
	}

	// Test invalid configuration
	invalidConfig := &RPOConfig{
		Level:       RPOZero,
		MaxDataLoss: 5 * time.Second, // Should be 0 for RPOZero
	}
	if err := invalidConfig.Validate(); err == nil {
		t.Error("Invalid config should fail validation")
	}
}

// Test alert configuration
func TestAlertConfiguration(t *testing.T) {
	config := DefaultAlertConfig()
	alertManager := NewDefaultAlertManager(config)

	// Test sending an alert
	alert := Alert{
		Type:      AlertTypeRPOViolation,
		Severity:  AlertSeverityWarning,
		Message:   "Test RPO violation",
		Timestamp: time.Now(),
		RPOValue:  5 * time.Second,
		Threshold: 2 * time.Second,
	}

	if err := alertManager.SendAlert(alert); err != nil {
		t.Errorf("Failed to send alert: %v", err)
	}

	// Check alert history
	history := alertManager.GetAlertHistory(10)
	if len(history) != 1 {
		t.Errorf("Expected 1 alert in history, got %d", len(history))
	}
}
