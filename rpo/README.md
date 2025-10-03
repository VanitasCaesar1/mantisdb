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

### Checkpoint Management

```go
// Create checkpoint manager
checkpointConfig := checkpoint.ProductionCheckpointConfig()
manager, err := checkpoint.NewManager(checkpointConfig)
if err != nil {
    log.Fatal(err)
}

// Create a checkpoint
checkpoint, err := manager.CreateCheckpoint(checkpoint.CheckpointTypeFull)
if err != nil {
    log.Fatal(err)
}

// List checkpoints
checkpoints, err := manager.ListCheckpoints(nil)
if err != nil {
    log.Fatal(err)
}
```

### Recovery Operations

```go
// Create recovery engine
recoveryEngine := checkpoint.NewRecoveryEngine(
    checkpointManager,
    walReader,
    dataRestorer,
)

// Recover to latest checkpoint
result, err := recoveryEngine.RecoverToLatestCheckpoint()
if err != nil {
    log.Fatal(err)
}

// Recover to specific point in time
targetTime := time.Now().Add(-1 * time.Hour)
result, err = recoveryEngine.RecoverToPointInTime(targetTime)
if err != nil {
    log.Fatal(err)
}
```

## Configuration

### RPO Configuration

```go
config := &rpo.RPOConfig{
    Level:               rpo.RPOLow,
    MaxDataLoss:         5 * time.Second,
    CheckpointFrequency: 30 * time.Second,
    WALSyncFrequency:    1 * time.Second,
    MonitoringInterval:  5 * time.Second,
    AlertThreshold:      3 * time.Second,
    CriticalThreshold:   4 * time.Second,
    EnableStrictMode:    true,
    EnableEmergencyMode: true,
}
```

### Checkpoint Configuration

```go
config := &checkpoint.CheckpointConfig{
    CheckpointDir:        "data/checkpoints",
    MaxCheckpoints:       20,
    CheckpointInterval:   2 * time.Minute,
    LSNInterval:          500,
    EnableCompression:    true,
    ValidateOnCreate:     true,
    AutoCleanup:          true,
    RetentionPeriod:      7 * 24 * time.Hour,
}
```

## Monitoring and Alerting

### RPO Compliance Monitoring

```go
// Check current compliance
compliance, err := manager.CheckCompliance()
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Current RPO: %v\n", compliance.CurrentRPO)
fmt.Printf("Is Compliant: %v\n", compliance.IsCompliant)
```

### Alert Configuration

```go
alertConfig := &rpo.AlertConfig{
    EnableRPOViolationAlerts: true,
    EnableCriticalAlerts:     true,
    MinAlertInterval:         1 * time.Minute,
    LogAlerts:                true,
    EmailAlerts:              true,
    EmailRecipients:          []string{"admin@example.com"},
}

alertManager := rpo.NewDefaultAlertManager(alertConfig)
```

### Statistics and Metrics

```go
// Get RPO statistics
stats := manager.GetStats()
fmt.Printf("Current RPO: %v\n", stats.CurrentRPO)
fmt.Printf("Compliance Ratio: %.2f%%\n", stats.ComplianceRatio*100)
fmt.Printf("Total Violations: %d\n", stats.TotalViolations)

// Get checkpoint statistics
checkpointStats := checkpointManager.GetStats()
fmt.Printf("Total Checkpoints: %d\n", checkpointStats.TotalCheckpoints)
fmt.Printf("Average Size: %d bytes\n", checkpointStats.AverageSize)
```

## Best Practices

### Production Deployment

1. **Use appropriate RPO levels**: Choose based on business requirements
2. **Monitor compliance**: Set up alerting for RPO violations
3. **Regular validation**: Validate checkpoints periodically
4. **Test recovery**: Regularly test recovery procedures
5. **Capacity planning**: Monitor disk usage and performance

### Performance Optimization

1. **Checkpoint frequency**: Balance between RPO and performance
2. **Compression**: Enable for large databases
3. **Parallel operations**: Use for better performance
4. **Cleanup policies**: Implement proper retention policies

### Security Considerations

1. **Encryption**: Enable for sensitive data
2. **Access control**: Restrict checkpoint access
3. **Audit logging**: Log all recovery operations
4. **Backup verification**: Validate backup integrity

## Error Handling

The system provides comprehensive error handling:

- **Recoverable errors**: Automatic retry with exponential backoff
- **Non-recoverable errors**: Immediate failure with detailed reporting
- **Partial failures**: Continue operation where possible
- **Validation failures**: Detailed error reporting and recovery suggestions

## Integration

### WAL Integration

The RPO system integrates with the Write-Ahead Log (WAL) system:

```go
type WALManager interface {
    Sync() error
    GetLastSyncTime() (time.Time, error)
    GetLastLSN() (uint64, error)
    GetUncommittedDataAge() (time.Duration, error)
}
```

### Storage Integration

Integration with the storage layer:

```go
type DataRestorer interface {
    RestoreFromSnapshot(reader io.Reader) error
    ApplyWALEntry(entry WALEntry) error
    ValidateDataConsistency() error
    GetCurrentLSN() (uint64, error)
}
```

## Testing

The package includes comprehensive tests:

```bash
# Run all tests
go test ./rpo/...

# Run with coverage
go test -cover ./rpo/...

# Run integration tests
go test -tags=integration ./rpo/...
```

## Troubleshooting

### Common Issues

1. **RPO violations**: Check checkpoint frequency and WAL sync settings
2. **Recovery failures**: Validate checkpoint integrity
3. **Performance issues**: Adjust checkpoint size and frequency
4. **Disk space**: Monitor and clean up old checkpoints

### Debug Information

Enable detailed logging:

```go
config.ReportProgress = true
config.DetailedProgress = true
```

### Health Checks

```go
// Check system health
compliance, _ := manager.CheckCompliance()
stats := manager.GetStats()

if !compliance.IsCompliant {
    log.Printf("RPO violation: %v > %v", compliance.CurrentRPO, compliance.MaxAllowedRPO)
}

if stats.ActiveViolations > 0 {
    log.Printf("Active violations: %d", stats.ActiveViolations)
}
```

## Future Enhancements

- **Distributed checkpoints**: Support for distributed systems
- **Cloud storage**: Integration with cloud storage providers
- **Advanced compression**: Support for additional compression algorithms
- **Machine learning**: Predictive RPO optimization
- **Real-time metrics**: Enhanced monitoring and dashboards