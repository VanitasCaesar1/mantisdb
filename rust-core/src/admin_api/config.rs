//! Configuration management handlers

use axum::{
    extract::State,
    http::StatusCode,
    response::{IntoResponse, Json},
};
use serde::{Deserialize, Serialize};

use super::AdminState;

#[derive(Debug, Serialize)]
pub struct ConfigResponse {
    pub server: ServerConfig,
    pub database: DatabaseConfig,
    pub features: FeaturesConfig,
}

#[derive(Debug, Serialize)]
pub struct ServerConfig {
    pub host: String,
    pub port: u16,
}

#[derive(Debug, Serialize)]
pub struct DatabaseConfig {
    pub data_dir: String,
    pub cache_size: String,
}

#[derive(Debug, Serialize)]
pub struct FeaturesConfig {
    pub auto_backup: bool,
    pub compression: bool,
    pub replication: bool,
    pub query_logging: bool,
    pub metrics_export: bool,
}

#[derive(Debug, Deserialize)]
pub struct UpdateConfigRequest {
    pub config: serde_json::Value,
}

#[derive(Debug, Deserialize)]
pub struct ValidateConfigRequest {
    pub config: serde_json::Value,
}

#[derive(Debug, Serialize)]
pub struct ValidationResponse {
    pub valid: bool,
    pub errors: Vec<String>,
    pub warnings: Vec<String>,
}

pub async fn get_config(
    State(_state): State<AdminState>,
) -> impl IntoResponse {
    let config = ConfigResponse {
        server: ServerConfig {
            host: "localhost".to_string(),
            port: 8081,
        },
        database: DatabaseConfig {
            data_dir: "./data".to_string(),
            cache_size: "100MB".to_string(),
        },
        features: FeaturesConfig {
            auto_backup: true,
            compression: true,
            replication: false,
            query_logging: true,
            metrics_export: true,
        },
    };
    
    (StatusCode::OK, Json(config))
}

pub async fn update_config(
    State(_state): State<AdminState>,
    Json(req): Json<UpdateConfigRequest>,
) -> impl IntoResponse {
    // Mock implementation - in production, validate and apply config
    (
        StatusCode::OK,
        Json(serde_json::json!({
            "success": true,
            "message": "Configuration updated successfully",
            "config": req.config,
        })),
    )
}

pub async fn validate_config(
    State(_state): State<AdminState>,
    Json(req): Json<ValidateConfigRequest>,
) -> impl IntoResponse {
    // Mock validation
    let mut errors = Vec::new();
    let mut warnings = Vec::new();
    
    // Check for required fields
    if !req.config.get("data_dir").is_some() {
        errors.push("data_dir is required".to_string());
    }
    
    if let Some(cache_size) = req.config.get("cache_size") {
        if cache_size.as_str() == Some("") {
            warnings.push("cache_size is empty, using default".to_string());
        }
    }
    
    let response = ValidationResponse {
        valid: errors.is_empty(),
        errors,
        warnings,
    };
    
    (StatusCode::OK, Json(response))
}
