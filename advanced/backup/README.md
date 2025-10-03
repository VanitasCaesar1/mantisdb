# MantisDB Hot Backup System

This package implements a comprehensive hot backup system for MantisDB that allows creating consistent backups without downtime using copy-on-write semantics and WAL checkpoints.

## Features

### 1. Snapshot Manager (`manager.go`)
- **Consistent Snapshot Creation**: Uses WAL checkpoints to ensure consistency
- **Copy-on-Write Semantics**: Allows concurrent operations during backup
- **Page-level Tracking**: Tracks data pages with reference counting
- **Integrity Verification**: Built-in checksum validation

### 2. Backup Streaming (`streaming.go`)
- **Multiple Destinations**: Support for file, S3, GCS, Azure destinations
- **Compression Support**: Built-in gzip compression with extensible architecture
- **Progress Tracking**: Real-time progress monitoring with ETA calculation
- **Integrity Verification**: Checksum validation during streaming

### 3. Backup Scheduling (`scheduler.go`)
- **Cron-like Scheduling**: Flexible scheduling with cron expressions
- **Retention Policies**: Configurable retention with daily/weekly/monthly/yearly rules
- **Retry Logic**: Automatic retry with exponential backoff
- **Notification Support**: Extensible notification system

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Hot Backup System                        │
├─────────────────────────────────────────────────────────────┤
│  Snapshot Manager                                           │
│  ├── WAL Checkpoint Integration                             │
│  ├── Copy-on-Write Page Management                          │
│  └── Consistency Guarantees                                 │
├─────────────────────────────────────────────────────────────┤
│  Backup Streamer                                            │
│  ├── Multi-destination Support                              │
│  ├── Compression & Verification                             │
│  └── Progress Monitoring                                     │
├─────────────────────────────────────────────────────────────┤
│  Backup Scheduler                                           │
│  ├── Cron-based Scheduling                                  │
│  ├── Retention Management                                   │
│  └── Failure Handling                                       │
└─────────────────────────────────────────────────────────────┘
```

## Usage

### Basic Usage

```go
// Create backup system
backupSystem, err := NewBackupSystem(walMgr, storageEngine, checkpointMgr)
if err != nil {
    log.Fatal(err)
}

// Start the system
backupSystem.Start()
defer backupSystem.Stop()

// Create manual backup
destination := BackupDestination{
    Type:     "file",
    Location: "/backups/manual_backup.dat",
}

tags := map[string]string{
    "type": "manual",
    "environment": "production",
}

backupInfo, err := backupSystem.CreateManualBackup(ctx, destination, tags)
```

### Scheduled Backups

```go
// Create daily backup schedule
schedule := &BackupSchedule{
    ID:          "daily_backup",
    Name:        "Daily Production Backup",
    CronExpr:    "0 2 * * *", // Daily at 2 AM
    Enabled:     true,
    Destination: BackupDestination{
        Type:     "file",
        Location: "/backups/daily/backup_{{.Date}}.dat",
    },
    Options: BackupOptions{
        CompressionType: "gzip",
        VerifyChecksum:  true,
        Timeout:         2 * time.Hour,
    },
    Retention: RetentionPolicy{
        KeepDaily:   7,
        KeepWeekly:  4,
        KeepMonthly: 12,
        MaxAge:      365 * 24 * time.Hour,
    },
}

err := backupSystem.Scheduler.CreateSchedule(schedule)
```

## Hot Backup Process

1. **Checkpoint Creation**: Creates a WAL checkpoint to establish a consistent point
2. **Snapshot Initialization**: Captures current LSN and creates snapshot metadata
3. **Copy-on-Write Setup**: Sets up page references for concurrent operation handling
4. **Data Streaming**: Streams data to destination while handling concurrent writes
5. **Integrity Verification**: Validates backup integrity using checksums
6. **Cleanup**: Removes temporary files and copy-on-write pages

## Copy-on-Write Mechanism

When a backup is in progress and a write operation occurs:

1. **Page Detection**: System detects write to a page referenced by active snapshots
2. **Copy Creation**: Creates a copy of the original page before modification
3. **Reference Update**: Updates page references to point to the copy
4. **Write Completion**: Allows the write operation to proceed on new pages
5. **Backup Continuation**: Backup continues reading from original pages
6. **Cleanup**: Removes copied pages after backup completion

## Configuration

### Snapshot Configuration
```go
config := &SnapshotConfig{
    SnapshotDir:     "data/snapshots",
    TempDir:         "data/temp",
    MaxConcurrent:   3,
    BufferSize:      64 * 1024,
    VerifyChecksum:  true,
    CompressionType: "gzip",
    Timeout:         30 * time.Minute,
}
```

### Streaming Configuration
```go
config := &StreamingConfig{
    BufferSize:      1024 * 1024,
    CompressionType: "gzip",
    VerifyChecksum:  true,
    MaxConcurrent:   5,
    Timeout:         2 * time.Hour,
    RetryAttempts:   3,
    RetryDelay:      30 * time.Second,
}
```

### Retention Configuration
```go
policy := &RetentionPolicy{
    KeepDaily:   7,  // Keep 7 daily backups
    KeepWeekly:  4,  // Keep 4 weekly backups
    KeepMonthly: 12, // Keep 12 monthly backups
    KeepYearly:  5,  // Keep 5 yearly backups
    MaxAge:      365 * 24 * time.Hour,
    MaxCount:    100,
}
```

## Supported Destinations

- **File System**: Local and network file systems
- **Amazon S3**: AWS S3 buckets (extensible)
- **Google Cloud Storage**: GCS buckets (extensible)
- **Azure Blob Storage**: Azure storage accounts (extensible)

## Monitoring and Observability

The backup system provides comprehensive monitoring:

- **Progress Tracking**: Real-time progress with transfer rates and ETA
- **Status Monitoring**: Backup status (creating, streaming, completed, failed)
- **Metrics Collection**: Backup sizes, durations, success/failure rates
- **Error Reporting**: Detailed error information with recovery suggestions

## Error Handling

The system includes robust error handling:

- **Retry Logic**: Automatic retry with configurable attempts and delays
- **Graceful Degradation**: Continues operation even if some backups fail
- **Cleanup on Failure**: Automatic cleanup of partial backups and temporary files
- **Notification System**: Alerts for backup failures and successes

## Performance Considerations

- **Minimal Impact**: Hot backups have minimal impact on database performance
- **Concurrent Operations**: Database remains fully operational during backups
- **Resource Management**: Configurable limits on concurrent backups and memory usage
- **Compression**: Optional compression to reduce storage requirements

## Requirements Satisfied

This implementation satisfies the following requirements from the specification:

- **Requirement 3.1**: Consistent snapshot creation without blocking operations
- **Requirement 3.2**: Concurrent read/write requests during backup
- **Requirement 3.3**: Backup integrity verification using checksums
- **Requirement 3.4**: Automated backup scheduling with retention policies

## Future Enhancements

- **Incremental Backups**: Support for incremental backup strategies
- **Encryption**: Built-in encryption for backup data
- **Deduplication**: Data deduplication to reduce storage requirements
- **Cloud Integration**: Enhanced cloud provider integrations
- **Monitoring Dashboard**: Web-based monitoring and management interface