package integrity

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// IntegrityMonitor provides comprehensive monitoring and alerting for data integrity
type IntegrityMonitor struct {
	config          *IntegrityConfig
	metrics         *IntegrityMetrics
	alertHandlers   []AlertHandler
	healthStatus    *IntegrityHealthStatus
	eventLog        []IntegrityEvent
	mutex           sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	lastHealthCheck time.Time
	operationStats  map[string]*OperationMetrics
}

// IntegrityEvent represents an integrity-related event
type IntegrityEvent struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Type      IntegrityEventType     `json:"type"`
	Component string                 `json:"component"`
	Operation string                 `json:"operation"`
	Success   bool                   `json:"success"`
	Duration  time.Duration          `json:"duration"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// IntegrityEventType represents different types of integrity events
type IntegrityEventType string

const (
	EventTypeChecksumOperation  IntegrityEventType = "checksum_operation"
	EventTypeCorruptionDetected IntegrityEventType = "corruption_detected"
	EventTypeIntegrityCheck     IntegrityEventType = "integrity_check"
	EventTypeRepairAttempt      IntegrityEventType = "repair_attempt"
	EventTypeHealthCheck        IntegrityEventType = "health_check"
	EventTypeAlert              IntegrityEventType = "alert"
)

// NewIntegrityMonitor creates a new integrity monitor
func NewIntegrityMonitor(config *IntegrityConfig) *IntegrityMonitor {
	if config == nil {
		config = DefaultIntegrityConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	monitor := &IntegrityMonitor{
		config:         config,
		alertHandlers:  make([]AlertHandler, 0),
		eventLog:       make([]IntegrityEvent, 0),
		ctx:            ctx,
		cancel:         cancel,
		operationStats: make(map[string]*OperationMetrics),
		metrics: &IntegrityMetrics{
			ChecksumOperations:  &OperationMetrics{},
			CorruptionDetection: &OperationMetrics{},
			BackgroundScans:     &ScanMetrics{},
			FileOperations:      &OperationMetrics{},
			PerformanceMetrics:  &PerformanceMetrics{},
			LastUpdated:         time.Now(),
		},
		healthStatus: &IntegrityHealthStatus{
			Status:          HealthStatusHealthy,
			LastCheckTime:   time.Now(),
			ComponentHealth: make(map[string]HealthStatus),
			ActiveScans:     0,
			RecentEvents:    make([]CorruptionEvent, 0),
		},
	}

	// Start background monitoring
	monitor.startBackgroundMonitoring()

	return monitor
}

// RecordChecksumOperation records metrics for checksum operations
func (im *IntegrityMonitor) RecordChecksumOperation(operation string, duration int64, success bool) {
	im.mutex.Lock()
	defer im.mutex.Unlock()

	// Update operation-specific metrics
	if _, exists := im.operationStats[operation]; !exists {
		im.operationStats[operation] = &OperationMetrics{
			MinLatency: time.Duration(duration),
			MaxLatency: time.Duration(duration),
		}
	}

	stats := im.operationStats[operation]
	stats.TotalOperations++
	stats.LastOperation = time.Now()

	latency := time.Duration(duration)
	if latency < stats.MinLatency || stats.MinLatency == 0 {
		stats.MinLatency = latency
	}
	if latency > stats.MaxLatency {
		stats.MaxLatency = latency
	}

	// Update average latency
	if stats.TotalOperations > 0 {
		totalLatency := time.Duration(int64(stats.AverageLatency) * (stats.TotalOperations - 1))
		stats.AverageLatency = (totalLatency + latency) / time.Duration(stats.TotalOperations)
	}

	if success {
		stats.SuccessfulOps++
	} else {
		stats.FailedOps++
	}

	// Update global checksum metrics
	im.metrics.ChecksumOperations.TotalOperations++
	if success {
		im.metrics.ChecksumOperations.SuccessfulOps++
	} else {
		im.metrics.ChecksumOperations.FailedOps++
	}

	// Record event
	event := IntegrityEvent{
		ID:        im.generateEventID(),
		Timestamp: time.Now(),
		Type:      EventTypeChecksumOperation,
		Component: "checksum_engine",
		Operation: operation,
		Success:   success,
		Duration:  latency,
		Details: map[string]interface{}{
			"operation_type": operation,
		},
	}

	if !success {
		event.Error = "checksum operation failed"
	}

	im.recordEvent(event)

	// Check if we need to trigger alerts
	im.checkAlertThresholds()
}

// RecordCorruptionEvent records a corruption event
func (im *IntegrityMonitor) RecordCorruptionEvent(event *CorruptionEvent) {
	im.mutex.Lock()
	defer im.mutex.Unlock()

	// Update corruption detection metrics
	im.metrics.CorruptionDetection.TotalOperations++
	im.metrics.CorruptionDetection.FailedOps++

	// Record integrity event
	integrityEvent := IntegrityEvent{
		ID:        im.generateEventID(),
		Timestamp: event.Timestamp,
		Type:      EventTypeCorruptionDetected,
		Component: "corruption_detector",
		Operation: "detect_corruption",
		Success:   false,
		Details: map[string]interface{}{
			"corruption_type": event.Type,
			"severity":        event.Severity,
			"location":        event.Location,
			"size":            event.Size,
			"expected":        event.Expected,
			"actual":          event.Actual,
		},
		Error: event.Description,
	}

	im.recordEvent(integrityEvent)

	// Add to recent events in health status
	im.healthStatus.RecentEvents = append(im.healthStatus.RecentEvents, *event)

	// Keep only the last 10 events
	if len(im.healthStatus.RecentEvents) > 10 {
		im.healthStatus.RecentEvents = im.healthStatus.RecentEvents[1:]
	}

	// Trigger critical alert for corruption
	im.triggerCorruptionAlert(event)

	// Update health status
	im.updateHealthStatus()
}

// RecordIntegrityCheck records the result of an integrity check
func (im *IntegrityMonitor) RecordIntegrityCheck(component string, success bool, details map[string]interface{}) {
	im.mutex.Lock()
	defer im.mutex.Unlock()

	event := IntegrityEvent{
		ID:        im.generateEventID(),
		Timestamp: time.Now(),
		Type:      EventTypeIntegrityCheck,
		Component: component,
		Operation: "integrity_check",
		Success:   success,
		Details:   details,
	}

	if !success {
		event.Error = "integrity check failed"
	}

	im.recordEvent(event)

	// Update component health
	if success {
		im.healthStatus.ComponentHealth[component] = HealthStatusHealthy
	} else {
		im.healthStatus.ComponentHealth[component] = HealthStatusCritical
	}

	// Update overall health status
	im.updateHealthStatus()
}

// PerformHealthCheck performs a comprehensive health check
func (im *IntegrityMonitor) PerformHealthCheck() *IntegrityHealthStatus {
	im.mutex.Lock()
	defer im.mutex.Unlock()

	im.lastHealthCheck = time.Now()
	im.healthStatus.LastCheckTime = im.lastHealthCheck

	// Check each component
	im.checkComponentHealth("checksum_engine")
	im.checkComponentHealth("corruption_detector")
	im.checkComponentHealth("wal_integrity")
	im.checkComponentHealth("background_scanner")

	// Update overall status
	im.updateHealthStatus()

	// Update metrics
	im.metrics.LastUpdated = time.Now()
	im.healthStatus.Metrics = im.metrics

	// Record health check event
	event := IntegrityEvent{
		ID:        im.generateEventID(),
		Timestamp: time.Now(),
		Type:      EventTypeHealthCheck,
		Component: "integrity_monitor",
		Operation: "health_check",
		Success:   im.healthStatus.Status != HealthStatusCritical,
		Details: map[string]interface{}{
			"overall_status": im.healthStatus.Status,
			"active_scans":   im.healthStatus.ActiveScans,
		},
	}

	im.recordEvent(event)

	// Return a copy to avoid race conditions
	return im.copyHealthStatus()
}

// GetIntegrityMetrics returns current integrity metrics
func (im *IntegrityMonitor) GetIntegrityMetrics() *IntegrityMetrics {
	im.mutex.RLock()
	defer im.mutex.RUnlock()

	// Return a copy to avoid race conditions
	return im.copyMetrics()
}

// RegisterAlertHandler registers a handler for alerts
func (im *IntegrityMonitor) RegisterAlertHandler(handler AlertHandler) {
	im.mutex.Lock()
	defer im.mutex.Unlock()
	im.alertHandlers = append(im.alertHandlers, handler)
}

// TriggerAlert triggers an alert with the specified level and message
func (im *IntegrityMonitor) TriggerAlert(level AlertLevel, message string, details map[string]interface{}) {
	im.mutex.RLock()
	handlers := make([]AlertHandler, len(im.alertHandlers))
	copy(handlers, im.alertHandlers)
	im.mutex.RUnlock()

	// Record alert event
	event := IntegrityEvent{
		ID:        im.generateEventID(),
		Timestamp: time.Now(),
		Type:      EventTypeAlert,
		Component: "integrity_monitor",
		Operation: "trigger_alert",
		Success:   true,
		Details: map[string]interface{}{
			"alert_level": level,
			"message":     message,
		},
	}

	for k, v := range details {
		event.Details[k] = v
	}

	im.mutex.Lock()
	im.recordEvent(event)
	im.mutex.Unlock()

	// Notify all handlers
	for _, handler := range handlers {
		go func(h AlertHandler) {
			if err := h.HandleAlert(level, message, details); err != nil {
				log.Printf("Alert handler error: %v", err)
			}
		}(handler)
	}
}

// GetEventHistory returns the event history
func (im *IntegrityMonitor) GetEventHistory(limit int) []IntegrityEvent {
	im.mutex.RLock()
	defer im.mutex.RUnlock()

	if limit <= 0 || limit > len(im.eventLog) {
		limit = len(im.eventLog)
	}

	// Return the most recent events
	start := len(im.eventLog) - limit
	events := make([]IntegrityEvent, limit)
	copy(events, im.eventLog[start:])

	return events
}

// ExportMetrics exports metrics to a JSON file
func (im *IntegrityMonitor) ExportMetrics(filePath string) error {
	im.mutex.RLock()
	defer im.mutex.RUnlock()

	data := struct {
		Metrics      *IntegrityMetrics      `json:"metrics"`
		HealthStatus *IntegrityHealthStatus `json:"health_status"`
		EventHistory []IntegrityEvent       `json:"event_history"`
		ExportTime   time.Time              `json:"export_time"`
	}{
		Metrics:      im.copyMetrics(),
		HealthStatus: im.copyHealthStatus(),
		EventHistory: im.eventLog,
		ExportTime:   time.Now(),
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	return os.WriteFile(filePath, jsonData, 0644)
}

// Close stops the integrity monitor
func (im *IntegrityMonitor) Close() error {
	im.cancel()
	im.wg.Wait()
	return nil
}

// Private methods

func (im *IntegrityMonitor) startBackgroundMonitoring() {
	im.wg.Add(1)
	go func() {
		defer im.wg.Done()

		ticker := time.NewTicker(30 * time.Second) // Health check every 30 seconds
		defer ticker.Stop()

		for {
			select {
			case <-im.ctx.Done():
				return
			case <-ticker.C:
				im.PerformHealthCheck()
			}
		}
	}()
}

func (im *IntegrityMonitor) recordEvent(event IntegrityEvent) {
	im.eventLog = append(im.eventLog, event)

	// Keep only the last 1000 events
	if len(im.eventLog) > 1000 {
		im.eventLog = im.eventLog[1:]
	}
}

func (im *IntegrityMonitor) generateEventID() string {
	return fmt.Sprintf("integrity_event_%d_%d", time.Now().UnixNano(), len(im.eventLog))
}

func (im *IntegrityMonitor) checkComponentHealth(component string) {
	// Simple health check logic - can be expanded
	switch component {
	case "checksum_engine":
		if im.metrics.ChecksumOperations.FailedOps > 0 {
			failureRate := float64(im.metrics.ChecksumOperations.FailedOps) / float64(im.metrics.ChecksumOperations.TotalOperations)
			if failureRate > im.config.AlertThresholds.FailureRate {
				im.healthStatus.ComponentHealth[component] = HealthStatusCritical
			} else if failureRate > im.config.AlertThresholds.FailureRate/2 {
				im.healthStatus.ComponentHealth[component] = HealthStatusWarning
			} else {
				im.healthStatus.ComponentHealth[component] = HealthStatusHealthy
			}
		} else {
			im.healthStatus.ComponentHealth[component] = HealthStatusHealthy
		}

	case "corruption_detector":
		if im.metrics.CorruptionDetection.TotalOperations > 0 {
			im.healthStatus.ComponentHealth[component] = HealthStatusWarning
		} else {
			im.healthStatus.ComponentHealth[component] = HealthStatusHealthy
		}

	default:
		im.healthStatus.ComponentHealth[component] = HealthStatusHealthy
	}
}

func (im *IntegrityMonitor) updateHealthStatus() {
	// Determine overall health based on component health
	overallStatus := HealthStatusHealthy

	for _, status := range im.healthStatus.ComponentHealth {
		switch status {
		case HealthStatusCritical:
			overallStatus = HealthStatusCritical
		case HealthStatusWarning:
			if overallStatus == HealthStatusHealthy {
				overallStatus = HealthStatusWarning
			}
		}
	}

	im.healthStatus.Status = overallStatus

	// Add recommendations based on status
	im.healthStatus.Recommendations = im.generateRecommendations()
}

func (im *IntegrityMonitor) generateRecommendations() []string {
	var recommendations []string

	// Check failure rates
	if im.metrics.ChecksumOperations.TotalOperations > 0 {
		failureRate := float64(im.metrics.ChecksumOperations.FailedOps) / float64(im.metrics.ChecksumOperations.TotalOperations)
		if failureRate > im.config.AlertThresholds.FailureRate {
			recommendations = append(recommendations, "High checksum operation failure rate detected. Consider investigating underlying storage issues.")
		}
	}

	// Check corruption events
	if len(im.healthStatus.RecentEvents) > 0 {
		recommendations = append(recommendations, "Recent corruption events detected. Consider running integrity scans and checking storage health.")
	}

	// Check response times
	if im.metrics.ChecksumOperations.AverageLatency > im.config.AlertThresholds.ResponseTime {
		recommendations = append(recommendations, "High checksum operation latency detected. Consider optimizing storage performance.")
	}

	return recommendations
}

func (im *IntegrityMonitor) checkAlertThresholds() {
	// Check failure rate threshold
	if im.metrics.ChecksumOperations.TotalOperations > 0 {
		failureRate := float64(im.metrics.ChecksumOperations.FailedOps) / float64(im.metrics.ChecksumOperations.TotalOperations)
		if failureRate > im.config.AlertThresholds.FailureRate {
			im.TriggerAlert(AlertLevelCritical, "High checksum operation failure rate", map[string]interface{}{
				"failure_rate": failureRate,
				"threshold":    im.config.AlertThresholds.FailureRate,
			})
		}
	}

	// Check response time threshold
	if im.metrics.ChecksumOperations.AverageLatency > im.config.AlertThresholds.ResponseTime {
		im.TriggerAlert(AlertLevelWarning, "High checksum operation latency", map[string]interface{}{
			"average_latency": im.metrics.ChecksumOperations.AverageLatency,
			"threshold":       im.config.AlertThresholds.ResponseTime,
		})
	}
}

func (im *IntegrityMonitor) triggerCorruptionAlert(event *CorruptionEvent) {
	level := AlertLevelError
	if event.Severity == SeverityCritical {
		level = AlertLevelCritical
	}

	im.TriggerAlert(level, fmt.Sprintf("Data corruption detected: %s", event.Description), map[string]interface{}{
		"corruption_type": event.Type,
		"location":        event.Location,
		"severity":        event.Severity,
		"size":            event.Size,
	})
}

func (im *IntegrityMonitor) copyMetrics() *IntegrityMetrics {
	return &IntegrityMetrics{
		ChecksumOperations:  im.copyOperationMetrics(im.metrics.ChecksumOperations),
		CorruptionDetection: im.copyOperationMetrics(im.metrics.CorruptionDetection),
		BackgroundScans:     im.copyScanMetrics(im.metrics.BackgroundScans),
		FileOperations:      im.copyOperationMetrics(im.metrics.FileOperations),
		PerformanceMetrics:  im.copyPerformanceMetrics(im.metrics.PerformanceMetrics),
		LastUpdated:         im.metrics.LastUpdated,
	}
}

func (im *IntegrityMonitor) copyOperationMetrics(src *OperationMetrics) *OperationMetrics {
	if src == nil {
		return &OperationMetrics{}
	}
	return &OperationMetrics{
		TotalOperations:  src.TotalOperations,
		SuccessfulOps:    src.SuccessfulOps,
		FailedOps:        src.FailedOps,
		AverageLatency:   src.AverageLatency,
		MaxLatency:       src.MaxLatency,
		MinLatency:       src.MinLatency,
		OperationsPerSec: src.OperationsPerSec,
		LastOperation:    src.LastOperation,
	}
}

func (im *IntegrityMonitor) copyScanMetrics(src *ScanMetrics) *ScanMetrics {
	if src == nil {
		return &ScanMetrics{}
	}
	return &ScanMetrics{
		TotalScans:      src.TotalScans,
		CompletedScans:  src.CompletedScans,
		FailedScans:     src.FailedScans,
		FilesScanned:    src.FilesScanned,
		BytesScanned:    src.BytesScanned,
		AverageScanTime: src.AverageScanTime,
		LastScanTime:    src.LastScanTime,
		ActiveScans:     src.ActiveScans,
	}
}

func (im *IntegrityMonitor) copyPerformanceMetrics(src *PerformanceMetrics) *PerformanceMetrics {
	if src == nil {
		return &PerformanceMetrics{}
	}
	return &PerformanceMetrics{
		ThroughputBytesPerSec: src.ThroughputBytesPerSec,
		MemoryUsage:           src.MemoryUsage,
		CPUUsage:              src.CPUUsage,
		DiskIOOperations:      src.DiskIOOperations,
		CacheHitRate:          src.CacheHitRate,
		AverageResponseTime:   src.AverageResponseTime,
	}
}

func (im *IntegrityMonitor) copyHealthStatus() *IntegrityHealthStatus {
	componentHealth := make(map[string]HealthStatus)
	for k, v := range im.healthStatus.ComponentHealth {
		componentHealth[k] = v
	}

	recentEvents := make([]CorruptionEvent, len(im.healthStatus.RecentEvents))
	copy(recentEvents, im.healthStatus.RecentEvents)

	recommendations := make([]string, len(im.healthStatus.Recommendations))
	copy(recommendations, im.healthStatus.Recommendations)

	return &IntegrityHealthStatus{
		Status:          im.healthStatus.Status,
		LastCheckTime:   im.healthStatus.LastCheckTime,
		ComponentHealth: componentHealth,
		ActiveScans:     im.healthStatus.ActiveScans,
		RecentEvents:    recentEvents,
		Metrics:         im.copyMetrics(),
		Recommendations: recommendations,
	}
}
