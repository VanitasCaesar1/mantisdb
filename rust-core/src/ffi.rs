//! FFI bindings for Go integration.
//!
//! This is the critical bridge between Go and Rust. We use opaque handles
//! (integers) instead of raw pointers to avoid Go's GC from moving memory
//! out from under us. The handle->Arc mapping lives entirely in Rust-controlled
//! memory, safe from Go's GC.
use std::ffi::CStr;
use std::os::raw::{c_char, c_int, c_void};
use std::sync::Arc;
use std::slice;
use std::collections::HashMap;
use parking_lot::Mutex;

use crate::storage::LockFreeStorage;
use crate::cache::LockFreeCache;
use crate::fast_writer::{FastWriter, FastWriteConfig};

// Global handle registry - maps opaque integer handles to Rust objects.
// We use Arc for shared ownership across FFI boundary. Mutex (not RwLock)
// because handle creation/deletion is infrequent compared to usage.
lazy_static::lazy_static! {
    static ref STORAGE_HANDLES: Mutex<HashMap<usize, Arc<LockFreeStorage>>> = Mutex::new(HashMap::new());
    static ref CACHE_HANDLES: Mutex<HashMap<usize, Arc<LockFreeCache>>> = Mutex::new(HashMap::new());
    static ref FAST_WRITER_HANDLES: Mutex<HashMap<usize, Arc<FastWriter>>> = Mutex::new(HashMap::new());
    static ref NEXT_HANDLE: Mutex<usize> = Mutex::new(1);
}

// Monotonically increasing handle IDs - never reuse even after free.
// Reusing IDs could let Go code use a stale handle and corrupt data.
// usize is large enough that we'll never wrap in practice.
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
    let storage = match LockFreeStorage::new(1024 * 1024 * 100) {
        Ok(s) => Arc::new(s),
        Err(_) => return 0,
    };
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

    // Trust Go to give us valid pointers and lengths - we can't verify them.
    // If Go passes bad pointers, we'll segfault. That's the FFI contract.
    let key_slice = unsafe { std::slice::from_raw_parts(key as *const u8, key_len) };
    let key_str = match std::str::from_utf8(key_slice) {
        Ok(s) => s.to_string(),
        Err(_) => return -1,
    };

    let value_slice = unsafe { std::slice::from_raw_parts(value, value_len) };

    match storage.put(key_str.as_bytes(), value_slice) {
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

    match storage.get(key_str.as_bytes()) {
        Ok(value) => {
            let len = value.len();
            let ptr = value.as_ptr() as *mut u8;

            // Allocate with libc malloc (not Rust allocator) because Go will free it.
            // Go's C.free() must match our malloc, not Rust's internal allocator.
            // This is a memory ownership handoff across the FFI boundary.
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
pub extern "C" fn storage_delete(handle: usize, key: *const c_char, key_len: usize) -> c_int {
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

    match storage.delete(key_str.as_bytes()) {
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
pub extern "C" fn cache_delete(handle: usize, key: *const c_char, key_len: usize) {
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

// ============================================================================
// Fast Writer FFI
// ============================================================================

/// Create a new fast writer instance
#[no_mangle]
pub extern "C" fn rust_fast_writer_new(
    storage_handle: usize,
    ring_buffer_size: usize,
    worker_threads: usize,
    batch_size: usize,
    flush_interval_ms: u64,
) -> usize {
    let storage = match STORAGE_HANDLES.lock().get(&storage_handle) {
        Some(s) => Arc::clone(s),
        None => return 0,
    };

    let config = FastWriteConfig {
        ring_buffer_size,
        worker_threads,
        batch_size,
        flush_interval_ms,
        enable_compression: false,
        enable_parallel_writes: true,
    };

    let writer = Arc::new(FastWriter::new(storage, config));
    let handle = next_handle();
    FAST_WRITER_HANDLES.lock().insert(handle, writer);
    handle
}

/// Free a fast writer instance
#[no_mangle]
pub extern "C" fn rust_fast_writer_destroy(handle: usize) {
    FAST_WRITER_HANDLES.lock().remove(&handle);
}

/// Write a key-value pair using fast writer
#[no_mangle]
pub extern "C" fn rust_fast_writer_write(
    handle: usize,
    key: *const c_char,
    value: *const u8,
    value_len: usize,
) -> c_int {
    if key.is_null() || value.is_null() {
        return -1;
    }

    let writer = match FAST_WRITER_HANDLES.lock().get(&handle) {
        Some(w) => Arc::clone(w),
        None => return -1,
    };

    let key_str = unsafe {
        match CStr::from_ptr(key).to_str() {
            Ok(s) => s.to_string(),
            Err(_) => return -1,
        }
    };

    let value_slice = unsafe { slice::from_raw_parts(value, value_len) };

    match writer.write(key_str, value_slice.to_vec()) {
        Ok(_) => 0,
        Err(_) => -1,
    }
}

/// Write a batch of key-value pairs
#[no_mangle]
pub extern "C" fn rust_fast_writer_write_batch(
    handle: usize,
    keys: *const *const c_char,
    values: *const *const u8,
    value_lens: *const usize,
    count: usize,
) -> c_int {
    if keys.is_null() || values.is_null() || value_lens.is_null() {
        return -1;
    }

    let writer = match FAST_WRITER_HANDLES.lock().get(&handle) {
        Some(w) => Arc::clone(w),
        None => return -1,
    };

    let keys_slice = unsafe { slice::from_raw_parts(keys, count) };
    let values_slice = unsafe { slice::from_raw_parts(values, count) };
    let lens_slice = unsafe { slice::from_raw_parts(value_lens, count) };

    let mut entries = Vec::with_capacity(count);

    for i in 0..count {
        let key_str = unsafe {
            match CStr::from_ptr(keys_slice[i]).to_str() {
                Ok(s) => s.to_string(),
                Err(_) => return -1,
            }
        };

        let value_vec = unsafe { slice::from_raw_parts(values_slice[i], lens_slice[i]).to_vec() };

        entries.push((key_str, value_vec));
    }

    match writer.write_batch(entries) {
        Ok(_) => 0,
        Err(_) => -1,
    }
}

/// Delete a key using fast writer
#[no_mangle]
pub extern "C" fn rust_fast_writer_delete(handle: usize, key: *const c_char) -> c_int {
    if key.is_null() {
        return -1;
    }

    let writer = match FAST_WRITER_HANDLES.lock().get(&handle) {
        Some(w) => Arc::clone(w),
        None => return -1,
    };

    let key_str = unsafe {
        match CStr::from_ptr(key).to_str() {
            Ok(s) => s.to_string(),
            Err(_) => return -1,
        }
    };

    match writer.delete(key_str) {
        Ok(_) => 0,
        Err(_) => -1,
    }
}

/// Get total writes from fast writer
#[no_mangle]
pub extern "C" fn rust_fast_writer_total_writes(handle: usize) -> u64 {
    let writer = match FAST_WRITER_HANDLES.lock().get(&handle) {
        Some(w) => Arc::clone(w),
        None => return 0,
    };

    writer.stats().total_writes
}

/// Get total bytes from fast writer
#[no_mangle]
pub extern "C" fn rust_fast_writer_total_bytes(handle: usize) -> u64 {
    let writer = match FAST_WRITER_HANDLES.lock().get(&handle) {
        Some(w) => Arc::clone(w),
        None => return 0,
    };

    writer.stats().total_bytes
}

/// Get peak throughput from fast writer
#[no_mangle]
pub extern "C" fn rust_fast_writer_peak_throughput(handle: usize) -> u64 {
    let writer = match FAST_WRITER_HANDLES.lock().get(&handle) {
        Some(w) => Arc::clone(w),
        None => return 0,
    };

    writer.stats().peak_throughput
}
