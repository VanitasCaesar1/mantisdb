package monitoring

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogLevel represents the severity of a log entry
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelFatal
)

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     LogLevel               `json:"level"`
	Component string                 `json:"component"`
	Operation string                 `json:"operation"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	TraceID   string                 `json:"trace_id,omitempty"`
	UserID    string                 `json:"user_id,omitempty"`
	SessionID string                 `json:"session_id,omitempty"`
}

// Logger provides structured logging capabilities
type Logger struct {
	level     LogLevel
	outputs   []io.Writer
	formatter LogFormatter
	mutex     sync.RWMutex

	// Context fields that are added to all log entries
	contextFields map[string]interface{}
}

// LogFormatter formats log entries for output
type LogFormatter interface {
	Format(entry LogEntry) ([]byte, error)
}

// NewLogger creates a new logger
func NewLogger(level LogLevel) *Logger {
	return &Logger{
		level:         level,
		outputs:       []io.Writer{os.Stdout},
		formatter:     &JSONFormatter{},
		contextFields: make(map[string]interface{}),
	}
}

// SetLevel sets the minimum log level
func (l *Logger) SetLevel(level LogLevel) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.level = level
}

// AddOutput adds an output writer
func (l *Logger) AddOutput(writer io.Writer) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.outputs = append(l.outputs, writer)
}

// SetFormatter sets the log formatter
func (l *Logger) SetFormatter(formatter LogFormatter) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.formatter = formatter
}

// WithField adds a field to the logger context
func (l *Logger) WithField(key string, value interface{}) *Logger {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	newLogger := &Logger{
		level:         l.level,
		outputs:       l.outputs,
		formatter:     l.formatter,
		contextFields: make(map[string]interface{}),
	}

	// Copy existing context fields
	for k, v := range l.contextFields {
		newLogger.contextFields[k] = v
	}

	// Add new field
	newLogger.contextFields[key] = value

	return newLogger
}

// WithFields adds multiple fields to the logger context
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	newLogger := &Logger{
		level:         l.level,
		outputs:       l.outputs,
		formatter:     l.formatter,
		contextFields: make(map[string]interface{}),
	}

	// Copy existing context fields
	for k, v := range l.contextFields {
		newLogger.contextFields[k] = v
	}

	// Add new fields
	for k, v := range fields {
		newLogger.contextFields[k] = v
	}

	return newLogger
}

// Log logs an entry at the specified level
func (l *Logger) Log(level LogLevel, component, operation, message string, fields map[string]interface{}) {
	if level < l.level {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Component: component,
		Operation: operation,
		Message:   message,
		Fields:    make(map[string]interface{}),
	}

	// Add context fields
	l.mutex.RLock()
	for k, v := range l.contextFields {
		entry.Fields[k] = v
	}
	l.mutex.RUnlock()

	// Add provided fields
	if fields != nil {
		for k, v := range fields {
			entry.Fields[k] = v
		}
	}

	// Format and write to outputs
	l.mutex.RLock()
	formatter := l.formatter
	outputs := l.outputs
	l.mutex.RUnlock()

	data, err := formatter.Format(entry)
	if err != nil {
		log.Printf("Failed to format log entry: %v", err)
		return
	}

	for _, output := range outputs {
		if _, err := output.Write(data); err != nil {
			log.Printf("Failed to write log entry: %v", err)
		}
	}
}

// Debug logs a debug message
func (l *Logger) Debug(component, operation, message string, fields map[string]interface{}) {
	l.Log(LogLevelDebug, component, operation, message, fields)
}

// Info logs an info message
func (l *Logger) Info(component, operation, message string, fields map[string]interface{}) {
	l.Log(LogLevelInfo, component, operation, message, fields)
}

// Warn logs a warning message
func (l *Logger) Warn(component, operation, message string, fields map[string]interface{}) {
	l.Log(LogLevelWarn, component, operation, message, fields)
}

// Error logs an error message
func (l *Logger) Error(component, operation, message string, fields map[string]interface{}) {
	l.Log(LogLevelError, component, operation, message, fields)
}

// Fatal logs a fatal message
func (l *Logger) Fatal(component, operation, message string, fields map[string]interface{}) {
	l.Log(LogLevelFatal, component, operation, message, fields)
}

// JSONFormatter formats log entries as JSON
type JSONFormatter struct{}

// Format formats a log entry as JSON
func (f *JSONFormatter) Format(entry LogEntry) ([]byte, error) {
	// Convert log level to string
	levelStr := map[LogLevel]string{
		LogLevelDebug: "DEBUG",
		LogLevelInfo:  "INFO",
		LogLevelWarn:  "WARN",
		LogLevelError: "ERROR",
		LogLevelFatal: "FATAL",
	}[entry.Level]

	// Create output structure
	output := map[string]interface{}{
		"timestamp": entry.Timestamp.Format(time.RFC3339Nano),
		"level":     levelStr,
		"component": entry.Component,
		"operation": entry.Operation,
		"message":   entry.Message,
	}

	// Add fields
	if entry.Fields != nil {
		for k, v := range entry.Fields {
			output[k] = v
		}
	}

	// Add trace information if present
	if entry.TraceID != "" {
		output["trace_id"] = entry.TraceID
	}
	if entry.UserID != "" {
		output["user_id"] = entry.UserID
	}
	if entry.SessionID != "" {
		output["session_id"] = entry.SessionID
	}

	data, err := json.Marshal(output)
	if err != nil {
		return nil, err
	}

	// Add newline
	data = append(data, '\n')
	return data, nil
}

// TextFormatter formats log entries as human-readable text
type TextFormatter struct{}

// Format formats a log entry as text
func (f *TextFormatter) Format(entry LogEntry) ([]byte, error) {
	levelStr := map[LogLevel]string{
		LogLevelDebug: "DEBUG",
		LogLevelInfo:  "INFO",
		LogLevelWarn:  "WARN",
		LogLevelError: "ERROR",
		LogLevelFatal: "FATAL",
	}[entry.Level]

	output := fmt.Sprintf("[%s] %s %s/%s: %s",
		entry.Timestamp.Format("2006-01-02 15:04:05.000"),
		levelStr,
		entry.Component,
		entry.Operation,
		entry.Message)

	// Add fields
	if entry.Fields != nil && len(entry.Fields) > 0 {
		output += " |"
		for k, v := range entry.Fields {
			output += fmt.Sprintf(" %s=%v", k, v)
		}
	}

	output += "\n"
	return []byte(output), nil
}

// FileRotatingWriter provides log file rotation
type FileRotatingWriter struct {
	filename    string
	maxSize     int64
	maxFiles    int
	currentFile *os.File
	currentSize int64
	mutex       sync.Mutex
}

// NewFileRotatingWriter creates a new rotating file writer
func NewFileRotatingWriter(filename string, maxSize int64, maxFiles int) (*FileRotatingWriter, error) {
	// Ensure directory exists
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	writer := &FileRotatingWriter{
		filename: filename,
		maxSize:  maxSize,
		maxFiles: maxFiles,
	}

	if err := writer.openFile(); err != nil {
		return nil, err
	}

	return writer, nil
}

// Write writes data to the file, rotating if necessary
func (w *FileRotatingWriter) Write(data []byte) (int, error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	// Check if rotation is needed
	if w.currentSize+int64(len(data)) > w.maxSize {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}

	n, err := w.currentFile.Write(data)
	if err != nil {
		return n, err
	}

	w.currentSize += int64(n)
	return n, nil
}

// Close closes the current file
func (w *FileRotatingWriter) Close() error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if w.currentFile != nil {
		return w.currentFile.Close()
	}
	return nil
}

// openFile opens the current log file
func (w *FileRotatingWriter) openFile() error {
	file, err := os.OpenFile(w.filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	// Get current file size
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return err
	}

	w.currentFile = file
	w.currentSize = info.Size()
	return nil
}

// rotate rotates the log files
func (w *FileRotatingWriter) rotate() error {
	// Close current file
	if w.currentFile != nil {
		w.currentFile.Close()
	}

	// Rotate existing files
	for i := w.maxFiles - 1; i > 0; i-- {
		oldName := fmt.Sprintf("%s.%d", w.filename, i)
		newName := fmt.Sprintf("%s.%d", w.filename, i+1)

		if i == w.maxFiles-1 {
			// Remove the oldest file
			os.Remove(newName)
		}

		// Rename if file exists
		if _, err := os.Stat(oldName); err == nil {
			os.Rename(oldName, newName)
		}
	}

	// Move current file to .1
	if _, err := os.Stat(w.filename); err == nil {
		os.Rename(w.filename, w.filename+".1")
	}

	// Open new file
	return w.openFile()
}

// OperationalLogger provides high-level logging for database operations
type OperationalLogger struct {
	logger *Logger
}

// NewOperationalLogger creates a new operational logger
func NewOperationalLogger() *OperationalLogger {
	logger := NewLogger(LogLevelInfo)

	// Set up file rotation for operational logs
	if rotatingWriter, err := NewFileRotatingWriter("logs/mantisdb-operations.log", 100*1024*1024, 10); err == nil {
		logger.AddOutput(rotatingWriter)
	}

	return &OperationalLogger{
		logger: logger,
	}
}

// LogWALOperation logs a WAL operation
func (ol *OperationalLogger) LogWALOperation(operation string, lsn uint64, txnID uint64, success bool, duration time.Duration, details map[string]interface{}) {
	fields := map[string]interface{}{
		"lsn":      lsn,
		"txn_id":   txnID,
		"success":  success,
		"duration": duration.String(),
	}

	// Add additional details
	if details != nil {
		for k, v := range details {
			fields[k] = v
		}
	}

	level := LogLevelInfo
	message := fmt.Sprintf("WAL operation %s completed", operation)

	if !success {
		level = LogLevelError
		message = fmt.Sprintf("WAL operation %s failed", operation)
	}

	ol.logger.Log(level, "wal", operation, message, fields)
}

// LogTransactionOperation logs a transaction operation
func (ol *OperationalLogger) LogTransactionOperation(operation string, txnID uint64, success bool, duration time.Duration, details map[string]interface{}) {
	fields := map[string]interface{}{
		"txn_id":   txnID,
		"success":  success,
		"duration": duration.String(),
	}

	// Add additional details
	if details != nil {
		for k, v := range details {
			fields[k] = v
		}
	}

	level := LogLevelInfo
	message := fmt.Sprintf("Transaction operation %s completed", operation)

	if !success {
		level = LogLevelError
		message = fmt.Sprintf("Transaction operation %s failed", operation)
	}

	ol.logger.Log(level, "transaction", operation, message, fields)
}

// LogRecoveryOperation logs a recovery operation
func (ol *OperationalLogger) LogRecoveryOperation(operation string, success bool, duration time.Duration, details map[string]interface{}) {
	fields := map[string]interface{}{
		"success":  success,
		"duration": duration.String(),
	}

	// Add additional details
	if details != nil {
		for k, v := range details {
			fields[k] = v
		}
	}

	level := LogLevelInfo
	message := fmt.Sprintf("Recovery operation %s completed", operation)

	if !success {
		level = LogLevelError
		message = fmt.Sprintf("Recovery operation %s failed", operation)
	}

	ol.logger.Log(level, "recovery", operation, message, fields)
}

// LogErrorEvent logs an error event
func (ol *OperationalLogger) LogErrorEvent(component, operation, errorType string, err error, details map[string]interface{}) {
	fields := map[string]interface{}{
		"error_type": errorType,
		"error":      err.Error(),
	}

	// Add additional details
	if details != nil {
		for k, v := range details {
			fields[k] = v
		}
	}

	message := fmt.Sprintf("Error in %s operation: %s", operation, err.Error())
	ol.logger.Log(LogLevelError, component, operation, message, fields)
}

// LogSystemEvent logs a system event
func (ol *OperationalLogger) LogSystemEvent(event string, details map[string]interface{}) {
	ol.logger.Log(LogLevelInfo, "system", event, fmt.Sprintf("System event: %s", event), details)
}

// LogPerformanceMetric logs a performance metric
func (ol *OperationalLogger) LogPerformanceMetric(metric string, value interface{}, unit string, details map[string]interface{}) {
	fields := map[string]interface{}{
		"metric": metric,
		"value":  value,
		"unit":   unit,
	}

	// Add additional details
	if details != nil {
		for k, v := range details {
			fields[k] = v
		}
	}

	message := fmt.Sprintf("Performance metric: %s = %v %s", metric, value, unit)
	ol.logger.Log(LogLevelInfo, "performance", "metric", message, fields)
}
