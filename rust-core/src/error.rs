use thiserror::Error;
use std::collections::HashMap;
use std::fmt;

/// Error codes for structured error reporting
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ErrorCode {
    // Storage errors (1000-1999)
    KeyNotFound = 1001,
    StorageFull = 1002,
    DiskFull = 1003,
    CorruptedData = 1004,
    
    // Cache errors (2000-2999)
    CacheFull = 2001,
    CacheMiss = 2002,
    
    // Connection errors (3000-3999)
    PoolExhausted = 3001,
    PoolClosed = 3002,
    ConnectionTimeout = 3003,
    ConnectionRefused = 3004,
    
    // Query errors (4000-4999)
    ParseError = 4001,
    InvalidQuery = 4002,
    ConstraintViolation = 4003,
    
    // Vector errors (5000-5999)
    DimensionMismatch = 5001,
    InvalidVector = 5002,
    
    // General errors (9000-9999)
    InvalidData = 9001,
    Timeout = 9002,
    InternalError = 9999,
}

impl fmt::Display for ErrorCode {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        write!(f, "MDB{:04}", *self as u32)
    }
}

/// Structured error with context, hints, and documentation
#[derive(Debug, Clone)]
pub struct DetailedError {
    pub code: ErrorCode,
    pub operation: String,
    pub message: String,
    pub context: HashMap<String, String>,
    pub hint: Option<String>,
    pub docs_url: Option<String>,
}

impl DetailedError {
    pub fn new(code: ErrorCode, operation: impl Into<String>, message: impl Into<String>) -> Self {
        Self {
            code,
            operation: operation.into(),
            message: message.into(),
            context: HashMap::new(),
            hint: None,
            docs_url: Some(format!("https://docs.mantisdb.io/errors/{}", code as u32)),
        }
    }
    
    pub fn with_context(mut self, key: impl Into<String>, value: impl Into<String>) -> Self {
        self.context.insert(key.into(), value.into());
        self
    }
    
    pub fn with_hint(mut self, hint: impl Into<String>) -> Self {
        self.hint = Some(hint.into());
        self
    }
    
    pub fn with_docs(mut self, url: impl Into<String>) -> Self {
        self.docs_url = Some(url.into());
        self
    }
}

impl fmt::Display for DetailedError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        writeln!(f, "\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")?;
        writeln!(f, "❌ MantisDB Error [{}]", self.code)?;
        writeln!(f, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")?;
        writeln!(f, "\n📍 Operation: {}", self.operation)?;
        writeln!(f, "💬 Message: {}", self.message)?;
        
        if !self.context.is_empty() {
            writeln!(f, "\n📋 Context:")?;
            for (key, value) in &self.context {
                writeln!(f, "   • {}: {}", key, value)?;
            }
        }
        
        if let Some(hint) = &self.hint {
            writeln!(f, "\n💡 Hint: {}", hint)?;
        }
        
        if let Some(url) = &self.docs_url {
            writeln!(f, "\n📚 Documentation: {}", url)?;
        }
        
        writeln!(f, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")?;
        Ok(())
    }
}

impl std::error::Error for DetailedError {}

#[derive(Error, Debug)]
pub enum Error {
    #[error("{0}")]
    Detailed(DetailedError),
    
    #[error("Key not found: {0}")]
    KeyNotFound(String),

    #[error("Not found")]
    NotFound,

    #[error("Serialization error: {0}")]
    SerializationError(String),

    #[error("IO error: {0}")]
    IoError(#[from] std::io::Error),

    #[error("IO: {0}")]
    Io(String),

    #[error("Cache full")]
    CacheFull,

    #[error("Invalid data")]
    InvalidData,

    #[error("Storage error: {0}")]
    StorageError(String),

    #[error("Connection pool exhausted")]
    PoolExhausted,

    #[error("Connection pool closed")]
    PoolClosed,

    #[error("Request timeout")]
    Timeout,
    
    #[error("Validation error: {0}")]
    ValidationError(String),
}

pub type Result<T> = std::result::Result<T, Error>;

// Helper functions for creating detailed errors
impl Error {
    pub fn key_not_found_detailed(key: &str, storage: &str) -> Self {
        Error::Detailed(
            DetailedError::new(
                ErrorCode::KeyNotFound,
                "get",
                format!("Key '{}' not found in storage", key)
            )
            .with_context("key", key)
            .with_context("storage", storage)
            .with_hint("Verify the key exists using 'EXISTS' or check for typos")
        )
    }
    
    pub fn disk_full_detailed(path: &str, used: u64, total: u64) -> Self {
        let percent = (used as f64 / total as f64 * 100.0) as u32;
        Error::Detailed(
            DetailedError::new(
                ErrorCode::DiskFull,
                "write",
                format!("Disk is {}% full", percent)
            )
            .with_context("path", path)
            .with_context("used_gb", format!("{}", used / 1_000_000_000))
            .with_context("total_gb", format!("{}", total / 1_000_000_000))
            .with_hint("Enable compression, increase disk space, or set up data retention policies")
        )
    }
    
    pub fn pool_exhausted_detailed(current: usize, max: usize, wait_time_ms: u64) -> Self {
        Error::Detailed(
            DetailedError::new(
                ErrorCode::PoolExhausted,
                "acquire_connection",
                "Connection pool exhausted"
            )
            .with_context("current_connections", current.to_string())
            .with_context("max_connections", max.to_string())
            .with_context("wait_time_ms", wait_time_ms.to_string())
            .with_hint("Increase max_connections in config or implement connection pooling in your app")
        )
    }
    
    pub fn dimension_mismatch_detailed(expected: usize, got: usize, vector_id: &str) -> Self {
        Error::Detailed(
            DetailedError::new(
                ErrorCode::DimensionMismatch,
                "vector_insert",
                "Vector dimension mismatch"
            )
            .with_context("expected_dimension", expected.to_string())
            .with_context("got_dimension", got.to_string())
            .with_context("vector_id", vector_id)
            .with_hint("All vectors in a collection must have the same dimension")
        )
    }
}

impl From<DetailedError> for Error {
    fn from(e: DetailedError) -> Self {
        Error::Detailed(e)
    }
}

// Extended error types for database engine
#[derive(Error, Debug)]
pub enum MantisError {
    #[error("Storage error: {0}")]
    StorageError(String),
    
    #[error("IO error: {0}")]
    IoError(String),
    
    #[error("Serialization error: {0}")]
    SerializationError(String),
    
    #[error("Parse error: {0}")]
    ParseError(String),
    
    #[error("Transaction error: {0}")]
    TransactionError(String),
    
    #[error("Lock timeout: {0}")]
    LockTimeout(String),
    
    #[error("Deadlock detected: {0}")]
    DeadlockDetected(String),
    
    #[error("WAL error: {0}")]
    WalError(String),
    
    #[error("Optimizer error: {0}")]
    OptimizerError(String),
    
    #[error("Executor error: {0}")]
    ExecutorError(String),
    
    #[error("Constraint violation: {0}")]
    ConstraintViolation(String),
    
    #[error("Not found: {0}")]
    NotFound(String),
    
    #[error("Already exists: {0}")]
    AlreadyExists(String),
    
    #[error("Internal error: {0}")]
    InternalError(String),

    #[error("Connection pool closed")]
    PoolClosed,

    #[error("Request timeout")]
    Timeout,
}
