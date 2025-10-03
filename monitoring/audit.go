package monitoring

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// AuditEventType represents the type of audit event
type AuditEventType string

const (
	AuditEventTypeDataAccess     AuditEventType = "data_access"
	AuditEventTypeDataModify     AuditEventType = "data_modify"
	AuditEventTypeTransaction    AuditEventType = "transaction"
	AuditEventTypeRecovery       AuditEventType = "recovery"
	AuditEventTypeConfiguration  AuditEventType = "configuration"
	AuditEventTypeAuthentication AuditEventType = "authentication"
	AuditEventTypeAuthorization  AuditEventType = "authorization"
	AuditEventTypeSystem         AuditEventType = "system"
)

// AuditEvent represents a single audit event
type AuditEvent struct {
	ID         string                 `json:"id"`
	Timestamp  time.Time              `json:"timestamp"`
	EventType  AuditEventType         `json:"event_type"`
	Component  string                 `json:"component"`
	Operation  string                 `json:"operation"`
	UserID     string                 `json:"user_id,omitempty"`
	SessionID  string                 `json:"session_id,omitempty"`
	Resource   string                 `json:"resource,omitempty"`
	Action     string                 `json:"action"`
	Result     string                 `json:"result"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Checksum   string                 `json:"checksum"`
	PreviousID string                 `json:"previous_id,omitempty"`
}

// AuditTrail manages audit events and ensures integrity
type AuditTrail struct {
	events    []AuditEvent
	mutex     sync.RWMutex
	logger    *Logger
	storage   AuditStorage
	lastEvent *AuditEvent

	// Configuration
	maxEvents   int
	retention   time.Duration
	enableChain bool
}

// AuditStorage interface for persisting audit events
type AuditStorage interface {
	Store(event AuditEvent) error
	Retrieve(id string) (AuditEvent, error)
	Query(filter AuditFilter) ([]AuditEvent, error)
	Purge(before time.Time) error
}

// AuditFilter represents criteria for querying audit events
type AuditFilter struct {
	EventType AuditEventType `json:"event_type,omitempty"`
	Component string         `json:"component,omitempty"`
	UserID    string         `json:"user_id,omitempty"`
	Resource  string         `json:"resource,omitempty"`
	StartTime time.Time      `json:"start_time,omitempty"`
	EndTime   time.Time      `json:"end_time,omitempty"`
	Limit     int            `json:"limit,omitempty"`
}

// NewAuditTrail creates a new audit trail
func NewAuditTrail(storage AuditStorage) *AuditTrail {
	logger := NewLogger(LogLevelInfo)

	// Set up file rotation for audit logs
	if rotatingWriter, err := NewFileRotatingWriter("logs/mantisdb-audit.log", 100*1024*1024, 50); err == nil {
		logger.AddOutput(rotatingWriter)
	}

	return &AuditTrail{
		events:      make([]AuditEvent, 0),
		logger:      logger,
		storage:     storage,
		maxEvents:   10000,
		retention:   365 * 24 * time.Hour, // 1 year
		enableChain: true,
	}
}

// LogEvent logs an audit event
func (at *AuditTrail) LogEvent(eventType AuditEventType, component, operation, userID, sessionID, resource, action, result string, details map[string]interface{}) {
	event := AuditEvent{
		ID:        at.generateEventID(),
		Timestamp: time.Now(),
		EventType: eventType,
		Component: component,
		Operation: operation,
		UserID:    userID,
		SessionID: sessionID,
		Resource:  resource,
		Action:    action,
		Result:    result,
		Details:   details,
	}

	// Add previous event ID for chaining if enabled
	if at.enableChain && at.lastEvent != nil {
		event.PreviousID = at.lastEvent.ID
	}

	// Calculate checksum for integrity
	event.Checksum = at.calculateChecksum(event)

	// Store the event
	at.mutex.Lock()
	at.events = append(at.events, event)
	at.lastEvent = &event

	// Cleanup old events if needed
	if len(at.events) > at.maxEvents {
		at.events = at.events[len(at.events)-at.maxEvents:]
	}
	at.mutex.Unlock()

	// Persist to storage
	if at.storage != nil {
		if err := at.storage.Store(event); err != nil {
			at.logger.Error("audit", "store", "Failed to store audit event", map[string]interface{}{
				"event_id": event.ID,
				"error":    err.Error(),
			})
		}
	}

	// Log to audit logger
	at.logger.Info("audit", operation, fmt.Sprintf("Audit event: %s", action), map[string]interface{}{
		"event_id":   event.ID,
		"event_type": string(eventType),
		"component":  component,
		"user_id":    userID,
		"session_id": sessionID,
		"resource":   resource,
		"result":     result,
		"checksum":   event.Checksum,
	})
}

// LogDataAccess logs a data access event
func (at *AuditTrail) LogDataAccess(userID, sessionID, resource, operation string, success bool, details map[string]interface{}) {
	result := "success"
	if !success {
		result = "failure"
	}

	at.LogEvent(AuditEventTypeDataAccess, "storage", operation, userID, sessionID, resource, "read", result, details)
}

// LogDataModification logs a data modification event
func (at *AuditTrail) LogDataModification(userID, sessionID, resource, operation string, success bool, details map[string]interface{}) {
	result := "success"
	if !success {
		result = "failure"
	}

	at.LogEvent(AuditEventTypeDataModify, "storage", operation, userID, sessionID, resource, "modify", result, details)
}

// LogTransaction logs a transaction event
func (at *AuditTrail) LogTransaction(userID, sessionID string, txnID uint64, operation string, success bool, details map[string]interface{}) {
	result := "success"
	if !success {
		result = "failure"
	}

	if details == nil {
		details = make(map[string]interface{})
	}
	details["txn_id"] = txnID

	at.LogEvent(AuditEventTypeTransaction, "transaction", operation, userID, sessionID, fmt.Sprintf("txn:%d", txnID), operation, result, details)
}

// LogRecovery logs a recovery event
func (at *AuditTrail) LogRecovery(operation string, success bool, details map[string]interface{}) {
	result := "success"
	if !success {
		result = "failure"
	}

	at.LogEvent(AuditEventTypeRecovery, "recovery", operation, "", "", "", operation, result, details)
}

// LogConfigurationChange logs a configuration change event
func (at *AuditTrail) LogConfigurationChange(userID, sessionID, setting, oldValue, newValue string, details map[string]interface{}) {
	if details == nil {
		details = make(map[string]interface{})
	}
	details["old_value"] = oldValue
	details["new_value"] = newValue

	at.LogEvent(AuditEventTypeConfiguration, "config", "change", userID, sessionID, setting, "modify", "success", details)
}

// LogSystemEvent logs a system event
func (at *AuditTrail) LogSystemEvent(event string, details map[string]interface{}) {
	at.LogEvent(AuditEventTypeSystem, "system", event, "", "", "", event, "success", details)
}

// GetEvents returns audit events matching the filter
func (at *AuditTrail) GetEvents(filter AuditFilter) ([]AuditEvent, error) {
	if at.storage != nil {
		return at.storage.Query(filter)
	}

	// Fallback to in-memory events
	at.mutex.RLock()
	defer at.mutex.RUnlock()

	var filtered []AuditEvent
	for _, event := range at.events {
		if at.matchesFilter(event, filter) {
			filtered = append(filtered, event)
		}
	}

	// Apply limit
	if filter.Limit > 0 && len(filtered) > filter.Limit {
		filtered = filtered[:filter.Limit]
	}

	return filtered, nil
}

// VerifyIntegrity verifies the integrity of the audit trail
func (at *AuditTrail) VerifyIntegrity() (bool, []string) {
	at.mutex.RLock()
	defer at.mutex.RUnlock()

	var issues []string

	// Verify checksums
	for _, event := range at.events {
		expectedChecksum := at.calculateChecksum(event)
		if event.Checksum != expectedChecksum {
			issues = append(issues, fmt.Sprintf("Checksum mismatch for event %s", event.ID))
		}
	}

	// Verify chain integrity if enabled
	if at.enableChain && len(at.events) > 1 {
		for i := 1; i < len(at.events); i++ {
			if at.events[i].PreviousID != at.events[i-1].ID {
				issues = append(issues, fmt.Sprintf("Chain break at event %s", at.events[i].ID))
			}
		}
	}

	return len(issues) == 0, issues
}

// ExportEvents exports audit events in JSON format
func (at *AuditTrail) ExportEvents(filter AuditFilter) ([]byte, error) {
	events, err := at.GetEvents(filter)
	if err != nil {
		return nil, err
	}

	export := struct {
		ExportTime time.Time    `json:"export_time"`
		Filter     AuditFilter  `json:"filter"`
		Events     []AuditEvent `json:"events"`
		Count      int          `json:"count"`
	}{
		ExportTime: time.Now(),
		Filter:     filter,
		Events:     events,
		Count:      len(events),
	}

	return json.MarshalIndent(export, "", "  ")
}

// PurgeOldEvents removes old audit events based on retention policy
func (at *AuditTrail) PurgeOldEvents() error {
	cutoff := time.Now().Add(-at.retention)

	if at.storage != nil {
		if err := at.storage.Purge(cutoff); err != nil {
			return err
		}
	}

	// Purge from memory
	at.mutex.Lock()
	defer at.mutex.Unlock()

	var kept []AuditEvent
	for _, event := range at.events {
		if event.Timestamp.After(cutoff) {
			kept = append(kept, event)
		}
	}

	purged := len(at.events) - len(kept)
	at.events = kept

	if purged > 0 {
		at.logger.Info("audit", "purge", fmt.Sprintf("Purged %d old audit events", purged), map[string]interface{}{
			"purged_count": purged,
			"cutoff_time":  cutoff,
		})
	}

	return nil
}

// generateEventID generates a unique event ID
func (at *AuditTrail) generateEventID() string {
	return fmt.Sprintf("audit-%d-%d", time.Now().UnixNano(), len(at.events))
}

// calculateChecksum calculates a checksum for an audit event
func (at *AuditTrail) calculateChecksum(event AuditEvent) string {
	// Create a copy without the checksum field
	eventCopy := event
	eventCopy.Checksum = ""

	// Serialize to JSON
	data, err := json.Marshal(eventCopy)
	if err != nil {
		return ""
	}

	// Calculate SHA-256 hash
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// matchesFilter checks if an event matches the given filter
func (at *AuditTrail) matchesFilter(event AuditEvent, filter AuditFilter) bool {
	if filter.EventType != "" && event.EventType != filter.EventType {
		return false
	}

	if filter.Component != "" && event.Component != filter.Component {
		return false
	}

	if filter.UserID != "" && event.UserID != filter.UserID {
		return false
	}

	if filter.Resource != "" && event.Resource != filter.Resource {
		return false
	}

	if !filter.StartTime.IsZero() && event.Timestamp.Before(filter.StartTime) {
		return false
	}

	if !filter.EndTime.IsZero() && event.Timestamp.After(filter.EndTime) {
		return false
	}

	return true
}

// InMemoryAuditStorage provides in-memory audit storage
type InMemoryAuditStorage struct {
	events map[string]AuditEvent
	mutex  sync.RWMutex
}

// NewInMemoryAuditStorage creates a new in-memory audit storage
func NewInMemoryAuditStorage() *InMemoryAuditStorage {
	return &InMemoryAuditStorage{
		events: make(map[string]AuditEvent),
	}
}

// Store stores an audit event
func (s *InMemoryAuditStorage) Store(event AuditEvent) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.events[event.ID] = event
	return nil
}

// Retrieve retrieves an audit event by ID
func (s *InMemoryAuditStorage) Retrieve(id string) (AuditEvent, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if event, exists := s.events[id]; exists {
		return event, nil
	}

	return AuditEvent{}, fmt.Errorf("audit event not found: %s", id)
}

// Query queries audit events based on filter
func (s *InMemoryAuditStorage) Query(filter AuditFilter) ([]AuditEvent, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var results []AuditEvent
	for _, event := range s.events {
		if s.matchesFilter(event, filter) {
			results = append(results, event)
		}
	}

	// Apply limit
	if filter.Limit > 0 && len(results) > filter.Limit {
		results = results[:filter.Limit]
	}

	return results, nil
}

// Purge removes events older than the specified time
func (s *InMemoryAuditStorage) Purge(before time.Time) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for id, event := range s.events {
		if event.Timestamp.Before(before) {
			delete(s.events, id)
		}
	}

	return nil
}

// matchesFilter checks if an event matches the filter (same logic as AuditTrail)
func (s *InMemoryAuditStorage) matchesFilter(event AuditEvent, filter AuditFilter) bool {
	if filter.EventType != "" && event.EventType != filter.EventType {
		return false
	}

	if filter.Component != "" && event.Component != filter.Component {
		return false
	}

	if filter.UserID != "" && event.UserID != filter.UserID {
		return false
	}

	if filter.Resource != "" && event.Resource != filter.Resource {
		return false
	}

	if !filter.StartTime.IsZero() && event.Timestamp.Before(filter.StartTime) {
		return false
	}

	if !filter.EndTime.IsZero() && event.Timestamp.After(filter.EndTime) {
		return false
	}

	return true
}
