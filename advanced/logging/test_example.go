package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// TestBasicLogging tests basic logging functionality
func TestBasicLogging() {
	// Create a buffer to capture output
	var buf bytes.Buffer

	// Create logger with buffer output
	logger := NewStructuredLogger(Config{
		Level:   DEBUG,
		Outputs: []LogOutput{NewJSONOutput(&buf)},
		Context: make(map[string]interface{}),
	})

	// Test basic logging
	logger.Info("Test message")

	// Parse the output
	var entry LogEntry
	if err := json.Unmarshal(buf.Bytes()[:len(buf.Bytes())-1], &entry); err != nil {
		fmt.Printf("Failed to parse log entry: %v\n", err)
		return
	}

	// Verify the entry
	if entry.Level != "INFO" {
		fmt.Printf("Expected level INFO, got %s\n", entry.Level)
		return
	}

	if entry.Message != "Test message" {
		fmt.Printf("Expected message 'Test message', got %s\n", entry.Message)
		return
	}

	fmt.Println("✓ Basic logging test passed")
}

// TestContextLogging tests context-aware logging
func TestContextLogging() {
	var buf bytes.Buffer

	logger := NewStructuredLogger(Config{
		Level:   DEBUG,
		Outputs: []LogOutput{NewJSONOutput(&buf)},
		Context: make(map[string]interface{}),
	})

	// Test context logging
	contextLogger := logger.WithRequestID("req_123").WithUserID("user_456")
	contextLogger.Info("Context test")

	// Parse the output
	var entry LogEntry
	if err := json.Unmarshal(buf.Bytes()[:len(buf.Bytes())-1], &entry); err != nil {
		fmt.Printf("Failed to parse log entry: %v\n", err)
		return
	}

	// Verify context
	if entry.RequestID != "req_123" {
		fmt.Printf("Expected request ID 'req_123', got %s\n", entry.RequestID)
		return
	}

	if entry.UserID != "user_456" {
		fmt.Printf("Expected user ID 'user_456', got %s\n", entry.UserID)
		return
	}

	fmt.Println("✓ Context logging test passed")
}

// TestFileOutput tests file output functionality
func TestFileOutput() {
	// Create temporary directory
	tempDir := filepath.Join(os.TempDir(), "mantisdb_log_test")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		fmt.Printf("Failed to create temp dir: %v\n", err)
		return
	}
	defer os.RemoveAll(tempDir)

	// Create file output
	logFile := filepath.Join(tempDir, "test.log")
	fileOutput, err := NewFileOutput(FileOutputConfig{
		Filename:   logFile,
		MaxSize:    1024, // 1KB for testing
		MaxAge:     1,
		MaxBackups: 3,
	})
	if err != nil {
		fmt.Printf("Failed to create file output: %v\n", err)
		return
	}
	defer fileOutput.Close()

	// Create logger with file output
	logger := NewStructuredLogger(Config{
		Level:   INFO,
		Outputs: []LogOutput{fileOutput},
		Context: make(map[string]interface{}),
	})

	// Write some logs
	for i := 0; i < 5; i++ {
		logger.InfoWithMetadata(fmt.Sprintf("Test message %d", i), map[string]interface{}{
			"iteration": i,
		})
	}

	// Check if file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		fmt.Printf("Log file was not created: %s\n", logFile)
		return
	}

	fmt.Println("✓ File output test passed")
}

// TestLogManager tests log management functionality
func TestLogManager() {
	// Create temporary directory
	tempDir := filepath.Join(os.TempDir(), "mantisdb_log_manager_test")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		fmt.Printf("Failed to create temp dir: %v\n", err)
		return
	}
	defer os.RemoveAll(tempDir)

	// Create test log file
	logFile := filepath.Join(tempDir, "test.log")
	file, err := os.Create(logFile)
	if err != nil {
		fmt.Printf("Failed to create test log file: %v\n", err)
		return
	}

	// Write test log entries
	testEntries := []LogEntry{
		{
			Timestamp: time.Now().Add(-2 * time.Hour),
			Level:     "INFO",
			Component: "test",
			Message:   "Test message 1",
		},
		{
			Timestamp: time.Now().Add(-1 * time.Hour),
			Level:     "ERROR",
			Component: "test",
			Message:   "Test error message",
		},
		{
			Timestamp: time.Now(),
			Level:     "DEBUG",
			Component: "other",
			Message:   "Debug message",
		},
	}

	for _, entry := range testEntries {
		data, _ := json.Marshal(entry)
		file.Write(append(data, '\n'))
	}
	file.Close()

	// Create log manager
	logger := NewStructuredLogger(Config{
		Level:   DEBUG,
		Outputs: []LogOutput{NewJSONOutput(os.Stdout)},
		Context: make(map[string]interface{}),
	})

	manager := NewLogManager(LogManagerConfig{
		LogDir: tempDir,
		Logger: logger,
	})

	// Test search functionality
	filter := LogFilter{
		Component: "test",
		Limit:     10,
	}

	results, err := manager.SearchLogs(filter)
	if err != nil {
		fmt.Printf("Failed to search logs: %v\n", err)
		return
	}

	if len(results.Entries) != 2 {
		fmt.Printf("Expected 2 entries, got %d\n", len(results.Entries))
		return
	}

	// Test tail functionality
	tailEntries, err := manager.TailLogs(1, LogFilter{})
	if err != nil {
		fmt.Printf("Failed to tail logs: %v\n", err)
		return
	}

	if len(tailEntries) != 1 {
		fmt.Printf("Expected 1 tail entry, got %d\n", len(tailEntries))
		return
	}

	fmt.Println("✓ Log manager test passed")
}

// RunAllTests runs all logging tests
func RunAllTests() {
	fmt.Println("Running structured logging tests...")

	TestBasicLogging()
	TestContextLogging()
	TestFileOutput()
	TestLogManager()

	fmt.Println("All tests completed!")
}
