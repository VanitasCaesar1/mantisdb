//! Adaptive Connection Pool
//!
//! Auto-scaling connection pool with circuit breaker, health checks, and adaptive sizing

use crate::error::{Error, Result};
use crate::pool::{ConnectionPool, PoolConfig};
use parking_lot::RwLock;
use std::sync::Arc;
use std::time::{Duration, Instant};
use serde::{Serialize, Deserialize};

/// Adaptive connection pool with auto-scaling
pub struct AdaptivePool {
    inner: Arc<RwLock<AdaptivePoolInner>>,
    base_pool: ConnectionPool,
}

struct AdaptivePoolInner {
    config: AdaptiveConfig,
    metrics: PoolMetrics,
    circuit_breaker: CircuitBreaker,
    last_scale_time: Instant,
}

#[derive(Debug, Clone)]
pub struct AdaptiveConfig {
    /// Minimum pool size
    pub min_size: usize,
    /// Maximum pool size
    pub max_size: usize,
    /// Target utilization (0.0-1.0)
    pub target_utilization: f64,
    /// Scale up threshold
    pub scale_up_threshold: f64,
    /// Scale down threshold
    pub scale_down_threshold: f64,
    /// Minimum time between scaling operations
    pub scale_cooldown: Duration,
    /// Circuit breaker failure threshold
    pub failure_threshold: usize,
    /// Circuit breaker timeout
    pub circuit_timeout: Duration,
    /// Health check interval
    pub health_check_interval: Duration,
}

impl Default for AdaptiveConfig {
    fn default() -> Self {
        Self {
            min_size: 5,
            max_size: 50,
            target_utilization: 0.7,
            scale_up_threshold: 0.8,
            scale_down_threshold: 0.3,
            scale_cooldown: Duration::from_secs(30),
            failure_threshold: 5,
            circuit_timeout: Duration::from_secs(60),
            health_check_interval: Duration::from_secs(10),
        }
    }
}

#[derive(Debug, Clone, Serialize)]
pub struct PoolMetrics {
    pub total_connections: usize,
    pub active_connections: usize,
    pub idle_connections: usize,
    pub utilization: f64,
    pub total_requests: u64,
    pub failed_requests: u64,
    pub avg_wait_time_ms: f64,
    pub circuit_state: CircuitState,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum CircuitState {
    Closed,  // Normal operation
    Open,    // Circuit tripped, rejecting requests
    HalfOpen, // Testing if service recovered
}

struct CircuitBreaker {
    state: CircuitState,
    failure_count: usize,
    last_failure_time: Option<Instant>,
    success_count: usize,
    open_time: Option<Instant>,
}

impl AdaptivePool {
    /// Create a new adaptive pool
    pub fn new(config: AdaptiveConfig) -> Result<Self> {
        let pool_config = PoolConfig {
            max_size: config.min_size,
            timeout: Duration::from_secs(30),
            idle_timeout: Some(Duration::from_secs(300)),
        };
        
        let base_pool = ConnectionPool::new(pool_config)?;
        
        Ok(Self {
            inner: Arc::new(RwLock::new(AdaptivePoolInner {
                config: config.clone(),
                metrics: PoolMetrics {
                    total_connections: config.min_size,
                    active_connections: 0,
                    idle_connections: config.min_size,
                    utilization: 0.0,
                    total_requests: 0,
                    failed_requests: 0,
                    avg_wait_time_ms: 0.0,
                    circuit_state: CircuitState::Closed,
                },
                circuit_breaker: CircuitBreaker {
                    state: CircuitState::Closed,
                    failure_count: 0,
                    last_failure_time: None,
                    success_count: 0,
                    open_time: None,
                },
                last_scale_time: Instant::now(),
            })),
            base_pool,
        })
    }
    
    /// Execute a closure with a connection
    pub fn with_connection<F, T>(&self, f: F) -> Result<T>
    where
        F: FnOnce(&mut ()) -> Result<T>,
    {
        let start = Instant::now();
        
        // Check circuit breaker
        {
            let mut inner = self.inner.write();
            
            match inner.circuit_breaker.state {
                CircuitState::Open => {
                    // Check if circuit should transition to half-open
                    if let Some(open_time) = inner.circuit_breaker.open_time {
                        if open_time.elapsed() >= inner.config.circuit_timeout {
                            inner.circuit_breaker.state = CircuitState::HalfOpen;
                            inner.circuit_breaker.success_count = 0;
                        } else {
                            return Err(Error::General("Circuit breaker is open".to_string()));
                        }
                    }
                },
                _ => {}
            }
        }
        
        // Execute operation
        let result = self.base_pool.with_connection(f);
        
        // Update metrics and circuit breaker
        let mut inner = self.inner.write();
        let duration = start.elapsed();
        
        inner.metrics.total_requests += 1;
        
        match result {
            Ok(value) => {
                self.record_success(&mut inner);
                
                // Update average wait time
                let total_time = inner.metrics.avg_wait_time_ms * (inner.metrics.total_requests - 1) as f64;
                inner.metrics.avg_wait_time_ms = (total_time + duration.as_millis() as f64) 
                    / inner.metrics.total_requests as f64;
                
                Ok(value)
            },
            Err(e) => {
                self.record_failure(&mut inner);
                Err(e)
            }
        }
    }
    
    /// Update pool metrics
    pub fn update_metrics(&self) {
        let mut inner = self.inner.write();
        let stats = self.base_pool.stats();
        
        inner.metrics.total_connections = stats.size;
        inner.metrics.active_connections = stats.active;
        inner.metrics.idle_connections = stats.idle;
        inner.metrics.utilization = if stats.size > 0 {
            stats.active as f64 / stats.size as f64
        } else {
            0.0
        };
        inner.metrics.circuit_state = inner.circuit_breaker.state;
        
        // Check if scaling is needed
        self.check_and_scale(&mut inner);
    }
    
    /// Get current metrics
    pub fn metrics(&self) -> PoolMetrics {
        let inner = self.inner.read();
        inner.metrics.clone()
    }
    
    /// Manually scale the pool
    pub fn scale_to(&self, new_size: usize) -> Result<()> {
        let mut inner = self.inner.write();
        
        let clamped_size = new_size.clamp(inner.config.min_size, inner.config.max_size);
        
        // Note: In a real implementation, we'd resize the pool here
        // For now, just update metrics
        inner.metrics.total_connections = clamped_size;
        inner.last_scale_time = Instant::now();
        
        Ok(())
    }
    
    /// Reset circuit breaker
    pub fn reset_circuit(&self) {
        let mut inner = self.inner.write();
        inner.circuit_breaker.state = CircuitState::Closed;
        inner.circuit_breaker.failure_count = 0;
        inner.circuit_breaker.success_count = 0;
        inner.circuit_breaker.open_time = None;
    }
    
    // Private helper methods
    
    fn record_success(&self, inner: &mut AdaptivePoolInner) {
        match inner.circuit_breaker.state {
            CircuitState::HalfOpen => {
                inner.circuit_breaker.success_count += 1;
                // After 3 successes, close the circuit
                if inner.circuit_breaker.success_count >= 3 {
                    inner.circuit_breaker.state = CircuitState::Closed;
                    inner.circuit_breaker.failure_count = 0;
                }
            },
            CircuitState::Closed => {
                // Reset failure count on success
                if inner.circuit_breaker.failure_count > 0 {
                    inner.circuit_breaker.failure_count = 0;
                }
            },
            _ => {}
        }
    }
    
    fn record_failure(&self, inner: &mut AdaptivePoolInner) {
        inner.metrics.failed_requests += 1;
        
        match inner.circuit_breaker.state {
            CircuitState::Closed => {
                inner.circuit_breaker.failure_count += 1;
                inner.circuit_breaker.last_failure_time = Some(Instant::now());
                
                if inner.circuit_breaker.failure_count >= inner.config.failure_threshold {
                    // Trip the circuit
                    inner.circuit_breaker.state = CircuitState::Open;
                    inner.circuit_breaker.open_time = Some(Instant::now());
                }
            },
            CircuitState::HalfOpen => {
                // Failure during half-open, reopen the circuit
                inner.circuit_breaker.state = CircuitState::Open;
                inner.circuit_breaker.open_time = Some(Instant::now());
                inner.circuit_breaker.success_count = 0;
            },
            _ => {}
        }
    }
    
    fn check_and_scale(&self, inner: &mut AdaptivePoolInner) {
        // Don't scale if in cooldown
        if inner.last_scale_time.elapsed() < inner.config.scale_cooldown {
            return;
        }
        
        let utilization = inner.metrics.utilization;
        let current_size = inner.metrics.total_connections;
        
        // Scale up if utilization is high
        if utilization > inner.config.scale_up_threshold && current_size < inner.config.max_size {
            let new_size = (current_size as f64 * 1.5).ceil() as usize;
            let new_size = new_size.min(inner.config.max_size);
            
            if new_size > current_size {
                let _ = self.scale_to(new_size);
            }
        }
        // Scale down if utilization is low
        else if utilization < inner.config.scale_down_threshold && current_size > inner.config.min_size {
            let new_size = (current_size as f64 * 0.75).floor() as usize;
            let new_size = new_size.max(inner.config.min_size);
            
            if new_size < current_size {
                let _ = self.scale_to(new_size);
            }
        }
    }
}

impl Clone for AdaptivePool {
    fn clone(&self) -> Self {
        Self {
            inner: Arc::clone(&self.inner),
            base_pool: self.base_pool.clone(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_create_adaptive_pool() {
        let config = AdaptiveConfig::default();
        let pool = AdaptivePool::new(config);
        assert!(pool.is_ok());
    }
    
    #[test]
    fn test_metrics() {
        let config = AdaptiveConfig::default();
        let pool = AdaptivePool::new(config).unwrap();
        
        let metrics = pool.metrics();
        assert_eq!(metrics.circuit_state, CircuitState::Closed);
        assert_eq!(metrics.total_requests, 0);
    }
    
    #[test]
    fn test_circuit_breaker_opens() {
        let mut config = AdaptiveConfig::default();
        config.failure_threshold = 3;
        let pool = AdaptivePool::new(config).unwrap();
        
        // Simulate failures
        {
            let mut inner = pool.inner.write();
            for _ in 0..3 {
                pool.record_failure(&mut inner);
            }
            
            assert_eq!(inner.circuit_breaker.state, CircuitState::Open);
        }
    }
    
    #[test]
    fn test_circuit_recovery() {
        let mut config = AdaptiveConfig::default();
        config.circuit_timeout = Duration::from_millis(100);
        let pool = AdaptivePool::new(config).unwrap();
        
        // Trip the circuit
        {
            let mut inner = pool.inner.write();
            inner.circuit_breaker.state = CircuitState::Open;
            inner.circuit_breaker.open_time = Some(Instant::now() - Duration::from_secs(1));
        }
        
        // Should transition to half-open
        let result = pool.with_connection(|_| Ok(()));
        // Circuit should be in half-open state now
        let metrics = pool.metrics();
        assert_eq!(metrics.circuit_state, CircuitState::HalfOpen);
    }
    
    #[test]
    fn test_scaling() {
        let config = AdaptiveConfig {
            min_size: 5,
            max_size: 20,
            ..Default::default()
        };
        let pool = AdaptivePool::new(config).unwrap();
        
        pool.scale_to(15).unwrap();
        let metrics = pool.metrics();
        assert_eq!(metrics.total_connections, 15);
        
        // Test clamping
        pool.scale_to(100).unwrap();
        let metrics = pool.metrics();
        assert_eq!(metrics.total_connections, 20); // Clamped to max
    }
}
