package integrity

import (
	"context"
	"crypto/rand"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CorruptionDetector implements automatic corruption detection
type CorruptionDetector struct {
	engine          *ChecksumEngine
	config          *IntegrityConfig
	stats           *CorruptionStats
	scanContext     context.Context
	scanCancel      context.CancelFunc
	scanWg          sync.WaitGroup
	mutex           sync.RWMutex
	eventHandlers   []func(*CorruptionEvent)
	checksumCache   map[string]uint32
	lastScanResults map[string]*ScanResult
}

// ScanResult contains the result of scanning a file or directory
type ScanResult struct {
	Path        string    `json:"path"`
	Checksum    uint32    `json:"checksum"`
	Size        int64     `json:"size"`
	ModTime     time.Time `json:"mod_time"`
	IsCorrupted bool      `json:"is_corrupted"`
	Error       string    `json:"error,omitempty"`
	ScanTime    time.Time `json:"scan_time"`
}

// NewCorruptionDetector creates a new corruption detector
func NewCorruptionDetector(config *IntegrityConfig) *CorruptionDetector {
	if config == nil {
		config = DefaultIntegrityConfig()
	}

	return &CorruptionDetector{
		engine: NewChecksumEngine(config.ChecksumAlgorithm),
		config: config,
		stats: &CorruptionStats{
			EventsByType:     make(map[CorruptionType]int64),
			EventsBySeverity: make(map[CorruptionSeverity]int64),
		},
		checksumCache:   make(map[string]uint32),
		lastScanResults: make(map[string]*ScanResult),
	}
}

// DetectCorruption performs real-time corruption detection on data
func (cd *CorruptionDetector) DetectCorruption(data []byte, expectedChecksum uint32) *CorruptionEvent {
	cd.mutex.RLock()
	defer cd.mutex.RUnlock()

	actualChecksum := cd.engine.Calculate(data)
	if actualChecksum != expectedChecksum {
		event := &CorruptionEvent{
			ID:          cd.generateEventID(),
			Timestamp:   time.Now(),
			Location:    "memory",
			Type:        CorruptionTypeChecksum,
			Severity:    cd.determineSeverity(CorruptionTypeChecksum, int64(len(data))),
			Description: "Checksum mismatch detected during real-time verification",
			Expected:    expectedChecksum,
			Actual:      actualChecksum,
			Size:        int64(len(data)),
		}

		cd.recordCorruptionEvent(event)
		return event
	}

	return nil
}

// ValidateData validates data integrity and returns corruption event if found
func (cd *CorruptionDetector) ValidateData(data []byte, location string) *CorruptionEvent {
	cd.mutex.RLock()
	defer cd.mutex.RUnlock()

	// Check for basic data integrity issues
	if len(data) == 0 {
		event := &CorruptionEvent{
			ID:          cd.generateEventID(),
			Timestamp:   time.Now(),
			Location:    location,
			Type:        CorruptionTypeSize,
			Severity:    SeverityMedium,
			Description: "Empty data detected",
			Size:        0,
		}
		cd.recordCorruptionEvent(event)
		return event
	}

	// Check if we have a cached checksum for this location
	if expectedChecksum, exists := cd.checksumCache[location]; exists {
		actualChecksum := cd.engine.Calculate(data)
		if actualChecksum != expectedChecksum {
			event := &CorruptionEvent{
				ID:          cd.generateEventID(),
				Timestamp:   time.Now(),
				Location:    location,
				Type:        CorruptionTypeChecksum,
				Severity:    cd.determineSeverity(CorruptionTypeChecksum, int64(len(data))),
				Description: "Cached checksum mismatch detected",
				Expected:    expectedChecksum,
				Actual:      actualChecksum,
				Size:        int64(len(data)),
			}
			cd.recordCorruptionEvent(event)
			return event
		}
	}

	return nil
}

// StartBackgroundScan starts background integrity scanning
func (cd *CorruptionDetector) StartBackgroundScan(directory string) error {
	cd.mutex.Lock()
	defer cd.mutex.Unlock()

	if !cd.config.EnableBackgroundScan {
		return fmt.Errorf("background scanning is disabled")
	}

	if cd.scanContext != nil {
		return fmt.Errorf("background scan is already running")
	}

	cd.scanContext, cd.scanCancel = context.WithCancel(context.Background())

	cd.scanWg.Add(1)
	go cd.backgroundScanWorker(directory)

	return nil
}

// StopBackgroundScan stops background integrity scanning
func (cd *CorruptionDetector) StopBackgroundScan() error {
	cd.mutex.Lock()
	defer cd.mutex.Unlock()

	if cd.scanCancel == nil {
		return fmt.Errorf("no background scan is running")
	}

	cd.scanCancel()
	cd.scanWg.Wait()
	cd.scanContext = nil
	cd.scanCancel = nil

	return nil
}

// ScanDirectory performs integrity scan on a directory
func (cd *CorruptionDetector) ScanDirectory(directory string) ([]CorruptionEvent, error) {
	var events []CorruptionEvent
	var scanErrors []error

	err := filepath.WalkDir(directory, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			scanErrors = append(scanErrors, fmt.Errorf("error accessing %s: %w", path, err))
			return nil // Continue scanning other files
		}

		if d.IsDir() {
			return nil // Skip directories
		}

		result, err := cd.scanFile(path)
		if err != nil {
			scanErrors = append(scanErrors, fmt.Errorf("error scanning %s: %w", path, err))
			return nil
		}

		if result.IsCorrupted {
			event := &CorruptionEvent{
				ID:          cd.generateEventID(),
				Timestamp:   time.Now(),
				Location:    path,
				Type:        CorruptionTypeChecksum,
				Severity:    cd.determineSeverity(CorruptionTypeChecksum, result.Size),
				Description: fmt.Sprintf("File corruption detected during scan: %s", result.Error),
				Size:        result.Size,
				Metadata: map[string]interface{}{
					"scan_time": result.ScanTime,
					"mod_time":  result.ModTime,
				},
			}
			events = append(events, *event)
			cd.recordCorruptionEvent(event)
		}

		// Update scan results cache
		cd.mutex.Lock()
		cd.lastScanResults[path] = result
		cd.mutex.Unlock()

		return nil
	})

	if err != nil {
		return events, fmt.Errorf("directory scan failed: %w", err)
	}

	if len(scanErrors) > 0 {
		return events, fmt.Errorf("scan completed with %d errors", len(scanErrors))
	}

	return events, nil
}

// GetCorruptionStats returns current corruption statistics
func (cd *CorruptionDetector) GetCorruptionStats() *CorruptionStats {
	cd.mutex.RLock()
	defer cd.mutex.RUnlock()

	// Create a copy to avoid race conditions
	stats := &CorruptionStats{
		TotalEvents:      cd.stats.TotalEvents,
		EventsByType:     make(map[CorruptionType]int64),
		EventsBySeverity: make(map[CorruptionSeverity]int64),
		LastScanTime:     cd.stats.LastScanTime,
		CorruptionRate:   cd.stats.CorruptionRate,
		RecoverySuccess:  cd.stats.RecoverySuccess,
		RecoveryFailures: cd.stats.RecoveryFailures,
	}

	for k, v := range cd.stats.EventsByType {
		stats.EventsByType[k] = v
	}
	for k, v := range cd.stats.EventsBySeverity {
		stats.EventsBySeverity[k] = v
	}

	if cd.stats.LastEvent != nil {
		lastEvent := *cd.stats.LastEvent
		stats.LastEvent = &lastEvent
	}

	return stats
}

// GetHealthStatus returns the current health status
func (cd *CorruptionDetector) GetHealthStatus() *IntegrityHealthStatus {
	cd.mutex.RLock()
	defer cd.mutex.RUnlock()

	status := &IntegrityHealthStatus{
		Status:        cd.determineOverallHealth(),
		LastCheckTime: time.Now(),
		ComponentHealth: map[string]HealthStatus{
			"corruption_detector": cd.getDetectorHealth(),
			"checksum_engine":     cd.getEngineHealth(),
			"background_scanner":  cd.getScannerHealth(),
		},
		ActiveScans: cd.getActiveScanCount(),
		Metrics:     cd.getIntegrityMetrics(),
	}

	// Add recent events (last 10)
	if cd.stats.LastEvent != nil {
		status.RecentEvents = []CorruptionEvent{*cd.stats.LastEvent}
	}

	return status
}

// RegisterEventHandler registers a handler for corruption events
func (cd *CorruptionDetector) RegisterEventHandler(handler func(*CorruptionEvent)) {
	cd.mutex.Lock()
	defer cd.mutex.Unlock()
	cd.eventHandlers = append(cd.eventHandlers, handler)
}

// UpdateChecksumCache updates the checksum cache for a location
func (cd *CorruptionDetector) UpdateChecksumCache(location string, checksum uint32) {
	cd.mutex.Lock()
	defer cd.mutex.Unlock()
	cd.checksumCache[location] = checksum
}

// ClearChecksumCache clears the checksum cache
func (cd *CorruptionDetector) ClearChecksumCache() {
	cd.mutex.Lock()
	defer cd.mutex.Unlock()
	cd.checksumCache = make(map[string]uint32)
}

// Private methods

func (cd *CorruptionDetector) backgroundScanWorker(directory string) {
	defer cd.scanWg.Done()

	ticker := time.NewTicker(cd.config.ScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-cd.scanContext.Done():
			return
		case <-ticker.C:
			events, err := cd.ScanDirectory(directory)
			if err != nil {
				// Log error but continue scanning
				continue
			}

			cd.mutex.Lock()
			cd.stats.LastScanTime = time.Now()
			cd.mutex.Unlock()

			// Notify handlers of any corruption events
			for _, event := range events {
				cd.notifyEventHandlers(&event)
			}
		}
	}
}

func (cd *CorruptionDetector) scanFile(filePath string) (*ScanResult, error) {
	startTime := time.Now()

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return &ScanResult{
			Path:        filePath,
			IsCorrupted: true,
			Error:       fmt.Sprintf("failed to stat file: %v", err),
			ScanTime:    startTime,
		}, nil
	}

	checksum, err := cd.engine.CalculateFileChecksum(filePath)
	if err != nil {
		return &ScanResult{
			Path:        filePath,
			Size:        fileInfo.Size(),
			ModTime:     fileInfo.ModTime(),
			IsCorrupted: true,
			Error:       fmt.Sprintf("failed to calculate checksum: %v", err),
			ScanTime:    startTime,
		}, nil
	}

	// Check against cached checksum if available
	isCorrupted := false
	errorMsg := ""

	if cachedChecksum, exists := cd.checksumCache[filePath]; exists {
		if checksum != cachedChecksum {
			isCorrupted = true
			errorMsg = fmt.Sprintf("checksum mismatch: expected %08x, got %08x", cachedChecksum, checksum)
		}
	} else {
		// Update cache with new checksum
		cd.checksumCache[filePath] = checksum
	}

	return &ScanResult{
		Path:        filePath,
		Checksum:    checksum,
		Size:        fileInfo.Size(),
		ModTime:     fileInfo.ModTime(),
		IsCorrupted: isCorrupted,
		Error:       errorMsg,
		ScanTime:    startTime,
	}, nil
}

func (cd *CorruptionDetector) recordCorruptionEvent(event *CorruptionEvent) {
	cd.stats.TotalEvents++
	cd.stats.EventsByType[event.Type]++
	cd.stats.EventsBySeverity[event.Severity]++
	cd.stats.LastEvent = event

	// Calculate corruption rate (simplified)
	if cd.stats.TotalEvents > 0 {
		cd.stats.CorruptionRate = float64(cd.stats.TotalEvents) / float64(cd.stats.TotalEvents+cd.stats.RecoverySuccess)
	}

	cd.notifyEventHandlers(event)
}

func (cd *CorruptionDetector) notifyEventHandlers(event *CorruptionEvent) {
	for _, handler := range cd.eventHandlers {
		go handler(event) // Run handlers asynchronously
	}
}

func (cd *CorruptionDetector) generateEventID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return fmt.Sprintf("corruption_%x_%d", bytes, time.Now().UnixNano())
}

func (cd *CorruptionDetector) determineSeverity(corruptionType CorruptionType, size int64) CorruptionSeverity {
	switch corruptionType {
	case CorruptionTypeChecksum:
		if size > 1024*1024 { // > 1MB
			return SeverityHigh
		} else if size > 1024 { // > 1KB
			return SeverityMedium
		}
		return SeverityLow
	case CorruptionTypeHeader:
		return SeverityHigh
	case CorruptionTypeFormat:
		return SeverityMedium
	case CorruptionTypeSize:
		return SeverityLow
	default:
		return SeverityMedium
	}
}

func (cd *CorruptionDetector) determineOverallHealth() HealthStatus {
	if cd.stats.CorruptionRate > cd.config.AlertThresholds.CorruptionRate {
		return HealthStatusCritical
	}
	if cd.stats.TotalEvents > 0 && cd.stats.LastEvent != nil {
		if time.Since(cd.stats.LastEvent.Timestamp) < time.Hour {
			return HealthStatusWarning
		}
	}
	return HealthStatusHealthy
}

func (cd *CorruptionDetector) getDetectorHealth() HealthStatus {
	if cd.stats.CorruptionRate > cd.config.AlertThresholds.CorruptionRate {
		return HealthStatusCritical
	}
	return HealthStatusHealthy
}

func (cd *CorruptionDetector) getEngineHealth() HealthStatus {
	// Simple health check - could be expanded
	return HealthStatusHealthy
}

func (cd *CorruptionDetector) getScannerHealth() HealthStatus {
	if cd.config.EnableBackgroundScan && cd.scanContext == nil {
		return HealthStatusWarning
	}
	return HealthStatusHealthy
}

func (cd *CorruptionDetector) getActiveScanCount() int {
	if cd.scanContext != nil {
		return 1
	}
	return 0
}

func (cd *CorruptionDetector) getIntegrityMetrics() *IntegrityMetrics {
	return &IntegrityMetrics{
		ChecksumOperations: &OperationMetrics{
			TotalOperations: cd.stats.TotalEvents + cd.stats.RecoverySuccess,
			SuccessfulOps:   cd.stats.RecoverySuccess,
			FailedOps:       cd.stats.TotalEvents,
		},
		CorruptionDetection: &OperationMetrics{
			TotalOperations: cd.stats.TotalEvents,
			FailedOps:       cd.stats.TotalEvents,
		},
		LastUpdated: time.Now(),
	}
}
