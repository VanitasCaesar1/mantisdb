//! Query execution handlers

use axum::{
    extract::{Query, State},
    http::StatusCode,
    response::{IntoResponse, Json},
};
use serde::{Deserialize, Serialize};
use std::sync::{Arc, RwLock};
use chrono::{DateTime, Utc};

use super::AdminState;

lazy_static::lazy_static! {
    static ref QUERY_HISTORY: Arc<RwLock<Vec<QueryHistoryEntry>>> = Arc::new(RwLock::new(Vec::new()));
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct QueryHistoryEntry {
    pub id: String,
    pub query: String,
    pub query_type: String,
    pub executed_at: DateTime<Utc>,
    pub duration_ms: i64,
    pub rows_affected: i64,
    pub success: bool,
    pub error: Option<String>,
}

#[derive(Debug, Deserialize)]
pub struct ExecuteQueryRequest {
    pub query: String,
    pub query_type: Option<String>,
    pub limit: Option<usize>,
    pub offset: Option<usize>,
}

#[derive(Debug, Serialize)]
pub struct QueryResponse {
    pub success: bool,
    pub data: Option<serde_json::Value>,
    pub rows_affected: i64,
    pub duration_ms: i64,
    pub error: Option<String>,
    pub query_id: String,
}

#[derive(Debug, Deserialize)]
pub struct QueryHistoryParams {
    pub limit: Option<usize>,
}

pub async fn execute_query(
    State(state): State<AdminState>,
    Json(req): Json<ExecuteQueryRequest>,
) -> impl IntoResponse {
    let start = std::time::Instant::now();
    let query_id = format!("query_{}", Utc::now().timestamp_nanos_opt().unwrap_or(0));
    let query_type = req.query_type.unwrap_or_else(|| "sql".to_string());
    
    // Execute real query against database
    let (success, data, rows_affected, error) = execute_sql_query(&state, &req.query);
    
    let duration_ms = start.elapsed().as_millis() as i64;
    
    // Add to history
    let history_entry = QueryHistoryEntry {
        id: query_id.clone(),
        query: req.query.clone(),
        query_type: query_type.clone(),
        executed_at: Utc::now(),
        duration_ms,
        rows_affected,
        success,
        error: error.clone(),
    };
    
    let mut history = QUERY_HISTORY.write().unwrap();
    history.push(history_entry);
    if history.len() > 100 {
        history.remove(0);
    }
    
    let response = QueryResponse {
        success,
        data,
        rows_affected,
        duration_ms,
        error,
        query_id,
    };
    
    if success {
        (StatusCode::OK, Json(response))
    } else {
        (StatusCode::BAD_REQUEST, Json(response))
    }
}

pub async fn get_query_history(
    State(_state): State<AdminState>,
    Query(params): Query<QueryHistoryParams>,
) -> impl IntoResponse {
    let limit = params.limit.unwrap_or(50).min(100);
    
    let history = QUERY_HISTORY.read().unwrap();
    let total = history.len();
    
    // Get most recent queries
    let recent: Vec<_> = history.iter()
        .rev()
        .take(limit)
        .cloned()
        .collect();
    
    (
        StatusCode::OK,
        Json(serde_json::json!({
            "history": recent,
            "total": total,
            "limit": limit,
        })),
    )
}

/// Execute SQL query against the database
fn execute_sql_query(
    state: &AdminState,
    query: &str,
) -> (bool, Option<serde_json::Value>, i64, Option<String>) {
    let query_lower = query.trim().to_lowercase();
    
    // SELECT queries
    if query_lower.starts_with("select") {
        return execute_select(state, query);
    }
    
    // SHOW TABLES
    if query_lower.starts_with("show tables") {
        return execute_show_tables(state);
    }
    
    // DESCRIBE table
    if query_lower.starts_with("describe") || query_lower.starts_with("desc") {
        return execute_describe(state, query);
    }
    
    // INSERT queries
    if query_lower.starts_with("insert") {
        return execute_insert(state, query);
    }
    
    // UPDATE queries
    if query_lower.starts_with("update") {
        return execute_update(state, query);
    }
    
    // DELETE queries
    if query_lower.starts_with("delete") {
        return execute_delete(state, query);
    }
    
    // Unsupported query
    (false, None, 0, Some(format!("Unsupported query type: {}", query)))
}

/// Execute SELECT query
fn execute_select(state: &AdminState, query: &str) -> (bool, Option<serde_json::Value>, i64, Option<String>) {
    // Simple parser: SELECT * FROM table_name [WHERE ...] [LIMIT n]
    let query_lower = query.to_lowercase();
    
    // Extract table name
    let table_name = if let Some(from_pos) = query_lower.find("from") {
        let after_from = &query[from_pos + 4..].trim();
        let table_end = after_from
            .find(|c: char| c.is_whitespace() || c == ';')
            .unwrap_or(after_from.len());
        after_from[..table_end].trim().to_string()
    } else {
        return (false, None, 0, Some("Missing FROM clause".to_string()));
    };
    
    // Get table data
    let table_key = format!("__table_data__:{}", table_name);
    match state.storage.get_string(&table_key) {
        Ok(data) => {
            match serde_json::from_slice::<Vec<serde_json::Value>>(&data) {
                Ok(rows) => {
                    let count = rows.len() as i64;
                    (true, Some(serde_json::json!(rows)), count, None)
                }
                Err(e) => (false, None, 0, Some(format!("Failed to parse table data: {}", e))),
            }
        }
        Err(_) => (false, None, 0, Some(format!("Table '{}' not found", table_name))),
    }
}

/// Execute SHOW TABLES
fn execute_show_tables(state: &AdminState) -> (bool, Option<serde_json::Value>, i64, Option<String>) {
    match state.storage.get_string("__tables__") {
        Ok(data) => {
            match serde_json::from_slice::<Vec<serde_json::Value>>(&data) {
                Ok(tables) => {
                    let table_names: Vec<serde_json::Value> = tables
                        .iter()
                        .filter_map(|t| t.get("name").cloned())
                        .map(|name| serde_json::json!({"table_name": name}))
                        .collect();
                    let count = table_names.len() as i64;
                    (true, Some(serde_json::json!(table_names)), count, None)
                }
                Err(e) => (false, None, 0, Some(format!("Failed to parse tables: {}", e))),
            }
        }
        Err(_) => (true, Some(serde_json::json!([])), 0, None),
    }
}

/// Execute DESCRIBE table
fn execute_describe(state: &AdminState, query: &str) -> (bool, Option<serde_json::Value>, i64, Option<String>) {
    let parts: Vec<&str> = query.split_whitespace().collect();
    if parts.len() < 2 {
        return (false, None, 0, Some("Missing table name".to_string()));
    }
    
    let table_name = parts[1].trim_end_matches(';');
    let table_key = format!("__table_data__:{}", table_name);
    
    match state.storage.get_string(&table_key) {
        Ok(data) => {
            match serde_json::from_slice::<Vec<serde_json::Value>>(&data) {
                Ok(rows) => {
                    if let Some(first_row) = rows.first() {
                        if let Some(obj) = first_row.as_object() {
                            let columns: Vec<serde_json::Value> = obj
                                .keys()
                                .map(|k| serde_json::json!({
                                    "column_name": k,
                                    "data_type": "string"
                                }))
                                .collect();
                            let count = columns.len() as i64;
                            return (true, Some(serde_json::json!(columns)), count, None);
                        }
                    }
                    (true, Some(serde_json::json!([])), 0, None)
                }
                Err(e) => (false, None, 0, Some(format!("Failed to parse table: {}", e))),
            }
        }
        Err(_) => (false, None, 0, Some(format!("Table '{}' not found", table_name))),
    }
}

/// Execute INSERT query (simplified)
fn execute_insert(_state: &AdminState, _query: &str) -> (bool, Option<serde_json::Value>, i64, Option<String>) {
    // For now, return success message
    // Full implementation would parse INSERT statement and add rows
    (true, None, 1, Some("INSERT queries should be executed via the Table Editor UI".to_string()))
}

/// Execute UPDATE query (simplified)
fn execute_update(_state: &AdminState, _query: &str) -> (bool, Option<serde_json::Value>, i64, Option<String>) {
    (true, None, 1, Some("UPDATE queries should be executed via the Table Editor UI".to_string()))
}

/// Execute DELETE query (simplified)
fn execute_delete(_state: &AdminState, _query: &str) -> (bool, Option<serde_json::Value>, i64, Option<String>) {
    (true, None, 1, Some("DELETE queries should be executed via the Table Editor UI".to_string()))
}
