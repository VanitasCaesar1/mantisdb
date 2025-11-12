// Integration tests for disk-backed storage engine
use mantisdb::error::Result;
use mantisdb::storage::LockFreeStorage;
use tempfile::TempDir;

#[test]
fn test_disk_storage_basic_operations() -> Result<()> {
    let temp_dir = TempDir::new().unwrap();
    let disk_path = temp_dir.path().join("test.db");
    
    // Create storage with disk backing
    let storage = LockFreeStorage::with_disk_storage(
        1000,
        &disk_path,
        100, // buffer pool size
    )?;
    
    // Write data
    storage.put_string("key1".to_string(), b"value1".to_vec())?;
    storage.put_string("key2".to_string(), b"value2".to_vec())?;
    storage.put_string("key3".to_string(), b"value3".to_vec())?;
    
    // Read data
    assert_eq!(storage.get_string("key1")?, b"value1");
    assert_eq!(storage.get_string("key2")?, b"value2");
    assert_eq!(storage.get_string("key3")?, b"value3");
    
    // Delete data
    storage.delete_string("key2")?;
    assert!(storage.get_string("key2").is_err());
    
    Ok(())
}

#[test]
fn test_disk_storage_persistence() -> Result<()> {
    let temp_dir = TempDir::new().unwrap();
    let disk_path = temp_dir.path().join("persist.db");
    
    // Write data and drop storage
    {
        let storage = LockFreeStorage::with_disk_storage(1000, &disk_path, 100)?;
        storage.put_string("persistent".to_string(), b"data".to_vec())?;
    }
    
    // Reopen and verify data persists
    {
        let storage = LockFreeStorage::with_disk_storage(1000, &disk_path, 100)?;
        assert_eq!(storage.get_string("persistent")?, b"data");
    }
    
    Ok(())
}

#[test]
fn test_disk_storage_large_dataset() -> Result<()> {
    let temp_dir = TempDir::new().unwrap();
    let disk_path = temp_dir.path().join("large.db");
    
    // Create storage with small buffer pool to force disk usage
    let storage = LockFreeStorage::with_disk_storage(10000, &disk_path, 10)?;
    
    // Write 1000 entries
    for i in 0..1000 {
        let key = format!("key_{:04}", i);
        let value = format!("value_{:04}", i).into_bytes();
        storage.put_string(key, value)?;
    }
    
    // Verify all entries
    for i in 0..1000 {
        let key = format!("key_{:04}", i);
        let expected = format!("value_{:04}", i).into_bytes();
        let actual = storage.get_string(&key)?;
        assert_eq!(actual, expected);
    }
    
    // Verify stats
    let stats = storage.stats();
    assert_eq!(stats.get_writes(), 1000);
    assert_eq!(stats.get_reads(), 1000);
    
    Ok(())
}

#[test]
fn test_memory_to_disk_fallback() -> Result<()> {
    let temp_dir = TempDir::new().unwrap();
    let disk_path = temp_dir.path().join("fallback.db");
    
    let storage = LockFreeStorage::with_disk_storage(1000, &disk_path, 50)?;
    
    // Write data
    storage.put_string("test_key".to_string(), b"test_value".to_vec())?;
    
    // Clear in-memory cache (simulate memory pressure)
    storage.clear();
    
    // Should still be able to read from disk
    // Note: Current implementation promotes back to memory on read
    let value = storage.get_string("test_key")?;
    assert_eq!(value, b"test_value");
    
    Ok(())
}

#[test]
fn test_disk_storage_batch_operations() -> Result<()> {
    let temp_dir = TempDir::new().unwrap();
    let disk_path = temp_dir.path().join("batch.db");
    
    let storage = LockFreeStorage::with_disk_storage(10000, &disk_path, 100)?;
    
    // Prepare batch data
    let mut entries = Vec::new();
    for i in 0..100 {
        entries.push((format!("batch_{}", i), format!("value_{}", i).into_bytes()));
    }
    
    // Batch write
    storage.batch_put(entries)?;
    
    // Verify all written
    for i in 0..100 {
        let key = format!("batch_{}", i);
        let value = storage.get_string(&key)?;
        assert_eq!(value, format!("value_{}", i).into_bytes());
    }
    
    Ok(())
}

#[test]
fn test_disk_storage_prefix_scan() -> Result<()> {
    let temp_dir = TempDir::new().unwrap();
    let disk_path = temp_dir.path().join("scan.db");
    
    let storage = LockFreeStorage::with_disk_storage(1000, &disk_path, 100)?;
    
    // Write data with prefixes
    storage.put_string("user:1".to_string(), b"alice".to_vec())?;
    storage.put_string("user:2".to_string(), b"bob".to_vec())?;
    storage.put_string("user:3".to_string(), b"charlie".to_vec())?;
    storage.put_string("order:1".to_string(), b"item1".to_vec())?;
    storage.put_string("order:2".to_string(), b"item2".to_vec())?;
    
    // Scan with prefix
    let user_results = storage.scan_prefix("user:");
    assert_eq!(user_results.len(), 3);
    
    let order_results = storage.scan_prefix("order:");
    assert_eq!(order_results.len(), 2);
    
    Ok(())
}

#[test]
fn test_disk_storage_concurrent_access() -> Result<()> {
    use std::sync::Arc;
    use std::thread;
    
    let temp_dir = TempDir::new().unwrap();
    let disk_path = temp_dir.path().join("concurrent.db");
    
    let storage = Arc::new(LockFreeStorage::with_disk_storage(10000, &disk_path, 100)?);
    
    let mut handles = vec![];
    
    // Spawn 10 threads writing concurrently
    for thread_id in 0..10 {
        let storage_clone = Arc::clone(&storage);
        
        let handle = thread::spawn(move || {
            for i in 0..100 {
                let key = format!("thread_{}_{}", thread_id, i);
                let value = format!("value_{}_{}", thread_id, i).into_bytes();
                storage_clone.put_string(key, value).unwrap();
            }
        });
        
        handles.push(handle);
    }
    
    // Wait for all threads
    for handle in handles {
        handle.join().unwrap();
    }
    
    // Verify all data written
    for thread_id in 0..10 {
        for i in 0..100 {
            let key = format!("thread_{}_{}", thread_id, i);
            let expected = format!("value_{}_{}", thread_id, i).into_bytes();
            let actual = storage.get_string(&key)?;
            assert_eq!(actual, expected);
        }
    }
    
    // Should have 1000 writes total
    let stats = storage.stats();
    assert_eq!(stats.get_writes(), 1000);
    
    Ok(())
}

#[test]
fn test_disk_storage_updates() -> Result<()> {
    let temp_dir = TempDir::new().unwrap();
    let disk_path = temp_dir.path().join("updates.db");
    
    let storage = LockFreeStorage::with_disk_storage(1000, &disk_path, 100)?;
    
    // Write initial value
    storage.put_string("update_key".to_string(), b"v1".to_vec())?;
    assert_eq!(storage.get_string("update_key")?, b"v1");
    
    // Update value
    storage.put_string("update_key".to_string(), b"v2".to_vec())?;
    assert_eq!(storage.get_string("update_key")?, b"v2");
    
    // Update again
    storage.put_string("update_key".to_string(), b"v3".to_vec())?;
    assert_eq!(storage.get_string("update_key")?, b"v3");
    
    Ok(())
}
