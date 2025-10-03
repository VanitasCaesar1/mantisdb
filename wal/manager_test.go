package wal

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWALManager_BasicOperations(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "wal_manager_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create WAL manager
	config := DefaultWALFileManagerConfig()
	config.WALDir = tempDir
	config.MaxFileSize = 1024
	config.SyncMode = SyncModeSync

	manager, err := NewWALFileManager(config)
	if err != nil {
		t.Fatalf("Failed to create WAL manager: %v", err)
	}
	defer manager.Close()

	// Test WriteEntry
	entry := &WALEntry{
		TxnID: 1,
		Operation: Operation{
			Type:  OpInsert,
			Key:   "test_key",
			Value: []byte("test_value"),
		},
		Timestamp: time.Now(),
	}

	err = manager.WriteEntry(entry)
	if err != nil {
		t.Fatalf("Failed to write entry: %v", err)
	}

	// Verify LSN was assigned
	if entry.LSN == 0 {
		t.Error("LSN was not assigned to entry")
	}

	// Test GetCurrentLSN
	currentLSN := manager.GetCurrentLSN()
	if currentLSN != entry.LSN {
		t.Errorf("Current LSN mismatch: expected %d, got %d", entry.LSN, currentLSN)
	}

	// Test GetNextLSN
	nextLSN := manager.GetNextLSN()
	if nextLSN != currentLSN+1 {
		t.Errorf("Next LSN should be %d, got %d", currentLSN+1, nextLSN)
	}
}

func TestWALManager_WriteBatch(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wal_batch_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultWALFileManagerConfig()
	config.WALDir = tempDir

	manager, err := NewWALFileManager(config)
	if err != nil {
		t.Fatalf("Failed to create WAL manager: %v", err)
	}
	defer manager.Close()

	// Create batch of entries
	entries := []*WALEntry{
		{TxnID: 1, Operation: Operation{Type: OpInsert, Key: "key1", Value: []byte("value1")}, Timestamp: time.Now()},
		{TxnID: 1, Operation: Operation{Type: OpCommit}, Timestamp: time.Now()},
		{TxnID: 2, Operation: Operation{Type: OpUpdate, Key: "key1", Value: []byte("value2"), OldValue: []byte("value1")}, Timestamp: time.Now()},
	}

	err = manager.WriteBatch(entries)
	if err != nil {
		t.Fatalf("Failed to write batch: %v", err)
	}

	// Verify all entries got LSNs
	for i, entry := range entries {
		if entry.LSN == 0 {
			t.Errorf("Entry %d did not get LSN assigned", i)
		}
		if i > 0 && entry.LSN <= entries[i-1].LSN {
			t.Errorf("Entry %d LSN not sequential: %d <= %d", i, entry.LSN, entries[i-1].LSN)
		}
	}
}

func TestWALManager_FileRotation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wal_rotation_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultWALFileManagerConfig()
	config.WALDir = tempDir
	config.MaxFileSize = 100 // Very small to force rotation

	manager, err := NewWALFileManager(config)
	if err != nil {
		t.Fatalf("Failed to create WAL manager: %v", err)
	}
	defer manager.Close()

	// Write enough entries to trigger rotation
	for i := 0; i < 20; i++ {
		entry := &WALEntry{
			TxnID: uint64(i + 1),
			Operation: Operation{
				Type:  OpInsert,
				Key:   "key_with_long_name_to_trigger_rotation",
				Value: []byte("this is a long value to help trigger file rotation"),
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
}

func TestWALManager_Sync(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wal_sync_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultWALFileManagerConfig()
	config.WALDir = tempDir
	config.SyncMode = SyncModeAsync

	manager, err := NewWALFileManager(config)
	if err != nil {
		t.Fatalf("Failed to create WAL manager: %v", err)
	}
	defer manager.Close()

	// Write entry
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

	// Test sync
	err = manager.Sync()
	if err != nil {
		t.Fatalf("Failed to sync: %v", err)
	}
}

func TestWALManager_Cleanup(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wal_cleanup_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultWALFileManagerConfig()
	config.WALDir = tempDir
	config.MaxFileSize = 50
	config.RetentionPeriod = 10 * time.Millisecond

	manager, err := NewWALFileManager(config)
	if err != nil {
		t.Fatalf("Failed to create WAL manager: %v", err)
	}
	defer manager.Close()

	// Write entries to create multiple files
	for i := 0; i < 10; i++ {
		entry := &WALEntry{
			TxnID: uint64(i + 1),
			Operation: Operation{
				Type:  OpInsert,
				Key:   "key",
				Value: []byte("value_to_force_rotation"),
			},
			Timestamp: time.Now(),
		}

		err = manager.WriteEntry(entry)
		if err != nil {
			t.Fatalf("Failed to write entry %d: %v", i, err)
		}
		time.Sleep(2 * time.Millisecond) // Small delay to age files
	}

	// Wait for files to age
	time.Sleep(20 * time.Millisecond)

	// Trigger cleanup
	err = manager.CleanupOldFiles()
	if err != nil {
		t.Fatalf("Failed to cleanup old files: %v", err)
	}

	// Verify archive directory exists
	archiveDir := filepath.Join(tempDir, "archive")
	if _, err := os.Stat(archiveDir); os.IsNotExist(err) {
		t.Error("Archive directory was not created")
	}
}

func TestWALManager_Status(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wal_status_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultWALFileManagerConfig()
	config.WALDir = tempDir

	manager, err := NewWALFileManager(config)
	if err != nil {
		t.Fatalf("Failed to create WAL manager: %v", err)
	}
	defer manager.Close()

	// Write some entries
	for i := 0; i < 5; i++ {
		entry := &WALEntry{
			TxnID: uint64(i + 1),
			Operation: Operation{
				Type:  OpInsert,
				Key:   "key",
				Value: []byte("value"),
			},
			Timestamp: time.Now(),
		}

		err = manager.WriteEntry(entry)
		if err != nil {
			t.Fatalf("Failed to write entry %d: %v", i, err)
		}
	}

	// Verify current LSN
	currentLSN := manager.GetCurrentLSN()
	if currentLSN != 5 {
		t.Errorf("Expected current LSN 5, got %d", currentLSN)
	}
}

func TestWALManager_ErrorHandling(t *testing.T) {
	// Test with invalid directory
	config := DefaultWALFileManagerConfig()
	config.WALDir = "/invalid/path/that/does/not/exist"

	_, err := NewWALFileManager(config)
	if err == nil {
		t.Error("Expected error for invalid WAL directory")
	}

	// Test with nil entry
	tempDir, err := os.MkdirTemp("", "wal_error_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config.WALDir = tempDir
	manager, err := NewWALFileManager(config)
	if err != nil {
		t.Fatalf("Failed to create WAL manager: %v", err)
	}
	defer manager.Close()

	err = manager.WriteEntry(nil)
	if err == nil {
		t.Error("Expected error for nil entry")
	}

	// Test with nil batch
	err = manager.WriteBatch(nil)
	if err == nil {
		t.Error("Expected error for nil batch")
	}

	// Test with empty batch
	err = manager.WriteBatch([]*WALEntry{})
	if err == nil {
		t.Error("Expected error for empty batch")
	}
}

func TestWALManager_ConcurrentWrites(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wal_concurrent_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultWALFileManagerConfig()
	config.WALDir = tempDir

	manager, err := NewWALFileManager(config)
	if err != nil {
		t.Fatalf("Failed to create WAL manager: %v", err)
	}
	defer manager.Close()

	// Test concurrent writes
	const numGoroutines = 10
	const entriesPerGoroutine = 10
	done := make(chan bool, numGoroutines)

	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer func() { done <- true }()

			for i := 0; i < entriesPerGoroutine; i++ {
				entry := &WALEntry{
					TxnID: uint64(goroutineID*100 + i),
					Operation: Operation{
						Type:  OpInsert,
						Key:   "key",
						Value: []byte("value"),
					},
					Timestamp: time.Now(),
				}

				if err := manager.WriteEntry(entry); err != nil {
					t.Errorf("Goroutine %d failed to write entry %d: %v", goroutineID, i, err)
					return
				}
			}
		}(g)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify final LSN
	expectedLSN := uint64(numGoroutines * entriesPerGoroutine)
	actualLSN := manager.GetCurrentLSN()
	if actualLSN != expectedLSN {
		t.Errorf("Expected final LSN %d, got %d", expectedLSN, actualLSN)
	}
}
