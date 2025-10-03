package errors

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// DiskSpaceMonitor monitors disk space and handles exhaustion scenarios
type DiskSpaceMonitor struct {
	mu                  sync.RWMutex
	config              *DiskMonitorConfig
	monitoredPaths      map[string]*DiskSpaceInfo
	alertCallbacks      []DiskSpaceAlertCallback
	isMonitoring        bool
	stopChan            chan struct{}
	diskSpaceThresholds map[string]DiskSpaceThreshold
}

// DiskMonitorConfig contains configuration for disk space monitoring
type DiskMonitorConfig struct {
	CheckInterval        time.Duration `json:"check_interval"`
	WarningThreshold     float64       `json:"warning_threshold"`   // Percentage (e.g., 0.85 for 85%)
	CriticalThreshold    float64       `json:"critical_threshold"`  // Percentage (e.g., 0.95 for 95%)
	EmergencyThreshold   float64       `json:"emergency_threshold"` // Percentage (e.g., 0.98 for 98%)
	EnableAlerts         bool          `json:"enable_alerts"`
	EnableGracefulReject bool          `json:"enable_graceful_reject"`
}

// DiskSpaceInfo contains information about disk space usage
type DiskSpaceInfo struct {
	Path         string          `json:"path"`
	TotalBytes   uint64          `json:"total_bytes"`
	FreeBytes    uint64          `json:"free_bytes"`
	UsedBytes    uint64          `json:"used_bytes"`
	UsagePercent float64         `json:"usage_percent"`
	LastChecked  time.Time       `json:"last_checked"`
	Status       DiskSpaceStatus `json:"status"`
}

// DiskSpaceStatus represents the current status of disk space
type DiskSpaceStatus int

const (
	DiskSpaceStatusOK DiskSpaceStatus = iota
	DiskSpaceStatusWarning
	DiskSpaceStatusCritical
	DiskSpaceStatusEmergency
	DiskSpaceStatusFull
)

func (s DiskSpaceStatus) String() string {
	switch s {
	case DiskSpaceStatusOK:
		return "OK"
	case DiskSpaceStatusWarning:
		return "WARNING"
	case DiskSpaceStatusCritical:
		return "CRITICAL"
	case DiskSpaceStatusEmergency:
		return "EMERGENCY"
	case DiskSpaceStatusFull:
		return "FULL"
	default:
		return "UNKNOWN"
	}
}

// DiskSpaceThreshold defines thresholds for a specific path
type DiskSpaceThreshold struct {
	Path               string  `json:"path"`
	WarningThreshold   float64 `json:"warning_threshold"`
	CriticalThreshold  float64 `json:"critical_threshold"`
	EmergencyThreshold float64 `json:"emergency_threshold"`
}

// DiskSpaceAlertCallback is called when disk space alerts are triggered
type DiskSpaceAlertCallback func(info *DiskSpaceInfo, previousStatus DiskSpaceStatus)

// NewDiskSpaceMonitor creates a new disk space monitor
func NewDiskSpaceMonitor(config *DiskMonitorConfig) *DiskSpaceMonitor {
	if config == nil {
		config = &DiskMonitorConfig{
			CheckInterval:        30 * time.Second,
			WarningThreshold:     0.80, // 80%
			CriticalThreshold:    0.90, // 90%
			EmergencyThreshold:   0.95, // 95%
			EnableAlerts:         true,
			EnableGracefulReject: true,
		}
	}

	return &DiskSpaceMonitor{
		config:              config,
		monitoredPaths:      make(map[string]*DiskSpaceInfo),
		alertCallbacks:      make([]DiskSpaceAlertCallback, 0),
		diskSpaceThresholds: make(map[string]DiskSpaceThreshold),
		stopChan:            make(chan struct{}),
	}
}

// AddPath adds a path to monitor for disk space
func (m *DiskSpaceMonitor) AddPath(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Resolve absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path for %s: %w", path, err)
	}

	// Check if path exists
	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("path does not exist: %s: %w", absPath, err)
	}

	// Initialize disk space info
	info := &DiskSpaceInfo{
		Path:        absPath,
		LastChecked: time.Time{},
		Status:      DiskSpaceStatusOK,
	}

	m.monitoredPaths[absPath] = info

	// Set default thresholds for this path
	m.diskSpaceThresholds[absPath] = DiskSpaceThreshold{
		Path:               absPath,
		WarningThreshold:   m.config.WarningThreshold,
		CriticalThreshold:  m.config.CriticalThreshold,
		EmergencyThreshold: m.config.EmergencyThreshold,
	}

	return nil
}

// RemovePath removes a path from monitoring
func (m *DiskSpaceMonitor) RemovePath(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	absPath, _ := filepath.Abs(path)
	delete(m.monitoredPaths, absPath)
	delete(m.diskSpaceThresholds, absPath)
}

// SetThresholds sets custom thresholds for a specific path
func (m *DiskSpaceMonitor) SetThresholds(path string, warning, critical, emergency float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path for %s: %w", path, err)
	}

	if _, exists := m.monitoredPaths[absPath]; !exists {
		return fmt.Errorf("path %s is not being monitored", absPath)
	}

	m.diskSpaceThresholds[absPath] = DiskSpaceThreshold{
		Path:               absPath,
		WarningThreshold:   warning,
		CriticalThreshold:  critical,
		EmergencyThreshold: emergency,
	}

	return nil
}

// AddAlertCallback adds a callback function for disk space alerts
func (m *DiskSpaceMonitor) AddAlertCallback(callback DiskSpaceAlertCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.alertCallbacks = append(m.alertCallbacks, callback)
}

// StartMonitoring starts the disk space monitoring loop
func (m *DiskSpaceMonitor) StartMonitoring() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isMonitoring {
		return fmt.Errorf("monitoring is already running")
	}

	if len(m.monitoredPaths) == 0 {
		return fmt.Errorf("no paths configured for monitoring")
	}

	m.isMonitoring = true
	m.stopChan = make(chan struct{})

	go m.monitoringLoop()

	return nil
}

// StopMonitoring stops the disk space monitoring loop
func (m *DiskSpaceMonitor) StopMonitoring() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isMonitoring {
		return
	}

	m.isMonitoring = false
	close(m.stopChan)
}

// CheckDiskSpace checks disk space for a specific path
func (m *DiskSpaceMonitor) CheckDiskSpace(path string) (*DiskSpaceInfo, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path for %s: %w", path, err)
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(absPath, &stat); err != nil {
		return nil, fmt.Errorf("failed to get disk space info for %s: %w", absPath, err)
	}

	totalBytes := stat.Blocks * uint64(stat.Bsize)
	freeBytes := stat.Bavail * uint64(stat.Bsize)
	usedBytes := totalBytes - freeBytes
	usagePercent := float64(usedBytes) / float64(totalBytes)

	info := &DiskSpaceInfo{
		Path:         absPath,
		TotalBytes:   totalBytes,
		FreeBytes:    freeBytes,
		UsedBytes:    usedBytes,
		UsagePercent: usagePercent,
		LastChecked:  time.Now(),
	}

	// Determine status based on thresholds
	threshold, exists := m.diskSpaceThresholds[absPath]
	if !exists {
		threshold = DiskSpaceThreshold{
			WarningThreshold:   m.config.WarningThreshold,
			CriticalThreshold:  m.config.CriticalThreshold,
			EmergencyThreshold: m.config.EmergencyThreshold,
		}
	}

	info.Status = m.determineStatus(usagePercent, threshold)

	return info, nil
}

// CheckAllPaths checks disk space for all monitored paths
func (m *DiskSpaceMonitor) CheckAllPaths() map[string]*DiskSpaceInfo {
	m.mu.RLock()
	paths := make([]string, 0, len(m.monitoredPaths))
	for path := range m.monitoredPaths {
		paths = append(paths, path)
	}
	m.mu.RUnlock()

	results := make(map[string]*DiskSpaceInfo)

	for _, path := range paths {
		info, err := m.CheckDiskSpace(path)
		if err != nil {
			// Create error info
			info = &DiskSpaceInfo{
				Path:        path,
				LastChecked: time.Now(),
				Status:      DiskSpaceStatusFull, // Assume worst case on error
			}
		}

		results[path] = info

		// Update stored info and trigger alerts if needed
		m.updatePathInfo(path, info)
	}

	return results
}

// CanWrite checks if a write operation can proceed based on disk space
func (m *DiskSpaceMonitor) CanWrite(path string, estimatedSize int64) error {
	if !m.config.EnableGracefulReject {
		return nil
	}

	info, err := m.CheckDiskSpace(path)
	if err != nil {
		return fmt.Errorf("failed to check disk space: %w", err)
	}

	// Check if there's enough space for the estimated write
	if int64(info.FreeBytes) < estimatedSize {
		return &DiskSpaceError{
			Path:           path,
			RequiredBytes:  estimatedSize,
			AvailableBytes: int64(info.FreeBytes),
			Status:         info.Status,
		}
	}

	// Check if we're in emergency or full status
	if info.Status >= DiskSpaceStatusEmergency {
		return &DiskSpaceError{
			Path:           path,
			RequiredBytes:  estimatedSize,
			AvailableBytes: int64(info.FreeBytes),
			Status:         info.Status,
		}
	}

	return nil
}

// GetDiskSpaceInfo returns current disk space information for a path
func (m *DiskSpaceMonitor) GetDiskSpaceInfo(path string) (*DiskSpaceInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path for %s: %w", path, err)
	}

	info, exists := m.monitoredPaths[absPath]
	if !exists {
		return nil, fmt.Errorf("path %s is not being monitored", absPath)
	}

	// Return a copy to avoid race conditions
	infoCopy := *info
	return &infoCopy, nil
}

// GetAllDiskSpaceInfo returns disk space information for all monitored paths
func (m *DiskSpaceMonitor) GetAllDiskSpaceInfo() map[string]*DiskSpaceInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*DiskSpaceInfo)
	for path, info := range m.monitoredPaths {
		infoCopy := *info
		result[path] = &infoCopy
	}

	return result
}

// DiskSpaceError represents a disk space related error
type DiskSpaceError struct {
	Path           string          `json:"path"`
	RequiredBytes  int64           `json:"required_bytes"`
	AvailableBytes int64           `json:"available_bytes"`
	Status         DiskSpaceStatus `json:"status"`
}

func (e *DiskSpaceError) Error() string {
	return fmt.Sprintf("insufficient disk space on %s: required %d bytes, available %d bytes, status: %s",
		e.Path, e.RequiredBytes, e.AvailableBytes, e.Status)
}

// Private methods

func (m *DiskSpaceMonitor) monitoringLoop() {
	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.CheckAllPaths()
		case <-m.stopChan:
			return
		}
	}
}

func (m *DiskSpaceMonitor) updatePathInfo(path string, newInfo *DiskSpaceInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldInfo, exists := m.monitoredPaths[path]
	if !exists {
		m.monitoredPaths[path] = newInfo
		return
	}

	previousStatus := oldInfo.Status
	m.monitoredPaths[path] = newInfo

	// Trigger alerts if status changed and alerts are enabled
	if m.config.EnableAlerts && newInfo.Status != previousStatus {
		m.triggerAlerts(newInfo, previousStatus)
	}
}

func (m *DiskSpaceMonitor) triggerAlerts(info *DiskSpaceInfo, previousStatus DiskSpaceStatus) {
	for _, callback := range m.alertCallbacks {
		go callback(info, previousStatus)
	}
}

func (m *DiskSpaceMonitor) determineStatus(usagePercent float64, threshold DiskSpaceThreshold) DiskSpaceStatus {
	if usagePercent >= 1.0 {
		return DiskSpaceStatusFull
	} else if usagePercent >= threshold.EmergencyThreshold {
		return DiskSpaceStatusEmergency
	} else if usagePercent >= threshold.CriticalThreshold {
		return DiskSpaceStatusCritical
	} else if usagePercent >= threshold.WarningThreshold {
		return DiskSpaceStatusWarning
	}

	return DiskSpaceStatusOK
}
