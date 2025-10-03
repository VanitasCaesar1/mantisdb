package backup

import (
	"context"
	"fmt"
	"log"
	"time"
)

// BackupSystem integrates all backup components
type BackupSystem struct {
	SnapshotManager *SnapshotManager
	Streamer        *BackupStreamer
	Scheduler       *BackupScheduler
	Verifier        *BackupVerifier
}

// NewBackupSystem creates a complete backup system
func NewBackupSystem(walMgr interface{}, storageEngine interface{},
	checkpointMgr interface{}) (*BackupSystem, error) {

	// Create snapshot manager
	snapshotConfig := DefaultSnapshotConfig()
	snapshotMgr, err := NewSnapshotManager(snapshotConfig, walMgr, storageEngine, checkpointMgr)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot manager: %w", err)
	}

	// Create backup streamer
	streamingConfig := DefaultStreamingConfig()
	streamer := NewBackupStreamer(streamingConfig)

	// Create backup scheduler
	schedulerConfig := DefaultSchedulerConfig()
	scheduler, err := NewBackupScheduler(schedulerConfig, snapshotMgr, streamer)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup scheduler: %w", err)
	}

	// Create backup verifier
	verificationConfig := DefaultVerificationConfig()
	verifier := NewBackupVerifier(verificationConfig)

	return &BackupSystem{
		SnapshotManager: snapshotMgr,
		Streamer:        streamer,
		Scheduler:       scheduler,
		Verifier:        verifier,
	}, nil
}

// Start starts all backup system components
func (bs *BackupSystem) Start() error {
	if err := bs.Scheduler.Start(); err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}

	log.Println("Backup system started successfully")
	return nil
}

// Stop stops all backup system components
func (bs *BackupSystem) Stop() error {
	if err := bs.Scheduler.Stop(); err != nil {
		return fmt.Errorf("failed to stop scheduler: %w", err)
	}

	if err := bs.SnapshotManager.Close(); err != nil {
		return fmt.Errorf("failed to close snapshot manager: %w", err)
	}

	log.Println("Backup system stopped successfully")
	return nil
}

// CreateManualBackup creates a manual backup immediately
func (bs *BackupSystem) CreateManualBackup(ctx context.Context, destination BackupDestination,
	tags map[string]string) (*BackupInfo, error) {

	// Create snapshot
	snapshot, err := bs.SnapshotManager.CreateSnapshot(ctx, tags)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot: %w", err)
	}

	// Wait for snapshot to complete
	if err := bs.waitForSnapshotCompletion(ctx, snapshot.ID); err != nil {
		return nil, fmt.Errorf("snapshot creation failed: %w", err)
	}

	// Stream backup
	stream, err := bs.Streamer.StreamBackup(ctx, snapshot.ID, destination)
	if err != nil {
		return nil, fmt.Errorf("failed to start backup stream: %w", err)
	}

	// Wait for stream to complete
	if err := bs.waitForStreamCompletion(ctx, stream.ID); err != nil {
		return nil, fmt.Errorf("backup streaming failed: %w", err)
	}

	// Get final stream info
	finalStream, err := bs.Streamer.GetStream(stream.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get final stream info: %w", err)
	}

	// Create backup info
	backupInfo := &BackupInfo{
		ID:          fmt.Sprintf("manual_%d", time.Now().Unix()),
		SnapshotID:  snapshot.ID,
		StreamID:    stream.ID,
		Destination: destination,
		Size:        finalStream.Progress.BytesWritten,
		Checksum:    finalStream.Checksum,
		CreatedAt:   snapshot.CreatedAt,
		CompletedAt: *finalStream.EndTime,
		Tags:        tags,
		Metadata:    make(map[string]interface{}),
	}

	return backupInfo, nil
}

// waitForSnapshotCompletion waits for a snapshot to complete
func (bs *BackupSystem) waitForSnapshotCompletion(ctx context.Context, snapshotID string) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			snapshot, err := bs.SnapshotManager.GetSnapshot(snapshotID)
			if err != nil {
				return err
			}

			switch snapshot.Status {
			case "completed":
				return nil
			case "failed":
				return fmt.Errorf("snapshot failed: %s", snapshot.Error)
			}
		}
	}
}

// waitForStreamCompletion waits for a backup stream to complete
func (bs *BackupSystem) waitForStreamCompletion(ctx context.Context, streamID string) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			stream, err := bs.Streamer.GetStream(streamID)
			if err != nil {
				return err
			}

			switch stream.Status {
			case "completed":
				return nil
			case "failed":
				return fmt.Errorf("stream failed: %s", stream.Error)
			case "cancelled":
				return fmt.Errorf("stream was cancelled")
			}
		}
	}
}

// ExampleUsage demonstrates how to use the backup system
func ExampleUsage() {
	// This example shows how to use the backup system

	// Initialize dependencies (these would be real implementations)
	var walMgr interface{}
	var storageEngine interface{}
	var checkpointMgr interface{}

	// Create backup system
	backupSystem, err := NewBackupSystem(walMgr, storageEngine, checkpointMgr)
	if err != nil {
		log.Fatalf("Failed to create backup system: %v", err)
	}

	// Start the system
	if err := backupSystem.Start(); err != nil {
		log.Fatalf("Failed to start backup system: %v", err)
	}
	defer backupSystem.Stop()

	// Example 1: Create a manual backup
	ctx := context.Background()
	destination := BackupDestination{
		Type:     "file",
		Location: "/backups/manual_backup.dat",
		Options:  make(map[string]string),
	}

	tags := map[string]string{
		"type":        "manual",
		"environment": "production",
		"reason":      "pre-deployment",
	}

	backupInfo, err := backupSystem.CreateManualBackup(ctx, destination, tags)
	if err != nil {
		log.Printf("Manual backup failed: %v", err)
	} else {
		log.Printf("Manual backup completed: %s", backupInfo.ID)
	}

	// Example 2: Create a scheduled backup
	schedule := &BackupSchedule{
		ID:          "daily_backup",
		Name:        "Daily Production Backup",
		Description: "Daily backup of production database",
		CronExpr:    "0 2 * * *", // Daily at 2 AM
		Enabled:     true,
		Destination: BackupDestination{
			Type:     "file",
			Location: "/backups/daily/backup_{{.Date}}.dat",
			Options:  make(map[string]string),
		},
		Options: BackupOptions{
			CompressionType: "gzip",
			VerifyChecksum:  true,
			Timeout:         2 * time.Hour,
			Tags: map[string]string{
				"type":        "scheduled",
				"frequency":   "daily",
				"environment": "production",
			},
			Priority: 1,
		},
		Retention: RetentionPolicy{
			KeepDaily:   7,
			KeepWeekly:  4,
			KeepMonthly: 12,
			MaxAge:      365 * 24 * time.Hour,
			MaxCount:    50,
		},
		Tags: map[string]string{
			"schedule_type": "production_daily",
		},
	}

	if err := backupSystem.Scheduler.CreateSchedule(schedule); err != nil {
		log.Printf("Failed to create schedule: %v", err)
	} else {
		log.Printf("Created backup schedule: %s", schedule.ID)
	}

	// Example 3: Verify a backup
	verificationResult, err := backupSystem.Verifier.VerifyBackup(
		ctx,
		"/backups/manual_backup.dat",
		backupInfo.Checksum,
	)
	if err != nil {
		log.Printf("Backup verification failed: %v", err)
	} else if verificationResult.Valid {
		log.Printf("Backup verification passed")
	} else {
		log.Printf("Backup verification failed: %v", verificationResult.Errors)
	}

	// Example 4: List active snapshots
	snapshots := backupSystem.SnapshotManager.ListSnapshots()
	log.Printf("Active snapshots: %d", len(snapshots))
	for _, snapshot := range snapshots {
		log.Printf("  - %s: %s (%s)", snapshot.ID, snapshot.Status, snapshot.Timestamp.Format(time.RFC3339))
	}

	// Example 5: List backup schedules
	schedules := backupSystem.Scheduler.ListSchedules()
	log.Printf("Backup schedules: %d", len(schedules))
	for _, sched := range schedules {
		status := "disabled"
		if sched.Enabled {
			status = "enabled"
		}
		log.Printf("  - %s: %s (%s) - %s", sched.ID, sched.Name, sched.CronExpr, status)
	}

	log.Println("Backup system example completed")
}

// ExampleHotBackupDuringOperations demonstrates hot backup during active operations
func ExampleHotBackupDuringOperations() {
	log.Println("=== Hot Backup During Operations Example ===")

	// This example shows how the backup system handles concurrent operations
	// during backup creation using copy-on-write semantics

	// Simulate active database operations
	_ = context.Background()

	// Start a backup
	log.Println("1. Starting hot backup...")

	// Create snapshot (this would use WAL checkpoint for consistency)
	_ = map[string]string{
		"type": "hot_backup",
		"test": "concurrent_operations",
	}

	// Simulate the snapshot creation process
	log.Println("2. Creating consistent snapshot using WAL checkpoint...")
	log.Println("   - Current LSN captured")
	log.Println("   - WAL synced to disk")
	log.Println("   - Checkpoint created")

	// Simulate concurrent operations during backup
	log.Println("3. Simulating concurrent database operations...")
	log.Println("   - INSERT operation on table 'users'")
	log.Println("   - Copy-on-write triggered for affected pages")
	log.Println("   - Original pages preserved for backup")
	log.Println("   - New data written to new pages")

	log.Println("4. Backup streaming in progress...")
	log.Println("   - Reading from original pages (snapshot view)")
	log.Println("   - Concurrent operations continue normally")
	log.Println("   - No blocking of read/write operations")

	log.Println("5. Backup completed successfully")
	log.Println("   - Snapshot integrity verified")
	log.Println("   - Checksum validation passed")
	log.Println("   - Copy-on-write pages cleaned up")

	log.Println("=== Hot Backup Example Completed ===")
}

// ExampleBackupRecovery demonstrates backup recovery process
func ExampleBackupRecovery() {
	log.Println("=== Backup Recovery Example ===")

	// This example shows how to recover from a backup

	log.Println("1. Disaster scenario detected")
	log.Println("2. Selecting backup for recovery...")
	log.Println("   - Latest backup: backup_20250103_020000.dat")
	log.Println("   - Backup size: 2.5 GB")
	log.Println("   - Backup date: 2025-01-03 02:00:00")

	log.Println("3. Verifying backup integrity...")
	log.Println("   - Checksum verification: PASSED")
	log.Println("   - Structure validation: PASSED")
	log.Println("   - Compression integrity: PASSED")

	log.Println("4. Starting recovery process...")
	log.Println("   - Stopping database engine")
	log.Println("   - Clearing data directory")
	log.Println("   - Extracting backup data")
	log.Println("   - Restoring database files")

	log.Println("5. Recovery completed successfully")
	log.Println("   - Database restored to LSN: 1234567")
	log.Println("   - Data integrity verified")
	log.Println("   - Database ready for operations")

	log.Println("=== Backup Recovery Example Completed ===")
}
