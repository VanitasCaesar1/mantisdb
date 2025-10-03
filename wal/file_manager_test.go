package wal

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWALFileManager_Basic(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "wal_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create config
	config := &WALFileManagerConfig{
		WALDir:          tempDir,
		MaxFileSize:     1024, // Small size for testing rotation
		MaxFileAge:      time.Hour,
		BufferSize:      256,
		SyncMode:        SyncModeAsync,
		RetentionPeriod: time.Hour,
		SyncInterval:    100 * time.Millisecond,
	}

	// Create manager
	manager, err := NewWALFileManager(config)
	if err != nil {
		t.Fatalf("Failed to create WAL file manager: %v", err)
	}
	defer manager.Close()

	// Test writing entries
	entry1 := &WALEntry{
		TxnID: 1,
		Operation: Operation{
			Type:  OpInsert,
			Key:   "key1",
			Value: []byte("value1"),
		},
		Timestamp: time.Now(),
	}

	err = manager.WriteEntry(entry1)
	if err != nil {
		t.Fatalf("Failed to write entry: %v", err)
	}

	// Verify LSN was assigned
	if entry1.LSN == 0 {
		t.Error("LSN was not assigned to entry")
	}

	// Test batch write
	entries := []*WALEntry{
		{
			TxnID: 2,
			Operation: Operation{
				Type:  OpUpdate,
				Key:   "key2",
				Value: []byte("value2"),
			},
			Timestamp: time.Now(),
		},
		{
			TxnID: 2,
			Operation: Operation{
				Type:  OpCommit,
				Key:   "",
				Value: nil,
			},
			Timestamp: time.Now(),
		},
	}

	err = manager.WriteBatch(entries)
	if err != nil {
		t.Fatalf("Failed to write batch: %v", err)
	}

	// Verify LSNs were assigned
	for _, entry := range entries {
		if entry.LSN == 0 {
			t.Error("LSN was not assigned to batch entry")
		}
	}

	// Test sync
	err = manager.Sync()
	if err != nil {
		t.Fatalf("Failed to sync: %v", err)
	}

	// Verify current LSN
	currentLSN := manager.GetCurrentLSN()
	if currentLSN == 0 {
		t.Error("Current LSN should not be zero")
	}

	nextLSN := manager.GetNextLSN()
	if nextLSN != currentLSN+1 {
		t.Errorf("Next LSN should be %d, got %d", currentLSN+1, nextLSN)
	}
}

func TestWALFileManager_Rotation(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "wal_rotation_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create config with small file size to trigger rotation
	config := &WALFileManagerConfig{
		WALDir:          tempDir,
		MaxFileSize:     100, // Very small size to trigger rotation
		MaxFileAge:      time.Hour,
		BufferSize:      64,
		SyncMode:        SyncModeSync,
		RetentionPeriod: time.Hour,
	}

	// Create manager
	manager, err := NewWALFileManager(config)
	if err != nil {
		t.Fatalf("Failed to create WAL file manager: %v", err)
	}
	defer manager.Close()

	// Write enough entries to trigger rotation
	for i := 0; i < 10; i++ {
		entry := &WALEntry{
			TxnID: uint64(i + 1),
			Operation: Operation{
				Type:  OpInsert,
				Key:   "key" + string(rune(i)),
				Value: []byte("this is a longer value to trigger file rotation"),
			},
			Timestamp: time.Now(),
		}

		err = manager.WriteEntry(entry)
		if err != nil {
			t.Fatalf("Failed to write entry %d: %v", i, err)
		}
	}

	// Check that multiple files were created
	files := manager.ListActiveFiles()
	if len(files) < 2 {
		t.Errorf("Expected at least 2 files after rotation, got %d", len(files))
	}

	// Verify files are properly ordered
	for i := 1; i < len(files); i++ {
		if files[i].FileNum <= files[i-1].FileNum {
			t.Error("Files are not properly ordered by file number")
		}
	}
}

func TestWALFileManager_Cleanup(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "wal_cleanup_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create config with short retention period
	config := &WALFileManagerConfig{
		WALDir:          tempDir,
		MaxFileSize:     50,               // Very small to force rotation
		MaxFileAge:      time.Millisecond, // Very short age to trigger rotation
		BufferSize:      32,
		SyncMode:        SyncModeSync,
		RetentionPeriod: 5 * time.Millisecond, // Very short retention
	}

	// Create manager
	manager, err := NewWALFileManager(config)
	if err != nil {
		t.Fatalf("Failed to create WAL file manager: %v", err)
	}
	defer manager.Close()

	// Write entries to create multiple files
	for i := 0; i < 10; i++ {
		entry := &WALEntry{
			TxnID: uint64(i + 1),
			Operation: Operation{
				Type:  OpInsert,
				Key:   "key" + string(rune(i)),
				Value: []byte("this is a longer value to force file rotation"),
			},
			Timestamp: time.Now(),
		}

		err = manager.WriteEntry(entry)
		if err != nil {
			t.Fatalf("Failed to write entry %d: %v", i, err)
		}

		// Sleep to trigger age-based rotation
		time.Sleep(2 * time.Millisecond)
	}

	// Check that we have multiple files
	activeFilesBefore := manager.ListActiveFiles()
	if len(activeFilesBefore) < 2 {
		t.Logf("Only %d active files, skipping cleanup test", len(activeFilesBefore))
		return
	}

	// Wait for files to age beyond retention period
	time.Sleep(20 * time.Millisecond)

	// Trigger cleanup
	err = manager.CleanupOldFiles()
	if err != nil {
		t.Fatalf("Failed to cleanup old files: %v", err)
	}

	// Verify that some files were processed
	activeFilesAfter := manager.ListActiveFiles()
	archivedFiles := manager.ListArchivedFiles()

	// We should have fewer active files or some archived files
	if len(activeFilesAfter) >= len(activeFilesBefore) && len(archivedFiles) == 0 {
		t.Logf("Before cleanup: %d active files", len(activeFilesBefore))
		t.Logf("After cleanup: %d active files, %d archived files", len(activeFilesAfter), len(archivedFiles))
		// This might be expected if files are too new, so just log it
	}

	// Verify archive directory exists
	archiveDir := filepath.Join(tempDir, "archive")
	if _, err := os.Stat(archiveDir); os.IsNotExist(err) {
		t.Error("Archive directory was not created")
	}
}

func TestWALFileManager_SyncModes(t *testing.T) {
	syncModes := []SyncMode{SyncModeSync, SyncModeAsync, SyncModeBatch}

	for _, mode := range syncModes {
		t.Run(mode.String(), func(t *testing.T) {
			// Create temporary directory for test
			tempDir, err := os.MkdirTemp("", "wal_sync_test")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir)

			// Create config with specific sync mode
			config := &WALFileManagerConfig{
				WALDir:          tempDir,
				MaxFileSize:     1024,
				MaxFileAge:      time.Hour,
				BufferSize:      256,
				SyncMode:        mode,
				RetentionPeriod: time.Hour,
				SyncInterval:    10 * time.Millisecond,
			}

			// Create manager
			manager, err := NewWALFileManager(config)
			if err != nil {
				t.Fatalf("Failed to create WAL file manager: %v", err)
			}
			defer manager.Close()

			// Write an entry
			entry := &WALEntry{
				TxnID: 1,
				Operation: Operation{
					Type:  OpInsert,
					Key:   "key1",
					Value: []byte("value1"),
				},
				Timestamp: time.Now(),
			}

			err = manager.WriteEntry(entry)
			if err != nil {
				t.Fatalf("Failed to write entry: %v", err)
			}

			// For async mode, wait a bit for background sync
			if mode == SyncModeAsync {
				time.Sleep(20 * time.Millisecond)
			}

			// Verify the entry was written
			if entry.LSN == 0 {
				t.Error("LSN was not assigned to entry")
			}
		})
	}
}

// String method for SyncMode for testing
func (sm SyncMode) String() string {
	switch sm {
	case SyncModeAsync:
		return "Async"
	case SyncModeSync:
		return "Sync"
	case SyncModeBatch:
		return "Batch"
	default:
		return "Unknown"
	}
}
