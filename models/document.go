package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// Document represents a document in the document store
type Document struct {
	ID         string                 `json:"id"`
	Collection string                 `json:"collection"`
	Data       map[string]interface{} `json:"data"`
	Metadata   DocumentMetadata       `json:"metadata"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
	Version    int64                  `json:"version"`
}

// DocumentMetadata holds metadata about the document
type DocumentMetadata struct {
	ContentType string            `json:"content_type"`
	Size        int64             `json:"size"`
	Checksum    string            `json:"checksum"`
	Tags        []string          `json:"tags"`
	Properties  map[string]string `json:"properties"`
}

// DocumentQuery represents a query for documents
type DocumentQuery struct {
	Collection string                 `json:"collection"`
	Filter     map[string]interface{} `json:"filter"`
	Sort       []SortField            `json:"sort"`
	Limit      int                    `json:"limit"`
	Offset     int                    `json:"offset"`
	Fields     []string               `json:"fields"`
}

// SortField represents a field to sort by
type SortField struct {
	Field string `json:"field"`
	Desc  bool   `json:"desc"`
}

// DocumentResult represents the result of a document operation
type DocumentResult struct {
	Documents  []*Document `json:"documents"`
	TotalCount int64       `json:"total_count"`
	HasMore    bool        `json:"has_more"`
	NextOffset int         `json:"next_offset"`
}

// DocumentIndex represents an index on document fields
type DocumentIndex struct {
	Name       string       `json:"name"`
	Collection string       `json:"collection"`
	Fields     []IndexField `json:"fields"`
	Unique     bool         `json:"unique"`
	Sparse     bool         `json:"sparse"`
	CreatedAt  time.Time    `json:"created_at"`
}

// IndexField represents a field in an index
type IndexField struct {
	Field string `json:"field"`
	Order int    `json:"order"` // 1 for ascending, -1 for descending
}

// NewDocument creates a new document
func NewDocument(id, collection string, data map[string]interface{}) *Document {
	now := time.Now()
	return &Document{
		ID:         id,
		Collection: collection,
		Data:       data,
		Metadata: DocumentMetadata{
			ContentType: "application/json",
			Tags:        make([]string, 0),
			Properties:  make(map[string]string),
		},
		CreatedAt: now,
		UpdatedAt: now,
		Version:   1,
	}
}

// ToJSON converts the document to JSON
func (d *Document) ToJSON() ([]byte, error) {
	return json.Marshal(d)
}

// FromJSON creates a document from JSON
func FromJSON(data []byte) (*Document, error) {
	var doc Document
	err := json.Unmarshal(data, &doc)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// GetField gets a field value from the document data
func (d *Document) GetField(field string) (interface{}, bool) {
	value, exists := d.Data[field]
	return value, exists
}

// SetField sets a field value in the document data
func (d *Document) SetField(field string, value interface{}) {
	if d.Data == nil {
		d.Data = make(map[string]interface{})
	}
	d.Data[field] = value
	d.UpdatedAt = time.Now()
	d.Version++
}

// RemoveField removes a field from the document data
func (d *Document) RemoveField(field string) {
	if d.Data != nil {
		delete(d.Data, field)
		d.UpdatedAt = time.Now()
		d.Version++
	}
}

// HasField checks if a field exists in the document data
func (d *Document) HasField(field string) bool {
	_, exists := d.Data[field]
	return exists
}

// AddTag adds a tag to the document metadata
func (d *Document) AddTag(tag string) {
	for _, existingTag := range d.Metadata.Tags {
		if existingTag == tag {
			return // Tag already exists
		}
	}
	d.Metadata.Tags = append(d.Metadata.Tags, tag)
	d.UpdatedAt = time.Now()
}

// RemoveTag removes a tag from the document metadata
func (d *Document) RemoveTag(tag string) {
	for i, existingTag := range d.Metadata.Tags {
		if existingTag == tag {
			d.Metadata.Tags = append(d.Metadata.Tags[:i], d.Metadata.Tags[i+1:]...)
			d.UpdatedAt = time.Now()
			break
		}
	}
}

// HasTag checks if the document has a specific tag
func (d *Document) HasTag(tag string) bool {
	for _, existingTag := range d.Metadata.Tags {
		if existingTag == tag {
			return true
		}
	}
	return false
}

// SetProperty sets a metadata property
func (d *Document) SetProperty(key, value string) {
	if d.Metadata.Properties == nil {
		d.Metadata.Properties = make(map[string]string)
	}
	d.Metadata.Properties[key] = value
	d.UpdatedAt = time.Now()
}

// GetProperty gets a metadata property
func (d *Document) GetProperty(key string) (string, bool) {
	if d.Metadata.Properties == nil {
		return "", false
	}
	value, exists := d.Metadata.Properties[key]
	return value, exists
}

// Clone creates a deep copy of the document
func (d *Document) Clone() *Document {
	// Marshal and unmarshal to create a deep copy
	data, err := d.ToJSON()
	if err != nil {
		return nil
	}

	clone, err := FromJSON(data)
	if err != nil {
		return nil
	}

	return clone
}

// Validate validates the document structure
func (d *Document) Validate() error {
	if d.ID == "" {
		return NewValidationError("document ID cannot be empty")
	}

	if d.Collection == "" {
		return NewValidationError("document collection cannot be empty")
	}

	if d.Data == nil {
		return NewValidationError("document data cannot be nil")
	}

	return nil
}

// CalculateSize calculates the approximate size of the document in bytes
func (d *Document) CalculateSize() int64 {
	data, err := d.ToJSON()
	if err != nil {
		return 0
	}
	return int64(len(data))
}

// UpdateChecksum updates the document checksum based on its data
func (d *Document) UpdateChecksum() error {
	data, err := json.Marshal(d.Data)
	if err != nil {
		return err
	}

	// Simple checksum calculation (in reality, use a proper hash function)
	checksum := calculateChecksum(data)
	d.Metadata.Checksum = checksum
	d.Metadata.Size = int64(len(data))

	return nil
}

// MatchesQuery checks if the document matches a query
func (d *Document) MatchesQuery(query *DocumentQuery) bool {
	// Check collection
	if query.Collection != "" && d.Collection != query.Collection {
		return false
	}

	// Check filters
	for field, expectedValue := range query.Filter {
		actualValue, exists := d.GetField(field)
		if !exists {
			return false
		}

		if !valuesEqual(actualValue, expectedValue) {
			return false
		}
	}

	return true
}

// DocumentStore interface defines operations for document storage
type DocumentStore interface {
	// Create a new document
	Create(doc *Document) error

	// Get a document by ID
	Get(collection, id string) (*Document, error)

	// Update an existing document
	Update(doc *Document) error

	// Delete a document
	Delete(collection, id string) error

	// Query documents
	Query(query *DocumentQuery) (*DocumentResult, error)

	// Create an index
	CreateIndex(index *DocumentIndex) error

	// Drop an index
	DropIndex(collection, indexName string) error

	// List indexes for a collection
	ListIndexes(collection string) ([]*DocumentIndex, error)
}

// Helper functions

func calculateChecksum(data []byte) string {
	// Simplified checksum - in reality, use SHA256 or similar
	sum := 0
	for _, b := range data {
		sum += int(b)
	}
	return fmt.Sprintf("%x", sum)
}

func valuesEqual(a, b interface{}) bool {
	// Simplified comparison - in reality, handle different types properly
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// ValidationError represents a document validation error
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

func NewValidationError(message string) *ValidationError {
	return &ValidationError{Message: message}
}

// DocumentEvent represents an event that occurred on a document
type DocumentEvent struct {
	Type       EventType `json:"type"`
	Collection string    `json:"collection"`
	DocumentID string    `json:"document_id"`
	Timestamp  time.Time `json:"timestamp"`
	Data       *Document `json:"data,omitempty"`
}

// EventType represents the type of document event
type EventType string

const (
	EventTypeCreated EventType = "created"
	EventTypeUpdated EventType = "updated"
	EventTypeDeleted EventType = "deleted"
)

// DocumentEventHandler handles document events
type DocumentEventHandler interface {
	HandleEvent(event *DocumentEvent) error
}
