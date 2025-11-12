//! Background cache maintenance tasks
//! 
//! Provides automated cleanup of expired entries, metrics collection,
//! and cache health monitoring

use crate::cache::LockFreeCache;
use std::sync::Arc;
use std::time::Duration;
use tokio::time::interval;
use tracing::{info, warn, debug};

/// Cache maintenance configuration
#[derive(Debug, Clone)]
pub struct MaintenanceConfig {
    /// Interval between cleanup runs
    pub cleanup_interval: Duration,
    /// Maximum entries to clean per run (rate limiting)
    pub max_cleanup_per_run: usize,
    /// Enable detailed logging
    pub enable_logging: bool,
}

impl Default for MaintenanceConfig {
    fn default() -> Self {
        Self {
            cleanup_interval: Duration::from_secs(60), // 1 minute
            max_cleanup_per_run: 10000,
            enable_logging: true,
        }
    }
}

/// Background cache maintenance task manager
pub struct CacheMaintenance {
    cache: Arc<LockFreeCache>,
    config: MaintenanceConfig,
}

impl CacheMaintenance {
    pub fn new(cache: Arc<LockFreeCache>, config: MaintenanceConfig) -> Self {
        Self { cache, config }
    }
    
    /// Start background maintenance task
    pub fn start(self) -> tokio::task::JoinHandle<()> {
        tokio::spawn(async move {
            self.run().await
        })
    }
    
    async fn run(self) {
        let mut ticker = interval(self.config.cleanup_interval);
        
        if self.config.enable_logging {
            info!(
                "Cache maintenance started (interval: {:?})",
                self.config.cleanup_interval
            );
        }
        
        loop {
            ticker.tick().await;
            
            // Run cleanup
            let removed = self.cache.cleanup_expired();
            
            if self.config.enable_logging && removed > 0 {
                debug!("Cache cleanup: removed {} expired entries", removed);
            }
            
            // Check cache health
            let stats = self.cache.stats();
            let size = self.cache.size();
            let count = self.cache.len();
            
            if count > 0 {
                let avg_size = size / count;
                let hit_rate = stats.hit_rate();
                
                if self.config.enable_logging {
                    debug!(
                        "Cache stats: {} entries, {} bytes, {:.2}% hit rate, avg size: {} bytes",
                        count, size, hit_rate * 100.0, avg_size
                    );
                }
                
                // Warn on low hit rate
                if hit_rate < 0.5 && stats.get_hits() + stats.get_misses() > 1000 {
                    warn!(
                        "Low cache hit rate: {:.2}% (consider adjusting cache size or TTL)",
                        hit_rate * 100.0
                    );
                }
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::cache::LockFreeCache;
    
    #[tokio::test]
    async fn test_background_cleanup() {
        let cache = Arc::new(LockFreeCache::new(1024 * 1024));
        
        // Add entries with short TTL
        for i in 0..10 {
            cache.put(format!("key_{}", i), vec![0u8; 100], 1).unwrap();
        }
        
        assert_eq!(cache.len(), 10);
        
        // Start maintenance with short interval
        let config = MaintenanceConfig {
            cleanup_interval: Duration::from_millis(100),
            max_cleanup_per_run: 1000,
            enable_logging: false,
        };
        
        let maintenance = CacheMaintenance::new(cache.clone(), config);
        let handle = maintenance.start();
        
        // Wait for TTL to expire
        tokio::time::sleep(Duration::from_secs(2)).await;
        
        // Should be cleaned up
        assert_eq!(cache.len(), 0);
        
        handle.abort();
    }
}
