# Time-Series Database Architecture

## Overview
Comprehensive time-series support for MantisDB with automatic retention, rollups, and compression.

## Core Components

### 1. Time-Series Table Type
```rust
pub struct TimeSeriesTable {
    name: String,
    schema: TimeSeriesSchema,
    retention_policy: RetentionPolicy,
    rollup_config: RollupConfig,
    compression: CompressionStrategy,
    partitions: Vec<TimePartition>,
}

pub struct TimeSeriesSchema {
    timestamp_column: String,
    value_columns: Vec<Column>,
    tag_columns: Vec<Column>,  // For grouping/filtering
}
```

### 2. Retention Policies
```rust
pub struct RetentionPolicy {
    raw_data_ttl: Duration,      // e.g., 7 days
    rollup_1m_ttl: Duration,     // e.g., 30 days
    rollup_1h_ttl: Duration,     // e.g., 1 year
    rollup_1d_ttl: Duration,     // e.g., 5 years
}
```

### 3. Automatic Rollups
```rust
pub enum RollupAggregation {
    Min,
    Max,
    Avg,
    Sum,
    Count,
    First,
    Last,
    Percentile(f64),
}

pub struct RollupConfig {
    intervals: Vec<RollupInterval>,
    aggregations: HashMap<String, Vec<RollupAggregation>>,
}

pub struct RollupInterval {
    name: String,
    duration: Duration,  // 1m, 5m, 1h, 1d
}
```

### 4. Time-Based Partitioning
```rust
pub struct TimePartition {
    id: String,
    start_time: DateTime<Utc>,
    end_time: DateTime<Utc>,
    data: BTreeMap<i64, Vec<DataPoint>>,  // timestamp -> data points
}
```

### 5. Compression Strategies
```rust
pub enum CompressionStrategy {
    None,
    Delta,           // Delta encoding
    Gorilla,         // Facebook Gorilla compression
    DoubleDelta,     // Double delta encoding
    Dictionary,      // Dictionary compression for strings
}
```

## API Design

### Creating Time-Series Tables
```sql
CREATE TIMESERIES TABLE metrics (
    timestamp TIMESTAMP,
    value DOUBLE,
    host VARCHAR TAG,
    metric_name VARCHAR TAG
)
WITH (
    retention_raw = '7d',
    retention_1m = '30d',
    retention_1h = '1y',
    rollup_intervals = '1m,5m,1h,1d',
    compression = 'gorilla'
);
```

### Querying Time-Series Data
```sql
SELECT 
    time_bucket('5m', timestamp) as time,
    host,
    AVG(value) as avg_value,
    MAX(value) as max_value
FROM metrics
WHERE timestamp >= NOW() - INTERVAL '1 hour'
GROUP BY time, host
ORDER BY time;
```

### Downsampling
```rust
impl TimeSeriesTable {
    pub fn downsample(&self, 
        start: DateTime<Utc>,
        end: DateTime<Utc>,
        interval: Duration,
        agg: RollupAggregation
    ) -> Result<Vec<DataPoint>> {
        // Automatically select appropriate rollup level
        // Return aggregated data
    }
}
```

## Storage Format

### Raw Data (Hot Storage)
- In-memory sorted arrays
- Fast inserts and recent queries
- Automatic compression after threshold

### Rolled-up Data (Warm Storage)
- Pre-aggregated summaries
- Multiple time windows (1m, 1h, 1d)
- B-Tree indexed

### Archived Data (Cold Storage)
- Highly compressed
- Disk-backed
- Rare access patterns

## Implementation Steps

### Phase 1: Core Infrastructure (2 days)
1. Define time-series table schema
2. Implement time-based partitioning
3. Basic insert and query operations

### Phase 2: Compression (1 day)
1. Implement Gorilla compression
2. Delta encoding
3. Automatic compression triggers

### Phase 3: Rollups (1 day)
1. Background rollup worker
2. Aggregation functions
3. Automatic selection of rollup level

### Phase 4: Retention (1 day)
1. TTL tracking
2. Automatic data expiration
3. Partition cleanup

## Performance Targets

- **Insert Rate**: 100K+ points/second
- **Query Latency**: <100ms for recent data
- **Compression Ratio**: 10:1 for typical metrics
- **Storage Efficiency**: 1 byte per data point (compressed)

## Example Usage

```rust
// Create time-series table
let ts_table = TimeSeriesTable::new(
    "metrics",
    TimeSeriesSchema {
        timestamp_column: "ts".to_string(),
        value_columns: vec![
            Column::new("value", DataType::Float64),
        ],
        tag_columns: vec![
            Column::new("host", DataType::String),
            Column::new("region", DataType::String),
        ],
    },
    RetentionPolicy {
        raw_data_ttl: Duration::from_days(7),
        rollup_1m_ttl: Duration::from_days(30),
        rollup_1h_ttl: Duration::from_days(365),
        rollup_1d_ttl: Duration::from_days(1825),
    },
)?;

// Insert data
ts_table.insert(DataPoint {
    timestamp: Utc::now(),
    values: vec![42.5],
    tags: hashmap! {
        "host" => "server1",
        "region" => "us-east",
    },
})?;

// Query with automatic rollup selection
let results = ts_table.query(
    Utc::now() - Duration::hours(24),
    Utc::now(),
    vec!["host"],
    RollupAggregation::Avg,
)?;
```

## Integration Points

- **SQL Interface**: `CREATE TIMESERIES TABLE`, `SELECT time_bucket()`
- **REST API**: POST `/api/timeseries/{table}`, GET `/api/timeseries/{table}/query`
- **Observability**: Built-in metrics for monitoring
- **Streaming**: Real-time ingest via CDC

## Future Enhancements

- Predictive downsampling
- Anomaly detection
- Continuous aggregates
- Multi-resolution storage
- Time-series specific indexes
