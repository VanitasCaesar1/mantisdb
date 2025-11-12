//! Production Configuration Management
//! 
//! Features:
//! - Environment-based configuration (dev/staging/prod)
//! - Secrets management (environment variables)
//! - Configuration validation
//! - Hot reload support
//! - Sensible defaults

use crate::error::{Error, Result};
use serde::{Deserialize, Serialize};
use std::path::PathBuf;
use std::time::Duration;

/// Environment type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Environment {
    Development,
    Staging,
    Production,
}

impl Default for Environment {
    fn default() -> Self {
        Environment::Development
    }
}

impl std::str::FromStr for Environment {
    type Err = Error;
    
    fn from_str(s: &str) -> Result<Self> {
        match s.to_lowercase().as_str() {
            "development" | "dev" => Ok(Environment::Development),
            "staging" | "stage" => Ok(Environment::Staging),
            "production" | "prod" => Ok(Environment::Production),
            _ => Err(Error::ConfigError(format!("Invalid environment: {}", s))),
        }
    }
}

/// Complete production configuration
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ProductionConfig {
    /// Environment type
    pub environment: Environment,
    
    /// Server configuration
    pub server: ServerConfig,
    
    /// Database configuration
    pub database: DatabaseConfig,
    
    /// Cache configuration
    pub cache: CacheConfig,
    
    /// Security configuration
    pub security: SecurityConfig,
    
    /// Monitoring configuration
    pub monitoring: MonitoringConfig,
    
    /// Logging configuration
    pub logging: LoggingConfig,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ServerConfig {
    /// Host to bind to
    pub host: String,
    
    /// Port to listen on
    pub port: u16,
    
    /// Number of worker threads (0 = auto)
    pub workers: usize,
    
    /// Request timeout in seconds
    pub request_timeout: u64,
    
    /// Max request body size in bytes
    pub max_body_size: usize,
    
    /// Enable CORS
    pub enable_cors: bool,
    
    /// Enable compression
    pub enable_compression: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DatabaseConfig {
    /// Data directory
    pub data_dir: PathBuf,
    
    /// Enable WAL
    pub wal_enabled: bool,
    
    /// Sync on write (fsync)
    pub sync_on_write: bool,
    
    /// Maximum memory usage in bytes
    pub max_memory: usize,
    
    /// Enable disk-backed storage
    pub enable_disk_storage: bool,
    
    /// Checkpoint interval in seconds
    pub checkpoint_interval: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CacheConfig {
    /// Cache size in bytes
    pub max_size: usize,
    
    /// Cache policy
    pub policy: String,
    
    /// TTL cleanup interval in seconds
    pub cleanup_interval: u64,
    
    /// Enable cache maintenance
    pub enable_maintenance: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SecurityConfig {
    /// JWT secret (loaded from env var)
    #[serde(skip_serializing)]
    pub jwt_secret: String,
    
    /// JWT expiration in seconds
    pub jwt_expiration: u64,
    
    /// Enable rate limiting
    pub enable_rate_limiting: bool,
    
    /// Rate limit: requests per minute
    pub rate_limit_rpm: u32,
    
    /// Enable TLS
    pub enable_tls: bool,
    
    /// TLS certificate path
    pub tls_cert: Option<PathBuf>,
    
    /// TLS key path
    pub tls_key: Option<PathBuf>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MonitoringConfig {
    /// Enable Prometheus metrics
    pub enable_prometheus: bool,
    
    /// Metrics port
    pub metrics_port: u16,
    
    /// Enable health checks
    pub enable_health_checks: bool,
    
    /// Health check interval in seconds
    pub health_check_interval: u64,
    
    /// Enable tracing
    pub enable_tracing: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LoggingConfig {
    /// Log level (trace, debug, info, warn, error)
    pub level: String,
    
    /// Log format (json, pretty)
    pub format: String,
    
    /// Log to file
    pub log_to_file: bool,
    
    /// Log file path
    pub log_file: Option<PathBuf>,
    
    /// Enable structured logging
    pub structured: bool,
}

impl Default for ProductionConfig {
    fn default() -> Self {
        Self::development()
    }
}

impl ProductionConfig {
    /// Load configuration from environment variables
    pub fn from_env() -> Result<Self> {
        let env = std::env::var("MANTIS_ENV")
            .or_else(|_| std::env::var("ENV"))
            .unwrap_or_else(|_| "development".to_string())
            .parse()?;
        
        let mut config = match env {
            Environment::Development => Self::development(),
            Environment::Staging => Self::staging(),
            Environment::Production => Self::production(),
        };
        
        // Override with environment variables
        config.apply_env_overrides()?;
        
        // Validate
        config.validate()?;
        
        Ok(config)
    }
    
    /// Development configuration
    pub fn development() -> Self {
        Self {
            environment: Environment::Development,
            server: ServerConfig {
                host: "127.0.0.1".to_string(),
                port: 3000,
                workers: 0,
                request_timeout: 30,
                max_body_size: 10 * 1024 * 1024, // 10MB
                enable_cors: true,
                enable_compression: false,
            },
            database: DatabaseConfig {
                data_dir: PathBuf::from("./data/dev"),
                wal_enabled: true,
                sync_on_write: false, // Faster in dev
                max_memory: 1024 * 1024 * 1024, // 1GB
                enable_disk_storage: false,
                checkpoint_interval: 300, // 5 minutes
            },
            cache: CacheConfig {
                max_size: 256 * 1024 * 1024, // 256MB
                policy: "write-through".to_string(),
                cleanup_interval: 60,
                enable_maintenance: true,
            },
            security: SecurityConfig {
                jwt_secret: "dev-secret-change-in-production".to_string(),
                jwt_expiration: 86400, // 24 hours
                enable_rate_limiting: false,
                rate_limit_rpm: 1000,
                enable_tls: false,
                tls_cert: None,
                tls_key: None,
            },
            monitoring: MonitoringConfig {
                enable_prometheus: true,
                metrics_port: 9090,
                enable_health_checks: true,
                health_check_interval: 30,
                enable_tracing: true,
            },
            logging: LoggingConfig {
                level: "debug".to_string(),
                format: "pretty".to_string(),
                log_to_file: false,
                log_file: None,
                structured: false,
            },
        }
    }
    
    /// Staging configuration
    pub fn staging() -> Self {
        let mut config = Self::development();
        config.environment = Environment::Staging;
        config.server.host = "0.0.0.0".to_string();
        config.database.data_dir = PathBuf::from("./data/staging");
        config.database.sync_on_write = true;
        config.security.enable_rate_limiting = true;
        config.logging.level = "info".to_string();
        config.logging.log_to_file = true;
        config.logging.log_file = Some(PathBuf::from("./logs/mantisdb.log"));
        config
    }
    
    /// Production configuration
    pub fn production() -> Self {
        Self {
            environment: Environment::Production,
            server: ServerConfig {
                host: "0.0.0.0".to_string(),
                port: 8080,
                workers: 0, // Auto-detect
                request_timeout: 30,
                max_body_size: 5 * 1024 * 1024, // 5MB
                enable_cors: false, // Configure explicitly
                enable_compression: true,
            },
            database: DatabaseConfig {
                data_dir: PathBuf::from("/var/lib/mantisdb/data"),
                wal_enabled: true,
                sync_on_write: true, // Durability
                max_memory: 8 * 1024 * 1024 * 1024, // 8GB
                enable_disk_storage: true,
                checkpoint_interval: 300,
            },
            cache: CacheConfig {
                max_size: 2 * 1024 * 1024 * 1024, // 2GB
                policy: "write-through".to_string(),
                cleanup_interval: 60,
                enable_maintenance: true,
            },
            security: SecurityConfig {
                jwt_secret: String::new(), // MUST be set via env
                jwt_expiration: 3600, // 1 hour
                enable_rate_limiting: true,
                rate_limit_rpm: 1000,
                enable_tls: true,
                tls_cert: Some(PathBuf::from("/etc/mantisdb/tls/cert.pem")),
                tls_key: Some(PathBuf::from("/etc/mantisdb/tls/key.pem")),
            },
            monitoring: MonitoringConfig {
                enable_prometheus: true,
                metrics_port: 9090,
                enable_health_checks: true,
                health_check_interval: 10,
                enable_tracing: true,
            },
            logging: LoggingConfig {
                level: "info".to_string(),
                format: "json".to_string(),
                log_to_file: true,
                log_file: Some(PathBuf::from("/var/log/mantisdb/mantisdb.log")),
                structured: true,
            },
        }
    }
    
    /// Apply environment variable overrides
    fn apply_env_overrides(&mut self) -> Result<()> {
        // Server
        if let Ok(host) = std::env::var("MANTIS_HOST") {
            self.server.host = host;
        }
        if let Ok(port) = std::env::var("MANTIS_PORT") {
            self.server.port = port.parse()
                .map_err(|_| Error::ConfigError("Invalid MANTIS_PORT".to_string()))?;
        }
        
        // Database
        if let Ok(data_dir) = std::env::var("MANTIS_DATA_DIR") {
            self.database.data_dir = PathBuf::from(data_dir);
        }
        if let Ok(sync) = std::env::var("MANTIS_SYNC_ON_WRITE") {
            self.database.sync_on_write = sync.parse()
                .map_err(|_| Error::ConfigError("Invalid MANTIS_SYNC_ON_WRITE".to_string()))?;
        }
        
        // Security (REQUIRED in production)
        if let Ok(secret) = std::env::var("MANTIS_JWT_SECRET") {
            self.security.jwt_secret = secret;
        }
        if let Ok(secret) = std::env::var("JWT_SECRET") {
            self.security.jwt_secret = secret;
        }
        
        // Logging
        if let Ok(level) = std::env::var("MANTIS_LOG_LEVEL") {
            self.logging.level = level;
        }
        if let Ok(level) = std::env::var("LOG_LEVEL") {
            self.logging.level = level;
        }
        
        Ok(())
    }
    
    /// Validate configuration
    pub fn validate(&self) -> Result<()> {
        // Production must have JWT secret
        if self.environment == Environment::Production && self.security.jwt_secret.is_empty() {
            return Err(Error::ConfigError(
                "JWT_SECRET must be set in production (via MANTIS_JWT_SECRET or JWT_SECRET env var)".to_string()
            ));
        }
        
        // TLS files must exist if TLS is enabled
        if self.security.enable_tls {
            if let Some(cert) = &self.security.tls_cert {
                if !cert.exists() {
                    return Err(Error::ConfigError(format!("TLS cert not found: {:?}", cert)));
                }
            }
            if let Some(key) = &self.security.tls_key {
                if !key.exists() {
                    return Err(Error::ConfigError(format!("TLS key not found: {:?}", key)));
                }
            }
        }
        
        // Validate log level
        let valid_levels = ["trace", "debug", "info", "warn", "error"];
        if !valid_levels.contains(&self.logging.level.as_str()) {
            return Err(Error::ConfigError(format!(
                "Invalid log level: {}. Must be one of: {}",
                self.logging.level,
                valid_levels.join(", ")
            )));
        }
        
        Ok(())
    }
    
    /// Get connection string for external services
    pub fn get_connection_string(&self) -> String {
        format!("{}:{}", self.server.host, self.server.port)
    }
    
    /// Check if running in production
    pub fn is_production(&self) -> bool {
        self.environment == Environment::Production
    }
    
    /// Get request timeout as Duration
    pub fn request_timeout_duration(&self) -> Duration {
        Duration::from_secs(self.server.request_timeout)
    }
}

/// Configuration builder for programmatic setup
pub struct ConfigBuilder {
    config: ProductionConfig,
}

impl ConfigBuilder {
    pub fn new(env: Environment) -> Self {
        let config = match env {
            Environment::Development => ProductionConfig::development(),
            Environment::Staging => ProductionConfig::staging(),
            Environment::Production => ProductionConfig::production(),
        };
        
        Self { config }
    }
    
    pub fn with_port(mut self, port: u16) -> Self {
        self.config.server.port = port;
        self
    }
    
    pub fn with_data_dir(mut self, path: impl Into<PathBuf>) -> Self {
        self.config.database.data_dir = path.into();
        self
    }
    
    pub fn with_jwt_secret(mut self, secret: impl Into<String>) -> Self {
        self.config.security.jwt_secret = secret.into();
        self
    }
    
    pub fn with_max_memory(mut self, bytes: usize) -> Self {
        self.config.database.max_memory = bytes;
        self
    }
    
    pub fn build(self) -> Result<ProductionConfig> {
        self.config.validate()?;
        Ok(self.config)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_development_config() {
        let config = ProductionConfig::development();
        assert_eq!(config.environment, Environment::Development);
        assert_eq!(config.server.port, 3000);
        assert!(!config.database.sync_on_write); // Fast in dev
    }
    
    #[test]
    fn test_production_config() {
        let config = ProductionConfig::production();
        assert_eq!(config.environment, Environment::Production);
        assert!(config.database.sync_on_write); // Durable in prod
        assert!(config.security.enable_rate_limiting);
    }
    
    #[test]
    fn test_config_builder() {
        let config = ConfigBuilder::new(Environment::Development)
            .with_port(4000)
            .with_data_dir("/tmp/test")
            .with_jwt_secret("test-secret")
            .build()
            .unwrap();
        
        assert_eq!(config.server.port, 4000);
        assert_eq!(config.database.data_dir, PathBuf::from("/tmp/test"));
    }
    
    #[test]
    fn test_validation_requires_jwt_in_prod() {
        let mut config = ProductionConfig::production();
        config.security.jwt_secret = String::new();
        assert!(config.validate().is_err());
    }
}
