//! Table management handlers

use axum::{
    extract::{Path, Query, State},
    http::StatusCode,
    response::{IntoResponse, Json},
};
use serde::{Deserialize, Serialize};

use super::AdminState;
use super::auth::ApiResponse;

#[derive(Debug, Serialize, Deserialize)]
pub struct TableInfo {
    pub name: String,
    pub r#type: String,
    pub row_count: i64,
    pub size_bytes: i64,
    pub created_at: String,
}

#[derive(Debug, Deserialize)]
pub struct UpdateSchemaRequest {
    pub columns: Vec<ColumnDef>,
}

/// Update table schema
pub async fn update_table_schema(
    State(state): State<AdminState>,
    Path(table_name): Path<String>,
    Json(req): Json<UpdateSchemaRequest>,
) -> impl IntoResponse {
    let schema_key = format!("__table_schema__:{}", table_name);
    match serde_json::to_vec(&req.columns) {
        Ok(buf) => {
            if let Err(e) = persist_data(&state, schema_key, buf) {
                return (
                    StatusCode::INTERNAL_SERVER_ERROR,
                    Json(serde_json::json!({
                        "success": false,
                        "error": format!("Failed to persist schema: {}", e)
                    })),
                );
            }
            (
                StatusCode::OK,
                Json(serde_json::json!({
                    "success": true,
                    "message": "Schema updated"
                })),
            )
        }
        Err(e) => (
            StatusCode::BAD_REQUEST,
            Json(serde_json::json!({
                "success": false,
                "error": format!("Invalid schema: {}", e)
            })),
        ),
    }
}

/// Get table schema (list of columns)
pub async fn get_table_schema(
    State(state): State<AdminState>,
    Path(table_name): Path<String>,
) -> impl IntoResponse {
    let schema_key = format!("__table_schema__:{}", table_name);
    match state.storage.get_string(&schema_key) {
        Ok(data) => {
            if let Ok(columns) = serde_json::from_slice::<Vec<ColumnDef>>(&data) {
                (
                    StatusCode::OK,
                    Json(serde_json::json!({
                        "success": true,
                        "columns": columns
                    })),
                )
            } else {
                (
                    StatusCode::OK,
                    Json(serde_json::json!({
                        "success": true,
                        "columns": Vec::<ColumnDef>::new()
                    })),
                )
            }
        }
        Err(_) => (
            StatusCode::OK,
            Json(serde_json::json!({
                "success": true,
                "columns": Vec::<ColumnDef>::new()
            })),
        ),
    }
}

#[derive(Debug, Deserialize)]
pub struct ListTablesQuery {
    pub r#type: Option<String>,
}

#[derive(Debug, Deserialize)]
pub struct GetTableDataQuery {
    pub limit: Option<usize>,
    pub offset: Option<usize>,
    pub r#type: Option<String>,
}

#[derive(Debug, Serialize)]
pub struct TableDataResponse {
    pub data: Vec<serde_json::Value>,
    pub total_count: i64,
    pub limit: usize,
    pub offset: usize,
    pub table: String,
    pub r#type: String,
}

#[derive(Debug, Deserialize)]
pub struct CreateTableRequest {
    pub name: String,
    pub r#type: String,
    pub columns: Vec<ColumnDef>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ColumnDef {
    pub name: String,
    pub r#type: String,
    pub required: bool,
}

pub async fn list_tables(
    State(state): State<AdminState>,
    Query(_params): Query<ListTablesQuery>,
) -> impl IntoResponse {
    // Get table metadata from storage
    let mut tables = Vec::new();
    
    // Try to get table list from metadata key
    match state.storage.get_string("__tables__") {
        Ok(data) => {
            if let Ok(table_list) = serde_json::from_slice::<Vec<TableInfo>>(&data) {
                tables = table_list;
            }
        }
        Err(_) => {
            // No tables yet, return empty list
        }
    }
    
    // Do not inject demo tables; return actual stored tables only
    
    (
        StatusCode::OK,
        Json(serde_json::json!({
            "tables": tables,
            "total": tables.len(),
        })),
    )
}

pub async fn get_table_data(
    State(state): State<AdminState>,
    Path(table): Path<String>,
    Query(params): Query<GetTableDataQuery>,
) -> impl IntoResponse {
    let limit = params.limit.unwrap_or(50);
    let offset = params.offset.unwrap_or(0);
    let table_type = params.r#type.unwrap_or_else(|| "table".to_string());
    
    // Get table data from storage
    let table_key = format!("__table_data__:{}", table);
    let mut all_rows = Vec::new();
    
    match state.storage.get_string(&table_key) {
        Ok(data) => {
            if let Ok(rows) = serde_json::from_slice::<Vec<serde_json::Value>>(&data) {
                all_rows = rows;
            }
        }
        Err(_) => {
            // No data yet, return empty
        }
    }
    
    // Apply pagination
    let total_count = all_rows.len() as i64;
    let paginated_data: Vec<serde_json::Value> = all_rows
        .into_iter()
        .skip(offset)
        .take(limit)
        .collect();
    
    let response = TableDataResponse {
        data: paginated_data,
        total_count,
        limit,
        offset,
        table,
        r#type: table_type,
    };
    
    (StatusCode::OK, Json(response))
}

pub async fn create_row(
    State(state): State<AdminState>,
    Path(table): Path<String>,
    Json(data): Json<serde_json::Value>,
) -> impl IntoResponse {
    let table_key = format!("__table_data__:{}", table);
    
    // Get existing rows
    let mut rows = match state.storage.get_string(&table_key) {
        Ok(data) => serde_json::from_slice::<Vec<serde_json::Value>>(&data).unwrap_or_default(),
        Err(_) => Vec::new(),
    };
    
    // Add new row
    rows.push(data);
    
    // Save back to persistent storage
    if let Ok(serialized) = serde_json::to_vec(&rows) {
        if let Ok(mut persistent) = state.persistent.lock() {
            if let Err(e) = persistent.put(table_key.clone(), serialized) {
                return (
                    StatusCode::INTERNAL_SERVER_ERROR,
                    Json(serde_json::json!({
                        "success": false,
                        "error": format!("Failed to save row: {}", e),
                    })),
                );
            }
        } else {
            return (
                StatusCode::INTERNAL_SERVER_ERROR,
                Json(serde_json::json!({
                    "success": false,
                    "error": "Failed to acquire storage lock",
                })),
            );
        }
    }
    
    // Update table row count
    update_table_row_count(&state, &table, rows.len() as i64).await;
    
    (
        StatusCode::OK,
        Json(serde_json::json!({
            "success": true,
            "message": format!("Row created in table '{}'", table),
            "rows_affected": 1,
        })),
    )
}

pub async fn get_row(
    State(state): State<AdminState>,
    Path((table, id)): Path<(String, String)>,
) -> impl IntoResponse {
    let table_key = format!("__table_data__:{}", table);
    // Load rows
    let rows = match state.storage.get_string(&table_key) {
        Ok(data) => serde_json::from_slice::<Vec<serde_json::Value>>(&data).unwrap_or_default(),
        Err(_) => Vec::new(),
    };

    // Find by id (string compare)
    if let Some(row) = rows.into_iter().find(|r| r.get("id").map(|v| v.to_string().trim_matches('"').to_string()) == Some(id.clone())) {
        return (
            StatusCode::OK,
            Json(serde_json::json!({
                "success": true,
                "row": row
            })),
        );
    }

    (
        StatusCode::NOT_FOUND,
        Json(serde_json::json!({
            "success": false,
            "error": format!("Row with id {} not found", id),
        })),
    )
}

pub async fn update_row(
    State(state): State<AdminState>,
    Path((table, id)): Path<(String, String)>,
    Json(data): Json<serde_json::Value>,
) -> impl IntoResponse {
    let table_key = format!("__table_data__:{}", table);
    
    // Get existing rows
    let mut rows = match state.storage.get_string(&table_key) {
        Ok(data) => serde_json::from_slice::<Vec<serde_json::Value>>(&data).unwrap_or_default(),
        Err(_) => Vec::new(),
    };
    
    // Find and update the row by id
    let mut found = false;
    for row in rows.iter_mut() {
        if let Some(row_id) = row.get("id") {
            if row_id.to_string().trim_matches('\"') == id {
                *row = data.clone();
                found = true;
                break;
            }
        }
    }
    
    if !found {
        return (
            StatusCode::NOT_FOUND,
            Json(serde_json::json!({
                "success": false,
                "error": format!("Row with id {} not found", id),
            })),
        );
    }
    
    // Save back to persistent storage
    if let Ok(serialized) = serde_json::to_vec(&rows) {
        if let Err(e) = persist_data(&state, table_key, serialized) {
            return (
                StatusCode::INTERNAL_SERVER_ERROR,
                Json(serde_json::json!({
                    "success": false,
                    "error": format!("Failed to update row: {}", e),
                })),
            );
        }
    }
    
    (
        StatusCode::OK,
        Json(serde_json::json!({
            "success": true,
            "message": format!("Row {} updated in table '{}'", id, table),
            "rows_affected": 1,
        })),
    )
}

pub async fn delete_row(
    State(state): State<AdminState>,
    Path((table, id)): Path<(String, String)>,
) -> impl IntoResponse {
    let table_key = format!("__table_data__:{}", table);
    
    // Get existing rows
    let mut rows = match state.storage.get_string(&table_key) {
        Ok(data) => serde_json::from_slice::<Vec<serde_json::Value>>(&data).unwrap_or_default(),
        Err(_) => Vec::new(),
    };
    
    // Find and remove the row by id
    let original_len = rows.len();
    rows.retain(|row| {
        if let Some(row_id) = row.get("id") {
            row_id.to_string().trim_matches('\"') != id
        } else {
            true
        }
    });
    
    if rows.len() == original_len {
        return (
            StatusCode::NOT_FOUND,
            Json(serde_json::json!({
                "success": false,
                "error": format!("Row with id {} not found", id),
            })),
        );
    }
    
    // Save back to persistent storage
    if let Ok(serialized) = serde_json::to_vec(&rows) {
        if let Err(e) = persist_data(&state, table_key, serialized) {
            return (
                StatusCode::INTERNAL_SERVER_ERROR,
                Json(serde_json::json!({
                    "success": false,
                    "error": format!("Failed to delete row: {}", e),
                })),
            );
        }
    }
    
    // Update table row count
    update_table_row_count(&state, &table, rows.len() as i64).await;
    
    (
        StatusCode::OK,
        Json(serde_json::json!({
            "success": true,
            "message": format!("Row {} deleted from table '{}'", id, table),
            "rows_affected": 1,
        })),
    )
}

pub async fn create_table(
    State(state): State<AdminState>,
    Json(request): Json<CreateTableRequest>,
) -> impl IntoResponse {
    // Get existing tables
    let mut tables = match state.storage.get_string("__tables__") {
        Ok(data) => serde_json::from_slice::<Vec<TableInfo>>(&data).unwrap_or_default(),
        Err(_) => Vec::new(),
    };
    
    // Check if table already exists
    if tables.iter().any(|t| t.name == request.name) {
        return (
            StatusCode::CONFLICT,
            Json(serde_json::json!({
                "success": false,
                "error": format!("Table '{}' already exists", request.name),
            })),
        );
    }
    
    // Add new table
    let new_table = TableInfo {
        name: request.name.clone(),
        r#type: request.r#type,
        row_count: 0,
        size_bytes: 0,
        created_at: chrono::Utc::now().to_rfc3339(),
    };
    tables.push(new_table);
    
    // Save table list
    if let Ok(serialized) = serde_json::to_vec(&tables) {
        if let Err(e) = persist_data(&state, "__tables__".to_string(), serialized) {
            return (
                StatusCode::INTERNAL_SERVER_ERROR,
                Json(serde_json::json!({
                    "success": false,
                    "error": format!("Failed to create table: {}", e),
                })),
            );
        }
    }
    
    // Persist schema for the table
    let schema_key = format!("__table_schema__:{}", request.name);
    if let Ok(serialized_schema) = serde_json::to_vec(&request.columns) {
        let _ = persist_data(&state, schema_key, serialized_schema);
    }

    // Initialize empty data for the table
    let table_key = format!("__table_data__:{}", request.name);
    let empty_rows: Vec<serde_json::Value> = Vec::new();
    if let Ok(serialized) = serde_json::to_vec(&empty_rows) {
        let _ = persist_data(&state, table_key, serialized);
    }
    
    (
        StatusCode::OK,
        Json(serde_json::json!({
            "success": true,
            "message": format!("Table '{}' created successfully", request.name),
        })),
    )
}

// Helper function to persist data
fn persist_data(state: &AdminState, key: String, data: Vec<u8>) -> Result<(), String> {
    if let Ok(mut persistent) = state.persistent.lock() {
        persistent.put(key, data).map_err(|e| e.to_string())
    } else {
        Err("Failed to acquire storage lock".to_string())
    }
}

// Helper function to update table row count
async fn update_table_row_count(state: &AdminState, table_name: &str, new_count: i64) {
    if let Ok(data) = state.storage.get_string("__tables__") {
        if let Ok(mut tables) = serde_json::from_slice::<Vec<TableInfo>>(&data) {
            for table in tables.iter_mut() {
                if table.name == table_name {
                    table.row_count = new_count;
                    break;
                }
            }
            if let Ok(serialized) = serde_json::to_vec(&tables) {
                let _ = persist_data(state, "__tables__".to_string(), serialized);
            }
        }
    }
}
