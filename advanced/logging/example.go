package logging

import (
	"context"
	"fmt"
	"time"
)

// Example demonstrates how to use the structured logging system
func Example() {
	// 1. Setup logging with configuration
	config := DefaultConfig()
	config.Level = "DEBUG"
	config.Output = "both" // Log to both stdout and file

	logger, manager, err := SetupLogging(config)
	if err != nil {
		panic(fmt.Sprintf("Failed to setup logging: %v", err))
	}
	defer logger.Close()

	// 2. Basic logging
	logger.Info("Application started")
	logger.DebugWithMetadata("Debug message with metadata", map[string]interface{}{
		"version": "1.0.0",
		"env":     "development",
	})

	// 3. Context-aware logging
	requestLogger := logger.WithRequestID("req_12345").WithUserID("user_789")
	requestLogger.Info("Processing user request")

	// 4. Component-specific logging
	queryLogger := GetComponentLogger(logger, config, "query_executor")
	queryLogger.LogQuery("SELECT * FROM users WHERE active = true", 45*time.Millisecond, map[string]interface{}{
		"rows_returned": 150,
		"cache_hit":     true,
	})

	// 5. Error logging
	err = fmt.Errorf("database connection failed")
	logger.ErrorWithMetadata("Database error occurred", map[string]interface{}{
		"error":      err.Error(),
		"connection": "primary",
		"retry":      3,
	})

	// 6. Log searching
	filter := LogFilter{
		Component:   "query_executor",
		SearchQuery: "SELECT",
		Limit:       10,
	}

	results, err := manager.SearchLogs(filter)
	if err != nil {
		logger.Error("Failed to search logs: " + err.Error())
	} else {
		logger.InfoWithMetadata("Log search completed", map[string]interface{}{
			"results_count": len(results.Entries),
			"total":         results.Total,
		})
	}

	// 7. Real-time log streaming
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	infoLevel := INFO
	streamFilter := LogFilter{
		Level: &infoLevel,
	}

	stream, err := manager.CreateStream(ctx, streamFilter)
	if err != nil {
		logger.Error("Failed to create log stream: " + err.Error())
		return
	}

	// Listen to stream in a goroutine
	go func() {
		for entry := range stream.Channel {
			fmt.Printf("Streamed log: %s - %s\n", entry.Level, entry.Message)
		}
	}()

	// Generate some logs to stream
	for i := 0; i < 5; i++ {
		time.Sleep(1 * time.Second)
		logger.InfoWithMetadata(fmt.Sprintf("Streamed message %d", i+1), map[string]interface{}{
			"iteration": i + 1,
		})
	}

	// 8. Get log statistics
	stats, err := manager.GetLogStats()
	if err != nil {
		logger.Error("Failed to get log stats: " + err.Error())
	} else {
		logger.InfoWithMetadata("Log statistics", stats)
	}

	// 9. Tail logs (get last N entries)
	tailEntries, err := manager.TailLogs(5, LogFilter{})
	if err != nil {
		logger.Error("Failed to tail logs: " + err.Error())
	} else {
		logger.InfoWithMetadata("Tail logs retrieved", map[string]interface{}{
			"count": len(tailEntries),
		})
	}
}

// ExampleWebSocketStreaming demonstrates how to use log streaming with WebSocket
func ExampleWebSocketStreaming() {
	config := DefaultConfig()
	logger, manager, err := SetupLogging(config)
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	// Create a stream for real-time log monitoring
	ctx := context.Background()
	warnLevel := WARN
	filter := LogFilter{
		Level:     &warnLevel, // Only warnings and above
		Component: "admin",
	}

	stream, err := manager.CreateStream(ctx, filter)
	if err != nil {
		logger.Error("Failed to create stream: " + err.Error())
		return
	}

	// In a real application, this would be connected to a WebSocket
	go func() {
		for entry := range stream.Channel {
			// Send to WebSocket client
			fmt.Printf("WebSocket: %s [%s] %s\n",
				entry.Timestamp.Format(time.RFC3339),
				entry.Level,
				entry.Message)
		}
	}()

	// Simulate some admin operations that generate logs
	adminLogger := logger.WithComponent("admin")
	adminLogger.Info("Admin dashboard accessed")
	adminLogger.Warn("High memory usage detected")
	adminLogger.Error("Backup operation failed")

	time.Sleep(2 * time.Second)
	manager.CloseStream(stream.ID)
}

// ExampleQueryLogging demonstrates query-specific logging
func ExampleQueryLogging() {
	config := DefaultConfig()
	logger, _, err := SetupLogging(config)
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	// Create a query-specific logger
	queryLogger := logger.WithComponent("query_executor")

	// Log successful query
	start := time.Now()
	// ... execute query ...
	duration := time.Since(start)

	queryLogger.LogQuery(
		"SELECT id, name, email FROM users WHERE created_at > ?",
		duration,
		map[string]interface{}{
			"rows_returned": 42,
			"cache_hit":     false,
			"index_used":    "idx_users_created_at",
		},
	)

	// Log query error
	queryLogger.LogQueryError(
		"SELECT * FROM non_existent_table",
		fmt.Errorf("table 'non_existent_table' doesn't exist"),
		map[string]interface{}{
			"user_id": "user_123",
			"client":  "web_dashboard",
		},
	)
}
