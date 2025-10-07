//! MantisDB High-Performance Core
//! 
//! This module provides lock-free, high-performance storage and caching
//! primitives designed for 5000+ ops/sec throughput with low latency.

#![allow(clippy::missing_safety_doc)]

pub mod storage;
pub mod cache;
pub mod ffi;
pub mod error;

pub use storage::LockFreeStorage;
pub use cache::LockFreeCache;
pub use error::{Result, Error};

#[global_allocator]
static GLOBAL: mimalloc::MiMalloc = mimalloc::MiMalloc;

/// Initialize the Rust core library
#[no_mangle]
pub extern "C" fn mantisdb_init() -> i32 {
    0 // Success
}

/// Get version information
#[no_mangle]
pub extern "C" fn mantisdb_version() -> *const libc::c_char {
    b"0.1.0\0".as_ptr() as *const libc::c_char
}
