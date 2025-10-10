//! Security utilities and middleware

use axum::{
    extract::Request,
    middleware::Next,
    response::Response,
    http::StatusCode,
};
use std::time::{Duration, SystemTime};
use std::collections::HashMap;
use std::sync::{Arc, RwLock};

lazy_static::lazy_static! {
    static ref RATE_LIMITER: Arc<RwLock<RateLimiter>> = Arc::new(RwLock::new(RateLimiter::new(100, Duration::from_secs(1))));
}

/// Rate limiter for API endpoints
pub struct RateLimiter {
    requests: HashMap<String, Vec<SystemTime>>,
    max_requests: usize,
    window: Duration,
}

impl RateLimiter {
    pub fn new(max_requests: usize, window: Duration) -> Self {
        Self {
            requests: HashMap::new(),
            max_requests,
            window,
        }
    }

    pub fn check_rate_limit(&mut self, ip: &str) -> bool {
        let now = SystemTime::now();

        // Get or create request history for this IP
        let requests = self.requests.entry(ip.to_string()).or_insert_with(Vec::new);

        // Remove old requests outside the window
        requests.retain(|&time| now.duration_since(time).unwrap_or_default() < self.window);

        // Check if under limit
        if requests.len() < self.max_requests {
            requests.push(now);
            true
        } else {
            false
        }
    }

    pub fn cleanup(&mut self) {
        let now = SystemTime::now();

        self.requests.retain(|_, requests| {
            requests.retain(|&time| now.duration_since(time).unwrap_or_default() < self.window);
            !requests.is_empty()
        });
    }
}

/// Rate limiting middleware
pub async fn rate_limit_middleware(
    request: Request,
    next: Next,
) -> Result<Response, StatusCode> {
    // Extract IP address
    let ip = request
        .headers()
        .get("x-forwarded-for")
        .and_then(|h| h.to_str().ok())
        .unwrap_or("unknown")
        .to_string();

    // Check rate limit
    let allowed = {
        let mut limiter = RATE_LIMITER.write().unwrap();
        limiter.check_rate_limit(&ip)
    };

    if !allowed {
        return Err(StatusCode::TOO_MANY_REQUESTS);
    }

    Ok(next.run(request).await)
}

/// Security headers middleware
pub async fn security_headers_middleware(
    request: Request,
    next: Next,
) -> Response {
    let mut response = next.run(request).await;
    
    let headers = response.headers_mut();
    
    // Security headers
    headers.insert("X-Content-Type-Options", "nosniff".parse().unwrap());
    headers.insert("X-Frame-Options", "DENY".parse().unwrap());
    headers.insert("X-XSS-Protection", "1; mode=block".parse().unwrap());
    headers.insert("Strict-Transport-Security", "max-age=31536000; includeSubDomains".parse().unwrap());
    headers.insert("Content-Security-Policy", "default-src 'self'".parse().unwrap());
    headers.insert("Referrer-Policy", "strict-origin-when-cross-origin".parse().unwrap());
    headers.insert("Permissions-Policy", "geolocation=(), microphone=(), camera=()".parse().unwrap());
    
    response
}

/// Password hashing utilities (bcrypt)
pub mod password {
    use argon2::{
        password_hash::{rand_core::OsRng, PasswordHash, PasswordHasher, PasswordVerifier, SaltString},
        Argon2,
    };

    pub fn hash_password(password: &str) -> Result<String, String> {
        let salt = SaltString::generate(&mut OsRng);
        let argon2 = Argon2::default();
        
        argon2
            .hash_password(password.as_bytes(), &salt)
            .map(|hash| hash.to_string())
            .map_err(|e| e.to_string())
    }

    pub fn verify_password(password: &str, hash: &str) -> Result<bool, String> {
        let parsed_hash = PasswordHash::new(hash).map_err(|e| e.to_string())?;
        let argon2 = Argon2::default();
        
        Ok(argon2.verify_password(password.as_bytes(), &parsed_hash).is_ok())
    }
}

/// Input validation utilities
pub mod validation {
    use regex::Regex;

    lazy_static::lazy_static! {
        static ref EMAIL_REGEX: Regex = Regex::new(
            r"^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$"
        ).unwrap();
        
        static ref SQL_INJECTION_REGEX: Regex = Regex::new(
            r"(?i)(union|select|insert|update|delete|drop|create|alter|exec|execute|script|javascript|<script)"
        ).unwrap();
    }

    pub fn is_valid_email(email: &str) -> bool {
        EMAIL_REGEX.is_match(email)
    }

    pub fn is_safe_input(input: &str) -> bool {
        !SQL_INJECTION_REGEX.is_match(input)
    }

    pub fn sanitize_string(input: &str) -> String {
        input
            .chars()
            .filter(|c| c.is_alphanumeric() || c.is_whitespace() || "-_@.".contains(*c))
            .collect()
    }

    pub fn validate_password_strength(password: &str) -> Result<(), String> {
        if password.len() < 8 {
            return Err("Password must be at least 8 characters long".to_string());
        }
        
        let has_uppercase = password.chars().any(|c| c.is_uppercase());
        let has_lowercase = password.chars().any(|c| c.is_lowercase());
        let has_digit = password.chars().any(|c| c.is_numeric());
        let has_special = password.chars().any(|c| !c.is_alphanumeric());
        
        if !has_uppercase || !has_lowercase || !has_digit || !has_special {
            return Err("Password must contain uppercase, lowercase, digit, and special character".to_string());
        }
        
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_rate_limiter() {
        let mut limiter = RateLimiter::new(5, Duration::from_secs(1));
        
        // Should allow requests under limit
        for _ in 0..5 {
            assert!(limiter.check_rate_limit("127.0.0.1"));
        }
        
        // Should deny request over limit
        assert!(!limiter.check_rate_limit("127.0.0.1"));
        
        // Different IP should have separate limit
        assert!(limiter.check_rate_limit("192.168.1.1"));
    }

    #[test]
    fn test_email_validation() {
        use validation::is_valid_email;
        
        assert!(is_valid_email("user@example.com"));
        assert!(is_valid_email("test.user@domain.co.uk"));
        assert!(!is_valid_email("invalid.email"));
        assert!(!is_valid_email("@example.com"));
    }

    #[test]
    fn test_password_hashing() {
        use password::{hash_password, verify_password};
        
        let password = "SecureP@ssw0rd";
        let hash = hash_password(password).unwrap();
        
        assert!(verify_password(password, &hash).unwrap());
        assert!(!verify_password("WrongPassword", &hash).unwrap());
    }
}
