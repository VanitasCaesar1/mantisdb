# Data Integrity System

The integrity package provides comprehensive data integrity verification, corruption detection, and monitoring capabilities for MantisDB. This system is designed to ensure data safety and detect corruption in real-time.

## Features

- **Checksum Engine**: Multiple checksum algorithms (CRC32, MD5, SHA256)
- **Corruption Detection**: Real-time and background corruption detection
- **WAL Integrity**: Write-Ahead Log entry verification and repair
- **Monitoring**: Comprehensive metrics, health checks, and alerting
- **Alert System**: Configurable alert handlers for different notification methods

## Components

### 1. ChecksumEngine

Provides data integrity verification through checksums:

```go
engine := integrity.NewChecksumEngine(integrity.ChecksumCRC32)

// Calculate checksum
data := []byte("important data")
checksum := engine.Calculate(data)

// Verify data integrity
err := engine.Verify(data, checksum)
if err != nil {
    // Handle corruption
}
```

### 2. CorruptionDetector

Detects data corruption in real-time and through background scanning:

```go
config := integrity.DefaultIntegrityConfig()
detector := integrity.NewCorruptionDetector(config)

// Real-time corruption detection
event := detector.DetectCorruption(data, expectedChecksum)
if event != nil {
    // Handle corruption event
    fmt.Printf("Corruption detected: %s\n", event.Description)
}
```

For complete documentation, see the [integrity package documentation](../integrity/).