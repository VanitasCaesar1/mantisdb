package errors

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// DiskRecoveryManager handles disk space recovery procedures
type DiskRecoveryManager struct {
	monitor      *DiskSpaceMonitor
	config       *DiskRecoveryConfig
	cleanupTasks []CleanupTask
}

// DiskRecoveryConfig contains configuration for disk recovery
type DiskRecoveryConfig struct {
	EnableAutoCleanup    bool          `json:"enable_auto_cleanup"`
	CleanupThreshold     float64       `json:"cleanup_threshold"` // Trigger cleanup at this usage %
	TargetFreeSpace      float64       `json:"target_free_space"` // Target free space % after cleanup
	MaxCleanupAttempts   int           `json:"max_cleanup_attempts"`
	CleanupRetryDelay    time.Duration `json:"cleanup_retry_delay"`
	EmergencyCleanupMode bool          `json:"emergency_cleanup_mode"`
}

// CleanupTask represents a disk cleanup task
type CleanupTask struct {
	Name        string                  `json:"name"`
	Priority    int                     `json:"priority"`     // Lower number = higher priority
	EstimatedMB int64                   `json:"estimated_mb"` // Estimated space to free in MB
	Execute     func(path string) error `json:"-"`
}

// CleanupResult contains the result of a cleanup operation
type CleanupResult struct {
	TaskName   string        `json:"task_name"`
	Success    bool          `json:"success"`
	BytesFreed int64         `json:"bytes_freed"`
	Duration   time.Duration `json:"duration"`
	Error      error         `json:"error,omitempty"`
}

// NewDiskRecoveryManager creates a new disk recovery manager
func NewDiskRecoveryManager(monitor *DiskSpaceMonitor, config *DiskRecoveryConfig) *DiskRecoveryManager {
	if config == nil {
		config = &DiskRecoveryConfig{
			EnableAutoCleanup:    true,
			CleanupThreshold:     0.85, // Start cleanup at 85% usage
			TargetFreeSpace:      0.20, // Target 20% free space
			MaxCleanupAttempts:   3,
			CleanupRetryDelay:    5 * time.Second,
			EmergencyCleanupMode: false,
		}
	}

	manager := &DiskRecoveryManager{
		monitor:      monitor,
		config:       config,
		cleanupTasks: make([]CleanupTask, 0),
	}

	// Register default cleanup tasks
	manager.registerDefaultCleanupTasks()

	return manager
}

// RegisterCleanupTask registers a new cleanup task
func (m *DiskRecoveryManager) RegisterCleanupTask(task CleanupTask) {
	m.cleanupTasks = append(m.cleanupTasks, task)

	// Sort tasks by priority (lower number = higher priority)
	sort.Slice(m.cleanupTasks, func(i, j int) bool {
		return m.cleanupTasks[i].Priority < m.cleanupTasks[j].Priority
	})
}

// AttemptRecovery attempts to recover disk space for a given path
func (m *DiskRecoveryManager) AttemptRecovery(path string) (*DiskRecoveryResult, error) {
	result := &DiskRecoveryResult{
		Path:      path,
		StartTime: time.Now(),
		Success:   false,
	}

	// Check initial disk space
	initialInfo, err := m.monitor.CheckDiskSpace(path)
	if err != nil {
		result.Error = fmt.Errorf("failed to check initial disk space: %w", err)
		return result, result.Error
	}

	result.InitialFreeBytes = int64(initialInfo.FreeBytes)
	result.InitialUsagePercent = initialInfo.UsagePercent

	// Determine if cleanup is needed
	if initialInfo.UsagePercent < m.config.CleanupThreshold {
		result.Success = true
		result.Message = "No cleanup needed"
		return result, nil
	}

	// Perform cleanup attempts
	for attempt := 1; attempt <= m.config.MaxCleanupAttempts; attempt++ {
		cleanupResult := m.performCleanup(path, attempt)
		result.CleanupResults = append(result.CleanupResults, cleanupResult...)

		// Check if we've achieved target free space
		currentInfo, err := m.monitor.CheckDiskSpace(path)
		if err != nil {
			result.Error = fmt.Errorf("failed to check disk space after cleanup attempt %d: %w", attempt, err)
			break
		}

		result.FinalFreeBytes = int64(currentInfo.FreeBytes)
		result.FinalUsagePercent = currentInfo.UsagePercent
		result.BytesFreed = result.FinalFreeBytes - result.InitialFreeBytes

		// Check if we've reached the target
		targetUsage := 1.0 - m.config.TargetFreeSpace
		if currentInfo.UsagePercent <= targetUsage {
			result.Success = true
			result.Message = fmt.Sprintf("Successfully freed %d bytes in %d attempts", result.BytesFreed, attempt)
			break
		}

		// Wait before next attempt
		if attempt < m.config.MaxCleanupAttempts {
			time.Sleep(m.config.CleanupRetryDelay)
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	if !result.Success {
		result.Error = fmt.Errorf("failed to achieve target free space after %d attempts", m.config.MaxCleanupAttempts)
	}

	return result, result.Error
}

// PerformEmergencyCleanup performs aggressive cleanup in emergency situations
func (m *DiskRecoveryManager) PerformEmergencyCleanup(path string) (*DiskRecoveryResult, error) {
	// Temporarily enable emergency cleanup mode
	originalMode := m.config.EmergencyCleanupMode
	m.config.EmergencyCleanupMode = true
	defer func() {
		m.config.EmergencyCleanupMode = originalMode
	}()

	// Lower the target free space for emergency cleanup
	originalTarget := m.config.TargetFreeSpace
	m.config.TargetFreeSpace = 0.10 // Target only 10% free space in emergency
	defer func() {
		m.config.TargetFreeSpace = originalTarget
	}()

	return m.AttemptRecovery(path)
}

// GetCleanupTasks returns the list of registered cleanup tasks
func (m *DiskRecoveryManager) GetCleanupTasks() []CleanupTask {
	return append([]CleanupTask(nil), m.cleanupTasks...) // Return a copy
}

// DiskRecoveryResult contains the result of a disk recovery operation
type DiskRecoveryResult struct {
	Path                string          `json:"path"`
	StartTime           time.Time       `json:"start_time"`
	EndTime             time.Time       `json:"end_time"`
	Duration            time.Duration   `json:"duration"`
	Success             bool            `json:"success"`
	InitialFreeBytes    int64           `json:"initial_free_bytes"`
	FinalFreeBytes      int64           `json:"final_free_bytes"`
	BytesFreed          int64           `json:"bytes_freed"`
	InitialUsagePercent float64         `json:"initial_usage_percent"`
	FinalUsagePercent   float64         `json:"final_usage_percent"`
	CleanupResults      []CleanupResult `json:"cleanup_results"`
	Message             string          `json:"message"`
	Error               error           `json:"error,omitempty"`
}

// Private methods

func (m *DiskRecoveryManager) performCleanup(path string, attempt int) []CleanupResult {
	var results []CleanupResult

	for _, task := range m.cleanupTasks {
		startTime := time.Now()

		result := CleanupResult{
			TaskName: task.Name,
			Success:  false,
		}

		// Get disk space before cleanup
		beforeInfo, err := m.monitor.CheckDiskSpace(path)
		if err != nil {
			result.Error = fmt.Errorf("failed to check disk space before cleanup: %w", err)
			result.Duration = time.Since(startTime)
			results = append(results, result)
			continue
		}

		// Execute cleanup task
		err = task.Execute(path)
		if err != nil {
			result.Error = fmt.Errorf("cleanup task failed: %w", err)
			result.Duration = time.Since(startTime)
			results = append(results, result)
			continue
		}

		// Get disk space after cleanup
		afterInfo, err := m.monitor.CheckDiskSpace(path)
		if err != nil {
			result.Error = fmt.Errorf("failed to check disk space after cleanup: %w", err)
			result.Duration = time.Since(startTime)
			results = append(results, result)
			continue
		}

		result.Success = true
		result.BytesFreed = int64(afterInfo.FreeBytes) - int64(beforeInfo.FreeBytes)
		result.Duration = time.Since(startTime)

		results = append(results, result)

		// If we've achieved enough free space, stop cleanup
		targetUsage := 1.0 - m.config.TargetFreeSpace
		if afterInfo.UsagePercent <= targetUsage {
			break
		}
	}

	return results
}

func (m *DiskRecoveryManager) registerDefaultCleanupTasks() {
	// Task 1: Clean temporary files
	m.RegisterCleanupTask(CleanupTask{
		Name:        "clean_temp_files",
		Priority:    1,
		EstimatedMB: 100,
		Execute:     m.cleanTempFiles,
	})

	// Task 2: Clean old log files
	m.RegisterCleanupTask(CleanupTask{
		Name:        "clean_old_logs",
		Priority:    2,
		EstimatedMB: 500,
		Execute:     m.cleanOldLogFiles,
	})

	// Task 3: Compact WAL files (if applicable)
	m.RegisterCleanupTask(CleanupTask{
		Name:        "compact_wal_files",
		Priority:    3,
		EstimatedMB: 1000,
		Execute:     m.compactWALFiles,
	})

	// Task 4: Clean old checkpoints
	m.RegisterCleanupTask(CleanupTask{
		Name:        "clean_old_checkpoints",
		Priority:    4,
		EstimatedMB: 2000,
		Execute:     m.cleanOldCheckpoints,
	})

	// Emergency task: Clean cache files (only in emergency mode)
	m.RegisterCleanupTask(CleanupTask{
		Name:        "clean_cache_files",
		Priority:    10,
		EstimatedMB: 5000,
		Execute:     m.cleanCacheFiles,
	})
}

// Default cleanup task implementations

func (m *DiskRecoveryManager) cleanTempFiles(path string) error {
	tempDir := filepath.Join(path, "temp")
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		return nil // No temp directory
	}

	// Remove files older than 1 hour
	cutoff := time.Now().Add(-1 * time.Hour)

	return filepath.Walk(tempDir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		if !info.IsDir() && info.ModTime().Before(cutoff) {
			os.Remove(filePath)
		}

		return nil
	})
}

func (m *DiskRecoveryManager) cleanOldLogFiles(path string) error {
	logDir := filepath.Join(path, "logs")
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		return nil // No log directory
	}

	// Remove log files older than 7 days
	cutoff := time.Now().Add(-7 * 24 * time.Hour)

	return filepath.Walk(logDir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		if !info.IsDir() && info.ModTime().Before(cutoff) &&
			(filepath.Ext(filePath) == ".log" || filepath.Ext(filePath) == ".txt") {
			os.Remove(filePath)
		}

		return nil
	})
}

func (m *DiskRecoveryManager) compactWALFiles(path string) error {
	// This would integrate with the WAL system to compact old WAL files
	// For now, this is a placeholder
	walDir := filepath.Join(path, "wal")
	if _, err := os.Stat(walDir); os.IsNotExist(err) {
		return nil // No WAL directory
	}

	// Implementation would depend on WAL system
	return nil
}

func (m *DiskRecoveryManager) cleanOldCheckpoints(path string) error {
	checkpointDir := filepath.Join(path, "checkpoints")
	if _, err := os.Stat(checkpointDir); os.IsNotExist(err) {
		return nil // No checkpoint directory
	}

	// Keep only the 5 most recent checkpoints
	var checkpoints []os.FileInfo

	err := filepath.Walk(checkpointDir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if !info.IsDir() {
			checkpoints = append(checkpoints, info)
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Sort by modification time (newest first)
	sort.Slice(checkpoints, func(i, j int) bool {
		return checkpoints[i].ModTime().After(checkpoints[j].ModTime())
	})

	// Remove old checkpoints (keep only 5 newest)
	for i := 5; i < len(checkpoints); i++ {
		checkpointPath := filepath.Join(checkpointDir, checkpoints[i].Name())
		os.Remove(checkpointPath)
	}

	return nil
}

func (m *DiskRecoveryManager) cleanCacheFiles(path string) error {
	// Only run in emergency mode
	if !m.config.EmergencyCleanupMode {
		return nil
	}

	cacheDir := filepath.Join(path, "cache")
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		return nil // No cache directory
	}

	// Remove all cache files in emergency mode
	return os.RemoveAll(cacheDir)
}
