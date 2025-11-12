//! Admin API Module
//!
//! Comprehensive admin API for MantisDB dashboard

pub mod auth;
pub mod oauth;
pub mod tables;
pub mod queries;
pub mod monitoring;
pub mod logs;
pub mod backups;
pub mod rls_api;
pub mod storage;
pub mod config;
pub mod security;
pub mod keyvalue;
pub mod document;
pub mod columnar;

use axum::{
    Router,
    routing::{get, post, put, delete},
    middleware,
    response::{Html, IntoResponse},
    extract::Path,
    http::{StatusCode, header::HeaderValue},
};
use tower_http::cors::CorsLayer;
use tower_http::trace::TraceLayer;
use tower_http::compression::CompressionLayer;
use std::sync::Arc;

use crate::rls::RlsEngine;
use crate::storage::LockFreeStorage;
use crate::persistent_storage::{PersistentStorage, PersistentStorageConfig};
use self::security::{rate_limit_middleware, security_headers_middleware};
use std::sync::Mutex;

use include_dir::{include_dir, Dir};
use mime_guess::MimeGuess;

/// Admin API state shared across handlers
#[derive(Clone)]
pub struct AdminState {
    pub rls_engine: Arc<RlsEngine>,
    pub storage: Arc<LockFreeStorage>,
    pub persistent: Arc<Mutex<PersistentStorage>>,
}

impl AdminState {
    pub fn new() -> Self {
        Self::with_data_dir("./data")
    }
    
    pub fn with_data_dir(data_dir: &str) -> Self {
        let config = PersistentStorageConfig {
            data_dir: data_dir.into(),
            wal_enabled: true,
            sync_on_write: true,
        };
        
        let persistent = PersistentStorage::new(config)
            .expect("Failed to create persistent storage");
        
        let storage = Arc::clone(persistent.memory());
        
        println!("✅ Database initialized with {} entries", storage.len());
        
        let state = Self {
            rls_engine: Arc::new(RlsEngine::new()),
            storage,
            persistent: Arc::new(Mutex::new(persistent)),
        };
        
        // Load users from storage and ensure demo user exists
        auth::load_users_from_storage(&state);
        auth::ensure_demo_user(&state);
        
        state
    }
    
    pub fn with_storage(storage: Arc<LockFreeStorage>) -> Self {
        let config = PersistentStorageConfig::default();
        let persistent = PersistentStorage::new(config)
            .expect("Failed to create persistent storage");
        
        Self {
            rls_engine: Arc::new(RlsEngine::new()),
            storage,
            persistent: Arc::new(Mutex::new(persistent)),
        }
    }
}

/// Serve the OpenAPI specification (embedded at build time)
async fn serve_openapi_spec() -> impl IntoResponse {
    const OPENAPI_YAML: &str = include_str!(concat!(env!("CARGO_MANIFEST_DIR"), "/openapi.yaml"));
    (
        StatusCode::OK,
        [("Content-Type", "text/yaml")],
        OPENAPI_YAML,
    )
}

/// Serve Swagger UI for API documentation
async fn serve_api_docs() -> impl IntoResponse {
    Html(r#"
    <!DOCTYPE html>
    <html lang="en">
    <head>
        <meta charset="UTF-8">
        <meta name="viewport" content="width=device-width, initial-scale=1.0">
        <title>MantisDB API Documentation</title>
        <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.10.0/swagger-ui.css" />
        <style>
            body { margin: 0; padding: 0; }
            #swagger-ui { max-width: 1460px; margin: 0 auto; }
        </style>
    </head>
    <body>
        <div id="swagger-ui"></div>
        <script src="https://unpkg.com/swagger-ui-dist@5.10.0/swagger-ui-bundle.js"></script>
        <script src="https://unpkg.com/swagger-ui-dist@5.10.0/swagger-ui-standalone-preset.js"></script>
        <script>
            window.onload = function() {
                SwaggerUIBundle({
                    url: "/api/docs/openapi.yaml",
                    dom_id: '#swagger-ui',
                    deepLinking: true,
                    presets: [
                        SwaggerUIBundle.presets.apis,
                        SwaggerUIStandalonePreset
                    ],
                    plugins: [
                        SwaggerUIBundle.plugins.DownloadUrl
                    ],
                    layout: "StandaloneLayout"
                });
            };
        </script>
    </body>
    </html>
    "#)
}

/// Embedded admin UI assets directory
static ASSETS: Dir = include_dir!("../../admin/api/assets/dist");

/// Serve embedded index.html
async fn serve_index() -> impl IntoResponse {
    if let Some(file) = ASSETS.get_file("index.html") {
        let body = file.contents();
        return (
            StatusCode::OK,
            [("Content-Type", "text/html; charset=utf-8")],
            body.to_vec(),
        );
    }
    (StatusCode::NOT_FOUND, "Admin UI not found")
}

/// Serve any embedded static asset, falling back to index.html for SPA routes
async fn serve_static(Path(path): Path<String>) -> impl IntoResponse {
    let req_path = if path.is_empty() { "index.html".to_string() } else { path };

    if let Some(file) = ASSETS.get_file(&req_path) {
        // Guess MIME type
        let mime: MimeGuess = mime_guess::from_path(&req_path);
        let content_type = mime.first_or_octet_stream().essence_str().to_string();
        return (
            StatusCode::OK,
            [("Content-Type", content_type.as_str())],
            file.contents().to_vec(),
        );
    }

    // SPA fallback to index.html
    if let Some(index) = ASSETS.get_file("index.html") {
        return (
            StatusCode::OK,
            [("Content-Type", "text/html; charset=utf-8")],
            index.contents().to_vec(),
        );
    }

    (StatusCode::NOT_FOUND, "Not found")
}

/// Build the complete admin API router
pub fn build_admin_router(state: AdminState) -> Router {
    Router::new()
        // API Documentation
        .route("/api/docs", get(serve_api_docs))
        .route("/api/docs/openapi.yaml", get(serve_openapi_spec))
        
        // Health and system
        .route("/api/health", get(monitoring::health_check))
        .route("/api/metrics", get(monitoring::get_metrics))
        .route("/api/metrics/detailed", get(monitoring::get_detailed_metrics))
        .route("/api/metrics/prometheus", get(monitoring::prometheus_metrics))
        .route("/api/stats", get(monitoring::get_system_stats))
        
        // Authentication
        .route("/api/auth/login", post(auth::login))
        .route("/api/auth/logout", post(auth::logout))
        .route("/api/auth/verify", get(auth::verify_token))
        .route("/api/auth/first-run/status", get(auth::first_run_status))
        .route("/api/auth/first-run/create", post(auth::first_run_create))
        .route("/api/auth/create-user", post(auth::create_user))
        .route("/api/auth/change-password", post(auth::change_password))
        .route("/api/auth/update-profile", put(auth::update_profile))
        
        // OAuth2 Authentication
        .route("/api/auth/oauth/providers", get(oauth::list_oauth_providers))
        .route("/api/auth/oauth/config", get(oauth::get_oauth_config))
        .route("/api/auth/oauth/config", put(oauth::update_oauth_config))
        .route("/api/auth/oauth/:provider/authorize", get(oauth::oauth_authorize))
        .route("/api/auth/oauth/callback", get(oauth::oauth_callback))
        
        // Tables
        .route("/api/tables", get(tables::list_tables))
        .route("/api/tables/create", post(tables::create_table))
        .route("/api/tables/:table", get(tables::get_table_data))
        .route("/api/tables/:table/schema", get(tables::get_table_schema))
        .route("/api/tables/:table/schema", put(tables::update_table_schema))
        .route("/api/tables/:table/data", get(tables::get_table_data))
        .route("/api/tables/:table/data", post(tables::create_row))
        .route("/api/tables/:table/data/:id", get(tables::get_row))
        .route("/api/tables/:table/data/:id", put(tables::update_row))
        .route("/api/tables/:table/data/:id", delete(tables::delete_row))
        
        // Queries
        .route("/api/query", post(queries::execute_query))
        .route("/api/query/history", get(queries::get_query_history))
        
        // RLS
        .route("/api/rls/enable", post(rls_api::enable_rls))
        .route("/api/rls/disable", post(rls_api::disable_rls))
        .route("/api/rls/status", get(rls_api::get_rls_status))
        .route("/api/rls/policies", get(rls_api::list_policies))
        .route("/api/rls/policies/add", post(rls_api::add_policy))
        .route("/api/rls/policies/remove", post(rls_api::remove_policy))
        .route("/api/rls/check", post(rls_api::check_permission))
        
        // Logs
        .route("/api/logs", get(logs::get_logs))
        .route("/api/logs/search", post(logs::search_logs))
        .route("/api/logs/stream", get(logs::stream_logs))
        
        // Backups
        .route("/api/backups", get(backups::list_backups))
        .route("/api/backups", post(backups::create_backup))
        .route("/api/backups/:id", get(backups::get_backup_status))
        .route("/api/backups/:id", delete(backups::delete_backup))
        .route("/api/backups/:id/restore", post(backups::restore_backup))
        
        // Storage
        .route("/api/storage/list", get(storage::list_files))
        .route("/api/storage/download", get(storage::download_file))
        
        // Config
        .route("/api/config", get(config::get_config))
        .route("/api/config", put(config::update_config))
        .route("/api/config/validate", post(config::validate_config))
        
        // Key-Value Store
        .route("/api/kv/:key", get(keyvalue::get_key))
        .route("/api/kv/:key", put(keyvalue::set_key))
        .route("/api/kv/:key", delete(keyvalue::delete_key))
        .route("/api/kv/:key/exists", get(keyvalue::key_exists))
        .route("/api/kv/query", get(keyvalue::query_keys))
        .route("/api/kv/batch", post(keyvalue::batch_operations))
        .route("/api/kv/stats", get(keyvalue::get_stats))
        
        // Document Store (MongoDB-style)
        .route("/api/documents/collections", get(document::list_collections))
        .route("/api/documents/:collection", post(document::create_document))
        .route("/api/documents/:collection/:id", get(document::get_document))
        .route("/api/documents/:collection/:id", put(document::update_document))
        .route("/api/documents/:collection/:id", delete(document::delete_document))
        .route("/api/documents/:collection/query", post(document::query_documents))
        .route("/api/documents/:collection/aggregate", post(document::aggregate_documents))
        
        // Columnar Store (Cassandra/ScyllaDB-style)
        .route("/api/columnar/tables", get(columnar::list_tables))
        .route("/api/columnar/tables", post(columnar::create_table))
        .route("/api/columnar/tables/:table", get(columnar::get_table))
        .route("/api/columnar/tables/:table", delete(columnar::drop_table))
        .route("/api/columnar/tables/:table/rows", post(columnar::insert_rows))
        .route("/api/columnar/tables/:table/query", post(columnar::query_rows))
        .route("/api/columnar/tables/:table/update", post(columnar::update_rows))
        .route("/api/columnar/tables/:table/delete", post(columnar::delete_rows))
        .route("/api/columnar/tables/:table/indexes", post(columnar::create_index))
        .route("/api/columnar/tables/:table/stats", get(columnar::get_table_stats))
        .route("/api/columnar/cql", post(columnar::execute_cql))
        
        // WebSocket-style endpoints (SSE)
        .route("/api/ws/metrics", get(monitoring::metrics_stream))
        .route("/api/ws/logs", get(logs::logs_stream))
        .route("/api/ws/events", get(monitoring::events_stream))
        
        // Static admin UI (embedded)
        .route("/", get(serve_index))
        .route("/*path", get(serve_static))
        
        .with_state(state)
        .layer(middleware::from_fn(security_headers_middleware))
        .layer(middleware::from_fn(rate_limit_middleware))
        .layer(CompressionLayer::new())
        .layer(CorsLayer::permissive())
        .layer(TraceLayer::new_for_http())
}
