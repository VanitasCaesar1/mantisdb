//! Columnar store API handlers (Cassandra/ScyllaDB-style)

use axum::{
    extract::{Path, State},
    http::StatusCode,
    response::{IntoResponse, Json},
};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::{Arc, RwLock};

use super::AdminState;

lazy_static::lazy_static! {
    static ref COLUMNAR_STORE: Arc<RwLock<HashMap<String, Table>>> = 
        Arc::new(RwLock::new(HashMap::new()));
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Table {
    pub name: String,
    pub columns: Vec<Column>,
    pub rows: Vec<Row>,
    pub indexes: Vec<Index>,
    pub partitions: Vec<String>,
    pub created_at: i64,
    pub updated_at: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Column {
    pub name: String,
    pub data_type: String,
    pub nullable: bool,
    pub indexed: bool,
    pub primary_key: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Row {
    pub values: HashMap<String, serde_json::Value>,
    pub row_id: u64,
    pub version: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Index {
    pub name: String,
    pub columns: Vec<String>,
    pub unique: bool,
    pub index_type: String, // btree, hash, bloom
}

#[derive(Debug, Deserialize)]
pub struct CreateTableRequest {
    pub name: String,
    pub columns: Vec<Column>,
    pub partition_key: Option<Vec<String>>,
}

#[derive(Debug, Deserialize)]
pub struct InsertRowsRequest {
    pub rows: Vec<HashMap<String, serde_json::Value>>,
}

#[derive(Debug, Deserialize)]
pub struct QueryRequest {
    pub columns: Option<Vec<String>>,
    pub filters: Option<Vec<Filter>>,
    pub order_by: Option<Vec<OrderBy>>,
    pub limit: Option<usize>,
    pub offset: Option<usize>,
}

#[derive(Debug, Deserialize)]
pub struct Filter {
    pub column: String,
    pub operator: String, // eq, ne, gt, gte, lt, lte, in
    pub value: serde_json::Value,
}

#[derive(Debug, Deserialize)]
pub struct OrderBy {
    pub column: String,
    pub desc: bool,
}

#[derive(Debug, Deserialize)]
pub struct UpdateRequest {
    pub filters: Vec<Filter>,
    pub updates: HashMap<String, serde_json::Value>,
}

#[derive(Debug, Deserialize)]
pub struct CreateIndexRequest {
    pub name: String,
    pub columns: Vec<String>,
    pub unique: bool,
    pub index_type: Option<String>,
}

#[derive(Debug, Deserialize)]
pub struct CQLRequest {
    pub statement: String,
    pub params: Option<Vec<serde_json::Value>>,
}

/// List all tables
pub async fn list_tables(
    State(_state): State<AdminState>,
) -> impl IntoResponse {
    let store = COLUMNAR_STORE.read().unwrap();

    let tables: Vec<_> = store.values()
        .map(|t| serde_json::json!({
            "name": t.name,
            "columns": t.columns,           // return full column metadata
            "row_count": t.rows.len(),      // align with frontend expectations
            "size_bytes": 0,
            "partitions": t.partitions.len(),
        }))
        .collect();

    (StatusCode::OK, Json(serde_json::json!({
        "success": true,
        "tables": tables,
        "count": tables.len()
    })))
}

/// Create a new table
pub async fn create_table(
    State(_state): State<AdminState>,
    Json(req): Json<CreateTableRequest>,
) -> impl IntoResponse {
    let mut store = COLUMNAR_STORE.write().unwrap();
    let now = chrono::Utc::now().timestamp();
    
    if store.contains_key(&req.name) {
        return (StatusCode::CONFLICT, Json(serde_json::json!({
            "success": false,
            "error": "Table already exists"
        })));
    }
    
    let table = Table {
        name: req.name.clone(),
        columns: req.columns,
        rows: Vec::new(),
        indexes: Vec::new(),
        partitions: req.partition_key.unwrap_or_default(),
        created_at: now,
        updated_at: now,
    };
    
    store.insert(req.name.clone(), table.clone());
    
    (StatusCode::CREATED, Json(serde_json::json!({
        "success": true,
        "table": table
    })))
}

/// Get table metadata
pub async fn get_table(
    State(_state): State<AdminState>,
    Path(table_name): Path<String>,
) -> impl IntoResponse {
    let store = COLUMNAR_STORE.read().unwrap();
    
    match store.get(&table_name) {
        Some(table) => (StatusCode::OK, Json(serde_json::json!({
            "success": true,
            "table": table
        }))),
        None => (StatusCode::NOT_FOUND, Json(serde_json::json!({
            "success": false,
            "error": "Table not found"
        }))),
    }
}

/// Drop a table
pub async fn drop_table(
    State(_state): State<AdminState>,
    Path(table_name): Path<String>,
) -> impl IntoResponse {
    let mut store = COLUMNAR_STORE.write().unwrap();
    
    if store.remove(&table_name).is_some() {
        (StatusCode::OK, Json(serde_json::json!({
            "success": true,
            "message": "Table dropped successfully"
        })))
    } else {
        (StatusCode::NOT_FOUND, Json(serde_json::json!({
            "success": false,
            "error": "Table not found"
        })))
    }
}

/// Insert rows into a table
pub async fn insert_rows(
    State(_state): State<AdminState>,
    Path(table_name): Path<String>,
    Json(req): Json<InsertRowsRequest>,
) -> impl IntoResponse {
    let mut store = COLUMNAR_STORE.write().unwrap();
    
    match store.get_mut(&table_name) {
        Some(table) => {
            let mut row_id = table.rows.len() as u64;
            let rows_count = req.rows.len();
            
            for row_data in req.rows {
                row_id += 1;
                let row = Row {
                    values: row_data,
                    row_id,
                    version: 1,
                };
                table.rows.push(row);
            }
            
            table.updated_at = chrono::Utc::now().timestamp();
            
            (StatusCode::CREATED, Json(serde_json::json!({
                "success": true,
                "rows_inserted": rows_count,
                "message": "Rows inserted successfully"
            })))
        }
        None => (StatusCode::NOT_FOUND, Json(serde_json::json!({
            "success": false,
            "error": "Table not found"
        }))),
    }
}

/// Query rows from a table
pub async fn query_rows(
    State(_state): State<AdminState>,
    Path(table_name): Path<String>,
    Json(req): Json<QueryRequest>,
) -> impl IntoResponse {
    let store = COLUMNAR_STORE.read().unwrap();
    
    match store.get(&table_name) {
        Some(table) => {
            let mut rows = table.rows.clone();
            
            // Apply filters
            if let Some(filters) = req.filters {
                for filter in filters {
                    rows.retain(|row| {
                        if let Some(value) = row.values.get(&filter.column) {
                            apply_filter(value, &filter.operator, &filter.value)
                        } else {
                            false
                        }
                    });
                }
            }
            
            // Apply sorting
            if let Some(order_by) = req.order_by {
                for order in order_by.iter().rev() {
                    rows.sort_by(|a, b| {
                        let a_val = a.values.get(&order.column);
                        let b_val = b.values.get(&order.column);
                        
                        let cmp = compare_json_values(a_val, b_val);
                        if order.desc { cmp.reverse() } else { cmp }
                    });
                }
            }
            
            let total = rows.len();
            let limit = req.limit.unwrap_or(100).min(10000);
            let offset = req.offset.unwrap_or(0);
            
            // Apply pagination
            let paginated: Vec<_> = rows.into_iter()
                .skip(offset)
                .take(limit)
                .collect();
            
            // Project columns if specified
            let results = if let Some(columns) = req.columns {
                paginated.into_iter()
                    .map(|row| {
                        let mut projected = HashMap::new();
                        for col in &columns {
                            if let Some(val) = row.values.get(col) {
                                projected.insert(col.clone(), val.clone());
                            }
                        }
                        projected
                    })
                    .collect::<Vec<_>>()
            } else {
                paginated.into_iter().map(|r| r.values).collect()
            };
            
            (StatusCode::OK, Json(serde_json::json!({
                "success": true,
                "rows": results,
                "total": total,
                "limit": limit,
                "offset": offset,
                "has_more": total > offset + limit
            })))
        }
        None => (StatusCode::NOT_FOUND, Json(serde_json::json!({
            "success": false,
            "error": "Table not found"
        }))),
    }
}

/// Update rows in a table
pub async fn update_rows(
    State(_state): State<AdminState>,
    Path(table_name): Path<String>,
    Json(req): Json<UpdateRequest>,
) -> impl IntoResponse {
    let mut store = COLUMNAR_STORE.write().unwrap();
    
    match store.get_mut(&table_name) {
        Some(table) => {
            let mut rows_affected = 0;
            
            for row in &mut table.rows {
                let mut matches = true;
                
                // Check if row matches all filters
                for filter in &req.filters {
                    if let Some(value) = row.values.get(&filter.column) {
                        if !apply_filter(value, &filter.operator, &filter.value) {
                            matches = false;
                            break;
                        }
                    } else {
                        matches = false;
                        break;
                    }
                }
                
                // Update matching rows
                if matches {
                    for (col, val) in &req.updates {
                        row.values.insert(col.clone(), val.clone());
                    }
                    row.version += 1;
                    rows_affected += 1;
                }
            }
            
            table.updated_at = chrono::Utc::now().timestamp();
            
            (StatusCode::OK, Json(serde_json::json!({
                "success": true,
                "rows_affected": rows_affected,
                "message": "Rows updated successfully"
            })))
        }
        None => (StatusCode::NOT_FOUND, Json(serde_json::json!({
            "success": false,
            "error": "Table not found"
        }))),
    }
}

#[derive(Debug, Deserialize)]
pub struct DeleteRequest {
    #[serde(rename = "where")]
    pub filters: Option<HashMap<String, serde_json::Value>>,
}

/// Delete rows from a table
pub async fn delete_rows(
    State(_state): State<AdminState>,
    Path(table_name): Path<String>,
    Json(req): Json<DeleteRequest>,
) -> impl IntoResponse {
    let mut store = COLUMNAR_STORE.write().unwrap();
    
    match store.get_mut(&table_name) {
        Some(table) => {
            let initial_count = table.rows.len();
            
            if let Some(filters) = req.filters {
                table.rows.retain(|row| {
                    // Check if row matches all filter conditions
                    for (col, expected_val) in &filters {
                        if let Some(actual_val) = row.values.get(col) {
                            if actual_val != expected_val {
                                return true; // Keep this row (doesn't match)
                            }
                        } else {
                            return true; // Keep this row (column not found)
                        }
                    }
                    false // Delete this row (matches all conditions)
                });
            } else {
                // No filters provided - don't delete anything for safety
                return (StatusCode::BAD_REQUEST, Json(serde_json::json!({
                    "success": false,
                    "error": "Delete requires filter conditions"
                })));
            }
            
            let rows_affected = initial_count - table.rows.len();
            table.updated_at = chrono::Utc::now().timestamp();
            
            (StatusCode::OK, Json(serde_json::json!({
                "success": true,
                "rows_affected": rows_affected,
                "message": "Rows deleted successfully"
            })))
        }
        None => (StatusCode::NOT_FOUND, Json(serde_json::json!({
            "success": false,
            "error": "Table not found"
        }))),
    }
}

/// Create an index on a table
pub async fn create_index(
    State(_state): State<AdminState>,
    Path(table_name): Path<String>,
    Json(req): Json<CreateIndexRequest>,
) -> impl IntoResponse {
    let mut store = COLUMNAR_STORE.write().unwrap();
    
    match store.get_mut(&table_name) {
        Some(table) => {
            let index = Index {
                name: req.name,
                columns: req.columns,
                unique: req.unique,
                index_type: req.index_type.unwrap_or_else(|| "btree".to_string()),
            };
            
            table.indexes.push(index.clone());
            table.updated_at = chrono::Utc::now().timestamp();
            
            (StatusCode::CREATED, Json(serde_json::json!({
                "success": true,
                "index": index
            })))
        }
        None => (StatusCode::NOT_FOUND, Json(serde_json::json!({
            "success": false,
            "error": "Table not found"
        }))),
    }
}

/// Execute CQL statement (Cassandra Query Language)
pub async fn execute_cql(
    State(_state): State<AdminState>,
    Json(req): Json<CQLRequest>,
) -> impl IntoResponse {
    // Very basic CQL handling: SELECT * FROM <table> [LIMIT n] [OFFSET m]
    let stmt = req.statement.trim();
    let upper = stmt.to_uppercase();

    if upper.starts_with("SELECT") {
        // parse: SELECT * FROM table [LIMIT n] [OFFSET m]
        let mut table_name: Option<String> = None;
        let mut limit: usize = 100;
        let mut offset: usize = 0;

        // Tokenize by spaces (very naive)
        let tokens: Vec<&str> = stmt.split_whitespace().collect();
        for i in 0..tokens.len() {
            if tokens[i].eq_ignore_ascii_case("FROM") && i + 1 < tokens.len() {
                table_name = Some(tokens[i + 1].trim_matches(';').to_string());
            }
            if tokens[i].eq_ignore_ascii_case("LIMIT") && i + 1 < tokens.len() {
                if let Ok(v) = tokens[i + 1].trim_matches(';').parse::<usize>() {
                    limit = v;
                }
            }
            if tokens[i].eq_ignore_ascii_case("OFFSET") && i + 1 < tokens.len() {
                if let Ok(v) = tokens[i + 1].trim_matches(';').parse::<usize>() {
                    offset = v;
                }
            }
        }

        if let Some(name) = table_name {
            let store = COLUMNAR_STORE.read().unwrap();
            if let Some(table) = store.get(&name) {
                let total = table.rows.len();
                let rows: Vec<_> = table.rows.iter()
                    .skip(offset)
                    .take(limit)
                    .map(|r| r.values.clone())
                    .collect();

                return (
                    StatusCode::OK,
                    Json(serde_json::json!({
                        "success": true,
                        "rows": rows,
                        "total": total,
                        "limit": limit,
                        "offset": offset
                    }))
                );
            } else {
                return (
                    StatusCode::NOT_FOUND,
                    Json(serde_json::json!({
                        "success": false,
                        "error": "Table not found"
                    }))
                );
            }
        }

        return (
            StatusCode::BAD_REQUEST,
            Json(serde_json::json!({
                "success": false,
                "error": "Invalid SELECT syntax. Expected: SELECT * FROM <table> [LIMIT n] [OFFSET m]"
            }))
        );
    }

    (
        StatusCode::BAD_REQUEST,
        Json(serde_json::json!({
            "success": false,
            "error": "Unsupported CQL statement"
        }))
    )
}

/// Get table statistics
pub async fn get_table_stats(
    State(_state): State<AdminState>,
    Path(table_name): Path<String>,
) -> impl IntoResponse {
    let store = COLUMNAR_STORE.read().unwrap();
    
    match store.get(&table_name) {
        Some(table) => {
            let stats = serde_json::json!({
                "table_name": table.name,
                "row_count": table.rows.len(),
                "column_count": table.columns.len(),
                "index_count": table.indexes.len(),
                "partition_count": table.partitions.len(),
                "created_at": table.created_at,
                "updated_at": table.updated_at
            });
            
            (StatusCode::OK, Json(serde_json::json!({
                "success": true,
                "stats": stats
            })))
        }
        None => (StatusCode::NOT_FOUND, Json(serde_json::json!({
            "success": false,
            "error": "Table not found"
        }))),
    }
}

// Helper functions

fn apply_filter(value: &serde_json::Value, operator: &str, expected: &serde_json::Value) -> bool {
    match operator {
        "eq" => value == expected,
        "ne" => value != expected,
        "gt" => compare_json_values(Some(value), Some(expected)) == std::cmp::Ordering::Greater,
        "gte" => matches!(compare_json_values(Some(value), Some(expected)), std::cmp::Ordering::Greater | std::cmp::Ordering::Equal),
        "lt" => compare_json_values(Some(value), Some(expected)) == std::cmp::Ordering::Less,
        "lte" => matches!(compare_json_values(Some(value), Some(expected)), std::cmp::Ordering::Less | std::cmp::Ordering::Equal),
        "in" => {
            if let Some(arr) = expected.as_array() {
                arr.contains(value)
            } else {
                false
            }
        }
        _ => false,
    }
}

fn compare_json_values(a: Option<&serde_json::Value>, b: Option<&serde_json::Value>) -> std::cmp::Ordering {
    use serde_json::Value;
    match (a, b) {
        (Some(Value::Number(a)), Some(Value::Number(b))) => {
            a.as_f64().partial_cmp(&b.as_f64()).unwrap_or(std::cmp::Ordering::Equal)
        }
        (Some(Value::String(a)), Some(Value::String(b))) => a.cmp(b),
        (Some(Value::Bool(a)), Some(Value::Bool(b))) => a.cmp(b),
        (None, None) => std::cmp::Ordering::Equal,
        (None, Some(_)) => std::cmp::Ordering::Less,
        (Some(_), None) => std::cmp::Ordering::Greater,
        _ => std::cmp::Ordering::Equal,
    }
}
