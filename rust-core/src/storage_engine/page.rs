// Page Management
pub const PAGE_SIZE: usize = 8192;

#[derive(Debug, Clone)]
pub struct Page {
    pub id: u64,
    pub data: Vec<u8>,
    pub dirty: bool,
}

impl Page {
    pub fn new(id: u64) -> Self {
        Page {
            id,
            data: vec![0; PAGE_SIZE],
            dirty: false,
        }
    }
}
