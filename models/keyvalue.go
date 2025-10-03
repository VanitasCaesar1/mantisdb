package models

import (
	"encoding/json"
	"time"
)

// KeyValue represents a key-value pair in the store
type KeyValue struct {
	Key       string     `json:"key"`
	Value     []byte     `json:"value"`
	TTL       int64      `json:"ttl,omitempty"`        // Time to live in seconds
	ExpiresAt time.Time  `json:"expires_at,omitempty"` // Absolute expiration time
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Version   int64      `json:"version"`
	Metadata  KVMetadata `json:"metadata"`
}

// KVMetadata holds metadata for key-value pairs
type KVMetadata struct {
	ContentType string            `json:"content_type"`
	Encoding    string            `json:"encoding"`
	Size        int64             `json:"size"`
	Checksum    string            `json:"checksum"`
	Tags        []string          `json:"tags"`
	Properties  map[string]string `json:"properties"`
}

// KVQuery represents a query for key-value pairs
type KVQuery struct {
	KeyPrefix      string            `json:"key_prefix"`
	KeyPattern     string            `json:"key_pattern"`
	Tags           []string          `json:"tags"`
	Properties     map[string]string `json:"properties"`
	Limit          int               `json:"limit"`
	Offset         int               `json:"offset"`
	IncludeExpired bool              `json:"include_expired"`
}

// KVResult represents the result of a key-value operation
type KVResult struct {
	KeyValues  []*KeyValue `json:"key_values"`
	TotalCount int64       `json:"total_count"`
	HasMore    bool        `json:"has_more"`
	NextOffset int         `json:"next_offset"`
}

// KVBatch represents a batch of key-value operations
type KVBatch struct {
	Operations []*KVOperation `json:"operations"`
	Atomic     bool           `json:"atomic"`
}

// KVOperation represents a single operation in a batch
type KVOperation struct {
	Type  OperationType `json:"type"`
	Key   string        `json:"key"`
	Value []byte        `json:"value,omitempty"`
	TTL   int64         `json:"ttl,omitempty"`
}

// OperationType represents the type of key-value operation
type OperationType string

const (
	OpTypeSet    OperationType = "set"
	OpTypeGet    OperationType = "get"
	OpTypeDelete OperationType = "delete"
	OpTypeExists OperationType = "exists"
)

// NewKeyValue creates a new key-value pair
func NewKeyValue(key string, value []byte) *KeyValue {
	now := time.Now()
	return &KeyValue{
		Key:       key,
		Value:     value,
		CreatedAt: now,
		UpdatedAt: now,
		Version:   1,
		Metadata: KVMetadata{
			ContentType: "application/octet-stream",
			Encoding:    "binary",
			Size:        int64(len(value)),
			Tags:        make([]string, 0),
			Properties:  make(map[string]string),
		},
	}
}

// NewKeyValueWithTTL creates a new key-value pair with TTL
func NewKeyValueWithTTL(key string, value []byte, ttl int64) *KeyValue {
	kv := NewKeyValue(key, value)
	kv.TTL = ttl
	kv.ExpiresAt = time.Now().Add(time.Duration(ttl) * time.Second)
	return kv
}

// IsExpired checks if the key-value pair has expired
func (kv *KeyValue) IsExpired() bool {
	if kv.TTL == 0 {
		return false
	}
	return time.Now().After(kv.ExpiresAt)
}

// TimeToExpiry returns the time until expiry
func (kv *KeyValue) TimeToExpiry() time.Duration {
	if kv.TTL == 0 {
		return 0
	}
	return time.Until(kv.ExpiresAt)
}

// SetTTL sets the time to live for the key-value pair
func (kv *KeyValue) SetTTL(ttl int64) {
	kv.TTL = ttl
	if ttl > 0 {
		kv.ExpiresAt = time.Now().Add(time.Duration(ttl) * time.Second)
	} else {
		kv.ExpiresAt = time.Time{}
	}
	kv.UpdatedAt = time.Now()
}

// UpdateValue updates the value and increments version
func (kv *KeyValue) UpdateValue(value []byte) {
	kv.Value = value
	kv.UpdatedAt = time.Now()
	kv.Version++
	kv.Metadata.Size = int64(len(value))
	kv.updateChecksum()
}

// GetString returns the value as a string
func (kv *KeyValue) GetString() string {
	return string(kv.Value)
}

// SetString sets the value from a string
func (kv *KeyValue) SetString(value string) {
	kv.UpdateValue([]byte(value))
	kv.Metadata.ContentType = "text/plain"
	kv.Metadata.Encoding = "utf-8"
}

// GetJSON unmarshals the value as JSON into the provided interface
func (kv *KeyValue) GetJSON(v interface{}) error {
	return json.Unmarshal(kv.Value, v)
}

// SetJSON marshals the provided interface as JSON and sets it as the value
func (kv *KeyValue) SetJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	kv.UpdateValue(data)
	kv.Metadata.ContentType = "application/json"
	kv.Metadata.Encoding = "utf-8"
	return nil
}

// AddTag adds a tag to the key-value metadata
func (kv *KeyValue) AddTag(tag string) {
	for _, existingTag := range kv.Metadata.Tags {
		if existingTag == tag {
			return // Tag already exists
		}
	}
	kv.Metadata.Tags = append(kv.Metadata.Tags, tag)
	kv.UpdatedAt = time.Now()
}

// RemoveTag removes a tag from the key-value metadata
func (kv *KeyValue) RemoveTag(tag string) {
	for i, existingTag := range kv.Metadata.Tags {
		if existingTag == tag {
			kv.Metadata.Tags = append(kv.Metadata.Tags[:i], kv.Metadata.Tags[i+1:]...)
			kv.UpdatedAt = time.Now()
			break
		}
	}
}

// HasTag checks if the key-value has a specific tag
func (kv *KeyValue) HasTag(tag string) bool {
	for _, existingTag := range kv.Metadata.Tags {
		if existingTag == tag {
			return true
		}
	}
	return false
}

// SetProperty sets a metadata property
func (kv *KeyValue) SetProperty(key, value string) {
	if kv.Metadata.Properties == nil {
		kv.Metadata.Properties = make(map[string]string)
	}
	kv.Metadata.Properties[key] = value
	kv.UpdatedAt = time.Now()
}

// GetProperty gets a metadata property
func (kv *KeyValue) GetProperty(key string) (string, bool) {
	if kv.Metadata.Properties == nil {
		return "", false
	}
	value, exists := kv.Metadata.Properties[key]
	return value, exists
}

// ToJSON converts the key-value pair to JSON
func (kv *KeyValue) ToJSON() ([]byte, error) {
	return json.Marshal(kv)
}

// FromJSON creates a key-value pair from JSON
func KVFromJSON(data []byte) (*KeyValue, error) {
	var kv KeyValue
	err := json.Unmarshal(data, &kv)
	if err != nil {
		return nil, err
	}
	return &kv, nil
}

// Clone creates a deep copy of the key-value pair
func (kv *KeyValue) Clone() *KeyValue {
	data, err := kv.ToJSON()
	if err != nil {
		return nil
	}

	clone, err := KVFromJSON(data)
	if err != nil {
		return nil
	}

	return clone
}

// Validate validates the key-value pair
func (kv *KeyValue) Validate() error {
	if kv.Key == "" {
		return NewValidationError("key cannot be empty")
	}

	if kv.Value == nil {
		return NewValidationError("value cannot be nil")
	}

	if kv.TTL < 0 {
		return NewValidationError("TTL cannot be negative")
	}

	return nil
}

// MatchesQuery checks if the key-value pair matches a query
func (kv *KeyValue) MatchesQuery(query *KVQuery) bool {
	// Check if expired (unless including expired)
	if !query.IncludeExpired && kv.IsExpired() {
		return false
	}

	// Check key prefix
	if query.KeyPrefix != "" {
		if len(kv.Key) < len(query.KeyPrefix) || kv.Key[:len(query.KeyPrefix)] != query.KeyPrefix {
			return false
		}
	}

	// Check key pattern (simplified pattern matching)
	if query.KeyPattern != "" {
		matched, err := matchPattern(kv.Key, query.KeyPattern)
		if err != nil || !matched {
			return false
		}
	}

	// Check tags
	for _, requiredTag := range query.Tags {
		if !kv.HasTag(requiredTag) {
			return false
		}
	}

	// Check properties
	for key, expectedValue := range query.Properties {
		actualValue, exists := kv.GetProperty(key)
		if !exists || actualValue != expectedValue {
			return false
		}
	}

	return true
}

// updateChecksum updates the checksum based on the value
func (kv *KeyValue) updateChecksum() {
	kv.Metadata.Checksum = calculateChecksum(kv.Value)
}

// KeyValueStore interface defines operations for key-value storage
type KeyValueStore interface {
	// Set a key-value pair
	Set(key string, value []byte) error

	// Set a key-value pair with TTL
	SetWithTTL(key string, value []byte, ttl int64) error

	// Get a value by key
	Get(key string) (*KeyValue, error)

	// Delete a key
	Delete(key string) error

	// Check if a key exists
	Exists(key string) (bool, error)

	// Query key-value pairs
	Query(query *KVQuery) (*KVResult, error)

	// Execute a batch of operations
	Batch(batch *KVBatch) error

	// Get all keys matching a prefix
	Keys(prefix string) ([]string, error)

	// Get the size of the store
	Size() (int64, error)

	// Clear all key-value pairs
	Clear() error
}

// Helper functions

func matchPattern(key, pattern string) (bool, error) {
	// Simplified pattern matching - in reality, use regex or glob patterns
	if pattern == "*" {
		return true, nil
	}

	// Simple wildcard matching
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(key) >= len(prefix) && key[:len(prefix)] == prefix, nil
	}

	return key == pattern, nil
}

// KVEvent represents an event that occurred on a key-value pair
type KVEvent struct {
	Type      EventType `json:"type"`
	Key       string    `json:"key"`
	Timestamp time.Time `json:"timestamp"`
	Data      *KeyValue `json:"data,omitempty"`
}

// KVEventHandler handles key-value events
type KVEventHandler interface {
	HandleEvent(event *KVEvent) error
}

// KVStats represents statistics about the key-value store
type KVStats struct {
	TotalKeys        int64   `json:"total_keys"`
	TotalSize        int64   `json:"total_size"`
	ExpiredKeys      int64   `json:"expired_keys"`
	AverageKeySize   float64 `json:"average_key_size"`
	AverageValueSize float64 `json:"average_value_size"`
	HitRate          float64 `json:"hit_rate"`
	MissRate         float64 `json:"miss_rate"`
}

// KVTransaction represents a transaction for key-value operations
type KVTransaction interface {
	// Set a key-value pair in the transaction
	Set(key string, value []byte) error

	// Get a value by key within the transaction
	Get(key string) (*KeyValue, error)

	// Delete a key in the transaction
	Delete(key string) error

	// Commit the transaction
	Commit() error

	// Rollback the transaction
	Rollback() error
}
