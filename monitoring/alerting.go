package monitoring

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// AlertLevel represents the severity of an alert
type AlertLevel int

const (
	AlertLevelInfo AlertLevel = iota
	AlertLevelWarning
	AlertLevelCritical
)

// Alert represents a system alert
type Alert struct {
	ID         string                 `json:"id"`
	Level      AlertLevel             `json:"level"`
	Component  string                 `json:"component"`
	Message    string                 `json:"message"`
	Timestamp  time.Time              `json:"timestamp"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Resolved   bool                   `json:"resolved"`
	ResolvedAt *time.Time             `json:"resolved_at,omitempty"`
}

// AlertHandler handles alert notifications
type AlertHandler interface {
	HandleAlert(alert Alert) error
	Name() string
}

// Alerter manages alert generation and notification
type Alerter struct {
	handlers []AlertHandler
	alerts   map[string]Alert
	mutex    sync.RWMutex

	// Alert suppression
	suppressionRules map[string]time.Duration
	lastAlerts       map[string]time.Time

	// Configuration
	maxAlerts int
	retention time.Duration

	// Escalation
	escalationRules map[AlertLevel]time.Duration
	escalationQueue map[string]time.Time
}

// NewAlerter creates a new alerter
func NewAlerter() *Alerter {
	return &Alerter{
		handlers:         make([]AlertHandler, 0),
		alerts:           make(map[string]Alert),
		suppressionRules: make(map[string]time.Duration),
		lastAlerts:       make(map[string]time.Time),
		escalationRules:  make(map[AlertLevel]time.Duration),
		escalationQueue:  make(map[string]time.Time),
		maxAlerts:        1000,
		retention:        24 * time.Hour,
	}
}

// RegisterHandler registers an alert handler
func (a *Alerter) RegisterHandler(handler AlertHandler) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.handlers = append(a.handlers, handler)
}

// SendAlert sends an alert through all registered handlers
func (a *Alerter) SendAlert(alert Alert) {
	// Generate alert ID if not provided
	if alert.ID == "" {
		alert.ID = a.generateAlertID(alert)
	}

	// Check suppression
	if a.isAlertSuppressed(alert) {
		return
	}

	// Store alert
	a.mutex.Lock()
	a.alerts[alert.ID] = alert
	a.lastAlerts[alert.Component] = alert.Timestamp

	// Clean up old alerts
	a.cleanupOldAlerts()
	a.mutex.Unlock()

	// Send to handlers
	for _, handler := range a.handlers {
		go func(h AlertHandler) {
			if err := h.HandleAlert(alert); err != nil {
				log.Printf("Alert handler %s failed: %v", h.Name(), err)
			}
		}(handler)
	}

	// Schedule escalation if needed
	a.scheduleEscalation(alert)
}

// ResolveAlert marks an alert as resolved
func (a *Alerter) ResolveAlert(alertID string) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if alert, exists := a.alerts[alertID]; exists {
		now := time.Now()
		alert.Resolved = true
		alert.ResolvedAt = &now
		a.alerts[alertID] = alert

		// Remove from escalation queue
		delete(a.escalationQueue, alertID)
	}
}

// GetAlerts returns all alerts
func (a *Alerter) GetAlerts() []Alert {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	alerts := make([]Alert, 0, len(a.alerts))
	for _, alert := range a.alerts {
		alerts = append(alerts, alert)
	}
	return alerts
}

// GetActiveAlerts returns only unresolved alerts
func (a *Alerter) GetActiveAlerts() []Alert {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	var activeAlerts []Alert
	for _, alert := range a.alerts {
		if !alert.Resolved {
			activeAlerts = append(activeAlerts, alert)
		}
	}
	return activeAlerts
}

// SetSuppressionRule sets a suppression rule for a component
func (a *Alerter) SetSuppressionRule(component string, duration time.Duration) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.suppressionRules[component] = duration
}

// SetEscalationRule sets an escalation rule for an alert level
func (a *Alerter) SetEscalationRule(level AlertLevel, duration time.Duration) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.escalationRules[level] = duration
}

// isAlertSuppressed checks if an alert should be suppressed
func (a *Alerter) isAlertSuppressed(alert Alert) bool {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	suppressionDuration, exists := a.suppressionRules[alert.Component]
	if !exists {
		return false
	}

	lastAlert, exists := a.lastAlerts[alert.Component]
	if !exists {
		return false
	}

	return time.Since(lastAlert) < suppressionDuration
}

// generateAlertID generates a unique ID for an alert
func (a *Alerter) generateAlertID(alert Alert) string {
	return fmt.Sprintf("%s-%s-%d", alert.Component, alert.Message[:min(20, len(alert.Message))], alert.Timestamp.Unix())
}

// cleanupOldAlerts removes old alerts based on retention policy
func (a *Alerter) cleanupOldAlerts() {
	if len(a.alerts) <= a.maxAlerts {
		return
	}

	cutoff := time.Now().Add(-a.retention)
	for id, alert := range a.alerts {
		if alert.Timestamp.Before(cutoff) {
			delete(a.alerts, id)
		}
	}
}

// scheduleEscalation schedules alert escalation if configured
func (a *Alerter) scheduleEscalation(alert Alert) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	escalationDuration, exists := a.escalationRules[alert.Level]
	if !exists {
		return
	}

	a.escalationQueue[alert.ID] = time.Now().Add(escalationDuration)
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// LogAlertHandler logs alerts to the standard logger
type LogAlertHandler struct{}

// NewLogAlertHandler creates a new log alert handler
func NewLogAlertHandler() *LogAlertHandler {
	return &LogAlertHandler{}
}

// HandleAlert handles an alert by logging it
func (h *LogAlertHandler) HandleAlert(alert Alert) error {
	levelStr := map[AlertLevel]string{
		AlertLevelInfo:     "INFO",
		AlertLevelWarning:  "WARNING",
		AlertLevelCritical: "CRITICAL",
	}[alert.Level]

	log.Printf("[ALERT-%s] %s: %s (Component: %s)",
		levelStr, alert.ID, alert.Message, alert.Component)

	if alert.Details != nil {
		log.Printf("[ALERT-%s] Details: %+v", levelStr, alert.Details)
	}

	return nil
}

// Name returns the handler name
func (h *LogAlertHandler) Name() string {
	return "log"
}

// ConsoleAlertHandler prints alerts to console with formatting
type ConsoleAlertHandler struct{}

// NewConsoleAlertHandler creates a new console alert handler
func NewConsoleAlertHandler() *ConsoleAlertHandler {
	return &ConsoleAlertHandler{}
}

// HandleAlert handles an alert by printing to console
func (h *ConsoleAlertHandler) HandleAlert(alert Alert) error {
	levelStr := map[AlertLevel]string{
		AlertLevelInfo:     "ℹ️  INFO",
		AlertLevelWarning:  "⚠️  WARNING",
		AlertLevelCritical: "🚨 CRITICAL",
	}[alert.Level]

	fmt.Printf("\n%s [%s] %s\n", levelStr, alert.Timestamp.Format("15:04:05"), alert.Message)
	fmt.Printf("   Component: %s\n", alert.Component)

	if alert.Details != nil {
		fmt.Printf("   Details: %+v\n", alert.Details)
	}

	return nil
}

// Name returns the handler name
func (h *ConsoleAlertHandler) Name() string {
	return "console"
}

// FileAlertHandler writes alerts to a file
type FileAlertHandler struct {
	filename string
}

// NewFileAlertHandler creates a new file alert handler
func NewFileAlertHandler(filename string) *FileAlertHandler {
	return &FileAlertHandler{
		filename: filename,
	}
}

// HandleAlert handles an alert by writing to file
func (h *FileAlertHandler) HandleAlert(alert Alert) error {
	// This is a simplified implementation
	// In production, you'd want proper file handling, rotation, etc.
	levelStr := map[AlertLevel]string{
		AlertLevelInfo:     "INFO",
		AlertLevelWarning:  "WARNING",
		AlertLevelCritical: "CRITICAL",
	}[alert.Level]

	message := fmt.Sprintf("[%s] %s %s: %s (Component: %s)\n",
		alert.Timestamp.Format(time.RFC3339),
		levelStr,
		alert.ID,
		alert.Message,
		alert.Component)

	// In a real implementation, you'd write to file here
	log.Printf("Would write to %s: %s", h.filename, message)

	return nil
}

// Name returns the handler name
func (h *FileAlertHandler) Name() string {
	return "file"
}

// WebhookAlertHandler sends alerts to a webhook endpoint
type WebhookAlertHandler struct {
	url string
}

// NewWebhookAlertHandler creates a new webhook alert handler
func NewWebhookAlertHandler(url string) *WebhookAlertHandler {
	return &WebhookAlertHandler{
		url: url,
	}
}

// HandleAlert handles an alert by sending to webhook
func (h *WebhookAlertHandler) HandleAlert(alert Alert) error {
	// This is a simplified implementation
	// In production, you'd make actual HTTP requests
	log.Printf("Would send webhook to %s: %+v", h.url, alert)
	return nil
}

// Name returns the handler name
func (h *WebhookAlertHandler) Name() string {
	return "webhook"
}

// AlertManager manages the complete alerting system
type AlertManager struct {
	alerter       *Alerter
	healthChecker *HealthChecker

	// Background processing
	ctx      context.Context
	cancel   context.CancelFunc
	stopChan chan struct{}
	doneChan chan struct{}
}

// NewAlertManager creates a new alert manager
func NewAlertManager() *AlertManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &AlertManager{
		alerter:  NewAlerter(),
		ctx:      ctx,
		cancel:   cancel,
		stopChan: make(chan struct{}),
		doneChan: make(chan struct{}),
	}
}

// Start starts the alert manager
func (am *AlertManager) Start() {
	go am.processEscalations()
}

// Stop stops the alert manager
func (am *AlertManager) Stop() {
	am.cancel()
	close(am.stopChan)
	<-am.doneChan
}

// GetAlerter returns the alerter
func (am *AlertManager) GetAlerter() *Alerter {
	return am.alerter
}

// SetHealthChecker sets the health checker
func (am *AlertManager) SetHealthChecker(hc *HealthChecker) {
	am.healthChecker = hc
}

// processEscalations processes alert escalations
func (am *AlertManager) processEscalations() {
	defer close(am.doneChan)

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-am.ctx.Done():
			return
		case <-am.stopChan:
			return
		case <-ticker.C:
			am.checkEscalations()
		}
	}
}

// checkEscalations checks for alerts that need escalation
func (am *AlertManager) checkEscalations() {
	am.alerter.mutex.RLock()
	escalationQueue := make(map[string]time.Time)
	for k, v := range am.alerter.escalationQueue {
		escalationQueue[k] = v
	}
	am.alerter.mutex.RUnlock()

	now := time.Now()
	for alertID, escalationTime := range escalationQueue {
		if now.After(escalationTime) {
			// Escalate alert
			am.alerter.mutex.RLock()
			alert, exists := am.alerter.alerts[alertID]
			am.alerter.mutex.RUnlock()

			if exists && !alert.Resolved {
				escalatedAlert := alert
				escalatedAlert.ID = alertID + "-escalated"
				escalatedAlert.Message = "ESCALATED: " + alert.Message
				escalatedAlert.Timestamp = now

				am.alerter.SendAlert(escalatedAlert)
			}

			// Remove from escalation queue
			am.alerter.mutex.Lock()
			delete(am.alerter.escalationQueue, alertID)
			am.alerter.mutex.Unlock()
		}
	}
}
