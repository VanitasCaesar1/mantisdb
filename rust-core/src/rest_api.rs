//! High-performance REST API for MantisDB
//!
//! This module provides a PostgREST-like REST API with automatic CRUD endpoints,
//! built on Axum for maximum performance.

use crate::pool::ConnectionPool;
use crate::error::{Error, Result};
use axum::{
    extract::{Path, Query, State},
    http::StatusCode,
    response::{IntoResponse, Response, Json},
    routing::{get, post, put, delete},
    Router,
};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::net::SocketAddr;
use std::sync::Arc;
use tower_http::cors::CorsLayer;
use tower_http::trace::TraceLayer;
use tower_http::compression::CompressionLayer;
use tracing::info;

/// REST API configuration
#[derive(Debug, Clone)]
pub struct RestApiConfig {
    /// Server bind address
    pub bind_addr: SocketAddr,
    /// Enable CORS
    pub enable_cors: bool,
    /// Enable compression
    pub enable_compression: bool,
    /// Enable request tracing
    pub enable_tracing: bool,
    /// Maximum request body size (bytes)
    pub max_body_size: usize,
    /// Request timeout (seconds)
    pub request_timeout: u64,
}

impl Default for RestApiConfig {
    fn default() -> Self {
        Self {
            bind_addr: "0.0.0.0:8080".parse().unwrap(),
            enable_cors: true,
            enable_compression: true,
            enable_tracing: true,
            max_body_size: 10 * 1024 * 1024, // 10MB
            request_timeout: 30,
        }
    }
}

/// Application state shared across handlers
#[derive(Clone)]
pub struct AppState {
    pool: Arc<ConnectionPool>,
}

/// Generic API response
#[derive(Debug, Serialize, Deserialize)]
pub struct ApiResponse<T> {
    pub success: bool,
    pub data: Option<T>,
    pub error: Option<String>,
    pub meta: Option<ResponseMeta>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ResponseMeta {
    pub count: Option<usize>,
    pub total: Option<usize>,
    pub page: Option<usize>,
    pub per_page: Option<usize>,
}

/// Key-Value operations
#[derive(Debug, Serialize, Deserialize)]
pub struct KvSetRequest {
    pub value: Vec<u8>,
    pub ttl: Option<u64>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct KvGetResponse {
    pub key: String,
    pub value: Vec<u8>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct KvBatchRequest {
    pub operations: Vec<KvOperation>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct KvOperation {
    pub op: String, // "get", "set", "delete"
    pub key: String,
    pub value: Option<Vec<u8>>,
    pub ttl: Option<u64>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct KvBatchResponse {
    pub results: Vec<KvOperationResult>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct KvOperationResult {
    pub key: String,
    pub success: bool,
    pub value: Option<Vec<u8>>,
    pub error: Option<String>,
}

/// Query parameters for listing
#[derive(Debug, Deserialize)]
pub struct ListParams {
    pub limit: Option<usize>,
    pub offset: Option<usize>,
    pub prefix: Option<String>,
}

/// Error response
impl IntoResponse for Error {
    fn into_response(self) -> Response {
        let (status, message) = match self {
            Error::NotFound => (StatusCode::NOT_FOUND, "Resource not found"),
            Error::PoolExhausted => (StatusCode::SERVICE_UNAVAILABLE, "Connection pool exhausted"),
            Error::PoolClosed => (StatusCode::SERVICE_UNAVAILABLE, "Connection pool closed"),
            Error::Timeout => (StatusCode::REQUEST_TIMEOUT, "Request timeout"),
            _ => (StatusCode::INTERNAL_SERVER_ERROR, "Internal server error"),
        };

        let body = Json(ApiResponse::<()> {
            success: false,
            data: None,
            error: Some(message.to_string()),
            meta: None,
        });

        (status, body).into_response()
    }
}

/// REST API server
pub struct RestApiServer {
    config: RestApiConfig,
    pool: Arc<ConnectionPool>,
}

impl RestApiServer {
    /// Create a new REST API server
    pub fn new(config: RestApiConfig, pool: Arc<ConnectionPool>) -> Self {
        Self { config, pool }
    }

    /// Build the router with all endpoints
    fn build_router(&self) -> Router {
        let state = AppState {
            pool: self.pool.clone(),
        };

        let mut app = Router::new()
            // Health and stats
            .route("/health", get(health_handler))
            .route("/stats", get(stats_handler))
            
            // Key-Value API (versioned and unversioned aliases)
            .route("/api/v1/kv/:key", get(kv_get_handler))
            .route("/api/v1/kv/:key", put(kv_set_handler))
            .route("/api/v1/kv/:key", delete(kv_delete_handler))
            .route("/api/v1/kv", post(kv_batch_handler))
            .route("/api/v1/kv", get(kv_list_handler))
            .route("/api/kv/:key", get(kv_get_handler))
            .route("/api/kv/:key", put(kv_set_handler))
            .route("/api/kv/:key", delete(kv_delete_handler))
            .route("/api/kv", post(kv_batch_handler))
            .route("/api/kv", get(kv_list_handler))
            
            // Table API (PostgREST-like)
            .route("/api/v1/tables/:table", get(table_query_handler))
            .route("/api/v1/tables/:table", post(table_insert_handler))
            .route("/api/v1/tables/:table/:id", get(table_get_handler))
            .route("/api/v1/tables/:table/:id", put(table_update_handler))
            .route("/api/v1/tables/:table/:id", delete(table_delete_handler))
            .route("/api/tables/:table", get(table_query_handler))
            .route("/api/tables/:table", post(table_insert_handler))
            .route("/api/tables/:table/:id", get(table_get_handler))
            .route("/api/tables/:table/:id", put(table_update_handler))
            .route("/api/tables/:table/:id", delete(table_delete_handler))
            
            .with_state(state);

        // Add middleware layers
        if self.config.enable_compression {
            app = app.layer(CompressionLayer::new());
        }

        if self.config.enable_cors {
            app = app.layer(CorsLayer::permissive());
        }

        if self.config.enable_tracing {
            app = app.layer(TraceLayer::new_for_http());
        }

        app
    }

    /// Start the REST API server
    pub async fn start(self) -> Result<()> {
        let app = self.build_router();
        let addr = self.config.bind_addr;

        info!("Starting REST API server on {}", addr);

        let listener = tokio::net::TcpListener::bind(addr)
            .await
            .map_err(|e| Error::Io(e.to_string()))?;

        axum::serve(listener, app)
            .await
            .map_err(|e| Error::Io(e.to_string()))?;

        Ok(())
    }
}

// ============================================================================
// Handler Functions
// ============================================================================

/// Health check handler
async fn health_handler() -> impl IntoResponse {
    Json(ApiResponse {
        success: true,
        data: Some(serde_json::json!({
            "status": "healthy",
            "timestamp": chrono::Utc::now().to_rfc3339(),
        })),
        error: None,
        meta: None,
    })
}

/// Stats handler
async fn stats_handler(State(state): State<AppState>) -> impl IntoResponse {
    let stats = state.pool.stats();
    
    Json(ApiResponse {
        success: true,
        data: Some(serde_json::json!({
            "pool": {
                "total_connections": stats.total_connections,
                "active_connections": stats.active_connections,
                "idle_connections": stats.idle_connections,
                "wait_count": stats.wait_count,
                "avg_wait_time_ms": if stats.wait_count > 0 {
                    stats.total_wait_time_ms / stats.wait_count
                } else {
                    0
                },
                "connections_created": stats.connections_created,
                "connections_closed": stats.connections_closed,
                "health_check_failures": stats.health_check_failures,
            }
        })),
        error: None,
        meta: None,
    })
}

/// KV Get handler
async fn kv_get_handler(
    State(state): State<AppState>,
    Path(key): Path<String>,
) -> Result<impl IntoResponse> {
    let conn = state.pool.acquire().await?;
    let value = conn.storage().get(key.as_bytes())?;

    Ok(Json(ApiResponse {
        success: true,
        data: Some(KvGetResponse {
            key,
            value: value.to_vec(),
        }),
        error: None,
        meta: None,
    }))
}

/// KV Set handler
async fn kv_set_handler(
    State(state): State<AppState>,
    Path(key): Path<String>,
    Json(req): Json<KvSetRequest>,
) -> Result<impl IntoResponse> {
    let conn = state.pool.acquire().await?;
    if let Some(ttl) = req.ttl {
        conn.storage().put_with_ttl(key.clone(), req.value.clone(), ttl)?;
    } else {
        conn.storage().put(key.as_bytes(), &req.value)?;
    }

    Ok(Json(ApiResponse {
        success: true,
        data: Some(serde_json::json!({
            "key": key,
            "message": "Value set successfully"
        })),
        error: None,
        meta: None,
    }))
}

/// KV Delete handler
async fn kv_delete_handler(
    State(state): State<AppState>,
    Path(key): Path<String>,
) -> Result<impl IntoResponse> {
    let conn = state.pool.acquire().await?;
    conn.storage().delete(key.as_bytes())?;

    Ok(Json(ApiResponse {
        success: true,
        data: Some(serde_json::json!({
            "key": key,
            "message": "Value deleted successfully"
        })),
        error: None,
        meta: None,
    }))
}

/// KV Batch handler
async fn kv_batch_handler(
    State(state): State<AppState>,
    Json(req): Json<KvBatchRequest>,
) -> Result<impl IntoResponse> {
    let conn = state.pool.acquire().await?;
    let mut results = Vec::new();

    for op in req.operations {
        let result = match op.op.as_str() {
            "get" => {
                match conn.storage().get(op.key.as_bytes()) {
                    Ok(value) => KvOperationResult {
                        key: op.key,
                        success: true,
                        value: Some(value.to_vec()),
                        error: None,
                    },
                    Err(e) => KvOperationResult {
                        key: op.key,
                        success: false,
                        value: None,
                        error: Some(e.to_string()),
                    },
                }
            }
            "set" => {
                if let Some(value) = op.value {
                    match conn.storage().put(op.key.as_bytes(), &value) {
                        Ok(_) => KvOperationResult {
                            key: op.key,
                            success: true,
                            value: None,
                            error: None,
                        },
                        Err(e) => KvOperationResult {
                            key: op.key,
                            success: false,
                            value: None,
                            error: Some(e.to_string()),
                        },
                    }
                } else {
                    KvOperationResult {
                        key: op.key,
                        success: false,
                        value: None,
                        error: Some("Value required for set operation".to_string()),
                    }
                }
            }
            "delete" => {
                match conn.storage().delete(op.key.as_bytes()) {
                    Ok(_) => KvOperationResult {
                        key: op.key,
                        success: true,
                        value: None,
                        error: None,
                    },
                    Err(e) => KvOperationResult {
                        key: op.key,
                        success: false,
                        value: None,
                        error: Some(e.to_string()),
                    },
                }
            }
            _ => KvOperationResult {
                key: op.key,
                success: false,
                value: None,
                error: Some(format!("Unknown operation: {}", op.op)),
            },
        };
        results.push(result);
    }

    Ok(Json(ApiResponse {
        success: true,
        data: Some(KvBatchResponse { results }),
        error: None,
        meta: None,
    }))
}

/// KV List handler with pagination and prefix filtering
async fn kv_list_handler(
    State(state): State<AppState>,
    Query(params): Query<ListParams>,
) -> Result<impl IntoResponse> {
    let conn = state.pool.acquire().await?;
    let storage = conn.storage();
    
    // Get prefix filter or empty string for all keys
    let prefix = params.prefix.as_deref().unwrap_or("");
    
    // Scan with prefix
    let all_results = storage.scan_prefix(prefix);
    
    let total = all_results.len();
    let limit = params.limit.unwrap_or(100).min(1000); // Cap at 1000
    let offset = params.offset.unwrap_or(0);
    
    // Paginate results
    let keys: Vec<String> = all_results
        .into_iter()
        .skip(offset)
        .take(limit)
        .map(|(key, _value)| key)
        .collect();
    
    let count = keys.len();

    Ok(Json(ApiResponse {
        success: true,
        data: Some(keys),
        error: None,
        meta: Some(ResponseMeta {
            count: Some(count),
            total: Some(total),
            page: Some(offset / limit),
            per_page: Some(limit),
        }),
    }))
}

/// Table query handler (PostgREST-like)
async fn table_query_handler(
    State(state): State<AppState>,
    Path(table): Path<String>,
    Query(params): Query<HashMap<String, String>>,
) -> Result<impl IntoResponse> {
    let conn = state.pool.acquire().await?;
    let storage = conn.storage();

    let limit = params.get("limit").and_then(|s| s.parse::<usize>().ok()).unwrap_or(100);
    let offset = params.get("offset").and_then(|s| s.parse::<usize>().ok()).unwrap_or(0);

    let prefix = format!("table/{}/", table);
    let mut items = storage.scan_prefix(&prefix);
    items.sort_by(|a, b| a.0.cmp(&b.0));

    let total = items.len();
    let rows: Vec<serde_json::Value> = items
        .into_iter()
        .skip(offset)
        .take(limit)
        .filter_map(|(_k, v)| serde_json::from_slice::<serde_json::Value>(&v).ok())
        .collect();

    Ok(Json(ApiResponse {
        success: true,
        data: Some(serde_json::json!({
            "table": table,
            "rows": rows,
            "total": total,
            "limit": limit,
            "offset": offset
        })),
        error: None,
        meta: Some(ResponseMeta { count: Some(rows.len()), total: Some(total), page: Some(offset / limit.max(1)), per_page: Some(limit) }),
    }))
}

/// Table insert handler
async fn table_insert_handler(
    State(state): State<AppState>,
    Path(table): Path<String>,
    Json(data): Json<serde_json::Value>,
) -> Result<impl IntoResponse> {
    let conn = state.pool.acquire().await?;
    let id = data.get("id").and_then(|v| v.as_str()).map(|s| s.to_string()).unwrap_or_else(|| uuid::Uuid::new_v4().to_string());
    let key = format!("table/{}/{}", table, id);
let bytes = serde_json::to_vec(&data).map_err(|e| Error::SerializationError(format!("Invalid JSON: {}", e)))?;
    conn.storage().put(key.as_bytes(), &bytes)?;

    Ok(Json(ApiResponse {
        success: true,
        data: Some(serde_json::json!({
            "table": table,
            "id": id,
            "inserted": 1
        })),
        error: None,
        meta: None,
    }))
}

/// Table get handler
async fn table_get_handler(
    State(state): State<AppState>,
    Path((table, id)): Path<(String, String)>,
) -> Result<impl IntoResponse> {
    let conn = state.pool.acquire().await?;
    let key = format!("table/{}/{}", table, id);
    let bytes = conn.storage().get(key.as_bytes())?;
let row: serde_json::Value = serde_json::from_slice(&bytes).map_err(|e| Error::SerializationError(format!("Invalid stored JSON: {}", e)))?;

    Ok(Json(ApiResponse {
        success: true,
        data: Some(serde_json::json!({
            "table": table,
            "id": id,
            "row": row
        })),
        error: None,
        meta: None,
    }))
}

/// Table update handler
async fn table_update_handler(
    State(state): State<AppState>,
    Path((table, id)): Path<(String, String)>,
    Json(data): Json<serde_json::Value>,
) -> Result<impl IntoResponse> {
    let conn = state.pool.acquire().await?;
    let key = format!("table/{}/{}", table, id);
let bytes = serde_json::to_vec(&data).map_err(|e| Error::SerializationError(format!("Invalid JSON: {}", e)))?;
    conn.storage().put(key.as_bytes(), &bytes)?;

    Ok(Json(ApiResponse {
        success: true,
        data: Some(serde_json::json!({
            "table": table,
            "id": id,
            "updated": true
        })),
        error: None,
        meta: None,
    }))
}

/// Table delete handler
async fn table_delete_handler(
    State(state): State<AppState>,
    Path((table, id)): Path<(String, String)>,
) -> Result<impl IntoResponse> {
    let conn = state.pool.acquire().await?;
    let key = format!("table/{}/{}", table, id);
    conn.storage().delete(key.as_bytes())?;

    Ok(Json(ApiResponse {
        success: true,
        data: Some(serde_json::json!({
            "table": table,
            "id": id,
            "deleted": true
        })),
        error: None,
        meta: None,
    }))
}
