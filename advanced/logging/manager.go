package logging

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// LogFilter defines criteria for filtering log entries
type LogFilter struct {
	Level       *LogLevel  `json:"level,omitempty"`
	Component   string     `json:"component,omitempty"`
	RequestID   string     `json:"request_id,omitempty"`
	UserID      string     `json:"user_id,omitempty"`
	StartTime   *time.Time `json:"start_time,omitempty"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	SearchQuery string     `json:"search_query,omitempty"`
	Limit       int        `json:"limit,omitempty"`
	Offset      int        `json:"offset,omitempty"`
}

// LogSearchResult represents search results with pagination
type LogSearchResult struct {
	Entries    []*LogEntry `json:"entries"`
	Total      int         `json:"total"`
	HasMore    bool        `json:"has_more"`
	NextOffset int         `json:"next_offset,omitempty"`
}

// LogStream represents a real-time log stream
type LogStream struct {
	ID       string    `json:"id"`
	Filter   LogFilter `json:"filter"`
	Channel  chan *LogEntry
	ctx      context.Context
	cancel   context.CancelFunc
	lastSeen time.Time
}

// LogManager provides log management capabilities
type LogManager struct {
	logDir      string
	streams     map[string]*LogStream
	streamMutex sync.RWMutex
	logger      *StructuredLogger
}

// LogManagerConfig holds configuration for the log manager
type LogManagerConfig struct {
	LogDir string
	Logger *StructuredLogger
}

// NewLogManager creates a new log manager
func NewLogManager(config LogManagerConfig) *LogManager {
	return &LogManager{
		logDir:  config.LogDir,
		streams: make(map[string]*LogStream),
		logger:  config.Logger,
	}
}

// SearchLogs searches for log entries based on the provided filter
func (lm *LogManager) SearchLogs(filter LogFilter) (*LogSearchResult, error) {
	if filter.Limit == 0 {
		filter.Limit = 100 // Default limit
	}

	// Get all log files
	files, err := lm.getLogFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to get log files: %w", err)
	}

	var allEntries []*LogEntry

	// Read and filter entries from all files
	for _, file := range files {
		entries, err := lm.readLogFile(file, filter)
		if err != nil {
			lm.logger.WarnWithMetadata("Failed to read log file", map[string]interface{}{
				"file":  file,
				"error": err.Error(),
			})
			continue
		}
		allEntries = append(allEntries, entries...)
	}

	// Sort by timestamp (newest first)
	sort.Slice(allEntries, func(i, j int) bool {
		return allEntries[i].Timestamp.After(allEntries[j].Timestamp)
	})

	// Apply pagination
	total := len(allEntries)
	start := filter.Offset
	end := start + filter.Limit

	if start >= total {
		return &LogSearchResult{
			Entries: []*LogEntry{},
			Total:   total,
			HasMore: false,
		}, nil
	}

	if end > total {
		end = total
	}

	result := &LogSearchResult{
		Entries: allEntries[start:end],
		Total:   total,
		HasMore: end < total,
	}

	if result.HasMore {
		result.NextOffset = end
	}

	return result, nil
}

// getLogFiles returns all log files sorted by modification time
func (lm *LogManager) getLogFiles() ([]string, error) {
	entries, err := os.ReadDir(lm.logDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read log directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if strings.HasSuffix(name, ".log") || strings.Contains(name, ".log.") {
			files = append(files, filepath.Join(lm.logDir, name))
		}
	}

	// Sort by modification time (newest first)
	sort.Slice(files, func(i, j int) bool {
		info1, err1 := os.Stat(files[i])
		info2, err2 := os.Stat(files[j])
		if err1 != nil || err2 != nil {
			return false
		}
		return info1.ModTime().After(info2.ModTime())
	})

	return files, nil
}

// readLogFile reads and filters log entries from a file
func (lm *LogManager) readLogFile(filename string, filter LogFilter) ([]*LogEntry, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	var entries []*LogEntry
	scanner := bufio.NewScanner(file)

	// Compile search regex if provided
	var searchRegex *regexp.Regexp
	if filter.SearchQuery != "" {
		searchRegex, err = regexp.Compile("(?i)" + regexp.QuoteMeta(filter.SearchQuery))
		if err != nil {
			// If regex compilation fails, use simple string matching
			searchRegex = nil
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// Skip malformed entries
			continue
		}

		// Apply filters
		if !lm.matchesFilter(&entry, filter, searchRegex) {
			continue
		}

		entries = append(entries, &entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading log file: %w", err)
	}

	return entries, nil
}

// matchesFilter checks if a log entry matches the given filter
func (lm *LogManager) matchesFilter(entry *LogEntry, filter LogFilter, searchRegex *regexp.Regexp) bool {
	// Level filter
	if filter.Level != nil {
		entryLevel := parseLogLevel(entry.Level)
		if entryLevel < *filter.Level {
			return false
		}
	}

	// Component filter
	if filter.Component != "" && !strings.Contains(strings.ToLower(entry.Component), strings.ToLower(filter.Component)) {
		return false
	}

	// Request ID filter
	if filter.RequestID != "" && entry.RequestID != filter.RequestID {
		return false
	}

	// User ID filter
	if filter.UserID != "" && entry.UserID != filter.UserID {
		return false
	}

	// Time range filter
	if filter.StartTime != nil && entry.Timestamp.Before(*filter.StartTime) {
		return false
	}

	if filter.EndTime != nil && entry.Timestamp.After(*filter.EndTime) {
		return false
	}

	// Search query filter
	if filter.SearchQuery != "" {
		searchText := strings.ToLower(entry.Message)
		if entry.Query != "" {
			searchText += " " + strings.ToLower(entry.Query)
		}

		// Add metadata to search text
		if entry.Metadata != nil {
			for _, v := range entry.Metadata {
				if str, ok := v.(string); ok {
					searchText += " " + strings.ToLower(str)
				}
			}
		}

		if searchRegex != nil {
			if !searchRegex.MatchString(searchText) {
				return false
			}
		} else {
			if !strings.Contains(searchText, strings.ToLower(filter.SearchQuery)) {
				return false
			}
		}
	}

	return true
}

// parseLogLevel parses a log level string
func parseLogLevel(level string) LogLevel {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN":
		return WARN
	case "ERROR":
		return ERROR
	case "FATAL":
		return FATAL
	default:
		return INFO
	}
}

// CreateStream creates a new real-time log stream
func (lm *LogManager) CreateStream(ctx context.Context, filter LogFilter) (*LogStream, error) {
	streamCtx, cancel := context.WithCancel(ctx)

	stream := &LogStream{
		ID:       generateStreamID(),
		Filter:   filter,
		Channel:  make(chan *LogEntry, 100), // Buffered channel
		ctx:      streamCtx,
		cancel:   cancel,
		lastSeen: time.Now(),
	}

	lm.streamMutex.Lock()
	lm.streams[stream.ID] = stream
	lm.streamMutex.Unlock()

	// Start streaming goroutine
	go lm.streamLogs(stream)

	return stream, nil
}

// streamLogs streams log entries to a stream channel
func (lm *LogManager) streamLogs(stream *LogStream) {
	defer func() {
		close(stream.Channel)
		lm.streamMutex.Lock()
		delete(lm.streams, stream.ID)
		lm.streamMutex.Unlock()
	}()

	ticker := time.NewTicker(1 * time.Second) // Check for new logs every second
	defer ticker.Stop()

	for {
		select {
		case <-stream.ctx.Done():
			return
		case <-ticker.C:
			// Check for new log entries
			if err := lm.checkForNewLogs(stream); err != nil {
				lm.logger.ErrorWithMetadata("Failed to check for new logs", map[string]interface{}{
					"stream_id": stream.ID,
					"error":     err.Error(),
				})
			}
		}
	}
}

// checkForNewLogs checks for new log entries and sends them to the stream
func (lm *LogManager) checkForNewLogs(stream *LogStream) error {
	// Get current log file
	files, err := lm.getLogFiles()
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return nil
	}

	// Read from the most recent log file
	currentFile := files[0]

	file, err := os.Open(currentFile)
	if err != nil {
		return err
	}
	defer file.Close()

	// Seek to end and read backwards to find new entries
	entries, err := lm.readNewEntries(file, stream.lastSeen)
	if err != nil {
		return err
	}

	// Filter and send entries
	for _, entry := range entries {
		if lm.matchesFilter(entry, stream.Filter, nil) {
			select {
			case stream.Channel <- entry:
				stream.lastSeen = entry.Timestamp
			case <-stream.ctx.Done():
				return nil
			default:
				// Channel is full, skip this entry
			}
		}
	}

	return nil
}

// readNewEntries reads log entries newer than the given timestamp
func (lm *LogManager) readNewEntries(file *os.File, since time.Time) ([]*LogEntry, error) {
	var entries []*LogEntry
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		if entry.Timestamp.After(since) {
			entries = append(entries, &entry)
		}
	}

	return entries, scanner.Err()
}

// CloseStream closes a log stream
func (lm *LogManager) CloseStream(streamID string) error {
	lm.streamMutex.Lock()
	defer lm.streamMutex.Unlock()

	stream, exists := lm.streams[streamID]
	if !exists {
		return fmt.Errorf("stream not found: %s", streamID)
	}

	stream.cancel()
	return nil
}

// GetActiveStreams returns information about active streams
func (lm *LogManager) GetActiveStreams() []map[string]interface{} {
	lm.streamMutex.RLock()
	defer lm.streamMutex.RUnlock()

	var streams []map[string]interface{}
	for _, stream := range lm.streams {
		streams = append(streams, map[string]interface{}{
			"id":        stream.ID,
			"filter":    stream.Filter,
			"last_seen": stream.lastSeen,
		})
	}

	return streams
}

// generateStreamID generates a unique stream ID
func generateStreamID() string {
	return fmt.Sprintf("stream_%d", time.Now().UnixNano())
}

// GetLogStats returns statistics about log files
func (lm *LogManager) GetLogStats() (map[string]interface{}, error) {
	files, err := lm.getLogFiles()
	if err != nil {
		return nil, err
	}

	var totalSize int64
	var totalEntries int

	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}

		totalSize += info.Size()

		// Count entries in file
		count, err := lm.countEntriesInFile(file)
		if err != nil {
			continue
		}

		totalEntries += count
	}

	return map[string]interface{}{
		"total_files":   len(files),
		"total_size":    totalSize,
		"total_entries": totalEntries,
		"files":         files,
	}, nil
}

// countEntriesInFile counts the number of log entries in a file
func (lm *LogManager) countEntriesInFile(filename string) (int, error) {
	file, err := os.Open(filename)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	count := 0
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			count++
		}
	}

	return count, scanner.Err()
}

// TailLogs returns the last N log entries
func (lm *LogManager) TailLogs(n int, filter LogFilter) ([]*LogEntry, error) {
	filter.Limit = n
	filter.Offset = 0

	result, err := lm.SearchLogs(filter)
	if err != nil {
		return nil, err
	}

	return result.Entries, nil
}
