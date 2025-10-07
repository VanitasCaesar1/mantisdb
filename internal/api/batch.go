// Package api provides batch processing functionality
package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	"mantisDB/models"
	"mantisDB/store"
)

// BatchOperation represents a single operation in a batch request
type BatchOperation struct {
	Type  string      `json:"type"` // "set", "get", "delete"
	Key   string      `json:"key"`
	Value interface{} `json:"value,omitempty"` // Only for "set" operations
	TTL   int         `json:"ttl,omitempty"`   // TTL in seconds, only for "set" operations
}

// BatchRequest represents a batch operation request
type BatchRequest struct {
	Operations []BatchOperation `json:"operations"`
	Atomic     bool             `json:"atomic,omitempty"` // Whether to execute atomically
}

// BatchResult represents the result of a single batch operation
type BatchResult struct {
	Key     string      `json:"key"`
	Value   interface{} `json:"value,omitempty"` // Only for "get" operations
	Success bool        `json:"success"`
	Error   string      `json:"error,omitempty"`
}

// BatchResponse represents the response to a batch operation request
type BatchResponse struct {
	Results []BatchResult `json:"results"`
	Success bool          `json:"success"`
	Errors  []string      `json:"errors,omitempty"`
}

// ValidateBatchRequest validates a batch request and returns validation errors
func ValidateBatchRequest(req *BatchRequest) []string {
	var errors []string

	if req == nil {
		return []string{"batch request cannot be nil"}
	}

	if len(req.Operations) == 0 {
		errors = append(errors, "operations list cannot be empty")
		return errors
	}

	// Maximum batch size limit
	const maxBatchSize = 100
	if len(req.Operations) > maxBatchSize {
		errors = append(errors, fmt.Sprintf("batch size cannot exceed %d operations", maxBatchSize))
	}

	// Validate each operation
	for i, op := range req.Operations {
		opErrors := validateBatchOperation(&op, i)
		errors = append(errors, opErrors...)
	}

	return errors
}

// validateBatchOperation validates a single batch operation
func validateBatchOperation(op *BatchOperation, index int) []string {
	var errors []string
	prefix := fmt.Sprintf("operation[%d]", index)

	// Validate operation type
	validTypes := map[string]bool{"set": true, "get": true, "delete": true}
	if !validTypes[op.Type] {
		errors = append(errors, fmt.Sprintf("%s: invalid operation type '%s', must be one of: set, get, delete", prefix, op.Type))
	}

	// Validate key
	if strings.TrimSpace(op.Key) == "" {
		errors = append(errors, fmt.Sprintf("%s: key cannot be empty", prefix))
	}

	// Key length limit
	const maxKeyLength = 250
	if len(op.Key) > maxKeyLength {
		errors = append(errors, fmt.Sprintf("%s: key length cannot exceed %d characters", prefix, maxKeyLength))
	}

	// Validate operation-specific requirements
	switch op.Type {
	case "set":
		if op.Value == nil {
			errors = append(errors, fmt.Sprintf("%s: value is required for set operations", prefix))
		} else {
			// Validate value size (convert to string to check size)
			valueStr := fmt.Sprintf("%v", op.Value)
			const maxValueSize = 1024 * 1024 // 1MB
			if len(valueStr) > maxValueSize {
				errors = append(errors, fmt.Sprintf("%s: value size cannot exceed %d bytes", prefix, maxValueSize))
			}
		}

		// Validate TTL if provided
		if op.TTL < 0 {
			errors = append(errors, fmt.Sprintf("%s: TTL cannot be negative", prefix))
		}
		const maxTTL = 31536000 // 1 year in seconds
		if op.TTL > maxTTL {
			errors = append(errors, fmt.Sprintf("%s: TTL cannot exceed %d seconds (1 year)", prefix, maxTTL))
		}

	case "get", "delete":
		// These operations should not have value or TTL
		if op.Value != nil {
			errors = append(errors, fmt.Sprintf("%s: value should not be provided for %s operations", prefix, op.Type))
		}
		if op.TTL != 0 {
			errors = append(errors, fmt.Sprintf("%s: TTL should not be provided for %s operations", prefix, op.Type))
		}
	}

	return errors
}

// sanitizeBatchRequest sanitizes input data in a batch request
func sanitizeBatchRequest(req *BatchRequest) {
	if req == nil {
		return
	}

	for i := range req.Operations {
		op := &req.Operations[i]

		// Trim whitespace from key
		op.Key = strings.TrimSpace(op.Key)

		// Normalize operation type to lowercase
		op.Type = strings.ToLower(strings.TrimSpace(op.Type))

		// For set operations, ensure value is properly typed
		if op.Type == "set" && op.Value != nil {
			// Convert value to string if it's not already
			if str, ok := op.Value.(string); ok {
				op.Value = strings.TrimSpace(str)
			}
		}
	}
}

// BatchProcessor handles batch operations with atomic support
type BatchProcessor struct {
	store *store.MantisStore
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(store *store.MantisStore) *BatchProcessor {
	return &BatchProcessor{
		store: store,
	}
}

// ProcessBatch processes a batch request and returns the response
func (bp *BatchProcessor) ProcessBatch(ctx context.Context, req *BatchRequest) *BatchResponse {
	response := &BatchResponse{
		Results: make([]BatchResult, len(req.Operations)),
		Success: true,
		Errors:  make([]string, 0),
	}

	if req.Atomic {
		return bp.processAtomicBatch(ctx, req, response)
	} else {
		return bp.processNonAtomicBatch(ctx, req, response)
	}
}

// processAtomicBatch processes operations atomically using transactions
func (bp *BatchProcessor) processAtomicBatch(ctx context.Context, req *BatchRequest, response *BatchResponse) *BatchResponse {
	// Begin transaction
	tx, err := bp.store.KV().BeginTransaction(ctx)
	if err != nil {
		response.Success = false
		response.Errors = append(response.Errors, fmt.Sprintf("failed to begin transaction: %v", err))
		// Mark all operations as failed
		for i := range response.Results {
			response.Results[i] = BatchResult{
				Key:     req.Operations[i].Key,
				Success: false,
				Error:   "transaction failed to start",
			}
		}
		return response
	}

	// Process all operations within the transaction
	allSuccessful := true
	for i, op := range req.Operations {
		result := bp.processOperationInTransaction(ctx, tx, &op)
		response.Results[i] = result

		if !result.Success {
			allSuccessful = false
			response.Errors = append(response.Errors, fmt.Sprintf("operation %d failed: %s", i, result.Error))
		}
	}

	// Commit or rollback based on success
	if allSuccessful {
		if err := tx.Commit(); err != nil {
			response.Success = false
			response.Errors = append(response.Errors, fmt.Sprintf("failed to commit transaction: %v", err))
			// Mark all operations as failed due to commit failure
			for i := range response.Results {
				response.Results[i].Success = false
				response.Results[i].Error = "transaction commit failed"
			}
		}
	} else {
		response.Success = false
		if err := tx.Rollback(); err != nil {
			response.Errors = append(response.Errors, fmt.Sprintf("failed to rollback transaction: %v", err))
		}
		// Mark all operations as failed due to rollback
		for i := range response.Results {
			response.Results[i].Success = false
			if response.Results[i].Error == "" {
				response.Results[i].Error = "transaction rolled back"
			}
		}
	}

	return response
}

// processNonAtomicBatch processes operations individually (best effort)
func (bp *BatchProcessor) processNonAtomicBatch(ctx context.Context, req *BatchRequest, response *BatchResponse) *BatchResponse {
	successCount := 0

	for i, op := range req.Operations {
		result := bp.processOperation(ctx, &op)
		response.Results[i] = result

		if result.Success {
			successCount++
		} else {
			response.Errors = append(response.Errors, fmt.Sprintf("operation %d failed: %s", i, result.Error))
		}
	}

	// Consider batch successful if at least one operation succeeded
	response.Success = successCount > 0

	return response
}

// processOperationInTransaction processes a single operation within a transaction
func (bp *BatchProcessor) processOperationInTransaction(ctx context.Context, tx store.TransactionWrapper, op *BatchOperation) BatchResult {
	result := BatchResult{
		Key:     op.Key,
		Success: false,
	}

	switch op.Type {
	case "get":
		value, err := tx.Get(fmt.Sprintf("kv:%s", op.Key))
		if err != nil {
			result.Error = fmt.Sprintf("key not found: %s", op.Key)
			return result
		}

		// Deserialize to check expiration
		kv, err := models.KVFromJSON([]byte(value))
		if err != nil {
			result.Error = fmt.Sprintf("failed to deserialize value: %v", err)
			return result
		}

		if kv.IsExpired() {
			tx.Delete(fmt.Sprintf("kv:%s", op.Key))
			result.Error = fmt.Sprintf("key expired: %s", op.Key)
			return result
		}

		result.Value = string(kv.Value)
		result.Success = true

	case "set":
		// Create key-value model
		var kv *models.KeyValue
		valueBytes := []byte(fmt.Sprintf("%v", op.Value))

		if op.TTL > 0 {
			kv = models.NewKeyValueWithTTL(op.Key, valueBytes, int64(op.TTL))
		} else {
			kv = models.NewKeyValue(op.Key, valueBytes)
		}

		// Serialize and store in transaction
		data, err := kv.ToJSON()
		if err != nil {
			result.Error = fmt.Sprintf("failed to serialize value: %v", err)
			return result
		}

		if err := tx.Put(fmt.Sprintf("kv:%s", op.Key), string(data)); err != nil {
			result.Error = fmt.Sprintf("failed to set key: %v", err)
			return result
		}

		result.Success = true

	case "delete":
		if err := tx.Delete(fmt.Sprintf("kv:%s", op.Key)); err != nil {
			result.Error = fmt.Sprintf("failed to delete key: %v", err)
			return result
		}

		result.Success = true

	default:
		result.Error = fmt.Sprintf("unsupported operation type: %s", op.Type)
	}

	return result
}

// processOperation processes a single operation outside of a transaction
func (bp *BatchProcessor) processOperation(ctx context.Context, op *BatchOperation) BatchResult {
	result := BatchResult{
		Key:     op.Key,
		Success: false,
	}

	switch op.Type {
	case "get":
		value, err := bp.store.KV().Get(ctx, op.Key)
		if err != nil {
			result.Error = fmt.Sprintf("key not found: %s", op.Key)
			return result
		}

		result.Value = string(value)
		result.Success = true

	case "set":
		valueBytes := []byte(fmt.Sprintf("%v", op.Value))
		ttl := time.Duration(op.TTL) * time.Second

		if err := bp.store.KV().Set(ctx, op.Key, valueBytes, ttl); err != nil {
			result.Error = fmt.Sprintf("failed to set key: %v", err)
			return result
		}

		result.Success = true

	case "delete":
		if err := bp.store.KV().Delete(ctx, op.Key); err != nil {
			result.Error = fmt.Sprintf("failed to delete key: %v", err)
			return result
		}

		result.Success = true

	default:
		result.Error = fmt.Sprintf("unsupported operation type: %s", op.Type)
	}

	return result
}
