package logging

import (
	"context"
	"fmt"
	"time"
)

// MantisDBLogger demonstrates integration with MantisDB components
type MantisDBLogger struct {
	baseLogger    *StructuredLogger
	queryLogger   *StructuredLogger
	storageLogger *StructuredLogger
	walLogger     *StructuredLogger
	adminLogger   *StructuredLogger
	manager       *LogManager
}

// NewMantisDBLogger creates a new MantisDB logger instance
func NewMantisDBLogger() (*MantisDBLogger, error) {
	// Setup logging configuration
	config := DefaultConfig()
	config.Level = "INFO"
	config.Output = "both"
	config.File.Path = "logs/mantisdb.log"

	// Component-specific log levels
	config.Components = map[string]string{
		"query_executor": "DEBUG",
		"storage":        "INFO",
		"wal":            "INFO",
		"admin":          "DEBUG",
	}

	// Initialize logging system
	baseLogger, manager, err := SetupLogging(config)
	if err != nil {
		return nil, fmt.Errorf("failed to setup logging: %w", err)
	}

	return &MantisDBLogger{
		baseLogger:    baseLogger,
		queryLogger:   GetComponentLogger(baseLogger, config, "query_executor"),
		storageLogger: GetComponentLogger(baseLogger, config, "storage"),
		walLogger:     GetComponentLogger(baseLogger, config, "wal"),
		adminLogger:   GetComponentLogger(baseLogger, config, "admin"),
		manager:       manager,
	}, nil
}

// LogDatabaseStartup logs database startup events
func (m *MantisDBLogger) LogDatabaseStartup(version string, config map[string]interface{}) {
	m.baseLogger.InfoWithMetadata("MantisDB starting up", map[string]interface{}{
		"version": version,
		"config":  config,
	})
}

// LogQuery logs query execution
func (m *MantisDBLogger) LogQuery(ctx context.Context, query string, duration time.Duration, result map[string]interface{}) {
	logger := m.queryLogger

	// Add context if available
	if requestID := ctx.Value("request_id"); requestID != nil {
		if rid, ok := requestID.(string); ok {
			logger = logger.WithRequestID(rid)
		}
	}

	if userID := ctx.Value("user_id"); userID != nil {
		if uid, ok := userID.(string); ok {
			logger = logger.WithUserID(uid)
		}
	}

	logger.LogQuery(query, duration, result)
}

// LogQueryError logs query execution errors
func (m *MantisDBLogger) LogQueryError(ctx context.Context, query string, err error) {
	logger := m.queryLogger

	// Add context if available
	if requestID := ctx.Value("request_id"); requestID != nil {
		if rid, ok := requestID.(string); ok {
			logger = logger.WithRequestID(rid)
		}
	}

	logger.LogQueryError(query, err, nil)
}

// LogStorageOperation logs storage operations
func (m *MantisDBLogger) LogStorageOperation(operation string, duration time.Duration, metadata map[string]interface{}) {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["operation"] = operation
	metadata["duration_ms"] = duration.Milliseconds()

	m.storageLogger.InfoWithMetadata("Storage operation completed", metadata)
}

// LogWALOperation logs WAL operations
func (m *MantisDBLogger) LogWALOperation(operation string, entryCount int, size int64) {
	m.walLogger.InfoWithMetadata("WAL operation", map[string]interface{}{
		"operation":   operation,
		"entry_count": entryCount,
		"size_bytes":  size,
	})
}

// LogAdminAction logs admin dashboard actions
func (m *MantisDBLogger) LogAdminAction(ctx context.Context, action string, metadata map[string]interface{}) {
	logger := m.adminLogger

	// Add context if available
	if requestID := ctx.Value("request_id"); requestID != nil {
		if rid, ok := requestID.(string); ok {
			logger = logger.WithRequestID(rid)
		}
	}

	if userID := ctx.Value("user_id"); userID != nil {
		if uid, ok := userID.(string); ok {
			logger = logger.WithUserID(uid)
		}
	}

	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["action"] = action

	logger.InfoWithMetadata("Admin action performed", metadata)
}

// LogError logs system errors
func (m *MantisDBLogger) LogError(component string, err error, metadata map[string]interface{}) {
	logger := m.baseLogger.WithComponent(component)

	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["error"] = err.Error()

	logger.ErrorWithMetadata("System error occurred", metadata)
}

// LogBackupOperation logs backup operations
func (m *MantisDBLogger) LogBackupOperation(operation string, status string, metadata map[string]interface{}) {
	logger := m.baseLogger.WithComponent("backup")

	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["operation"] = operation
	metadata["status"] = status

	level := INFO
	message := "Backup operation"

	if status == "failed" {
		level = ERROR
		message = "Backup operation failed"
	} else if status == "completed" {
		message = "Backup operation completed"
	}

	switch level {
	case ERROR:
		logger.ErrorWithMetadata(message, metadata)
	default:
		logger.InfoWithMetadata(message, metadata)
	}
}

// GetLogManager returns the log manager for advanced operations
func (m *MantisDBLogger) GetLogManager() *LogManager {
	return m.manager
}

// Close closes all logging resources
func (m *MantisDBLogger) Close() error {
	return m.baseLogger.Close()
}

// ExampleUsage demonstrates how to use MantisDBLogger
func ExampleUsage() {
	// Initialize logger
	logger, err := NewMantisDBLogger()
	if err != nil {
		panic(err)
	}
	defer logger.Close()

	// Log database startup
	logger.LogDatabaseStartup("1.0.0", map[string]interface{}{
		"port":     8080,
		"data_dir": "/var/lib/mantisdb",
	})

	// Create context with request information
	ctx := context.Background()
	ctx = context.WithValue(ctx, "request_id", "req_12345")
	ctx = context.WithValue(ctx, "user_id", "user_789")

	// Log query execution
	start := time.Now()
	// ... execute query ...
	duration := time.Since(start)

	logger.LogQuery(ctx, "SELECT * FROM users WHERE active = true", duration, map[string]interface{}{
		"rows_returned": 42,
		"cache_hit":     true,
	})

	// Log storage operation
	logger.LogStorageOperation("write", 15*time.Millisecond, map[string]interface{}{
		"bytes_written": 1024,
		"file":          "data_001.db",
	})

	// Log WAL operation
	logger.LogWALOperation("flush", 10, 2048)

	// Log admin action
	logger.LogAdminAction(ctx, "backup_create", map[string]interface{}{
		"backup_type": "full",
		"destination": "s3://backups/mantisdb",
	})

	// Log backup operation
	logger.LogBackupOperation("create", "completed", map[string]interface{}{
		"backup_id":   "backup_001",
		"size_bytes":  1024 * 1024 * 100, // 100MB
		"duration_ms": 5000,
	})

	// Log error
	logger.LogError("storage", fmt.Errorf("disk space low"), map[string]interface{}{
		"available_bytes": 1024 * 1024 * 10, // 10MB
		"threshold":       1024 * 1024 * 50, // 50MB
	})
}
