//! Buffer Pool Manager with LRU eviction
//! Manages in-memory pages for disk-backed storage

use parking_lot::RwLock;
use std::collections::HashMap;
use std::sync::Arc;

type PageId = u64;

/// Buffer pool with LRU eviction for page caching
pub struct BufferPool {
    inner: Arc<RwLock<BufferPoolInner>>,
}

struct BufferPoolInner {
    pages: HashMap<PageId, CachedPage>,
    capacity: usize,
    clock_hand: usize, // For clock/second-chance eviction
    page_list: Vec<PageId>, // For eviction policy
}

struct CachedPage {
    data: Vec<u8>,
    dirty: bool,
    reference_bit: bool, // For clock algorithm
}

impl BufferPool {
    /// Create a new buffer pool with given capacity (in pages)
    pub fn new(capacity: usize) -> Self {
        BufferPool {
            inner: Arc::new(RwLock::new(BufferPoolInner {
                pages: HashMap::with_capacity(capacity),
                capacity,
                clock_hand: 0,
                page_list: Vec::with_capacity(capacity),
            })),
        }
    }
    
    /// Get a page from the buffer pool
    pub fn get(&self, page_id: PageId) -> Option<Vec<u8>> {
        let mut inner = self.inner.write();
        
        if let Some(page) = inner.pages.get_mut(&page_id) {
            page.reference_bit = true; // Mark as recently used
            Some(page.data.clone())
        } else {
            None
        }
    }
    
    /// Put a page into the buffer pool
    pub fn put(&self, page_id: PageId, data: Vec<u8>, dirty: bool) {
        let mut inner = self.inner.write();
        
        // If at capacity, evict using clock algorithm
        if inner.pages.len() >= inner.capacity && !inner.pages.contains_key(&page_id) {
            inner.evict_page();
        }
        
        // Insert or update page
        if !inner.pages.contains_key(&page_id) {
            inner.page_list.push(page_id);
        }
        
        inner.pages.insert(page_id, CachedPage {
            data,
            dirty,
            reference_bit: true,
        });
    }
    
    /// Mark a page as dirty (modified)
    pub fn mark_dirty(&self, page_id: PageId) {
        let mut inner = self.inner.write();
        if let Some(page) = inner.pages.get_mut(&page_id) {
            page.dirty = true;
        }
    }
    
    /// Get all dirty pages (for flushing)
    pub fn get_dirty_pages(&self) -> Vec<(PageId, Vec<u8>)> {
        let inner = self.inner.read();
        inner.pages.iter()
            .filter(|(_, page)| page.dirty)
            .map(|(id, page)| (*id, page.data.clone()))
            .collect()
    }
    
    /// Clear dirty bit for a page (after flushing)
    pub fn clear_dirty(&self, page_id: PageId) {
        let mut inner = self.inner.write();
        if let Some(page) = inner.pages.get_mut(&page_id) {
            page.dirty = false;
        }
    }
    
    /// Flush all dirty pages
    pub fn flush_all(&self) -> Vec<(PageId, Vec<u8>)> {
        let mut inner = self.inner.write();
        let dirty_pages: Vec<_> = inner.pages.iter()
            .filter(|(_, page)| page.dirty)
            .map(|(id, page)| (*id, page.data.clone()))
            .collect();
        
        // Clear dirty bits
        for (id, _) in &dirty_pages {
            if let Some(page) = inner.pages.get_mut(id) {
                page.dirty = false;
            }
        }
        
        dirty_pages
    }
    
    /// Clear the entire buffer pool
    pub fn clear(&self) {
        let mut inner = self.inner.write();
        inner.pages.clear();
        inner.page_list.clear();
        inner.clock_hand = 0;
    }
    
    /// Get buffer pool statistics
    pub fn stats(&self) -> BufferPoolStats {
        let inner = self.inner.read();
        BufferPoolStats {
            capacity: inner.capacity,
            used: inner.pages.len(),
            dirty_pages: inner.pages.values().filter(|p| p.dirty).count(),
        }
    }
}

impl BufferPoolInner {
    /// Evict a page using clock/second-chance algorithm
    fn evict_page(&mut self) {
        if self.page_list.is_empty() {
            return;
        }
        
        loop {
            let page_id = self.page_list[self.clock_hand];
            
            if let Some(page) = self.pages.get_mut(&page_id) {
                if page.reference_bit {
                    // Give second chance
                    page.reference_bit = false;
                } else {
                    // Evict this page
                    // Note: In production, we'd flush if dirty
                    self.pages.remove(&page_id);
                    self.page_list.remove(self.clock_hand);
                    
                    // Adjust clock hand
                    if self.clock_hand >= self.page_list.len() && !self.page_list.is_empty() {
                        self.clock_hand = 0;
                    }
                    
                    return;
                }
            }
            
            // Move clock hand
            self.clock_hand = (self.clock_hand + 1) % self.page_list.len().max(1);
        }
    }
}

impl Clone for BufferPool {
    fn clone(&self) -> Self {
        BufferPool {
            inner: Arc::clone(&self.inner),
        }
    }
}

#[derive(Debug, Clone)]
pub struct BufferPoolStats {
    pub capacity: usize,
    pub used: usize,
    pub dirty_pages: usize,
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_buffer_pool_basic() {
        let pool = BufferPool::new(3);
        
        // Insert pages
        pool.put(1, vec![1, 2, 3], false);
        pool.put(2, vec![4, 5, 6], true);
        
        // Get pages
        assert_eq!(pool.get(1), Some(vec![1, 2, 3]));
        assert_eq!(pool.get(2), Some(vec![4, 5, 6]));
        assert_eq!(pool.get(999), None);
        
        // Check stats
        let stats = pool.stats();
        assert_eq!(stats.used, 2);
        assert_eq!(stats.dirty_pages, 1);
    }
    
    #[test]
    fn test_buffer_pool_eviction() {
        let pool = BufferPool::new(2);
        
        pool.put(1, vec![1], false);
        pool.put(2, vec![2], false);
        pool.put(3, vec![3], false); // Should evict page 1 or 2
        
        let stats = pool.stats();
        assert_eq!(stats.used, 2); // Should still be at capacity
    }
    
    #[test]
    fn test_dirty_page_tracking() {
        let pool = BufferPool::new(10);
        
        pool.put(1, vec![1], false);
        pool.put(2, vec![2], true);
        pool.mark_dirty(1);
        
        let dirty = pool.get_dirty_pages();
        assert_eq!(dirty.len(), 2);
        
        pool.clear_dirty(1);
        let dirty = pool.get_dirty_pages();
        assert_eq!(dirty.len(), 1);
    }
}
