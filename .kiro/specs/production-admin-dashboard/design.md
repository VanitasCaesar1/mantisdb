# Design Document

## Overview

This design implements a comprehensive production-ready MantisDB with an integrated admin dashboard. The system will be architected as a single binary that includes both the database engine and web interface, with modular components for advanced features like hot backups, improved concurrency, memory management, logging, metrics, and client libraries.

## Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    MantisDB Binary                          │
├─────────────────────────────────────────────────────────────┤
│  Admin Dashboard (Web UI)                                  │
│  ├── React Frontend (Mantis Theme)                         │
│  ├── REST API Server                                       │
│  └── WebSocket for Real-time Updates                       │
├─────────────────────────────────────────────────────────────┤
│  Database Engine                                           │
│  ├── Hot Backup System                                     │
│  ├── Advanced Concurrency (RW Locks)                       │
│  ├── Memory Management & Cache                             │
│  ├── Structured Logging                                    │
│  ├── Metrics & Health Checks                               │
│  └── Compression Engine                                     │
├─────────────────────────────────────────────────────────────┤
│  Client Libraries                                          │
│  ├── Go SDK                                                │
│  ├── Python SDK                                            │
│  └── JavaScript SDK                                        │
└─────────────────────────────────────────────────────────────┘
```

### Component Integration

The admin dashboard will be embedded as static assets in the Go binary using `embed` package, with the database engine exposing internal APIs for dashboard operations.

## Components and Interfaces

### 1. Admin Dashboard Frontend

**Technology Stack:**
- React with TypeScript for type safety
- Tailwind CSS with custom mantis theme (green/nature colors)
- React Query for data fetching and caching
- Monaco Editor for SQL query interface
- Chart.js for metrics visualization

**Key Components:**
```typescript
interface DashboardComponents {
  DataBrowser: React.FC<{table: string}>
  QueryEditor: React.FC<{onExecute: (query: string) => void}>
  MetricsDashboard: React.FC<{metrics: SystemMetrics}>
  BackupManager: React.FC<{backups: BackupInfo[]}>
  LogViewer: React.FC<{filters: LogFilters}>
  ConfigEditor: React.FC<{config: DatabaseConfig}>
}
```

### 2. Admin API Server

**REST Endpoints:**
```go
type AdminAPI struct {
    // Data operations
    GET    /api/tables
    GET    /api/tables/{table}/data
    POST   /api/tables/{table}/data
    PUT    /api/tables/{table}/data/{id}
    DELETE /api/tables/{table}/data/{id}
    
    // Query operations
    POST   /api/query
    GET    /api/query/history
    
    // Backup operations
    GET    /api/backups
    POST   /api/backups
    GET    /api/backups/{id}/status
    POST   /api/backups/{id}/restore
    
    // Monitoring
    GET    /api/metrics
    GET    /api/health
    GET    /api/logs
    
    // Configuration
    GET    /api/config
    PUT    /api/config
}
```

### 3. Hot Backup System

**Design Pattern:** Copy-on-Write with WAL coordination

```go
type HotBackupManager struct {
    wal           *wal.Manager
    storage       storage.Interface
    snapshots     map[string]*Snapshot
    backupQueue   chan BackupRequest
}

type BackupStrategy interface {
    CreateSnapshot() (*Snapshot, error)
    WriteBackup(snapshot *Snapshot, dest io.Writer) error
    VerifyBackup(backup *BackupInfo) error
}
```

**Implementation:**
1. Create consistent snapshot using WAL checkpoint
2. Stream data to backup destination while allowing normal operations
3. Use reference counting for data pages to handle concurrent modifications
4. Verify backup integrity using checksums

### 4. Advanced Concurrency System

**Read-Write Lock Implementation:**
```go
type RWLockManager struct {
    locks    map[string]*RWLock
    deadlock *DeadlockDetector
    metrics  *LockMetrics
}

type RWLock struct {
    readers    int32
    writers    int32
    waitingW   int32
    readerSem  chan struct{}
    writerSem  chan struct{}
    mutex      sync.Mutex
}
```

**Features:**
- Reader-writer locks with writer preference
- Deadlock detection using wait-for graphs
- Lock timeout and priority-based resolution
- Granular locking at row/page level

### 5. Memory Management System

**Cache Architecture:**
```go
type CacheManager struct {
    policies map[string]EvictionPolicy
    limits   *MemoryLimits
    monitor  *MemoryMonitor
    stats    *CacheStats
}

type EvictionPolicy interface {
    Evict(cache *Cache, needed int64) []CacheEntry
    OnAccess(entry *CacheEntry)
    OnInsert(entry *CacheEntry)
}
```

**Eviction Policies:**
- LRU (Least Recently Used)
- LFU (Least Frequently Used)
- TTL (Time To Live)
- Adaptive (combines multiple strategies)

### 6. Structured Logging System

**Log Format:**
```json
{
  "timestamp": "2025-01-03T10:30:00Z",
  "level": "INFO",
  "component": "query_executor",
  "request_id": "req_123456",
  "user_id": "user_789",
  "message": "Query executed successfully",
  "duration_ms": 45,
  "query": "SELECT * FROM users WHERE active = true",
  "metadata": {
    "rows_returned": 150,
    "cache_hit": true
  }
}
```

**Implementation:**
```go
type StructuredLogger struct {
    level     LogLevel
    outputs   []LogOutput
    formatter LogFormatter
    context   map[string]interface{}
}
```

### 7. Metrics and Observability

**Prometheus Metrics:**
```go
var (
    QueryDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "mantisdb_query_duration_seconds",
            Help: "Query execution duration",
        },
        []string{"operation", "table"},
    )
    
    ActiveConnections = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "mantisdb_active_connections",
            Help: "Number of active connections",
        },
    )
)
```

**Health Check System:**
```go
type HealthChecker struct {
    checks map[string]HealthCheck
}

type HealthCheck interface {
    Name() string
    Check(ctx context.Context) HealthResult
}
```

### 8. Client Libraries

**Go Client:**
```go
type Client struct {
    conn   *grpc.ClientConn
    config *Config
    pool   *ConnectionPool
}

func (c *Client) Query(ctx context.Context, query string) (*Result, error)
func (c *Client) Insert(ctx context.Context, table string, data interface{}) error
func (c *Client) Update(ctx context.Context, table string, id string, data interface{}) error
```

**Python Client:**
```python
class MantisClient:
    def __init__(self, connection_string: str):
        self.connection = Connection(connection_string)
    
    async def query(self, sql: str) -> List[Dict[str, Any]]:
        pass
    
    def query_sync(self, sql: str) -> List[Dict[str, Any]]:
        pass
```

**JavaScript Client:**
```typescript
class MantisClient {
    constructor(config: ClientConfig) {}
    
    async query(sql: string): Promise<QueryResult>
    async insert(table: string, data: Record<string, any>): Promise<void>
    async update(table: string, id: string, data: Record<string, any>): Promise<void>
}
```

### 9. Compression System

**Compression Strategy:**
```go
type CompressionEngine struct {
    algorithms map[string]CompressionAlgorithm
    policies   []CompressionPolicy
    monitor    *CompressionMonitor
}

type CompressionAlgorithm interface {
    Compress(data []byte) ([]byte, error)
    Decompress(data []byte) ([]byte, error)
    Ratio() float64
}
```

**Cold Data Detection:**
- Track access patterns using bloom filters
- Age-based policies (data not accessed in X days)
- Size-based policies (compress large objects first)
- Manual compression triggers

## Data Models

### Dashboard State Management

```typescript
interface DashboardState {
  currentTable: string
  queryHistory: Query[]
  metrics: SystemMetrics
  backups: BackupInfo[]
  logs: LogEntry[]
  config: DatabaseConfig
}

interface SystemMetrics {
  cpu_usage: number
  memory_usage: number
  disk_usage: number
  query_latency: number[]
  active_connections: number
  cache_hit_ratio: number
}
```

### Configuration Schema

```go
type Config struct {
    Server struct {
        Port        int    `yaml:"port" default:"8080"`
        AdminPort   int    `yaml:"admin_port" default:"8081"`
        Host        string `yaml:"host" default:"localhost"`
    } `yaml:"server"`
    
    Database struct {
        DataDir     string `yaml:"data_dir" default:"./data"`
        WALDir      string `yaml:"wal_dir" default:"./wal"`
        CacheSize   string `yaml:"cache_size" default:"1GB"`
    } `yaml:"database"`
    
    Backup struct {
        Enabled     bool   `yaml:"enabled" default:"true"`
        Schedule    string `yaml:"schedule" default:"0 2 * * *"`
        Retention   int    `yaml:"retention_days" default:"30"`
        Destination string `yaml:"destination"`
    } `yaml:"backup"`
    
    Logging struct {
        Level  string `yaml:"level" default:"INFO"`
        Format string `yaml:"format" default:"json"`
        Output string `yaml:"output" default:"stdout"`
    } `yaml:"logging"`
}
```

## Error Handling

### Error Categories

1. **User Errors:** Invalid queries, permission denied, validation failures
2. **System Errors:** Storage failures, network issues, resource exhaustion
3. **Data Errors:** Corruption, consistency violations, backup failures

### Error Response Format

```json
{
  "error": {
    "code": "INVALID_QUERY",
    "message": "Syntax error in SQL query",
    "details": {
      "line": 1,
      "column": 15,
      "suggestion": "Expected WHERE clause"
    },
    "request_id": "req_123456"
  }
}
```

## Testing Strategy

### Unit Testing
- Component isolation with mocks
- Property-based testing for data structures
- Concurrency testing with race detection
- Memory leak detection

### Integration Testing
- End-to-end dashboard workflows
- Client library compatibility
- Backup and restore procedures
- Performance regression testing

### Load Testing
- Concurrent user simulation
- Backup performance under load
- Memory pressure testing
- Lock contention scenarios

### Security Testing
- SQL injection prevention
- Authentication and authorization
- Input validation
- Rate limiting effectiveness

## Deployment Architecture

### Single Binary Distribution
- Embedded web assets using Go embed
- Self-contained with no external dependencies
- Cross-platform compilation support
- Minimal resource footprint

### Configuration Management
- YAML configuration files
- Environment variable overrides
- Runtime configuration updates
- Configuration validation

### Monitoring Integration
- Prometheus metrics endpoint
- Health check endpoints for load balancers
- Structured logging for log aggregation
- Distributed tracing support