//! Document store API handlers (MongoDB-style)

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
    static ref DOC_STORE: Arc<RwLock<HashMap<String, HashMap<String, Document>>>> = 
        Arc::new(RwLock::new(HashMap::new()));
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Document {
    pub id: String,
    pub collection: String,
    pub data: serde_json::Value,
    pub created_at: i64,
    pub updated_at: i64,
    pub version: u64,
    pub metadata: DocumentMetadata,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DocumentMetadata {
    pub tags: Vec<String>,
    pub properties: HashMap<String, String>,
}

#[derive(Debug, Deserialize)]
pub struct CreateDocumentRequest {
    pub id: Option<String>,
    pub data: serde_json::Value,
    pub tags: Option<Vec<String>>,
}

#[derive(Debug, Deserialize)]
pub struct UpdateDocumentRequest {
    pub data: serde_json::Value,
}

#[derive(Debug, Deserialize)]
pub struct QueryRequest {
    pub filter: Option<serde_json::Value>,
    pub sort: Option<Vec<SortField>>,
    pub limit: Option<usize>,
    pub offset: Option<usize>,
    pub fields: Option<Vec<String>>,
}

#[derive(Debug, Deserialize)]
pub struct SortField {
    pub field: String,
    pub desc: bool,
}

#[derive(Debug, Deserialize)]
pub struct AggregateRequest {
    pub pipeline: Vec<serde_json::Value>,
}

#[derive(Debug, Deserialize)]
pub struct IndexRequest {
    pub name: String,
    pub fields: Vec<String>,
    pub unique: bool,
}

/// Create a new document
pub async fn create_document(
    State(_state): State<AdminState>,
    Path(collection): Path<String>,
    Json(req): Json<CreateDocumentRequest>,
) -> impl IntoResponse {
    let mut store = DOC_STORE.write().unwrap();
    let now = chrono::Utc::now().timestamp();
    
    let id = req.id.unwrap_or_else(|| format!("doc_{}", uuid::Uuid::new_v4()));
    
    let doc = Document {
        id: id.clone(),
        collection: collection.clone(),
        data: req.data,
        created_at: now,
        updated_at: now,
        version: 1,
        metadata: DocumentMetadata {
            tags: req.tags.unwrap_or_default(),
            properties: HashMap::new(),
        },
    };
    
    store.entry(collection.clone())
        .or_insert_with(HashMap::new)
        .insert(id.clone(), doc.clone());
    
    (StatusCode::CREATED, Json(serde_json::json!({
        "success": true,
        "document": doc
    })))
}

/// Get a document by ID
pub async fn get_document(
    State(_state): State<AdminState>,
    Path((collection, id)): Path<(String, String)>,
) -> impl IntoResponse {
    let store = DOC_STORE.read().unwrap();
    
    match store.get(&collection).and_then(|coll| coll.get(&id)) {
        Some(doc) => (StatusCode::OK, Json(serde_json::json!({
            "success": true,
            "document": doc
        }))),
        None => (StatusCode::NOT_FOUND, Json(serde_json::json!({
            "success": false,
            "error": "Document not found"
        }))),
    }
}

/// Update a document
pub async fn update_document(
    State(_state): State<AdminState>,
    Path((collection, id)): Path<(String, String)>,
    Json(req): Json<UpdateDocumentRequest>,
) -> impl IntoResponse {
    let mut store = DOC_STORE.write().unwrap();
    let now = chrono::Utc::now().timestamp();
    
    if let Some(coll) = store.get_mut(&collection) {
        if let Some(doc) = coll.get_mut(&id) {
            doc.data = req.data;
            doc.updated_at = now;
            doc.version += 1;
            
            return (StatusCode::OK, Json(serde_json::json!({
                "success": true,
                "document": doc.clone()
            })));
        }
    }
    
    (StatusCode::NOT_FOUND, Json(serde_json::json!({
        "success": false,
        "error": "Document not found"
    })))
}

/// Delete a document
pub async fn delete_document(
    State(_state): State<AdminState>,
    Path((collection, id)): Path<(String, String)>,
) -> impl IntoResponse {
    let mut store = DOC_STORE.write().unwrap();
    
    if let Some(coll) = store.get_mut(&collection) {
        if coll.remove(&id).is_some() {
            return (StatusCode::OK, Json(serde_json::json!({
                "success": true,
                "message": "Document deleted successfully"
            })));
        }
    }
    
    (StatusCode::NOT_FOUND, Json(serde_json::json!({
        "success": false,
        "error": "Document not found"
    })))
}

/// Query documents in a collection
pub async fn query_documents(
    State(_state): State<AdminState>,
    Path(collection): Path<String>,
    Json(req): Json<QueryRequest>,
) -> impl IntoResponse {
    let store = DOC_STORE.read().unwrap();
    let limit = req.limit.unwrap_or(100).min(1000);
    let offset = req.offset.unwrap_or(0);
    
    let documents: Vec<Document> = store
        .get(&collection)
        .map(|coll| {
            let mut docs: Vec<_> = coll.values().cloned().collect();
            
            // Apply filter if provided
            if let Some(filter) = req.filter {
                docs.retain(|doc| matches_filter(&doc.data, &filter));
            }
            
            // Apply sorting
            if let Some(sort_fields) = req.sort {
                for sort_field in sort_fields.iter().rev() {
                    docs.sort_by(|a, b| {
                        let a_val = get_field_value(&a.data, &sort_field.field);
                        let b_val = get_field_value(&b.data, &sort_field.field);
                        
                        let cmp = compare_values(&a_val, &b_val);
                        if sort_field.desc {
                            cmp.reverse()
                        } else {
                            cmp
                        }
                    });
                }
            }
            
            docs
        })
        .unwrap_or_default();
    
    let total = documents.len();
    let paginated: Vec<_> = documents.into_iter()
        .skip(offset)
        .take(limit)
        .collect();
    
    (StatusCode::OK, Json(serde_json::json!({
        "success": true,
        "documents": paginated,
        "total": total,
        "limit": limit,
        "offset": offset,
        "has_more": total > offset + limit
    })))
}

/// Aggregate documents (MongoDB-style pipeline)
pub async fn aggregate_documents(
    State(_state): State<AdminState>,
    Path(collection): Path<String>,
    Json(req): Json<AggregateRequest>,
) -> impl IntoResponse {
    let store = DOC_STORE.read().unwrap();
    
    let mut documents: Vec<Document> = store
        .get(&collection)
        .map(|coll| coll.values().cloned().collect())
        .unwrap_or_default();
    
    // Process pipeline stages
    for stage in req.pipeline {
        if let Some(stage_obj) = stage.as_object() {
            // $match stage
            if let Some(match_filter) = stage_obj.get("$match") {
                documents.retain(|doc| matches_filter(&doc.data, match_filter));
            }
            
            // $limit stage
            if let Some(limit) = stage_obj.get("$limit").and_then(|v| v.as_u64()) {
                documents.truncate(limit as usize);
            }
            
            // $skip stage
            if let Some(skip) = stage_obj.get("$skip").and_then(|v| v.as_u64()) {
                documents = documents.into_iter().skip(skip as usize).collect();
            }
            
            // $sort stage
            if let Some(sort_obj) = stage_obj.get("$sort").and_then(|v| v.as_object()) {
                for (field, order) in sort_obj {
                    let desc = order.as_i64().unwrap_or(1) == -1;
                    documents.sort_by(|a, b| {
                        let a_val = get_field_value(&a.data, field);
                        let b_val = get_field_value(&b.data, field);
                        let cmp = compare_values(&a_val, &b_val);
                        if desc { cmp.reverse() } else { cmp }
                    });
                }
            }
        }
    }
    
    (StatusCode::OK, Json(serde_json::json!({
        "success": true,
        "results": documents,
        "count": documents.len()
    })))
}

/// List all collections
pub async fn list_collections(
    State(_state): State<AdminState>,
) -> impl IntoResponse {
    let store = DOC_STORE.read().unwrap();
    
    let collections: Vec<_> = store.keys()
        .map(|name| {
            let doc_count = store.get(name).map(|c| c.len()).unwrap_or(0);
            serde_json::json!({
                "name": name,
                "document_count": doc_count
            })
        })
        .collect();
    
    (StatusCode::OK, Json(serde_json::json!({
        "collections": collections,
        "count": collections.len()
    })))
}

// Helper functions

fn matches_filter(data: &serde_json::Value, filter: &serde_json::Value) -> bool {
    if let Some(filter_obj) = filter.as_object() {
        for (key, expected) in filter_obj {
            let actual = get_field_value(data, key);
            
            // Handle operators
            if let Some(op_obj) = expected.as_object() {
                for (op, value) in op_obj {
                    match op.as_str() {
                        "$eq" => if actual != *value { return false; },
                        "$ne" => if actual == *value { return false; },
                        "$gt" => if compare_values(&actual, value) != std::cmp::Ordering::Greater { return false; },
                        "$gte" => if matches!(compare_values(&actual, value), std::cmp::Ordering::Less) { return false; },
                        "$lt" => if compare_values(&actual, value) != std::cmp::Ordering::Less { return false; },
                        "$lte" => if matches!(compare_values(&actual, value), std::cmp::Ordering::Greater) { return false; },
                        _ => {}
                    }
                }
            } else if actual != *expected {
                return false;
            }
        }
    }
    true
}

fn get_field_value(data: &serde_json::Value, field: &str) -> serde_json::Value {
    data.get(field).cloned().unwrap_or(serde_json::Value::Null)
}

fn compare_values(a: &serde_json::Value, b: &serde_json::Value) -> std::cmp::Ordering {
    use serde_json::Value;
    match (a, b) {
        (Value::Number(a), Value::Number(b)) => {
            a.as_f64().partial_cmp(&b.as_f64()).unwrap_or(std::cmp::Ordering::Equal)
        }
        (Value::String(a), Value::String(b)) => a.cmp(b),
        (Value::Bool(a), Value::Bool(b)) => a.cmp(b),
        _ => std::cmp::Ordering::Equal,
    }
}
