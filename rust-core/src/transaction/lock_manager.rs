// Lock Manager with Deadlock Detection
use super::types::*;
use crate::error::MantisError;
use std::collections::{HashMap, HashSet, VecDeque};
use std::sync::Arc;
use std::time::{Duration, Instant};
use parking_lot::RwLock;

pub struct LockManager {
    // Maps lock keys to list of holders and waiters
    locks: Arc<RwLock<HashMap<LockKey, LockEntry>>>,
    
    // Maps transaction IDs to their lock requests
    txn_locks: Arc<RwLock<HashMap<TransactionId, HashSet<LockKey>>>>,
    
    // Deadlock detection interval
    deadlock_check_interval: Duration,
    last_deadlock_check: Arc<RwLock<Instant>>,
}

struct LockEntry {
    holders: Vec<LockHolder>,
    waiters: VecDeque<LockWaiter>,
}

struct LockHolder {
    txn_id: TransactionId,
    mode: LockMode,
    acquired_at: Instant,
}

struct LockWaiter {
    txn_id: TransactionId,
    mode: LockMode,
    requested_at: Instant,
}

impl LockManager {
    pub fn new() -> Self {
        LockManager {
            locks: Arc::new(RwLock::new(HashMap::new())),
            txn_locks: Arc::new(RwLock::new(HashMap::new())),
            deadlock_check_interval: Duration::from_secs(1),
            last_deadlock_check: Arc::new(RwLock::new(Instant::now())),
        }
    }
    
    pub fn acquire_lock(
        &self,
        txn_id: TransactionId,
        key: LockKey,
        mode: LockMode,
        timeout: Duration,
    ) -> Result<(), MantisError> {
        let start = Instant::now();
        
        loop {
            // Try to acquire the lock
            if self.try_acquire_lock(txn_id, key.clone(), mode)? {
                // Track this lock for the transaction
                self.txn_locks
                    .write()
                    .entry(txn_id)
                    .or_insert_with(HashSet::new)
                    .insert(key);
                return Ok(());
            }
            
            // Check for timeout
            if start.elapsed() > timeout {
                return Err(MantisError::LockTimeout(format!(
                    "Failed to acquire lock on {:?} within {:?}",
                    key, timeout
                )));
            }
            
            // Check for deadlocks periodically
            if self.should_check_deadlock() {
                if let Some(victim) = self.detect_deadlock(txn_id)? {
                    if victim == txn_id {
                        return Err(MantisError::DeadlockDetected(format!(
                            "Transaction {} is part of a deadlock cycle",
                            txn_id
                        )));
                    }
                }
            }
            
            // Wait a bit before retrying
            std::thread::sleep(Duration::from_micros(100));
        }
    }
    
    fn try_acquire_lock(
        &self,
        txn_id: TransactionId,
        key: LockKey,
        mode: LockMode,
    ) -> Result<bool, MantisError> {
        let mut locks = self.locks.write();
        let entry = locks.entry(key).or_insert_with(|| LockEntry {
            holders: Vec::new(),
            waiters: VecDeque::new(),
        });
        
        // Check if this transaction already holds the lock
        if entry.holders.iter().any(|h| h.txn_id == txn_id) {
            return Ok(true);
        }
        
        // Check if lock is compatible with current holders
        let compatible = entry.holders.iter().all(|h| mode.is_compatible(&h.mode));
        
        if compatible && entry.waiters.is_empty() {
            // Grant the lock
            entry.holders.push(LockHolder {
                txn_id,
                mode,
                acquired_at: Instant::now(),
            });
            Ok(true)
        } else {
            // Add to wait queue if not already there
            if !entry.waiters.iter().any(|w| w.txn_id == txn_id) {
                entry.waiters.push_back(LockWaiter {
                    txn_id,
                    mode,
                    requested_at: Instant::now(),
                });
            }
            Ok(false)
        }
    }
    
    pub fn release_lock(&self, txn_id: TransactionId, key: &LockKey) -> Result<(), MantisError> {
        let mut locks = self.locks.write();
        
        if let Some(entry) = locks.get_mut(key) {
            // Remove this transaction from holders
            entry.holders.retain(|h| h.txn_id != txn_id);
            
            // Try to grant lock to waiters
            self.try_grant_to_waiters(entry);
            
            // Clean up empty entries
            if entry.holders.is_empty() && entry.waiters.is_empty() {
                locks.remove(key);
            }
        }
        
        // Remove from transaction's lock set
        if let Some(txn_locks) = self.txn_locks.write().get_mut(&txn_id) {
            txn_locks.remove(key);
        }
        
        Ok(())
    }
    
    pub fn release_all_locks(&self, txn_id: TransactionId) -> Result<(), MantisError> {
        // Get all locks held by this transaction
        let keys: Vec<LockKey> = {
            let txn_locks = self.txn_locks.read();
            txn_locks
                .get(&txn_id)
                .map(|locks| locks.iter().cloned().collect())
                .unwrap_or_default()
        };
        
        // Release each lock
        for key in keys {
            self.release_lock(txn_id, &key)?;
        }
        
        // Clean up transaction entry
        self.txn_locks.write().remove(&txn_id);
        
        Ok(())
    }
    
    fn try_grant_to_waiters(&self, entry: &mut LockEntry) {
        while let Some(waiter) = entry.waiters.front() {
            let compatible = entry
                .holders
                .iter()
                .all(|h| waiter.mode.is_compatible(&h.mode));
            
            if compatible {
                let waiter = entry.waiters.pop_front().unwrap();
                entry.holders.push(LockHolder {
                    txn_id: waiter.txn_id,
                    mode: waiter.mode,
                    acquired_at: Instant::now(),
                });
            } else {
                break;
            }
        }
    }
    
    fn should_check_deadlock(&self) -> bool {
        let mut last_check = self.last_deadlock_check.write();
        if last_check.elapsed() > self.deadlock_check_interval {
            *last_check = Instant::now();
            true
        } else {
            false
        }
    }
    
    fn detect_deadlock(&self, txn_id: TransactionId) -> Result<Option<TransactionId>, MantisError> {
        // Build wait-for graph
        let wait_for_graph = self.build_wait_for_graph();
        
        // Detect cycles using DFS
        let mut visited = HashSet::new();
        let mut rec_stack = HashSet::new();
        
        if self.has_cycle_dfs(txn_id, &wait_for_graph, &mut visited, &mut rec_stack) {
            // Choose victim (youngest transaction)
            Ok(Some(self.choose_deadlock_victim(&rec_stack)))
        } else {
            Ok(None)
        }
    }
    
    fn build_wait_for_graph(&self) -> HashMap<TransactionId, Vec<TransactionId>> {
        let mut graph: HashMap<TransactionId, Vec<TransactionId>> = HashMap::new();
        let locks = self.locks.read();
        
        for entry in locks.values() {
            // Each waiter waits for all holders
            for waiter in &entry.waiters {
                let waiting_for: Vec<TransactionId> = entry
                    .holders
                    .iter()
                    .map(|h| h.txn_id)
                    .collect();
                
                graph.entry(waiter.txn_id)
                    .or_insert_with(Vec::new)
                    .extend(waiting_for);
            }
        }
        
        graph
    }
    
    fn has_cycle_dfs(
        &self,
        node: TransactionId,
        graph: &HashMap<TransactionId, Vec<TransactionId>>,
        visited: &mut HashSet<TransactionId>,
        rec_stack: &mut HashSet<TransactionId>,
    ) -> bool {
        visited.insert(node);
        rec_stack.insert(node);
        
        if let Some(neighbors) = graph.get(&node) {
            for &neighbor in neighbors {
                if !visited.contains(&neighbor) {
                    if self.has_cycle_dfs(neighbor, graph, visited, rec_stack) {
                        return true;
                    }
                } else if rec_stack.contains(&neighbor) {
                    return true;
                }
            }
        }
        
        rec_stack.remove(&node);
        false
    }
    
    fn choose_deadlock_victim(&self, cycle: &HashSet<TransactionId>) -> TransactionId {
        // Choose the transaction with the highest ID (youngest)
        *cycle.iter().max().unwrap()
    }
}

impl Default for LockManager {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_lock_acquisition() {
        let lm = LockManager::new();
        let txn1 = TransactionId::new();
        let key = LockKey::new("test_table", b"key1".to_vec());
        
        let result = lm.acquire_lock(
            txn1,
            key.clone(),
            LockMode::Shared,
            Duration::from_secs(1),
        );
        
        assert!(result.is_ok());
    }
    
    #[test]
    fn test_lock_release() {
        let lm = LockManager::new();
        let txn1 = TransactionId::new();
        let key = LockKey::new("test_table", b"key1".to_vec());
        
        lm.acquire_lock(txn1, key.clone(), LockMode::Shared, Duration::from_secs(1))
            .unwrap();
        
        let result = lm.release_lock(txn1, &key);
        assert!(result.is_ok());
    }
}
