package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"time"
)

// LogLevel represents the severity level of a log entry
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp  time.Time              `json:"timestamp"`
	Level      string                 `json:"level"`
	Component  string                 `json:"component"`
	RequestID  string                 `json:"request_id,omitempty"`
	UserID     string                 `json:"user_id,omitempty"`
	Message    string                 `json:"message"`
	DurationMs int64                  `json:"duration_ms,omitempty"`
	Query      string                 `json:"query,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	File       string                 `json:"file,omitempty"`
	Line       int                    `json:"line,omitempty"`
	Function   string                 `json:"function,omitempty"`
}

// LogOutput defines where logs should be written
type LogOutput interface {
	Write(entry *LogEntry) error
	Close() error
}

// JSONOutput writes log entries as JSON to an io.Writer
type JSONOutput struct {
	writer io.Writer
	mutex  sync.Mutex
}

// NewJSONOutput creates a new JSON output writer
func NewJSONOutput(writer io.Writer) *JSONOutput {
	return &JSONOutput{
		writer: writer,
	}
}

// Write writes a log entry as JSON
func (j *JSONOutput) Write(entry *LogEntry) error {
	j.mutex.Lock()
	defer j.mutex.Unlock()

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %w", err)
	}

	_, err = j.writer.Write(append(data, '\n'))
	return err
}

// Close closes the output (no-op for basic JSON output)
func (j *JSONOutput) Close() error {
	return nil
}

// StructuredLogger provides JSON-formatted structured logging
type StructuredLogger struct {
	level     LogLevel
	outputs   []LogOutput
	context   map[string]interface{}
	mutex     sync.RWMutex
	component string
}

// Config holds configuration for the structured logger
type Config struct {
	Level     LogLevel
	Component string
	Outputs   []LogOutput
	Context   map[string]interface{}
}

// NewStructuredLogger creates a new structured logger
func NewStructuredLogger(config Config) *StructuredLogger {
	if len(config.Outputs) == 0 {
		config.Outputs = []LogOutput{NewJSONOutput(os.Stdout)}
	}

	if config.Context == nil {
		config.Context = make(map[string]interface{})
	}

	return &StructuredLogger{
		level:     config.Level,
		outputs:   config.Outputs,
		context:   config.Context,
		component: config.Component,
	}
}

// WithContext returns a new logger with additional context
func (s *StructuredLogger) WithContext(key string, value interface{}) *StructuredLogger {
	s.mutex.RLock()
	newContext := make(map[string]interface{})
	for k, v := range s.context {
		newContext[k] = v
	}
	s.mutex.RUnlock()

	newContext[key] = value

	return &StructuredLogger{
		level:     s.level,
		outputs:   s.outputs,
		context:   newContext,
		component: s.component,
	}
}

// WithRequestID returns a new logger with request ID context
func (s *StructuredLogger) WithRequestID(requestID string) *StructuredLogger {
	return s.WithContext("request_id", requestID)
}

// WithUserID returns a new logger with user ID context
func (s *StructuredLogger) WithUserID(userID string) *StructuredLogger {
	return s.WithContext("user_id", userID)
}

// WithComponent returns a new logger with component context
func (s *StructuredLogger) WithComponent(component string) *StructuredLogger {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return &StructuredLogger{
		level:     s.level,
		outputs:   s.outputs,
		context:   s.context,
		component: component,
	}
}

// log writes a log entry at the specified level
func (s *StructuredLogger) log(level LogLevel, message string, metadata map[string]interface{}) {
	if level < s.level {
		return
	}

	// Get caller information
	_, file, line, ok := runtime.Caller(2)
	var function string
	if ok {
		if pc, _, _, ok := runtime.Caller(2); ok {
			if fn := runtime.FuncForPC(pc); fn != nil {
				function = fn.Name()
			}
		}
	}

	s.mutex.RLock()
	entry := &LogEntry{
		Timestamp: time.Now().UTC(),
		Level:     level.String(),
		Component: s.component,
		Message:   message,
		Metadata:  metadata,
	}

	// Add caller information if available
	if ok {
		entry.File = file
		entry.Line = line
		entry.Function = function
	}

	// Add context information
	if requestID, exists := s.context["request_id"]; exists {
		if rid, ok := requestID.(string); ok {
			entry.RequestID = rid
		}
	}

	if userID, exists := s.context["user_id"]; exists {
		if uid, ok := userID.(string); ok {
			entry.UserID = uid
		}
	}

	// Add other context to metadata
	if entry.Metadata == nil {
		entry.Metadata = make(map[string]interface{})
	}

	for k, v := range s.context {
		if k != "request_id" && k != "user_id" {
			entry.Metadata[k] = v
		}
	}
	s.mutex.RUnlock()

	// Write to all outputs
	for _, output := range s.outputs {
		if err := output.Write(entry); err != nil {
			// Fallback to stderr if output fails
			fmt.Fprintf(os.Stderr, "Failed to write log entry: %v\n", err)
		}
	}
}

// Debug logs a debug message
func (s *StructuredLogger) Debug(message string) {
	s.log(DEBUG, message, nil)
}

// DebugWithMetadata logs a debug message with metadata
func (s *StructuredLogger) DebugWithMetadata(message string, metadata map[string]interface{}) {
	s.log(DEBUG, message, metadata)
}

// Info logs an info message
func (s *StructuredLogger) Info(message string) {
	s.log(INFO, message, nil)
}

// InfoWithMetadata logs an info message with metadata
func (s *StructuredLogger) InfoWithMetadata(message string, metadata map[string]interface{}) {
	s.log(INFO, message, metadata)
}

// Warn logs a warning message
func (s *StructuredLogger) Warn(message string) {
	s.log(WARN, message, nil)
}

// WarnWithMetadata logs a warning message with metadata
func (s *StructuredLogger) WarnWithMetadata(message string, metadata map[string]interface{}) {
	s.log(WARN, message, metadata)
}

// Error logs an error message
func (s *StructuredLogger) Error(message string) {
	s.log(ERROR, message, nil)
}

// ErrorWithMetadata logs an error message with metadata
func (s *StructuredLogger) ErrorWithMetadata(message string, metadata map[string]interface{}) {
	s.log(ERROR, message, metadata)
}

// Fatal logs a fatal message and exits
func (s *StructuredLogger) Fatal(message string) {
	s.log(FATAL, message, nil)
	os.Exit(1)
}

// FatalWithMetadata logs a fatal message with metadata and exits
func (s *StructuredLogger) FatalWithMetadata(message string, metadata map[string]interface{}) {
	s.log(FATAL, message, metadata)
	os.Exit(1)
}

// LogQuery logs a query execution with timing information
func (s *StructuredLogger) LogQuery(query string, duration time.Duration, metadata map[string]interface{}) {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["query"] = query
	metadata["duration_ms"] = duration.Milliseconds()

	s.log(INFO, "Query executed", metadata)
}

// LogQueryError logs a query execution error
func (s *StructuredLogger) LogQueryError(query string, err error, metadata map[string]interface{}) {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["query"] = query
	metadata["error"] = err.Error()

	s.log(ERROR, "Query execution failed", metadata)
}

// Close closes all outputs
func (s *StructuredLogger) Close() error {
	var lastErr error
	for _, output := range s.outputs {
		if err := output.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// SetLevel sets the minimum log level
func (s *StructuredLogger) SetLevel(level LogLevel) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.level = level
}

// GetLevel returns the current log level
func (s *StructuredLogger) GetLevel() LogLevel {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.level
}

// ContextLogger provides context-aware logging utilities
type ContextLogger struct {
	logger *StructuredLogger
}

// NewContextLogger creates a new context logger
func NewContextLogger(logger *StructuredLogger) *ContextLogger {
	return &ContextLogger{logger: logger}
}

// FromContext extracts logger context from a context.Context
func (c *ContextLogger) FromContext(ctx context.Context) *StructuredLogger {
	if requestID := ctx.Value("request_id"); requestID != nil {
		if rid, ok := requestID.(string); ok {
			c.logger = c.logger.WithRequestID(rid)
		}
	}

	if userID := ctx.Value("user_id"); userID != nil {
		if uid, ok := userID.(string); ok {
			c.logger = c.logger.WithUserID(uid)
		}
	}

	return c.logger
}
