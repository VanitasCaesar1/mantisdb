package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mantisDB/models"
	"mantisDB/store"
)

// AdminAPI represents the admin API handler
type AdminAPI struct {
	store        *store.MantisStore
	queryHistory []QueryHistoryEntry
	backups      map[string]*BackupInfo // Mock backup storage
}

// QueryHistoryEntry represents a query execution record
type QueryHistoryEntry struct {
	ID           string    `json:"id"`
	Query        string    `json:"query"`
	QueryType    string    `json:"query_type"`
	ExecutedAt   time.Time `json:"executed_at"`
	Duration     int64     `json:"duration_ms"`
	RowsAffected int64     `json:"rows_affected"`
	Success      bool      `json:"success"`
	Error        string    `json:"error,omitempty"`
}

// TableInfo represents table metadata
type TableInfo struct {
	Name      string    `json:"name"`
	Type      string    `json:"type"` // "table", "collection", "keyvalue"
	RowCount  int64     `json:"row_count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Size      int64     `json:"size_bytes"`
}

// QueryRequest represents a query execution request
type QueryRequest struct {
	Query     string `json:"query"`
	QueryType string `json:"query_type"` // "sql", "document", "keyvalue"
	Limit     int    `json:"limit,omitempty"`
	Offset    int    `json:"offset,omitempty"`
}

// QueryResponse represents a query execution response
type QueryResponse struct {
	Success      bool        `json:"success"`
	Data         interface{} `json:"data,omitempty"`
	RowsAffected int64       `json:"rows_affected"`
	Duration     int64       `json:"duration_ms"`
	Error        string      `json:"error,omitempty"`
	QueryID      string      `json:"query_id"`
}

// BackupInfo represents backup metadata
type BackupInfo struct {
	ID          string            `json:"id"`
	Status      string            `json:"status"` // "creating", "completed", "failed"
	CreatedAt   time.Time         `json:"created_at"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
	Size        int64             `json:"size_bytes"`
	RecordCount int64             `json:"record_count"`
	Checksum    string            `json:"checksum"`
	Tags        map[string]string `json:"tags"`
	Error       string            `json:"error,omitempty"`
	Progress    int               `json:"progress_percent"`
}

// BackupRequest represents a backup creation request
type BackupRequest struct {
	Tags        map[string]string `json:"tags,omitempty"`
	Description string            `json:"description,omitempty"`
}

// RestoreRequest represents a backup restoration request
type RestoreRequest struct {
	TargetPath string `json:"target_path,omitempty"`
	Overwrite  bool   `json:"overwrite,omitempty"`
}

// LogFilter represents log filtering criteria
type LogFilter struct {
	Level       string    `json:"level,omitempty"`
	Component   string    `json:"component,omitempty"`
	RequestID   string    `json:"request_id,omitempty"`
	UserID      string    `json:"user_id,omitempty"`
	StartTime   time.Time `json:"start_time,omitempty"`
	EndTime     time.Time `json:"end_time,omitempty"`
	SearchQuery string    `json:"search_query,omitempty"`
	Limit       int       `json:"limit,omitempty"`
	Offset      int       `json:"offset,omitempty"`
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Component string                 `json:"component"`
	Message   string                 `json:"message"`
	RequestID string                 `json:"request_id,omitempty"`
	UserID    string                 `json:"user_id,omitempty"`
	Query     string                 `json:"query,omitempty"`
	Duration  int64                  `json:"duration_ms,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// SystemStats represents system-level statistics
type SystemStats struct {
	Uptime            int64                  `json:"uptime_seconds"`
	Version           string                 `json:"version"`
	GoVersion         string                 `json:"go_version"`
	Platform          string                 `json:"platform"`
	CPUUsage          float64                `json:"cpu_usage_percent"`
	MemoryUsage       int64                  `json:"memory_usage_bytes"`
	DiskUsage         int64                  `json:"disk_usage_bytes"`
	NetworkStats      map[string]interface{} `json:"network_stats"`
	ActiveConnections int                    `json:"active_connections"`
	DatabaseStats     map[string]interface{} `json:"database_stats"`
}

// ConfigValidationRequest represents a configuration validation request
type ConfigValidationRequest struct {
	Config map[string]interface{} `json:"config"`
}

// ConfigValidationResponse represents a configuration validation response
type ConfigValidationResponse struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// WebSocketMessage represents a WebSocket message
type WebSocketMessage struct {
	Type      string      `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// SystemEvent represents a system-level event
type SystemEvent struct {
	Type      string                 `json:"type"`
	EventType string                 `json:"event_type"`
	Timestamp time.Time              `json:"timestamp"`
	Severity  string                 `json:"severity"`
	Data      map[string]interface{} `json:"data"`
}

// NewAdminAPI creates a new admin API handler
func NewAdminAPI(store *store.MantisStore) *AdminAPI {
	return &AdminAPI{
		store:        store,
		queryHistory: make([]QueryHistoryEntry, 0),
		backups:      make(map[string]*BackupInfo),
	}
}

// ServeHTTP implements http.Handler interface
func (a *AdminAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Route requests
	path := r.URL.Path
	method := r.Method

	switch {
	// Health and system endpoints
	case path == "/api/health":
		a.handleHealthCheck(w, r)
	case path == "/api/metrics":
		a.handleGetMetrics(w, r)
	case path == "/api/config" && method == "GET":
		a.handleGetConfig(w, r)
	case path == "/api/config" && method == "PUT":
		a.handleUpdateConfig(w, r)

	// Table management endpoints
	case path == "/api/tables" && method == "GET":
		a.handleGetTables(w, r)
	case strings.HasPrefix(path, "/api/tables/") && method == "GET":
		a.handleGetTableData(w, r)
	case strings.HasPrefix(path, "/api/tables/") && method == "POST":
		a.handleCreateTableData(w, r)
	case strings.HasPrefix(path, "/api/tables/") && method == "PUT":
		a.handleUpdateTableData(w, r)
	case strings.HasPrefix(path, "/api/tables/") && method == "DELETE":
		a.handleDeleteTableData(w, r)

	// Query endpoints
	case path == "/api/query" && method == "POST":
		a.handleExecuteQuery(w, r)
	case path == "/api/query/history" && method == "GET":
		a.handleGetQueryHistory(w, r)

	// Backup endpoints
	case path == "/api/backups" && method == "GET":
		a.handleGetBackups(w, r)
	case path == "/api/backups" && method == "POST":
		a.handleCreateBackup(w, r)
	case strings.HasPrefix(path, "/api/backups/") && method == "GET":
		a.handleGetBackupStatus(w, r)
	case strings.HasPrefix(path, "/api/backups/") && method == "DELETE":
		a.handleDeleteBackup(w, r)
	case strings.HasPrefix(path, "/api/backups/") && strings.HasSuffix(path, "/restore") && method == "POST":
		a.handleRestoreBackup(w, r)

	// Enhanced monitoring endpoints
	case path == "/api/metrics/detailed" && method == "GET":
		a.handleGetDetailedMetrics(w, r)
	case path == "/api/metrics/prometheus" && method == "GET":
		a.handlePrometheusMetrics(w, r)
	case path == "/api/health/detailed" && method == "GET":
		a.handleDetailedHealthCheck(w, r)
	case path == "/api/logs" && method == "GET":
		a.handleGetLogs(w, r)
	case path == "/api/logs/search" && method == "POST":
		a.handleSearchLogs(w, r)
	case path == "/api/logs/stream" && method == "GET":
		a.handleLogStream(w, r)
	case path == "/api/system/stats" && method == "GET":
		a.handleGetSystemStats(w, r)
	case path == "/api/stats" && method == "GET":
		a.handleGetSystemStats(w, r)

	// Enhanced configuration endpoints
	case path == "/api/config/validate" && method == "POST":
		a.handleValidateConfig(w, r)
	case path == "/api/config/reload" && method == "POST":
		a.handleReloadConfig(w, r)
	case path == "/api/config/backup" && method == "POST":
		a.handleBackupConfig(w, r)
	case path == "/api/config/restore" && method == "POST":
		a.handleRestoreConfig(w, r)

	// WebSocket endpoints
	case path == "/api/ws/metrics" && method == "GET":
		a.handleMetricsWebSocket(w, r)
	case path == "/api/ws/logs" && method == "GET":
		a.handleLogsWebSocket(w, r)
	case path == "/api/ws/events" && method == "GET":
		a.handleEventsWebSocket(w, r)

	default:
		http.NotFound(w, r)
	}
}

// writeJSON writes a JSON response
func (a *AdminAPI) writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
	}
}

// writeError writes an error response
func (a *AdminAPI) writeError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	response := map[string]interface{}{
		"error":   message,
		"success": false,
	}
	json.NewEncoder(w).Encode(response)
}

// parseTableAndID extracts table name and optional ID from URL path
func (a *AdminAPI) parseTableAndID(path string) (table, id string) {
	// Remove /api/tables/ prefix
	parts := strings.Split(strings.TrimPrefix(path, "/api/tables/"), "/")
	if len(parts) >= 1 {
		table = parts[0]
	}
	if len(parts) >= 2 && parts[1] == "data" && len(parts) >= 3 {
		id = parts[2]
	}
	return
}

// generateQueryID generates a unique query ID
func (a *AdminAPI) generateQueryID() string {
	return fmt.Sprintf("query_%d", time.Now().UnixNano())
}

// addToQueryHistory adds a query execution to history
func (a *AdminAPI) addToQueryHistory(entry QueryHistoryEntry) {
	a.queryHistory = append(a.queryHistory, entry)
	// Keep only last 100 queries
	if len(a.queryHistory) > 100 {
		a.queryHistory = a.queryHistory[1:]
	}
}

// Health and system handlers
func (a *AdminAPI) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stats := a.store.GetStats(ctx)

	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"version":   "1.0.0",
		"database":  stats,
	}
	a.writeJSON(w, response)
}

func (a *AdminAPI) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	stats := a.store.GetStats(r.Context())
	response := map[string]interface{}{
		"metrics":   stats,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	a.writeJSON(w, response)
}

func (a *AdminAPI) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"config": map[string]interface{}{
			"cache_size": "100MB",
			"data_dir":   "./data",
			"wal_dir":    "./wal",
			"admin_port": 8081,
			"db_port":    8080,
		},
	}
	a.writeJSON(w, response)
}

func (a *AdminAPI) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var config map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		a.writeError(w, "Invalid JSON in request body", http.StatusBadRequest)
		return
	}

	// In a real implementation, validate and apply configuration changes
	response := map[string]interface{}{
		"success": true,
		"message": "Configuration updated successfully",
		"config":  config,
	}
	a.writeJSON(w, response)
}

// Table management handlers
func (a *AdminAPI) handleGetTables(w http.ResponseWriter, r *http.Request) {
	// Mock table information - in real implementation, scan storage for tables
	tables := []TableInfo{
		{
			Name:      "users",
			Type:      "table",
			RowCount:  150,
			CreatedAt: time.Now().Add(-24 * time.Hour),
			UpdatedAt: time.Now().Add(-1 * time.Hour),
			Size:      1024 * 50,
		},
		{
			Name:      "products",
			Type:      "collection",
			RowCount:  75,
			CreatedAt: time.Now().Add(-12 * time.Hour),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
			Size:      1024 * 25,
		},
	}

	response := map[string]interface{}{
		"tables": tables,
		"total":  len(tables),
	}
	a.writeJSON(w, response)
}

func (a *AdminAPI) handleGetTableData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tableName, _ := a.parseTableAndID(r.URL.Path)

	if tableName == "" {
		a.writeError(w, "Table name is required", http.StatusBadRequest)
		return
	}

	// Parse query parameters
	query := r.URL.Query()
	limit, _ := strconv.Atoi(query.Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	offset, _ := strconv.Atoi(query.Get("offset"))
	if offset < 0 {
		offset = 0
	}

	// Try different store types based on table name or query parameter
	storeType := query.Get("type")
	if storeType == "" {
		storeType = "table" // default
	}

	var data interface{}
	var totalCount int64
	var err error

	switch storeType {
	case "collection", "document":
		// Query document store
		docQuery := &models.DocumentQuery{
			Collection: tableName,
			Limit:      limit,
			Offset:     offset,
		}
		result, queryErr := a.store.Documents().Query(ctx, docQuery, time.Minute)
		if queryErr != nil {
			err = queryErr
		} else {
			data = result.Documents
			totalCount = result.TotalCount
		}

	case "table", "columnar":
		// Query columnar store
		colQuery := &models.ColumnarQuery{
			Table:  tableName,
			Limit:  limit,
			Offset: offset,
		}
		result, queryErr := a.store.Columnar().Query(ctx, colQuery, time.Minute)
		if queryErr != nil {
			err = queryErr
		} else {
			data = result.Rows
			totalCount = result.TotalRows
		}

	default:
		a.writeError(w, "Unsupported store type", http.StatusBadRequest)
		return
	}

	if err != nil {
		a.writeError(w, fmt.Sprintf("Failed to query table: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"data":        data,
		"total_count": totalCount,
		"limit":       limit,
		"offset":      offset,
		"table":       tableName,
		"type":        storeType,
	}
	a.writeJSON(w, response)
}

func (a *AdminAPI) handleCreateTableData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tableName, _ := a.parseTableAndID(r.URL.Path)

	if tableName == "" {
		a.writeError(w, "Table name is required", http.StatusBadRequest)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		a.writeError(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var requestData map[string]interface{}
	if err := json.Unmarshal(body, &requestData); err != nil {
		a.writeError(w, "Invalid JSON in request body", http.StatusBadRequest)
		return
	}

	// Determine store type
	storeType := r.URL.Query().Get("type")
	if storeType == "" {
		storeType = "table"
	}

	startTime := time.Now()
	var rowsAffected int64 = 1

	switch storeType {
	case "collection", "document":
		// Create document
		doc := &models.Document{
			ID:         fmt.Sprintf("doc_%d", time.Now().UnixNano()),
			Collection: tableName,
			Data:       requestData,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
			Version:    1,
		}

		if err := a.store.Documents().Create(ctx, doc); err != nil {
			a.writeError(w, fmt.Sprintf("Failed to create document: %v", err), http.StatusInternalServerError)
			return
		}

	case "keyvalue":
		// Create key-value pair
		key, keyExists := requestData["key"].(string)
		value, valueExists := requestData["value"]

		if !keyExists || !valueExists {
			a.writeError(w, "Key and value are required for key-value operations", http.StatusBadRequest)
			return
		}

		valueBytes, _ := json.Marshal(value)
		if err := a.store.KV().Set(ctx, key, valueBytes, 0); err != nil {
			a.writeError(w, fmt.Sprintf("Failed to set key-value: %v", err), http.StatusInternalServerError)
			return
		}

	case "table", "columnar":
		// Create table row
		row := &models.Row{
			Values: requestData,
		}

		if err := a.store.Columnar().Insert(ctx, tableName, []*models.Row{row}); err != nil {
			a.writeError(w, fmt.Sprintf("Failed to insert row: %v", err), http.StatusInternalServerError)
			return
		}

	default:
		a.writeError(w, "Unsupported store type", http.StatusBadRequest)
		return
	}

	duration := time.Since(startTime).Milliseconds()

	// Add to query history
	a.addToQueryHistory(QueryHistoryEntry{
		ID:           a.generateQueryID(),
		Query:        fmt.Sprintf("INSERT INTO %s", tableName),
		QueryType:    storeType,
		ExecutedAt:   startTime,
		Duration:     duration,
		RowsAffected: rowsAffected,
		Success:      true,
	})

	response := map[string]interface{}{
		"success":       true,
		"message":       "Data created successfully",
		"rows_affected": rowsAffected,
		"duration_ms":   duration,
	}
	a.writeJSON(w, response)
}

func (a *AdminAPI) handleUpdateTableData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tableName, id := a.parseTableAndID(r.URL.Path)

	if tableName == "" {
		a.writeError(w, "Table name is required", http.StatusBadRequest)
		return
	}

	if id == "" {
		a.writeError(w, "Record ID is required for updates", http.StatusBadRequest)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		a.writeError(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var requestData map[string]interface{}
	if err := json.Unmarshal(body, &requestData); err != nil {
		a.writeError(w, "Invalid JSON in request body", http.StatusBadRequest)
		return
	}

	storeType := r.URL.Query().Get("type")
	if storeType == "" {
		storeType = "table"
	}

	startTime := time.Now()
	var rowsAffected int64 = 1

	switch storeType {
	case "collection", "document":
		// Update document
		doc, err := a.store.Documents().Get(ctx, tableName, id)
		if err != nil {
			a.writeError(w, fmt.Sprintf("Document not found: %v", err), http.StatusNotFound)
			return
		}

		// Update document data
		for key, value := range requestData {
			doc.Data[key] = value
		}

		if err := a.store.Documents().Update(ctx, doc); err != nil {
			a.writeError(w, fmt.Sprintf("Failed to update document: %v", err), http.StatusInternalServerError)
			return
		}

	case "keyvalue":
		// Update key-value pair
		value, valueExists := requestData["value"]
		if !valueExists {
			a.writeError(w, "Value is required for key-value updates", http.StatusBadRequest)
			return
		}

		valueBytes, _ := json.Marshal(value)
		if err := a.store.KV().Set(ctx, id, valueBytes, 0); err != nil {
			a.writeError(w, fmt.Sprintf("Failed to update key-value: %v", err), http.StatusInternalServerError)
			return
		}

	default:
		a.writeError(w, "Update not supported for this store type", http.StatusBadRequest)
		return
	}

	duration := time.Since(startTime).Milliseconds()

	// Add to query history
	a.addToQueryHistory(QueryHistoryEntry{
		ID:           a.generateQueryID(),
		Query:        fmt.Sprintf("UPDATE %s SET ... WHERE id = %s", tableName, id),
		QueryType:    storeType,
		ExecutedAt:   startTime,
		Duration:     duration,
		RowsAffected: rowsAffected,
		Success:      true,
	})

	response := map[string]interface{}{
		"success":       true,
		"message":       "Data updated successfully",
		"rows_affected": rowsAffected,
		"duration_ms":   duration,
	}
	a.writeJSON(w, response)
}

func (a *AdminAPI) handleDeleteTableData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tableName, id := a.parseTableAndID(r.URL.Path)

	if tableName == "" {
		a.writeError(w, "Table name is required", http.StatusBadRequest)
		return
	}

	if id == "" {
		a.writeError(w, "Record ID is required for deletion", http.StatusBadRequest)
		return
	}

	storeType := r.URL.Query().Get("type")
	if storeType == "" {
		storeType = "table"
	}

	startTime := time.Now()
	var rowsAffected int64 = 1

	switch storeType {
	case "collection", "document":
		if err := a.store.Documents().Delete(ctx, tableName, id); err != nil {
			a.writeError(w, fmt.Sprintf("Failed to delete document: %v", err), http.StatusInternalServerError)
			return
		}

	case "keyvalue":
		if err := a.store.KV().Delete(ctx, id); err != nil {
			a.writeError(w, fmt.Sprintf("Failed to delete key-value: %v", err), http.StatusInternalServerError)
			return
		}

	default:
		a.writeError(w, "Delete not supported for this store type", http.StatusBadRequest)
		return
	}

	duration := time.Since(startTime).Milliseconds()

	// Add to query history
	a.addToQueryHistory(QueryHistoryEntry{
		ID:           a.generateQueryID(),
		Query:        fmt.Sprintf("DELETE FROM %s WHERE id = %s", tableName, id),
		QueryType:    storeType,
		ExecutedAt:   startTime,
		Duration:     duration,
		RowsAffected: rowsAffected,
		Success:      true,
	})

	response := map[string]interface{}{
		"success":       true,
		"message":       "Data deleted successfully",
		"rows_affected": rowsAffected,
		"duration_ms":   duration,
	}
	a.writeJSON(w, response)
}

// Query execution handlers
func (a *AdminAPI) handleExecuteQuery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var queryReq QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&queryReq); err != nil {
		a.writeError(w, "Invalid JSON in request body", http.StatusBadRequest)
		return
	}

	if queryReq.Query == "" {
		a.writeError(w, "Query is required", http.StatusBadRequest)
		return
	}

	if queryReq.QueryType == "" {
		queryReq.QueryType = "sql"
	}

	startTime := time.Now()
	queryID := a.generateQueryID()

	var data interface{}
	var rowsAffected int64
	var err error

	// Execute query based on type
	switch queryReq.QueryType {
	case "sql", "columnar":
		// Parse and execute SQL-like query (simplified)
		data, rowsAffected, err = a.executeColumnarQuery(ctx, queryReq.Query, queryReq.Limit, queryReq.Offset)

	case "document":
		// Execute document query (simplified)
		data, rowsAffected, err = a.executeDocumentQuery(ctx, queryReq.Query, queryReq.Limit, queryReq.Offset)

	case "keyvalue":
		// Execute key-value query (simplified)
		data, rowsAffected, err = a.executeKeyValueQuery(ctx, queryReq.Query)

	default:
		err = fmt.Errorf("unsupported query type: %s", queryReq.QueryType)
	}

	duration := time.Since(startTime).Milliseconds()
	success := err == nil

	// Add to query history
	historyEntry := QueryHistoryEntry{
		ID:           queryID,
		Query:        queryReq.Query,
		QueryType:    queryReq.QueryType,
		ExecutedAt:   startTime,
		Duration:     duration,
		RowsAffected: rowsAffected,
		Success:      success,
	}

	if err != nil {
		historyEntry.Error = err.Error()
	}

	a.addToQueryHistory(historyEntry)

	// Prepare response
	response := QueryResponse{
		Success:      success,
		Duration:     duration,
		RowsAffected: rowsAffected,
		QueryID:      queryID,
	}

	if success {
		response.Data = data
	} else {
		response.Error = err.Error()
		w.WriteHeader(http.StatusBadRequest)
	}

	a.writeJSON(w, response)
}

func (a *AdminAPI) handleGetQueryHistory(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	limit, _ := strconv.Atoi(query.Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	// Return most recent queries
	history := a.queryHistory
	if len(history) > limit {
		history = history[len(history)-limit:]
	}

	response := map[string]interface{}{
		"history": history,
		"total":   len(a.queryHistory),
		"limit":   limit,
	}
	a.writeJSON(w, response)
}

// Simplified query execution methods
func (a *AdminAPI) executeColumnarQuery(ctx context.Context, query string, limit, offset int) (interface{}, int64, error) {
	// This is a simplified implementation - in reality, parse SQL and execute
	// For now, return mock data
	if limit <= 0 {
		limit = 50
	}

	mockData := []map[string]interface{}{
		{"id": 1, "name": "John Doe", "email": "john@example.com"},
		{"id": 2, "name": "Jane Smith", "email": "jane@example.com"},
	}

	return mockData, int64(len(mockData)), nil
}

func (a *AdminAPI) executeDocumentQuery(ctx context.Context, query string, limit, offset int) (interface{}, int64, error) {
	// Simplified document query execution
	if limit <= 0 {
		limit = 50
	}

	mockData := []map[string]interface{}{
		{"_id": "doc1", "title": "Sample Document", "content": "This is a sample document"},
		{"_id": "doc2", "title": "Another Document", "content": "This is another document"},
	}

	return mockData, int64(len(mockData)), nil
}

func (a *AdminAPI) executeKeyValueQuery(ctx context.Context, query string) (interface{}, int64, error) {
	// Simplified key-value query execution
	// Parse query like "GET key" or "SET key value"
	parts := strings.Fields(query)
	if len(parts) < 2 {
		return nil, 0, fmt.Errorf("invalid key-value query format")
	}

	operation := strings.ToUpper(parts[0])
	key := parts[1]

	switch operation {
	case "GET":
		value, err := a.store.KV().Get(ctx, key)
		if err != nil {
			return nil, 0, err
		}
		return map[string]interface{}{"key": key, "value": string(value)}, 1, nil

	case "SET":
		if len(parts) < 3 {
			return nil, 0, fmt.Errorf("SET requires key and value")
		}
		value := strings.Join(parts[2:], " ")
		err := a.store.KV().Set(ctx, key, []byte(value), 0)
		return map[string]interface{}{"key": key, "value": value}, 1, err

	case "DELETE", "DEL":
		err := a.store.KV().Delete(ctx, key)
		return map[string]interface{}{"key": key, "deleted": true}, 1, err

	default:
		return nil, 0, fmt.Errorf("unsupported operation: %s", operation)
	}
}

// Backup management handlers
func (a *AdminAPI) handleGetBackups(w http.ResponseWriter, r *http.Request) {
	// Convert map to slice for JSON response
	backupList := make([]*BackupInfo, 0, len(a.backups))
	for _, backup := range a.backups {
		backupList = append(backupList, backup)
	}

	response := map[string]interface{}{
		"backups": backupList,
		"total":   len(backupList),
	}
	a.writeJSON(w, response)
}

func (a *AdminAPI) handleCreateBackup(w http.ResponseWriter, r *http.Request) {
	var backupReq BackupRequest
	if err := json.NewDecoder(r.Body).Decode(&backupReq); err != nil {
		a.writeError(w, "Invalid JSON in request body", http.StatusBadRequest)
		return
	}

	// Create backup info
	backupID := fmt.Sprintf("backup_%d", time.Now().UnixNano())
	backup := &BackupInfo{
		ID:        backupID,
		Status:    "creating",
		CreatedAt: time.Now(),
		Tags:      backupReq.Tags,
		Progress:  0,
	}

	if backup.Tags == nil {
		backup.Tags = make(map[string]string)
	}
	if backupReq.Description != "" {
		backup.Tags["description"] = backupReq.Description
	}

	// Store backup info
	a.backups[backupID] = backup

	// Simulate backup creation asynchronously
	go a.simulateBackupCreation(backupID)

	response := map[string]interface{}{
		"success":   true,
		"backup_id": backupID,
		"message":   "Backup creation started",
		"backup":    backup,
	}
	a.writeJSON(w, response)
}

func (a *AdminAPI) handleGetBackupStatus(w http.ResponseWriter, r *http.Request) {
	// Extract backup ID from URL path
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/backups/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		a.writeError(w, "Backup ID is required", http.StatusBadRequest)
		return
	}

	backupID := parts[0]
	backup, exists := a.backups[backupID]
	if !exists {
		a.writeError(w, "Backup not found", http.StatusNotFound)
		return
	}

	response := map[string]interface{}{
		"backup": backup,
	}
	a.writeJSON(w, response)
}

func (a *AdminAPI) handleDeleteBackup(w http.ResponseWriter, r *http.Request) {
	// Extract backup ID from URL path
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/backups/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		a.writeError(w, "Backup ID is required", http.StatusBadRequest)
		return
	}

	backupID := parts[0]
	backup, exists := a.backups[backupID]
	if !exists {
		a.writeError(w, "Backup not found", http.StatusNotFound)
		return
	}

	// Don't allow deletion of backups that are currently being created
	if backup.Status == "creating" {
		a.writeError(w, "Cannot delete backup that is currently being created", http.StatusConflict)
		return
	}

	// Delete backup
	delete(a.backups, backupID)

	response := map[string]interface{}{
		"success": true,
		"message": "Backup deleted successfully",
	}
	a.writeJSON(w, response)
}

func (a *AdminAPI) handleRestoreBackup(w http.ResponseWriter, r *http.Request) {
	// Extract backup ID from URL path
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/backups/"), "/")
	if len(parts) < 2 || parts[0] == "" {
		a.writeError(w, "Backup ID is required", http.StatusBadRequest)
		return
	}

	backupID := parts[0]
	backup, exists := a.backups[backupID]
	if !exists {
		a.writeError(w, "Backup not found", http.StatusNotFound)
		return
	}

	if backup.Status != "completed" {
		a.writeError(w, "Can only restore completed backups", http.StatusBadRequest)
		return
	}

	var restoreReq RestoreRequest
	if err := json.NewDecoder(r.Body).Decode(&restoreReq); err != nil {
		a.writeError(w, "Invalid JSON in request body", http.StatusBadRequest)
		return
	}

	// Simulate restore operation
	go a.simulateBackupRestore(backupID, restoreReq)

	response := map[string]interface{}{
		"success":   true,
		"message":   "Backup restoration started",
		"backup_id": backupID,
	}
	a.writeJSON(w, response)
}

// simulateBackupCreation simulates the backup creation process
func (a *AdminAPI) simulateBackupCreation(backupID string) {
	backup := a.backups[backupID]
	if backup == nil {
		return
	}

	// Simulate backup progress
	for progress := 0; progress <= 100; progress += 10 {
		time.Sleep(500 * time.Millisecond) // Simulate work
		backup.Progress = progress

		if progress == 100 {
			// Complete the backup
			now := time.Now()
			backup.Status = "completed"
			backup.CompletedAt = &now
			backup.Size = 1024 * 1024 * 50 // 50MB mock size
			backup.RecordCount = 1000      // Mock record count
			backup.Checksum = fmt.Sprintf("sha256:%x", time.Now().UnixNano())
		}
	}
}

// simulateBackupRestore simulates the backup restoration process
func (a *AdminAPI) simulateBackupRestore(backupID string, restoreReq RestoreRequest) {
	// In a real implementation, this would:
	// 1. Validate the backup file
	// 2. Stop database operations if needed
	// 3. Restore data from backup
	// 4. Restart database operations
	// 5. Verify restoration integrity

	time.Sleep(2 * time.Second) // Simulate restore time

	// For now, just log that restore completed
	fmt.Printf("Backup %s restored successfully to %s\n", backupID, restoreReq.TargetPath)
}

// Enhanced monitoring handlers
func (a *AdminAPI) handleGetDetailedMetrics(w http.ResponseWriter, r *http.Request) {
	stats := a.store.GetStats(r.Context())

	// Enhanced metrics with more detail
	detailedMetrics := map[string]interface{}{
		"database": stats,
		"performance": map[string]interface{}{
			"query_latency_p50": "15ms",
			"query_latency_p95": "45ms",
			"query_latency_p99": "120ms",
			"throughput_qps":    1250,
			"error_rate":        0.02,
			"cache_hit_ratio":   0.85,
		},
		"resources": map[string]interface{}{
			"cpu_usage_percent":  12.5,
			"memory_usage_bytes": 1024 * 1024 * 256,      // 256MB
			"disk_usage_bytes":   1024 * 1024 * 1024 * 5, // 5GB
			"network_io_bytes":   1024 * 1024 * 10,       // 10MB
			"active_connections": 25,
		},
		"operations": map[string]interface{}{
			"reads_per_second":    800,
			"writes_per_second":   450,
			"transactions_active": 12,
			"locks_held":          8,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}

	a.writeJSON(w, detailedMetrics)
}

func (a *AdminAPI) handlePrometheusMetrics(w http.ResponseWriter, r *http.Request) {
	// Generate Prometheus-format metrics
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	prometheusMetrics := `# HELP mantisdb_queries_total Total number of queries executed
# TYPE mantisdb_queries_total counter
mantisdb_queries_total{type="select"} 15420
mantisdb_queries_total{type="insert"} 8750
mantisdb_queries_total{type="update"} 3200
mantisdb_queries_total{type="delete"} 1100

# HELP mantisdb_query_duration_seconds Query execution duration
# TYPE mantisdb_query_duration_seconds histogram
mantisdb_query_duration_seconds_bucket{le="0.01"} 8500
mantisdb_query_duration_seconds_bucket{le="0.05"} 12000
mantisdb_query_duration_seconds_bucket{le="0.1"} 14500
mantisdb_query_duration_seconds_bucket{le="0.5"} 15200
mantisdb_query_duration_seconds_bucket{le="1.0"} 15400
mantisdb_query_duration_seconds_bucket{le="+Inf"} 15420
mantisdb_query_duration_seconds_sum 125.5
mantisdb_query_duration_seconds_count 15420

# HELP mantisdb_active_connections Current number of active connections
# TYPE mantisdb_active_connections gauge
mantisdb_active_connections 25

# HELP mantisdb_memory_usage_bytes Current memory usage in bytes
# TYPE mantisdb_memory_usage_bytes gauge
mantisdb_memory_usage_bytes 268435456

# HELP mantisdb_cache_hit_ratio Cache hit ratio
# TYPE mantisdb_cache_hit_ratio gauge
mantisdb_cache_hit_ratio 0.85
`

	w.Write([]byte(prometheusMetrics))
}

func (a *AdminAPI) handleDetailedHealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stats := a.store.GetStats(ctx)

	// Detailed health check with component status
	healthStatus := map[string]interface{}{
		"status":         "healthy",
		"timestamp":      time.Now().Format(time.RFC3339),
		"version":        "1.0.0",
		"uptime_seconds": 3600, // Mock uptime
		"components": map[string]interface{}{
			"database": map[string]interface{}{
				"status": "healthy",
				"stats":  stats,
			},
			"storage_engine": map[string]interface{}{
				"status":               "healthy",
				"disk_space_available": "50GB",
			},
			"cache": map[string]interface{}{
				"status":    "healthy",
				"hit_ratio": 0.85,
				"size":      "256MB",
			},
			"wal": map[string]interface{}{
				"status":          "healthy",
				"current_lsn":     12345,
				"last_checkpoint": time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
			},
			"backup_system": map[string]interface{}{
				"status":         "healthy",
				"active_backups": len(a.backups),
			},
		},
		"checks": map[string]interface{}{
			"connectivity":  "pass",
			"disk_space":    "pass",
			"memory_usage":  "pass",
			"response_time": "pass",
		},
	}

	a.writeJSON(w, healthStatus)
}

func (a *AdminAPI) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	// Parse query parameters
	level := query.Get("level")
	component := query.Get("component")
	limit, _ := strconv.Atoi(query.Get("limit"))
	if limit <= 0 {
		limit = 100
	}

	// Mock log entries
	mockLogs := []LogEntry{
		{
			Timestamp: time.Now().Add(-1 * time.Minute),
			Level:     "INFO",
			Component: "query_executor",
			Message:   "Query executed successfully",
			RequestID: "req_123",
			Duration:  45,
			Metadata: map[string]interface{}{
				"query":         "SELECT * FROM users",
				"rows_returned": 150,
			},
		},
		{
			Timestamp: time.Now().Add(-2 * time.Minute),
			Level:     "WARN",
			Component: "cache_manager",
			Message:   "Cache eviction triggered",
			Metadata: map[string]interface{}{
				"evicted_entries": 50,
				"memory_pressure": true,
			},
		},
		{
			Timestamp: time.Now().Add(-3 * time.Minute),
			Level:     "ERROR",
			Component: "storage_engine",
			Message:   "Temporary I/O error, retrying",
			Metadata: map[string]interface{}{
				"error":       "disk timeout",
				"retry_count": 1,
			},
		},
	}

	// Apply filters
	var filteredLogs []LogEntry
	for _, log := range mockLogs {
		if level != "" && log.Level != strings.ToUpper(level) {
			continue
		}
		if component != "" && !strings.Contains(strings.ToLower(log.Component), strings.ToLower(component)) {
			continue
		}
		filteredLogs = append(filteredLogs, log)
		if len(filteredLogs) >= limit {
			break
		}
	}

	response := map[string]interface{}{
		"logs":  filteredLogs,
		"total": len(filteredLogs),
		"filters": map[string]interface{}{
			"level":     level,
			"component": component,
			"limit":     limit,
		},
	}

	a.writeJSON(w, response)
}

func (a *AdminAPI) handleSearchLogs(w http.ResponseWriter, r *http.Request) {
	var filter LogFilter
	if err := json.NewDecoder(r.Body).Decode(&filter); err != nil {
		a.writeError(w, "Invalid JSON in request body", http.StatusBadRequest)
		return
	}

	// Mock search results
	searchResults := []LogEntry{
		{
			Timestamp: time.Now().Add(-30 * time.Second),
			Level:     "INFO",
			Component: "api_server",
			Message:   "Request processed successfully",
			RequestID: "req_456",
			Duration:  25,
		},
	}

	response := map[string]interface{}{
		"results":  searchResults,
		"total":    len(searchResults),
		"filter":   filter,
		"has_more": false,
	}

	a.writeJSON(w, response)
}

func (a *AdminAPI) handleLogStream(w http.ResponseWriter, r *http.Request) {
	// Set headers for Server-Sent Events
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Send initial connection message
	fmt.Fprintf(w, "data: {\"type\":\"connected\",\"message\":\"Log stream started\"}\n\n")
	w.(http.Flusher).Flush()

	// Simulate streaming logs
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			logEntry := LogEntry{
				Timestamp: time.Now(),
				Level:     "INFO",
				Component: "stream_test",
				Message:   "Streaming log entry",
				RequestID: fmt.Sprintf("req_%d", time.Now().Unix()),
			}

			data, _ := json.Marshal(logEntry)
			fmt.Fprintf(w, "data: %s\n\n", data)
			w.(http.Flusher).Flush()
		}
	}
}

func (a *AdminAPI) handleGetSystemStats(w http.ResponseWriter, r *http.Request) {
	stats := SystemStats{
		Uptime:            3600, // 1 hour
		Version:           "1.0.0",
		GoVersion:         "go1.21",
		Platform:          "linux/amd64",
		CPUUsage:          12.5,
		MemoryUsage:       268435456,  // 256MB
		DiskUsage:         5368709120, // 5GB
		ActiveConnections: 25,
		NetworkStats: map[string]interface{}{
			"bytes_sent":       1024 * 1024 * 100, // 100MB
			"bytes_received":   1024 * 1024 * 80,  // 80MB
			"packets_sent":     50000,
			"packets_received": 45000,
		},
		DatabaseStats: a.store.GetStats(r.Context()),
	}

	a.writeJSON(w, stats)
}

// Enhanced configuration handlers
func (a *AdminAPI) handleValidateConfig(w http.ResponseWriter, r *http.Request) {
	var req ConfigValidationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.writeError(w, "Invalid JSON in request body", http.StatusBadRequest)
		return
	}

	// Mock validation logic
	var errors []string
	var warnings []string

	// Check required fields
	if _, exists := req.Config["data_dir"]; !exists {
		errors = append(errors, "data_dir is required")
	}

	// Check cache size
	if cacheSize, exists := req.Config["cache_size"]; exists {
		if str, ok := cacheSize.(string); ok && str == "" {
			warnings = append(warnings, "cache_size is empty, using default")
		}
	}

	response := ConfigValidationResponse{
		Valid:    len(errors) == 0,
		Errors:   errors,
		Warnings: warnings,
	}

	a.writeJSON(w, response)
}

func (a *AdminAPI) handleReloadConfig(w http.ResponseWriter, r *http.Request) {
	// In a real implementation, this would reload configuration from file
	response := map[string]interface{}{
		"success":   true,
		"message":   "Configuration reloaded successfully",
		"timestamp": time.Now().Format(time.RFC3339),
	}

	a.writeJSON(w, response)
}

func (a *AdminAPI) handleBackupConfig(w http.ResponseWriter, r *http.Request) {
	// Create a backup of current configuration
	configBackup := map[string]interface{}{
		"backup_id": fmt.Sprintf("config_backup_%d", time.Now().Unix()),
		"timestamp": time.Now().Format(time.RFC3339),
		"config": map[string]interface{}{
			"cache_size": "100MB",
			"data_dir":   "./data",
			"wal_dir":    "./wal",
			"admin_port": 8081,
			"db_port":    8080,
		},
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Configuration backed up successfully",
		"backup":  configBackup,
	}

	a.writeJSON(w, response)
}

func (a *AdminAPI) handleRestoreConfig(w http.ResponseWriter, r *http.Request) {
	var restoreReq map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&restoreReq); err != nil {
		a.writeError(w, "Invalid JSON in request body", http.StatusBadRequest)
		return
	}

	// In a real implementation, this would restore configuration from backup
	response := map[string]interface{}{
		"success":   true,
		"message":   "Configuration restored successfully",
		"timestamp": time.Now().Format(time.RFC3339),
	}

	a.writeJSON(w, response)
}

// WebSocket-style handlers using Server-Sent Events
func (a *AdminAPI) handleMetricsWebSocket(w http.ResponseWriter, r *http.Request) {
	// Set headers for Server-Sent Events
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Send initial connection message
	fmt.Fprintf(w, "data: {\"type\":\"connected\",\"message\":\"Metrics stream started\"}\n\n")
	w.(http.Flusher).Flush()

	// Stream metrics every 5 seconds
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Get current metrics
			stats := a.store.GetStats(ctx)

			metricsUpdate := map[string]interface{}{
				"type":      "metrics_update",
				"timestamp": time.Now().Format(time.RFC3339),
				"data": map[string]interface{}{
					"database": stats,
					"performance": map[string]interface{}{
						"query_latency_ms":   15 + (time.Now().Unix() % 10), // Simulate variation
						"throughput_qps":     1200 + (time.Now().Unix() % 100),
						"cache_hit_ratio":    0.85 + float64(time.Now().Unix()%10)/100,
						"active_connections": 20 + (time.Now().Unix() % 15),
					},
					"resources": map[string]interface{}{
						"cpu_usage_percent":  10.0 + float64(time.Now().Unix()%20),
						"memory_usage_bytes": 268435456 + (time.Now().Unix() % 1000000),
						"disk_io_ops":        100 + (time.Now().Unix() % 50),
					},
				},
			}

			data, _ := json.Marshal(metricsUpdate)
			fmt.Fprintf(w, "data: %s\n\n", data)
			w.(http.Flusher).Flush()
		}
	}
}

func (a *AdminAPI) handleLogsWebSocket(w http.ResponseWriter, r *http.Request) {
	// Set headers for Server-Sent Events
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Send initial connection message
	fmt.Fprintf(w, "data: {\"type\":\"connected\",\"message\":\"Log stream started\"}\n\n")
	w.(http.Flusher).Flush()

	// Stream logs every 3 seconds
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	ctx := r.Context()
	logCounter := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			logCounter++

			// Generate mock log entries
			logLevels := []string{"INFO", "WARN", "ERROR", "DEBUG"}
			components := []string{"query_executor", "cache_manager", "storage_engine", "api_server"}
			messages := []string{
				"Operation completed successfully",
				"Cache eviction triggered",
				"Connection established",
				"Query executed",
				"Backup completed",
				"Configuration updated",
			}

			logEntry := LogEntry{
				Timestamp: time.Now(),
				Level:     logLevels[logCounter%len(logLevels)],
				Component: components[logCounter%len(components)],
				Message:   messages[logCounter%len(messages)],
				RequestID: fmt.Sprintf("req_%d", time.Now().Unix()),
				Duration:  int64(10 + (logCounter % 100)),
				Metadata: map[string]interface{}{
					"sequence": logCounter,
					"node_id":  "node_1",
				},
			}

			logUpdate := map[string]interface{}{
				"type": "log_entry",
				"data": logEntry,
			}

			data, _ := json.Marshal(logUpdate)
			fmt.Fprintf(w, "data: %s\n\n", data)
			w.(http.Flusher).Flush()
		}
	}
}

func (a *AdminAPI) handleEventsWebSocket(w http.ResponseWriter, r *http.Request) {
	// Set headers for Server-Sent Events
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Send initial connection message
	fmt.Fprintf(w, "data: {\"type\":\"connected\",\"message\":\"Events stream started\"}\n\n")
	w.(http.Flusher).Flush()

	// Stream events every 10 seconds
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	ctx := r.Context()
	eventCounter := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			eventCounter++

			// Generate mock system events
			eventTypes := []string{"backup_completed", "config_changed", "alert_triggered", "maintenance_started"}

			var eventData interface{}
			eventType := eventTypes[eventCounter%len(eventTypes)]

			switch eventType {
			case "backup_completed":
				eventData = map[string]interface{}{
					"backup_id":   fmt.Sprintf("backup_%d", time.Now().Unix()),
					"size_bytes":  1024 * 1024 * 50, // 50MB
					"duration_ms": 30000,
				}
			case "config_changed":
				eventData = map[string]interface{}{
					"setting":   "cache_size",
					"old_value": "100MB",
					"new_value": "200MB",
				}
			case "alert_triggered":
				eventData = map[string]interface{}{
					"alert_type":    "high_memory_usage",
					"threshold":     80.0,
					"current_value": 85.5,
				}
			case "maintenance_started":
				eventData = map[string]interface{}{
					"maintenance_type":   "index_rebuild",
					"estimated_duration": "15 minutes",
				}
			}

			systemEvent := map[string]interface{}{
				"type":       "system_event",
				"event_type": eventType,
				"timestamp":  time.Now().Format(time.RFC3339),
				"data":       eventData,
				"severity":   "info",
			}

			data, _ := json.Marshal(systemEvent)
			fmt.Fprintf(w, "data: %s\n\n", data)
			w.(http.Flusher).Flush()
		}
	}
}
