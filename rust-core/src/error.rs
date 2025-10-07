use thiserror::Error;

#[derive(Error, Debug)]
pub enum Error {
    #[error("Key not found: {0}")]
    KeyNotFound(String),
    
    #[error("Serialization error: {0}")]
    SerializationError(String),
    
    #[error("IO error: {0}")]
    IoError(#[from] std::io::Error),
    
    #[error("Cache full")]
    CacheFull,
    
    #[error("Invalid data")]
    InvalidData,
    
    #[error("Storage error: {0}")]
    StorageError(String),
}

pub type Result<T> = std::result::Result<T, Error>;
