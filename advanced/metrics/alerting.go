package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"os"
	"sync"
	"time"
)

// AlertSeverity represents the severity level of an alert
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityCritical AlertSeverity = "critical"
)

// AlertingSystem provides comprehensive alerting with configurable thresholds and notifications
type AlertingSystem struct {
	config       *AlertingConfig
	rules        map[string]*AlertRule
	channels     map[string]NotificationChannel
	activeAlerts map[string]*Alert
	alertHistory []*Alert
	metrics      *PrometheusMetrics

	mutex  sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

// AlertingConfig holds configuration for the alerting system
type AlertingConfig struct {
	Enabled            bool          `json:"enabled"`
	EvaluationInterval time.Duration `json:"evaluation_interval"`
	AlertRetention     time.Duration `json:"alert_retention"`
	MaxAlerts          int           `json:"max_alerts"`
	DefaultSuppression time.Duration `json:"default_suppression"`
	EscalationTimeout  time.Duration `json:"escalation_timeout"`

	// Notification settings
	EmailEnabled   bool `json:"email_enabled"`
	SlackEnabled   bool `json:"slack_enabled"`
	WebhookEnabled bool `json:"webhook_enabled"`

	// SMTP settings for email notifications
	SMTPHost     string   `json:"smtp_host"`
	SMTPPort     int      `json:"smtp_port"`
	SMTPUsername string   `json:"smtp_username"`
	SMTPPassword string   `json:"smtp_password"`
	EmailFrom    string   `json:"email_from"`
	EmailTo      []string `json:"email_to"`

	// Slack settings
	SlackWebhookURL string `json:"slack_webhook_url"`
	SlackChannel    string `json:"slack_channel"`

	// Webhook settings
	WebhookURL     string        `json:"webhook_url"`
	WebhookTimeout time.Duration `json:"webhook_timeout"`
}

// AlertRule defines conditions for triggering alerts
type AlertRule struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Metric      string            `json:"metric"`
	Condition   AlertCondition    `json:"condition"`
	Threshold   float64           `json:"threshold"`
	Duration    time.Duration     `json:"duration"`
	Severity    AlertSeverity     `json:"severity"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Enabled     bool              `json:"enabled"`
	Suppression time.Duration     `json:"suppression"`

	// State tracking
	LastEvaluation time.Time `json:"last_evaluation"`
	LastTriggered  time.Time `json:"last_triggered"`
	TriggerCount   int       `json:"trigger_count"`
}

// AlertCondition defines how to evaluate alert conditions
type AlertCondition string

const (
	ConditionGreaterThan    AlertCondition = "gt"
	ConditionLessThan       AlertCondition = "lt"
	ConditionEquals         AlertCondition = "eq"
	ConditionNotEquals      AlertCondition = "ne"
	ConditionGreaterOrEqual AlertCondition = "gte"
	ConditionLessOrEqual    AlertCondition = "lte"
)

// Alert represents an active or historical alert
type Alert struct {
	ID          string            `json:"id"`
	RuleName    string            `json:"rule_name"`
	Severity    AlertSeverity     `json:"severity"`
	Status      AlertStatus       `json:"status"`
	Message     string            `json:"message"`
	Description string            `json:"description"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Value       float64           `json:"value"`
	Threshold   float64           `json:"threshold"`

	StartsAt  time.Time  `json:"starts_at"`
	EndsAt    *time.Time `json:"ends_at,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`

	// Notification tracking
	NotificationsSent map[string]time.Time `json:"notifications_sent"`
	EscalationLevel   int                  `json:"escalation_level"`
}

// AlertStatus represents the current status of an alert
type AlertStatus string

const (
	AlertStatusFiring   AlertStatus = "firing"
	AlertStatusResolved AlertStatus = "resolved"
	AlertStatusSilenced AlertStatus = "silenced"
)

// NotificationChannel defines how to send notifications
type NotificationChannel interface {
	Name() string
	Send(ctx context.Context, alert *Alert) error
	IsEnabled() bool
}

// NewAlertingSystem creates a new alerting system
func NewAlertingSystem(config *AlertingConfig, metrics *PrometheusMetrics) *AlertingSystem {
	if config == nil {
		config = DefaultAlertingConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	as := &AlertingSystem{
		config:       config,
		rules:        make(map[string]*AlertRule),
		channels:     make(map[string]NotificationChannel),
		activeAlerts: make(map[string]*Alert),
		alertHistory: make([]*Alert, 0),
		metrics:      metrics,
		ctx:          ctx,
		cancel:       cancel,
	}

	as.setupNotificationChannels()
	as.setupDefaultRules()

	return as
}

// DefaultAlertingConfig returns default alerting configuration
func DefaultAlertingConfig() *AlertingConfig {
	return &AlertingConfig{
		Enabled:            true,
		EvaluationInterval: 30 * time.Second,
		AlertRetention:     24 * time.Hour,
		MaxAlerts:          1000,
		DefaultSuppression: 5 * time.Minute,
		EscalationTimeout:  15 * time.Minute,
		EmailEnabled:       false,
		SlackEnabled:       false,
		WebhookEnabled:     false,
		WebhookTimeout:     10 * time.Second,
	}
}

// AddRule adds a new alert rule
func (as *AlertingSystem) AddRule(rule *AlertRule) {
	as.mutex.Lock()
	defer as.mutex.Unlock()

	rule.LastEvaluation = time.Now()
	as.rules[rule.Name] = rule
}

// RemoveRule removes an alert rule
func (as *AlertingSystem) RemoveRule(name string) {
	as.mutex.Lock()
	defer as.mutex.Unlock()

	delete(as.rules, name)
}

// Start starts the alerting system
func (as *AlertingSystem) Start() {
	if !as.config.Enabled {
		return
	}

	go as.evaluationLoop()
	go as.cleanupLoop()
}

// Stop stops the alerting system
func (as *AlertingSystem) Stop() {
	as.cancel()
}

// GetActiveAlerts returns all currently active alerts
func (as *AlertingSystem) GetActiveAlerts() []*Alert {
	as.mutex.RLock()
	defer as.mutex.RUnlock()

	alerts := make([]*Alert, 0, len(as.activeAlerts))
	for _, alert := range as.activeAlerts {
		alerts = append(alerts, alert)
	}

	return alerts
}

// GetAlertHistory returns historical alerts
func (as *AlertingSystem) GetAlertHistory(limit int) []*Alert {
	as.mutex.RLock()
	defer as.mutex.RUnlock()

	if limit <= 0 || limit > len(as.alertHistory) {
		limit = len(as.alertHistory)
	}

	// Return most recent alerts first
	result := make([]*Alert, limit)
	start := len(as.alertHistory) - limit
	copy(result, as.alertHistory[start:])

	// Reverse to get newest first
	for i := 0; i < len(result)/2; i++ {
		result[i], result[len(result)-1-i] = result[len(result)-1-i], result[i]
	}

	return result
}

// evaluationLoop runs the main alert evaluation loop
func (as *AlertingSystem) evaluationLoop() {
	ticker := time.NewTicker(as.config.EvaluationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-as.ctx.Done():
			return
		case <-ticker.C:
			as.evaluateRules()
		}
	}
}

// evaluateRules evaluates all alert rules
func (as *AlertingSystem) evaluateRules() {
	as.mutex.RLock()
	rules := make(map[string]*AlertRule)
	for k, v := range as.rules {
		if v.Enabled {
			rules[k] = v
		}
	}
	as.mutex.RUnlock()

	for _, rule := range rules {
		as.evaluateRule(rule)
	}
}

// evaluateRule evaluates a single alert rule
func (as *AlertingSystem) evaluateRule(rule *AlertRule) {
	now := time.Now()

	// Get current metric value
	value, err := as.getMetricValue(rule.Metric, rule.Labels)
	if err != nil {
		fmt.Printf("Error getting metric value for rule %s: %v\n", rule.Name, err)
		return
	}

	// Evaluate condition
	triggered := as.evaluateCondition(value, rule.Condition, rule.Threshold)

	// Update rule state
	as.mutex.Lock()
	rule.LastEvaluation = now
	if triggered {
		rule.TriggerCount++
		rule.LastTriggered = now
	}
	as.mutex.Unlock()

	alertID := as.generateAlertID(rule)

	if triggered {
		// Check if alert already exists
		as.mutex.RLock()
		existingAlert, exists := as.activeAlerts[alertID]
		as.mutex.RUnlock()

		if !exists {
			// Create new alert
			alert := &Alert{
				ID:                alertID,
				RuleName:          rule.Name,
				Severity:          rule.Severity,
				Status:            AlertStatusFiring,
				Message:           as.generateAlertMessage(rule, value),
				Description:       rule.Description,
				Labels:            rule.Labels,
				Annotations:       rule.Annotations,
				Value:             value,
				Threshold:         rule.Threshold,
				StartsAt:          now,
				UpdatedAt:         now,
				NotificationsSent: make(map[string]time.Time),
				EscalationLevel:   0,
			}

			as.mutex.Lock()
			as.activeAlerts[alertID] = alert
			as.mutex.Unlock()

			// Send notifications
			as.sendNotifications(alert)

		} else {
			// Update existing alert
			as.mutex.Lock()
			existingAlert.Value = value
			existingAlert.UpdatedAt = now
			as.mutex.Unlock()
		}
	} else {
		// Check if we need to resolve an existing alert
		as.mutex.RLock()
		existingAlert, exists := as.activeAlerts[alertID]
		as.mutex.RUnlock()

		if exists && existingAlert.Status == AlertStatusFiring {
			// Resolve alert
			as.mutex.Lock()
			existingAlert.Status = AlertStatusResolved
			existingAlert.EndsAt = &now
			existingAlert.UpdatedAt = now

			// Move to history
			as.alertHistory = append(as.alertHistory, existingAlert)
			delete(as.activeAlerts, alertID)
			as.mutex.Unlock()

			// Send resolution notification
			as.sendResolutionNotification(existingAlert)
		}
	}
}

// getMetricValue retrieves the current value of a metric
func (as *AlertingSystem) getMetricValue(metricName string, labels map[string]string) (float64, error) {
	if as.metrics == nil {
		return 0, fmt.Errorf("metrics not available")
	}

	// This is a simplified implementation
	// In a real system, you'd query the actual metric values
	switch metricName {
	case "memory_usage_percent":
		// Simulate memory usage
		return 75.5, nil
	case "query_latency_p95":
		// Simulate query latency
		return 0.25, nil
	case "error_rate":
		// Simulate error rate
		return 0.02, nil
	case "disk_usage_percent":
		// Simulate disk usage
		return 85.0, nil
	default:
		return 0, fmt.Errorf("unknown metric: %s", metricName)
	}
}

// evaluateCondition evaluates an alert condition
func (as *AlertingSystem) evaluateCondition(value float64, condition AlertCondition, threshold float64) bool {
	switch condition {
	case ConditionGreaterThan:
		return value > threshold
	case ConditionLessThan:
		return value < threshold
	case ConditionEquals:
		return value == threshold
	case ConditionNotEquals:
		return value != threshold
	case ConditionGreaterOrEqual:
		return value >= threshold
	case ConditionLessOrEqual:
		return value <= threshold
	default:
		return false
	}
}

// generateAlertID generates a unique ID for an alert
func (as *AlertingSystem) generateAlertID(rule *AlertRule) string {
	return fmt.Sprintf("%s-%d", rule.Name, time.Now().Unix())
}

// generateAlertMessage generates a human-readable alert message
func (as *AlertingSystem) generateAlertMessage(rule *AlertRule, value float64) string {
	return fmt.Sprintf("%s: %s is %.2f (threshold: %.2f)",
		rule.Name, rule.Metric, value, rule.Threshold)
}

// sendNotifications sends notifications for an alert
func (as *AlertingSystem) sendNotifications(alert *Alert) {
	for _, channel := range as.channels {
		if !channel.IsEnabled() {
			continue
		}

		// Check suppression
		if as.isNotificationSuppressed(alert, channel.Name()) {
			continue
		}

		go func(ch NotificationChannel) {
			ctx, cancel := context.WithTimeout(as.ctx, 30*time.Second)
			defer cancel()

			if err := ch.Send(ctx, alert); err != nil {
				fmt.Printf("Failed to send notification via %s: %v\n", ch.Name(), err)
			} else {
				as.mutex.Lock()
				alert.NotificationsSent[ch.Name()] = time.Now()
				as.mutex.Unlock()
			}
		}(channel)
	}
}

// sendResolutionNotification sends a notification when an alert is resolved
func (as *AlertingSystem) sendResolutionNotification(alert *Alert) {
	// Create a copy for resolution notification
	resolvedAlert := *alert
	resolvedAlert.Message = "RESOLVED: " + alert.Message

	as.sendNotifications(&resolvedAlert)
}

// isNotificationSuppressed checks if a notification should be suppressed
func (as *AlertingSystem) isNotificationSuppressed(alert *Alert, channelName string) bool {
	lastSent, exists := alert.NotificationsSent[channelName]
	if !exists {
		return false
	}

	// Get suppression duration from rule or use default
	as.mutex.RLock()
	rule, exists := as.rules[alert.RuleName]
	as.mutex.RUnlock()

	suppression := as.config.DefaultSuppression
	if exists && rule.Suppression > 0 {
		suppression = rule.Suppression
	}

	return time.Since(lastSent) < suppression
}

// setupNotificationChannels sets up notification channels
func (as *AlertingSystem) setupNotificationChannels() {
	// Email channel
	if as.config.EmailEnabled {
		as.channels["email"] = &EmailChannel{
			config: as.config,
		}
	}

	// Slack channel
	if as.config.SlackEnabled {
		as.channels["slack"] = &SlackChannel{
			config: as.config,
		}
	}

	// Webhook channel
	if as.config.WebhookEnabled {
		as.channels["webhook"] = &WebhookChannel{
			config: as.config,
		}
	}

	// Console channel (always enabled for development)
	as.channels["console"] = &ConsoleChannel{}
}

// setupDefaultRules sets up default alert rules
func (as *AlertingSystem) setupDefaultRules() {
	// High memory usage alert
	as.AddRule(&AlertRule{
		Name:        "high_memory_usage",
		Description: "Memory usage is above threshold",
		Metric:      "memory_usage_percent",
		Condition:   ConditionGreaterThan,
		Threshold:   80.0,
		Duration:    5 * time.Minute,
		Severity:    AlertSeverityWarning,
		Enabled:     true,
		Suppression: 10 * time.Minute,
		Labels: map[string]string{
			"component": "system",
			"type":      "resource",
		},
		Annotations: map[string]string{
			"summary":     "High memory usage detected",
			"description": "Memory usage has exceeded 80% for more than 5 minutes",
		},
	})

	// High query latency alert
	as.AddRule(&AlertRule{
		Name:        "high_query_latency",
		Description: "Query latency is above threshold",
		Metric:      "query_latency_p95",
		Condition:   ConditionGreaterThan,
		Threshold:   1.0,
		Duration:    2 * time.Minute,
		Severity:    AlertSeverityCritical,
		Enabled:     true,
		Suppression: 5 * time.Minute,
		Labels: map[string]string{
			"component": "database",
			"type":      "performance",
		},
		Annotations: map[string]string{
			"summary":     "High query latency detected",
			"description": "95th percentile query latency has exceeded 1 second",
		},
	})

	// High error rate alert
	as.AddRule(&AlertRule{
		Name:        "high_error_rate",
		Description: "Error rate is above threshold",
		Metric:      "error_rate",
		Condition:   ConditionGreaterThan,
		Threshold:   0.05,
		Duration:    1 * time.Minute,
		Severity:    AlertSeverityCritical,
		Enabled:     true,
		Suppression: 5 * time.Minute,
		Labels: map[string]string{
			"component": "application",
			"type":      "error",
		},
		Annotations: map[string]string{
			"summary":     "High error rate detected",
			"description": "Error rate has exceeded 5% for more than 1 minute",
		},
	})

	// Disk space alert
	as.AddRule(&AlertRule{
		Name:        "low_disk_space",
		Description: "Disk usage is above threshold",
		Metric:      "disk_usage_percent",
		Condition:   ConditionGreaterThan,
		Threshold:   90.0,
		Duration:    5 * time.Minute,
		Severity:    AlertSeverityCritical,
		Enabled:     true,
		Suppression: 15 * time.Minute,
		Labels: map[string]string{
			"component": "system",
			"type":      "resource",
		},
		Annotations: map[string]string{
			"summary":     "Low disk space detected",
			"description": "Disk usage has exceeded 90%",
		},
	})
}

// cleanupLoop runs periodic cleanup of old alerts
func (as *AlertingSystem) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-as.ctx.Done():
			return
		case <-ticker.C:
			as.cleanupOldAlerts()
		}
	}
}

// cleanupOldAlerts removes old alerts from history
func (as *AlertingSystem) cleanupOldAlerts() {
	as.mutex.Lock()
	defer as.mutex.Unlock()

	cutoff := time.Now().Add(-as.config.AlertRetention)

	// Filter out old alerts
	filtered := make([]*Alert, 0)
	for _, alert := range as.alertHistory {
		if alert.StartsAt.After(cutoff) {
			filtered = append(filtered, alert)
		}
	}

	as.alertHistory = filtered

	// Limit total number of alerts
	if len(as.alertHistory) > as.config.MaxAlerts {
		start := len(as.alertHistory) - as.config.MaxAlerts
		as.alertHistory = as.alertHistory[start:]
	}
}

// Notification channel implementations

// EmailChannel sends notifications via email
type EmailChannel struct {
	config *AlertingConfig
}

func (ec *EmailChannel) Name() string {
	return "email"
}

func (ec *EmailChannel) IsEnabled() bool {
	return ec.config.EmailEnabled && ec.config.SMTPHost != ""
}

func (ec *EmailChannel) Send(ctx context.Context, alert *Alert) error {
	if !ec.IsEnabled() {
		return fmt.Errorf("email channel not enabled")
	}

	subject := fmt.Sprintf("[MantisDB Alert] %s - %s", alert.Severity, alert.RuleName)
	body := fmt.Sprintf(`
Alert: %s
Severity: %s
Message: %s
Description: %s
Value: %.2f
Threshold: %.2f
Started At: %s

Labels: %v
Annotations: %v
`, alert.RuleName, alert.Severity, alert.Message, alert.Description,
		alert.Value, alert.Threshold, alert.StartsAt.Format(time.RFC3339),
		alert.Labels, alert.Annotations)

	// Simple SMTP implementation
	auth := smtp.PlainAuth("", ec.config.SMTPUsername, ec.config.SMTPPassword, ec.config.SMTPHost)

	for _, to := range ec.config.EmailTo {
		msg := fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s", to, subject, body)
		addr := fmt.Sprintf("%s:%d", ec.config.SMTPHost, ec.config.SMTPPort)

		err := smtp.SendMail(addr, auth, ec.config.EmailFrom, []string{to}, []byte(msg))
		if err != nil {
			return fmt.Errorf("failed to send email to %s: %v", to, err)
		}
	}

	return nil
}

// SlackChannel sends notifications to Slack
type SlackChannel struct {
	config *AlertingConfig
}

func (sc *SlackChannel) Name() string {
	return "slack"
}

func (sc *SlackChannel) IsEnabled() bool {
	return sc.config.SlackEnabled && sc.config.SlackWebhookURL != ""
}

func (sc *SlackChannel) Send(ctx context.Context, alert *Alert) error {
	if !sc.IsEnabled() {
		return fmt.Errorf("slack channel not enabled")
	}

	color := "good"
	if alert.Severity == AlertSeverityWarning {
		color = "warning"
	} else if alert.Severity == AlertSeverityCritical {
		color = "danger"
	}

	payload := map[string]interface{}{
		"channel": sc.config.SlackChannel,
		"attachments": []map[string]interface{}{
			{
				"color":     color,
				"title":     fmt.Sprintf("MantisDB Alert: %s", alert.RuleName),
				"text":      alert.Message,
				"timestamp": alert.StartsAt.Unix(),
				"fields": []map[string]interface{}{
					{
						"title": "Severity",
						"value": string(alert.Severity),
						"short": true,
					},
					{
						"title": "Value",
						"value": fmt.Sprintf("%.2f", alert.Value),
						"short": true,
					},
					{
						"title": "Threshold",
						"value": fmt.Sprintf("%.2f", alert.Threshold),
						"short": true,
					},
				},
			},
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal slack payload: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", sc.config.SlackWebhookURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create slack request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send slack notification: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// WebhookChannel sends notifications to a generic webhook
type WebhookChannel struct {
	config *AlertingConfig
}

func (wc *WebhookChannel) Name() string {
	return "webhook"
}

func (wc *WebhookChannel) IsEnabled() bool {
	return wc.config.WebhookEnabled && wc.config.WebhookURL != ""
}

func (wc *WebhookChannel) Send(ctx context.Context, alert *Alert) error {
	if !wc.IsEnabled() {
		return fmt.Errorf("webhook channel not enabled")
	}

	jsonPayload, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", wc.config.WebhookURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: wc.config.WebhookTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook notification: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// ConsoleChannel prints notifications to console
type ConsoleChannel struct{}

func (cc *ConsoleChannel) Name() string {
	return "console"
}

func (cc *ConsoleChannel) IsEnabled() bool {
	return true
}

func (cc *ConsoleChannel) Send(ctx context.Context, alert *Alert) error {
	emoji := "ℹ️"
	if alert.Severity == AlertSeverityWarning {
		emoji = "⚠️"
	} else if alert.Severity == AlertSeverityCritical {
		emoji = "🚨"
	}

	fmt.Printf("\n%s ALERT [%s] %s\n", emoji, alert.StartsAt.Format("15:04:05"), alert.Message)
	fmt.Printf("   Rule: %s\n", alert.RuleName)
	fmt.Printf("   Severity: %s\n", alert.Severity)
	fmt.Printf("   Value: %.2f (threshold: %.2f)\n", alert.Value, alert.Threshold)

	if len(alert.Labels) > 0 {
		fmt.Printf("   Labels: %v\n", alert.Labels)
	}

	return nil
}

// LoadAlertingConfigFromEnv loads alerting configuration from environment variables
func LoadAlertingConfigFromEnv() *AlertingConfig {
	config := DefaultAlertingConfig()

	// Email configuration
	if host := os.Getenv("MANTIS_SMTP_HOST"); host != "" {
		config.EmailEnabled = true
		config.SMTPHost = host
	}

	if port := os.Getenv("MANTIS_SMTP_PORT"); port != "" {
		if p, err := time.ParseDuration(port); err == nil {
			config.SMTPPort = int(p)
		}
	}

	if username := os.Getenv("MANTIS_SMTP_USERNAME"); username != "" {
		config.SMTPUsername = username
	}

	if password := os.Getenv("MANTIS_SMTP_PASSWORD"); password != "" {
		config.SMTPPassword = password
	}

	if from := os.Getenv("MANTIS_EMAIL_FROM"); from != "" {
		config.EmailFrom = from
	}

	// Slack configuration
	if webhookURL := os.Getenv("MANTIS_SLACK_WEBHOOK_URL"); webhookURL != "" {
		config.SlackEnabled = true
		config.SlackWebhookURL = webhookURL
	}

	if channel := os.Getenv("MANTIS_SLACK_CHANNEL"); channel != "" {
		config.SlackChannel = channel
	}

	// Webhook configuration
	if webhookURL := os.Getenv("MANTIS_WEBHOOK_URL"); webhookURL != "" {
		config.WebhookEnabled = true
		config.WebhookURL = webhookURL
	}

	return config
}
