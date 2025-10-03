package rpo

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// DefaultAlertManager provides default alerting functionality
type DefaultAlertManager struct {
	config *AlertConfig
	mutex  sync.RWMutex

	// Alert handlers
	handlers map[AlertType][]AlertHandler

	// Alert history
	alertHistory []Alert
	maxHistory   int

	// Rate limiting
	lastAlerts map[AlertType]time.Time
	rateLimits map[AlertType]time.Duration
}

// AlertConfig holds alerting configuration
type AlertConfig struct {
	// Enable different alert types
	EnableRPOViolationAlerts bool `json:"enable_rpo_violation_alerts"`
	EnableCriticalAlerts     bool `json:"enable_critical_alerts"`
	EnableRecoveryAlerts     bool `json:"enable_recovery_alerts"`

	// Rate limiting
	MinAlertInterval time.Duration `json:"min_alert_interval"`
	MaxAlertsPerHour int           `json:"max_alerts_per_hour"`

	// Alert destinations
	LogAlerts   bool `json:"log_alerts"`
	EmailAlerts bool `json:"email_alerts"`
	SlackAlerts bool `json:"slack_alerts"`

	// Email configuration
	EmailRecipients []string `json:"email_recipients"`
	SMTPServer      string   `json:"smtp_server"`
	SMTPPort        int      `json:"smtp_port"`
	SMTPUsername    string   `json:"smtp_username"`
	SMTPPassword    string   `json:"smtp_password"`

	// Slack configuration
	SlackWebhookURL string `json:"slack_webhook_url"`
	SlackChannel    string `json:"slack_channel"`

	// Alert formatting
	IncludeMetrics   bool `json:"include_metrics"`
	IncludeTimestamp bool `json:"include_timestamp"`
	IncludeHostname  bool `json:"include_hostname"`

	// History settings
	MaxAlertHistory  int           `json:"max_alert_history"`
	HistoryRetention time.Duration `json:"history_retention"`
}

// AlertHandler interface for handling alerts
type AlertHandler interface {
	HandleAlert(alert Alert) error
	GetHandlerType() string
}

// LogAlertHandler logs alerts to the system log
type LogAlertHandler struct{}

// EmailAlertHandler sends alerts via email
type EmailAlertHandler struct {
	config *AlertConfig
}

// SlackAlertHandler sends alerts to Slack
type SlackAlertHandler struct {
	config *AlertConfig
}

// NewDefaultAlertManager creates a new default alert manager
func NewDefaultAlertManager(config *AlertConfig) *DefaultAlertManager {
	if config == nil {
		config = DefaultAlertConfig()
	}

	manager := &DefaultAlertManager{
		config:       config,
		handlers:     make(map[AlertType][]AlertHandler),
		alertHistory: make([]Alert, 0),
		maxHistory:   config.MaxAlertHistory,
		lastAlerts:   make(map[AlertType]time.Time),
		rateLimits:   make(map[AlertType]time.Duration),
	}

	// Set up default rate limits
	manager.rateLimits[AlertTypeRPOViolation] = config.MinAlertInterval
	manager.rateLimits[AlertTypeRPOCritical] = config.MinAlertInterval / 2
	manager.rateLimits[AlertTypeCheckpointFailed] = config.MinAlertInterval
	manager.rateLimits[AlertTypeWALSyncFailed] = config.MinAlertInterval
	manager.rateLimits[AlertTypeRPORecovered] = config.MinAlertInterval * 2

	// Register default handlers
	manager.registerDefaultHandlers()

	return manager
}

// DefaultAlertConfig returns a default alert configuration
func DefaultAlertConfig() *AlertConfig {
	return &AlertConfig{
		EnableRPOViolationAlerts: true,
		EnableCriticalAlerts:     true,
		EnableRecoveryAlerts:     true,
		MinAlertInterval:         1 * time.Minute,
		MaxAlertsPerHour:         60,
		LogAlerts:                true,
		EmailAlerts:              false,
		SlackAlerts:              false,
		IncludeMetrics:           true,
		IncludeTimestamp:         true,
		IncludeHostname:          true,
		MaxAlertHistory:          1000,
		HistoryRetention:         24 * time.Hour,
	}
}

// SendAlert sends a regular alert
func (am *DefaultAlertManager) SendAlert(alert Alert) error {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	// Check if alerts of this type are enabled
	if !am.isAlertTypeEnabled(alert.Type) {
		return nil
	}

	// Apply rate limiting
	if am.isRateLimited(alert.Type) {
		return nil
	}

	// Add to history
	am.addToHistory(alert)

	// Send to handlers
	return am.sendToHandlers(alert)
}

// SendCriticalAlert sends a critical alert (bypasses some rate limiting)
func (am *DefaultAlertManager) SendCriticalAlert(alert Alert) error {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	// Critical alerts are always enabled
	alert.Severity = AlertSeverityCritical

	// Reduced rate limiting for critical alerts
	if am.isRateLimited(alert.Type) && alert.Severity < AlertSeverityCritical {
		return nil
	}

	// Add to history
	am.addToHistory(alert)

	// Send to handlers
	return am.sendToHandlers(alert)
}

// RegisterHandler registers an alert handler for specific alert types
func (am *DefaultAlertManager) RegisterHandler(alertType AlertType, handler AlertHandler) {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	if am.handlers[alertType] == nil {
		am.handlers[alertType] = make([]AlertHandler, 0)
	}

	am.handlers[alertType] = append(am.handlers[alertType], handler)
}

// GetAlertHistory returns the alert history
func (am *DefaultAlertManager) GetAlertHistory(limit int) []Alert {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	if limit <= 0 || limit > len(am.alertHistory) {
		limit = len(am.alertHistory)
	}

	// Return most recent alerts
	start := len(am.alertHistory) - limit
	history := make([]Alert, limit)
	copy(history, am.alertHistory[start:])

	return history
}

// ClearAlertHistory clears the alert history
func (am *DefaultAlertManager) ClearAlertHistory() {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	am.alertHistory = make([]Alert, 0)
}

// Internal methods

// registerDefaultHandlers registers default alert handlers
func (am *DefaultAlertManager) registerDefaultHandlers() {
	// Register log handler for all alert types
	if am.config.LogAlerts {
		logHandler := &LogAlertHandler{}
		for alertType := AlertTypeRPOViolation; alertType <= AlertTypeRPORecovered; alertType++ {
			am.RegisterHandler(alertType, logHandler)
		}
	}

	// Register email handler if configured
	if am.config.EmailAlerts && len(am.config.EmailRecipients) > 0 {
		emailHandler := &EmailAlertHandler{config: am.config}
		for alertType := AlertTypeRPOViolation; alertType <= AlertTypeRPORecovered; alertType++ {
			am.RegisterHandler(alertType, emailHandler)
		}
	}

	// Register Slack handler if configured
	if am.config.SlackAlerts && am.config.SlackWebhookURL != "" {
		slackHandler := &SlackAlertHandler{config: am.config}
		for alertType := AlertTypeRPOViolation; alertType <= AlertTypeRPORecovered; alertType++ {
			am.RegisterHandler(alertType, slackHandler)
		}
	}
}

// isAlertTypeEnabled checks if an alert type is enabled
func (am *DefaultAlertManager) isAlertTypeEnabled(alertType AlertType) bool {
	switch alertType {
	case AlertTypeRPOViolation:
		return am.config.EnableRPOViolationAlerts
	case AlertTypeRPOCritical:
		return am.config.EnableCriticalAlerts
	case AlertTypeRPORecovered:
		return am.config.EnableRecoveryAlerts
	case AlertTypeCheckpointFailed, AlertTypeWALSyncFailed:
		return am.config.EnableCriticalAlerts
	default:
		return true
	}
}

// isRateLimited checks if an alert type is rate limited
func (am *DefaultAlertManager) isRateLimited(alertType AlertType) bool {
	lastAlert, exists := am.lastAlerts[alertType]
	if !exists {
		am.lastAlerts[alertType] = time.Now()
		return false
	}

	rateLimit, exists := am.rateLimits[alertType]
	if !exists {
		rateLimit = am.config.MinAlertInterval
	}

	if time.Since(lastAlert) < rateLimit {
		return true
	}

	am.lastAlerts[alertType] = time.Now()
	return false
}

// addToHistory adds an alert to the history
func (am *DefaultAlertManager) addToHistory(alert Alert) {
	am.alertHistory = append(am.alertHistory, alert)

	// Trim history if it exceeds maximum
	if len(am.alertHistory) > am.maxHistory {
		am.alertHistory = am.alertHistory[1:]
	}
}

// sendToHandlers sends an alert to all registered handlers
func (am *DefaultAlertManager) sendToHandlers(alert Alert) error {
	handlers, exists := am.handlers[alert.Type]
	if !exists || len(handlers) == 0 {
		return nil
	}

	var lastError error
	for _, handler := range handlers {
		if err := handler.HandleAlert(alert); err != nil {
			lastError = err
			// Continue to other handlers even if one fails
		}
	}

	return lastError
}

// Alert handler implementations

// HandleAlert implements AlertHandler for LogAlertHandler
func (h *LogAlertHandler) HandleAlert(alert Alert) error {
	logLevel := "INFO"
	switch alert.Severity {
	case AlertSeverityWarning:
		logLevel = "WARN"
	case AlertSeverityCritical:
		logLevel = "ERROR"
	case AlertSeverityEmergency:
		logLevel = "FATAL"
	}

	message := fmt.Sprintf("[%s] RPO Alert: %s (Type: %s, RPO: %v, Threshold: %v)",
		logLevel, alert.Message, alert.Type.String(), alert.RPOValue, alert.Threshold)

	log.Printf("%s", message)
	return nil
}

// GetHandlerType implements AlertHandler for LogAlertHandler
func (h *LogAlertHandler) GetHandlerType() string {
	return "log"
}

// HandleAlert implements AlertHandler for EmailAlertHandler
func (h *EmailAlertHandler) HandleAlert(alert Alert) error {
	// Placeholder for email sending logic
	// In a real implementation, this would use SMTP to send emails
	log.Printf("EMAIL ALERT: %s", alert.Message)
	return nil
}

// GetHandlerType implements AlertHandler for EmailAlertHandler
func (h *EmailAlertHandler) GetHandlerType() string {
	return "email"
}

// HandleAlert implements AlertHandler for SlackAlertHandler
func (h *SlackAlertHandler) HandleAlert(alert Alert) error {
	// Placeholder for Slack webhook logic
	// In a real implementation, this would send HTTP POST to Slack webhook
	log.Printf("SLACK ALERT: %s", alert.Message)
	return nil
}

// GetHandlerType implements AlertHandler for SlackAlertHandler
func (h *SlackAlertHandler) GetHandlerType() string {
	return "slack"
}

// String methods for alert types

// String returns string representation of alert type
func (t AlertType) String() string {
	switch t {
	case AlertTypeRPOViolation:
		return "rpo_violation"
	case AlertTypeRPOCritical:
		return "rpo_critical"
	case AlertTypeCheckpointFailed:
		return "checkpoint_failed"
	case AlertTypeWALSyncFailed:
		return "wal_sync_failed"
	case AlertTypeRPORecovered:
		return "rpo_recovered"
	default:
		return fmt.Sprintf("unknown(%d)", t)
	}
}

// String returns string representation of alert severity
func (s AlertSeverity) String() string {
	switch s {
	case AlertSeverityInfo:
		return "info"
	case AlertSeverityWarning:
		return "warning"
	case AlertSeverityCritical:
		return "critical"
	case AlertSeverityEmergency:
		return "emergency"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

// String returns string representation of violation type
func (t ViolationType) String() string {
	switch t {
	case ViolationTypeCheckpointDelay:
		return "checkpoint_delay"
	case ViolationTypeWALSyncDelay:
		return "wal_sync_delay"
	case ViolationTypeDataLoss:
		return "data_loss"
	case ViolationTypeSystemFailure:
		return "system_failure"
	default:
		return fmt.Sprintf("unknown(%d)", t)
	}
}

// String returns string representation of violation severity
func (s ViolationSeverity) String() string {
	switch s {
	case ViolationSeverityMinor:
		return "minor"
	case ViolationSeverityMajor:
		return "major"
	case ViolationSeverityCritical:
		return "critical"
	case ViolationSeverityEmergency:
		return "emergency"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}
