package errors

import (
	"crypto/md5"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CorruptionDetector detects and handles data corruption
type CorruptionDetector struct {
	mu             sync.RWMutex
	config         *CorruptionDetectorConfig
	checksumCache  map[string]*ChecksumInfo
	corruptionLog  []*CorruptionEvent
	alertCallbacks []CorruptionAlertCallback
}

// CorruptionDetectorConfig contains configuration for corruption detection
type CorruptionDetectorConfig struct {
	EnableRealTimeChecking bool              `json:"enable_realtime_checking"`
	EnableBackgroundScan   bool              `json:"enable_background_scan"`
	ScanInterval           time.Duration     `json:"scan_interval"`
	ChecksumAlgorithm      ChecksumAlgorithm `json:"checksum_algorithm"`
	MaxCorruptionEvents    int               `json:"max_corruption_events"`
	EnableAutoIsolation    bool              `json:"enable_auto_isolation"`
	EnableAutoRecovery     bool              `json:"enable_auto_recovery"`
	CorruptionThreshold    int               `json:"corruption_threshold"` // Max corruptions before emergency action
}

// ChecksumAlgorithm represents different checksum algorithms
type ChecksumAlgorithm int

const (
	ChecksumCRC32 ChecksumAlgorithm = iota
	ChecksumMD5
	ChecksumSHA256
)

func (a ChecksumAlgorithm) String() string {
	switch a {
	case ChecksumCRC32:
		return "CRC32"
	case ChecksumMD5:
		return "MD5"
	case ChecksumSHA256:
		return "SHA256"
	default:
		return "UNKNOWN"
	}
}

// ChecksumInfo contains checksum information for a file or data block
type ChecksumInfo struct {
	Path         string            `json:"path"`
	Algorithm    ChecksumAlgorithm `json:"algorithm"`
	Checksum     string            `json:"checksum"`
	Size         int64             `json:"size"`
	LastChecked  time.Time         `json:"last_checked"`
	LastModified time.Time         `json:"last_modified"`
	IsValid      bool              `json:"is_valid"`
}

// CorruptionEvent represents a detected corruption event
type CorruptionEvent struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	Location    DataLocation           `json:"location"`
	Type        CorruptionType         `json:"type"`
	Severity    CorruptionSeverity     `json:"severity"`
	Description string                 `json:"description"`
	Expected    string                 `json:"expected,omitempty"`
	Actual      string                 `json:"actual,omitempty"`
	Isolated    bool                   `json:"isolated"`
	Recovered   bool                   `json:"recovered"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// CorruptionType represents different types of corruption
type CorruptionType int

const (
	CorruptionTypeChecksum CorruptionType = iota
	CorruptionTypeStructural
	CorruptionTypeLogical
	CorruptionTypePartial
	CorruptionTypeComplete
)

func (t CorruptionType) String() string {
	switch t {
	case CorruptionTypeChecksum:
		return "CHECKSUM"
	case CorruptionTypeStructural:
		return "STRUCTURAL"
	case CorruptionTypeLogical:
		return "LOGICAL"
	case CorruptionTypePartial:
		return "PARTIAL"
	case CorruptionTypeComplete:
		return "COMPLETE"
	default:
		return "UNKNOWN"
	}
}

// CorruptionSeverity represents the severity of corruption
type CorruptionSeverity int

const (
	CorruptionSeverityLow CorruptionSeverity = iota
	CorruptionSeverityMedium
	CorruptionSeverityHigh
	CorruptionSeverityCritical
)

func (s CorruptionSeverity) String() string {
	switch s {
	case CorruptionSeverityLow:
		return "LOW"
	case CorruptionSeverityMedium:
		return "MEDIUM"
	case CorruptionSeverityHigh:
		return "HIGH"
	case CorruptionSeverityCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// CorruptionAlertCallback is called when corruption is detected
type CorruptionAlertCallback func(event *CorruptionEvent)

// CorruptionScanResult contains the result of a corruption scan
type CorruptionScanResult struct {
	StartTime        time.Time          `json:"start_time"`
	EndTime          time.Time          `json:"end_time"`
	Duration         time.Duration      `json:"duration"`
	FilesScanned     int                `json:"files_scanned"`
	CorruptionsFound int                `json:"corruptions_found"`
	Events           []*CorruptionEvent `json:"events"`
	Success          bool               `json:"success"`
	Error            error              `json:"error,omitempty"`
}

// NewCorruptionDetector creates a new corruption detector
func NewCorruptionDetector(config *CorruptionDetectorConfig) *CorruptionDetector {
	if config == nil {
		config = &CorruptionDetectorConfig{
			EnableRealTimeChecking: true,
			EnableBackgroundScan:   true,
			ScanInterval:           1 * time.Hour,
			ChecksumAlgorithm:      ChecksumCRC32,
			MaxCorruptionEvents:    1000,
			EnableAutoIsolation:    true,
			EnableAutoRecovery:     false,
			CorruptionThreshold:    10,
		}
	}

	return &CorruptionDetector{
		config:         config,
		checksumCache:  make(map[string]*ChecksumInfo),
		corruptionLog:  make([]*CorruptionEvent, 0),
		alertCallbacks: make([]CorruptionAlertCallback, 0),
	}
}

// AddAlertCallback adds a callback function for corruption alerts
func (d *CorruptionDetector) AddAlertCallback(callback CorruptionAlertCallback) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.alertCallbacks = append(d.alertCallbacks, callback)
}

// CalculateChecksum calculates checksum for data
func (d *CorruptionDetector) CalculateChecksum(data []byte) string {
	switch d.config.ChecksumAlgorithm {
	case ChecksumCRC32:
		return fmt.Sprintf("%08x", crc32.ChecksumIEEE(data))
	case ChecksumMD5:
		return fmt.Sprintf("%x", md5.Sum(data))
	default:
		return fmt.Sprintf("%08x", crc32.ChecksumIEEE(data))
	}
}

// CalculateFileChecksum calculates checksum for a file
func (d *CorruptionDetector) CalculateFileChecksum(path string) (*ChecksumInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	stat, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file %s: %w", path, err)
	}

	checksum := d.CalculateChecksum(data)

	info := &ChecksumInfo{
		Path:         path,
		Algorithm:    d.config.ChecksumAlgorithm,
		Checksum:     checksum,
		Size:         stat.Size(),
		LastChecked:  time.Now(),
		LastModified: stat.ModTime(),
		IsValid:      true,
	}

	// Cache the checksum
	d.mu.Lock()
	d.checksumCache[path] = info
	d.mu.Unlock()

	return info, nil
}

// VerifyData verifies data against expected checksum
func (d *CorruptionDetector) VerifyData(data []byte, expectedChecksum string) *CorruptionEvent {
	actualChecksum := d.CalculateChecksum(data)

	if actualChecksum != expectedChecksum {
		event := &CorruptionEvent{
			ID:          d.generateEventID(),
			Timestamp:   time.Now(),
			Type:        CorruptionTypeChecksum,
			Severity:    CorruptionSeverityMedium,
			Description: "Checksum mismatch detected",
			Expected:    expectedChecksum,
			Actual:      actualChecksum,
			Metadata: map[string]interface{}{
				"data_size": len(data),
				"algorithm": d.config.ChecksumAlgorithm.String(),
			},
		}

		d.recordCorruptionEvent(event)
		return event
	}

	return nil
}

// VerifyFile verifies a file against its stored checksum
func (d *CorruptionDetector) VerifyFile(path string) *CorruptionEvent {
	// Get stored checksum info
	d.mu.RLock()
	storedInfo, exists := d.checksumCache[path]
	d.mu.RUnlock()

	if !exists {
		// Calculate checksum if not cached
		info, err := d.CalculateFileChecksum(path)
		if err != nil {
			event := &CorruptionEvent{
				ID:          d.generateEventID(),
				Timestamp:   time.Now(),
				Location:    DataLocation{File: path},
				Type:        CorruptionTypeComplete,
				Severity:    CorruptionSeverityCritical,
				Description: fmt.Sprintf("Failed to read file for verification: %v", err),
				Metadata: map[string]interface{}{
					"error": err.Error(),
				},
			}

			d.recordCorruptionEvent(event)
			return event
		}

		storedInfo = info
	}

	// Calculate current checksum
	currentInfo, err := d.CalculateFileChecksum(path)
	if err != nil {
		event := &CorruptionEvent{
			ID:          d.generateEventID(),
			Timestamp:   time.Now(),
			Location:    DataLocation{File: path},
			Type:        CorruptionTypeComplete,
			Severity:    CorruptionSeverityCritical,
			Description: fmt.Sprintf("Failed to calculate current checksum: %v", err),
			Metadata: map[string]interface{}{
				"error": err.Error(),
			},
		}

		d.recordCorruptionEvent(event)
		return event
	}

	// Compare checksums
	if currentInfo.Checksum != storedInfo.Checksum {
		event := &CorruptionEvent{
			ID:          d.generateEventID(),
			Timestamp:   time.Now(),
			Location:    DataLocation{File: path, Size: currentInfo.Size},
			Type:        CorruptionTypeChecksum,
			Severity:    d.determineSeverity(path),
			Description: "File checksum mismatch detected",
			Expected:    storedInfo.Checksum,
			Actual:      currentInfo.Checksum,
			Metadata: map[string]interface{}{
				"file_size":     currentInfo.Size,
				"last_modified": currentInfo.LastModified,
				"algorithm":     d.config.ChecksumAlgorithm.String(),
			},
		}

		d.recordCorruptionEvent(event)
		return event
	}

	return nil
}

// ScanDirectory scans a directory for corruption
func (d *CorruptionDetector) ScanDirectory(dirPath string) *CorruptionScanResult {
	result := &CorruptionScanResult{
		StartTime: time.Now(),
		Events:    make([]*CorruptionEvent, 0),
		Success:   true,
	}

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		result.FilesScanned++

		// Verify file
		event := d.VerifyFile(path)
		if event != nil {
			result.Events = append(result.Events, event)
			result.CorruptionsFound++
		}

		return nil
	})

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	if err != nil {
		result.Success = false
		result.Error = err
	}

	return result
}

// IsolateCorruptedData isolates corrupted data
func (d *CorruptionDetector) IsolateCorruptedData(location DataLocation) error {
	if !d.config.EnableAutoIsolation {
		return fmt.Errorf("auto-isolation is disabled")
	}

	// Create isolation directory
	isolationDir := filepath.Join(filepath.Dir(location.File), ".corrupted")
	if err := os.MkdirAll(isolationDir, 0755); err != nil {
		return fmt.Errorf("failed to create isolation directory: %w", err)
	}

	// Generate isolation filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	baseName := filepath.Base(location.File)
	isolatedPath := filepath.Join(isolationDir, fmt.Sprintf("%s.%s.corrupted", baseName, timestamp))

	// Move corrupted file to isolation
	if err := os.Rename(location.File, isolatedPath); err != nil {
		return fmt.Errorf("failed to isolate corrupted file: %w", err)
	}

	// Create isolation metadata
	metadataPath := isolatedPath + ".metadata"
	metadata := fmt.Sprintf(`{
	"original_path": "%s",
	"isolation_time": "%s",
	"reason": "corruption_detected",
	"size": %d,
	"offset": %d
}`, location.File, time.Now().Format(time.RFC3339), location.Size, location.Offset)

	if err := os.WriteFile(metadataPath, []byte(metadata), 0644); err != nil {
		// Log error but don't fail isolation
		fmt.Printf("Warning: failed to write isolation metadata: %v\n", err)
	}

	return nil
}

// GetCorruptionEvents returns recent corruption events
func (d *CorruptionDetector) GetCorruptionEvents(limit int) []*CorruptionEvent {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if limit <= 0 || limit > len(d.corruptionLog) {
		limit = len(d.corruptionLog)
	}

	// Return most recent events
	start := len(d.corruptionLog) - limit
	if start < 0 {
		start = 0
	}

	events := make([]*CorruptionEvent, limit)
	copy(events, d.corruptionLog[start:])

	return events
}

// GetCorruptionStats returns corruption statistics
func (d *CorruptionDetector) GetCorruptionStats() *CorruptionStats {
	d.mu.RLock()
	defer d.mu.RUnlock()

	stats := &CorruptionStats{
		TotalEvents:      len(d.corruptionLog),
		IsolatedEvents:   0,
		RecoveredEvents:  0,
		EventsBySeverity: make(map[CorruptionSeverity]int),
		EventsByType:     make(map[CorruptionType]int),
	}

	for _, event := range d.corruptionLog {
		if event.Isolated {
			stats.IsolatedEvents++
		}
		if event.Recovered {
			stats.RecoveredEvents++
		}

		stats.EventsBySeverity[event.Severity]++
		stats.EventsByType[event.Type]++
	}

	return stats
}

// CorruptionStats contains corruption statistics
type CorruptionStats struct {
	TotalEvents      int                        `json:"total_events"`
	IsolatedEvents   int                        `json:"isolated_events"`
	RecoveredEvents  int                        `json:"recovered_events"`
	EventsBySeverity map[CorruptionSeverity]int `json:"events_by_severity"`
	EventsByType     map[CorruptionType]int     `json:"events_by_type"`
}

// Private methods

func (d *CorruptionDetector) recordCorruptionEvent(event *CorruptionEvent) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Add to log
	d.corruptionLog = append(d.corruptionLog, event)

	// Trim log if it exceeds maximum
	if len(d.corruptionLog) > d.config.MaxCorruptionEvents {
		d.corruptionLog = d.corruptionLog[1:]
	}

	// Auto-isolate if enabled
	if d.config.EnableAutoIsolation && event.Location.File != "" {
		if err := d.IsolateCorruptedData(event.Location); err == nil {
			event.Isolated = true
		}
	}

	// Trigger alerts
	d.triggerAlerts(event)

	// Check corruption threshold
	if len(d.corruptionLog) >= d.config.CorruptionThreshold {
		d.handleCorruptionThresholdExceeded()
	}
}

func (d *CorruptionDetector) triggerAlerts(event *CorruptionEvent) {
	for _, callback := range d.alertCallbacks {
		go callback(event)
	}
}

func (d *CorruptionDetector) generateEventID() string {
	return fmt.Sprintf("corruption_%d", time.Now().UnixNano())
}

func (d *CorruptionDetector) determineSeverity(path string) CorruptionSeverity {
	// Determine severity based on file type and location
	ext := filepath.Ext(path)
	dir := filepath.Dir(path)

	// Critical files
	if contains(dir, "wal", "log") || contains(ext, ".wal", ".log") {
		return CorruptionSeverityCritical
	}

	// Important data files
	if contains(ext, ".db", ".data", ".idx") {
		return CorruptionSeverityHigh
	}

	// Configuration files
	if contains(ext, ".conf", ".config", ".json", ".yaml") {
		return CorruptionSeverityMedium
	}

	// Default
	return CorruptionSeverityLow
}

func (d *CorruptionDetector) handleCorruptionThresholdExceeded() {
	// This could trigger emergency procedures like:
	// - Switching to read-only mode
	// - Alerting administrators
	// - Initiating backup restoration
	fmt.Printf("CRITICAL: Corruption threshold exceeded (%d events)\n", len(d.corruptionLog))
}
