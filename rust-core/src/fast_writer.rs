//! High-performance write optimizations for MantisDB
//! Features: SIMD, zero-copy, lock-free ring buffer, parallel writes

use crate::error::Result;
use crate::storage::LockFreeStorage;
use crossbeam::channel::{bounded, Receiver, Sender};
use parking_lot::RwLock;
use std::sync::Arc;
use std::time::{Duration, Instant};

/// Write operation types
#[derive(Clone)]
pub enum WriteOp {
    Put { key: String, value: Vec<u8> },
    PutBatch { entries: Vec<(String, Vec<u8>)> },
    Delete { key: String },
}

/// High-performance write configuration
#[derive(Clone)]
pub struct FastWriteConfig {
    pub ring_buffer_size: usize,
    pub worker_threads: usize,
    pub batch_size: usize,
    pub flush_interval_ms: u64,
    pub enable_compression: bool,
    pub enable_parallel_writes: bool,
}

impl Default for FastWriteConfig {
    fn default() -> Self {
        Self {
            ring_buffer_size: 65536,
            worker_threads: num_cpus::get(),
            batch_size: 1000,
            flush_interval_ms: 5,
            enable_compression: false,
            enable_parallel_writes: true,
        }
    }
}

/// Fast writer with lock-free ring buffer and parallel processing
pub struct FastWriter {
    storage: Arc<LockFreeStorage>,
    config: FastWriteConfig,
    tx: Sender<WriteOp>,
    stats: Arc<RwLock<FastWriteStats>>,
}

/// Write statistics
#[derive(Default, Clone)]
pub struct FastWriteStats {
    pub total_writes: u64,
    pub total_bytes: u64,
    pub batches_processed: u64,
    pub avg_latency_us: u64,
    pub peak_throughput: u64,
}

impl FastWriter {
    pub fn new(storage: Arc<LockFreeStorage>, config: FastWriteConfig) -> Self {
        let (tx, rx) = bounded(config.ring_buffer_size);
        let stats = Arc::new(RwLock::new(FastWriteStats::default()));

        let writer = Self {
            storage: storage.clone(),
            config: config.clone(),
            tx,
            stats: stats.clone(),
        };

        // Start worker threads
        writer.start_workers(rx, storage, config, stats);

        writer
    }

    /// Submit write operation (non-blocking)
    pub fn write(&self, key: String, value: Vec<u8>) -> Result<()> {
        self.tx
            .send(WriteOp::Put { key, value })
            .map_err(|_| crate::error::Error::StorageError("Channel closed".to_string()))
    }

    /// Submit batch write operation
    pub fn write_batch(&self, entries: Vec<(String, Vec<u8>)>) -> Result<()> {
        self.tx
            .send(WriteOp::PutBatch { entries })
            .map_err(|_| crate::error::Error::StorageError("Channel closed".to_string()))
    }

    /// Submit delete operation
    pub fn delete(&self, key: String) -> Result<()> {
        self.tx
            .send(WriteOp::Delete { key })
            .map_err(|_| crate::error::Error::StorageError("Channel closed".to_string()))
    }

    /// Get current statistics
    pub fn stats(&self) -> FastWriteStats {
        self.stats.read().clone()
    }

    /// Start worker threads for parallel write processing
    fn start_workers(
        &self,
        rx: Receiver<WriteOp>,
        storage: Arc<LockFreeStorage>,
        config: FastWriteConfig,
        stats: Arc<RwLock<FastWriteStats>>,
    ) {
        for worker_id in 0..config.worker_threads {
            let rx = rx.clone();
            let storage = storage.clone();
            let stats = stats.clone();
            let batch_size = config.batch_size;
            let flush_interval = Duration::from_millis(config.flush_interval_ms);

            std::thread::Builder::new()
                .name(format!("fast-writer-{}", worker_id))
                .spawn(move || {
                    let mut batch = Vec::with_capacity(batch_size);
                    let mut last_flush = Instant::now();

                    loop {
                        // Try to receive with timeout
                        match rx.recv_timeout(flush_interval) {
                            Ok(op) => {
                                batch.push(op);

                                // Flush if batch is full or timeout
                                if batch.len() >= batch_size
                                    || last_flush.elapsed() >= flush_interval
                                {
                                    Self::process_batch(&storage, &mut batch, &stats);
                                    last_flush = Instant::now();
                                }
                            }
                            Err(_) => {
                                // Timeout - flush pending batch
                                if !batch.is_empty() {
                                    Self::process_batch(&storage, &mut batch, &stats);
                                    last_flush = Instant::now();
                                }
                            }
                        }
                    }
                })
                .expect("Failed to spawn worker thread");
        }
    }

    /// Process a batch of write operations
    fn process_batch(
        storage: &Arc<LockFreeStorage>,
        batch: &mut Vec<WriteOp>,
        stats: &Arc<RwLock<FastWriteStats>>,
    ) {
        if batch.is_empty() {
            return;
        }

        let start = Instant::now();
        let mut total_bytes = 0u64;
        let mut write_count = 0u64;

        // Collect all puts for batch processing
        let mut put_entries = Vec::new();

        for op in batch.drain(..) {
            match op {
                WriteOp::Put { key, value } => {
                    total_bytes += value.len() as u64;
                    write_count += 1;
                    put_entries.push((key, value));
                }
                WriteOp::PutBatch { entries } => {
                    for (key, value) in entries {
                        total_bytes += value.len() as u64;
                        write_count += 1;
                        put_entries.push((key, value));
                    }
                }
                WriteOp::Delete { key } => {
                    let _ = storage.delete(key.as_bytes());
                }
            }
        }

        // Batch write all puts
        if !put_entries.is_empty() {
            let _ = storage.batch_put(put_entries);
        }

        let latency_us = start.elapsed().as_micros() as u64;

        // Update statistics
        let mut stats = stats.write();
        stats.total_writes += write_count;
        stats.total_bytes += total_bytes;
        stats.batches_processed += 1;
        stats.avg_latency_us = (stats.avg_latency_us + latency_us) / 2;

        let throughput = if latency_us > 0 {
            (write_count * 1_000_000) / latency_us
        } else {
            0
        };
        stats.peak_throughput = stats.peak_throughput.max(throughput);
    }
}

/// Zero-copy write buffer for minimizing allocations
pub struct ZeroCopyBuffer {
    buffer: Vec<u8>,
    position: usize,
}

impl ZeroCopyBuffer {
    pub fn new(capacity: usize) -> Self {
        Self {
            buffer: Vec::with_capacity(capacity),
            position: 0,
        }
    }

    /// Write data without copying (uses buffer reuse)
    pub fn write(&mut self, data: &[u8]) -> Result<()> {
        if self.position + data.len() > self.buffer.capacity() {
            return Err(crate::error::Error::StorageError("Buffer full".to_string()));
        }

        unsafe {
            let ptr = self.buffer.as_mut_ptr().add(self.position);
            std::ptr::copy_nonoverlapping(data.as_ptr(), ptr, data.len());
        }

        self.position += data.len();
        unsafe {
            self.buffer.set_len(self.position);
        }

        Ok(())
    }

    /// Get buffer slice
    pub fn as_slice(&self) -> &[u8] {
        &self.buffer[..self.position]
    }

    /// Reset buffer for reuse
    pub fn reset(&mut self) {
        self.position = 0;
        unsafe {
            self.buffer.set_len(0);
        }
    }
}

/// SIMD-accelerated data operations (when available)
#[cfg(target_arch = "x86_64")]
pub mod simd {
    use std::arch::x86_64::*;

    /// Fast memory copy using SIMD instructions
    #[target_feature(enable = "avx2")]
    pub unsafe fn fast_memcpy(dst: *mut u8, src: *const u8, len: usize) {
        if len < 32 {
            // Fall back to standard copy for small sizes
            std::ptr::copy_nonoverlapping(src, dst, len);
            return;
        }

        let chunks = len / 32;
        let remainder = len % 32;

        for i in 0..chunks {
            let offset = i * 32;
            let data = _mm256_loadu_si256(src.add(offset) as *const __m256i);
            _mm256_storeu_si256(dst.add(offset) as *mut __m256i, data);
        }

        // Copy remainder
        if remainder > 0 {
            std::ptr::copy_nonoverlapping(src.add(chunks * 32), dst.add(chunks * 32), remainder);
        }
    }

    /// Fast comparison using SIMD
    #[target_feature(enable = "avx2")]
    pub unsafe fn fast_compare(a: *const u8, b: *const u8, len: usize) -> bool {
        if len < 32 {
            return std::slice::from_raw_parts(a, len) == std::slice::from_raw_parts(b, len);
        }

        let chunks = len / 32;

        for i in 0..chunks {
            let offset = i * 32;
            let va = _mm256_loadu_si256(a.add(offset) as *const __m256i);
            let vb = _mm256_loadu_si256(b.add(offset) as *const __m256i);
            let cmp = _mm256_cmpeq_epi8(va, vb);
            let mask = _mm256_movemask_epi8(cmp);

            if mask != -1 {
                return false;
            }
        }

        // Compare remainder
        let remainder = len % 32;
        if remainder > 0 {
            let a_slice = std::slice::from_raw_parts(a.add(chunks * 32), remainder);
            let b_slice = std::slice::from_raw_parts(b.add(chunks * 32), remainder);
            return a_slice == b_slice;
        }

        true
    }
}

#[cfg(not(target_arch = "x86_64"))]
pub mod simd {
    /// Fallback for non-x86_64 architectures
    pub unsafe fn fast_memcpy(dst: *mut u8, src: *const u8, len: usize) {
        std::ptr::copy_nonoverlapping(src, dst, len);
    }

    pub unsafe fn fast_compare(a: *const u8, b: *const u8, len: usize) -> bool {
        std::slice::from_raw_parts(a, len) == std::slice::from_raw_parts(b, len)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_fast_writer() {
        let storage = Arc::new(LockFreeStorage::new(1024).unwrap());
        let config = FastWriteConfig::default();
        let writer = FastWriter::new(storage.clone(), config);

        // Write test data
        for i in 0..1000 {
            writer
                .write(format!("key_{}", i), format!("value_{}", i).into_bytes())
                .unwrap();
        }

        // Give workers time to process
        std::thread::sleep(Duration::from_millis(100));

        let stats = writer.stats();
        assert!(stats.total_writes > 0);
    }

    #[test]
    fn test_zero_copy_buffer() {
        let mut buffer = ZeroCopyBuffer::new(1024);

        buffer.write(b"hello").unwrap();
        buffer.write(b"world").unwrap();

        assert_eq!(buffer.as_slice(), b"helloworld");

        buffer.reset();
        assert_eq!(buffer.as_slice().len(), 0);
    }
}
