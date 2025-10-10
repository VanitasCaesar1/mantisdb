//! Lock-free storage engine using skiplist for O(log n) operations
//! Optimized for high-throughput concurrent access

use crate::error::{Error, Result};
use crossbeam_skiplist::SkipMap;
use rkyv::{Archive, Deserialize, Serialize};
use std::sync::Arc;
use std::time::{SystemTime, UNIX_EPOCH};

/// Storage entry with metadata
#[derive(Archive, Deserialize, Serialize, Debug, Clone)]
#[archive(compare(PartialEq), check_bytes)]
pub struct StorageEntry {
    pub key: String,
    pub value: Vec<u8>,
    pub timestamp: u64,
    pub version: u64,
    pub ttl: u64, // 0 means no expiration
}

impl StorageEntry {
    pub fn new(key: String, value: Vec<u8>) -> Self {
        Self {
            key,
            value,
            timestamp: current_timestamp(),
            version: 1,
            ttl: 0,
        }
    }

    pub fn with_ttl(key: String, value: Vec<u8>, ttl: u64) -> Self {
        Self {
            key,
            value,
            timestamp: current_timestamp(),
            version: 1,
            ttl,
        }
    }

    pub fn is_expired(&self) -> bool {
        if self.ttl == 0 {
            return false;
        }
        let now = current_timestamp();
        now > self.timestamp + self.ttl
    }
}

/// Lock-free storage engine
pub struct LockFreeStorage {
    data: Arc<SkipMap<String, Arc<StorageEntry>>>,
    stats: Arc<StorageStats>,
}

/// Storage statistics (lock-free counters)
pub struct StorageStats {
    reads: atomic::Atomic<u64>,
    writes: atomic::Atomic<u64>,
    deletes: atomic::Atomic<u64>,
    hits: atomic::Atomic<u64>,
    misses: atomic::Atomic<u64>,
}

impl StorageStats {
    fn new() -> Self {
        Self {
            reads: atomic::Atomic::new(0),
            writes: atomic::Atomic::new(0),
            deletes: atomic::Atomic::new(0),
            hits: atomic::Atomic::new(0),
            misses: atomic::Atomic::new(0),
        }
    }

    pub fn get_reads(&self) -> u64 {
        self.reads.load(atomic::Ordering::Relaxed)
    }

    pub fn get_writes(&self) -> u64 {
        self.writes.load(atomic::Ordering::Relaxed)
    }

    pub fn get_deletes(&self) -> u64 {
        self.deletes.load(atomic::Ordering::Relaxed)
    }

    pub fn hit_rate(&self) -> f64 {
        let hits = self.hits.load(atomic::Ordering::Relaxed);
        let total = hits + self.misses.load(atomic::Ordering::Relaxed);
        if total == 0 {
            return 0.0;
        }
        hits as f64 / total as f64
    }
}

impl LockFreeStorage {
    pub fn new(_capacity: usize) -> Result<Self> {
        Ok(Self {
            data: Arc::new(SkipMap::new()),
            stats: Arc::new(StorageStats::new()),
        })
    }

    /// Health check for connection pool
    pub fn health_check(&self) -> Result<()> {
        // Simple health check - verify storage is accessible
        Ok(())
    }

    /// Put a key-value pair (lock-free) - accepts byte slices
    pub fn put(&self, key: &[u8], value: &[u8]) -> Result<()> {
        let key = String::from_utf8_lossy(key).to_string();
        let value = value.to_vec();
        self.put_string(key, value)
    }

    /// Put a key-value pair with String key
    pub fn put_string(&self, key: String, value: Vec<u8>) -> Result<()> {
        let entry = Arc::new(StorageEntry::new(key.clone(), value));
        self.data.insert(key, entry);
        self.stats.writes.fetch_add(1, atomic::Ordering::Relaxed);
        Ok(())
    }

    /// Put with TTL
    pub fn put_with_ttl(&self, key: String, value: Vec<u8>, ttl: u64) -> Result<()> {
        let entry = Arc::new(StorageEntry::with_ttl(key.clone(), value, ttl));
        self.data.insert(key, entry);
        self.stats.writes.fetch_add(1, atomic::Ordering::Relaxed);
        Ok(())
    }

    /// Get a value by key (lock-free) - accepts byte slices
    pub fn get(&self, key: &[u8]) -> Result<Vec<u8>> {
        let key = String::from_utf8_lossy(key);
        self.get_string(&key)
    }

    /// Get a value by string key
    pub fn get_string(&self, key: &str) -> Result<Vec<u8>> {
        self.stats.reads.fetch_add(1, atomic::Ordering::Relaxed);

        match self.data.get(key) {
            Some(entry) => {
                let entry = entry.value();

                // Check expiration
                if entry.is_expired() {
                    self.stats.misses.fetch_add(1, atomic::Ordering::Relaxed);
                    // Lazy deletion
                    self.data.remove(key);
                    return Err(Error::KeyNotFound(key.to_string()));
                }

                self.stats.hits.fetch_add(1, atomic::Ordering::Relaxed);
                Ok(entry.value.clone())
            }
            None => {
                self.stats.misses.fetch_add(1, atomic::Ordering::Relaxed);
                Err(Error::KeyNotFound(key.to_string()))
            }
        }
    }

    /// Delete a key (lock-free) - accepts byte slices
    pub fn delete(&self, key: &[u8]) -> Result<()> {
        let key = String::from_utf8_lossy(key);
        self.delete_string(&key)
    }

    /// Delete a key by string
    pub fn delete_string(&self, key: &str) -> Result<()> {
        self.stats.deletes.fetch_add(1, atomic::Ordering::Relaxed);
        self.data.remove(key);
        Ok(())
    }

    /// Batch put (optimized for maximum throughput)
    pub fn batch_put(&self, entries: Vec<(String, Vec<u8>)>) -> Result<()> {
        if entries.is_empty() {
            return Ok(());
        }
        
        // Use parallel writes for all batch sizes to maximize throughput
        let num_threads = num_cpus::get().min(16); // Cap at 16 threads
        let chunk_size = (entries.len() / num_threads).max(10);
        
        use std::sync::atomic::{AtomicUsize, Ordering};
        use std::thread;
        
        let error_count = Arc::new(AtomicUsize::new(0));
        let mut handles = vec![];
        
        for chunk in entries.chunks(chunk_size) {
            let storage = self.clone();
            let chunk = chunk.to_vec();
            let error_count = Arc::clone(&error_count);
            
            let handle = thread::spawn(move || {
                for (key, value) in chunk {
                    if storage.put_string(key, value).is_err() {
                        error_count.fetch_add(1, Ordering::Relaxed);
                    }
                }
            });
            handles.push(handle);
        }
        
        for handle in handles {
            let _ = handle.join();
        }
        
        if error_count.load(Ordering::Relaxed) > 0 {
            return Err(Error::StorageError("Some batch writes failed".to_string()));
        }
        
        Ok(())
    }

    /// Batch get (optimized for throughput)
    pub fn batch_get(&self, keys: &[String]) -> Vec<Option<Vec<u8>>> {
        keys.iter().map(|key| self.get(key.as_bytes()).ok()).collect()
    }

    /// Check if key exists
    pub fn exists(&self, key: &str) -> bool {
        self.data.contains_key(key)
    }

    /// Get number of entries
    pub fn len(&self) -> usize {
        self.data.len()
    }

    /// Check if empty
    pub fn is_empty(&self) -> bool {
        self.data.is_empty()
    }

    /// Clear all entries
    pub fn clear(&self) {
        self.data.clear();
    }

    /// Get statistics
    pub fn stats(&self) -> &StorageStats {
        &self.stats
    }

    /// Scan with prefix (returns iterator)
    pub fn scan_prefix(&self, prefix: &str) -> Vec<(String, Vec<u8>)> {
        let mut results = Vec::new();

        for entry in self.data.iter() {
            let key = entry.key();
            if key.starts_with(prefix) {
                let value = entry.value();
                if !value.is_expired() {
                    results.push((key.clone(), value.value.clone()));
                }
            }
        }

        results
    }

    /// Cleanup expired entries (background task)
    pub fn cleanup_expired(&self) -> usize {
        let mut removed = 0;
        let mut to_remove = Vec::new();

        for entry in self.data.iter() {
            if entry.value().is_expired() {
                to_remove.push(entry.key().clone());
            }
        }

        for key in to_remove {
            self.data.remove(&key);
            removed += 1;
        }

        removed
    }
}

impl Default for LockFreeStorage {
    fn default() -> Self {
        Self::new(1024 * 1024 * 100).expect("Failed to create default storage")
    }
}

// Thread-safe clone
impl Clone for LockFreeStorage {
    fn clone(&self) -> Self {
        Self {
            data: Arc::clone(&self.data),
            stats: Arc::clone(&self.stats),
        }
    }
}

// Helper function
fn current_timestamp() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_secs()
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::thread;

    #[test]
    fn test_basic_operations() {
        let storage = LockFreeStorage::new(1024).unwrap();

        storage.put(b"key1", b"value1").unwrap();
        let value = storage.get(b"key1").unwrap();
        assert_eq!(value, b"value1");

        storage.delete(b"key1").unwrap();
        assert!(storage.get(b"key1").is_err());
    }

    #[test]
    fn test_concurrent_access() {
        let storage = LockFreeStorage::new(1024).unwrap();
        let mut handles = vec![];

        for i in 0..10 {
            let storage = storage.clone();
            let handle = thread::spawn(move || {
                for j in 0..1000 {
                    let key = format!("key_{}_{}", i, j);
                    let value = format!("value_{}_{}", i, j).into_bytes();
                    storage.put(key.as_bytes(), &value).unwrap();
                    let retrieved = storage.get(key.as_bytes()).unwrap();
                    assert_eq!(retrieved, value);
                }
            });
            handles.push(handle);
        }

        for handle in handles {
            handle.join().unwrap();
        }

        assert_eq!(storage.len(), 10000);
    }

    #[test]
    fn test_ttl_expiration() {
        let storage = LockFreeStorage::new(1024).unwrap();

        storage
            .put_with_ttl("key1".to_string(), b"value1".to_vec(), 1)
            .unwrap();
        assert!(storage.get(b"key1").is_ok());

        std::thread::sleep(std::time::Duration::from_secs(2));
        assert!(storage.get(b"key1").is_err());
    }
}
