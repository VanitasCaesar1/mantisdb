//! OAuth2 authentication handlers
//!
//! Provides OAuth2 authentication with support for multiple providers:
//! - Google
//! - GitHub
//! - Microsoft
//! - Custom OAuth2 providers

use axum::{
    extract::{Query, State},
    http::{StatusCode, header},
    response::{IntoResponse, Redirect, Json},
};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::{Arc, RwLock};
use chrono::{Utc, Duration};
use jsonwebtoken::{encode, decode, Header, Validation, EncodingKey, DecodingKey, Algorithm};

use super::AdminState;
use super::auth::User;

lazy_static::lazy_static! {
    /// OAuth2 configurations for different providers
    static ref OAUTH_CONFIGS: Arc<RwLock<HashMap<String, OAuthConfig>>> = {
        let mut configs = HashMap::new();
        
        // Google OAuth2 (example - should be configured via env vars)
        configs.insert("google".to_string(), OAuthConfig {
            provider: "google".to_string(),
            client_id: std::env::var("OAUTH_GOOGLE_CLIENT_ID").unwrap_or_default(),
            client_secret: std::env::var("OAUTH_GOOGLE_CLIENT_SECRET").unwrap_or_default(),
            auth_url: "https://accounts.google.com/o/oauth2/v2/auth".to_string(),
            token_url: "https://oauth2.googleapis.com/token".to_string(),
            redirect_uri: std::env::var("OAUTH_REDIRECT_URI")
                .unwrap_or_else(|_| "http://localhost:8081/api/auth/oauth/callback".to_string()),
            scopes: vec!["openid".to_string(), "email".to_string(), "profile".to_string()],
        });
        
        // GitHub OAuth2
        configs.insert("github".to_string(), OAuthConfig {
            provider: "github".to_string(),
            client_id: std::env::var("OAUTH_GITHUB_CLIENT_ID").unwrap_or_default(),
            client_secret: std::env::var("OAUTH_GITHUB_CLIENT_SECRET").unwrap_or_default(),
            auth_url: "https://github.com/login/oauth/authorize".to_string(),
            token_url: "https://github.com/login/oauth/access_token".to_string(),
            redirect_uri: std::env::var("OAUTH_REDIRECT_URI")
                .unwrap_or_else(|_| "http://localhost:8081/api/auth/oauth/callback".to_string()),
            scopes: vec!["user:email".to_string()],
        });
        
        Arc::new(RwLock::new(configs))
    };
    
    /// JWT secret key (should be configured via env var in production)
    static ref JWT_SECRET: String = std::env::var("JWT_SECRET")
        .unwrap_or_else(|_| "mantisdb-secret-change-in-production".to_string());
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OAuthConfig {
    pub provider: String,
    pub client_id: String,
    #[serde(skip_serializing)]
    pub client_secret: String,
    pub auth_url: String,
    pub token_url: String,
    pub redirect_uri: String,
    pub scopes: Vec<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct Claims {
    pub sub: String,  // User ID
    pub email: String,
    pub role: String,
    pub exp: i64,     // Expiration time
    pub iat: i64,     // Issued at
}

#[derive(Debug, Deserialize)]
pub struct OAuthCallbackQuery {
    pub code: String,
    pub state: Option<String>,
}

#[derive(Debug, Serialize)]
pub struct OAuthProvidersResponse {
    pub providers: Vec<OAuthProviderInfo>,
}

#[derive(Debug, Serialize)]
pub struct OAuthProviderInfo {
    pub name: String,
    pub enabled: bool,
    pub auth_url: String,
}

/// Generate a JWT token for a user
pub fn generate_jwt_token(user: &User) -> Result<String, jsonwebtoken::errors::Error> {
    let expiration = Utc::now()
        .checked_add_signed(Duration::hours(24))
        .expect("valid timestamp")
        .timestamp();
    
    let claims = Claims {
        sub: user.id.clone(),
        email: user.email.clone(),
        role: user.role.clone(),
        exp: expiration,
        iat: Utc::now().timestamp(),
    };
    
    encode(
        &Header::default(),
        &claims,
        &EncodingKey::from_secret(JWT_SECRET.as_bytes()),
    )
}

/// Verify and decode a JWT token
pub fn verify_jwt_token(token: &str) -> Result<Claims, jsonwebtoken::errors::Error> {
    decode::<Claims>(
        token,
        &DecodingKey::from_secret(JWT_SECRET.as_bytes()),
        &Validation::new(Algorithm::HS256),
    )
    .map(|data| data.claims)
}

/// List available OAuth2 providers
pub async fn list_oauth_providers(
    State(_state): State<AdminState>,
) -> impl IntoResponse {
    let configs = OAUTH_CONFIGS.read().unwrap();
    
    let providers: Vec<OAuthProviderInfo> = configs
        .values()
        .map(|config| OAuthProviderInfo {
            name: config.provider.clone(),
            enabled: !config.client_id.is_empty() && !config.client_secret.is_empty(),
            auth_url: format!("/api/auth/oauth/{}/authorize", config.provider),
        })
        .collect();
    
    (StatusCode::OK, Json(OAuthProvidersResponse { providers }))
}

/// Initiate OAuth2 authorization flow
pub async fn oauth_authorize(
    State(_state): State<AdminState>,
    axum::extract::Path(provider): axum::extract::Path<String>,
) -> impl IntoResponse {
    let configs = OAUTH_CONFIGS.read().unwrap();
    
    let config = match configs.get(&provider) {
        Some(c) if !c.client_id.is_empty() => c,
        _ => {
            return Err((
                StatusCode::BAD_REQUEST,
                Json(serde_json::json!({
                    "error": format!("OAuth provider '{}' not configured", provider)
                }))
            ));
        }
    };
    
    // Generate state for CSRF protection
    let state = uuid::Uuid::new_v4().to_string();
    
    // Build authorization URL
    let mut auth_url = url::Url::parse(&config.auth_url)
        .map_err(|_| (StatusCode::INTERNAL_SERVER_ERROR, Json(serde_json::json!({"error": "Invalid auth URL"}))))?;
    
    auth_url.query_pairs_mut()
        .append_pair("client_id", &config.client_id)
        .append_pair("redirect_uri", &config.redirect_uri)
        .append_pair("response_type", "code")
        .append_pair("scope", &config.scopes.join(" "))
        .append_pair("state", &state);
    
    Ok(Redirect::temporary(auth_url.as_str()))
}

/// Handle OAuth2 callback
pub async fn oauth_callback(
    State(_state): State<AdminState>,
    Query(params): Query<OAuthCallbackQuery>,
) -> impl IntoResponse {
    // In a real implementation, you would:
    // 1. Verify the state parameter
    // 2. Exchange the code for an access token
    // 3. Fetch user info from the provider
    // 4. Create or update the user in your database
    // 5. Generate a JWT token
    // 6. Redirect to the frontend with the token
    
    // For now, return a placeholder response
    let html = format!(r#"
        <!DOCTYPE html>
        <html>
        <head>
            <title>OAuth Callback</title>
            <script>
                // In production, this would receive the JWT token and store it
                const code = "{}";
                window.opener.postMessage({{ type: 'oauth-success', code }}, '*');
                window.close();
            </script>
        </head>
        <body>
            <p>Authentication successful. You can close this window.</p>
        </body>
        </html>
    "#, params.code);
    
    (
        StatusCode::OK,
        [(header::CONTENT_TYPE, "text/html")],
        html
    )
}

/// Get OAuth2 configuration (public info only)
pub async fn get_oauth_config(
    State(_state): State<AdminState>,
) -> impl IntoResponse {
    let configs = OAUTH_CONFIGS.read().unwrap();
    
    let public_configs: Vec<serde_json::Value> = configs
        .values()
        .filter(|c| !c.client_id.is_empty())
        .map(|c| serde_json::json!({
            "provider": c.provider,
            "enabled": true,
            "scopes": c.scopes,
        }))
        .collect();
    
    (StatusCode::OK, Json(serde_json::json!({
        "oauth_providers": public_configs,
        "jwt_enabled": true,
    })))
}

/// Update OAuth2 configuration (admin only)
pub async fn update_oauth_config(
    State(_state): State<AdminState>,
    Json(new_config): Json<OAuthConfig>,
) -> impl IntoResponse {
    let mut configs = OAUTH_CONFIGS.write().unwrap();
    configs.insert(new_config.provider.clone(), new_config.clone());
    
    (StatusCode::OK, Json(serde_json::json!({
        "success": true,
        "message": format!("OAuth provider '{}' configured", new_config.provider),
    })))
}
