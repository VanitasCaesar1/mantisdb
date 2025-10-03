package wal

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// WALFileManager manages WAL files including writing, rotation, and cleanup
type WALFileManager struct {
	mu sync.RWMutex

	// Configuration
	walDir          string        // Directory for WAL files
	maxFileSize     int64         // Maximum size before rotation
	maxFileAge      time.Duration // Maximum age before rotation
	bufferSize      int           // Buffer size for writes
	syncMode        SyncMode      // Sync behavior
	retentionPeriod time.Duration // How long to keep old WAL files
	archiveDir      string        // Directory for archived WAL files

	// Current state
	currentFile    *WALFile      // Current active WAL file
	currentFileNum uint64        // Current file number
	nextLSN        uint64        // Next LSN to assign
	lastSyncTime   time.Time     // Last time we synced to disk
	syncInterval   time.Duration // How often to sync in async mode

	// File tracking
	activeFiles   map[uint64]*WALFile // Active WAL files by file number
	archivedFiles []string            // List of archived file paths
}

// WALFile represents a single WAL file
type WALFile struct {
	mu sync.RWMutex

	fileNum    uint64    // File number
	filePath   string    // Full path to the file
	file       *os.File  // File handle
	writer     io.Writer // Buffered or direct writer
	size       int64     // Current file size
	createdAt  time.Time // When the file was created
	lastWrite  time.Time // Last write time
	minLSN     uint64    // Minimum LSN in this file
	maxLSN     uint64    // Maximum LSN in this file
	entryCount int64     // Number of entries in this file
	closed     bool      // Whether the file is closed
}

// SyncMode defines how writes are synchronized to disk
type SyncMode int

const (
	SyncModeAsync SyncMode = iota // Buffer writes, sync periodically
	SyncModeSync                  // Sync every write
	SyncModeBatch                 // Sync after each batch
)

// WALFileManagerConfig holds configuration for the WAL file manager
type WALFileManagerConfig struct {
	WALDir          string        // Directory for WAL files
	MaxFileSize     int64         // Maximum file size (default: 64MB)
	MaxFileAge      time.Duration // Maximum file age (default: 1 hour)
	BufferSize      int           // Buffer size (default: 64KB)
	SyncMode        SyncMode      // Sync mode (default: SyncModeAsync)
	RetentionPeriod time.Duration // Retention period (default: 24 hours)
	ArchiveDir      string        // Archive directory (default: walDir/archive)
	SyncInterval    time.Duration // Sync interval for async mode (default: 1 second)
}

// DefaultWALFileManagerConfig returns default configuration
func DefaultWALFileManagerConfig() *WALFileManagerConfig {
	return &WALFileManagerConfig{
		WALDir:          "data/wal",
		MaxFileSize:     64 * 1024 * 1024, // 64MB
		MaxFileAge:      time.Hour,        // 1 hour
		BufferSize:      64 * 1024,        // 64KB
		SyncMode:        SyncModeAsync,
		RetentionPeriod: 24 * time.Hour, // 24 hours
		SyncInterval:    time.Second,    // 1 second
	}
}

// NewWALFileManager creates a new WAL file manager
func NewWALFileManager(config *WALFileManagerConfig) (*WALFileManager, error) {
	if config == nil {
		config = DefaultWALFileManagerConfig()
	}

	// Set default archive directory if not specified
	archiveDir := config.ArchiveDir
	if archiveDir == "" {
		archiveDir = filepath.Join(config.WALDir, "archive")
	}

	// Create directories if they don't exist
	if err := os.MkdirAll(config.WALDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create WAL directory: %w", err)
	}
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create archive directory: %w", err)
	}

	manager := &WALFileManager{
		walDir:          config.WALDir,
		maxFileSize:     config.MaxFileSize,
		maxFileAge:      config.MaxFileAge,
		bufferSize:      config.BufferSize,
		syncMode:        config.SyncMode,
		retentionPeriod: config.RetentionPeriod,
		archiveDir:      archiveDir,
		syncInterval:    config.SyncInterval,
		activeFiles:     make(map[uint64]*WALFile),
		nextLSN:         1, // Start from LSN 1
	}

	// Initialize by scanning existing files
	if err := manager.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize WAL file manager: %w", err)
	}

	// Start background sync routine for async mode
	if config.SyncMode == SyncModeAsync {
		go manager.backgroundSync()
	}

	return manager, nil
}

// initialize scans existing WAL files and sets up initial state
func (wfm *WALFileManager) initialize() error {
	// Scan WAL directory for existing files
	files, err := filepath.Glob(filepath.Join(wfm.walDir, "wal-*.log"))
	if err != nil {
		return fmt.Errorf("failed to scan WAL directory: %w", err)
	}

	// Parse file numbers and find the highest
	var maxFileNum uint64 = 0
	for _, filePath := range files {
		fileName := filepath.Base(filePath)
		var fileNum uint64
		if _, err := fmt.Sscanf(fileName, "wal-%d.log", &fileNum); err != nil {
			continue // Skip files that don't match pattern
		}
		if fileNum > maxFileNum {
			maxFileNum = fileNum
		}
	}

	// Set next file number
	wfm.currentFileNum = maxFileNum

	// If we have existing files, open the latest one
	if maxFileNum > 0 {
		latestFile := filepath.Join(wfm.walDir, fmt.Sprintf("wal-%d.log", maxFileNum))
		walFile, err := wfm.openExistingFile(latestFile, maxFileNum)
		if err != nil {
			return fmt.Errorf("failed to open latest WAL file: %w", err)
		}
		wfm.currentFile = walFile
		wfm.activeFiles[maxFileNum] = walFile

		// Set next LSN based on the file's max LSN
		wfm.nextLSN = walFile.maxLSN + 1
	}

	// Create initial file if none exists
	if wfm.currentFile == nil {
		if err := wfm.rotateFile(); err != nil {
			return fmt.Errorf("failed to create initial WAL file: %w", err)
		}
	}

	return nil
}

// openExistingFile opens an existing WAL file and reads its metadata
func (wfm *WALFileManager) openExistingFile(filePath string, fileNum uint64) (*WALFile, error) {
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}

	walFile := &WALFile{
		fileNum:   fileNum,
		filePath:  filePath,
		file:      file,
		size:      stat.Size(),
		createdAt: stat.ModTime(), // Approximate creation time
		lastWrite: stat.ModTime(),
		minLSN:    0, // Will be determined by scanning if needed
		maxLSN:    0, // Will be determined by scanning if needed
	}

	// Set up writer based on sync mode
	walFile.writer = wfm.createWriter(file)

	// For existing files, we'd need to scan to determine LSN range
	// For now, we'll set a reasonable default and let it be updated during writes
	walFile.maxLSN = wfm.nextLSN - 1

	return walFile, nil
}

// createWriter creates the appropriate writer based on sync mode
func (wfm *WALFileManager) createWriter(file *os.File) io.Writer {
	switch wfm.syncMode {
	case SyncModeSync:
		return &syncWriter{file: file}
	case SyncModeAsync, SyncModeBatch:
		return &bufferedWriter{
			file:       file,
			bufferSize: wfm.bufferSize,
		}
	default:
		return file
	}
}

// WriteEntry writes a WAL entry to the current file
func (wfm *WALFileManager) WriteEntry(entry *WALEntry) error {
	wfm.mu.Lock()
	defer wfm.mu.Unlock()

	// Assign LSN if not set
	if entry.LSN == 0 {
		entry.LSN = wfm.nextLSN
		wfm.nextLSN++
	}

	// Check if we need to rotate the file
	if wfm.shouldRotate() {
		if err := wfm.rotateFile(); err != nil {
			return fmt.Errorf("failed to rotate WAL file: %w", err)
		}
	}

	// Serialize the entry
	data, err := entry.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize WAL entry: %w", err)
	}

	// Write to current file
	if err := wfm.currentFile.writeEntry(data, entry.LSN); err != nil {
		return fmt.Errorf("failed to write WAL entry: %w", err)
	}

	// Sync if in sync mode
	if wfm.syncMode == SyncModeSync {
		if err := wfm.currentFile.sync(); err != nil {
			return fmt.Errorf("failed to sync WAL file: %w", err)
		}
	}

	return nil
}

// WriteBatch writes multiple WAL entries as a batch
func (wfm *WALFileManager) WriteBatch(entries []*WALEntry) error {
	wfm.mu.Lock()
	defer wfm.mu.Unlock()

	for _, entry := range entries {
		// Assign LSN if not set
		if entry.LSN == 0 {
			entry.LSN = wfm.nextLSN
			wfm.nextLSN++
		}

		// Check if we need to rotate the file
		if wfm.shouldRotate() {
			if err := wfm.rotateFile(); err != nil {
				return fmt.Errorf("failed to rotate WAL file: %w", err)
			}
		}

		// Serialize the entry
		data, err := entry.Serialize()
		if err != nil {
			return fmt.Errorf("failed to serialize WAL entry: %w", err)
		}

		// Write to current file
		if err := wfm.currentFile.writeEntry(data, entry.LSN); err != nil {
			return fmt.Errorf("failed to write WAL entry: %w", err)
		}
	}

	// Sync if in batch sync mode
	if wfm.syncMode == SyncModeBatch {
		if err := wfm.currentFile.sync(); err != nil {
			return fmt.Errorf("failed to sync WAL file: %w", err)
		}
	}

	return nil
}

// shouldRotate determines if the current file should be rotated
func (wfm *WALFileManager) shouldRotate() bool {
	if wfm.currentFile == nil {
		return true
	}

	// Check file size
	if wfm.currentFile.size >= wfm.maxFileSize {
		return true
	}

	// Check file age
	if time.Since(wfm.currentFile.createdAt) >= wfm.maxFileAge {
		return true
	}

	return false
}

// rotateFile creates a new WAL file and closes the current one
func (wfm *WALFileManager) rotateFile() error {
	// Close current file if it exists
	if wfm.currentFile != nil {
		if err := wfm.currentFile.close(); err != nil {
			return fmt.Errorf("failed to close current WAL file: %w", err)
		}
	}

	// Increment file number
	wfm.currentFileNum++

	// Create new file
	fileName := fmt.Sprintf("wal-%d.log", wfm.currentFileNum)
	filePath := filepath.Join(wfm.walDir, fileName)

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to create new WAL file: %w", err)
	}

	walFile := &WALFile{
		fileNum:   wfm.currentFileNum,
		filePath:  filePath,
		file:      file,
		writer:    wfm.createWriter(file),
		size:      0,
		createdAt: time.Now(),
		lastWrite: time.Now(),
		minLSN:    wfm.nextLSN,
		maxLSN:    wfm.nextLSN - 1, // Will be updated on first write
	}

	wfm.currentFile = walFile
	wfm.activeFiles[wfm.currentFileNum] = walFile

	return nil
}

// Sync forces a sync of the current WAL file
func (wfm *WALFileManager) Sync() error {
	wfm.mu.RLock()
	defer wfm.mu.RUnlock()

	if wfm.currentFile == nil {
		return nil
	}

	return wfm.currentFile.sync()
}

// GetCurrentLSN returns the current LSN
func (wfm *WALFileManager) GetCurrentLSN() uint64 {
	wfm.mu.RLock()
	defer wfm.mu.RUnlock()
	return wfm.nextLSN - 1
}

// GetNextLSN returns the next LSN that will be assigned
func (wfm *WALFileManager) GetNextLSN() uint64 {
	wfm.mu.RLock()
	defer wfm.mu.RUnlock()
	return wfm.nextLSN
}

// CleanupOldFiles removes old WAL files based on retention policy
func (wfm *WALFileManager) CleanupOldFiles() error {
	wfm.mu.Lock()
	defer wfm.mu.Unlock()

	cutoffTime := time.Now().Add(-wfm.retentionPeriod)

	// Find files to clean up
	var filesToCleanup []uint64
	for fileNum, walFile := range wfm.activeFiles {
		if walFile != wfm.currentFile && walFile.createdAt.Before(cutoffTime) {
			filesToCleanup = append(filesToCleanup, fileNum)
		}
	}

	// Archive and remove old files
	for _, fileNum := range filesToCleanup {
		walFile := wfm.activeFiles[fileNum]

		// Archive the file
		if err := wfm.archiveFile(walFile); err != nil {
			return fmt.Errorf("failed to archive WAL file %d: %w", fileNum, err)
		}

		// Remove from active files
		delete(wfm.activeFiles, fileNum)
	}

	// Also cleanup old archived files
	return wfm.cleanupArchivedFiles()
}

// archiveFile moves a WAL file to the archive directory
func (wfm *WALFileManager) archiveFile(walFile *WALFile) error {
	// Close the file first
	if err := walFile.close(); err != nil {
		return fmt.Errorf("failed to close WAL file before archiving: %w", err)
	}

	// Create archive file path
	fileName := filepath.Base(walFile.filePath)
	archivePath := filepath.Join(wfm.archiveDir, fileName)

	// Move file to archive
	if err := os.Rename(walFile.filePath, archivePath); err != nil {
		return fmt.Errorf("failed to move WAL file to archive: %w", err)
	}

	// Add to archived files list
	wfm.archivedFiles = append(wfm.archivedFiles, archivePath)

	return nil
}

// cleanupArchivedFiles removes archived files older than retention period
func (wfm *WALFileManager) cleanupArchivedFiles() error {
	cutoffTime := time.Now().Add(-wfm.retentionPeriod)

	var remainingFiles []string
	for _, filePath := range wfm.archivedFiles {
		stat, err := os.Stat(filePath)
		if err != nil {
			// File doesn't exist, skip it
			continue
		}

		if stat.ModTime().Before(cutoffTime) {
			// Remove old archived file
			if err := os.Remove(filePath); err != nil {
				return fmt.Errorf("failed to remove archived WAL file %s: %w", filePath, err)
			}
		} else {
			remainingFiles = append(remainingFiles, filePath)
		}
	}

	wfm.archivedFiles = remainingFiles
	return nil
}

// ListActiveFiles returns information about active WAL files
func (wfm *WALFileManager) ListActiveFiles() []WALFileInfo {
	wfm.mu.RLock()
	defer wfm.mu.RUnlock()

	var files []WALFileInfo
	for _, walFile := range wfm.activeFiles {
		walFile.mu.RLock()
		info := WALFileInfo{
			FileNum:    walFile.fileNum,
			FilePath:   walFile.filePath,
			Size:       walFile.size,
			CreatedAt:  walFile.createdAt,
			LastWrite:  walFile.lastWrite,
			MinLSN:     walFile.minLSN,
			MaxLSN:     walFile.maxLSN,
			EntryCount: walFile.entryCount,
			IsCurrent:  walFile == wfm.currentFile,
		}
		walFile.mu.RUnlock()
		files = append(files, info)
	}

	// Sort by file number
	sort.Slice(files, func(i, j int) bool {
		return files[i].FileNum < files[j].FileNum
	})

	return files
}

// ListArchivedFiles returns information about archived WAL files
func (wfm *WALFileManager) ListArchivedFiles() []string {
	wfm.mu.RLock()
	defer wfm.mu.RUnlock()

	// Return a copy to avoid race conditions
	archived := make([]string, len(wfm.archivedFiles))
	copy(archived, wfm.archivedFiles)
	return archived
}

// backgroundSync runs periodic sync for async mode
func (wfm *WALFileManager) backgroundSync() {
	ticker := time.NewTicker(wfm.syncInterval)
	defer ticker.Stop()

	for range ticker.C {
		wfm.mu.RLock()
		currentFile := wfm.currentFile
		wfm.mu.RUnlock()

		if currentFile != nil {
			currentFile.sync()
		}
	}
}

// Close closes the WAL file manager and all open files
func (wfm *WALFileManager) Close() error {
	wfm.mu.Lock()
	defer wfm.mu.Unlock()

	var errors []string

	// Close all active files
	for _, walFile := range wfm.activeFiles {
		if err := walFile.close(); err != nil {
			errors = append(errors, err.Error())
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors closing WAL files: %s", strings.Join(errors, "; "))
	}

	return nil
}

// WALFileInfo contains information about a WAL file
type WALFileInfo struct {
	FileNum    uint64
	FilePath   string
	Size       int64
	CreatedAt  time.Time
	LastWrite  time.Time
	MinLSN     uint64
	MaxLSN     uint64
	EntryCount int64
	IsCurrent  bool
}

// writeEntry writes entry data to the WAL file
func (wf *WALFile) writeEntry(data []byte, lsn uint64) error {
	wf.mu.Lock()
	defer wf.mu.Unlock()

	if wf.closed {
		return fmt.Errorf("cannot write to closed WAL file")
	}

	// Write the data
	n, err := wf.writer.Write(data)
	if err != nil {
		return err
	}

	// Update file metadata
	wf.size += int64(n)
	wf.lastWrite = time.Now()
	wf.entryCount++

	// Update LSN range
	if wf.minLSN == 0 || lsn < wf.minLSN {
		wf.minLSN = lsn
	}
	if lsn > wf.maxLSN {
		wf.maxLSN = lsn
	}

	return nil
}

// sync forces a sync of the WAL file to disk
func (wf *WALFile) sync() error {
	wf.mu.Lock()
	defer wf.mu.Unlock()

	if wf.closed {
		return nil
	}

	// If we have a buffered writer, flush it first
	if bw, ok := wf.writer.(*bufferedWriter); ok {
		if err := bw.flush(); err != nil {
			return err
		}
	}

	// Sync the file to disk
	return wf.file.Sync()
}

// close closes the WAL file
func (wf *WALFile) close() error {
	wf.mu.Lock()
	defer wf.mu.Unlock()

	if wf.closed {
		return nil
	}

	// Flush any buffered data
	if bw, ok := wf.writer.(*bufferedWriter); ok {
		if err := bw.flush(); err != nil {
			return err
		}
	}

	// Sync and close the file
	if err := wf.file.Sync(); err != nil {
		return err
	}

	err := wf.file.Close()
	wf.closed = true
	return err
}

// syncWriter is a writer that syncs after every write
type syncWriter struct {
	file *os.File
}

func (sw *syncWriter) Write(data []byte) (int, error) {
	n, err := sw.file.Write(data)
	if err != nil {
		return n, err
	}

	// Sync after every write
	if err := sw.file.Sync(); err != nil {
		return n, err
	}

	return n, nil
}

// bufferedWriter is a writer that buffers writes
type bufferedWriter struct {
	file       *os.File
	buffer     []byte
	bufferSize int
	pos        int
}

func (bw *bufferedWriter) Write(data []byte) (int, error) {
	if bw.buffer == nil {
		bw.buffer = make([]byte, bw.bufferSize)
	}

	totalWritten := 0
	remaining := data

	for len(remaining) > 0 {
		// Calculate how much we can write to the buffer
		available := bw.bufferSize - bw.pos
		toWrite := len(remaining)
		if toWrite > available {
			toWrite = available
		}

		// Copy data to buffer
		copy(bw.buffer[bw.pos:], remaining[:toWrite])
		bw.pos += toWrite
		totalWritten += toWrite
		remaining = remaining[toWrite:]

		// If buffer is full, flush it
		if bw.pos >= bw.bufferSize {
			if err := bw.flush(); err != nil {
				return totalWritten, err
			}
		}
	}

	return totalWritten, nil
}

func (bw *bufferedWriter) flush() error {
	if bw.pos == 0 {
		return nil
	}

	_, err := bw.file.Write(bw.buffer[:bw.pos])
	bw.pos = 0
	return err
}
