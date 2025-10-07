# Recovery Point Objective (RPO) System

This package implements a comprehensive Recovery Point Objective (RPO) system for MantisDB, providing configurable data loss protection and automated recovery capabilities.

## Overview

The RPO system consists of several key components:

- **RPO Manager**: Monitors and enforces RPO compliance
- **Checkpoint System**: Creates and manages data checkpoints
- **Recovery Engine**: Handles checkpoint-based recovery
- **Alerting System**: Provides notifications for RPO violations
- **Configuration Management**: Flexible RPO and checkpoint configuration

## Key Features

### RPO Levels

The system supports five RPO levels:

- **Zero RPO**: No data loss tolerance (RPO = 0)
- **Minimal RPO**: Very low data loss (RPO < 1 second)
- **Low RPO**: Low data loss (RPO < 5 seconds)
- **Medium RPO**: Medium data loss (RPO < 30 seconds)
- **High RPO**: High data loss tolerance (RPO < 5 minutes)

### Checkpoint Types

- **Full Checkpoints**: Complete database snapshots
- **Incremental Checkpoints**: Changes since last checkpoint
- **Snapshot Checkpoints**: Point-in-time snapshots

### Recovery Capabilities

- **Checkpoint-based Recovery**: Restore from any checkpoint
- **Point-in-time Recovery**: Recover to specific timestamp
- **Incremental Recovery**: Combine checkpoints with WAL replay
- **Validation**: Comprehensive data consistency checks

## Usage Examples

### Basic RPO Setup

```go
// Create RPO configuration
config := rpo.ProductionRPOConfig()

// Create RPO manager
manager, err := rpo.NewManager(config)
if err != nil {
    log.Fatal(err)
}

// Set dependencies
manager.SetCheckpointManager(checkpointManager)
manager.SetWALManager(walManager)
manager.SetAlertManager(alertManager)

// Start monitoring
ctx := context.Background()
if err := manager.Start(ctx); err != nil {
    log.Fatal(err)
}
defer manager.Stop()
```

For complete documentation, see the [RPO package documentation](../rpo/).