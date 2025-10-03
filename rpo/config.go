package rpo

import (
	"fmt"
	"time"
)

// RPOLevel defines the Recovery Point Objective level
type RPOLevel int

const (
	// RPOZero - Zero data loss (RPO = 0)
	RPOZero RPOLevel = iota

	// RPOMinimal - Minimal data loss (RPO < 1 second)
	RPOMinimal

	// RPOLow - Low data loss (RPO < 5 seconds)
	RPOLow

	// RPOMedium - Medium data loss (RPO < 30 seconds)
	RPOMedium

	// RPOHigh - High data loss tolerance (RPO < 5 minutes)
	RPOHigh
)

// RPOConfig holds Recovery Point Objective configuration
type RPOConfig struct {
	// Level defines the RPO level
	Level RPOLevel `json:"level"`

	// MaxDataLoss defines the maximum acceptable data loss duration
	MaxDataLoss time.Duration `json:"max_data_loss"`

	// CheckpointFrequency defines how often checkpoints should be created
	CheckpointFrequency time.Duration `json:"checkpoint_frequency"`

	// WALSyncFrequency defines how often WAL should be synced
	WALSyncFrequency time.Duration `json:"wal_sync_frequency"`

	// MonitoringInterval defines how often RPO compliance is checked
	MonitoringInterval time.Duration `json:"monitoring_interval"`

	// AlertThreshold defines when to alert about RPO violations
	AlertThreshold time.Duration `json:"alert_threshold"`

	// CriticalThreshold defines when to take emergency action
	CriticalThreshold time.Duration `json:"critical_threshold"`

	// EnableStrictMode enforces strict RPO compliance
	EnableStrictMode bool `json:"enable_strict_mode"`

	// EnableEmergencyMode allows emergency actions when RPO is violated
	EnableEmergencyMode bool `json:"enable_emergency_mode"`

	// MaxRetries for RPO enforcement actions
	MaxRetries int `json:"max_retries"`

	// RetryDelay between RPO enforcement attempts
	RetryDelay time.Duration `json:"retry_delay"`

	// EnableMetrics enables RPO metrics collection
	EnableMetrics bool `json:"enable_metrics"`

	// MetricsInterval defines metrics collection frequency
	MetricsInterval time.Duration `json:"metrics_interval"`
}

// DefaultRPOConfig returns a default RPO configuration
func DefaultRPOConfig() *RPOConfig {
	return &RPOConfig{
		Level:               RPOLow,
		MaxDataLoss:         5 * time.Second,
		CheckpointFrequency: 30 * time.Second,
		WALSyncFrequency:    1 * time.Second,
		MonitoringInterval:  5 * time.Second,
		AlertThreshold:      3 * time.Second,
		CriticalThreshold:   4 * time.Second,
		EnableStrictMode:    false,
		EnableEmergencyMode: true,
		MaxRetries:          3,
		RetryDelay:          100 * time.Millisecond,
		EnableMetrics:       true,
		MetricsInterval:     10 * time.Second,
	}
}

// ZeroRPOConfig returns a configuration for zero data loss
func ZeroRPOConfig() *RPOConfig {
	return &RPOConfig{
		Level:               RPOZero,
		MaxDataLoss:         0,
		CheckpointFrequency: 1 * time.Second,
		WALSyncFrequency:    0, // Immediate sync
		MonitoringInterval:  1 * time.Second,
		AlertThreshold:      0,
		CriticalThreshold:   0,
		EnableStrictMode:    true,
		EnableEmergencyMode: true,
		MaxRetries:          5,
		RetryDelay:          50 * time.Millisecond,
		EnableMetrics:       true,
		MetricsInterval:     5 * time.Second,
	}
}

// ProductionRPOConfig returns a configuration suitable for production
func ProductionRPOConfig() *RPOConfig {
	return &RPOConfig{
		Level:               RPOLow,
		MaxDataLoss:         2 * time.Second,
		CheckpointFrequency: 10 * time.Second,
		WALSyncFrequency:    500 * time.Millisecond,
		MonitoringInterval:  2 * time.Second,
		AlertThreshold:      1500 * time.Millisecond,
		CriticalThreshold:   1800 * time.Millisecond,
		EnableStrictMode:    true,
		EnableEmergencyMode: true,
		MaxRetries:          3,
		RetryDelay:          100 * time.Millisecond,
		EnableMetrics:       true,
		MetricsInterval:     5 * time.Second,
	}
}

// RelaxedRPOConfig returns a configuration with relaxed RPO requirements
func RelaxedRPOConfig() *RPOConfig {
	return &RPOConfig{
		Level:               RPOHigh,
		MaxDataLoss:         5 * time.Minute,
		CheckpointFrequency: 2 * time.Minute,
		WALSyncFrequency:    10 * time.Second,
		MonitoringInterval:  30 * time.Second,
		AlertThreshold:      4 * time.Minute,
		CriticalThreshold:   4*time.Minute + 30*time.Second,
		EnableStrictMode:    false,
		EnableEmergencyMode: false,
		MaxRetries:          2,
		RetryDelay:          1 * time.Second,
		EnableMetrics:       true,
		MetricsInterval:     30 * time.Second,
	}
}

// TestRPOConfig returns a configuration suitable for testing
func TestRPOConfig() *RPOConfig {
	return &RPOConfig{
		Level:               RPOMinimal,
		MaxDataLoss:         100 * time.Millisecond,
		CheckpointFrequency: 500 * time.Millisecond,
		WALSyncFrequency:    50 * time.Millisecond,
		MonitoringInterval:  100 * time.Millisecond,
		AlertThreshold:      80 * time.Millisecond,
		CriticalThreshold:   90 * time.Millisecond,
		EnableStrictMode:    true,
		EnableEmergencyMode: true,
		MaxRetries:          2,
		RetryDelay:          10 * time.Millisecond,
		EnableMetrics:       true,
		MetricsInterval:     1 * time.Second,
	}
}

// Validate validates the RPO configuration
func (c *RPOConfig) Validate() error {
	if c.Level < RPOZero || c.Level > RPOHigh {
		return fmt.Errorf("invalid RPO level: %d", c.Level)
	}

	if c.MaxDataLoss < 0 {
		return fmt.Errorf("max data loss cannot be negative: %v", c.MaxDataLoss)
	}

	if c.CheckpointFrequency <= 0 {
		return fmt.Errorf("checkpoint frequency must be positive: %v", c.CheckpointFrequency)
	}

	if c.WALSyncFrequency < 0 {
		return fmt.Errorf("WAL sync frequency cannot be negative: %v", c.WALSyncFrequency)
	}

	if c.MonitoringInterval <= 0 {
		return fmt.Errorf("monitoring interval must be positive: %v", c.MonitoringInterval)
	}

	if c.AlertThreshold < 0 {
		return fmt.Errorf("alert threshold cannot be negative: %v", c.AlertThreshold)
	}

	if c.CriticalThreshold < 0 {
		return fmt.Errorf("critical threshold cannot be negative: %v", c.CriticalThreshold)
	}

	if c.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be negative: %d", c.MaxRetries)
	}

	if c.RetryDelay < 0 {
		return fmt.Errorf("retry delay cannot be negative: %v", c.RetryDelay)
	}

	if c.MetricsInterval <= 0 {
		return fmt.Errorf("metrics interval must be positive: %v", c.MetricsInterval)
	}

	// Validate consistency between thresholds
	if c.AlertThreshold > c.MaxDataLoss {
		return fmt.Errorf("alert threshold (%v) cannot be greater than max data loss (%v)",
			c.AlertThreshold, c.MaxDataLoss)
	}

	if c.CriticalThreshold > c.MaxDataLoss {
		return fmt.Errorf("critical threshold (%v) cannot be greater than max data loss (%v)",
			c.CriticalThreshold, c.MaxDataLoss)
	}

	if c.CriticalThreshold < c.AlertThreshold {
		return fmt.Errorf("critical threshold (%v) cannot be less than alert threshold (%v)",
			c.CriticalThreshold, c.AlertThreshold)
	}

	// Validate level-specific constraints
	switch c.Level {
	case RPOZero:
		if c.MaxDataLoss != 0 {
			return fmt.Errorf("RPO zero level requires max data loss to be 0")
		}
		if c.WALSyncFrequency != 0 {
			return fmt.Errorf("RPO zero level requires immediate WAL sync (frequency = 0)")
		}
	case RPOMinimal:
		if c.MaxDataLoss > 1*time.Second {
			return fmt.Errorf("RPO minimal level requires max data loss <= 1 second")
		}
	case RPOLow:
		if c.MaxDataLoss > 5*time.Second {
			return fmt.Errorf("RPO low level requires max data loss <= 5 seconds")
		}
	case RPOMedium:
		if c.MaxDataLoss > 30*time.Second {
			return fmt.Errorf("RPO medium level requires max data loss <= 30 seconds")
		}
	case RPOHigh:
		if c.MaxDataLoss > 5*time.Minute {
			return fmt.Errorf("RPO high level requires max data loss <= 5 minutes")
		}
	}

	return nil
}

// Clone creates a deep copy of the configuration
func (c *RPOConfig) Clone() *RPOConfig {
	clone := *c
	return &clone
}

// IsZeroRPO returns true if this is a zero RPO configuration
func (c *RPOConfig) IsZeroRPO() bool {
	return c.Level == RPOZero || c.MaxDataLoss == 0
}

// IsStrictMode returns true if strict mode is enabled
func (c *RPOConfig) IsStrictMode() bool {
	return c.EnableStrictMode
}

// IsEmergencyModeEnabled returns true if emergency mode is enabled
func (c *RPOConfig) IsEmergencyModeEnabled() bool {
	return c.EnableEmergencyMode
}

// IsMetricsEnabled returns true if metrics collection is enabled
func (c *RPOConfig) IsMetricsEnabled() bool {
	return c.EnableMetrics
}

// ShouldSyncImmediately returns true if WAL should be synced immediately
func (c *RPOConfig) ShouldSyncImmediately() bool {
	return c.WALSyncFrequency == 0
}

// GetEffectiveCheckpointFrequency returns the effective checkpoint frequency
func (c *RPOConfig) GetEffectiveCheckpointFrequency() time.Duration {
	// For zero RPO, checkpoint frequency should not exceed max data loss
	if c.IsZeroRPO() {
		return c.CheckpointFrequency
	}

	// For other levels, ensure checkpoint frequency is reasonable relative to max data loss
	if c.CheckpointFrequency > c.MaxDataLoss/2 {
		return c.MaxDataLoss / 2
	}

	return c.CheckpointFrequency
}

// GetEffectiveWALSyncFrequency returns the effective WAL sync frequency
func (c *RPOConfig) GetEffectiveWALSyncFrequency() time.Duration {
	if c.ShouldSyncImmediately() {
		return 0
	}

	// Ensure WAL sync frequency is reasonable relative to max data loss
	if c.WALSyncFrequency > c.MaxDataLoss/4 {
		return c.MaxDataLoss / 4
	}

	return c.WALSyncFrequency
}

// String returns a string representation of the RPO level
func (l RPOLevel) String() string {
	switch l {
	case RPOZero:
		return "zero"
	case RPOMinimal:
		return "minimal"
	case RPOLow:
		return "low"
	case RPOMedium:
		return "medium"
	case RPOHigh:
		return "high"
	default:
		return fmt.Sprintf("unknown(%d)", l)
	}
}

// String returns a string representation of the configuration
func (c *RPOConfig) String() string {
	return fmt.Sprintf("RPOConfig{Level:%s, MaxDataLoss:%v, CheckpointFreq:%v, WALSyncFreq:%v, Strict:%v}",
		c.Level.String(), c.MaxDataLoss, c.CheckpointFrequency, c.WALSyncFrequency, c.EnableStrictMode)
}

// GetLevelFromDuration returns the appropriate RPO level for a given max data loss duration
func GetLevelFromDuration(maxDataLoss time.Duration) RPOLevel {
	if maxDataLoss == 0 {
		return RPOZero
	} else if maxDataLoss <= 1*time.Second {
		return RPOMinimal
	} else if maxDataLoss <= 5*time.Second {
		return RPOLow
	} else if maxDataLoss <= 30*time.Second {
		return RPOMedium
	} else {
		return RPOHigh
	}
}

// GetRecommendedConfig returns a recommended configuration for the given RPO level
func GetRecommendedConfig(level RPOLevel) *RPOConfig {
	switch level {
	case RPOZero:
		return ZeroRPOConfig()
	case RPOMinimal:
		config := DefaultRPOConfig()
		config.Level = RPOMinimal
		config.MaxDataLoss = 500 * time.Millisecond
		config.CheckpointFrequency = 2 * time.Second
		config.WALSyncFrequency = 100 * time.Millisecond
		return config
	case RPOLow:
		return ProductionRPOConfig()
	case RPOMedium:
		config := DefaultRPOConfig()
		config.Level = RPOMedium
		config.MaxDataLoss = 15 * time.Second
		config.CheckpointFrequency = 1 * time.Minute
		config.WALSyncFrequency = 2 * time.Second
		return config
	case RPOHigh:
		return RelaxedRPOConfig()
	default:
		return DefaultRPOConfig()
	}
}
