//! Logs handlers

use axum::{
    extract::{Query, State},
    http::StatusCode,
    response::{IntoResponse, Json, Sse},
    response::sse::{Event, KeepAlive},
};
use serde::{Deserialize, Serialize};
use std::convert::Infallible;
use std::time::Duration;
use tokio_stream::StreamExt as _;
use tokio_stream::wrappers::IntervalStream;
use chrono::{DateTime, Utc};
use std::fs::File;
use std::io::{BufRead, BufReader};
use std::path::PathBuf;

use super::AdminState;

#[derive(Debug, Serialize, Deserialize)]
pub struct LogEntry {
    pub timestamp: DateTime<Utc>,
    pub level: String,
    pub component: String,
    pub message: String,
    pub request_id: Option<String>,
    pub user_id: Option<String>,
    pub query: Option<String>,
    pub duration_ms: Option<i64>,
    pub metadata: Option<serde_json::Value>,
}

#[derive(Debug, Deserialize)]
pub struct LogsQuery {
    pub level: Option<String>,
    pub component: Option<String>,
    pub limit: Option<usize>,
}

#[derive(Debug, Deserialize, Serialize)]
pub struct SearchLogsRequest {
    pub level: Option<String>,
    pub component: Option<String>,
    pub request_id: Option<String>,
    pub user_id: Option<String>,
    pub start_time: Option<DateTime<Utc>>,
    pub end_time: Option<DateTime<Utc>>,
    pub search_query: Option<String>,
    pub limit: Option<usize>,
    pub offset: Option<usize>,
}

pub async fn get_logs(
    State(_state): State<AdminState>,
    Query(params): Query<LogsQuery>,
) -> impl IntoResponse {
    let limit = params.limit.unwrap_or(100).min(2000);

    let mut entries: Vec<LogEntry> = Vec::new();
    let log_paths = vec![
        PathBuf::from("monitoring/logs/mantisdb-audit.log"),
        PathBuf::from("monitoring/logs/mantisdb-operations.log"),
    ];

    for path in log_paths {
        if let Ok(file) = File::open(&path) {
            use std::collections::VecDeque;
            let reader = BufReader::new(file);
            // Keep only the last `limit` lines in memory per file for efficiency
            let mut ring: VecDeque<String> = VecDeque::with_capacity(limit);
            for line in reader.lines().flatten() {
                if ring.len() == limit { ring.pop_front(); }
                ring.push_back(line);
            }
            for line in ring.into_iter().rev() {
                if line.trim().is_empty() { continue; }
                if let Ok(json) = serde_json::from_str::<serde_json::Value>(&line) {
                    let level = json.get("level").and_then(|v| v.as_str()).unwrap_or("info").to_string();
                    let component = json.get("component").and_then(|v| v.as_str()).unwrap_or("").to_string();
                    // Filters
                    if let Some(lvl) = &params.level {
                        if !level.eq_ignore_ascii_case(lvl) { continue; }
                    }
                    if let Some(comp) = &params.component {
                        if !component.to_lowercase().contains(&comp.to_lowercase()) { continue; }
                    }
                    let timestamp: String = json
                        .get("timestamp")
                        .and_then(|v| v.as_str())
                        .map(|s| s.to_string())
                        .unwrap_or_else(|| Utc::now().to_rfc3339());
                    let parsed_ts = DateTime::parse_from_rfc3339(&timestamp)
                        .map(|dt| dt.with_timezone(&Utc))
                        .unwrap_or_else(|_| Utc::now());
                    let message = json.get("message").and_then(|v| v.as_str()).unwrap_or("").to_string();
                    let request_id = json.get("request_id").and_then(|v| v.as_str()).map(|s| s.to_string());
                    let user_id = json.get("user_id").and_then(|v| v.as_str()).map(|s| s.to_string());
                    let query = json.get("query").and_then(|v| v.as_str()).map(|s| s.to_string());
                    let duration_ms = json.get("duration_ms").and_then(|v| v.as_i64());

                    entries.push(LogEntry {
                        timestamp: parsed_ts,
                        level,
                        component,
                        message,
                        request_id,
                        user_id,
                        query,
                        duration_ms,
                        metadata: Some(json.clone()),
                    });
                }
            }
        }
    }

    // Sort newest first and cap to limit
    entries.sort_by(|a, b| b.timestamp.cmp(&a.timestamp));
    if entries.len() > limit { entries.truncate(limit); }

    (
        StatusCode::OK,
        Json(serde_json::json!({
            "logs": entries,
            "total": entries.len(),
        })),
    )
}

pub async fn search_logs(
    State(_state): State<AdminState>,
    Json(req): Json<SearchLogsRequest>,
) -> impl IntoResponse {
    let limit = req.limit.unwrap_or(200).min(5000);
    let mut matches: Vec<LogEntry> = Vec::new();
    let log_paths = vec![
        PathBuf::from("monitoring/logs/mantisdb-audit.log"),
        PathBuf::from("monitoring/logs/mantisdb-operations.log"),
    ];

    for path in log_paths {
        if let Ok(file) = File::open(&path) {
            use std::collections::VecDeque;
            let reader = BufReader::new(file);
            // Keep only the last `limit` lines per file to bound work
            let mut ring: VecDeque<String> = VecDeque::with_capacity(limit);
            for line in reader.lines().flatten() {
                if ring.len() == limit { ring.pop_front(); }
                ring.push_back(line);
            }
            for line in ring.into_iter().rev() {
                if line.trim().is_empty() { continue; }
                if let Ok(json) = serde_json::from_str::<serde_json::Value>(&line) {
                    let level = json.get("level").and_then(|v| v.as_str()).unwrap_or("").to_string();
                    let component = json.get("component").and_then(|v| v.as_str()).unwrap_or("").to_string();

                    if let Some(ref lvl) = req.level {
                        if !level.eq_ignore_ascii_case(lvl) { continue; }
                    }
                    if let Some(ref comp) = req.component {
                        if !component.to_lowercase().contains(&comp.to_lowercase()) { continue; }
                    }
                    if let Some(ref q) = req.search_query {
                        if !line.to_lowercase().contains(&q.to_lowercase()) { continue; }
                    }

                    let timestamp: String = json
                        .get("timestamp")
                        .and_then(|v| v.as_str())
                        .map(|s| s.to_string())
                        .unwrap_or_else(|| Utc::now().to_rfc3339());
                    let parsed_ts = DateTime::parse_from_rfc3339(&timestamp)
                        .map(|dt| dt.with_timezone(&Utc))
                        .unwrap_or_else(|_| Utc::now());

                    if let Some(start) = req.start_time {
                        if parsed_ts < start { continue; }
                    }
                    if let Some(end) = req.end_time {
                        if parsed_ts > end { continue; }
                    }

                    let message = json.get("message").and_then(|v| v.as_str()).unwrap_or("").to_string();
                    let request_id = json.get("request_id").and_then(|v| v.as_str()).map(|s| s.to_string());
                    let user_id = json.get("user_id").and_then(|v| v.as_str()).map(|s| s.to_string());
                    let query = json.get("query").and_then(|v| v.as_str()).map(|s| s.to_string());
                    let duration_ms = json.get("duration_ms").and_then(|v| v.as_i64());

                    matches.push(LogEntry {
                        timestamp: parsed_ts,
                        level,
                        component,
                        message,
                        request_id,
                        user_id,
                        query,
                        duration_ms,
                        metadata: Some(json.clone()),
                    });
                    if matches.len() >= limit { break; }
                }
            }
        }
    }
    matches.sort_by(|a, b| b.timestamp.cmp(&a.timestamp));
    if matches.len() > limit { matches.truncate(limit); }

    (
        StatusCode::OK,
        Json(serde_json::json!({
            "results": matches,
            "total": matches.len(),
            "has_more": false,
        })),
    )
}

pub async fn stream_logs(
    State(_state): State<AdminState>,
) -> Sse<impl tokio_stream::Stream<Item = Result<Event, Infallible>>> {
    use std::sync::atomic::{AtomicUsize, Ordering};
    use std::sync::Arc;
    let counter = Arc::new(AtomicUsize::new(0));
    let stream = IntervalStream::new(tokio::time::interval(Duration::from_secs(2)))
        .map(move |_| {
            let count = counter.fetch_add(1, Ordering::Relaxed);
            let log_levels = ["INFO", "WARN", "ERROR", "DEBUG"];
            let components = ["query_executor", "cache_manager", "storage_engine", "api_server"];
            let messages = [
                "Operation completed successfully",
                "Cache eviction triggered",
                "Connection established",
                "Query executed",
                "Backup completed",
                "Configuration updated",
            ];
            
            let log_entry = LogEntry {
                timestamp: Utc::now(),
                level: log_levels[count % log_levels.len()].to_string(),
                component: components[count % components.len()].to_string(),
                message: messages[count % messages.len()].to_string(),
                request_id: Some(format!("req_{}", Utc::now().timestamp())),
                user_id: None,
                query: None,
                duration_ms: Some(10 + ((count % 100) as i64)),
                metadata: Some(serde_json::json!({
                    "sequence": count,
                    "node_id": "node_1",
                })),
            };
            
            Ok(Event::default().json_data(log_entry).unwrap())
        });
    
    Sse::new(stream).keep_alive(KeepAlive::default())
}

pub async fn logs_stream(
    State(_state): State<AdminState>,
) -> Sse<impl tokio_stream::Stream<Item = Result<Event, Infallible>>> {
    use std::sync::atomic::{AtomicUsize, Ordering};
    use std::sync::Arc;
    let counter = Arc::new(AtomicUsize::new(0));
    let stream = IntervalStream::new(tokio::time::interval(Duration::from_secs(3)))
        .map(move |_| {
            let count = counter.fetch_add(1, Ordering::Relaxed);
            let log_levels = ["INFO", "WARN", "ERROR", "DEBUG"];
            let components = ["query_executor", "cache_manager", "storage_engine", "api_server"];
            let messages = [
                "Operation completed successfully",
                "Cache eviction triggered",
                "Connection established",
                "Query executed",
                "Backup completed",
                "Configuration updated",
            ];
            
            let log_entry = LogEntry {
                timestamp: Utc::now(),
                level: log_levels[count % log_levels.len()].to_string(),
                component: components[count % components.len()].to_string(),
                message: messages[count % messages.len()].to_string(),
                request_id: Some(format!("req_{}", Utc::now().timestamp())),
                user_id: None,
                query: None,
                duration_ms: Some(10 + ((count % 100) as i64)),
                metadata: Some(serde_json::json!({
                    "sequence": count,
                    "node_id": "node_1",
                })),
            };
            
            let log_update = serde_json::json!({
                "type": "log_entry",
                "data": log_entry,
            });
            
            Ok(Event::default().json_data(log_update).unwrap())
        });
    
    Sse::new(stream).keep_alive(KeepAlive::default())
}
