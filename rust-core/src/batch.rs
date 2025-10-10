//! Batch write optimization for high-throughput scenarios
//! Provides write coalescing and async flushing

use crate::error::Result;
use crate::storage::LockFreeStorage;
use parking_lot::Mutex;
use std::collections::VecDeque;
use std::sync::Arc;
use std::time::{Duration, Instant};

/// Batch write configuration
pub struct BatchConfig {
    pub max_batch_size: usize,
    pub max_delay: Duration,
    pub enable_compression: bool,
}

impl Default for BatchConfig {
    fn default() -> Self {
        Self {
            max_batch_size: 1000,
            max_delay: Duration::from_millis(10),
            enable_compression: false,
        }
    }
}

/// Pending write operation
struct PendingWrite {
    key: String,
    value: Vec<u8>,
    timestamp: Instant,
}

/// Batch writer for optimized write throughput
pub struct BatchWriter {
    storage: Arc<LockFreeStorage>,
    config: BatchConfig,
    pending: Arc<Mutex<VecDeque<PendingWrite>>>,
    stats: Arc<BatchStats>,
}

/// Batch statistics
pub struct BatchStats {
    pub batches_written: atomic::Atomic<u64>,
    pub items_written: atomic::Atomic<u64>,
    pub bytes_written: atomic::Atomic<u64>,
}

impl BatchStats {
    fn new() -> Self {
        Self {
            batches_written: atomic::Atomic::new(0),
            items_written: atomic::Atomic::new(0),
            bytes_written: atomic::Atomic::new(0),
        }
    }
}

impl BatchWriter {
    pub fn new(storage: Arc<LockFreeStorage>, config: BatchConfig) -> Self {
        Self {
            storage,
            config,
            pending: Arc::new(Mutex::new(VecDeque::new())),
            stats: Arc::new(BatchStats::new()),
        }
    }

    /// Add write to batch (non-blocking)
    pub fn write(&self, key: String, value: Vec<u8>) -> Result<()> {
        let write = PendingWrite {
            key,
            value,
            timestamp: Instant::now(),
        };

        let mut pending = self.pending.lock();
        pending.push_back(write);

        // Trigger flush if batch is full
        if pending.len() >= self.config.max_batch_size {
            drop(pending);
            self.flush()?;
        }

        Ok(())
    }

    /// Flush pending writes
    pub fn flush(&self) -> Result<()> {
        let mut pending = self.pending.lock();
        if pending.is_empty() {
            return Ok(());
        }

        // Collect all pending writes
        let writes: Vec<_> = pending.drain(..).collect();
        drop(pending);

        // Batch write to storage
        let mut total_bytes = 0;
        for write in &writes {
            total_bytes += write.value.len();
            self.storage.put(write.key.as_bytes(), &write.value)?;
        }

        // Update stats
        self.stats
            .batches_written
            .fetch_add(1, atomic::Ordering::Relaxed);
        self.stats
            .items_written
            .fetch_add(writes.len() as u64, atomic::Ordering::Relaxed);
        self.stats
            .bytes_written
            .fetch_add(total_bytes as u64, atomic::Ordering::Relaxed);

        Ok(())
    }

    /// Start background flusher
    pub fn start_auto_flush(&self) {
        let pending = Arc::clone(&self.pending);
        let storage = Arc::clone(&self.storage);
        let stats = Arc::clone(&self.stats);
        let max_delay = self.config.max_delay;

        std::thread::spawn(move || {
            loop {
                std::thread::sleep(max_delay);

                let mut pending_lock = pending.lock();
                if pending_lock.is_empty() {
                    continue;
                }

                // Check if oldest write is past deadline
                if let Some(oldest) = pending_lock.front() {
                    if oldest.timestamp.elapsed() >= max_delay {
                        let writes: Vec<_> = pending_lock.drain(..).collect();
                        drop(pending_lock);

                        // Flush writes
                        let mut total_bytes = 0;
                        for write in &writes {
                            total_bytes += write.value.len();
                            let _ = storage.put(write.key.as_bytes(), &write.value);
                        }

                        stats
                            .batches_written
                            .fetch_add(1, atomic::Ordering::Relaxed);
                        stats
                            .items_written
                            .fetch_add(writes.len() as u64, atomic::Ordering::Relaxed);
                        stats
                            .bytes_written
                            .fetch_add(total_bytes as u64, atomic::Ordering::Relaxed);
                    }
                }
            }
        });
    }

    pub fn stats(&self) -> &BatchStats {
        &self.stats
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_batch_write() {
        let storage = Arc::new(LockFreeStorage::new(1024).unwrap());
        let config = BatchConfig::default();
        let writer = BatchWriter::new(storage.clone(), config);

        // Write multiple items
        for i in 0..100 {
            writer
                .write(format!("key_{}", i), format!("value_{}", i).into_bytes())
                .unwrap();
        }

        // Flush
        writer.flush().unwrap();

        // Verify
        assert_eq!(storage.len(), 100);
    }
}
