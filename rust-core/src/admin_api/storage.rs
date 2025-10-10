//! Storage/file management handlers

use axum::{
    extract::{Query, State},
    http::{header, StatusCode},
    response::{IntoResponse, Json, Response},
};
use chrono::{DateTime, Utc};
use serde::Deserialize;
use std::fs;
use std::path::PathBuf;

use super::AdminState;

#[derive(Debug, Deserialize)]
pub struct ListQuery {
    pub path: Option<String>,
}

#[derive(Debug, Deserialize)]
pub struct DownloadQuery {
    pub path: String,
}

pub async fn list_files(
    State(state): State<AdminState>,
    Query(params): Query<ListQuery>,
) -> impl IntoResponse {
    let guard = state.persistent.lock().unwrap();
    let base = guard.data_dir();
    drop(guard);

    let rel = params.path.unwrap_or("/".to_string());
    let mut target = if rel == "/" { base.clone() } else { base.join(rel.trim_start_matches('/')) };

    // Canonicalize and ensure it's under base
    let target_can: PathBuf = fs::canonicalize(&target).unwrap_or_else(|_| target.clone());
    if !target_can.starts_with(&base) {
        return (
            StatusCode::BAD_REQUEST,
            Json(serde_json::json!({ "error": "Invalid path" })),
        );
    }

    let mut items = Vec::new();
    if let Ok(read_dir) = fs::read_dir(&target_can) {
        for entry in read_dir.flatten() {
            if let Ok(meta) = entry.metadata() {
                let is_dir = meta.is_dir();
                let name = entry.file_name().to_string_lossy().to_string();
                let item_path = entry.path();
                let modified = meta.modified().ok().map(|m| DateTime::<Utc>::from(m).to_rfc3339());
                let path_rel = item_path.strip_prefix(&base).unwrap_or(&item_path).to_string_lossy().to_string();
                items.push(serde_json::json!({
                    "name": name,
                    "type": if is_dir { "folder" } else { "file" },
                    "size": if is_dir { 0 } else { meta.len() },
                    "modified": modified,
                    "path": format!("/{}", path_rel.trim_start_matches('/')),
                }));
            }
        }
    }

    items.sort_by(|a, b| {
        let ta = a.get("type").and_then(|v| v.as_str()).unwrap_or("");
        let tb = b.get("type").and_then(|v| v.as_str()).unwrap_or("");
        ta.cmp(tb).then_with(|| a.get("name").and_then(|v| v.as_str()).unwrap_or("").cmp(b.get("name").and_then(|v| v.as_str()).unwrap_or("")))
    });

    (
        StatusCode::OK,
        Json(serde_json::json!({ "files": items, "total": items.len() })),
    )
}

pub async fn download_file(
    State(state): State<AdminState>,
    Query(params): Query<DownloadQuery>,
) -> impl IntoResponse {
    let guard = state.persistent.lock().unwrap();
    let base = guard.data_dir();
    drop(guard);

    let mut target = base.join(params.path.trim_start_matches('/'));
    let target_can: PathBuf = match fs::canonicalize(&target) {
        Ok(p) => p,
        Err(_) => return (
            StatusCode::NOT_FOUND,
            Json(serde_json::json!({ "error": "File not found" })),
        ).into_response(),
    };
    if !target_can.starts_with(&base) || target_can.is_dir() {
        return (
            StatusCode::BAD_REQUEST,
            Json(serde_json::json!({ "error": "Invalid file path" })),
        ).into_response();
    }

    match fs::read(&target_can) {
        Ok(bytes) => {
            let filename = target_can.file_name().and_then(|s| s.to_str()).unwrap_or("download.bin");
            Response::builder()
                .status(StatusCode::OK)
                .header(header::CONTENT_TYPE, "application/octet-stream")
                .header(header::CONTENT_DISPOSITION, format!("attachment; filename=\"{}\"", filename))
                .body(axum::body::Body::from(bytes))
                .unwrap()
        }
        Err(_) => (
            StatusCode::NOT_FOUND,
            Json(serde_json::json!({ "error": "File not found" })),
        ).into_response(),
    }
}
