# MantisDB Hot Backup System

This package implements a comprehensive hot backup system for MantisDB that allows creating consistent backups without downtime using copy-on-write semantics and WAL checkpoints.

## Features

### 1. Snapshot Manager
- **Consistent Snapshot Creation**: Uses WAL checkpoints to ensure consistency
- **Copy-on-Write Semantics**: Allows concurrent operations during backup
- **Page-level Tracking**: Tracks data pages with reference counting
- **Integrity Verification**: Built-in checksum validation

### 2. Backup Streaming
- **Multiple Destinations**: Support for file, S3, GCS, Azure destinations
- **Compression Support**: Built-in gzip compression with extensible architecture
- **Progress Tracking**: Real-time progress monitoring with ETA calculation
- **Integrity Verification**: Checksum validation during streaming

### 3. Backup Scheduling
- **Cron-like Scheduling**: Flexible scheduling with cron expressions
- **Retention Policies**: Configurable retention with daily/weekly/monthly/yearly rules
- **Retry Logic**: Automatic retry with exponential backoff
- **Notification Support**: Extensible notification system

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

For complete documentation, see the [backup package documentation](../advanced/backup/).