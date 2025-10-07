//! FFI bindings for Go integration
//! Provides C-compatible interface for the Rust core

use std::ffi::{CStr, CString};
use std::os::raw::{c_char, c_int, c_void};
use std::ptr;
use std::sync::Arc;
use parking_lot::Mutex;
use std::collections::HashMap;

use crate::storage::LockFreeStorage;
use crate::cache::LockFreeCache;

// Global storage for handles
lazy_static::lazy_static! {
    static ref STORAGE_HANDLES: Mutex<HashMap<usize, Arc<LockFreeStorage>>> = Mutex::new(HashMap::new());
    static ref CACHE_HANDLES: Mutex<HashMap<usize, Arc<LockFreeCache>>> = Mutex::new(HashMap::new());
    static ref NEXT_HANDLE: Mutex<usize> = Mutex::new(1);
}

// Helper to get next handle ID
fn next_handle() -> usize {
    let mut handle = NEXT_HANDLE.lock();
    let id = *handle;
    *handle += 1;
    id
}

// ============================================================================
// Storage FFI
// ============================================================================

/// Create a new storage instance
#[no_mangle]
pub extern "C" fn storage_new() -> usize {
    let storage = Arc::new(LockFreeStorage::new());
    let handle = next_handle();
    STORAGE_HANDLES.lock().insert(handle, storage);
    handle
}

/// Free a storage instance
#[no_mangle]
pub extern "C" fn storage_free(handle: usize) {
    STORAGE_HANDLES.lock().remove(&handle);
}

/// Put a key-value pair
#[no_mangle]
pub extern "C" fn storage_put(
    handle: usize,
    key: *const c_char,
    key_len: usize,
    value: *const u8,
    value_len: usize,
) -> c_int {
    if key.is_null() || value.is_null() {
        return -1;
    }
    
    let storage = match STORAGE_HANDLES.lock().get(&handle) {
        Some(s) => Arc::clone(s),
        None => return -1,
    };
    
    let key_slice = unsafe { std::slice::from_raw_parts(key as *const u8, key_len) };
    let key_str = match std::str::from_utf8(key_slice) {
        Ok(s) => s.to_string(),
        Err(_) => return -1,
    };
    
    let value_slice = unsafe { std::slice::from_raw_parts(value, value_len) };
    
    match storage.put(key_str, value_slice.to_vec()) {
        Ok(_) => 0,
        Err(_) => -1,
    }
}

/// Get a value by key
#[no_mangle]
pub extern "C" fn storage_get(
    handle: usize,
    key: *const c_char,
    key_len: usize,
    value_out: *mut *mut u8,
    value_len_out: *mut usize,
) -> c_int {
    if key.is_null() || value_out.is_null() || value_len_out.is_null() {
        return -1;
    }
    
    let storage = match STORAGE_HANDLES.lock().get(&handle) {
        Some(s) => Arc::clone(s),
        None => return -1,
    };
    
    let key_slice = unsafe { std::slice::from_raw_parts(key as *const u8, key_len) };
    let key_str = match std::str::from_utf8(key_slice) {
        Ok(s) => s,
        Err(_) => return -1,
    };
    
    match storage.get(key_str) {
        Ok(value) => {
            let len = value.len();
            let ptr = value.as_ptr() as *mut u8;
            
            // Allocate and copy
            let buffer = unsafe {
                let buf = libc::malloc(len) as *mut u8;
                std::ptr::copy_nonoverlapping(ptr, buf, len);
                buf
            };
            
            unsafe {
                *value_out = buffer;
                *value_len_out = len;
            }
            0
        }
        Err(_) => -1,
    }
}

/// Delete a key
#[no_mangle]
pub extern "C" fn storage_delete(
    handle: usize,
    key: *const c_char,
    key_len: usize,
) -> c_int {
    if key.is_null() {
        return -1;
    }
    
    let storage = match STORAGE_HANDLES.lock().get(&handle) {
        Some(s) => Arc::clone(s),
        None => return -1,
    };
    
    let key_slice = unsafe { std::slice::from_raw_parts(key as *const u8, key_len) };
    let key_str = match std::str::from_utf8(key_slice) {
        Ok(s) => s,
        Err(_) => return -1,
    };
    
    match storage.delete(key_str) {
        Ok(_) => 0,
        Err(_) => -1,
    }
}

/// Get storage statistics
#[no_mangle]
pub extern "C" fn storage_stats(
    handle: usize,
    reads_out: *mut u64,
    writes_out: *mut u64,
    deletes_out: *mut u64,
) -> c_int {
    if reads_out.is_null() || writes_out.is_null() || deletes_out.is_null() {
        return -1;
    }
    
    let storage = match STORAGE_HANDLES.lock().get(&handle) {
        Some(s) => Arc::clone(s),
        None => return -1,
    };
    
    let stats = storage.stats();
    unsafe {
        *reads_out = stats.get_reads();
        *writes_out = stats.get_writes();
        *deletes_out = stats.get_deletes();
    }
    
    0
}

// ============================================================================
// Cache FFI
// ============================================================================

/// Create a new cache instance
#[no_mangle]
pub extern "C" fn cache_new(max_size: usize) -> usize {
    let cache = Arc::new(LockFreeCache::new(max_size));
    let handle = next_handle();
    CACHE_HANDLES.lock().insert(handle, cache);
    handle
}

/// Free a cache instance
#[no_mangle]
pub extern "C" fn cache_free(handle: usize) {
    CACHE_HANDLES.lock().remove(&handle);
}

/// Put a key-value pair in cache
#[no_mangle]
pub extern "C" fn cache_put(
    handle: usize,
    key: *const c_char,
    key_len: usize,
    value: *const u8,
    value_len: usize,
    ttl: u64,
) -> c_int {
    if key.is_null() || value.is_null() {
        return -1;
    }
    
    let cache = match CACHE_HANDLES.lock().get(&handle) {
        Some(c) => Arc::clone(c),
        None => return -1,
    };
    
    let key_slice = unsafe { std::slice::from_raw_parts(key as *const u8, key_len) };
    let key_str = match std::str::from_utf8(key_slice) {
        Ok(s) => s.to_string(),
        Err(_) => return -1,
    };
    
    let value_slice = unsafe { std::slice::from_raw_parts(value, value_len) };
    
    match cache.put(key_str, value_slice.to_vec(), ttl) {
        Ok(_) => 0,
        Err(_) => -1,
    }
}

/// Get a value from cache
#[no_mangle]
pub extern "C" fn cache_get(
    handle: usize,
    key: *const c_char,
    key_len: usize,
    value_out: *mut *mut u8,
    value_len_out: *mut usize,
) -> c_int {
    if key.is_null() || value_out.is_null() || value_len_out.is_null() {
        return -1;
    }
    
    let cache = match CACHE_HANDLES.lock().get(&handle) {
        Some(c) => Arc::clone(c),
        None => return -1,
    };
    
    let key_slice = unsafe { std::slice::from_raw_parts(key as *const u8, key_len) };
    let key_str = match std::str::from_utf8(key_slice) {
        Ok(s) => s,
        Err(_) => return -1,
    };
    
    match cache.get(key_str) {
        Some(value) => {
            let len = value.len();
            let ptr = value.as_ptr() as *mut u8;
            
            // Allocate and copy
            let buffer = unsafe {
                let buf = libc::malloc(len) as *mut u8;
                std::ptr::copy_nonoverlapping(ptr, buf, len);
                buf
            };
            
            unsafe {
                *value_out = buffer;
                *value_len_out = len;
            }
            0
        }
        None => -1,
    }
}

/// Delete from cache
#[no_mangle]
pub extern "C" fn cache_delete(
    handle: usize,
    key: *const c_char,
    key_len: usize,
) {
    if key.is_null() {
        return;
    }
    
    let cache = match CACHE_HANDLES.lock().get(&handle) {
        Some(c) => Arc::clone(c),
        None => return,
    };
    
    let key_slice = unsafe { std::slice::from_raw_parts(key as *const u8, key_len) };
    if let Ok(key_str) = std::str::from_utf8(key_slice) {
        cache.delete(key_str);
    }
}

/// Get cache statistics
#[no_mangle]
pub extern "C" fn cache_stats(
    handle: usize,
    hits_out: *mut u64,
    misses_out: *mut u64,
    evictions_out: *mut u64,
) -> c_int {
    if hits_out.is_null() || misses_out.is_null() || evictions_out.is_null() {
        return -1;
    }
    
    let cache = match CACHE_HANDLES.lock().get(&handle) {
        Some(c) => Arc::clone(c),
        None => return -1,
    };
    
    let stats = cache.stats();
    unsafe {
        *hits_out = stats.get_hits();
        *misses_out = stats.get_misses();
        *evictions_out = stats.get_evictions();
    }
    
    0
}

/// Free memory allocated by Rust
#[no_mangle]
pub extern "C" fn rust_free(ptr: *mut u8) {
    if !ptr.is_null() {
        unsafe {
            libc::free(ptr as *mut c_void);
        }
    }
}
