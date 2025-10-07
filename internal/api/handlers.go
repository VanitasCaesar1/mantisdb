// Package api provides internal API handler implementations
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mantisDB/models"
	"mantisDB/pkg/monitoring"
	"mantisDB/store"
)

// HandlerManager manages API handlers
type HandlerManager struct {
	store          *store.MantisStore
	logger         monitoring.Logger
	batchProcessor *BatchProcessor
}

// NewHandlerManager creates a new handler manager
func NewHandlerManager(store *store.MantisStore, logger monitoring.Logger) *HandlerManager {
	return &HandlerManager{
		store:          store,
		logger:         logger,
		batchProcessor: NewBatchProcessor(store),
	}
}

// RegisterHandlers registers all API handlers with the server
func (hm *HandlerManager) RegisterHandlers(server HTTPServer) {
	// Key-Value API endpoints
	server.RegisterHandler("/api/v1/kv/batch", http.HandlerFunc(hm.handleKVBatch))
	server.RegisterHandler("/api/v1/kv/", http.HandlerFunc(hm.handleKV))

	// Document API endpoints
	server.RegisterHandler("/api/v1/docs/", http.HandlerFunc(hm.handleDocuments))
	server.RegisterHandler("/api/v1/docs/query", http.HandlerFunc(hm.handleDocumentQuery))

	// Columnar API endpoints
	server.RegisterHandler("/api/v1/tables/", http.HandlerFunc(hm.handleTables))
	server.RegisterHandler("/api/v1/tables/query", http.HandlerFunc(hm.handleColumnarQuery))

	// Stats endpoint
	server.RegisterHandler("/api/v1/stats", http.HandlerFunc(hm.handleStats))

	// Version endpoint
	server.RegisterHandler("/api/v1/version", http.HandlerFunc(hm.handleVersion))

	// Health check
	server.RegisterHandler("/health", http.HandlerFunc(hm.handleHealth))
}

// Key-Value API handlers
func (hm *HandlerManager) handleKV(w http.ResponseWriter, r *http.Request) {
	// Extract key from URL path
	path := r.URL.Path[len("/api/v1/kv/"):]
	if path == "" {
		hm.writeError(w, http.StatusBadRequest, "Key is required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		hm.handleKVGet(w, r, path)
	case http.MethodPut, http.MethodPost:
		hm.handleKVSet(w, r, path)
	case http.MethodDelete:
		hm.handleKVDelete(w, r, path)
	default:
		hm.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (hm *HandlerManager) handleKVGet(w http.ResponseWriter, r *http.Request, key string) {
	ctx := r.Context()

	value, err := hm.store.KV().Get(ctx, key)
	if err != nil {
		details := map[string]string{
			"key":    key,
			"reason": "key not found",
		}
		hm.writeStructuredError(w, http.StatusNotFound, "Key not found", details, "")
		return
	}

	response := map[string]interface{}{
		"key":   key,
		"value": string(value),
	}
	hm.writeJSON(w, response)
}

func (hm *HandlerManager) handleKVSet(w http.ResponseWriter, r *http.Request, key string) {
	ctx := r.Context()

	var request struct {
		Value string `json:"value"`
		TTL   int    `json:"ttl,omitempty"` // TTL in seconds
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		hm.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	ttl := time.Duration(request.TTL) * time.Second
	if err := hm.store.KV().Set(ctx, key, []byte(request.Value), ttl); err != nil {
		hm.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := map[string]interface{}{
		"key":     key,
		"success": true,
	}
	hm.writeJSON(w, response)
}

func (hm *HandlerManager) handleKVDelete(w http.ResponseWriter, r *http.Request, key string) {
	ctx := r.Context()

	if err := hm.store.KV().Delete(ctx, key); err != nil {
		hm.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := map[string]interface{}{
		"key":     key,
		"deleted": true,
	}
	hm.writeJSON(w, response)
}

// handleKVBatch handles batch operations for key-value pairs
func (hm *HandlerManager) handleKVBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		hm.writeStructuredError(w, http.StatusMethodNotAllowed, "Method not allowed", nil, "")
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
		hm.writeStructuredError(w, http.StatusBadRequest, "Invalid JSON", details, "")
		return
	}

	// Sanitize input
	sanitizeBatchRequest(&batchReq)

	// Validate request
	if validationErrors := ValidateBatchRequest(&batchReq); len(validationErrors) > 0 {
		details := map[string]string{
			"validation_errors": strings.Join(validationErrors, "; "),
		}
		hm.writeStructuredError(w, http.StatusBadRequest, "Validation failed", details, "")
		return
	}

	// Process batch operations
	response := hm.batchProcessor.ProcessBatch(ctx, &batchReq)

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
	hm.writeJSON(w, response)
}

// Document handlers
func (hm *HandlerManager) handleDocuments(w http.ResponseWriter, r *http.Request) {
	// Extract collection and document ID from URL path
	path := r.URL.Path[len("/api/v1/docs/"):]
	parts := splitPath(path)

	if len(parts) == 0 {
		hm.writeError(w, http.StatusBadRequest, "Collection is required")
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
			hm.writeError(w, http.StatusBadRequest, "Document ID is required for GET")
			return
		}
		hm.handleDocumentGet(w, r, collection, docID)
	case http.MethodPost:
		hm.handleDocumentCreate(w, r, collection)
	case http.MethodPut:
		if docID == "" {
			hm.writeError(w, http.StatusBadRequest, "Document ID is required for PUT")
			return
		}
		hm.handleDocumentUpdate(w, r, collection, docID)
	case http.MethodDelete:
		if docID == "" {
			hm.writeError(w, http.StatusBadRequest, "Document ID is required for DELETE")
			return
		}
		hm.handleDocumentDelete(w, r, collection, docID)
	default:
		hm.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (hm *HandlerManager) handleDocumentGet(w http.ResponseWriter, r *http.Request, collection, id string) {
	ctx := r.Context()

	doc, err := hm.store.Documents().Get(ctx, collection, id)
	if err != nil {
		details := map[string]string{
			"collection": collection,
			"id":         id,
			"reason":     "document not found",
		}
		hm.writeStructuredError(w, http.StatusNotFound, "Document not found", details, "")
		return
	}

	hm.writeJSON(w, doc)
}

func (hm *HandlerManager) handleDocumentCreate(w http.ResponseWriter, r *http.Request, collection string) {
	ctx := r.Context()

	var request struct {
		ID   string                 `json:"id"`
		Data map[string]interface{} `json:"data"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		hm.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if request.ID == "" {
		request.ID = generateID()
	}

	doc := models.NewDocument(request.ID, collection, request.Data)
	if err := hm.store.Documents().Create(ctx, doc); err != nil {
		hm.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	hm.writeJSON(w, doc)
}

func (hm *HandlerManager) handleDocumentUpdate(w http.ResponseWriter, r *http.Request, collection, id string) {
	ctx := r.Context()

	// Get existing document
	doc, err := hm.store.Documents().Get(ctx, collection, id)
	if err != nil {
		hm.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var request struct {
		Data map[string]interface{} `json:"data"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		hm.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Update document data
	doc.Data = request.Data

	if err := hm.store.Documents().Update(ctx, doc); err != nil {
		hm.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	hm.writeJSON(w, doc)
}

func (hm *HandlerManager) handleDocumentDelete(w http.ResponseWriter, r *http.Request, collection, id string) {
	ctx := r.Context()

	if err := hm.store.Documents().Delete(ctx, collection, id); err != nil {
		hm.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := map[string]interface{}{
		"collection": collection,
		"id":         id,
		"deleted":    true,
	}
	hm.writeJSON(w, response)
}

func (hm *HandlerManager) handleDocumentQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		hm.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ctx := r.Context()

	var query models.DocumentQuery
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		hm.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Get cache TTL from query parameter
	cacheTTL := time.Hour // Default
	if ttlStr := r.URL.Query().Get("cache_ttl"); ttlStr != "" {
		if ttlSeconds, err := strconv.Atoi(ttlStr); err == nil {
			cacheTTL = time.Duration(ttlSeconds) * time.Second
		}
	}

	result, err := hm.store.Documents().Query(ctx, &query, cacheTTL)
	if err != nil {
		hm.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	hm.writeJSON(w, result)
}

// Stats and health endpoints
func (hm *HandlerManager) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		hm.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ctx := r.Context()
	stats := hm.store.GetStats(ctx)
	hm.writeJSON(w, stats)
}

func (hm *HandlerManager) handleVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		hm.writeStructuredError(w, http.StatusMethodNotAllowed, "Method not allowed", nil, "")
		return
	}

	versionInfo := map[string]interface{}{
		"version":    "1.0.0",
		"build":      "dev",
		"build_time": time.Now().Format(time.RFC3339),
		"go_version": "go1.21",
	}
	hm.writeJSON(w, versionInfo)
}

func (hm *HandlerManager) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		hm.writeStructuredError(w, http.StatusMethodNotAllowed, "Method not allowed", nil, "")
		return
	}

	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
	}
	hm.writeJSON(w, response)
}

// Columnar handlers (simplified for now)
func (hm *HandlerManager) handleTables(w http.ResponseWriter, r *http.Request) {
	hm.writeError(w, http.StatusNotImplemented, "Columnar API not yet implemented in new structure")
}

func (hm *HandlerManager) handleColumnarQuery(w http.ResponseWriter, r *http.Request) {
	hm.writeError(w, http.StatusNotImplemented, "Columnar query API not yet implemented in new structure")
}

// Helper methods
func (hm *HandlerManager) writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (hm *HandlerManager) writeError(w http.ResponseWriter, status int, message string) {
	hm.writeStructuredError(w, status, message, nil, "")
}

func (hm *HandlerManager) writeStructuredError(w http.ResponseWriter, status int, message string, details map[string]string, traceID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	apiError := map[string]interface{}{
		"error":    message,
		"code":     status,
		"details":  details,
		"trace_id": traceID,
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
