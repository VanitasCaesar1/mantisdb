//! Observability System - Real-time monitoring, alerts, and query analysis
//!
//! Comprehensive observability with metrics collection, alerting, and analysis

use crate::error::{Error, Result};
use crate::query_analyzer::{QueryAnalyzer, QueryRecord, IndexSuggestion};
use parking_lot::RwLock;
use std::collections::{HashMap, VecDeque};
use std::sync::Arc;
use std::time::{Duration, Instant, SystemTime};
use serde::{Serialize, Deserialize};

/// Complete observability system
pub struct ObservabilitySystem {
    inner: Arc<RwLock<ObservabilityInner>>,
    query_analyzer: QueryAnalyzer,
}

struct ObservabilityInner {
    metrics: MetricsCollector,
    alerts: AlertManager,
    enabled: bool,
}

/// Real-time metrics collector
#[derive(Debug, Clone)]
pub struct MetricsCollector {
    pub query_count: u64,
    pub query_latencies: VecDeque<Duration>,
    pub error_count: u64,
    pub cache_hits: u64,
    pub cache_misses: u64,
    pub active_connections: u32,
    pub throughput_history: VecDeque<ThroughputSample>,
    pub resource_usage: ResourceUsage,
    pub start_time: Instant,
    max_latency_samples: usize,
    max_throughput_samples: usize,
}

#[derive(Debug, Clone, Serialize)]
pub struct ThroughputSample {
    pub timestamp: SystemTime,
    pub queries_per_second: f64,
    pub bytes_per_second: u64,
}

#[derive(Debug, Clone, Serialize)]
pub struct ResourceUsage {
    pub cpu_percent: f64,
    pub memory_bytes: u64,
    pub disk_bytes: u64,
    pub network_rx_bytes: u64,
    pub network_tx_bytes: u64,
}

/// Alert management system
#[derive(Debug, Clone)]
pub struct AlertManager {
    alerts: Vec<Alert>,
    rules: Vec<AlertRule>,
    max_alerts: usize,
}

#[derive(Debug, Clone, Serialize)]
pub struct Alert {
    pub id: String,
    pub rule_id: String,
    pub severity: AlertSeverity,
    pub message: String,
    pub timestamp: SystemTime,
    pub metadata: HashMap<String, String>,
    pub resolved: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum AlertSeverity {
    Info,
    Warning,
    Critical,
}

#[derive(Debug, Clone)]
pub struct AlertRule {
    pub id: String,
    pub name: String,
    pub condition: AlertCondition,
    pub threshold: f64,
    pub severity: AlertSeverity,
    pub enabled: bool,
}

#[derive(Debug, Clone)]
pub enum AlertCondition {
    HighLatency,
    HighErrorRate,
    LowCacheHitRatio,
    HighMemoryUsage,
    HighConnectionCount,
    SlowQueryDetected,
}

/// Dashboard metrics snapshot
#[derive(Debug, Serialize)]
pub struct DashboardMetrics {
    pub overview: OverviewMetrics,
    pub performance: PerformanceMetrics,
    pub resources: ResourceMetrics,
    pub recent_alerts: Vec<Alert>,
    pub index_suggestions: Vec<IndexSuggestion>,
    pub timestamp: SystemTime,
}

#[derive(Debug, Serialize)]
pub struct OverviewMetrics {
    pub uptime_seconds: u64,
    pub total_queries: u64,
    pub queries_per_second: f64,
    pub error_rate: f64,
    pub active_connections: u32,
}

#[derive(Debug, Serialize)]
pub struct PerformanceMetrics {
    pub avg_latency_ms: f64,
    pub p50_latency_ms: f64,
    pub p95_latency_ms: f64,
    pub p99_latency_ms: f64,
    pub cache_hit_ratio: f64,
    pub throughput_qps: f64,
}

#[derive(Debug, Serialize)]
pub struct ResourceMetrics {
    pub cpu_percent: f64,
    pub memory_bytes: u64,
    pub disk_bytes: u64,
    pub network_rx_bytes: u64,
    pub network_tx_bytes: u64,
}

impl ObservabilitySystem {
    /// Create a new observability system
    pub fn new() -> Self {
        let query_analyzer = QueryAnalyzer::new(100); // 100ms threshold
        
        let inner = ObservabilityInner {
            metrics: MetricsCollector::new(),
            alerts: AlertManager::new(),
            enabled: true,
        };
        
        let mut sys = Self {
            inner: Arc::new(RwLock::new(inner)),
            query_analyzer,
        };
        
        // Add default alert rules
        sys.add_default_rules();
        
        sys
    }
    
    /// Enable observability
    pub fn enable(&self) {
        let mut inner = self.inner.write();
        inner.enabled = true;
        self.query_analyzer.enable();
    }
    
    /// Disable observability
    pub fn disable(&self) {
        let mut inner = self.inner.write();
        inner.enabled = false;
        self.query_analyzer.disable();
    }
    
    /// Record a query execution
    pub fn record_query(&self, duration: Duration, table: String, columns: QueryColumns) {
        let mut inner = self.inner.write();
        
        if !inner.enabled {
            return;
        }
        
        inner.metrics.record_query(duration);
        
        // Also record in query analyzer
        drop(inner);
        self.query_analyzer.record_query(QueryRecord {
            query: format!("SELECT ... FROM {}", table),
            duration,
            timestamp: Instant::now(),
            table,
            where_columns: columns.where_cols,
            order_by_columns: columns.order_cols,
        });
        
        // Check alerts
        self.check_alerts();
    }
    
    /// Record a query error
    pub fn record_error(&self) {
        let mut inner = self.inner.write();
        
        if inner.enabled {
            inner.metrics.record_error();
        }
    }
    
    /// Record cache hit
    pub fn record_cache_hit(&self) {
        let mut inner = self.inner.write();
        
        if inner.enabled {
            inner.metrics.record_cache_hit();
        }
    }
    
    /// Record cache miss
    pub fn record_cache_miss(&self) {
        let mut inner = self.inner.write();
        
        if inner.enabled {
            inner.metrics.record_cache_miss();
        }
    }
    
    /// Update connection count
    pub fn update_connections(&self, count: u32) {
        let mut inner = self.inner.write();
        
        if inner.enabled {
            inner.metrics.active_connections = count;
        }
    }
    
    /// Update resource usage
    pub fn update_resources(&self, usage: ResourceUsage) {
        let mut inner = self.inner.write();
        
        if inner.enabled {
            inner.metrics.resource_usage = usage;
        }
    }
    
    /// Get dashboard metrics
    pub fn get_dashboard_metrics(&self) -> Result<DashboardMetrics> {
        let inner = self.inner.read();
        let metrics = &inner.metrics;
        let alerts = &inner.alerts;
        
        // Calculate percentiles
        let mut latencies: Vec<_> = metrics.query_latencies.iter()
            .map(|d| d.as_millis() as f64)
            .collect();
        latencies.sort_by(|a, b| a.partial_cmp(b).unwrap());
        
        let p50 = percentile(&latencies, 0.50);
        let p95 = percentile(&latencies, 0.95);
        let p99 = percentile(&latencies, 0.99);
        let avg = if !latencies.is_empty() {
            latencies.iter().sum::<f64>() / latencies.len() as f64
        } else {
            0.0
        };
        
        // Calculate cache hit ratio
        let total_cache_ops = metrics.cache_hits + metrics.cache_misses;
        let cache_hit_ratio = if total_cache_ops > 0 {
            metrics.cache_hits as f64 / total_cache_ops as f64
        } else {
            0.0
        };
        
        // Calculate QPS
        let uptime = metrics.start_time.elapsed().as_secs();
        let qps = if uptime > 0 {
            metrics.query_count as f64 / uptime as f64
        } else {
            0.0
        };
        
        // Calculate error rate
        let error_rate = if metrics.query_count > 0 {
            metrics.error_count as f64 / metrics.query_count as f64
        } else {
            0.0
        };
        
        // Get index suggestions
        drop(inner);
        let index_suggestions = self.query_analyzer.analyze()?;
        let inner = self.inner.read();
        
        Ok(DashboardMetrics {
            overview: OverviewMetrics {
                uptime_seconds: uptime,
                total_queries: metrics.query_count,
                queries_per_second: qps,
                error_rate,
                active_connections: metrics.active_connections,
            },
            performance: PerformanceMetrics {
                avg_latency_ms: avg,
                p50_latency_ms: p50,
                p95_latency_ms: p95,
                p99_latency_ms: p99,
                cache_hit_ratio,
                throughput_qps: qps,
            },
            resources: ResourceMetrics {
                cpu_percent: metrics.resource_usage.cpu_percent,
                memory_bytes: metrics.resource_usage.memory_bytes,
                disk_bytes: metrics.resource_usage.disk_bytes,
                network_rx_bytes: metrics.resource_usage.network_rx_bytes,
                network_tx_bytes: metrics.resource_usage.network_tx_bytes,
            },
            recent_alerts: alerts.get_recent_alerts(10),
            index_suggestions: index_suggestions.into_iter().take(5).collect(),
            timestamp: SystemTime::now(),
        })
    }
    
    /// Add a custom alert rule
    pub fn add_alert_rule(&self, rule: AlertRule) {
        let mut inner = self.inner.write();
        inner.alerts.add_rule(rule);
    }
    
    /// Get all alerts
    pub fn get_alerts(&self) -> Vec<Alert> {
        let inner = self.inner.read();
        inner.alerts.get_all_alerts()
    }
    
    /// Get all active alerts
    pub fn get_active_alerts(&self) -> Vec<Alert> {
        let inner = self.inner.read();
        inner.alerts.get_active_alerts()
    }
    
    /// Resolve an alert
    pub fn resolve_alert(&self, alert_id: &str) {
        let mut inner = self.inner.write();
        inner.alerts.resolve_alert(alert_id);
    }
    
    /// Clear all resolved alerts
    pub fn clear_resolved_alerts(&self) {
        let mut inner = self.inner.write();
        inner.alerts.clear_resolved();
    }
    
    /// Check alert conditions
    fn check_alerts(&self) {
        let mut inner = self.inner.write();
        let metrics = &inner.metrics;
        
        // Check each rule
        for rule in inner.alerts.rules.clone() {
            if !rule.enabled {
                continue;
            }
            
            let triggered = match rule.condition {
                AlertCondition::HighLatency => {
                    if let Some(latest) = metrics.query_latencies.back() {
                        latest.as_millis() as f64 > rule.threshold
                    } else {
                        false
                    }
                },
                AlertCondition::HighErrorRate => {
                    let error_rate = if metrics.query_count > 0 {
                        metrics.error_count as f64 / metrics.query_count as f64
                    } else {
                        0.0
                    };
                    error_rate > rule.threshold
                },
                AlertCondition::LowCacheHitRatio => {
                    let total = metrics.cache_hits + metrics.cache_misses;
                    let ratio = if total > 0 {
                        metrics.cache_hits as f64 / total as f64
                    } else {
                        1.0
                    };
                    ratio < rule.threshold
                },
                AlertCondition::HighMemoryUsage => {
                    let usage_gb = metrics.resource_usage.memory_bytes as f64 / 1_073_741_824.0;
                    usage_gb > rule.threshold
                },
                AlertCondition::HighConnectionCount => {
                    metrics.active_connections as f64 > rule.threshold
                },
                AlertCondition::SlowQueryDetected => {
                    // Checked by query analyzer
                    false
                },
            };
            
            if triggered {
                inner.alerts.trigger_alert(&rule, metrics);
            }
        }
    }
    
    /// Add default alert rules
    fn add_default_rules(&mut self) {
        let rules = vec![
            AlertRule {
                id: "high_latency".to_string(),
                name: "High Query Latency".to_string(),
                condition: AlertCondition::HighLatency,
                threshold: 500.0, // 500ms
                severity: AlertSeverity::Warning,
                enabled: true,
            },
            AlertRule {
                id: "high_error_rate".to_string(),
                name: "High Error Rate".to_string(),
                condition: AlertCondition::HighErrorRate,
                threshold: 0.05, // 5%
                severity: AlertSeverity::Critical,
                enabled: true,
            },
            AlertRule {
                id: "low_cache_hit".to_string(),
                name: "Low Cache Hit Ratio".to_string(),
                condition: AlertCondition::LowCacheHitRatio,
                threshold: 0.5, // 50%
                severity: AlertSeverity::Warning,
                enabled: true,
            },
            AlertRule {
                id: "high_memory".to_string(),
                name: "High Memory Usage".to_string(),
                condition: AlertCondition::HighMemoryUsage,
                threshold: 8.0, // 8GB
                severity: AlertSeverity::Critical,
                enabled: true,
            },
        ];
        
        let mut inner = self.inner.write();
        for rule in rules {
            inner.alerts.add_rule(rule);
        }
    }
}

impl Clone for ObservabilitySystem {
    fn clone(&self) -> Self {
        Self {
            inner: Arc::clone(&self.inner),
            query_analyzer: self.query_analyzer.clone(),
        }
    }
}

/// Query column information
pub struct QueryColumns {
    pub where_cols: Vec<String>,
    pub order_cols: Vec<String>,
}

impl MetricsCollector {
    fn new() -> Self {
        Self {
            query_count: 0,
            query_latencies: VecDeque::new(),
            error_count: 0,
            cache_hits: 0,
            cache_misses: 0,
            active_connections: 0,
            throughput_history: VecDeque::new(),
            resource_usage: ResourceUsage {
                cpu_percent: 0.0,
                memory_bytes: 0,
                disk_bytes: 0,
                network_rx_bytes: 0,
                network_tx_bytes: 0,
            },
            start_time: Instant::now(),
            max_latency_samples: 10000,
            max_throughput_samples: 1000,
        }
    }
    
    fn record_query(&mut self, duration: Duration) {
        self.query_count += 1;
        self.query_latencies.push_back(duration);
        
        if self.query_latencies.len() > self.max_latency_samples {
            self.query_latencies.pop_front();
        }
    }
    
    fn record_error(&mut self) {
        self.error_count += 1;
    }
    
    fn record_cache_hit(&mut self) {
        self.cache_hits += 1;
    }
    
    fn record_cache_miss(&mut self) {
        self.cache_misses += 1;
    }
}

impl AlertManager {
    fn new() -> Self {
        Self {
            alerts: Vec::new(),
            rules: Vec::new(),
            max_alerts: 1000,
        }
    }
    
    fn add_rule(&mut self, rule: AlertRule) {
        self.rules.push(rule);
    }
    
    fn trigger_alert(&mut self, rule: &AlertRule, metrics: &MetricsCollector) {
        // Check if alert already exists for this rule
        let existing = self.alerts.iter()
            .any(|a| a.rule_id == rule.id && !a.resolved);
        
        if existing {
            return; // Don't duplicate alerts
        }
        
        let alert = Alert {
            id: format!("alert_{}", chrono::Utc::now().timestamp()),
            rule_id: rule.id.clone(),
            severity: rule.severity.clone(),
            message: format!("{} triggered", rule.name),
            timestamp: SystemTime::now(),
            metadata: HashMap::new(),
            resolved: false,
        };
        
        self.alerts.push(alert);
        
        if self.alerts.len() > self.max_alerts {
            self.alerts.remove(0);
        }
    }
    
    fn get_all_alerts(&self) -> Vec<Alert> {
        self.alerts.clone()
    }
    
    fn get_active_alerts(&self) -> Vec<Alert> {
        self.alerts.iter()
            .filter(|a| !a.resolved)
            .cloned()
            .collect()
    }
    
    fn get_recent_alerts(&self, limit: usize) -> Vec<Alert> {
        self.alerts.iter()
            .rev()
            .take(limit)
            .cloned()
            .collect()
    }
    
    fn resolve_alert(&mut self, alert_id: &str) {
        if let Some(alert) = self.alerts.iter_mut().find(|a| a.id == alert_id) {
            alert.resolved = true;
        }
    }
    
    fn clear_resolved(&mut self) {
        self.alerts.retain(|a| !a.resolved);
    }
}

/// Calculate percentile from sorted data
fn percentile(sorted_data: &[f64], p: f64) -> f64 {
    if sorted_data.is_empty() {
        return 0.0;
    }
    
    let idx = ((sorted_data.len() as f64) * p) as usize;
    let idx = idx.min(sorted_data.len() - 1);
    sorted_data[idx]
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_record_query() {
        let obs = ObservabilitySystem::new();
        
        obs.record_query(
            Duration::from_millis(50),
            "users".to_string(),
            QueryColumns {
                where_cols: vec!["email".to_string()],
                order_cols: vec![],
            },
        );
        
        let metrics = obs.get_dashboard_metrics().unwrap();
        assert_eq!(metrics.overview.total_queries, 1);
    }
    
    #[test]
    fn test_cache_metrics() {
        let obs = ObservabilitySystem::new();
        
        obs.record_cache_hit();
        obs.record_cache_hit();
        obs.record_cache_miss();
        
        let metrics = obs.get_dashboard_metrics().unwrap();
        assert!(metrics.performance.cache_hit_ratio > 0.6);
    }
    
    #[test]
    fn test_alerts() {
        let obs = ObservabilitySystem::new();
        
        // Trigger high latency alert
        obs.record_query(
            Duration::from_millis(600),
            "test".to_string(),
            QueryColumns {
                where_cols: vec![],
                order_cols: vec![],
            },
        );
        
        let alerts = obs.get_active_alerts();
        assert!(!alerts.is_empty());
    }
}
