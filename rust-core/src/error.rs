use thiserror::Error;

#[derive(Error, Debug)]
pub enum Error {
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
}

pub type Result<T> = std::result::Result<T, Error>;

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
