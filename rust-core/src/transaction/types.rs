// Transaction System Types
use serde::{Deserialize, Serialize};
use std::fmt;
use std::sync::atomic::{AtomicU64, Ordering};

static TRANSACTION_ID_COUNTER: AtomicU64 = AtomicU64::new(1);

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, PartialOrd, Ord, Serialize, Deserialize)]
pub struct TransactionId(pub u64);

impl TransactionId {
    pub fn new() -> Self {
        TransactionId(TRANSACTION_ID_COUNTER.fetch_add(1, Ordering::SeqCst))
    }
    
    pub fn as_u64(&self) -> u64 {
        self.0
    }
}

impl Default for TransactionId {
    fn default() -> Self {
        Self::new()
    }
}

impl fmt::Display for TransactionId {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "txn:{}", self.0)
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum IsolationLevel {
    ReadUncommitted,
    ReadCommitted,
    RepeatableRead,
    Serializable,
}

impl Default for IsolationLevel {
    fn default() -> Self {
        IsolationLevel::ReadCommitted
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum TransactionState {
    Active,
    Preparing,
    Prepared,
    Committing,
    Committed,
    Aborting,
    Aborted,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum LockMode {
    Shared,
    Exclusive,
    IntentShared,
    IntentExclusive,
    SharedIntentExclusive,
}

impl LockMode {
    pub fn is_compatible(&self, other: &LockMode) -> bool {
        use LockMode::*;
        match (self, other) {
            (Shared, Shared) => true,
            (Shared, IntentShared) => true,
            (IntentShared, Shared) => true,
            (IntentShared, IntentShared) => true,
            (IntentShared, IntentExclusive) => true,
            (IntentExclusive, IntentShared) => true,
            _ => false,
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub struct LockKey {
    pub table: String,
    pub key: Vec<u8>,
}

impl LockKey {
    pub fn new(table: impl Into<String>, key: Vec<u8>) -> Self {
        LockKey {
            table: table.into(),
            key,
        }
    }
    
    pub fn table_lock(table: impl Into<String>) -> Self {
        LockKey {
            table: table.into(),
            key: Vec::new(),
        }
    }
}

#[derive(Debug, Clone)]
pub struct LockRequest {
    pub txn_id: TransactionId,
    pub key: LockKey,
    pub mode: LockMode,
    pub timestamp: std::time::Instant,
}

#[derive(Debug, Clone)]
pub struct WriteIntent {
    pub txn_id: TransactionId,
    pub key: Vec<u8>,
    pub value: Option<Vec<u8>>, // None for deletes
    pub timestamp: u64,
}
