//! MantisDB High-Performance Core
//!
//! This module provides lock-free, high-performance storage and caching
//! primitives designed for 5000+ ops/sec throughput with low latency.

#![allow(clippy::missing_safety_doc)]

// Core MantisDB library - Rust-powered database engine
pub mod admin_api;
pub mod batch;
pub mod cache;
pub mod cache_maintenance;
pub mod columnar_engine;
pub mod document_store;
pub mod durability;
pub mod production_config;
pub mod error;
pub mod fast_writer;
pub mod ffi;
pub mod persistent_storage;
pub mod pool;
pub mod pool_ffi;
pub mod rest_api;
pub mod rls;
pub mod rls_ffi;
pub mod sql;
pub mod storage;
pub mod storage_engine;
pub mod transaction;
pub mod vector_db;
pub mod wal;
pub mod query_builder;
pub mod query_analyzer;
pub mod observability;
pub mod fts;
pub mod adaptive_pool;
pub mod graphql_api;
pub mod timeseries;
pub mod geospatial;
pub mod cdc;

pub use batch::{BatchConfig, BatchWriter};
pub use cache::LockFreeCache;
pub use error::{Error, Result};
pub use fast_writer::{FastWriteConfig, FastWriteStats, FastWriter};
pub use pool::{ConnectionPool, PoolConfig, PooledConnection, PoolStats};
pub use rest_api::{RestApiServer, RestApiConfig};
pub use rls::{RlsEngine, Policy, PolicyContext, PolicyCommand, PolicyPermission};
pub use admin_api::{AdminState, build_admin_router};

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
