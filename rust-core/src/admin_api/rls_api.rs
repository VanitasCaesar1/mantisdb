//! RLS API handlers

use axum::{
    extract::{Query, State},
    http::StatusCode,
    response::{IntoResponse, Json},
};
use serde::{Deserialize, Serialize};

use super::AdminState;
use crate::rls::{Policy, PolicyContext};

#[derive(Debug, Deserialize)]
pub struct EnableRlsRequest {
    pub table: String,
}

#[derive(Debug, Deserialize)]
pub struct RlsStatusQuery {
    pub table: String,
}

#[derive(Debug, Deserialize)]
pub struct PoliciesQuery {
    pub table: String,
}

#[derive(Debug, Deserialize)]
pub struct AddPolicyRequest {
    pub policy: Policy,
}

#[derive(Debug, Deserialize)]
pub struct RemovePolicyRequest {
    pub table: String,
    pub policy_name: String,
}

#[derive(Debug, Deserialize)]
pub struct CheckPermissionRequest {
    pub table: String,
    pub operation: String, // "select", "insert", "update", "delete"
    pub context: PolicyContext,
    pub row_data: serde_json::Value,
    pub new_row_data: Option<serde_json::Value>,
}

#[derive(Debug, Serialize)]
pub struct ApiResponse<T> {
    pub success: bool,
    pub message: Option<String>,
    pub data: Option<T>,
    pub error: Option<String>,
}

impl<T> ApiResponse<T> {
    pub fn success(data: T) -> Self {
        Self {
            success: true,
            message: None,
            data: Some(data),
            error: None,
        }
    }
    
    pub fn success_with_message(data: T, message: String) -> Self {
        Self {
            success: true,
            message: Some(message),
            data: Some(data),
            error: None,
        }
    }
    
    pub fn error(message: String) -> Self {
        Self {
            success: false,
            message: None,
            data: None,
            error: Some(message),
        }
    }
}

pub async fn enable_rls(
    State(state): State<AdminState>,
    Json(req): Json<EnableRlsRequest>,
) -> impl IntoResponse {
    match state.rls_engine.enable_rls(&req.table) {
        Ok(_) => (
            StatusCode::OK,
            Json(ApiResponse::<()>::success_with_message(
                (),
                format!("RLS enabled for table '{}'", req.table),
            )),
        ),
        Err(e) => (
            StatusCode::INTERNAL_SERVER_ERROR,
            Json(ApiResponse::<()>::error(format!("Failed to enable RLS: {}", e))),
        ),
    }
}

pub async fn disable_rls(
    State(state): State<AdminState>,
    Json(req): Json<EnableRlsRequest>,
) -> impl IntoResponse {
    match state.rls_engine.disable_rls(&req.table) {
        Ok(_) => (
            StatusCode::OK,
            Json(ApiResponse::<()>::success_with_message(
                (),
                format!("RLS disabled for table '{}'", req.table),
            )),
        ),
        Err(e) => (
            StatusCode::INTERNAL_SERVER_ERROR,
            Json(ApiResponse::<()>::error(format!("Failed to disable RLS: {}", e))),
        ),
    }
}

pub async fn get_rls_status(
    State(state): State<AdminState>,
    Query(params): Query<RlsStatusQuery>,
) -> impl IntoResponse {
    let enabled = state.rls_engine.is_rls_enabled(&params.table);
    
    (
        StatusCode::OK,
        Json(ApiResponse::success(serde_json::json!({
            "table": params.table,
            "enabled": enabled,
        }))),
    )
}

pub async fn list_policies(
    State(state): State<AdminState>,
    Query(params): Query<PoliciesQuery>,
) -> impl IntoResponse {
    let policies = state.rls_engine.get_policies(&params.table);
    
    (
        StatusCode::OK,
        Json(ApiResponse::success(serde_json::json!({
            "table": params.table,
            "policies": policies,
            "count": policies.len(),
        }))),
    )
}

pub async fn add_policy(
    State(state): State<AdminState>,
    Json(req): Json<AddPolicyRequest>,
) -> impl IntoResponse {
    let policy_name = req.policy.name.clone();
    match state.rls_engine.add_policy(req.policy.clone()) {
        Ok(_) => (
            StatusCode::OK,
            Json(ApiResponse::success_with_message(
                req.policy,
                format!("Policy '{}' added successfully", policy_name),
            )),
        ),
        Err(e) => (
            StatusCode::INTERNAL_SERVER_ERROR,
            Json(ApiResponse::<Policy>::error(format!("Failed to add policy: {}", e))),
        ),
    }
}

pub async fn remove_policy(
    State(state): State<AdminState>,
    Json(req): Json<RemovePolicyRequest>,
) -> impl IntoResponse {
    match state.rls_engine.remove_policy(&req.table, &req.policy_name) {
        Ok(_) => (
            StatusCode::OK,
            Json(ApiResponse::<()>::success_with_message(
                (),
                format!("Policy '{}' removed from table '{}'", req.policy_name, req.table),
            )),
        ),
        Err(e) => (
            StatusCode::INTERNAL_SERVER_ERROR,
            Json(ApiResponse::<()>::error(format!("Failed to remove policy: {}", e))),
        ),
    }
}

pub async fn check_permission(
    State(state): State<AdminState>,
    Json(req): Json<CheckPermissionRequest>,
) -> impl IntoResponse {
    let operation = req.operation.to_lowercase();
    
    let result = match operation.as_str() {
        "select" => state.rls_engine.check_select(&req.table, &req.context, &req.row_data),
        "insert" => state.rls_engine.check_insert(&req.table, &req.context, &req.row_data),
        "update" => {
            if let Some(new_row) = &req.new_row_data {
                state.rls_engine.check_update(&req.table, &req.context, &req.row_data, new_row)
            } else {
                return (
                    StatusCode::BAD_REQUEST,
                    Json(ApiResponse::<serde_json::Value>::error(
                        "New row data is required for update operations".to_string()
                    )),
                );
            }
        }
        "delete" => state.rls_engine.check_delete(&req.table, &req.context, &req.row_data),
        _ => {
            return (
                StatusCode::BAD_REQUEST,
                Json(ApiResponse::<serde_json::Value>::error(
                    format!("Unknown operation: {}", req.operation)
                )),
            );
        }
    };
    
    match result {
        Ok(allowed) => (
            StatusCode::OK,
            Json(ApiResponse::success(serde_json::json!({
                "table": req.table,
                "operation": req.operation,
                "allowed": allowed,
            }))),
        ),
        Err(e) => (
            StatusCode::INTERNAL_SERVER_ERROR,
            Json(ApiResponse::<serde_json::Value>::error(
                format!("Permission check failed: {}", e)
            )),
        ),
    }
}
