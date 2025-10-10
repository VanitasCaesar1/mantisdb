// Transaction Implementation with MVCC
use super::types::*;
use crate::error::MantisError;
use std::collections::HashMap;
use std::sync::Arc;
use std::time::Instant;
use parking_lot::RwLock;

pub struct Transaction {
    pub id: TransactionId,
    pub isolation_level: IsolationLevel,
    pub state: Arc<RwLock<TransactionState>>,
    pub start_time: Instant,
    pub read_timestamp: u64,
    pub write_timestamp: u64,
    
    // Read and write sets for conflict detection
    pub read_set: Arc<RwLock<HashMap<Vec<u8>, u64>>>,
    pub write_set: Arc<RwLock<HashMap<Vec<u8>, WriteIntent>>>,
    
    // Locks held by this transaction
    pub locks_held: Arc<RwLock<Vec<LockKey>>>,
}

impl Transaction {
    pub fn new(isolation_level: IsolationLevel) -> Self {
        let now = Instant::now();
        let timestamp = now.elapsed().as_nanos() as u64;
        
        Transaction {
            id: TransactionId::new(),
            isolation_level,
            state: Arc::new(RwLock::new(TransactionState::Active)),
            start_time: now,
            read_timestamp: timestamp,
            write_timestamp: timestamp,
            read_set: Arc::new(RwLock::new(HashMap::new())),
            write_set: Arc::new(RwLock::new(HashMap::new())),
            locks_held: Arc::new(RwLock::new(Vec::new())),
        }
    }
    
    pub fn is_active(&self) -> bool {
        *self.state.read() == TransactionState::Active
    }
    
    pub fn is_committed(&self) -> bool {
        *self.state.read() == TransactionState::Committed
    }
    
    pub fn is_aborted(&self) -> bool {
        *self.state.read() == TransactionState::Aborted
    }
    
    pub fn add_read(&self, key: Vec<u8>, version: u64) {
        self.read_set.write().insert(key, version);
    }
    
    pub fn add_write(&self, key: Vec<u8>, value: Option<Vec<u8>>) {
        let intent = WriteIntent {
            txn_id: self.id,
            key: key.clone(),
            value,
            timestamp: self.write_timestamp,
        };
        self.write_set.write().insert(key, intent);
    }
    
    pub fn add_lock(&self, lock_key: LockKey) {
        self.locks_held.write().push(lock_key);
    }
    
    pub fn prepare(&self) -> Result<(), MantisError> {
        let mut state = self.state.write();
        if *state != TransactionState::Active {
            return Err(MantisError::TransactionError(
                "Transaction is not active".to_string()
            ));
        }
        *state = TransactionState::Preparing;
        Ok(())
    }
    
    pub fn commit(&self) -> Result<(), MantisError> {
        let mut state = self.state.write();
        match *state {
            TransactionState::Active | TransactionState::Prepared => {
                *state = TransactionState::Committing;
                // Actual commit logic would go here
                *state = TransactionState::Committed;
                Ok(())
            }
            _ => Err(MantisError::TransactionError(
                format!("Cannot commit transaction in state {:?}", *state)
            )),
        }
    }
    
    pub fn abort(&self) -> Result<(), MantisError> {
        let mut state = self.state.write();
        if *state == TransactionState::Committed {
            return Err(MantisError::TransactionError(
                "Cannot abort committed transaction".to_string()
            ));
        }
        *state = TransactionState::Aborting;
        // Rollback logic would go here
        *state = TransactionState::Aborted;
        Ok(())
    }
    
    pub fn get_write_intents(&self) -> Vec<WriteIntent> {
        self.write_set.read().values().cloned().collect()
    }
    
    pub fn has_conflict_with(&self, other: &Transaction) -> bool {
        let my_reads = self.read_set.read();
        let other_writes = other.write_set.read();
        
        // Check for read-write conflicts
        for read_key in my_reads.keys() {
            if other_writes.contains_key(read_key) {
                return true;
            }
        }
        
        let my_writes = self.write_set.read();
        let other_reads = other.read_set.read();
        
        // Check for write-read conflicts
        for write_key in my_writes.keys() {
            if other_reads.contains_key(write_key) {
                return true;
            }
        }
        
        // Check for write-write conflicts
        for write_key in my_writes.keys() {
            if other_writes.contains_key(write_key) {
                return true;
            }
        }
        
        false
    }
}

impl Drop for Transaction {
    fn drop(&mut self) {
        // Ensure transaction is either committed or aborted
        let state = *self.state.read();
        if state == TransactionState::Active {
            let _ = self.abort();
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_transaction_lifecycle() {
        let txn = Transaction::new(IsolationLevel::ReadCommitted);
        assert!(txn.is_active());
        
        txn.commit().unwrap();
        assert!(txn.is_committed());
    }
    
    #[test]
    fn test_transaction_abort() {
        let txn = Transaction::new(IsolationLevel::ReadCommitted);
        txn.abort().unwrap();
        assert!(txn.is_aborted());
    }
    
    #[test]
    fn test_conflict_detection() {
        let txn1 = Transaction::new(IsolationLevel::ReadCommitted);
        let txn2 = Transaction::new(IsolationLevel::ReadCommitted);
        
        let key = b"test_key".to_vec();
        txn1.add_read(key.clone(), 1);
        txn2.add_write(key, Some(b"value".to_vec()));
        
        assert!(txn1.has_conflict_with(&txn2));
    }
}
