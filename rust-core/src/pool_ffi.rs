//! FFI bindings for connection pool to be used from Go

use crate::pool::{ConnectionPool, PoolConfig, PooledConnection};
use crate::storage::LockFreeStorage;
use std::sync::Arc;
use std::ffi::CStr;
use std::os::raw::{c_char, c_int};
use std::time::Duration;
use lazy_static::lazy_static;

lazy_static! {
    static ref RUNTIME: tokio::runtime::Runtime = tokio::runtime::Builder::new_multi_thread()
        .worker_threads(num_cpus::get())
        .enable_all()
        .build()
        .expect("Failed to create Tokio runtime");
}

/// Opaque handle for connection pool
pub struct PoolHandle {
    pool: Arc<ConnectionPool>,
}

/// Opaque handle for pooled connection
pub struct ConnectionHandle {
    conn: Option<PooledConnection>,
}

/// Pool statistics for FFI
#[repr(C)]
pub struct CPoolStats {
    pub total_connections: usize,
    pub active_connections: usize,
    pub idle_connections: usize,
    pub wait_count: u64,
    pub avg_wait_time_ms: u64,
    pub connections_created: u64,
    pub connections_closed: u64,
    pub health_check_failures: u64,
}

/// Create a new connection pool
/// Returns NULL on error
#[no_mangle]
pub extern "C" fn mantis_pool_new(
    min_connections: c_int,
    max_connections: c_int,
    max_idle_seconds: c_int,
    connection_timeout_seconds: c_int,
) -> *mut PoolHandle {
    let config = PoolConfig {
        min_connections: min_connections as usize,
        max_connections: max_connections as usize,
        max_idle_time: Duration::from_secs(max_idle_seconds as u64),
        connection_timeout: Duration::from_secs(connection_timeout_seconds as u64),
        max_lifetime: Duration::from_secs(3600),
        health_check_interval: Duration::from_secs(30),
        recycle_connections: true,
    };

    let result = RUNTIME.block_on(async {
        ConnectionPool::new(config, || {
            LockFreeStorage::new(1024 * 1024 * 100).map(Arc::new) // 100MB default
        })
        .await
    });

    match result {
        Ok(pool) => {
            let handle = Box::new(PoolHandle {
                pool: Arc::new(pool),
            });
            Box::into_raw(handle)
        }
        Err(_) => std::ptr::null_mut(),
    }
}

/// Acquire a connection from the pool
/// Returns NULL on error or timeout
#[no_mangle]
pub extern "C" fn mantis_pool_acquire(pool: *mut PoolHandle) -> *mut ConnectionHandle {
    if pool.is_null() {
        return std::ptr::null_mut();
    }

    let pool = unsafe { &*pool };

    let result = RUNTIME.block_on(async {
        pool.pool.acquire().await
    });

    match result {
        Ok(conn) => {
            let handle = Box::new(ConnectionHandle {
                conn: Some(conn),
            });
            Box::into_raw(handle)
        }
        Err(_) => std::ptr::null_mut(),
    }
}

/// Release a connection back to the pool
#[no_mangle]
pub extern "C" fn mantis_pool_release(conn: *mut ConnectionHandle) {
    if conn.is_null() {
        return;
    }

    unsafe {
        let _ = Box::from_raw(conn);
        // Connection will be automatically returned to pool on drop
    }
}

/// Get value from storage using pooled connection
/// Returns 0 on success, -1 on error
#[no_mangle]
pub extern "C" fn mantis_conn_get(
    conn: *mut ConnectionHandle,
    key: *const c_char,
    value_out: *mut *mut u8,
    value_len_out: *mut usize,
) -> c_int {
    if conn.is_null() || key.is_null() || value_out.is_null() || value_len_out.is_null() {
        return -1;
    }

    let conn = unsafe { &*conn };
    let key = unsafe { CStr::from_ptr(key) };
    
    if let Some(pooled_conn) = &conn.conn {
        match pooled_conn.storage().get(key.to_bytes()) {
            Ok(value) => {
                let len = value.len();
                let mut boxed = value.into_boxed_slice();
                let ptr = boxed.as_mut_ptr();
                std::mem::forget(boxed);
                
                unsafe {
                    *value_out = ptr;
                    *value_len_out = len;
                }
                0
            }
            Err(_) => -1,
        }
    } else {
        -1
    }
}

/// Put value into storage using pooled connection
/// Returns 0 on success, -1 on error
#[no_mangle]
pub extern "C" fn mantis_conn_put(
    conn: *mut ConnectionHandle,
    key: *const c_char,
    value: *const u8,
    value_len: usize,
) -> c_int {
    if conn.is_null() || key.is_null() || value.is_null() {
        return -1;
    }

    let conn = unsafe { &*conn };
    let key = unsafe { CStr::from_ptr(key) };
    let value = unsafe { std::slice::from_raw_parts(value, value_len) };
    
    if let Some(pooled_conn) = &conn.conn {
        match pooled_conn.storage().put(key.to_bytes(), value) {
            Ok(_) => 0,
            Err(_) => -1,
        }
    } else {
        -1
    }
}

/// Delete value from storage using pooled connection
/// Returns 0 on success, -1 on error
#[no_mangle]
pub extern "C" fn mantis_conn_delete(
    conn: *mut ConnectionHandle,
    key: *const c_char,
) -> c_int {
    if conn.is_null() || key.is_null() {
        return -1;
    }

    let conn = unsafe { &*conn };
    let key = unsafe { CStr::from_ptr(key) };
    
    if let Some(pooled_conn) = &conn.conn {
        match pooled_conn.storage().delete(key.to_bytes()) {
            Ok(_) => 0,
            Err(_) => -1,
        }
    } else {
        -1
    }
}

/// Get pool statistics
#[no_mangle]
pub extern "C" fn mantis_pool_stats(pool: *mut PoolHandle, stats_out: *mut CPoolStats) -> c_int {
    if pool.is_null() || stats_out.is_null() {
        return -1;
    }

    let pool = unsafe { &*pool };
    let stats = pool.pool.stats();

    let avg_wait_time = if stats.wait_count > 0 {
        stats.total_wait_time_ms / stats.wait_count
    } else {
        0
    };

    unsafe {
        *stats_out = CPoolStats {
            total_connections: stats.total_connections,
            active_connections: stats.active_connections,
            idle_connections: stats.idle_connections,
            wait_count: stats.wait_count,
            avg_wait_time_ms: avg_wait_time,
            connections_created: stats.connections_created,
            connections_closed: stats.connections_closed,
            health_check_failures: stats.health_check_failures,
        };
    }

    0
}

/// Close and destroy the connection pool
#[no_mangle]
pub extern "C" fn mantis_pool_destroy(pool: *mut PoolHandle) {
    if pool.is_null() {
        return;
    }

    unsafe {
        let pool = Box::from_raw(pool);
        RUNTIME.block_on(async {
            pool.pool.close().await;
        });
    }
}

/// Free memory allocated for value
#[no_mangle]
pub extern "C" fn mantis_free_value(ptr: *mut u8, len: usize) {
    if !ptr.is_null() {
        unsafe {
            let _ = Vec::from_raw_parts(ptr, len, len);
        }
    }
}
