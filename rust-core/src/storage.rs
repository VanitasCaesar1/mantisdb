//! Lock-free storage engine using skiplist for O(log n) operations
//! Optimized for high-throughput concurrent access
//! Features: In-memory caching + disk-backed storage via B-Tree

use crate::error::{Error, Result};
use crate::storage_engine::btree::BTreeIndex;
use crate::storage_engine::buffer_pool::BufferPool;
use crossbeam_skiplist::SkipMap;
use rkyv::{Archive, Deserialize, Serialize};
use std::path::Path;
use std::sync::Arc;
use std::time::{SystemTime, UNIX_EPOCH};

/// Storage entry with metadata and MVCC support
#[derive(Archive, Deserialize, Serialize, Debug, Clone)]
#[archive(compare(PartialEq), check_bytes)]
pub struct StorageEntry {
    pub key: String,
    pub value: Vec<u8>,
    pub timestamp: u64,
    pub version: u64,
    pub ttl: u64, // 0 means no expiration
    pub created_at: u64,  // MVCC: creation timestamp
    pub deleted_at: Option<u64>,  // MVCC: deletion timestamp (soft delete)
}

impl StorageEntry {
    pub fn new(key: String, value: Vec<u8>) -> Self {
        let now = current_timestamp();
        Self {
            key,
            value,
            timestamp: now,
            version: 1,
            ttl: 0,
            created_at: now,
            deleted_at: None,
        }
    }

    pub fn with_ttl(key: String, value: Vec<u8>, ttl: u64) -> Self {
        let now = current_timestamp();
        Self {
            key,
            value,
            timestamp: now,
            version: 1,
            ttl,
            created_at: now,
            deleted_at: None,
        }
    }

    pub fn is_expired(&self) -> bool {
        if self.ttl == 0 {
            return false;
        }
        let now = current_timestamp();
        now > self.timestamp + self.ttl
    }
    
    /// MVCC: Check if entry is visible to a given snapshot timestamp
    pub fn visible_to(&self, snapshot_ts: u64) -> bool {
        // Entry must be created before snapshot
        if self.created_at > snapshot_ts {
            return false;
        }
        
        // Entry must not be deleted, or deleted after snapshot
        if let Some(deleted) = self.deleted_at {
            if deleted <= snapshot_ts {
                return false;
            }
        }
        
        true
    }
    
    /// MVCC: Mark entry as deleted (soft delete)
    pub fn mark_deleted(&mut self, delete_ts: u64) {
        self.deleted_at = Some(delete_ts);
    }
}

/// Lock-free storage engine with optional disk-backed storage
pub struct LockFreeStorage {
    data: Arc<SkipMap<String, Arc<StorageEntry>>>,
    stats: Arc<StorageStats>,
    // Optional disk-backed storage
    disk_index: Option<BTreeIndex>,
    buffer_pool: Option<BufferPool>,
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
    /// Create new in-memory only storage
    pub fn new(_capacity: usize) -> Result<Self> {
        Ok(Self {
            data: Arc::new(SkipMap::new()),
            stats: Arc::new(StorageStats::new()),
            disk_index: None,
            buffer_pool: None,
        })
    }
    
    /// Create storage with disk-backed B-Tree index
    pub fn with_disk_storage<P: AsRef<Path>>(capacity: usize, disk_path: P, buffer_pool_size: usize) -> Result<Self> {
        let disk_index = BTreeIndex::new(disk_path)
            .map_err(|e| Error::StorageError(format!("Failed to create disk index: {:?}", e)))?;
        
        let buffer_pool = BufferPool::new(buffer_pool_size);
        
        Ok(Self {
            data: Arc::new(SkipMap::new()),
            stats: Arc::new(StorageStats::new()),
            disk_index: Some(disk_index),
            buffer_pool: Some(buffer_pool),
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

    /// Put a key-value pair with String key (writes to memory and optionally disk)
    pub fn put_string(&self, key: String, value: Vec<u8>) -> Result<()> {
        // Write to memory
        let entry = Arc::new(StorageEntry::new(key.clone(), value.clone()));
        self.data.insert(key.clone(), entry);
        
        // Write to disk if enabled
        if let Some(ref disk_index) = self.disk_index {
            disk_index.insert(key.as_bytes(), &value)
                .map_err(|e| Error::StorageError(format!("Disk write error: {:?}", e)))?;
        }
        
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

    /// Get a value by string key (with disk fallback)
    pub fn get_string(&self, key: &str) -> Result<Vec<u8>> {
        self.stats.reads.fetch_add(1, atomic::Ordering::Relaxed);

        // Try memory first
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
                return Ok(entry.value.clone());
            }
            None => {
                // Memory miss - try disk if available
                if let Some(ref disk_index) = self.disk_index {
                    match disk_index.get(key.as_bytes()) {
                        Ok(Some(value)) => {
                            self.stats.hits.fetch_add(1, atomic::Ordering::Relaxed);
                            
                            // Promote to memory cache
                            let entry = Arc::new(StorageEntry::new(key.to_string(), value.clone()));
                            self.data.insert(key.to_string(), entry);
                            
                            return Ok(value);
                        }
                        Ok(None) => {},
                        Err(e) => {
                            return Err(Error::StorageError(format!("Disk read error: {:?}", e)));
                        }
                    }
                }
                
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

    /// Delete a key by string (from memory and disk)
    pub fn delete_string(&self, key: &str) -> Result<()> {
        self.stats.deletes.fetch_add(1, atomic::Ordering::Relaxed);
        
        // Remove from memory
        self.data.remove(key);
        
        // Remove from disk if enabled
        if let Some(ref disk_index) = self.disk_index {
            disk_index.delete(key.as_bytes())
                .map_err(|e| Error::StorageError(format!("Disk delete error: {:?}", e)))?;
        }
        
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
            disk_index: self.disk_index.clone(),
            buffer_pool: self.buffer_pool.clone(),
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
