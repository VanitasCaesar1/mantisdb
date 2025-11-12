//! Change Data Capture (CDC)
//!
//! Real-time change streaming for replication and event sourcing

use crate::error::{Error, Result};
use parking_lot::RwLock;
use std::collections::{HashMap, VecDeque};
use std::sync::Arc;
use std::time::SystemTime;
use serde::{Serialize, Deserialize};

/// CDC stream manager
pub struct CDCStream {
    inner: Arc<RwLock<CDCInner>>,
}

struct CDCInner {
    streams: HashMap<String, ChangeStream>,
    global_offset: u64,
}

/// Change stream for a specific consumer
struct ChangeStream {
    name: String,
    changes: VecDeque<ChangeEvent>,
    consumers: HashMap<String, ConsumerState>,
    max_size: usize,
}

/// Consumer state tracking
#[derive(Debug, Clone)]
struct ConsumerState {
    consumer_id: String,
    offset: u64,
    last_ack_time: SystemTime,
}

/// Change event
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChangeEvent {
    pub offset: u64,
    pub timestamp: SystemTime,
    pub operation: Operation,
    pub table: String,
    pub key: String,
    pub before: Option<serde_json::Value>,
    pub after: Option<serde_json::Value>,
    pub metadata: HashMap<String, String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum Operation {
    Insert,
    Update,
    Delete,
}

/// CDC configuration
#[derive(Debug, Clone)]
pub struct CDCConfig {
    pub stream_name: String,
    pub max_buffer_size: usize,
    pub retention_period: std::time::Duration,
}

impl Default for CDCConfig {
    fn default() -> Self {
        Self {
            stream_name: "default".to_string(),
            max_buffer_size: 10000,
            retention_period: std::time::Duration::from_secs(3600), // 1 hour
        }
    }
}

impl CDCStream {
    /// Create a new CDC stream manager
    pub fn new() -> Self {
        Self {
            inner: Arc::new(RwLock::new(CDCInner {
                streams: HashMap::new(),
                global_offset: 0,
            })),
        }
    }
    
    /// Create a stream
    pub fn create_stream(&self, config: CDCConfig) -> Result<()> {
        let mut inner = self.inner.write();
        
        if inner.streams.contains_key(&config.stream_name) {
            return Err(Error::General(format!(
                "Stream '{}' already exists",
                config.stream_name
            )));
        }
        
        inner.streams.insert(
            config.stream_name.clone(),
            ChangeStream {
                name: config.stream_name,
                changes: VecDeque::new(),
                consumers: HashMap::new(),
                max_size: config.max_buffer_size,
            },
        );
        
        Ok(())
    }
    
    /// Capture a change event
    pub fn capture(&self, stream_name: &str, mut event: ChangeEvent) -> Result<u64> {
        let mut inner = self.inner.write();
        
        let stream = inner.streams.get_mut(stream_name)
            .ok_or_else(|| Error::General(format!("Stream '{}' not found", stream_name)))?;
        
        // Assign global offset
        event.offset = inner.global_offset;
        inner.global_offset += 1;
        
        // Add to stream
        stream.changes.push_back(event);
        
        // Enforce max size
        while stream.changes.len() > stream.max_size {
            stream.changes.pop_front();
        }
        
        Ok(inner.global_offset - 1)
    }
    
    /// Register a consumer
    pub fn register_consumer(&self, stream_name: &str, consumer_id: String) -> Result<()> {
        let mut inner = self.inner.write();
        
        let stream = inner.streams.get_mut(stream_name)
            .ok_or_else(|| Error::General(format!("Stream '{}' not found", stream_name)))?;
        
        if stream.consumers.contains_key(&consumer_id) {
            return Err(Error::General(format!(
                "Consumer '{}' already registered",
                consumer_id
            )));
        }
        
        stream.consumers.insert(
            consumer_id.clone(),
            ConsumerState {
                consumer_id,
                offset: 0,
                last_ack_time: SystemTime::now(),
            },
        );
        
        Ok(())
    }
    
    /// Read changes from stream
    pub fn read(
        &self,
        stream_name: &str,
        consumer_id: &str,
        limit: usize,
    ) -> Result<Vec<ChangeEvent>> {
        let inner = self.inner.read();
        
        let stream = inner.streams.get(stream_name)
            .ok_or_else(|| Error::General(format!("Stream '{}' not found", stream_name)))?;
        
        let consumer = stream.consumers.get(consumer_id)
            .ok_or_else(|| Error::General(format!(
                "Consumer '{}' not registered",
                consumer_id
            )))?;
        
        // Find changes after consumer's offset
        let changes: Vec<_> = stream.changes.iter()
            .filter(|event| event.offset >= consumer.offset)
            .take(limit)
            .cloned()
            .collect();
        
        Ok(changes)
    }
    
    /// Acknowledge processed events
    pub fn acknowledge(
        &self,
        stream_name: &str,
        consumer_id: &str,
        offset: u64,
    ) -> Result<()> {
        let mut inner = self.inner.write();
        
        let stream = inner.streams.get_mut(stream_name)
            .ok_or_else(|| Error::General(format!("Stream '{}' not found", stream_name)))?;
        
        let consumer = stream.consumers.get_mut(consumer_id)
            .ok_or_else(|| Error::General(format!(
                "Consumer '{}' not registered",
                consumer_id
            )))?;
        
        if offset > consumer.offset {
            consumer.offset = offset + 1; // Next offset to read
            consumer.last_ack_time = SystemTime::now();
        }
        
        Ok(())
    }
    
    /// Get stream statistics
    pub fn get_stats(&self, stream_name: &str) -> Result<StreamStats> {
        let inner = self.inner.read();
        
        let stream = inner.streams.get(stream_name)
            .ok_or_else(|| Error::General(format!("Stream '{}' not found", stream_name)))?;
        
        let oldest_offset = stream.changes.front().map(|e| e.offset);
        let newest_offset = stream.changes.back().map(|e| e.offset);
        
        Ok(StreamStats {
            total_events: stream.changes.len(),
            consumers: stream.consumers.len(),
            oldest_offset,
            newest_offset,
        })
    }
    
    /// Apply retention policy
    pub fn apply_retention(&self, stream_name: &str, retention: std::time::Duration) -> Result<usize> {
        let mut inner = self.inner.write();
        
        let stream = inner.streams.get_mut(stream_name)
            .ok_or_else(|| Error::General(format!("Stream '{}' not found", stream_name)))?;
        
        let cutoff = SystemTime::now() - retention;
        let mut removed = 0;
        
        while let Some(event) = stream.changes.front() {
            if event.timestamp < cutoff {
                stream.changes.pop_front();
                removed += 1;
            } else {
                break;
            }
        }
        
        Ok(removed)
    }
}

impl Clone for CDCStream {
    fn clone(&self) -> Self {
        Self {
            inner: Arc::clone(&self.inner),
        }
    }
}

#[derive(Debug, Serialize)]
pub struct StreamStats {
    pub total_events: usize,
    pub consumers: usize,
    pub oldest_offset: Option<u64>,
    pub newest_offset: Option<u64>,
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_create_stream() {
        let cdc = CDCStream::new();
        let result = cdc.create_stream(CDCConfig::default());
        assert!(result.is_ok());
    }
    
    #[test]
    fn test_capture_event() {
        let cdc = CDCStream::new();
        cdc.create_stream(CDCConfig::default()).unwrap();
        
        let event = ChangeEvent {
            offset: 0,
            timestamp: SystemTime::now(),
            operation: Operation::Insert,
            table: "users".to_string(),
            key: "123".to_string(),
            before: None,
            after: Some(serde_json::json!({"name": "Alice"})),
            metadata: HashMap::new(),
        };
        
        let offset = cdc.capture("default", event).unwrap();
        assert_eq!(offset, 0);
    }
    
    #[test]
    fn test_consumer_flow() {
        let cdc = CDCStream::new();
        cdc.create_stream(CDCConfig::default()).unwrap();
        cdc.register_consumer("default", "consumer1".to_string()).unwrap();
        
        // Capture some events
        for i in 0..5 {
            let event = ChangeEvent {
                offset: 0,
                timestamp: SystemTime::now(),
                operation: Operation::Insert,
                table: "users".to_string(),
                key: i.to_string(),
                before: None,
                after: Some(serde_json::json!({"id": i})),
                metadata: HashMap::new(),
            };
            cdc.capture("default", event).unwrap();
        }
        
        // Read events
        let events = cdc.read("default", "consumer1", 10).unwrap();
        assert_eq!(events.len(), 5);
        
        // Acknowledge
        cdc.acknowledge("default", "consumer1", 4).unwrap();
        
        // Read again (should be empty)
        let events = cdc.read("default", "consumer1", 10).unwrap();
        assert_eq!(events.len(), 0);
    }
    
    #[test]
    fn test_multiple_consumers() {
        let cdc = CDCStream::new();
        cdc.create_stream(CDCConfig::default()).unwrap();
        cdc.register_consumer("default", "consumer1".to_string()).unwrap();
        cdc.register_consumer("default", "consumer2".to_string()).unwrap();
        
        // Capture event
        let event = ChangeEvent {
            offset: 0,
            timestamp: SystemTime::now(),
            operation: Operation::Insert,
            table: "users".to_string(),
            key: "1".to_string(),
            before: None,
            after: Some(serde_json::json!({"name": "Bob"})),
            metadata: HashMap::new(),
        };
        cdc.capture("default", event).unwrap();
        
        // Both consumers should see the event
        let events1 = cdc.read("default", "consumer1", 10).unwrap();
        let events2 = cdc.read("default", "consumer2", 10).unwrap();
        
        assert_eq!(events1.len(), 1);
        assert_eq!(events2.len(), 1);
    }
    
    #[test]
    fn test_stats() {
        let cdc = CDCStream::new();
        cdc.create_stream(CDCConfig::default()).unwrap();
        cdc.register_consumer("default", "consumer1".to_string()).unwrap();
        
        let stats = cdc.get_stats("default").unwrap();
        assert_eq!(stats.total_events, 0);
        assert_eq!(stats.consumers, 1);
    }
}
