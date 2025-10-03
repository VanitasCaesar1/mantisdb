package errors

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CorruptionRecoveryManager handles recovery from corruption
type CorruptionRecoveryManager struct {
	detector      *CorruptionDetector
	config        *CorruptionRecoveryConfig
	recoveryTasks []CorruptionRecoveryTask
}

// CorruptionRecoveryConfig contains configuration for corruption recovery
type CorruptionRecoveryConfig struct {
	EnableAutoRecovery    bool          `json:"enable_auto_recovery"`
	BackupDirectory       string        `json:"backup_directory"`
	MaxRecoveryAttempts   int           `json:"max_recovery_attempts"`
	RecoveryRetryDelay    time.Duration `json:"recovery_retry_delay"`
	EnableBackupRestore   bool          `json:"enable_backup_restore"`
	EnableRedundantCopy   bool          `json:"enable_redundant_copy"`
	EnablePartialRecovery bool          `json:"enable_partial_recovery"`
}

// CorruptionRecoveryTask represents a corruption recovery task
type CorruptionRecoveryTask struct {
	Name       string                             `json:"name"`
	Priority   int                                `json:"priority"`
	CanRecover func(event *CorruptionEvent) bool  `json:"-"`
	Execute    func(event *CorruptionEvent) error `json:"-"`
}

// CorruptionRecoveryResult contains the result of a corruption recovery operation
type CorruptionRecoveryResult struct {
	Event          *CorruptionEvent               `json:"event"`
	StartTime      time.Time                      `json:"start_time"`
	EndTime        time.Time                      `json:"end_time"`
	Duration       time.Duration                  `json:"duration"`
	Success        bool                           `json:"success"`
	RecoveryMethod string                         `json:"recovery_method"`
	TaskResults    []CorruptionRecoveryTaskResult `json:"task_results"`
	Message        string                         `json:"message"`
	Error          error                          `json:"error,omitempty"`
}

// CorruptionRecoveryTaskResult contains the result of a single recovery task
type CorruptionRecoveryTaskResult struct {
	TaskName string        `json:"task_name"`
	Success  bool          `json:"success"`
	Duration time.Duration `json:"duration"`
	Message  string        `json:"message"`
	Error    error         `json:"error,omitempty"`
}

// NewCorruptionRecoveryManager creates a new corruption recovery manager
func NewCorruptionRecoveryManager(detector *CorruptionDetector, config *CorruptionRecoveryConfig) *CorruptionRecoveryManager {
	if config == nil {
		config = &CorruptionRecoveryConfig{
			EnableAutoRecovery:    false, // Disabled by default for safety
			BackupDirectory:       "./backups",
			MaxRecoveryAttempts:   3,
			RecoveryRetryDelay:    5 * time.Second,
			EnableBackupRestore:   true,
			EnableRedundantCopy:   true,
			EnablePartialRecovery: false,
		}
	}

	manager := &CorruptionRecoveryManager{
		detector:      detector,
		config:        config,
		recoveryTasks: make([]CorruptionRecoveryTask, 0),
	}

	// Register default recovery tasks
	manager.registerDefaultRecoveryTasks()

	return manager
}

// RegisterRecoveryTask registers a new corruption recovery task
func (m *CorruptionRecoveryManager) RegisterRecoveryTask(task CorruptionRecoveryTask) {
	m.recoveryTasks = append(m.recoveryTasks, task)

	// Sort tasks by priority (lower number = higher priority)
	for i := len(m.recoveryTasks) - 1; i > 0; i-- {
		if m.recoveryTasks[i].Priority < m.recoveryTasks[i-1].Priority {
			m.recoveryTasks[i], m.recoveryTasks[i-1] = m.recoveryTasks[i-1], m.recoveryTasks[i]
		} else {
			break
		}
	}
}

// AttemptRecovery attempts to recover from a corruption event
func (m *CorruptionRecoveryManager) AttemptRecovery(event *CorruptionEvent) *CorruptionRecoveryResult {
	result := &CorruptionRecoveryResult{
		Event:       event,
		StartTime:   time.Now(),
		Success:     false,
		TaskResults: make([]CorruptionRecoveryTaskResult, 0),
	}

	// Find applicable recovery tasks
	applicableTasks := make([]CorruptionRecoveryTask, 0)
	for _, task := range m.recoveryTasks {
		if task.CanRecover(event) {
			applicableTasks = append(applicableTasks, task)
		}
	}

	if len(applicableTasks) == 0 {
		result.Error = fmt.Errorf("no recovery tasks available for corruption type %s", event.Type)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result
	}

	// Attempt recovery with each applicable task
	for attempt := 1; attempt <= m.config.MaxRecoveryAttempts; attempt++ {
		for _, task := range applicableTasks {
			taskResult := m.executeRecoveryTask(task, event)
			result.TaskResults = append(result.TaskResults, taskResult)

			if taskResult.Success {
				result.Success = true
				result.RecoveryMethod = task.Name
				result.Message = fmt.Sprintf("Successfully recovered using %s in attempt %d", task.Name, attempt)

				// Mark event as recovered
				event.Recovered = true

				result.EndTime = time.Now()
				result.Duration = result.EndTime.Sub(result.StartTime)
				return result
			}
		}

		// Wait before next attempt
		if attempt < m.config.MaxRecoveryAttempts {
			time.Sleep(m.config.RecoveryRetryDelay)
		}
	}

	result.Error = fmt.Errorf("failed to recover after %d attempts", m.config.MaxRecoveryAttempts)
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result
}

// RestoreFromBackup restores a file from backup
func (m *CorruptionRecoveryManager) RestoreFromBackup(filePath string) error {
	if !m.config.EnableBackupRestore {
		return fmt.Errorf("backup restore is disabled")
	}

	// Find the most recent backup
	backupPath := m.findMostRecentBackup(filePath)
	if backupPath == "" {
		return fmt.Errorf("no backup found for file: %s", filePath)
	}

	// Restore from backup
	backupData, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup file %s: %w", backupPath, err)
	}

	// Verify backup integrity
	if event := m.detector.VerifyData(backupData, ""); event != nil {
		return fmt.Errorf("backup file is also corrupted: %s", backupPath)
	}

	// Write restored data
	if err := os.WriteFile(filePath, backupData, 0644); err != nil {
		return fmt.Errorf("failed to restore file %s: %w", filePath, err)
	}

	// Update checksum cache
	_, err = m.detector.CalculateFileChecksum(filePath)
	if err != nil {
		return fmt.Errorf("failed to update checksum after restore: %w", err)
	}

	return nil
}

// CreateBackup creates a backup of a file
func (m *CorruptionRecoveryManager) CreateBackup(filePath string) error {
	// Ensure backup directory exists
	if err := os.MkdirAll(m.config.BackupDirectory, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Generate backup filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	baseName := filepath.Base(filePath)
	backupPath := filepath.Join(m.config.BackupDirectory, fmt.Sprintf("%s.%s.backup", baseName, timestamp))

	// Read original file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file for backup: %w", err)
	}

	// Write backup
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	// Create backup metadata
	metadataPath := backupPath + ".metadata"
	metadata := fmt.Sprintf(`{
	"original_path": "%s",
	"backup_time": "%s",
	"size": %d,
	"checksum": "%s"
}`, filePath, time.Now().Format(time.RFC3339), len(data), m.detector.CalculateChecksum(data))

	if err := os.WriteFile(metadataPath, []byte(metadata), 0644); err != nil {
		// Log error but don't fail backup
		fmt.Printf("Warning: failed to write backup metadata: %v\n", err)
	}

	return nil
}

// GetRecoveryTasks returns the list of registered recovery tasks
func (m *CorruptionRecoveryManager) GetRecoveryTasks() []CorruptionRecoveryTask {
	return append([]CorruptionRecoveryTask(nil), m.recoveryTasks...) // Return a copy
}

// Private methods

func (m *CorruptionRecoveryManager) executeRecoveryTask(task CorruptionRecoveryTask, event *CorruptionEvent) CorruptionRecoveryTaskResult {
	result := CorruptionRecoveryTaskResult{
		TaskName: task.Name,
		Success:  false,
	}

	startTime := time.Now()

	err := task.Execute(event)

	result.Duration = time.Since(startTime)

	if err != nil {
		result.Error = err
		result.Message = fmt.Sprintf("Task failed: %v", err)
	} else {
		result.Success = true
		result.Message = "Task completed successfully"
	}

	return result
}

func (m *CorruptionRecoveryManager) findMostRecentBackup(filePath string) string {
	baseName := filepath.Base(filePath)
	pattern := filepath.Join(m.config.BackupDirectory, baseName+"*.backup")

	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return ""
	}

	// Find the most recent backup (simple approach - could be improved)
	var mostRecent string
	var mostRecentTime time.Time

	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}

		if mostRecent == "" || info.ModTime().After(mostRecentTime) {
			mostRecent = match
			mostRecentTime = info.ModTime()
		}
	}

	return mostRecent
}

func (m *CorruptionRecoveryManager) registerDefaultRecoveryTasks() {
	// Task 1: Restore from backup
	m.RegisterRecoveryTask(CorruptionRecoveryTask{
		Name:     "restore_from_backup",
		Priority: 1,
		CanRecover: func(event *CorruptionEvent) bool {
			return m.config.EnableBackupRestore && event.Location.File != ""
		},
		Execute: func(event *CorruptionEvent) error {
			return m.RestoreFromBackup(event.Location.File)
		},
	})

	// Task 2: Restore from redundant copy
	m.RegisterRecoveryTask(CorruptionRecoveryTask{
		Name:     "restore_from_redundant_copy",
		Priority: 2,
		CanRecover: func(event *CorruptionEvent) bool {
			return m.config.EnableRedundantCopy && event.Location.File != ""
		},
		Execute: func(event *CorruptionEvent) error {
			return m.restoreFromRedundantCopy(event.Location.File)
		},
	})

	// Task 3: Partial recovery (for partial corruption)
	m.RegisterRecoveryTask(CorruptionRecoveryTask{
		Name:     "partial_recovery",
		Priority: 3,
		CanRecover: func(event *CorruptionEvent) bool {
			return m.config.EnablePartialRecovery &&
				event.Type == CorruptionTypePartial &&
				event.Location.File != ""
		},
		Execute: func(event *CorruptionEvent) error {
			return m.attemptPartialRecovery(event)
		},
	})

	// Task 4: Recreate from source (if possible)
	m.RegisterRecoveryTask(CorruptionRecoveryTask{
		Name:     "recreate_from_source",
		Priority: 4,
		CanRecover: func(event *CorruptionEvent) bool {
			// This would depend on the specific file type and whether it can be recreated
			return event.Location.File != "" && m.canRecreateFromSource(event.Location.File)
		},
		Execute: func(event *CorruptionEvent) error {
			return m.recreateFromSource(event.Location.File)
		},
	})
}

func (m *CorruptionRecoveryManager) restoreFromRedundantCopy(filePath string) error {
	// Look for redundant copies (e.g., .copy, .replica files)
	redundantPaths := []string{
		filePath + ".copy",
		filePath + ".replica",
		filePath + ".backup",
	}

	for _, redundantPath := range redundantPaths {
		if _, err := os.Stat(redundantPath); err == nil {
			// Verify redundant copy
			data, err := os.ReadFile(redundantPath)
			if err != nil {
				continue
			}

			if event := m.detector.VerifyData(data, ""); event == nil {
				// Redundant copy is valid, restore from it
				if err := os.WriteFile(filePath, data, 0644); err != nil {
					return fmt.Errorf("failed to restore from redundant copy: %w", err)
				}

				// Update checksum cache
				_, err = m.detector.CalculateFileChecksum(filePath)
				return err
			}
		}
	}

	return fmt.Errorf("no valid redundant copy found for file: %s", filePath)
}

func (m *CorruptionRecoveryManager) attemptPartialRecovery(event *CorruptionEvent) error {
	// This is a placeholder for partial recovery logic
	// In a real implementation, this would:
	// 1. Identify the corrupted portion
	// 2. Attempt to recover uncorrupted parts
	// 3. Reconstruct the file with recovered data

	return fmt.Errorf("partial recovery not implemented")
}

func (m *CorruptionRecoveryManager) canRecreateFromSource(filePath string) bool {
	// Check if this is a file that can be recreated from source
	// For example: index files, cache files, derived data

	ext := filepath.Ext(filePath)
	recreatableExtensions := []string{".idx", ".cache", ".tmp", ".derived"}

	for _, recreatable := range recreatableExtensions {
		if ext == recreatable {
			return true
		}
	}

	return false
}

func (m *CorruptionRecoveryManager) recreateFromSource(filePath string) error {
	// This is a placeholder for recreation logic
	// In a real implementation, this would:
	// 1. Identify the source data
	// 2. Regenerate the corrupted file
	// 3. Verify the regenerated file

	return fmt.Errorf("recreation from source not implemented")
}
