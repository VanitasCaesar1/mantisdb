// Buffer Pool Manager
use std::collections::HashMap;
use parking_lot::RwLock;

pub struct BufferPool {
    pages: RwLock<HashMap<u64, Vec<u8>>>,
    capacity: usize,
}

impl BufferPool {
    pub fn new(capacity: usize) -> Self {
        BufferPool {
            pages: RwLock::new(HashMap::new()),
            capacity,
        }
    }
}
