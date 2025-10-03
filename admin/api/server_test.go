package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Test helper functions
func setupTestAPI() *AdminAPI {
	// Create a minimal AdminAPI without a real store for basic HTTP testing
	api := &AdminAPI{
		store:        nil, // We'll test endpoints that don't require store access
		queryHistory: make([]QueryHistoryEntry, 0),
		backups:      make(map[string]*BackupInfo),
	}
	return api
}

func makeRequest(api *AdminAPI, method, path string, body interface{}) *httptest.ResponseRecorder {
	var reqBody io.Reader
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req := httptest.NewRequest(method, path, reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	recorder := httptest.NewRecorder()
	api.ServeHTTP(recorder, req)
	return recorder
}

func assertJSONResponse(t *testing.T, recorder *httptest.ResponseRecorder, expectedStatus int) map[string]interface{} {
	if recorder.Code != expectedStatus {
		t.Errorf("Expected status %d, got %d. Body: %s", expectedStatus, recorder.Code, recorder.Body.String())
	}

	if !strings.Contains(recorder.Header().Get("Content-Type"), "application/json") {
		t.Errorf("Expected JSON content type, got %s", recorder.Header().Get("Content-Type"))
	}

	var response map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON response: %v. Body: %s", err, recorder.Body.String())
	}

	return response
}

// Test CORS Headers
func TestCORSHeaders(t *testing.T) {
	api := setupTestAPI()
	recorder := makeRequest(api, "OPTIONS", "/api/health", nil)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200 for OPTIONS, got %d", recorder.Code)
	}

	expectedHeaders := map[string]string{
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Methods": "GET, POST, PUT, DELETE, OPTIONS",
		"Access-Control-Allow-Headers": "Content-Type, Authorization",
	}

	for header, expectedValue := range expectedHeaders {
		actualValue := recorder.Header().Get(header)
		if actualValue != expectedValue {
			t.Errorf("Expected header %s: %s, got %s", header, expectedValue, actualValue)
		}
	}
}

// Test Configuration Endpoints (these don't require store access)
func TestGetConfig(t *testing.T) {
	api := setupTestAPI()
	recorder := makeRequest(api, "GET", "/api/config", nil)

	response := assertJSONResponse(t, recorder, http.StatusOK)

	config, exists := response["config"].(map[string]interface{})
	if !exists {
		t.Fatal("Expected config section in response")
	}

	expectedKeys := []string{"cache_size", "data_dir", "wal_dir", "admin_port", "db_port"}
	for _, key := range expectedKeys {
		if _, exists := config[key]; !exists {
			t.Errorf("Expected config key %s", key)
		}
	}
}

func TestUpdateConfig(t *testing.T) {
	api := setupTestAPI()

	newConfig := map[string]interface{}{
		"cache_size": "200MB",
		"data_dir":   "./new_data",
	}

	recorder := makeRequest(api, "PUT", "/api/config", newConfig)
	response := assertJSONResponse(t, recorder, http.StatusOK)

	if response["success"] != true {
		t.Errorf("Expected success true, got %v", response["success"])
	}

	if _, exists := response["message"]; !exists {
		t.Error("Expected message in config update response")
	}
}

func TestUpdateConfigInvalidJSON(t *testing.T) {
	api := setupTestAPI()

	req := httptest.NewRequest("PUT", "/api/config", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	api.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", recorder.Code)
	}
}

func TestValidateConfig(t *testing.T) {
	api := setupTestAPI()

	validConfig := map[string]interface{}{
		"config": map[string]interface{}{
			"data_dir":   "./data",
			"cache_size": "100MB",
		},
	}

	recorder := makeRequest(api, "POST", "/api/config/validate", validConfig)
	response := assertJSONResponse(t, recorder, http.StatusOK)

	if response["valid"] != true {
		t.Errorf("Expected valid true, got %v", response["valid"])
	}
}

func TestValidateConfigMissingRequired(t *testing.T) {
	api := setupTestAPI()

	invalidConfig := map[string]interface{}{
		"config": map[string]interface{}{
			"cache_size": "100MB",
			// Missing data_dir
		},
	}

	recorder := makeRequest(api, "POST", "/api/config/validate", invalidConfig)
	response := assertJSONResponse(t, recorder, http.StatusOK)

	if response["valid"] != false {
		t.Errorf("Expected valid false, got %v", response["valid"])
	}

	errors, exists := response["errors"].([]interface{})
	if !exists || len(errors) == 0 {
		t.Error("Expected validation errors")
	}
}

func TestReloadConfig(t *testing.T) {
	api := setupTestAPI()
	recorder := makeRequest(api, "POST", "/api/config/reload", nil)

	response := assertJSONResponse(t, recorder, http.StatusOK)

	if response["success"] != true {
		t.Errorf("Expected success true, got %v", response["success"])
	}
}

func TestBackupConfig(t *testing.T) {
	api := setupTestAPI()
	recorder := makeRequest(api, "POST", "/api/config/backup", nil)

	response := assertJSONResponse(t, recorder, http.StatusOK)

	if response["success"] != true {
		t.Errorf("Expected success true, got %v", response["success"])
	}

	backup, exists := response["backup"].(map[string]interface{})
	if !exists {
		t.Fatal("Expected backup section in response")
	}

	if _, exists := backup["backup_id"]; !exists {
		t.Error("Expected backup_id in backup response")
	}
}

func TestRestoreConfig(t *testing.T) {
	api := setupTestAPI()

	restoreReq := map[string]interface{}{
		"backup_id": "config_backup_123",
	}

	recorder := makeRequest(api, "POST", "/api/config/restore", restoreReq)
	response := assertJSONResponse(t, recorder, http.StatusOK)

	if response["success"] != true {
		t.Errorf("Expected success true, got %v", response["success"])
	}
}

// Test Backup Management Endpoints
func TestGetBackups(t *testing.T) {
	api := setupTestAPI()
	recorder := makeRequest(api, "GET", "/api/backups", nil)

	response := assertJSONResponse(t, recorder, http.StatusOK)

	if _, exists := response["backups"]; !exists {
		t.Error("Expected backups array in response")
	}

	if _, exists := response["total"]; !exists {
		t.Error("Expected total in backups response")
	}
}

func TestCreateBackup(t *testing.T) {
	api := setupTestAPI()

	backupReq := BackupRequest{
		Description: "Test backup",
		Tags: map[string]string{
			"environment": "test",
			"type":        "manual",
		},
	}

	recorder := makeRequest(api, "POST", "/api/backups", backupReq)
	response := assertJSONResponse(t, recorder, http.StatusOK)

	if response["success"] != true {
		t.Errorf("Expected success true, got %v", response["success"])
	}

	backupID, exists := response["backup_id"].(string)
	if !exists || backupID == "" {
		t.Error("Expected backup_id in response")
	}

	backup, exists := response["backup"].(map[string]interface{})
	if !exists {
		t.Fatal("Expected backup object in response")
	}

	if backup["status"] != "creating" {
		t.Errorf("Expected status 'creating', got %v", backup["status"])
	}

	// Wait a moment and check backup status
	time.Sleep(100 * time.Millisecond)

	statusRecorder := makeRequest(api, "GET", fmt.Sprintf("/api/backups/%s", backupID), nil)
	statusResponse := assertJSONResponse(t, statusRecorder, http.StatusOK)

	backupStatus, exists := statusResponse["backup"].(map[string]interface{})
	if !exists {
		t.Fatal("Expected backup object in status response")
	}

	// Progress should be a number (could be 0 initially)
	if progress, exists := backupStatus["progress_percent"]; !exists {
		t.Error("Expected progress_percent in backup status")
	} else if _, ok := progress.(float64); !ok {
		t.Errorf("Expected progress_percent to be a number, got %T: %v", progress, progress)
	}
}

func TestGetBackupStatus(t *testing.T) {
	api := setupTestAPI()

	// Create a backup first
	backupReq := BackupRequest{Description: "Test backup"}
	createRecorder := makeRequest(api, "POST", "/api/backups", backupReq)
	createResponse := assertJSONResponse(t, createRecorder, http.StatusOK)

	backupID := createResponse["backup_id"].(string)

	// Get backup status
	recorder := makeRequest(api, "GET", fmt.Sprintf("/api/backups/%s", backupID), nil)
	response := assertJSONResponse(t, recorder, http.StatusOK)

	backup, exists := response["backup"].(map[string]interface{})
	if !exists {
		t.Fatal("Expected backup object in response")
	}

	if backup["id"] != backupID {
		t.Errorf("Expected backup ID %s, got %v", backupID, backup["id"])
	}
}

func TestGetBackupStatusNotFound(t *testing.T) {
	api := setupTestAPI()
	recorder := makeRequest(api, "GET", "/api/backups/nonexistent", nil)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", recorder.Code)
	}
}

func TestDeleteBackup(t *testing.T) {
	api := setupTestAPI()

	// Create a backup first
	backupReq := BackupRequest{Description: "Test backup"}
	createRecorder := makeRequest(api, "POST", "/api/backups", backupReq)
	createResponse := assertJSONResponse(t, createRecorder, http.StatusOK)

	backupID := createResponse["backup_id"].(string)

	// Wait for backup to complete (simulate)
	time.Sleep(6 * time.Second) // Wait for mock backup to complete

	// Delete backup
	recorder := makeRequest(api, "DELETE", fmt.Sprintf("/api/backups/%s", backupID), nil)
	response := assertJSONResponse(t, recorder, http.StatusOK)

	if response["success"] != true {
		t.Errorf("Expected success true, got %v", response["success"])
	}

	// Verify backup is deleted
	statusRecorder := makeRequest(api, "GET", fmt.Sprintf("/api/backups/%s", backupID), nil)
	if statusRecorder.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 after deletion, got %d", statusRecorder.Code)
	}
}

func TestDeleteBackupInProgress(t *testing.T) {
	api := setupTestAPI()

	// Create a backup
	backupReq := BackupRequest{Description: "Test backup"}
	createRecorder := makeRequest(api, "POST", "/api/backups", backupReq)
	createResponse := assertJSONResponse(t, createRecorder, http.StatusOK)

	backupID := createResponse["backup_id"].(string)

	// Try to delete immediately (while creating)
	recorder := makeRequest(api, "DELETE", fmt.Sprintf("/api/backups/%s", backupID), nil)

	if recorder.Code != http.StatusConflict {
		t.Errorf("Expected status 409, got %d", recorder.Code)
	}
}

func TestRestoreBackup(t *testing.T) {
	api := setupTestAPI()

	// Create and wait for backup to complete
	backupReq := BackupRequest{Description: "Test backup"}
	createRecorder := makeRequest(api, "POST", "/api/backups", backupReq)
	createResponse := assertJSONResponse(t, createRecorder, http.StatusOK)

	backupID := createResponse["backup_id"].(string)

	// Wait for backup to complete
	time.Sleep(6 * time.Second)

	// Restore backup
	restoreReq := RestoreRequest{
		TargetPath: "./restored_data",
		Overwrite:  true,
	}

	recorder := makeRequest(api, "POST", fmt.Sprintf("/api/backups/%s/restore", backupID), restoreReq)
	response := assertJSONResponse(t, recorder, http.StatusOK)

	if response["success"] != true {
		t.Errorf("Expected success true, got %v", response["success"])
	}

	if response["backup_id"] != backupID {
		t.Errorf("Expected backup_id %s, got %v", backupID, response["backup_id"])
	}
}

// Test Prometheus Metrics Endpoint
func TestPrometheusMetrics(t *testing.T) {
	api := setupTestAPI()
	recorder := makeRequest(api, "GET", "/api/metrics/prometheus", nil)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}

	contentType := recorder.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("Expected text/plain content type, got %s", contentType)
	}

	body := recorder.Body.String()
	expectedMetrics := []string{
		"mantisdb_queries_total",
		"mantisdb_query_duration_seconds",
		"mantisdb_active_connections",
		"mantisdb_memory_usage_bytes",
		"mantisdb_cache_hit_ratio",
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(body, metric) {
			t.Errorf("Expected metric %s in Prometheus output", metric)
		}
	}
}

// Test Server-Sent Events (WebSocket-style endpoints)
func TestLogStreamEndpoint(t *testing.T) {
	api := setupTestAPI()

	// Create a request with a timeout context
	req := httptest.NewRequest("GET", "/api/logs/stream", nil)
	ctx, cancel := context.WithTimeout(req.Context(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	recorder := httptest.NewRecorder()
	api.ServeHTTP(recorder, req)

	// For SSE, we expect specific headers
	contentType := recorder.Header().Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("Expected Content-Type 'text/event-stream', got %s", contentType)
	}

	cacheControl := recorder.Header().Get("Cache-Control")
	if cacheControl != "no-cache" {
		t.Errorf("Expected Cache-Control 'no-cache', got %s", cacheControl)
	}

	connection := recorder.Header().Get("Connection")
	if connection != "keep-alive" {
		t.Errorf("Expected Connection 'keep-alive', got %s", connection)
	}
}

func TestMetricsStreamEndpoint(t *testing.T) {
	api := setupTestAPI()

	// Create a request with a timeout context
	req := httptest.NewRequest("GET", "/api/ws/metrics", nil)
	ctx, cancel := context.WithTimeout(req.Context(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	recorder := httptest.NewRecorder()
	api.ServeHTTP(recorder, req)

	// Check SSE headers
	contentType := recorder.Header().Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("Expected Content-Type 'text/event-stream', got %s", contentType)
	}
}

func TestEventsStreamEndpoint(t *testing.T) {
	api := setupTestAPI()

	// Create a request with a timeout context
	req := httptest.NewRequest("GET", "/api/ws/events", nil)
	ctx, cancel := context.WithTimeout(req.Context(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	recorder := httptest.NewRecorder()
	api.ServeHTTP(recorder, req)

	// Check SSE headers
	contentType := recorder.Header().Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("Expected Content-Type 'text/event-stream', got %s", contentType)
	}
}

// Test Error Handling
func TestNotFoundEndpoint(t *testing.T) {
	api := setupTestAPI()
	recorder := makeRequest(api, "GET", "/api/nonexistent", nil)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", recorder.Code)
	}
}

// Test Security and Input Validation
func TestInputValidation(t *testing.T) {
	api := setupTestAPI()

	// Test with extremely long input
	longString := strings.Repeat("a", 10000)
	queryReq := QueryRequest{
		Query:     longString,
		QueryType: "sql",
	}

	recorder := makeRequest(api, "POST", "/api/query", queryReq)

	// Should handle long input gracefully (will fail due to no store, but shouldn't crash)
	if recorder.Code != http.StatusOK && recorder.Code != http.StatusBadRequest && recorder.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200, 400, or 500 for long input, got %d", recorder.Code)
	}
}

// Test Rate Limiting (if implemented)
func TestRateLimiting(t *testing.T) {
	api := setupTestAPI()

	// Make multiple rapid requests to config endpoint (doesn't require store)
	for i := 0; i < 10; i++ {
		recorder := makeRequest(api, "GET", "/api/config", nil)
		if recorder.Code != http.StatusOK {
			// If rate limiting is implemented, we might get 429
			if recorder.Code == http.StatusTooManyRequests {
				return // Rate limiting is working
			}
			t.Errorf("Unexpected status code %d on request %d", recorder.Code, i)
		}
	}

	// If we get here, rate limiting is not implemented (which is fine for now)
}

// Benchmark tests
func BenchmarkGetConfig(b *testing.B) {
	api := setupTestAPI()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		recorder := makeRequest(api, "GET", "/api/config", nil)
		if recorder.Code != http.StatusOK {
			b.Errorf("Expected status 200, got %d", recorder.Code)
		}
	}
}

func BenchmarkPrometheusMetrics(b *testing.B) {
	api := setupTestAPI()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		recorder := makeRequest(api, "GET", "/api/metrics/prometheus", nil)
		if recorder.Code != http.StatusOK {
			b.Errorf("Expected status 200, got %d", recorder.Code)
		}
	}
}

// Test JSON Error Response Format
func TestJSONErrorFormat(t *testing.T) {
	api := setupTestAPI()

	// Test with invalid JSON
	req := httptest.NewRequest("PUT", "/api/config", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	api.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", recorder.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse error response JSON: %v", err)
	}

	if response["success"] != false {
		t.Errorf("Expected success false in error response, got %v", response["success"])
	}

	if _, exists := response["error"]; !exists {
		t.Error("Expected error field in error response")
	}
}

// Test HTTP Method Validation
func TestMethodNotAllowed(t *testing.T) {
	api := setupTestAPI()

	// Test unsupported method on config endpoint
	recorder := makeRequest(api, "PATCH", "/api/config", nil)

	// The current implementation routes to 404, but could be 405 Method Not Allowed
	if recorder.Code != http.StatusNotFound && recorder.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 404 or 405, got %d", recorder.Code)
	}
}

// Test Content-Type Validation
func TestContentTypeValidation(t *testing.T) {
	api := setupTestAPI()

	// Test POST without Content-Type header
	req := httptest.NewRequest("POST", "/api/config/validate", strings.NewReader(`{"config": {}}`))
	// Deliberately not setting Content-Type

	recorder := httptest.NewRecorder()
	api.ServeHTTP(recorder, req)

	// Should still work as Go's JSON decoder is lenient
	if recorder.Code != http.StatusOK && recorder.Code != http.StatusBadRequest {
		t.Errorf("Expected status 200 or 400, got %d", recorder.Code)
	}
}

// Test Large Request Body Handling
func TestLargeRequestBody(t *testing.T) {
	api := setupTestAPI()

	// Create a large config object
	largeConfig := map[string]interface{}{
		"config": map[string]interface{}{
			"large_field": strings.Repeat("x", 1024*1024), // 1MB string
		},
	}

	recorder := makeRequest(api, "POST", "/api/config/validate", largeConfig)

	// Should handle large requests gracefully
	if recorder.Code != http.StatusOK && recorder.Code != http.StatusBadRequest && recorder.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status 200, 400, or 413, got %d", recorder.Code)
	}
}

// Test Concurrent Requests
func TestConcurrentRequests(t *testing.T) {
	api := setupTestAPI()

	// Test concurrent config requests (these don't modify shared state)
	const numRequests = 10
	results := make(chan int, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			recorder := makeRequest(api, "GET", "/api/config", nil)
			results <- recorder.Code
		}()
	}

	// Collect results
	for i := 0; i < numRequests; i++ {
		statusCode := <-results
		if statusCode != http.StatusOK {
			t.Errorf("Expected status 200 for concurrent request, got %d", statusCode)
		}
	}
}

// Test API Response Time
func TestResponseTime(t *testing.T) {
	api := setupTestAPI()

	start := time.Now()
	recorder := makeRequest(api, "GET", "/api/config", nil)
	duration := time.Since(start)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}

	// API should respond within reasonable time (1 second for simple config endpoint)
	if duration > time.Second {
		t.Errorf("API response took too long: %v", duration)
	}
}
