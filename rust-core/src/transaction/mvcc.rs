// Multi-Version Concurrency Control (MVCC)
use super::types::*;
use std::collections::HashMap;
use std::sync::Arc;
use parking_lot::RwLock;

#[derive(Debug, Clone)]
pub struct MvccVersion {
    pub value: Vec<u8>,
    pub created_by: TransactionId,
    pub deleted_by: Option<TransactionId>,
    pub timestamp: u64,
}

pub struct MvccStore {
    // Key -> List of versions (sorted by timestamp)
    versions: Arc<RwLock<HashMap<Vec<u8>, Vec<MvccVersion>>>>,
}

impl MvccStore {
    pub fn new() -> Self {
        MvccStore {
            versions: Arc::new(RwLock::new(HashMap::new())),
        }
    }
    
    pub fn read(&self, key: &[u8], txn_id: TransactionId, read_timestamp: u64) -> Option<Vec<u8>> {
        let versions = self.versions.read();
        let key_versions = versions.get(key)?;
        
        // Find the latest version visible to this transaction
        for version in key_versions.iter().rev() {
            if version.timestamp <= read_timestamp {
                // Check if deleted
                if let Some(deleted_by) = version.deleted_by {
                    if deleted_by.as_u64() > txn_id.as_u64() {
                        return Some(version.value.clone());
                    }
                } else {
                    return Some(version.value.clone());
                }
            }
        }
        
        None
    }
    
    pub fn write(&self, key: Vec<u8>, value: Vec<u8>, txn_id: TransactionId, timestamp: u64) {
        let mut versions = self.versions.write();
        let key_versions = versions.entry(key).or_insert_with(Vec::new);
        
        key_versions.push(MvccVersion {
            value,
            created_by: txn_id,
            deleted_by: None,
            timestamp,
        });
        
        // Keep versions sorted by timestamp
        key_versions.sort_by_key(|v| v.timestamp);
    }
    
    pub fn delete(&self, key: &[u8], txn_id: TransactionId, _timestamp: u64) {
        let mut versions = self.versions.write();
        if let Some(key_versions) = versions.get_mut(key) {
            // Mark the latest version as deleted
            if let Some(latest) = key_versions.last_mut() {
                latest.deleted_by = Some(txn_id);
            }
        }
    }
    
    pub fn vacuum(&self, min_active_timestamp: u64) {
        let mut versions = self.versions.write();
        
        for key_versions in versions.values_mut() {
            // Remove versions that are no longer visible to any active transaction
            key_versions.retain(|v| {
                v.timestamp >= min_active_timestamp || v.deleted_by.is_none()
            });
        }
        
        // Remove empty entries
        versions.retain(|_, v| !v.is_empty());
    }
}

impl Default for MvccStore {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_mvcc_read_write() {
        let store = MvccStore::new();
        let txn1 = TransactionId::new();
        
        store.write(b"key1".to_vec(), b"value1".to_vec(), txn1, 100);
        
        let result = store.read(b"key1", txn1, 100);
        assert_eq!(result, Some(b"value1".to_vec()));
    }
}
