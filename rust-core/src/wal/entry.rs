// WAL Entry Types
use serde::{Deserialize, Serialize};
use std::time::SystemTime;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WalEntry {
    pub lsn: LogSequenceNumber,
    pub txn_id: u64,
    pub timestamp: u64,
    pub entry_type: WalEntryType,
    pub checksum: u32,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize)]
pub struct LogSequenceNumber(pub u64);

impl LogSequenceNumber {
    pub fn new(value: u64) -> Self {
        LogSequenceNumber(value)
    }
    
    pub fn next(&self) -> Self {
        LogSequenceNumber(self.0 + 1)
    }
    
    pub fn as_u64(&self) -> u64 {
        self.0
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum WalEntryType {
    // Transaction control
    BeginTransaction,
    CommitTransaction,
    AbortTransaction,
    
    // Data operations
    Insert {
        table: String,
        key: Vec<u8>,
        value: Vec<u8>,
    },
    Update {
        table: String,
        key: Vec<u8>,
        old_value: Vec<u8>,
        new_value: Vec<u8>,
    },
    Delete {
        table: String,
        key: Vec<u8>,
        old_value: Vec<u8>,
    },
    
    // Checkpoint
    Checkpoint {
        lsn: LogSequenceNumber,
        active_txns: Vec<u64>,
    },
    
    // Schema operations
    CreateTable {
        name: String,
        schema: Vec<u8>,
    },
    DropTable {
        name: String,
    },
}

impl WalEntry {
    pub fn new(txn_id: u64, lsn: LogSequenceNumber, entry_type: WalEntryType) -> Self {
        let timestamp = SystemTime::now()
            .duration_since(SystemTime::UNIX_EPOCH)
            .unwrap()
            .as_micros() as u64;
        
        let mut entry = WalEntry {
            lsn,
            txn_id,
            timestamp,
            entry_type,
            checksum: 0,
        };
        
        entry.checksum = entry.calculate_checksum();
        entry
    }
    
    pub fn calculate_checksum(&self) -> u32 {
        use std::hash::{Hash, Hasher};
        use std::collections::hash_map::DefaultHasher;
        
        let mut hasher = DefaultHasher::new();
        self.lsn.0.hash(&mut hasher);
        self.txn_id.hash(&mut hasher);
        self.timestamp.hash(&mut hasher);
        
        hasher.finish() as u32
    }
    
    pub fn verify_checksum(&self) -> bool {
        let calculated = self.calculate_checksum();
        calculated == self.checksum
    }
    
    pub fn serialize(&self) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
        Ok(bincode::serialize(self)?)
    }
    
    pub fn deserialize(data: &[u8]) -> Result<Self, Box<dyn std::error::Error>> {
        Ok(bincode::deserialize(data)?)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_wal_entry_checksum() {
        let entry = WalEntry::new(
            1,
            LogSequenceNumber::new(100),
            WalEntryType::BeginTransaction,
        );
        
        assert!(entry.verify_checksum());
    }
    
    #[test]
    fn test_wal_entry_serialization() {
        let entry = WalEntry::new(
            1,
            LogSequenceNumber::new(100),
            WalEntryType::Insert {
                table: "users".to_string(),
                key: b"key1".to_vec(),
                value: b"value1".to_vec(),
            },
        );
        
        let serialized = entry.serialize().unwrap();
        let deserialized = WalEntry::deserialize(&serialized).unwrap();
        
        assert_eq!(entry.lsn, deserialized.lsn);
        assert_eq!(entry.txn_id, deserialized.txn_id);
    }
}
