// Isolation Level Implementation
use super::types::*;

pub struct IsolationManager;

impl IsolationManager {
    pub fn can_read(
        isolation_level: IsolationLevel,
        reader_txn: TransactionId,
        writer_txn: TransactionId,
        writer_committed: bool,
    ) -> bool {
        match isolation_level {
            IsolationLevel::ReadUncommitted => true,
            IsolationLevel::ReadCommitted => writer_committed,
            IsolationLevel::RepeatableRead => {
                writer_committed && writer_txn.as_u64() < reader_txn.as_u64()
            }
            IsolationLevel::Serializable => {
                writer_committed && writer_txn.as_u64() < reader_txn.as_u64()
            }
        }
    }
}
