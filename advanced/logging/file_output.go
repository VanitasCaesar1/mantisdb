package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FileOutput writes log entries to files with rotation support
type FileOutput struct {
	filename    string
	maxSize     int64 // Maximum file size in bytes
	maxAge      int   // Maximum age in days
	maxBackups  int   // Maximum number of backup files
	currentFile *os.File
	currentSize int64
	mutex       sync.Mutex
}

// FileOutputConfig holds configuration for file output
type FileOutputConfig struct {
	Filename   string // Base filename
	MaxSize    int64  // Maximum file size in bytes (default: 100MB)
	MaxAge     int    // Maximum age in days (default: 30)
	MaxBackups int    // Maximum number of backup files (default: 10)
}

// NewFileOutput creates a new file output with rotation
func NewFileOutput(config FileOutputConfig) (*FileOutput, error) {
	if config.MaxSize == 0 {
		config.MaxSize = 100 * 1024 * 1024 // 100MB default
	}
	if config.MaxAge == 0 {
		config.MaxAge = 30 // 30 days default
	}
	if config.MaxBackups == 0 {
		config.MaxBackups = 10 // 10 backups default
	}

	fo := &FileOutput{
		filename:   config.Filename,
		maxSize:    config.MaxSize,
		maxAge:     config.MaxAge,
		maxBackups: config.MaxBackups,
	}

	if err := fo.openFile(); err != nil {
		return nil, err
	}

	return fo, nil
}

// openFile opens the current log file
func (f *FileOutput) openFile() error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(f.filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	file, err := os.OpenFile(f.filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Get current file size
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to stat log file: %w", err)
	}

	f.currentFile = file
	f.currentSize = info.Size()

	return nil
}

// Write writes a log entry to the file
func (f *FileOutput) Write(entry *LogEntry) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	// Convert entry to JSON
	jsonOutput := NewJSONOutput(f.currentFile)
	data, err := jsonOutput.formatEntry(entry)
	if err != nil {
		return err
	}

	// Check if rotation is needed
	if f.currentSize+int64(len(data)) > f.maxSize {
		if err := f.rotate(); err != nil {
			return fmt.Errorf("failed to rotate log file: %w", err)
		}
	}

	// Write to current file
	n, err := f.currentFile.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write to log file: %w", err)
	}

	f.currentSize += int64(n)

	// Sync to ensure data is written
	if err := f.currentFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync log file: %w", err)
	}

	return nil
}

// formatEntry formats a log entry as JSON bytes
func (j *JSONOutput) formatEntry(entry *LogEntry) ([]byte, error) {
	data, err := json.Marshal(entry)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal log entry: %w", err)
	}
	return append(data, '\n'), nil
}

// rotate rotates the current log file
func (f *FileOutput) rotate() error {
	// Close current file
	if f.currentFile != nil {
		f.currentFile.Close()
	}

	// Generate backup filename with timestamp
	timestamp := time.Now().Format("2006-01-02T15-04-05")
	backupName := fmt.Sprintf("%s.%s", f.filename, timestamp)

	// Rename current file to backup
	if err := os.Rename(f.filename, backupName); err != nil {
		return fmt.Errorf("failed to rename log file: %w", err)
	}

	// Clean up old backups
	if err := f.cleanupOldBackups(); err != nil {
		// Log error but don't fail rotation
		fmt.Fprintf(os.Stderr, "Failed to cleanup old backups: %v\n", err)
	}

	// Open new file
	return f.openFile()
}

// cleanupOldBackups removes old backup files based on age and count
func (f *FileOutput) cleanupOldBackups() error {
	dir := filepath.Dir(f.filename)
	base := filepath.Base(f.filename)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read log directory: %w", err)
	}

	var backups []os.DirEntry
	cutoff := time.Now().AddDate(0, 0, -f.maxAge)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !isBackupFile(name, base) {
			continue
		}

		// Check age
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			// Remove old backup
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to remove old backup %s: %v\n", name, err)
			}
			continue
		}

		backups = append(backups, entry)
	}

	// Remove excess backups (keep only maxBackups)
	if len(backups) > f.maxBackups {
		// Sort by modification time (oldest first)
		for i := 0; i < len(backups)-f.maxBackups; i++ {
			name := backups[i].Name()
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to remove excess backup %s: %v\n", name, err)
			}
		}
	}

	return nil
}

// isBackupFile checks if a filename is a backup of the base log file
func isBackupFile(filename, base string) bool {
	if len(filename) <= len(base)+1 {
		return false
	}

	if !strings.HasPrefix(filename, base+".") {
		return false
	}

	// Check if the suffix looks like a timestamp
	suffix := filename[len(base)+1:]
	_, err := time.Parse("2006-01-02T15-04-05", suffix)
	return err == nil
}

// Close closes the file output
func (f *FileOutput) Close() error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if f.currentFile != nil {
		return f.currentFile.Close()
	}

	return nil
}
