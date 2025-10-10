// Storage Engine Types
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StorageConfig {
    pub data_dir: String,
    pub page_size: usize,
    pub buffer_pool_size: usize,
}

impl Default for StorageConfig {
    fn default() -> Self {
        StorageConfig {
            data_dir: "./data".to_string(),
            page_size: 8192,
            buffer_pool_size: 1024,
        }
    }
}
