//! Query Analyzer - Automatic index suggestions and optimization
//!
//! Analyzes slow queries and suggests optimal indexes

use crate::error::{Error, Result};
use parking_lot::RwLock;
use std::collections::HashMap;
use std::sync::Arc;
use std::time::{Duration, Instant};

/// Query performance record
#[derive(Debug, Clone)]
pub struct QueryRecord {
    pub query: String,
    pub duration: Duration,
    pub timestamp: Instant,
    pub table: String,
    pub where_columns: Vec<String>,
    pub order_by_columns: Vec<String>,
}

/// Index suggestion
#[derive(Debug, Clone)]
pub struct IndexSuggestion {
    pub table: String,
    pub columns: Vec<String>,
    pub index_type: IndexType,
    pub reason: String,
    pub estimated_improvement: f64, // Speedup factor (e.g., 225.0 = 225x faster)
    pub current_avg_ms: f64,
    pub estimated_avg_ms: f64,
    pub affected_queries: usize,
}

#[derive(Debug, Clone, PartialEq)]
pub enum IndexType {
    BTree,
    Hash,
    FullText,
}

impl IndexType {
    pub fn to_sql(&self) -> &str {
        match self {
            IndexType::BTree => "BTREE",
            IndexType::Hash => "HASH",
            IndexType::FullText => "FULLTEXT",
        }
    }
}

/// Query analyzer that tracks performance and suggests optimizations
pub struct QueryAnalyzer {
    inner: Arc<RwLock<QueryAnalyzerInner>>,
}

struct QueryAnalyzerInner {
    slow_query_threshold: Duration,
    query_records: Vec<QueryRecord>,
    query_patterns: HashMap<String, QueryPattern>,
    enabled: bool,
    max_records: usize,
}

#[derive(Debug, Clone)]
struct QueryPattern {
    pattern: String,
    count: usize,
    total_duration: Duration,
    avg_duration: Duration,
}

impl QueryAnalyzer {
    /// Create a new query analyzer
    pub fn new(slow_query_threshold_ms: u64) -> Self {
        Self {
            inner: Arc::new(RwLock::new(QueryAnalyzerInner {
                slow_query_threshold: Duration::from_millis(slow_query_threshold_ms),
                query_records: Vec::new(),
                query_patterns: HashMap::new(),
                enabled: true,
                max_records: 10000,
            })),
        }
    }
    
    /// Enable query analysis
    pub fn enable(&self) {
        let mut inner = self.inner.write();
        inner.enabled = true;
    }
    
    /// Disable query analysis
    pub fn disable(&self) {
        let mut inner = self.inner.write();
        inner.enabled = false;
    }
    
    /// Record a query execution
    pub fn record_query(&self, record: QueryRecord) {
        let mut inner = self.inner.write();
        
        if !inner.enabled {
            return;
        }
        
        // Update pattern statistics
        let pattern = Self::extract_pattern(&record.query);
        let entry = inner.query_patterns.entry(pattern.clone()).or_insert(QueryPattern {
            pattern: pattern.clone(),
            count: 0,
            total_duration: Duration::ZERO,
            avg_duration: Duration::ZERO,
        });
        
        entry.count += 1;
        entry.total_duration += record.duration;
        entry.avg_duration = entry.total_duration / entry.count as u32;
        
        // Store record if slow
        if record.duration >= inner.slow_query_threshold {
            inner.query_records.push(record);
            
            // Limit records to prevent unbounded growth
            if inner.query_records.len() > inner.max_records {
                inner.query_records.remove(0);
            }
        }
    }
    
    /// Analyze queries and generate index suggestions
    pub fn analyze(&self) -> Result<Vec<IndexSuggestion>> {
        let inner = self.inner.read();
        let mut suggestions = Vec::new();
        
        // Group slow queries by table
        let mut table_queries: HashMap<String, Vec<&QueryRecord>> = HashMap::new();
        for record in &inner.query_records {
            table_queries.entry(record.table.clone())
                .or_insert_with(Vec::new)
                .push(record);
        }
        
        // Analyze each table's queries
        for (table, queries) in table_queries {
            // Find columns used in WHERE clauses
            let mut where_column_usage: HashMap<String, usize> = HashMap::new();
            for query in &queries {
                for col in &query.where_columns {
                    *where_column_usage.entry(col.clone()).or_insert(0) += 1;
                }
            }
            
            // Suggest index for frequently filtered columns
            for (column, count) in where_column_usage {
                if count >= 3 { // At least 3 queries use this column
                    let affected: Vec<_> = queries.iter()
                        .filter(|q| q.where_columns.contains(&column))
                        .collect();
                    
                    let avg_duration = affected.iter()
                        .map(|q| q.duration.as_millis() as f64)
                        .sum::<f64>() / affected.len() as f64;
                    
                    // Estimate improvement (indexed lookup is typically 100-1000x faster)
                    let estimated_improvement = 200.0; // Conservative estimate
                    let estimated_duration = avg_duration / estimated_improvement;
                    
                    suggestions.push(IndexSuggestion {
                        table: table.clone(),
                        columns: vec![column.clone()],
                        index_type: IndexType::BTree,
                        reason: format!(
                            "{} queries filtering on '{}' averaging {:.1}ms",
                            affected.len(),
                            column,
                            avg_duration
                        ),
                        estimated_improvement,
                        current_avg_ms: avg_duration,
                        estimated_avg_ms: estimated_duration,
                        affected_queries: affected.len(),
                    });
                }
            }
            
            // Find columns used in ORDER BY
            let mut order_column_usage: HashMap<String, usize> = HashMap::new();
            for query in &queries {
                for col in &query.order_by_columns {
                    *order_column_usage.entry(col.clone()).or_insert(0) += 1;
                }
            }
            
            // Suggest index for frequently sorted columns
            for (column, count) in order_column_usage {
                if count >= 3 {
                    let affected: Vec<_> = queries.iter()
                        .filter(|q| q.order_by_columns.contains(&column))
                        .collect();
                    
                    let avg_duration = affected.iter()
                        .map(|q| q.duration.as_millis() as f64)
                        .sum::<f64>() / affected.len() as f64;
                    
                    let estimated_improvement = 150.0;
                    let estimated_duration = avg_duration / estimated_improvement;
                    
                    suggestions.push(IndexSuggestion {
                        table: table.clone(),
                        columns: vec![column.clone()],
                        index_type: IndexType::BTree,
                        reason: format!(
                            "{} queries sorting by '{}' averaging {:.1}ms",
                            affected.len(),
                            column,
                            avg_duration
                        ),
                        estimated_improvement,
                        current_avg_ms: avg_duration,
                        estimated_avg_ms: estimated_duration,
                        affected_queries: affected.len(),
                    });
                }
            }
        }
        
        // Sort by impact (improvement * affected queries)
        suggestions.sort_by(|a, b| {
            let impact_a = a.estimated_improvement * a.affected_queries as f64;
            let impact_b = b.estimated_improvement * b.affected_queries as f64;
            impact_b.partial_cmp(&impact_a).unwrap()
        });
        
        Ok(suggestions)
    }
    
    /// Generate SQL for creating suggested indexes
    pub fn generate_index_sql(&self, suggestion: &IndexSuggestion) -> String {
        let index_name = format!(
            "idx_{}_{}", 
            suggestion.table,
            suggestion.columns.join("_")
        );
        
        format!(
            "CREATE INDEX {} ON {} ({}) USING {};",
            index_name,
            suggestion.table,
            suggestion.columns.join(", "),
            suggestion.index_type.to_sql()
        )
    }
    
    /// Get slow query statistics
    pub fn get_slow_queries(&self) -> Vec<QueryRecord> {
        let inner = self.inner.read();
        inner.query_records.clone()
    }
    
    /// Get query pattern statistics
    pub fn get_query_patterns(&self) -> Vec<QueryPattern> {
        let inner = self.inner.read();
        inner.query_patterns.values().cloned().collect()
    }
    
    /// Clear all recorded data
    pub fn clear(&self) {
        let mut inner = self.inner.write();
        inner.query_records.clear();
        inner.query_patterns.clear();
    }
    
    /// Extract query pattern (normalize parameters)
    fn extract_pattern(query: &str) -> String {
        // Simple pattern extraction - replace literals with ?
        // In production, use a proper SQL parser
        query
            .replace(|c: char| c.is_numeric(), "?")
            .replace("'.*?'", "?")
            .to_lowercase()
    }
}

impl Clone for QueryAnalyzer {
    fn clone(&self) -> Self {
        Self {
            inner: Arc::clone(&self.inner),
        }
    }
}

/// Helper to format suggestions for display
impl IndexSuggestion {
    pub fn format(&self) -> String {
        format!(
            "🐌 Slow queries detected on table '{}'
💡 Suggestion: CREATE INDEX idx_{}_{} ON {} ({})
📈 Expected improvement: {:.1}ms → {:.1}ms ({:.0}x faster)
📊 Affects {} queries
💭 Reason: {}
",
            self.table,
            self.table,
            self.columns.join("_"),
            self.table,
            self.columns.join(", "),
            self.current_avg_ms,
            self.estimated_avg_ms,
            self.estimated_improvement,
            self.affected_queries,
            self.reason
        )
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_query_recording() {
        let analyzer = QueryAnalyzer::new(100);
        
        analyzer.record_query(QueryRecord {
            query: "SELECT * FROM users WHERE email = 'test@example.com'".to_string(),
            duration: Duration::from_millis(150),
            timestamp: Instant::now(),
            table: "users".to_string(),
            where_columns: vec!["email".to_string()],
            order_by_columns: vec![],
        });
        
        let slow_queries = analyzer.get_slow_queries();
        assert_eq!(slow_queries.len(), 1);
    }
    
    #[test]
    fn test_index_suggestions() {
        let analyzer = QueryAnalyzer::new(50);
        
        // Record multiple slow queries on same column
        for _ in 0..5 {
            analyzer.record_query(QueryRecord {
                query: "SELECT * FROM users WHERE email = ?".to_string(),
                duration: Duration::from_millis(200),
                timestamp: Instant::now(),
                table: "users".to_string(),
                where_columns: vec!["email".to_string()],
                order_by_columns: vec![],
            });
        }
        
        let suggestions = analyzer.analyze().unwrap();
        assert!(!suggestions.is_empty());
        assert_eq!(suggestions[0].table, "users");
        assert_eq!(suggestions[0].columns[0], "email");
    }
    
    #[test]
    fn test_sql_generation() {
        let analyzer = QueryAnalyzer::new(100);
        
        let suggestion = IndexSuggestion {
            table: "users".to_string(),
            columns: vec!["email".to_string()],
            index_type: IndexType::BTree,
            reason: "Test".to_string(),
            estimated_improvement: 200.0,
            current_avg_ms: 400.0,
            estimated_avg_ms: 2.0,
            affected_queries: 10,
        };
        
        let sql = analyzer.generate_index_sql(&suggestion);
        assert!(sql.contains("CREATE INDEX"));
        assert!(sql.contains("idx_users_email"));
    }
    
    #[test]
    fn test_disable_enable() {
        let analyzer = QueryAnalyzer::new(100);
        
        analyzer.disable();
        analyzer.record_query(QueryRecord {
            query: "SELECT * FROM test".to_string(),
            duration: Duration::from_millis(200),
            timestamp: Instant::now(),
            table: "test".to_string(),
            where_columns: vec![],
            order_by_columns: vec![],
        });
        
        assert_eq!(analyzer.get_slow_queries().len(), 0);
        
        analyzer.enable();
        analyzer.record_query(QueryRecord {
            query: "SELECT * FROM test".to_string(),
            duration: Duration::from_millis(200),
            timestamp: Instant::now(),
            table: "test".to_string(),
            where_columns: vec![],
            order_by_columns: vec![],
        });
        
        assert_eq!(analyzer.get_slow_queries().len(), 1);
    }
}
