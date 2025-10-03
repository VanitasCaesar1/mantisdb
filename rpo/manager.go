package rpo

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Manager handles RPO enforcement and monitoring
type Manager struct {
	config *RPOConfig
	mutex  sync.RWMutex

	// Dependencies
	checkpointManager CheckpointManager
	walManager        WALManager
	alertManager      AlertManager
	metricsCollector  MetricsCollector

	// Internal state
	running        bool
	stopChan       chan struct{}
	wg             sync.WaitGroup
	lastCheckpoint time.Time
	lastWALSync    time.Time
	currentRPO     time.Duration
	violations     []RPOViolation
	stats          *RPOStats

	// Monitoring
	monitorTicker *time.Ticker
	metricsTicker *time.Ticker
}

// CheckpointManager interface for checkpoint operations
type CheckpointManager interface {
	CreateCheckpoint(checkpointType CheckpointType) (*Checkpoint, error)
	GetLastCheckpointTime() (time.Time, error)
	GetCheckpointStats() (*CheckpointStats, error)
}

// WALManager interface for WAL operations
type WALManager interface {
	Sync() error
	GetLastSyncTime() (time.Time, error)
	GetLastLSN() (uint64, error)
	GetUncommittedDataAge() (time.Duration, error)
}

// AlertManager interface for alerting
type AlertManager interface {
	SendAlert(alert Alert) error
	SendCriticalAlert(alert Alert) error
}

// MetricsCollector interface for metrics collection
type MetricsCollector interface {
	RecordRPOMetric(metric RPOMetric) error
	RecordViolation(violation RPOViolation) error
}

// CheckpointType represents the type of checkpoint
type CheckpointType int

const (
	CheckpointTypeFull CheckpointType = iota
	CheckpointTypeIncremental
)

// Checkpoint represents a checkpoint
type Checkpoint struct {
	ID        string
	LSN       uint64
	Timestamp time.Time
	Type      CheckpointType
}

// CheckpointStats represents checkpoint statistics
type CheckpointStats struct {
	TotalCheckpoints   int
	LastCheckpointTime time.Time
	AverageInterval    time.Duration
}

// Alert represents an alert
type Alert struct {
	Type      AlertType
	Severity  AlertSeverity
	Message   string
	Timestamp time.Time
	Details   map[string]interface{}
	RPOValue  time.Duration
	Threshold time.Duration
}

// AlertType defines types of alerts
type AlertType int

const (
	AlertTypeRPOViolation AlertType = iota
	AlertTypeRPOCritical
	AlertTypeCheckpointFailed
	AlertTypeWALSyncFailed
	AlertTypeRPORecovered
)

// AlertSeverity defines alert severity levels
type AlertSeverity int

const (
	AlertSeverityInfo AlertSeverity = iota
	AlertSeverityWarning
	AlertSeverityCritical
	AlertSeverityEmergency
)

// RPOMetric represents an RPO metric
type RPOMetric struct {
	Timestamp       time.Time
	CurrentRPO      time.Duration
	MaxAllowedRPO   time.Duration
	CheckpointAge   time.Duration
	WALSyncAge      time.Duration
	ViolationCount  int
	ComplianceRatio float64
}

// RPOViolation represents an RPO violation
type RPOViolation struct {
	ID            string
	Timestamp     time.Time
	ActualRPO     time.Duration
	MaxAllowedRPO time.Duration
	ViolationType ViolationType
	Severity      ViolationSeverity
	Duration      time.Duration
	Resolved      bool
	ResolvedAt    *time.Time
	Actions       []string
	Details       map[string]interface{}
}

// ViolationType defines types of RPO violations
type ViolationType int

const (
	ViolationTypeCheckpointDelay ViolationType = iota
	ViolationTypeWALSyncDelay
	ViolationTypeDataLoss
	ViolationTypeSystemFailure
)

// ViolationSeverity defines violation severity levels
type ViolationSeverity int

const (
	ViolationSeverityMinor ViolationSeverity = iota
	ViolationSeverityMajor
	ViolationSeverityCritical
	ViolationSeverityEmergency
)

// RPOStats contains RPO statistics
type RPOStats struct {
	CurrentRPO          time.Duration
	MaxObservedRPO      time.Duration
	AverageRPO          time.Duration
	ComplianceRatio     float64
	TotalViolations     int
	ActiveViolations    int
	LastViolationTime   time.Time
	LastCheckpointTime  time.Time
	LastWALSyncTime     time.Time
	CheckpointInterval  time.Duration
	WALSyncInterval     time.Duration
	UptimeSeconds       int64
	MonitoringStartTime time.Time
}

// NewManager creates a new RPO manager
func NewManager(config *RPOConfig) (*Manager, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid RPO config: %w", err)
	}

	return &Manager{
		config:     config,
		stopChan:   make(chan struct{}),
		violations: make([]RPOViolation, 0),
		stats: &RPOStats{
			MonitoringStartTime: time.Now(),
		},
	}, nil
}

// SetCheckpointManager sets the checkpoint manager
func (m *Manager) SetCheckpointManager(cm CheckpointManager) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.checkpointManager = cm
}

// SetWALManager sets the WAL manager
func (m *Manager) SetWALManager(wm WALManager) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.walManager = wm
}

// SetAlertManager sets the alert manager
func (m *Manager) SetAlertManager(am AlertManager) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.alertManager = am
}

// SetMetricsCollector sets the metrics collector
func (m *Manager) SetMetricsCollector(mc MetricsCollector) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.metricsCollector = mc
}

// Start starts the RPO manager
func (m *Manager) Start(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.running {
		return fmt.Errorf("RPO manager is already running")
	}

	if m.checkpointManager == nil {
		return fmt.Errorf("checkpoint manager not configured")
	}

	if m.walManager == nil {
		return fmt.Errorf("WAL manager not configured")
	}

	m.running = true

	// Start monitoring loop
	m.wg.Add(1)
	go m.monitoringLoop(ctx)

	// Start metrics collection if enabled
	if m.config.IsMetricsEnabled() {
		m.wg.Add(1)
		go m.metricsLoop(ctx)
	}

	return nil
}

// Stop stops the RPO manager
func (m *Manager) Stop() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.running {
		return nil
	}

	m.running = false
	close(m.stopChan)

	if m.monitorTicker != nil {
		m.monitorTicker.Stop()
	}

	if m.metricsTicker != nil {
		m.metricsTicker.Stop()
	}

	m.wg.Wait()
	return nil
}

// CheckCompliance checks current RPO compliance
func (m *Manager) CheckCompliance() (*RPOComplianceResult, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := &RPOComplianceResult{
		Timestamp:     time.Now(),
		MaxAllowedRPO: m.config.MaxDataLoss,
	}

	// Get current RPO
	currentRPO, err := m.calculateCurrentRPO()
	if err != nil {
		return nil, fmt.Errorf("failed to calculate current RPO: %w", err)
	}

	result.CurrentRPO = currentRPO
	result.IsCompliant = currentRPO <= m.config.MaxDataLoss

	// Check for violations
	if !result.IsCompliant {
		violation := m.createViolation(currentRPO)
		result.Violation = &violation
	}

	// Get checkpoint status
	if lastCheckpointTime, err := m.checkpointManager.GetLastCheckpointTime(); err == nil {
		result.LastCheckpointTime = lastCheckpointTime
		result.CheckpointAge = time.Since(lastCheckpointTime)
	}

	// Get WAL sync status
	if lastSyncTime, err := m.walManager.GetLastSyncTime(); err == nil {
		result.LastWALSyncTime = lastSyncTime
		result.WALSyncAge = time.Since(lastSyncTime)
	}

	return result, nil
}

// EnforceCompliance enforces RPO compliance
func (m *Manager) EnforceCompliance() error {
	compliance, err := m.CheckCompliance()
	if err != nil {
		return fmt.Errorf("failed to check compliance: %w", err)
	}

	if compliance.IsCompliant {
		return nil // Already compliant
	}

	// Take enforcement actions
	actions := m.determineEnforcementActions(compliance)

	for _, action := range actions {
		if err := m.executeEnforcementAction(action); err != nil {
			return fmt.Errorf("failed to execute enforcement action %s: %w", action.Type, err)
		}
	}

	return nil
}

// GetStats returns current RPO statistics
func (m *Manager) GetStats() *RPOStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Create a copy to avoid race conditions
	stats := *m.stats
	stats.UptimeSeconds = int64(time.Since(stats.MonitoringStartTime).Seconds())
	return &stats
}

// GetViolations returns current RPO violations
func (m *Manager) GetViolations(activeOnly bool) []RPOViolation {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if !activeOnly {
		// Return all violations
		violations := make([]RPOViolation, len(m.violations))
		copy(violations, m.violations)
		return violations
	}

	// Return only active violations
	var activeViolations []RPOViolation
	for _, violation := range m.violations {
		if !violation.Resolved {
			activeViolations = append(activeViolations, violation)
		}
	}

	return activeViolations
}

// UpdateConfig updates the RPO configuration
func (m *Manager) UpdateConfig(newConfig *RPOConfig) error {
	if err := newConfig.Validate(); err != nil {
		return fmt.Errorf("invalid RPO config: %w", err)
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	oldConfig := m.config
	m.config = newConfig

	// Restart monitoring with new intervals if running
	if m.running {
		if m.monitorTicker != nil {
			m.monitorTicker.Stop()
		}
		if m.metricsTicker != nil {
			m.metricsTicker.Stop()
		}

		// Create new tickers with updated intervals
		m.monitorTicker = time.NewTicker(m.config.MonitoringInterval)
		if m.config.IsMetricsEnabled() {
			m.metricsTicker = time.NewTicker(m.config.MetricsInterval)
		}
	}

	// Send alert about configuration change
	if m.alertManager != nil {
		alert := Alert{
			Type:      AlertTypeRPORecovered, // Reusing for config change
			Severity:  AlertSeverityInfo,
			Message:   "RPO configuration updated",
			Timestamp: time.Now(),
			Details: map[string]interface{}{
				"old_max_data_loss": oldConfig.MaxDataLoss,
				"new_max_data_loss": newConfig.MaxDataLoss,
				"old_level":         oldConfig.Level.String(),
				"new_level":         newConfig.Level.String(),
			},
		}
		m.alertManager.SendAlert(alert)
	}

	return nil
}

// Internal methods

// monitoringLoop runs the main RPO monitoring loop
func (m *Manager) monitoringLoop(ctx context.Context) {
	defer m.wg.Done()

	m.monitorTicker = time.NewTicker(m.config.MonitoringInterval)
	defer m.monitorTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case <-m.monitorTicker.C:
			if err := m.performMonitoringCheck(); err != nil {
				// Log error but continue monitoring
				fmt.Printf("RPO monitoring check failed: %v\n", err)
			}
		}
	}
}

// metricsLoop runs the metrics collection loop
func (m *Manager) metricsLoop(ctx context.Context) {
	defer m.wg.Done()

	m.metricsTicker = time.NewTicker(m.config.MetricsInterval)
	defer m.metricsTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case <-m.metricsTicker.C:
			if err := m.collectMetrics(); err != nil {
				// Log error but continue collecting
				fmt.Printf("RPO metrics collection failed: %v\n", err)
			}
		}
	}
}

// performMonitoringCheck performs a single monitoring check
func (m *Manager) performMonitoringCheck() error {
	compliance, err := m.CheckCompliance()
	if err != nil {
		return fmt.Errorf("compliance check failed: %w", err)
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Update current RPO
	m.currentRPO = compliance.CurrentRPO
	m.stats.CurrentRPO = compliance.CurrentRPO

	// Update max observed RPO
	if compliance.CurrentRPO > m.stats.MaxObservedRPO {
		m.stats.MaxObservedRPO = compliance.CurrentRPO
	}

	// Handle violations
	if !compliance.IsCompliant && compliance.Violation != nil {
		m.handleViolation(*compliance.Violation)
	}

	// Update stats
	m.updateStats(compliance)

	return nil
}

// collectMetrics collects and records RPO metrics
func (m *Manager) collectMetrics() error {
	if m.metricsCollector == nil {
		return nil
	}

	m.mutex.RLock()
	stats := *m.stats
	m.mutex.RUnlock()

	metric := RPOMetric{
		Timestamp:       time.Now(),
		CurrentRPO:      stats.CurrentRPO,
		MaxAllowedRPO:   m.config.MaxDataLoss,
		CheckpointAge:   time.Since(stats.LastCheckpointTime),
		WALSyncAge:      time.Since(stats.LastWALSyncTime),
		ViolationCount:  stats.ActiveViolations,
		ComplianceRatio: stats.ComplianceRatio,
	}

	return m.metricsCollector.RecordRPOMetric(metric)
}

// calculateCurrentRPO calculates the current RPO
func (m *Manager) calculateCurrentRPO() (time.Duration, error) {
	// Get uncommitted data age from WAL
	if uncommittedAge, err := m.walManager.GetUncommittedDataAge(); err == nil {
		return uncommittedAge, nil
	}

	// Fallback: use time since last checkpoint
	if lastCheckpointTime, err := m.checkpointManager.GetLastCheckpointTime(); err == nil {
		return time.Since(lastCheckpointTime), nil
	}

	// Fallback: use time since last WAL sync
	if lastSyncTime, err := m.walManager.GetLastSyncTime(); err == nil {
		return time.Since(lastSyncTime), nil
	}

	return 0, fmt.Errorf("unable to calculate current RPO")
}

// createViolation creates a new RPO violation
func (m *Manager) createViolation(actualRPO time.Duration) RPOViolation {
	severity := m.determineViolationSeverity(actualRPO)
	violationType := m.determineViolationType(actualRPO)

	return RPOViolation{
		ID:            fmt.Sprintf("rpo-violation-%d", time.Now().UnixNano()),
		Timestamp:     time.Now(),
		ActualRPO:     actualRPO,
		MaxAllowedRPO: m.config.MaxDataLoss,
		ViolationType: violationType,
		Severity:      severity,
		Resolved:      false,
		Actions:       make([]string, 0),
		Details: map[string]interface{}{
			"config_level": m.config.Level.String(),
			"strict_mode":  m.config.IsStrictMode(),
		},
	}
}

// handleViolation handles an RPO violation
func (m *Manager) handleViolation(violation RPOViolation) {
	// Add to violations list
	m.violations = append(m.violations, violation)
	m.stats.TotalViolations++
	m.stats.ActiveViolations++
	m.stats.LastViolationTime = violation.Timestamp

	// Record violation in metrics
	if m.metricsCollector != nil {
		m.metricsCollector.RecordViolation(violation)
	}

	// Send alerts
	m.sendViolationAlert(violation)

	// Take enforcement actions if enabled
	if m.config.IsStrictMode() || m.config.IsEmergencyModeEnabled() {
		go m.handleViolationAsync(violation)
	}
}

// sendViolationAlert sends an alert for an RPO violation
func (m *Manager) sendViolationAlert(violation RPOViolation) {
	if m.alertManager == nil {
		return
	}

	alert := Alert{
		Type:      AlertTypeRPOViolation,
		Severity:  m.mapViolationSeverityToAlertSeverity(violation.Severity),
		Message:   fmt.Sprintf("RPO violation detected: actual RPO %v exceeds maximum %v", violation.ActualRPO, violation.MaxAllowedRPO),
		Timestamp: violation.Timestamp,
		RPOValue:  violation.ActualRPO,
		Threshold: violation.MaxAllowedRPO,
		Details: map[string]interface{}{
			"violation_id":   violation.ID,
			"violation_type": violation.ViolationType,
			"severity":       violation.Severity,
		},
	}

	if violation.Severity >= ViolationSeverityCritical {
		m.alertManager.SendCriticalAlert(alert)
	} else {
		m.alertManager.SendAlert(alert)
	}
}

// Helper methods for violation handling
func (m *Manager) determineViolationSeverity(actualRPO time.Duration) ViolationSeverity {
	if actualRPO >= m.config.CriticalThreshold {
		return ViolationSeverityCritical
	} else if actualRPO >= m.config.AlertThreshold {
		return ViolationSeverityMajor
	} else {
		return ViolationSeverityMinor
	}
}

func (m *Manager) determineViolationType(actualRPO time.Duration) ViolationType {
	// This is a simplified determination - in practice, you'd analyze
	// the specific cause of the RPO violation
	return ViolationTypeCheckpointDelay
}

func (m *Manager) mapViolationSeverityToAlertSeverity(severity ViolationSeverity) AlertSeverity {
	switch severity {
	case ViolationSeverityMinor:
		return AlertSeverityWarning
	case ViolationSeverityMajor:
		return AlertSeverityWarning
	case ViolationSeverityCritical:
		return AlertSeverityCritical
	case ViolationSeverityEmergency:
		return AlertSeverityEmergency
	default:
		return AlertSeverityWarning
	}
}

// updateStats updates internal statistics
func (m *Manager) updateStats(compliance *RPOComplianceResult) {
	m.stats.LastCheckpointTime = compliance.LastCheckpointTime
	m.stats.LastWALSyncTime = compliance.LastWALSyncTime
	m.stats.CheckpointInterval = compliance.CheckpointAge
	m.stats.WALSyncInterval = compliance.WALSyncAge

	// Calculate compliance ratio
	totalChecks := m.stats.TotalViolations + 1 // +1 for current check
	m.stats.ComplianceRatio = float64(totalChecks-m.stats.TotalViolations) / float64(totalChecks)
}

// RPOComplianceResult represents the result of an RPO compliance check
type RPOComplianceResult struct {
	Timestamp          time.Time
	CurrentRPO         time.Duration
	MaxAllowedRPO      time.Duration
	IsCompliant        bool
	Violation          *RPOViolation
	LastCheckpointTime time.Time
	LastWALSyncTime    time.Time
	CheckpointAge      time.Duration
	WALSyncAge         time.Duration
}

// Placeholder methods for enforcement actions
func (m *Manager) determineEnforcementActions(compliance *RPOComplianceResult) []EnforcementAction {
	// Placeholder - would determine appropriate actions based on violation
	return []EnforcementAction{}
}

func (m *Manager) executeEnforcementAction(action EnforcementAction) error {
	// Placeholder - would execute the specific enforcement action
	return nil
}

func (m *Manager) handleViolationAsync(violation RPOViolation) {
	// Placeholder - would handle violation asynchronously
}

// EnforcementAction represents an action to enforce RPO compliance
type EnforcementAction struct {
	Type        string
	Description string
	Priority    int
	Timeout     time.Duration
}
