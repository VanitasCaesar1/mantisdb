package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"mantisDB/models"
	"mantisDB/store"
)

// VersionInfo contains version and build information
type VersionInfo struct {
	Version   string `json:"version"`
	Build     string `json:"build"`
	BuildTime string `json:"build_time"`
	GoVersion string `json:"go_version"`
}

// APIError represents a structured API error response
type APIError struct {
	Error   string            `json:"error"`
	Code    int               `json:"code"`
	Details map[string]string `json:"details,omitempty"`
	TraceID string            `json:"trace_id,omitempty"`
}

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

// Server provides HTTP API for MantisDB
type Server struct {
	store          *store.MantisStore
	port           int
	server         *http.Server
	versionInfo    *VersionInfo
	batchProcessor *BatchProcessor
}

// NewServer creates a new API server
func NewServer(store *store.MantisStore, port int) *Server {
	// Initialize version information
	versionInfo := &VersionInfo{
		Version:   "1.0.0", // This could be set via build flags
		Build:     "dev",   // This could be set via build flags
		BuildTime: time.Now().Format(time.RFC3339),
		GoVersion: runtime.Version(),
	}

	return &Server{
		store:          store,
		port:           port,
		versionInfo:    versionInfo,
		batchProcessor: NewBatchProcessor(store),
	}
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// Auth API endpoints (for admin UI)
	mux.HandleFunc("/api/auth/login", s.handleAuthLogin)
	mux.HandleFunc("/api/auth/verify", s.handleAuthVerify)
	mux.HandleFunc("/api/auth/logout", s.handleAuthLogout)

	// Key-Value API endpoints
	mux.HandleFunc("/api/v1/kv/batch", s.handleKVBatch)
	mux.HandleFunc("/api/v1/kv/", s.handleKV)

	// Document API endpoints
	mux.HandleFunc("/api/v1/docs/", s.handleDocuments)
	mux.HandleFunc("/api/v1/docs/query", s.handleDocumentQuery)

	// Columnar API endpoints
	mux.HandleFunc("/api/v1/tables/", s.handleTables)
	mux.HandleFunc("/api/v1/tables/query", s.handleColumnarQuery)
	
	// Admin API endpoints (for frontend)
	mux.HandleFunc("/api/columnar/tables", s.handleColumnarTables)
	mux.HandleFunc("/api/columnar/tables/", s.handleColumnarTableOperations)
	mux.HandleFunc("/api/metrics", s.handleMetrics)
	mux.HandleFunc("/api/stats", s.handleSystemStats) // Alias for system stats
	mux.HandleFunc("/api/system/stats", s.handleSystemStats)
	mux.HandleFunc("/api/query", s.handleSQLQuery)
	mux.HandleFunc("/api/tables", s.handleAdminTables)
	mux.HandleFunc("/api/ws/metrics", s.handleMetricsWebSocket)

	// Stats endpoint
	mux.HandleFunc("/api/v1/stats", s.handleStats)

	// Version endpoint
	mux.HandleFunc("/api/v1/version", s.handleVersion)

	// Health check
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/health", s.handleHealth)

	// Find an available port starting from the configured port
	actualPort, err := findAvailablePort(s.port, 10)
	if err != nil {
		return fmt.Errorf("failed to find available API port: %v", err)
	}

	// Update port if different
	if actualPort != s.port {
		log.Printf("API port %d in use, using port %d instead", s.port, actualPort)
		s.port = actualPort
	}

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", actualPort),
		Handler: s.corsMiddleware(s.versionMiddleware(s.loggingMiddleware(mux))),
	}

	// Start server silently
	return s.server.ListenAndServe()
}

// findAvailablePort tries to find an available port starting from the given port
func findAvailablePort(startPort int, maxAttempts int) (int, error) {
	for i := 0; i < maxAttempts; i++ {
		port := startPort + i
		addr := fmt.Sprintf(":%d", port)

		// Try to listen on the port
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			// Port is available, close the listener and return
			listener.Close()
			return port, nil
		}
	}

	return 0, fmt.Errorf("no available port found after %d attempts starting from %d", maxAttempts, startPort)
}

// Stop stops the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

// GetPort returns the actual port the server is running on
func (s *Server) GetPort() int {
	return s.port
}

// Key-Value API handlers
func (s *Server) handleKV(w http.ResponseWriter, r *http.Request) {
	// Extract key from URL path
	path := r.URL.Path[len("/api/v1/kv/"):]
	if path == "" {
		s.writeError(w, http.StatusBadRequest, "Key is required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.handleKVGet(w, r, path)
	case http.MethodPut, http.MethodPost:
		s.handleKVSet(w, r, path)
	case http.MethodDelete:
		s.handleKVDelete(w, r, path)
	default:
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (s *Server) handleKVGet(w http.ResponseWriter, r *http.Request, key string) {
	ctx := r.Context()

	value, err := s.store.KV().Get(ctx, key)
	if err != nil {
		// Return structured 404 error for nonexistent keys
		details := map[string]string{
			"key":    key,
			"reason": "key not found",
		}
		s.writeStructuredError(w, http.StatusNotFound, "Key not found", details, "")
		return
	}

	response := map[string]interface{}{
		"key":   key,
		"value": string(value),
	}
	s.writeJSON(w, response)
}

func (s *Server) handleKVSet(w http.ResponseWriter, r *http.Request, key string) {
	ctx := r.Context()

	var request struct {
		Value string `json:"value"`
		TTL   int    `json:"ttl,omitempty"` // TTL in seconds
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	ttl := time.Duration(request.TTL) * time.Second
	if err := s.store.KV().Set(ctx, key, []byte(request.Value), ttl); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := map[string]interface{}{
		"key":     key,
		"success": true,
	}
	s.writeJSON(w, response)
}

func (s *Server) handleKVDelete(w http.ResponseWriter, r *http.Request, key string) {
	ctx := r.Context()

	if err := s.store.KV().Delete(ctx, key); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := map[string]interface{}{
		"key":     key,
		"deleted": true,
	}
	s.writeJSON(w, response)
}

// handleKVBatch handles batch operations for key-value pairs
func (s *Server) handleKVBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeStructuredError(w, http.StatusMethodNotAllowed, "Method not allowed", nil, "")
		return
	}

	ctx := r.Context()

	// Check request size limit (1MB)
	const maxRequestSize = 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestSize)

	var batchReq BatchRequest
	if err := json.NewDecoder(r.Body).Decode(&batchReq); err != nil {
		details := map[string]string{
			"reason": "invalid JSON format",
		}
		s.writeStructuredError(w, http.StatusBadRequest, "Invalid JSON", details, "")
		return
	}

	// Sanitize input
	sanitizeBatchRequest(&batchReq)

	// Validate request
	if validationErrors := ValidateBatchRequest(&batchReq); len(validationErrors) > 0 {
		details := map[string]string{
			"validation_errors": strings.Join(validationErrors, "; "),
		}
		s.writeStructuredError(w, http.StatusBadRequest, "Validation failed", details, "")
		return
	}

	// Process batch operations
	response := s.batchProcessor.ProcessBatch(ctx, &batchReq)

	// Set appropriate HTTP status code
	statusCode := http.StatusOK
	if !response.Success {
		if batchReq.Atomic {
			statusCode = http.StatusConflict // Atomic operation failed
		} else {
			statusCode = http.StatusMultiStatus // Partial success
		}
	}

	w.WriteHeader(statusCode)
	s.writeJSON(w, response)
}

func (s *Server) handleDocuments(w http.ResponseWriter, r *http.Request) {
	// Extract collection and document ID from URL path
	path := r.URL.Path[len("/api/v1/docs/"):]
	parts := splitPath(path)

	if len(parts) == 0 {
		s.writeError(w, http.StatusBadRequest, "Collection is required")
		return
	}

	collection := parts[0]
	var docID string
	if len(parts) > 1 {
		docID = parts[1]
	}

	switch r.Method {
	case http.MethodGet:
		if docID == "" {
			s.writeError(w, http.StatusBadRequest, "Document ID is required for GET")
			return
		}
		s.handleDocumentGet(w, r, collection, docID)
	case http.MethodPost:
		s.handleDocumentCreate(w, r, collection)
	case http.MethodPut:
		if docID == "" {
			s.writeError(w, http.StatusBadRequest, "Document ID is required for PUT")
			return
		}
		s.handleDocumentUpdate(w, r, collection, docID)
	case http.MethodDelete:
		if docID == "" {
			s.writeError(w, http.StatusBadRequest, "Document ID is required for DELETE")
			return
		}
		s.handleDocumentDelete(w, r, collection, docID)
	default:
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (s *Server) handleDocumentGet(w http.ResponseWriter, r *http.Request, collection, id string) {
	ctx := r.Context()

	doc, err := s.store.Documents().Get(ctx, collection, id)
	if err != nil {
		details := map[string]string{
			"collection": collection,
			"id":         id,
			"reason":     "document not found",
		}
		s.writeStructuredError(w, http.StatusNotFound, "Document not found", details, "")
		return
	}

	s.writeJSON(w, doc)
}

func (s *Server) handleDocumentCreate(w http.ResponseWriter, r *http.Request, collection string) {
	ctx := r.Context()

	var request struct {
		ID   string                 `json:"id"`
		Data map[string]interface{} `json:"data"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if request.ID == "" {
		request.ID = generateID()
	}

	doc := models.NewDocument(request.ID, collection, request.Data)
	if err := s.store.Documents().Create(ctx, doc); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, doc)
}

func (s *Server) handleDocumentUpdate(w http.ResponseWriter, r *http.Request, collection, id string) {
	ctx := r.Context()

	// Get existing document
	doc, err := s.store.Documents().Get(ctx, collection, id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var request struct {
		Data map[string]interface{} `json:"data"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Update document data
	doc.Data = request.Data

	if err := s.store.Documents().Update(ctx, doc); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, doc)
}

func (s *Server) handleDocumentDelete(w http.ResponseWriter, r *http.Request, collection, id string) {
	ctx := r.Context()

	if err := s.store.Documents().Delete(ctx, collection, id); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := map[string]interface{}{
		"collection": collection,
		"id":         id,
		"deleted":    true,
	}
	s.writeJSON(w, response)
}

func (s *Server) handleDocumentQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ctx := r.Context()

	var query models.DocumentQuery
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Get cache TTL from query parameter
	cacheTTL := time.Hour // Default
	if ttlStr := r.URL.Query().Get("cache_ttl"); ttlStr != "" {
		if ttlSeconds, err := strconv.Atoi(ttlStr); err == nil {
			cacheTTL = time.Duration(ttlSeconds) * time.Second
		}
	}

	result, err := s.store.Documents().Query(ctx, &query, cacheTTL)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, result)
}

// Columnar API handlers
func (s *Server) handleTables(w http.ResponseWriter, r *http.Request) {
	// Extract table name from URL path
	path := r.URL.Path[len("/api/v1/tables/"):]
	parts := splitPath(path)

	if len(parts) == 0 {
		s.writeError(w, http.StatusBadRequest, "Table name is required")
		return
	}

	tableName := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch r.Method {
	case http.MethodGet:
		s.handleTableGet(w, r, tableName)
	case http.MethodPost:
		if action == "insert" {
			s.handleTableInsert(w, r, tableName)
		} else {
			s.handleTableCreate(w, r, tableName)
		}
	default:
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (s *Server) handleTableGet(w http.ResponseWriter, r *http.Request, tableName string) {
	ctx := r.Context()

	table, err := s.store.Columnar().GetTable(ctx, tableName)
	if err != nil {
		details := map[string]string{
			"table":  tableName,
			"reason": "table not found",
		}
		s.writeStructuredError(w, http.StatusNotFound, "Table not found", details, "")
		return
	}

	s.writeJSON(w, table)
}

func (s *Server) handleTableCreate(w http.ResponseWriter, r *http.Request, tableName string) {
	ctx := r.Context()

	var request struct {
		Columns []*models.Column `json:"columns"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	table := models.NewTable(tableName, request.Columns)
	if err := s.store.Columnar().CreateTable(ctx, table); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, table)
}

func (s *Server) handleTableInsert(w http.ResponseWriter, r *http.Request, tableName string) {
	ctx := r.Context()

	var request struct {
		Rows []*models.Row `json:"rows"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if err := s.store.Columnar().Insert(ctx, tableName, request.Rows); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := map[string]interface{}{
		"table":         tableName,
		"rows_inserted": len(request.Rows),
		"success":       true,
	}
	s.writeJSON(w, response)
}

func (s *Server) handleColumnarQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ctx := r.Context()

	var query models.ColumnarQuery
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Get cache TTL from query parameter
	cacheTTL := time.Hour // Default
	if ttlStr := r.URL.Query().Get("cache_ttl"); ttlStr != "" {
		if ttlSeconds, err := strconv.Atoi(ttlStr); err == nil {
			cacheTTL = time.Duration(ttlSeconds) * time.Second
		}
	}

	result, err := s.store.Columnar().Query(ctx, &query, cacheTTL)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, result)
}

// Stats and health endpoints
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ctx := r.Context()
	stats := s.store.GetStats(ctx)
	s.writeJSON(w, stats)
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeStructuredError(w, http.StatusMethodNotAllowed, "Method not allowed", nil, "")
		return
	}

	s.writeJSON(w, s.versionInfo)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeStructuredError(w, http.StatusMethodNotAllowed, "Method not allowed", nil, "")
		return
	}

	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
	}
	s.writeJSON(w, response)
}

// Middleware
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) versionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add version information to response headers
		w.Header().Set("X-API-Version", s.versionInfo.Version)
		w.Header().Set("X-Build-Version", s.versionInfo.Build)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		fmt.Printf("%s %s %v\n", r.Method, r.URL.Path, time.Since(start))
	})
}

// Helper methods
func (s *Server) writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeStructuredError(w, status, message, nil, "")
}

func (s *Server) writeStructuredError(w http.ResponseWriter, status int, message string, details map[string]string, traceID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	apiError := APIError{
		Error:   message,
		Code:    status,
		Details: details,
		TraceID: traceID,
	}

	json.NewEncoder(w).Encode(apiError)
}

func splitPath(path string) []string {
	if path == "" {
		return []string{}
	}

	parts := []string{}
	current := ""

	for _, char := range path {
		if char == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}

	if current != "" {
		parts = append(parts, current)
	}

	return parts
}

func generateID() string {
	return fmt.Sprintf("doc_%d", time.Now().UnixNano())
}

// Auth handlers for admin UI
func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Simple auth for development - accept any email/password
	// In production, implement proper authentication
	if request.Email == "" || request.Password == "" {
		s.writeError(w, http.StatusBadRequest, "Email and password required")
		return
	}

	// Generate a simple token (in production, use JWT or similar)
	token := fmt.Sprintf("token_%d", time.Now().UnixNano())

	response := map[string]interface{}{
		"token": token,
		"user": map[string]interface{}{
			"id":    "user_1",
			"email": request.Email,
			"role":  "admin",
		},
	}

	s.writeJSON(w, response)
}

func (s *Server) handleAuthVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		s.writeError(w, http.StatusUnauthorized, "Authorization header required")
		return
	}

	// Simple token validation (in production, validate JWT properly)
	if !strings.HasPrefix(authHeader, "Bearer ") {
		s.writeError(w, http.StatusUnauthorized, "Invalid authorization format")
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		s.writeError(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	// Return user info (in production, decode from JWT)
	response := map[string]interface{}{
		"user": map[string]interface{}{
			"id":    "user_1",
			"email": "admin@mantisdb.local",
			"role":  "admin",
		},
	}

	s.writeJSON(w, response)
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// In production, invalidate the token
	response := map[string]interface{}{
		"success": true,
		"message": "Logged out successfully",
	}

	s.writeJSON(w, response)
}

// Admin API handlers for frontend
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ctx := r.Context()
	_ = s.store.GetStats(ctx)

	// Transform stats to metrics format
	metrics := map[string]interface{}{
		"queries_per_second": 0,
		"cache_hit_ratio":    0.0,
		"avg_response_time":  0,
		"timestamp":          time.Now().Unix(),
	}

	s.writeJSON(w, map[string]interface{}{
		"metrics":   metrics,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// Store server start time
var serverStartTime = time.Now()

func (s *Server) handleSystemStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ctx := r.Context()
	stats := s.store.GetStats(ctx)

	// Get system information
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Calculate actual uptime
	uptime := time.Since(serverStartTime).Seconds()

	systemStats := map[string]interface{}{
		"version":            s.versionInfo.Version,
		"platform":           runtime.GOOS + "/" + runtime.GOARCH,
		"uptime_seconds":     uptime,
		"active_connections": runtime.NumGoroutine(), // Use goroutines as proxy for connections
		"cpu_usage_percent":  float64(runtime.NumGoroutine()) / 1000.0 * 100.0, // Rough estimate
		"memory_usage_bytes": memStats.Alloc,
		"database_stats":     stats,
	}

	s.writeJSON(w, systemStats)
}

func (s *Server) handleSQLQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var request struct {
		Query     string `json:"query"`
		QueryType string `json:"query_type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// For now, return a mock response
	// In production, integrate with query executor
	response := map[string]interface{}{
		"success":     false,
		"error":       "SQL query execution not yet implemented. Use the Table Editor or API endpoints instead.",
		"query_id":    fmt.Sprintf("query_%d", time.Now().UnixNano()),
		"duration_ms": 0,
	}

	s.writeJSON(w, response)
}

func (s *Server) handleAdminTables(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Return empty tables list for now
	// In production, list all tables from columnar store
	response := map[string]interface{}{
		"success": true,
		"tables":  []interface{}{},
	}

	s.writeJSON(w, response)
}

func (s *Server) handleColumnarTables(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Return empty tables list
	response := map[string]interface{}{
		"tables": []interface{}{},
	}

	s.writeJSON(w, response)
}

func (s *Server) handleColumnarTableOperations(w http.ResponseWriter, r *http.Request) {
	// Extract table name from URL path
	path := r.URL.Path[len("/api/columnar/tables/"):]
	parts := splitPath(path)

	if len(parts) == 0 {
		s.writeError(w, http.StatusBadRequest, "Table name is required")
		return
	}

	tableName := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	ctx := r.Context()

	switch r.Method {
	case http.MethodGet:
		// Get table schema
		table, err := s.store.Columnar().GetTable(ctx, tableName)
		if err != nil {
			s.writeError(w, http.StatusNotFound, "Table not found")
			return
		}
		s.writeJSON(w, table)

	case http.MethodPost:
		if action == "query" {
			// Query table data
			var query models.ColumnarQuery
			if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
				s.writeError(w, http.StatusBadRequest, "Invalid JSON")
				return
			}

			query.Table = tableName
			result, err := s.store.Columnar().Query(ctx, &query, time.Hour)
			if err != nil {
				s.writeError(w, http.StatusInternalServerError, err.Error())
				return
			}

			s.writeJSON(w, result)
		} else if action == "rows" {
			// Insert rows
			var request struct {
				Rows []*models.Row `json:"rows"`
			}

			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				s.writeError(w, http.StatusBadRequest, "Invalid JSON")
				return
			}

			if err := s.store.Columnar().Insert(ctx, tableName, request.Rows); err != nil {
				s.writeError(w, http.StatusInternalServerError, err.Error())
				return
			}

			response := map[string]interface{}{
				"table":         tableName,
				"rows_inserted": len(request.Rows),
				"success":       true,
			}
			s.writeJSON(w, response)
		} else if action == "delete" {
			// Delete rows
			s.writeJSON(w, map[string]interface{}{
				"success": true,
				"message": "Delete operation not yet fully implemented",
			})
		} else {
			s.writeError(w, http.StatusBadRequest, "Invalid action")
		}

	default:
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (s *Server) handleMetricsWebSocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Set headers for Server-Sent Events
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Send initial metrics
	metrics := map[string]interface{}{
		"queries_per_second": 0,
		"cache_hit_ratio":    0.0,
		"avg_response_time":  0,
		"timestamp":          time.Now().Unix(),
	}

	data, _ := json.Marshal(metrics)
	fmt.Fprintf(w, "data: %s\n\n", data)

	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	// Keep connection alive for 30 seconds then close
	// In production, implement proper WebSocket or SSE with continuous updates
	time.Sleep(30 * time.Second)
}
