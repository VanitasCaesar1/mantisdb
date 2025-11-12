//! Time-Series Database
//!
//! High-performance time-series storage with retention policies and automatic rollups

use crate::error::{Error, Result};
use parking_lot::RwLock;
use std::collections::{BTreeMap, HashMap};
use std::sync::Arc;
use std::time::{Duration, SystemTime};
use serde::{Serialize, Deserialize};

/// Time-series database
pub struct TimeSeriesDB {
    inner: Arc<RwLock<TimeSeriesInner>>,
}

struct TimeSeriesInner {
    tables: HashMap<String, TimeSeriesTable>,
}

/// Time-series table
pub struct TimeSeriesTable {
    name: String,
    schema: TimeSeriesSchema,
    retention: RetentionPolicy,
    raw_data: BTreeMap<i64, Vec<DataPoint>>,
    rollups: HashMap<String, BTreeMap<i64, RollupData>>,
}

#[derive(Debug, Clone)]
pub struct TimeSeriesSchema {
    pub timestamp_field: String,
    pub value_fields: Vec<String>,
    pub tag_fields: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RetentionPolicy {
    pub raw_ttl: Duration,
    pub rollup_1m_ttl: Duration,
    pub rollup_1h_ttl: Duration,
    pub rollup_1d_ttl: Duration,
}

impl Default for RetentionPolicy {
    fn default() -> Self {
        Self {
            raw_ttl: Duration::from_secs(7 * 24 * 3600),      // 7 days
            rollup_1m_ttl: Duration::from_secs(30 * 24 * 3600), // 30 days
            rollup_1h_ttl: Duration::from_secs(365 * 24 * 3600), // 1 year
            rollup_1d_ttl: Duration::from_secs(5 * 365 * 24 * 3600), // 5 years
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DataPoint {
    pub timestamp: i64,
    pub values: HashMap<String, f64>,
    pub tags: HashMap<String, String>,
}

#[derive(Debug, Clone, Serialize)]
pub struct RollupData {
    pub count: u64,
    pub sum: f64,
    pub min: f64,
    pub max: f64,
    pub avg: f64,
}

#[derive(Debug, Clone)]
pub enum RollupInterval {
    OneMinute,
    FiveMinutes,
    OneHour,
    OneDay,
}

impl RollupInterval {
    fn duration(&self) -> Duration {
        match self {
            RollupInterval::OneMinute => Duration::from_secs(60),
            RollupInterval::FiveMinutes => Duration::from_secs(300),
            RollupInterval::OneHour => Duration::from_secs(3600),
            RollupInterval::OneDay => Duration::from_secs(86400),
        }
    }
    
    fn name(&self) -> &str {
        match self {
            RollupInterval::OneMinute => "1m",
            RollupInterval::FiveMinutes => "5m",
            RollupInterval::OneHour => "1h",
            RollupInterval::OneDay => "1d",
        }
    }
}

impl TimeSeriesDB {
    /// Create a new time-series database
    pub fn new() -> Self {
        Self {
            inner: Arc::new(RwLock::new(TimeSeriesInner {
                tables: HashMap::new(),
            })),
        }
    }
    
    /// Create a time-series table
    pub fn create_table(
        &self,
        name: String,
        schema: TimeSeriesSchema,
        retention: RetentionPolicy,
    ) -> Result<()> {
        let mut inner = self.inner.write();
        
        if inner.tables.contains_key(&name) {
            return Err(Error::General(format!("Table '{}' already exists", name)));
        }
        
        let table = TimeSeriesTable {
            name: name.clone(),
            schema,
            retention,
            raw_data: BTreeMap::new(),
            rollups: HashMap::new(),
        };
        
        inner.tables.insert(name, table);
        Ok(())
    }
    
    /// Insert a data point
    pub fn insert(&self, table_name: &str, point: DataPoint) -> Result<()> {
        let mut inner = self.inner.write();
        
        let table = inner.tables.get_mut(table_name)
            .ok_or_else(|| Error::General(format!("Table '{}' not found", table_name)))?;
        
        // Insert into raw data
        table.raw_data
            .entry(point.timestamp)
            .or_insert_with(Vec::new)
            .push(point);
        
        Ok(())
    }
    
    /// Query data points within a time range
    pub fn query(
        &self,
        table_name: &str,
        start_time: i64,
        end_time: i64,
    ) -> Result<Vec<DataPoint>> {
        let inner = self.inner.read();
        
        let table = inner.tables.get(table_name)
            .ok_or_else(|| Error::General(format!("Table '{}' not found", table_name)))?;
        
        let mut results = Vec::new();
        
        for (timestamp, points) in table.raw_data.range(start_time..=end_time) {
            results.extend(points.clone());
        }
        
        Ok(results)
    }
    
    /// Perform rollup aggregation
    pub fn rollup(&self, table_name: &str, interval: RollupInterval) -> Result<()> {
        let mut inner = self.inner.write();
        
        let table = inner.tables.get_mut(table_name)
            .ok_or_else(|| Error::General(format!("Table '{}' not found", table_name)))?;
        
        let interval_secs = interval.duration().as_secs() as i64;
        let rollup_name = interval.name().to_string();
        
        // Create rollup map if it doesn't exist
        if !table.rollups.contains_key(&rollup_name) {
            table.rollups.insert(rollup_name.clone(), BTreeMap::new());
        }
        
        // Group data by time buckets
        let rollup_map = table.rollups.get_mut(&rollup_name).unwrap();
        
        for (timestamp, points) in &table.raw_data {
            let bucket = (timestamp / interval_secs) * interval_secs;
            
            for point in points {
                for (field, value) in &point.values {
                    let entry = rollup_map.entry(bucket).or_insert(RollupData {
                        count: 0,
                        sum: 0.0,
                        min: f64::MAX,
                        max: f64::MIN,
                        avg: 0.0,
                    });
                    
                    entry.count += 1;
                    entry.sum += value;
                    entry.min = entry.min.min(*value);
                    entry.max = entry.max.max(*value);
                    entry.avg = entry.sum / entry.count as f64;
                }
            }
        }
        
        Ok(())
    }
    
    /// Get rollup data
    pub fn get_rollup(
        &self,
        table_name: &str,
        interval: RollupInterval,
        start_time: i64,
        end_time: i64,
    ) -> Result<Vec<(i64, RollupData)>> {
        let inner = self.inner.read();
        
        let table = inner.tables.get(table_name)
            .ok_or_else(|| Error::General(format!("Table '{}' not found", table_name)))?;
        
        let rollup_name = interval.name();
        let rollup_map = table.rollups.get(rollup_name)
            .ok_or_else(|| Error::General(format!("Rollup '{}' not found", rollup_name)))?;
        
        let results: Vec<_> = rollup_map
            .range(start_time..=end_time)
            .map(|(ts, data)| (*ts, data.clone()))
            .collect();
        
        Ok(results)
    }
    
    /// Apply retention policy (remove old data)
    pub fn apply_retention(&self, table_name: &str) -> Result<usize> {
        let mut inner = self.inner.write();
        
        let table = inner.tables.get_mut(table_name)
            .ok_or_else(|| Error::General(format!("Table '{}' not found", table_name)))?;
        
        let now = SystemTime::now()
            .duration_since(SystemTime::UNIX_EPOCH)
            .unwrap()
            .as_secs() as i64;
        
        let cutoff = now - table.retention.raw_ttl.as_secs() as i64;
        
        let mut removed = 0;
        let keys_to_remove: Vec<_> = table.raw_data
            .range(..cutoff)
            .map(|(k, _)| *k)
            .collect();
        
        for key in keys_to_remove {
            table.raw_data.remove(&key);
            removed += 1;
        }
        
        Ok(removed)
    }
    
    /// Get table statistics
    pub fn get_stats(&self, table_name: &str) -> Result<TableStats> {
        let inner = self.inner.read();
        
        let table = inner.tables.get(table_name)
            .ok_or_else(|| Error::General(format!("Table '{}' not found", table_name)))?;
        
        let total_points: usize = table.raw_data.values()
            .map(|v| v.len())
            .sum();
        
        let time_range = if let (Some(first), Some(last)) = 
            (table.raw_data.keys().next(), table.raw_data.keys().last()) {
            Some((*first, *last))
        } else {
            None
        };
        
        Ok(TableStats {
            total_points,
            time_buckets: table.raw_data.len(),
            time_range,
            rollup_count: table.rollups.len(),
        })
    }
}

impl Clone for TimeSeriesDB {
    fn clone(&self) -> Self {
        Self {
            inner: Arc::clone(&self.inner),
        }
    }
}

#[derive(Debug, Serialize)]
pub struct TableStats {
    pub total_points: usize,
    pub time_buckets: usize,
    pub time_range: Option<(i64, i64)>,
    pub rollup_count: usize,
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_create_table() {
        let db = TimeSeriesDB::new();
        
        let result = db.create_table(
            "metrics".to_string(),
            TimeSeriesSchema {
                timestamp_field: "ts".to_string(),
                value_fields: vec!["value".to_string()],
                tag_fields: vec!["host".to_string()],
            },
            RetentionPolicy::default(),
        );
        
        assert!(result.is_ok());
    }
    
    #[test]
    fn test_insert_and_query() {
        let db = TimeSeriesDB::new();
        
        db.create_table(
            "metrics".to_string(),
            TimeSeriesSchema {
                timestamp_field: "ts".to_string(),
                value_fields: vec!["value".to_string()],
                tag_fields: vec!["host".to_string()],
            },
            RetentionPolicy::default(),
        ).unwrap();
        
        let mut values = HashMap::new();
        values.insert("value".to_string(), 42.5);
        
        let mut tags = HashMap::new();
        tags.insert("host".to_string(), "server1".to_string());
        
        db.insert("metrics", DataPoint {
            timestamp: 1000,
            values,
            tags,
        }).unwrap();
        
        let results = db.query("metrics", 0, 2000).unwrap();
        assert_eq!(results.len(), 1);
    }
    
    #[test]
    fn test_rollup() {
        let db = TimeSeriesDB::new();
        
        db.create_table(
            "metrics".to_string(),
            TimeSeriesSchema {
                timestamp_field: "ts".to_string(),
                value_fields: vec!["value".to_string()],
                tag_fields: vec![],
            },
            RetentionPolicy::default(),
        ).unwrap();
        
        // Insert multiple points
        for i in 0..120 {
            let mut values = HashMap::new();
            values.insert("value".to_string(), i as f64);
            
            db.insert("metrics", DataPoint {
                timestamp: i,
                values,
                tags: HashMap::new(),
            }).unwrap();
        }
        
        // Perform 1-minute rollup
        db.rollup("metrics", RollupInterval::OneMinute).unwrap();
        
        // Query rollup data
        let rollup_data = db.get_rollup("metrics", RollupInterval::OneMinute, 0, 120).unwrap();
        assert!(!rollup_data.is_empty());
    }
    
    #[test]
    fn test_retention() {
        let db = TimeSeriesDB::new();
        
        db.create_table(
            "metrics".to_string(),
            TimeSeriesSchema {
                timestamp_field: "ts".to_string(),
                value_fields: vec!["value".to_string()],
                tag_fields: vec![],
            },
            RetentionPolicy {
                raw_ttl: Duration::from_secs(100),
                ..Default::default()
            },
        ).unwrap();
        
        // Insert old data
        let mut values = HashMap::new();
        values.insert("value".to_string(), 1.0);
        
        db.insert("metrics", DataPoint {
            timestamp: 1,
            values,
            tags: HashMap::new(),
        }).unwrap();
        
        let removed = db.apply_retention("metrics").unwrap();
        assert_eq!(removed, 1);
    }
}
