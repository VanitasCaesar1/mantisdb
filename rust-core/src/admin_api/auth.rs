//! Authentication handlers for admin API

use axum::{
    extract::State,
    http::StatusCode,
    response::{IntoResponse, Json},
};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::{Arc, RwLock};
use chrono::{DateTime, Utc, Duration};

use super::AdminState;
use super::security::password::{hash_password, verify_password};
use super::security::validation::{is_valid_email, validate_password_strength};

lazy_static::lazy_static! {
    static ref USERS: Arc<RwLock<HashMap<String, User>>> = {
        Arc::new(RwLock::new(HashMap::new()))
    };
    
    static ref SESSIONS: Arc<RwLock<HashMap<String, Session>>> = Arc::new(RwLock::new(HashMap::new()));
}

const USERS_STORAGE_KEY: &str = "__users__";

/// Load users from persistent storage into memory
pub fn load_users_from_storage(state: &AdminState) {
    if let Ok(data) = state.storage.get_string(USERS_STORAGE_KEY) {
        if let Ok(users_vec) = serde_json::from_slice::<Vec<User>>(&data) {
            let mut users = USERS.write().unwrap();
            for user in users_vec {
                users.insert(user.email.clone(), user);
            }
            println!("✓ Loaded {} users from storage", users.len());
        }
    }
}

/// Save users from memory to persistent storage
fn save_users_to_storage(state: &AdminState) -> Result<(), String> {
    let users = USERS.read().unwrap();
    let users_vec: Vec<User> = users.values().cloned().collect();
    let data = serde_json::to_vec(&users_vec).map_err(|e| e.to_string())?;
    
    if let Ok(mut persistent) = state.persistent.lock() {
        persistent.put(USERS_STORAGE_KEY.to_string(), data).map_err(|e| e.to_string())?;
    }
    Ok(())
}

/// Create demo user if no users exist
pub fn ensure_demo_user(state: &AdminState) {
    let users = USERS.read().unwrap();
    if users.is_empty() {
        drop(users);
        
        let demo_email = "admin@mantisdb.io";
        let demo_password = "admin123";
        
        if let Ok(password_hash) = hash_password(demo_password) {
            let demo_user = User {
                id: format!("user_{}", Utc::now().timestamp_nanos_opt().unwrap_or(0)),
                email: demo_email.to_string(),
                password_hash,
                role: "admin".to_string(),
                created_at: Utc::now(),
            };
            
            let mut users = USERS.write().unwrap();
            users.insert(demo_email.to_string(), demo_user);
            drop(users);
            
            let _ = save_users_to_storage(state);
            println!("✓ Created demo user: {} / {}", demo_email, demo_password);
        }
    }
}

#[derive(Debug, Deserialize)]
pub struct FirstRunCreateRequest {
    pub email: String,
    pub password: String,
}

/// First-run status: true if no users exist
pub async fn first_run_status(
    State(_state): State<AdminState>,
) -> impl IntoResponse {
    let users = USERS.read().unwrap();
    let first_run = users.is_empty();
    (
        StatusCode::OK,
        Json(serde_json::json!({ "first_run": first_run })),
    )
}

/// Create initial admin account (only when there are no users yet)
pub async fn first_run_create(
    State(_state): State<AdminState>,
    Json(req): Json<FirstRunCreateRequest>,
) -> impl IntoResponse {
    // Block if setup already done
    {
        let users = USERS.read().unwrap();
        if !users.is_empty() {
            return (
                StatusCode::FORBIDDEN,
                Json(ApiResponse::<User>::error("Initial setup already completed".to_string())),
            );
        }
    }

    // Validate inputs
    if !is_valid_email(&req.email) {
        return (
            StatusCode::BAD_REQUEST,
            Json(ApiResponse::<User>::error("Invalid email format".to_string())),
        );
    }
    if let Err(e) = validate_password_strength(&req.password) {
        return (
            StatusCode::BAD_REQUEST,
            Json(ApiResponse::<User>::error(e)),
        );
    }

    // Hash password and persist
    let password_hash = match hash_password(&req.password) {
        Ok(h) => h,
        Err(e) => {
            return (
                StatusCode::INTERNAL_SERVER_ERROR,
                Json(ApiResponse::<User>::error(format!("Failed to hash password: {}", e))),
            );
        }
    };

    let new_user = User {
        id: format!("user_{}", Utc::now().timestamp_nanos_opt().unwrap_or(0)),
        email: req.email.clone(),
        password_hash,
        role: "admin".to_string(),
        created_at: Utc::now(),
    };

    let mut users = USERS.write().unwrap();
    users.insert(req.email.clone(), new_user.clone());
    drop(users);
    
    // Persist to storage
    let _ = save_users_to_storage(&_state);

    (
        StatusCode::OK,
        Json(ApiResponse::success(new_user)),
    )
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct User {
    pub id: String,
    pub email: String,
    #[serde(skip_serializing)]
    pub password_hash: String,
    pub role: String,
    pub created_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Session {
    pub token: String,
    pub user_id: String,
    pub created_at: DateTime<Utc>,
    pub expires_at: DateTime<Utc>,
}

#[derive(Debug, Deserialize)]
pub struct LoginRequest {
    pub email: String,
    pub password: String,
}

#[derive(Debug, Serialize)]
pub struct LoginResponse {
    pub success: bool,
    pub token: String,
    pub user: User,
}

#[derive(Debug, Deserialize)]
pub struct CreateUserRequest {
    pub email: String,
    pub password: String,
    pub role: String,
}

#[derive(Debug, Deserialize)]
pub struct ChangePasswordRequest {
    pub current_password: String,
    pub new_password: String,
}

#[derive(Debug, Deserialize)]
pub struct UpdateProfileRequest {
    pub email: String,
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
    
    pub fn error(message: String) -> Self {
        Self {
            success: false,
            message: None,
            data: None,
            error: Some(message),
        }
    }
}

fn generate_token() -> String {
    use rand::Rng;
    use base64::{Engine as _, engine::general_purpose};
    let mut rng = rand::thread_rng();
    let bytes: Vec<u8> = (0..32).map(|_| rng.gen()).collect();
    general_purpose::STANDARD.encode(&bytes)
}

pub async fn login(
    State(_state): State<AdminState>,
    Json(req): Json<LoginRequest>,
) -> impl IntoResponse {
    // Validate email format
    if !is_valid_email(&req.email) {
        return (
            StatusCode::BAD_REQUEST,
            Json(LoginResponse {
                success: false,
                token: String::new(),
                user: User {
                    id: String::new(),
                    email: String::new(),
                    password_hash: String::new(),
                    role: String::new(),
                    created_at: Utc::now(),
                },
            })
        );
    }
    
    let users = USERS.read().unwrap();
    
    if let Some(user) = users.get(&req.email) {
        // Use secure password verification
        if verify_password(&req.password, &user.password_hash).unwrap_or(false) {
            let token = generate_token();
            let session = Session {
                token: token.clone(),
                user_id: user.id.clone(),
                created_at: Utc::now(),
                expires_at: Utc::now() + Duration::hours(24),
            };
            
            SESSIONS.write().unwrap().insert(token.clone(), session);
            
            return (
                StatusCode::OK,
                Json(LoginResponse {
                    success: true,
                    token,
                    user: user.clone(),
                })
            );
        }
    }
    
    (
        StatusCode::UNAUTHORIZED,
        Json(LoginResponse {
            success: false,
            token: String::new(),
            user: User {
                id: String::new(),
                email: String::new(),
                password_hash: String::new(),
                role: String::new(),
                created_at: Utc::now(),
            },
        })
    )
}

pub async fn logout(
    State(_state): State<AdminState>,
    headers: axum::http::HeaderMap,
) -> impl IntoResponse {
    if let Some(auth_header) = headers.get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                SESSIONS.write().unwrap().remove(token);
            }
        }
    }
    
    (
        StatusCode::OK,
        Json(ApiResponse::<()>::success(())),
    )
}

pub async fn verify_token(
    State(_state): State<AdminState>,
    headers: axum::http::HeaderMap,
) -> impl IntoResponse {
    if let Some(auth_header) = headers.get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                let sessions = SESSIONS.read().unwrap();
                if let Some(session) = sessions.get(token) {
                    if Utc::now() < session.expires_at {
                        let users = USERS.read().unwrap();
                        if let Some(user) = users.values().find(|u| u.id == session.user_id) {
                            return (
                                StatusCode::OK,
                                Json(serde_json::json!({
                                    "success": true,
                                    "user": user.clone()
                                })),
                            );
                        }
                    }
                }
            }
        }
    }
    
    (
        StatusCode::UNAUTHORIZED,
        Json(serde_json::json!({
            "success": false,
            "error": "Invalid or expired token"
        })),
    )
}

pub async fn create_user(
    State(_state): State<AdminState>,
    headers: axum::http::HeaderMap,
    Json(req): Json<CreateUserRequest>,
) -> impl IntoResponse {
    // Verify admin access
    if let Some(auth_header) = headers.get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                let sessions = SESSIONS.read().unwrap();
                if let Some(session) = sessions.get(token) {
                    let users = USERS.read().unwrap();
                    if let Some(requesting_user) = users.values().find(|u| u.id == session.user_id) {
                        if requesting_user.role != "admin" {
                            return (
                                StatusCode::FORBIDDEN,
                                Json(ApiResponse::<User>::error("Admin access required".to_string())),
                            );
                        }
                        
                        drop(users);
                        
                        // Validate email
                        if !is_valid_email(&req.email) {
                            return (
                                StatusCode::BAD_REQUEST,
                                Json(ApiResponse::<User>::error("Invalid email format".to_string())),
                            );
                        }
                        
                        // Validate password strength
                        if let Err(e) = validate_password_strength(&req.password) {
                            return (
                                StatusCode::BAD_REQUEST,
                                Json(ApiResponse::<User>::error(e)),
                            );
                        }
                        
                        // Check if user exists
                        let mut users = USERS.write().unwrap();
                        if users.contains_key(&req.email) {
                            return (
                                StatusCode::CONFLICT,
                                Json(ApiResponse::<User>::error("User already exists".to_string())),
                            );
                        }
                        
                        // Hash password
                        let password_hash = match hash_password(&req.password) {
                            Ok(hash) => hash,
                            Err(e) => {
                                return (
                                    StatusCode::INTERNAL_SERVER_ERROR,
                                    Json(ApiResponse::<User>::error(format!("Failed to hash password: {}", e))),
                                );
                            }
                        };
                        
                        // Create new user
                        let new_user = User {
                            id: format!("user_{}", Utc::now().timestamp_nanos_opt().unwrap_or(0)),
                            email: req.email.clone(),
                            password_hash,
                            role: req.role,
                            created_at: Utc::now(),
                        };
                        
                        users.insert(req.email.clone(), new_user.clone());
                        drop(users);
                        
                        // Persist to storage
                        let _ = save_users_to_storage(&_state);
                        
                        return (
                            StatusCode::OK,
                            Json(ApiResponse::success(new_user)),
                        );
                    }
                }
            }
        }
    }
    
    (
        StatusCode::UNAUTHORIZED,
        Json(ApiResponse::<User>::error("Unauthorized".to_string())),
    )
}

pub async fn change_password(
    State(_state): State<AdminState>,
    headers: axum::http::HeaderMap,
    Json(req): Json<ChangePasswordRequest>,
) -> impl IntoResponse {
    if let Some(auth_header) = headers.get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                let sessions = SESSIONS.read().unwrap();
                if let Some(session) = sessions.get(token) {
                    // Validate new password strength
                    if let Err(e) = validate_password_strength(&req.new_password) {
                        return (
                            StatusCode::BAD_REQUEST,
                            Json(ApiResponse::<()>::error(e)),
                        );
                    }
                    
                    let mut users = USERS.write().unwrap();
                    if let Some(user) = users.values_mut().find(|u| u.id == session.user_id) {
                        // Verify current password
                        if verify_password(&req.current_password, &user.password_hash).unwrap_or(false) {
                            // Hash new password
                            match hash_password(&req.new_password) {
                                Ok(new_hash) => {
                                    user.password_hash = new_hash;
                                    drop(users);
                                    
                                    // Persist to storage
                                    let _ = save_users_to_storage(&_state);
                                    
                                    return (
                                        StatusCode::OK,
                                        Json(ApiResponse::<()>::success(())),
                                    );
                                }
                                Err(e) => {
                                    return (
                                        StatusCode::INTERNAL_SERVER_ERROR,
                                        Json(ApiResponse::<()>::error(format!("Failed to hash password: {}", e))),
                                    );
                                }
                            }
                        } else {
                            return (
                                StatusCode::UNAUTHORIZED,
                                Json(ApiResponse::<()>::error("Current password is incorrect".to_string())),
                            );
                        }
                    }
                }
            }
        }
    }
    
    (
        StatusCode::UNAUTHORIZED,
        Json(ApiResponse::<()>::error("Unauthorized".to_string())),
    )
}

pub async fn update_profile(
    State(_state): State<AdminState>,
    headers: axum::http::HeaderMap,
    Json(req): Json<UpdateProfileRequest>,
) -> impl IntoResponse {
    if let Some(auth_header) = headers.get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                let sessions = SESSIONS.read().unwrap();
                if let Some(session) = sessions.get(token) {
                    let mut users = USERS.write().unwrap();
                    
                    // Find and update user
                    let mut updated_user = None;
                    let mut old_email = String::new();
                    
                    for (email, user) in users.iter_mut() {
                        if user.id == session.user_id {
                            old_email = email.clone();
                            user.email = req.email.clone();
                            updated_user = Some(user.clone());
                            break;
                        }
                    }
                    
                    if let Some(user) = updated_user {
                        // Update the key in the map
                        if old_email != req.email {
                            users.remove(&old_email);
                            users.insert(req.email.clone(), user.clone());
                        }
                        drop(users);
                        
                        // Persist to storage
                        let _ = save_users_to_storage(&_state);
                        
                        return (
                            StatusCode::OK,
                            Json(serde_json::json!({
                                "success": true,
                                "user": user
                            })),
                        );
                    }
                }
            }
        }
    }
    
    (
        StatusCode::UNAUTHORIZED,
        Json(serde_json::json!({
            "success": false,
            "error": "Unauthorized"
        })),
    )
}
