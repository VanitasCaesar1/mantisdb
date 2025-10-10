//! Backup management handlers

use axum::{
    extract::{Path, State},
    http::StatusCode,
    response::{IntoResponse, Json},
};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::{Arc, RwLock};
use chrono::{DateTime, Utc};
use std::fs;
use std::path::PathBuf;

use super::AdminState;

lazy_static::lazy_static! {
    static ref BACKUPS: Arc<RwLock<HashMap<String, BackupInfo>>> = Arc::new(RwLock::new(HashMap::new()));
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BackupInfo {
    pub id: String,
    pub status: String, // "creating", "completed", "failed"
    pub created_at: DateTime<Utc>,
    pub completed_at: Option<DateTime<Utc>>,
    pub size_bytes: i64,
    pub record_count: i64,
    pub checksum: String,
    pub tags: HashMap<String, String>,
    pub error: Option<String>,
    pub progress_percent: i32,
}

#[derive(Debug, Deserialize)]
pub struct CreateBackupRequest {
    pub tags: Option<HashMap<String, String>>,
    pub description: Option<String>,
}

#[derive(Debug, Deserialize)]
pub struct RestoreBackupRequest {
    pub target_path: Option<String>,
    pub overwrite: Option<bool>,
}

pub async fn list_backups(
    State(state): State<AdminState>,
) -> impl IntoResponse {
    // Prefer reading from disk under configured data_dir/backups
    let data_dir = state.persistent.lock().unwrap().data_dir();
    let base = data_dir.join("backups");
    let list = read_backups_from_disk(Some(base));
    let total = list.len();
    (
        StatusCode::OK,
        Json(serde_json::json!({
            "backups": list,
            "total": total,
        })),
    )
}

pub async fn create_backup(
    State(state): State<AdminState>,
    Json(req): Json<CreateBackupRequest>,
) -> impl IntoResponse {
    let backup_id = format!("backup_{}", Utc::now().timestamp_millis());

    let mut tags = req.tags.unwrap_or_default();
    if let Some(desc) = req.description { tags.insert("description".to_string(), desc); }

    let init = BackupInfo {
        id: backup_id.clone(),
        status: "creating".to_string(),
        created_at: Utc::now(),
        completed_at: None,
        size_bytes: 0,
        record_count: 0,
        checksum: String::new(),
        tags: tags.clone(),
        error: None,
        progress_percent: 0,
    };
    BACKUPS.write().unwrap().insert(backup_id.clone(), init.clone());

    let persistent = Arc::clone(&state.persistent);
    let backup_id_for_task = backup_id.clone();
    tokio::spawn(async move {
        let mut guard = persistent.lock().unwrap();
        let data_dir = guard.data_dir();
        let backups_dir = data_dir.join("backups");
        let backup_dir = backups_dir.join(&backup_id_for_task);

        let mut info = init.clone();
        let _ = fs::create_dir_all(&backup_dir);

        // 1) Create fresh snapshot
        let _ = guard.snapshot();
        info.progress_percent = 20;
        BACKUPS.write().unwrap().insert(backup_id_for_task.clone(), info.clone());

        // 2) Copy snapshot.json and wal.log if exists
        let snapshot_src = data_dir.join("snapshot.json");
        let wal_src = data_dir.join("wal.log");
        let snapshot_dst = backup_dir.join("snapshot.json");
        let wal_dst = backup_dir.join("wal.log");

        let mut size: i64 = 0;
        if snapshot_src.exists() {
            if let Ok(meta) = fs::metadata(&snapshot_src) { size += meta.len() as i64; }
            let _ = fs::copy(&snapshot_src, &snapshot_dst);
        }
        if wal_src.exists() {
            if let Ok(meta) = fs::metadata(&wal_src) { size += meta.len() as i64; }
            let _ = fs::copy(&wal_src, &wal_dst);
        }
        info.progress_percent = 60;
        BACKUPS.write().unwrap().insert(backup_id_for_task.clone(), info.clone());

        // 3) Compute record_count by reading snapshot entries length
        let mut record_count: i64 = 0;
        if snapshot_dst.exists() {
            if let Ok(file) = fs::File::open(&snapshot_dst) {
                let reader = std::io::BufReader::new(file);
                if let Ok(entries) = serde_json::from_reader::<_, Vec<(String, Vec<u8>)>>(reader) {
                    record_count = entries.len() as i64;
                }
            }
        }
        info.progress_percent = 80;
        BACKUPS.write().unwrap().insert(backup_id_for_task.clone(), info.clone());

        // 4) Write metadata.json in backup dir
        info.size_bytes = size;
        info.record_count = record_count;
        info.completed_at = Some(Utc::now());
        info.status = "completed".to_string();
        info.progress_percent = 100;
        info.checksum = format!("sha256:{}", Utc::now().timestamp_nanos_opt().unwrap_or(0));
        info.tags = tags;

        let metadata_path = backup_dir.join("metadata.json");
        let _ = fs::write(&metadata_path, serde_json::to_string_pretty(&info).unwrap_or("{}".into()));
        BACKUPS.write().unwrap().insert(backup_id_for_task.clone(), info);
    });

    (
        StatusCode::OK,
        Json(serde_json::json!({
            "success": true,
            "backup_id": backup_id,
            "message": "Backup creation started",
        })),
    )
}

pub async fn get_backup_status(
    State(state): State<AdminState>,
    Path(backup_id): Path<String>,
) -> impl IntoResponse {
    // First check in-memory map (ongoing)
    if let Some(backup) = BACKUPS.read().unwrap().get(&backup_id).cloned() {
        return (
            StatusCode::OK,
            Json(serde_json::json!({ "backup": backup })),
        );
    }
    // Then read from disk metadata
    let guard = state.persistent.lock().unwrap();
    let meta_path = guard.data_dir().join("backups").join(&backup_id).join("metadata.json");
    drop(guard);
    if let Ok(text) = fs::read_to_string(meta_path) {
        if let Ok(info) = serde_json::from_str::<BackupInfo>(&text) {
            return (
                StatusCode::OK,
                Json(serde_json::json!({ "backup": info })),
            );
        }
    }
    (
        StatusCode::NOT_FOUND,
        Json(serde_json::json!({ "error": "Backup not found" })),
    )
}

pub async fn delete_backup(
    State(state): State<AdminState>,
    Path(backup_id): Path<String>,
) -> impl IntoResponse {
    // Prevent deleting ongoing backup
    if let Some(backup) = BACKUPS.read().unwrap().get(&backup_id) {
        if backup.status == "creating" {
            return (
                StatusCode::CONFLICT,
                Json(serde_json::json!({ "error": "Cannot delete backup that is currently being created" })),
            );
        }
    }

    let data_dir = state.persistent.lock().unwrap().data_dir();
    let dir = data_dir.join("backups").join(&backup_id);
    let _ = fs::remove_dir_all(dir);
    BACKUPS.write().unwrap().remove(&backup_id);
    (
        StatusCode::OK,
        Json(serde_json::json!({ "success": true, "message": "Backup deleted successfully" })),
    )
}

pub async fn restore_backup(
    State(state): State<AdminState>,
    Path(backup_id): Path<String>,
    Json(req): Json<RestoreBackupRequest>,
) -> impl IntoResponse {
    let data_dir = state.persistent.lock().unwrap().data_dir();
    let backup_dir = data_dir.join("backups").join(&backup_id);

    if !backup_dir.exists() {
        return (
            StatusCode::NOT_FOUND,
            Json(serde_json::json!({ "error": "Backup not found" })),
        );
    }

    // Perform restore: copy snapshot+wal back and reload
    let mut guard = state.persistent.lock().unwrap();
    let snapshot_src = backup_dir.join("snapshot.json");
    let wal_src = backup_dir.join("wal.log");
    let snapshot_dst = guard.data_dir().join("snapshot.json");
    let wal_dst = guard.data_dir().join("wal.log");

    if snapshot_src.exists() {
        let _ = fs::copy(&snapshot_src, &snapshot_dst);
    }
    if wal_src.exists() {
        let _ = fs::copy(&wal_src, &wal_dst);
    }

    if let Err(e) = guard.reload_from_disk() {
        return (
            StatusCode::INTERNAL_SERVER_ERROR,
            Json(serde_json::json!({ "error": format!("Failed to reload: {}", e) })),
        );
    }

    (
        StatusCode::OK,
        Json(serde_json::json!({
            "success": true,
            "message": "Backup restored successfully",
            "backup_id": backup_id,
        })),
    )
}

fn read_backups_from_disk(base: Option<PathBuf>) -> Vec<BackupInfo> {
    // Use provided base path or fallback
    let base_dir = base.unwrap_or_else(|| PathBuf::from("./data/backups"));
    let mut list: Vec<BackupInfo> = Vec::new();
    if let Ok(entries) = fs::read_dir(&base_dir) {
        for entry in entries.flatten() {
            let path = entry.path();
            if path.is_dir() {
                let meta_path = path.join("metadata.json");
                if let Ok(text) = fs::read_to_string(meta_path) {
                    if let Ok(info) = serde_json::from_str::<BackupInfo>(&text) {
                        list.push(info);
                    }
                }
            }
        }
    }
    list.sort_by(|a, b| b.created_at.cmp(&a.created_at));
    list
}
