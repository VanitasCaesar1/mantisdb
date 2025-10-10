// ! Key-Value API handlers

use axum::{
    extract::{Path, Query, State},
    http::StatusCode,
    response::{IntoResponse, Json},
};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::{Arc, RwLock};

use super::AdminState;

lazy_static::lazy_static! {
    static ref KV_STORE: Arc<RwLock<HashMap<String, KVEntry>>> = Arc::new(RwLock::new(HashMap::new()));
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct KVEntry {
    pub key: String,
    pub value: serde_json::Value,
    pub ttl: Option<u64>,
    pub created_at: i64,
    pub updated_at: i64,
    pub version: u64,
    pub metadata: KVMetadata,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct KVMetadata {
    pub content_type: String,
    pub tags: Vec<String>,
    pub properties: HashMap<String, String>,
}

#[derive(Debug, Deserialize)]
pub struct SetRequest {
    pub value: serde_json::Value,
    pub ttl: Option<u64>,
    pub tags: Option<Vec<String>>,
}

#[derive(Debug, Deserialize)]
pub struct QueryParams {
    pub prefix: Option<String>,
    pub limit: Option<usize>,
    pub offset: Option<usize>,
}

#[derive(Debug, Deserialize)]
pub struct BatchOperation {
    pub operation: String, // "set", "get", "delete"
    pub key: String,
    pub value: Option<serde_json::Value>,
    pub ttl: Option<u64>,
}

#[derive(Debug, Deserialize)]
pub struct BatchRequest {
    pub operations: Vec<BatchOperation>,
    pub atomic: bool,
}

/// Get a value by key
pub async fn get_key(
    State(_state): State<AdminState>,
    Path(key): Path<String>,
) -> impl IntoResponse {
    let store = KV_STORE.read().unwrap();
    
    match store.get(&key) {
        Some(entry) => {
            // Check TTL
            if let Some(ttl) = entry.ttl {
                let now = chrono::Utc::now().timestamp();
                if now > entry.created_at + ttl as i64 {
                    return (
                        StatusCode::NOT_FOUND,
                        Json(serde_json::json!({
                            "success": false,
                            "error": "Key has expired"
                        }))
                    );
                }
            }
            
            (StatusCode::OK, Json(serde_json::json!({
                "success": true,
                "data": entry
            })))
        }
        None => (
            StatusCode::NOT_FOUND,
            Json(serde_json::json!({
                "success": false,
                "error": "Key not found"
            }))
        ),
    }
}

/// Set a key-value pair
pub async fn set_key(
    State(_state): State<AdminState>,
    Path(key): Path<String>,
    Json(req): Json<SetRequest>,
) -> impl IntoResponse {
    let mut store = KV_STORE.write().unwrap();
    let now = chrono::Utc::now().timestamp();
    
    let entry = KVEntry {
        key: key.clone(),
        value: req.value,
        ttl: req.ttl,
        created_at: now,
        updated_at: now,
        version: store.get(&key).map(|e| e.version + 1).unwrap_or(1),
        metadata: KVMetadata {
            content_type: "application/json".to_string(),
            tags: req.tags.unwrap_or_default(),
            properties: HashMap::new(),
        },
    };
    
    store.insert(key.clone(), entry);
    
    (StatusCode::OK, Json(serde_json::json!({
        "success": true,
        "key": key,
        "message": "Key set successfully"
    })))
}

/// Delete a key
pub async fn delete_key(
    State(_state): State<AdminState>,
    Path(key): Path<String>,
) -> impl IntoResponse {
    let mut store = KV_STORE.write().unwrap();
    
    if store.remove(&key).is_some() {
        (StatusCode::OK, Json(serde_json::json!({
            "success": true,
            "message": "Key deleted successfully"
        })))
    } else {
        (StatusCode::NOT_FOUND, Json(serde_json::json!({
            "success": false,
            "error": "Key not found"
        })))
    }
}

/// Check if a key exists
pub async fn key_exists(
    State(_state): State<AdminState>,
    Path(key): Path<String>,
) -> impl IntoResponse {
    let store = KV_STORE.read().unwrap();
    let exists = store.contains_key(&key);
    
    (StatusCode::OK, Json(serde_json::json!({
        "exists": exists,
        "key": key
    })))
}
/// Query keys with filters
pub async fn query_keys(
    State(_state): State<AdminState>,
    Query(params): Query<QueryParams>,
) -> impl IntoResponse {
    let store = KV_STORE.read().unwrap();

    let mut keys: Vec<_> = store.iter()
        .filter(|(k, _)| {
            if let Some(ref prefix) = params.prefix {
                k.starts_with(prefix)
            } else {
                true
            }
        })
        .map(|(k, v)| serde_json::json!({
            "key": k,
            "value": v.value.to_string(),
            "ttl": v.ttl,
            "created_at": chrono::DateTime::from_timestamp(v.created_at, 0).map(|dt| dt.to_rfc3339()),
            "updated_at": chrono::DateTime::from_timestamp(v.updated_at, 0).map(|dt| dt.to_rfc3339()),
        }))
        .collect();
    
    let total = keys.len();
    let offset = params.offset.unwrap_or(0);
    let limit = params.limit.unwrap_or(100);
    
    keys = keys.into_iter().skip(offset).take(limit).collect();
    
    (StatusCode::OK, Json(serde_json::json!({
        "success": true,
        "keys": keys,
        "total": total,
        "limit": limit,
        "offset": offset,
        "has_more": total > offset + limit
    })))
}

/// Execute batch operations
pub async fn batch_operations(
    State(_state): State<AdminState>,
    Json(req): Json<BatchRequest>,
) -> impl IntoResponse {
    let mut store = KV_STORE.write().unwrap();
    let mut results = Vec::new();
    let now = chrono::Utc::now().timestamp();
    
    for op in req.operations {
        let result = match op.operation.as_str() {
            "set" => {
                if let Some(value) = op.value {
                    let entry = KVEntry {
                        key: op.key.clone(),
                        value,
                        ttl: op.ttl,
                        created_at: now,
                        updated_at: now,
                        version: store.get(&op.key).map(|e| e.version + 1).unwrap_or(1),
                        metadata: KVMetadata {
                            content_type: "application/json".to_string(),
                            tags: Vec::new(),
                            properties: HashMap::new(),
                        },
                    };
                    store.insert(op.key.clone(), entry);
                    serde_json::json!({"operation": "set", "key": op.key, "success": true})
                } else {
                    serde_json::json!({"operation": "set", "key": op.key, "success": false, "error": "Value required"})
                }
            }
            "get" => {
                let value = store.get(&op.key).map(|e| e.value.clone());
                serde_json::json!({"operation": "get", "key": op.key, "success": value.is_some(), "value": value})
            }
            "delete" => {
                let success = store.remove(&op.key).is_some();
                serde_json::json!({"operation": "delete", "key": op.key, "success": success})
            }
            _ => serde_json::json!({"operation": op.operation, "key": op.key, "success": false, "error": "Unknown operation"}),
        };
        results.push(result);
    }
    
    (StatusCode::OK, Json(serde_json::json!({
        "success": true,
        "operations": results.len(),
        "results": results
    })))
}

/// Get KV store statistics
pub async fn get_stats(
    State(_state): State<AdminState>,
) -> impl IntoResponse {
    let store = KV_STORE.read().unwrap();
    
    (StatusCode::OK, Json(serde_json::json!({
        "total_keys": store.len(),
        "store_type": "key-value",
        "memory_usage_estimate": store.len() * 1024 // Rough estimate
    })))
}
