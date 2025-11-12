//! High-performance connection pooling for MantisDB.
//!
//! This is a PgBouncer-style connection pool built on lock-free data structures.
//! We use crossbeam's ArrayQueue (lock-free) for the idle pool and tokio's
//! Semaphore for backpressure. The combination gives us:
//! - Zero contention on connection checkout/return (lock-free queue)
//! - Fair backpressure when pool is exhausted (semaphore)
//! - Thousands of concurrent waiters without spinlock overhead

use crate::storage::LockFreeStorage;
use crate::error::{Error, Result};
use std::sync::atomic::{AtomicBool, AtomicU64, AtomicUsize, Ordering};
use std::sync::Arc;
use std::time::{Duration, Instant};
use tokio::sync::{Semaphore, Mutex};
use crossbeam::queue::ArrayQueue;

/// Connection pool configuration
#[derive(Debug, Clone)]
pub struct PoolConfig {
    /// Minimum number of connections to maintain
    pub min_connections: usize,
    /// Maximum number of connections allowed
    pub max_connections: usize,
    /// Maximum time a connection can be idle before being closed
    pub max_idle_time: Duration,
    /// Timeout for acquiring a connection
    pub connection_timeout: Duration,
    /// Maximum lifetime of a connection
    pub max_lifetime: Duration,
    /// Health check interval
    pub health_check_interval: Duration,
    /// Enable connection recycling
    pub recycle_connections: bool,
}

impl Default for PoolConfig {
    fn default() -> Self {
        Self {
            min_connections: 10,
            max_connections: 1000,
            // 5 min idle timeout prevents connection buildup during traffic spikes.
            max_idle_time: Duration::from_secs(300),
            // 10s connection timeout is aggressive - fail fast rather than queue up.
            connection_timeout: Duration::from_secs(10),
            // 1hr max lifetime recycles connections periodically to avoid leaks
            // from slow resource leaks in the storage layer.
            max_lifetime: Duration::from_secs(3600),
            health_check_interval: Duration::from_secs(30),
            recycle_connections: true,
        }
    }
}

/// A pooled connection wrapper
pub struct PooledConnection {
    storage: Arc<LockFreeStorage>,
    created_at: Instant,
    last_used: Instant,
    id: u64,
    pool: Arc<ConnectionPoolInner>,
}

impl PooledConnection {
    /// Get the underlying storage
    pub fn storage(&self) -> &Arc<LockFreeStorage> {
        &self.storage
    }

    /// Check if connection is still valid.
    /// We check both age and health - age check is cheap, health check is expensive.
    /// Checking age first short-circuits most calls without hitting storage.
    pub fn is_valid(&self) -> bool {
        let now = Instant::now();
        let lifetime = now.duration_since(self.created_at);
        
        // Age check first - O(1), no I/O
        if lifetime > self.pool.config.max_lifetime {
            return false;
        }

        // Health check hits storage - only do this if age is OK
        self.storage.health_check().is_ok()
    }

    /// Get connection age
    pub fn age(&self) -> Duration {
        Instant::now().duration_since(self.created_at)
    }

    /// Get idle time
    pub fn idle_time(&self) -> Duration {
        Instant::now().duration_since(self.last_used)
    }
}

impl Drop for PooledConnection {
    fn drop(&mut self) {
        // RAII connection return - Drop runs automatically when conn goes out of scope.
        // We spawn a task (not blocking) to avoid holding up the dropping thread.
        // Trade-off: slightly delayed return for non-blocking Drop.
        let pool = self.pool.clone();
        let storage = self.storage.clone();
        let id = self.id;
        
        tokio::spawn(async move {
            pool.return_connection(storage, id).await;
        });
    }
}

/// Connection pool statistics
#[derive(Debug, Clone, Default)]
pub struct PoolStats {
    pub total_connections: usize,
    pub active_connections: usize,
    pub idle_connections: usize,
    pub wait_count: u64,
    pub total_wait_time_ms: u64,
    pub connections_created: u64,
    pub connections_closed: u64,
    pub health_check_failures: u64,
}

struct ConnectionEntry {
    storage: Arc<LockFreeStorage>,
    created_at: Instant,
    last_used: Instant,
    id: u64,
}

struct ConnectionPoolInner {
    config: PoolConfig,
    idle_connections: ArrayQueue<ConnectionEntry>,
    semaphore: Semaphore,
    active_count: AtomicUsize,
    next_id: AtomicU64,
    closed: AtomicBool,
    
    // Statistics
    wait_count: AtomicU64,
    total_wait_time_ms: AtomicU64,
    connections_created: AtomicU64,
    connections_closed: AtomicU64,
    health_check_failures: AtomicU64,
}

impl ConnectionPoolInner {
    async fn return_connection(&self, storage: Arc<LockFreeStorage>, id: u64) {
        if self.closed.load(Ordering::Relaxed) {
            return;
        }

        let entry = ConnectionEntry {
            storage,
            created_at: Instant::now(),
            last_used: Instant::now(),
            id,
        };

        // Try to return to idle pool (lock-free push).
        // If queue is full, we drop the connection - better to close excess
        // connections than let them pile up and exhaust memory.
        if self.idle_connections.push(entry).is_err() {
            self.connections_closed.fetch_add(1, Ordering::Relaxed);
        }
        
        // Atomics for stats - Relaxed ordering is sufficient for metrics.
        self.active_count.fetch_sub(1, Ordering::Relaxed);
        // Release permit to unblock waiters
        self.semaphore.add_permits(1);
    }
}

/// High-performance connection pool
pub struct ConnectionPool {
    inner: Arc<ConnectionPoolInner>,
    factory: Arc<Mutex<Box<dyn Fn() -> Result<Arc<LockFreeStorage>> + Send + Sync>>>,
}

impl ConnectionPool {
    /// Create a new connection pool
    pub async fn new<F>(config: PoolConfig, factory: F) -> Result<Self>
    where
        F: Fn() -> Result<Arc<LockFreeStorage>> + Send + Sync + 'static,
    {
        let idle_connections = ArrayQueue::new(config.max_connections);
        
        let inner = Arc::new(ConnectionPoolInner {
            config: config.clone(),
            idle_connections,
            semaphore: Semaphore::new(config.max_connections),
            active_count: AtomicUsize::new(0),
            next_id: AtomicU64::new(0),
            closed: AtomicBool::new(false),
            wait_count: AtomicU64::new(0),
            total_wait_time_ms: AtomicU64::new(0),
            connections_created: AtomicU64::new(0),
            connections_closed: AtomicU64::new(0),
            health_check_failures: AtomicU64::new(0),
        });

        let factory = Arc::new(Mutex::new(Box::new(factory) as Box<dyn Fn() -> Result<Arc<LockFreeStorage>> + Send + Sync>));

        let pool = Self {
            inner: inner.clone(),
            factory: factory.clone(),
        };

        // Pre-create minimum connections
        for _ in 0..config.min_connections {
            let storage = {
                let factory = factory.lock().await;
                factory()?
            };
            
            let id = inner.next_id.fetch_add(1, Ordering::Relaxed);
            let entry = ConnectionEntry {
                storage,
                created_at: Instant::now(),
                last_used: Instant::now(),
                id,
            };
            
            inner.idle_connections.push(entry).ok();
            inner.connections_created.fetch_add(1, Ordering::Relaxed);
        }

        // Start health check task
        if config.health_check_interval > Duration::ZERO {
            let inner_clone = inner.clone();
            tokio::spawn(async move {
                Self::health_check_loop(inner_clone).await;
            });
        }

        Ok(pool)
    }

    /// Acquire a connection from the pool
    pub async fn acquire(&self) -> Result<PooledConnection> {
        if self.inner.closed.load(Ordering::Relaxed) {
            return Err(Error::PoolClosed);
        }

        let start = Instant::now();
        self.inner.wait_count.fetch_add(1, Ordering::Relaxed);

        // Wait for available slot
        let permit = tokio::time::timeout(
            self.inner.config.connection_timeout,
            self.inner.semaphore.acquire(),
        )
        .await
        .map_err(|_| Error::PoolExhausted)?
        .map_err(|_| Error::PoolClosed)?;

        permit.forget(); // We'll manage the permit manually

        // Try to get an idle connection
        if let Some(mut entry) = self.inner.idle_connections.pop() {
            // Check if connection is still valid
            if entry.storage.health_check().is_ok() {
                let wait_time = start.elapsed().as_millis() as u64;
                self.inner.total_wait_time_ms.fetch_add(wait_time, Ordering::Relaxed);
                self.inner.active_count.fetch_add(1, Ordering::Relaxed);
                
                entry.last_used = Instant::now();
                
                return Ok(PooledConnection {
                    storage: entry.storage,
                    created_at: entry.created_at,
                    last_used: entry.last_used,
                    id: entry.id,
                    pool: self.inner.clone(),
                });
            } else {
                self.inner.health_check_failures.fetch_add(1, Ordering::Relaxed);
                self.inner.connections_closed.fetch_add(1, Ordering::Relaxed);
            }
        }

        // Create new connection
        let storage = {
            let factory = self.factory.lock().await;
            factory()?
        };

        let id = self.inner.next_id.fetch_add(1, Ordering::Relaxed);
        let now = Instant::now();
        
        self.inner.connections_created.fetch_add(1, Ordering::Relaxed);
        self.inner.active_count.fetch_add(1, Ordering::Relaxed);
        
        let wait_time = start.elapsed().as_millis() as u64;
        self.inner.total_wait_time_ms.fetch_add(wait_time, Ordering::Relaxed);

        Ok(PooledConnection {
            storage,
            created_at: now,
            last_used: now,
            id,
            pool: self.inner.clone(),
        })
    }

    /// Get pool statistics
    pub fn stats(&self) -> PoolStats {
        PoolStats {
            total_connections: self.inner.connections_created.load(Ordering::Relaxed) as usize
                - self.inner.connections_closed.load(Ordering::Relaxed) as usize,
            active_connections: self.inner.active_count.load(Ordering::Relaxed),
            idle_connections: self.inner.idle_connections.len(),
            wait_count: self.inner.wait_count.load(Ordering::Relaxed),
            total_wait_time_ms: self.inner.total_wait_time_ms.load(Ordering::Relaxed),
            connections_created: self.inner.connections_created.load(Ordering::Relaxed),
            connections_closed: self.inner.connections_closed.load(Ordering::Relaxed),
            health_check_failures: self.inner.health_check_failures.load(Ordering::Relaxed),
        }
    }

    /// Close the pool
    pub async fn close(&self) {
        self.inner.closed.store(true, Ordering::Relaxed);
        
        // Drain idle connections
        while self.inner.idle_connections.pop().is_some() {
            self.inner.connections_closed.fetch_add(1, Ordering::Relaxed);
        }
    }

    /// Health check loop
    async fn health_check_loop(inner: Arc<ConnectionPoolInner>) {
        let mut interval = tokio::time::interval(inner.config.health_check_interval);
        
        loop {
            interval.tick().await;
            
            if inner.closed.load(Ordering::Relaxed) {
                break;
            }

            // Check idle connections
            let mut to_remove = Vec::new();
            let idle_count = inner.idle_connections.len();
            
            for _ in 0..idle_count {
                if let Some(entry) = inner.idle_connections.pop() {
                    let now = Instant::now();
                    let idle_time = now.duration_since(entry.last_used);
                    let age = now.duration_since(entry.created_at);

                    // Check if should be removed
                    if idle_time > inner.config.max_idle_time
                        || age > inner.config.max_lifetime
                        || entry.storage.health_check().is_err()
                    {
                        to_remove.push(entry);
                    } else {
                        // Return to pool
                        inner.idle_connections.push(entry).ok();
                    }
                }
            }

            // Close removed connections
            for _ in to_remove {
                inner.connections_closed.fetch_add(1, Ordering::Relaxed);
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn test_connection_pool() {
        let config = PoolConfig {
            min_connections: 2,
            max_connections: 10,
            ..Default::default()
        };

        let pool = ConnectionPool::new(config, || {
            Ok(Arc::new(LockFreeStorage::new(1024 * 1024)?))
        })
        .await
        .unwrap();

        // Acquire connection
        let conn = pool.acquire().await.unwrap();
        assert!(conn.is_valid());

        // Check stats
        let stats = pool.stats();
        assert_eq!(stats.active_connections, 1);
    }
}
