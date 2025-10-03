# Critical Data Safety Design

## Overview

This design document outlines the implementation of critical data safety features for MantisDB, including Write-Ahead Logging (WAL), crash recovery, ACID transactions, comprehensive error handling, and data integrity mechanisms. These features are essential for production deployment and data safety.

## Architecture

### High-Level Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Client API    │    │  Transaction    │    │   WAL Manager   │
│                 │    │    Manager      │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  Storage Layer  │◄──►│  Lock Manager   │◄──►│ Recovery Engine │
│                 │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ Checksum Engine │    │ Error Handler   │    │   Monitoring    │
│                 │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## Components and Interfaces

### 1. Write-Ahead Log (WAL) Manager

The WAL Manager handles all write-ahead logging operations:

```go
type WALManager interface {
    // Write operations
    WriteEntry(entry *WALEntry) error
    WriteBatch(entries []*WALEntry) error
    
    // Recovery operations
    Replay(fromLSN uint64) error
    GetLastLSN() uint64
    
    // Maintenance operations
    Checkpoint() error
    Rotate() error
    Cleanup(beforeLSN uint64) error
    
    // Status operations
    GetStatus() *WALStatus
    Close() error
}

type WALEntry struct {
    LSN       uint64    // Log Sequence Number
    TxnID     uint64    // Transaction ID
    Operation Operation // The operation being logged
    Timestamp time.Time
    Checksum  uint32    // Entry integrity checksum
}

type Operation struct {
    Type   OperationType // INSERT, UPDATE, DELETE, COMMIT, ABORT
    Key    string
    Value  []byte
    OldValue []byte // For rollback
}
```

### 2. Transaction Manager

Manages ACID transactions with proper isolation and consistency:

```go
type TransactionManager interface {
    // Transaction lifecycle
    Begin(isolation IsolationLevel) (*Transaction, error)
    Commit(txn *Transaction) error
    Abort(txn *Transaction) error
    
    // Lock management
    AcquireLock(txn *Transaction, key string, lockType LockType) error
    ReleaseLocks(txn *Transaction) error
    
    // Deadlock detection
    DetectDeadlocks() []DeadlockInfo
    ResolveDeadlock(deadlock DeadlockInfo) error
}

type Transaction struct {
    ID        uint64
    StartTime time.Time
    Status    TxnStatus
    Isolation IsolationLevel
    Operations []Operation
    Locks     []Lock
}
```

### 3. Recovery Engine

Handles crash recovery and system restoration:

```go
type RecoveryEngine interface {
    // Recovery operations
    Recover() error
    AnalyzeWAL() (*RecoveryPlan, error)
    ReplayOperations(plan *RecoveryPlan) error
    
    // Checkpoint operations
    CreateCheckpoint() error
    RestoreFromCheckpoint(checkpointID string) error
    
    // Validation
    ValidateRecovery() error
}

type RecoveryPlan struct {
    StartLSN     uint64
    EndLSN       uint64
    Operations   []Operation
    Transactions map[uint64]*Transaction
    Checkpoints  []CheckpointInfo
}
```

### 4. Lock Manager

Provides concurrency control and deadlock detection:

```go
type LockManager interface {
    // Lock operations
    AcquireLock(txnID uint64, resource string, lockType LockType) error
    ReleaseLock(txnID uint64, resource string) error
    ReleaseAllLocks(txnID uint64) error
    
    // Deadlock detection
    DetectDeadlocks() []DeadlockCycle
    BuildWaitForGraph() *WaitForGraph
    
    // Lock information
    GetLockInfo(resource string) *LockInfo
    GetBlockedTransactions() []uint64
}

type Lock struct {
    Resource    string
    TxnID       uint64
    Type        LockType
    AcquiredAt  time.Time
    WaitingTxns []uint64
}
```

### 5. Checksum Engine

Provides data integrity verification:

```go
type ChecksumEngine interface {
    // Checksum operations
    Calculate(data []byte) uint32
    Verify(data []byte, expectedChecksum uint32) error
    
    // Batch operations
    CalculateBatch(dataBlocks [][]byte) []uint32
    VerifyBatch(dataBlocks [][]byte, checksums []uint32) []error
    
    // File-level checksums
    CalculateFileChecksum(filePath string) (uint32, error)
    VerifyFileChecksum(filePath string, expectedChecksum uint32) error
}
```

### 6. Error Handler

Comprehensive error handling and recovery:

```go
type ErrorHandler interface {
    // Error handling
    HandleError(err error, context ErrorContext) ErrorAction
    RecoverFromError(err error, context ErrorContext) error
    
    // Resource exhaustion
    HandleDiskFull(operation Operation) error
    HandleMemoryExhaustion(operation Operation) error
    HandleIOError(err error, retryCount int) error
    
    // Corruption handling
    HandleCorruption(corruptionInfo CorruptionInfo) error
    IsolateCorruptedData(location DataLocation) error
}

type ErrorContext struct {
    Operation   string
    Resource    string
    Severity    ErrorSeverity
    Recoverable bool
    Metadata    map[string]interface{}
}
```

## Data Models

### WAL Entry Format

```
┌─────────────┬─────────────┬─────────────┬─────────────┬─────────────┐
│   Header    │   TxnID     │  Operation  │   Payload   │  Checksum   │
│  (8 bytes)  │  (8 bytes)  │  (4 bytes)  │ (variable)  │  (4 bytes)  │
└─────────────┴─────────────┴─────────────┴─────────────┴─────────────┘

Header: LSN (8 bytes)
TxnID: Transaction ID (8 bytes)
Operation: Operation type and flags (4 bytes)
Payload: Operation-specific data (variable length)
Checksum: CRC32 checksum of entire entry (4 bytes)
```

### Transaction Log Format

```
┌─────────────┬─────────────┬─────────────┬─────────────┐
│   TxnID     │   Status    │  StartTime  │ Operations  │
│  (8 bytes)  │  (4 bytes)  │  (8 bytes)  │ (variable)  │
└─────────────┴─────────────┴─────────────┴─────────────┘
```

### Checkpoint Format

```
┌─────────────┬─────────────┬─────────────┬─────────────┐
│ CheckpointID│   LSN       │  Timestamp  │  Metadata   │
│  (8 bytes)  │  (8 bytes)  │  (8 bytes)  │ (variable)  │
└─────────────┴─────────────┴─────────────┴─────────────┘
```

## Error Handling

### Error Classification

1. **Recoverable Errors**
   - Temporary I/O failures
   - Lock timeouts
   - Memory pressure
   - Network timeouts

2. **Non-Recoverable Errors**
   - Disk corruption
   - WAL corruption
   - Critical system failures
   - Hardware failures

3. **Resource Exhaustion**
   - Disk space full
   - Memory exhausted
   - File descriptor limits
   - Connection limits

### Error Recovery Strategies

1. **Retry with Exponential Backoff**
   - Initial delay: 100ms
   - Maximum delay: 30s
   - Maximum retries: 5

2. **Circuit Breaker Pattern**
   - Failure threshold: 5 consecutive failures
   - Recovery timeout: 60s
   - Half-open state testing

3. **Graceful Degradation**
   - Read-only mode during recovery
   - Reduced functionality under resource pressure
   - Emergency shutdown procedures

## Testing Strategy

### Unit Tests

1. **WAL Manager Tests**
   - Entry writing and reading
   - File rotation and cleanup
   - Corruption detection
   - Recovery replay

2. **Transaction Manager Tests**
   - ACID property verification
   - Deadlock detection and resolution
   - Isolation level enforcement
   - Concurrent transaction handling

3. **Recovery Engine Tests**
   - Crash simulation and recovery
   - Checkpoint creation and restoration
   - WAL replay accuracy
   - Data consistency verification

### Integration Tests

1. **End-to-End Recovery Tests**
   - Simulated crashes during operations
   - Multi-transaction recovery scenarios
   - Large dataset recovery
   - Performance under recovery load

2. **Stress Tests**
   - High-concurrency transaction processing
   - Resource exhaustion scenarios
   - Long-running transaction handling
   - System limits testing

3. **Chaos Engineering Tests**
   - Random failure injection
   - Network partition simulation
   - Hardware failure simulation
   - Time synchronization issues

### Performance Tests

1. **WAL Performance**
   - Write throughput measurement
   - Sync vs async performance
   - File rotation overhead
   - Recovery time measurement

2. **Transaction Performance**
   - Transaction throughput
   - Lock contention impact
   - Deadlock resolution time
   - Isolation overhead

## Monitoring and Observability

### Key Metrics

1. **WAL Metrics**
   - WAL write rate (entries/second)
   - WAL file size and rotation frequency
   - WAL sync latency
   - Recovery time

2. **Transaction Metrics**
   - Transaction throughput
   - Average transaction duration
   - Deadlock frequency
   - Lock wait time

3. **Error Metrics**
   - Error rate by type
   - Recovery success rate
   - Corruption detection rate
   - Resource exhaustion events

### Alerting

1. **Critical Alerts**
   - WAL write failures
   - Recovery failures
   - Data corruption detected
   - System unavailability

2. **Warning Alerts**
   - High error rates
   - Resource pressure
   - Performance degradation
   - Long-running transactions

## Security Considerations

1. **WAL Security**
   - WAL file encryption at rest
   - Secure WAL file permissions
   - WAL integrity verification
   - Audit logging of WAL operations

2. **Transaction Security**
   - Transaction isolation enforcement
   - Access control integration
   - Audit trail for transactions
   - Secure error reporting

## Deployment Considerations

1. **Configuration**
   - WAL file location and sizing
   - Checkpoint frequency
   - Recovery timeout settings
   - Error handling policies

2. **Operational Procedures**
   - Backup and restore procedures
   - Disaster recovery planning
   - Monitoring setup
   - Maintenance procedures

3. **Performance Tuning**
   - WAL buffer sizing
   - Checkpoint frequency tuning
   - Lock timeout configuration
   - I/O optimization settings