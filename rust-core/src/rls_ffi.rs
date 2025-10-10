//! FFI bindings for RLS Engine
//!
//! Provides C-compatible interface for Go integration

use crate::rls::{RlsEngine, Policy, PolicyContext};
use std::ffi::{CStr, CString};
use std::os::raw::c_char;
use std::sync::Arc;

/// Opaque handle to RLS Engine
pub struct RlsEngineHandle {
    engine: Arc<RlsEngine>,
}

/// Create a new RLS engine
#[no_mangle]
pub extern "C" fn rls_engine_new() -> *mut RlsEngineHandle {
    let engine = Arc::new(RlsEngine::new());
    Box::into_raw(Box::new(RlsEngineHandle { engine }))
}

/// Free an RLS engine
#[no_mangle]
pub extern "C" fn rls_engine_free(handle: *mut RlsEngineHandle) {
    if !handle.is_null() {
        unsafe {
            let _ = Box::from_raw(handle);
        }
    }
}

/// Enable RLS for a table
#[no_mangle]
pub extern "C" fn rls_enable(handle: *mut RlsEngineHandle, table: *const c_char) -> i32 {
    if handle.is_null() || table.is_null() {
        return -1;
    }

    unsafe {
        let handle = &*handle;
        let table_str = match CStr::from_ptr(table).to_str() {
            Ok(s) => s,
            Err(_) => return -2,
        };

        match handle.engine.enable_rls(table_str) {
            Ok(_) => 0,
            Err(_) => -3,
        }
    }
}

/// Disable RLS for a table
#[no_mangle]
pub extern "C" fn rls_disable(handle: *mut RlsEngineHandle, table: *const c_char) -> i32 {
    if handle.is_null() || table.is_null() {
        return -1;
    }

    unsafe {
        let handle = &*handle;
        let table_str = match CStr::from_ptr(table).to_str() {
            Ok(s) => s,
            Err(_) => return -2,
        };

        match handle.engine.disable_rls(table_str) {
            Ok(_) => 0,
            Err(_) => -3,
        }
    }
}

/// Check if RLS is enabled for a table
#[no_mangle]
pub extern "C" fn rls_is_enabled(handle: *mut RlsEngineHandle, table: *const c_char) -> i32 {
    if handle.is_null() || table.is_null() {
        return -1;
    }

    unsafe {
        let handle = &*handle;
        let table_str = match CStr::from_ptr(table).to_str() {
            Ok(s) => s,
            Err(_) => return -1,
        };

        if handle.engine.is_rls_enabled(table_str) {
            1
        } else {
            0
        }
    }
}

/// Add a policy (JSON format)
#[no_mangle]
pub extern "C" fn rls_add_policy(handle: *mut RlsEngineHandle, policy_json: *const c_char) -> i32 {
    if handle.is_null() || policy_json.is_null() {
        return -1;
    }

    unsafe {
        let handle = &*handle;
        let json_str = match CStr::from_ptr(policy_json).to_str() {
            Ok(s) => s,
            Err(_) => return -2,
        };

        let policy: Policy = match serde_json::from_str(json_str) {
            Ok(p) => p,
            Err(_) => return -3,
        };

        match handle.engine.add_policy(policy) {
            Ok(_) => 0,
            Err(_) => -4,
        }
    }
}

/// Remove a policy
#[no_mangle]
pub extern "C" fn rls_remove_policy(
    handle: *mut RlsEngineHandle,
    table: *const c_char,
    policy_name: *const c_char,
) -> i32 {
    if handle.is_null() || table.is_null() || policy_name.is_null() {
        return -1;
    }

    unsafe {
        let handle = &*handle;
        let table_str = match CStr::from_ptr(table).to_str() {
            Ok(s) => s,
            Err(_) => return -2,
        };
        let name_str = match CStr::from_ptr(policy_name).to_str() {
            Ok(s) => s,
            Err(_) => return -2,
        };

        match handle.engine.remove_policy(table_str, name_str) {
            Ok(_) => 0,
            Err(_) => -3,
        }
    }
}

/// Get policies for a table (returns JSON array)
#[no_mangle]
pub extern "C" fn rls_get_policies(
    handle: *mut RlsEngineHandle,
    table: *const c_char,
) -> *mut c_char {
    if handle.is_null() || table.is_null() {
        return std::ptr::null_mut();
    }

    unsafe {
        let handle = &*handle;
        let table_str = match CStr::from_ptr(table).to_str() {
            Ok(s) => s,
            Err(_) => return std::ptr::null_mut(),
        };

        let policies = handle.engine.get_policies(table_str);
        let json = match serde_json::to_string(&policies) {
            Ok(j) => j,
            Err(_) => return std::ptr::null_mut(),
        };

        match CString::new(json) {
            Ok(c) => c.into_raw(),
            Err(_) => std::ptr::null_mut(),
        }
    }
}

/// Check SELECT permission
#[no_mangle]
pub extern "C" fn rls_check_select(
    handle: *mut RlsEngineHandle,
    table: *const c_char,
    context_json: *const c_char,
    row_json: *const c_char,
) -> i32 {
    if handle.is_null() || table.is_null() || context_json.is_null() || row_json.is_null() {
        return -1;
    }

    unsafe {
        let handle = &*handle;
        let table_str = match CStr::from_ptr(table).to_str() {
            Ok(s) => s,
            Err(_) => return -2,
        };

        let context_str = match CStr::from_ptr(context_json).to_str() {
            Ok(s) => s,
            Err(_) => return -2,
        };

        let row_str = match CStr::from_ptr(row_json).to_str() {
            Ok(s) => s,
            Err(_) => return -2,
        };

        // Parse context
        let context: PolicyContext = match serde_json::from_str(context_str) {
            Ok(c) => c,
            Err(_) => return -3,
        };

        // Parse row data
        let row_data: serde_json::Value = match serde_json::from_str(row_str) {
            Ok(r) => r,
            Err(_) => return -3,
        };

        match handle.engine.check_select(table_str, &context, &row_data) {
            Ok(true) => 1,  // Allowed
            Ok(false) => 0, // Denied
            Err(_) => -4,   // Error
        }
    }
}

/// Check INSERT permission
#[no_mangle]
pub extern "C" fn rls_check_insert(
    handle: *mut RlsEngineHandle,
    table: *const c_char,
    context_json: *const c_char,
    row_json: *const c_char,
) -> i32 {
    if handle.is_null() || table.is_null() || context_json.is_null() || row_json.is_null() {
        return -1;
    }

    unsafe {
        let handle = &*handle;
        let table_str = match CStr::from_ptr(table).to_str() {
            Ok(s) => s,
            Err(_) => return -2,
        };

        let context_str = match CStr::from_ptr(context_json).to_str() {
            Ok(s) => s,
            Err(_) => return -2,
        };

        let row_str = match CStr::from_ptr(row_json).to_str() {
            Ok(s) => s,
            Err(_) => return -2,
        };

        let context: PolicyContext = match serde_json::from_str(context_str) {
            Ok(c) => c,
            Err(_) => return -3,
        };

        let row_data: serde_json::Value = match serde_json::from_str(row_str) {
            Ok(r) => r,
            Err(_) => return -3,
        };

        match handle.engine.check_insert(table_str, &context, &row_data) {
            Ok(true) => 1,
            Ok(false) => 0,
            Err(_) => -4,
        }
    }
}

/// Check UPDATE permission
#[no_mangle]
pub extern "C" fn rls_check_update(
    handle: *mut RlsEngineHandle,
    table: *const c_char,
    context_json: *const c_char,
    old_row_json: *const c_char,
    new_row_json: *const c_char,
) -> i32 {
    if handle.is_null() || table.is_null() || context_json.is_null() 
        || old_row_json.is_null() || new_row_json.is_null() {
        return -1;
    }

    unsafe {
        let handle = &*handle;
        let table_str = match CStr::from_ptr(table).to_str() {
            Ok(s) => s,
            Err(_) => return -2,
        };

        let context_str = match CStr::from_ptr(context_json).to_str() {
            Ok(s) => s,
            Err(_) => return -2,
        };

        let old_row_str = match CStr::from_ptr(old_row_json).to_str() {
            Ok(s) => s,
            Err(_) => return -2,
        };

        let new_row_str = match CStr::from_ptr(new_row_json).to_str() {
            Ok(s) => s,
            Err(_) => return -2,
        };

        let context: PolicyContext = match serde_json::from_str(context_str) {
            Ok(c) => c,
            Err(_) => return -3,
        };

        let old_row: serde_json::Value = match serde_json::from_str(old_row_str) {
            Ok(r) => r,
            Err(_) => return -3,
        };

        let new_row: serde_json::Value = match serde_json::from_str(new_row_str) {
            Ok(r) => r,
            Err(_) => return -3,
        };

        match handle.engine.check_update(table_str, &context, &old_row, &new_row) {
            Ok(true) => 1,
            Ok(false) => 0,
            Err(_) => -4,
        }
    }
}

/// Check DELETE permission
#[no_mangle]
pub extern "C" fn rls_check_delete(
    handle: *mut RlsEngineHandle,
    table: *const c_char,
    context_json: *const c_char,
    row_json: *const c_char,
) -> i32 {
    if handle.is_null() || table.is_null() || context_json.is_null() || row_json.is_null() {
        return -1;
    }

    unsafe {
        let handle = &*handle;
        let table_str = match CStr::from_ptr(table).to_str() {
            Ok(s) => s,
            Err(_) => return -2,
        };

        let context_str = match CStr::from_ptr(context_json).to_str() {
            Ok(s) => s,
            Err(_) => return -2,
        };

        let row_str = match CStr::from_ptr(row_json).to_str() {
            Ok(s) => s,
            Err(_) => return -2,
        };

        let context: PolicyContext = match serde_json::from_str(context_str) {
            Ok(c) => c,
            Err(_) => return -3,
        };

        let row_data: serde_json::Value = match serde_json::from_str(row_str) {
            Ok(r) => r,
            Err(_) => return -3,
        };

        match handle.engine.check_delete(table_str, &context, &row_data) {
            Ok(true) => 1,
            Ok(false) => 0,
            Err(_) => -4,
        }
    }
}

/// Free a C string returned by this library
#[no_mangle]
pub extern "C" fn rls_free_string(s: *mut c_char) {
    if !s.is_null() {
        unsafe {
            let _ = CString::from_raw(s);
        }
    }
}
