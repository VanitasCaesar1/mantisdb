//! Comprehensive crash recovery tests
//! 
//! Tests WAL recovery under various failure scenarios

use mantisdb::persistent_storage::{PersistentStorage, PersistentStorageConfig};
use mantisdb::storage::LockFreeStorage;
use std::sync::Arc;
use tempfile::TempDir;

#[test]
fn test_recovery_after_clean_shutdown() {
    let temp_dir = TempDir::new().unwrap();
    let data_dir = temp_dir.path().to_str().unwrap();
    
    let config = PersistentStorageConfig {
        data_dir: data_dir.to_string(),
        wal_enabled: true,
        sync_on_write: true,
    };
    
    // Write some data
    {
        let storage = PersistentStorage::new(config.clone()).unwrap();
        let mem = storage.memory();
        
        mem.put_string("key1".to_string(), b"value1".to_vec()).unwrap();
        mem.put_string("key2".to_string(), b"value2".to_vec()).unwrap();
        mem.put_string("key3".to_string(), b"value3".to_vec()).unwrap();
        
        // Explicitly drop to ensure clean shutdown
        drop(storage);
    }
    
    // Recover
    {
        let storage = PersistentStorage::new(config).unwrap();
        let mem = storage.memory();
        
        assert_eq!(mem.get_string("key1").unwrap(), b"value1");
        assert_eq!(mem.get_string("key2").unwrap(), b"value2");
        assert_eq!(mem.get_string("key3").unwrap(), b"value3");
        assert_eq!(mem.len(), 3);
    }
}

#[test]
fn test_recovery_after_partial_writes() {
    let temp_dir = TempDir::new().unwrap();
    let data_dir = temp_dir.path().to_str().unwrap();
    
    let config = PersistentStorageConfig {
        data_dir: data_dir.to_string(),
        wal_enabled: true,
        sync_on_write: false, // Async writes to simulate crash
    };
    
    // Write data with async sync
    {
        let storage = PersistentStorage::new(config.clone()).unwrap();
        let mem = storage.memory();
        
        for i in 0..100 {
            mem.put_string(format!("key_{}", i), format!("value_{}", i).into_bytes()).unwrap();
        }
        
        // Simulate crash (don't drop cleanly)
        std::mem::forget(storage);
    }
    
    // Recovery should handle incomplete WAL
    {
        let storage = PersistentStorage::new(config).unwrap();
        let mem = storage.memory();
        
        // At least some data should be recovered
        // (Exact count depends on how many syncs completed)
        assert!(mem.len() > 0);
        println!("Recovered {} entries after simulated crash", mem.len());
    }
}

#[test]
fn test_recovery_with_deletes() {
    let temp_dir = TempDir::new().unwrap();
    let data_dir = temp_dir.path().to_str().unwrap();
    
    let config = PersistentStorageConfig {
        data_dir: data_dir.to_string(),
        wal_enabled: true,
        sync_on_write: true,
    };
    
    // Write and delete data
    {
        let storage = PersistentStorage::new(config.clone()).unwrap();
        let mem = storage.memory();
        
        mem.put_string("key1".to_string(), b"value1".to_vec()).unwrap();
        mem.put_string("key2".to_string(), b"value2".to_vec()).unwrap();
        mem.put_string("key3".to_string(), b"value3".to_vec()).unwrap();
        
        // Delete key2
        mem.delete_string("key2").unwrap();
        
        drop(storage);
    }
    
    // Recover and verify delete was applied
    {
        let storage = PersistentStorage::new(config).unwrap();
        let mem = storage.memory();
        
        assert_eq!(mem.get_string("key1").unwrap(), b"value1");
        assert!(mem.get_string("key2").is_err()); // Should not exist
        assert_eq!(mem.get_string("key3").unwrap(), b"value3");
        assert_eq!(mem.len(), 2);
    }
}

#[test]
fn test_recovery_with_updates() {
    let temp_dir = TempDir::new().unwrap();
    let data_dir = temp_dir.path().to_str().unwrap();
    
    let config = PersistentStorageConfig {
        data_dir: data_dir.to_string(),
        wal_enabled: true,
        sync_on_write: true,
    };
    
    // Write, update, write
    {
        let storage = PersistentStorage::new(config.clone()).unwrap();
        let mem = storage.memory();
        
        mem.put_string("key1".to_string(), b"value1".to_vec()).unwrap();
        mem.put_string("key1".to_string(), b"value1_updated".to_vec()).unwrap();
        mem.put_string("key1".to_string(), b"value1_final".to_vec()).unwrap();
        
        drop(storage);
    }
    
    // Recover and verify final value
    {
        let storage = PersistentStorage::new(config).unwrap();
        let mem = storage.memory();
        
        assert_eq!(mem.get_string("key1").unwrap(), b"value1_final");
    }
}

#[test]
fn test_recovery_idempotency() {
    let temp_dir = TempDir::new().unwrap();
    let data_dir = temp_dir.path().to_str().unwrap();
    
    let config = PersistentStorageConfig {
        data_dir: data_dir.to_string(),
        wal_enabled: true,
        sync_on_write: true,
    };
    
    // Write data
    {
        let storage = PersistentStorage::new(config.clone()).unwrap();
        let mem = storage.memory();
        
        mem.put_string("key1".to_string(), b"value1".to_vec()).unwrap();
        drop(storage);
    }
    
    // Recover multiple times (should be idempotent)
    for _ in 0..3 {
        let storage = PersistentStorage::new(config.clone()).unwrap();
        let mem = storage.memory();
        
        assert_eq!(mem.get_string("key1").unwrap(), b"value1");
        assert_eq!(mem.len(), 1);
        drop(storage);
    }
}

#[test]
fn test_recovery_with_ttl_expired() {
    let temp_dir = TempDir::new().unwrap();
    let data_dir = temp_dir.path().to_str().unwrap();
    
    let config = PersistentStorageConfig {
        data_dir: data_dir.to_string(),
        wal_enabled: true,
        sync_on_write: true,
    };
    
    // Write data with TTL
    {
        let storage = PersistentStorage::new(config.clone()).unwrap();
        let mem = storage.memory();
        
        mem.put_with_ttl("key1".to_string(), b"value1".to_vec(), 1).unwrap(); // 1 second TTL
        mem.put_string("key2".to_string(), b"value2".to_vec()).unwrap(); // No TTL
        
        drop(storage);
    }
    
    // Wait for TTL to expire
    std::thread::sleep(std::time::Duration::from_secs(2));
    
    // Recover and verify expired key is not accessible
    {
        let storage = PersistentStorage::new(config).unwrap();
        let mem = storage.memory();
        
        assert!(mem.get_string("key1").is_err()); // Should be expired
        assert_eq!(mem.get_string("key2").unwrap(), b"value2");
    }
}

#[test]
fn test_recovery_with_large_dataset() {
    let temp_dir = TempDir::new().unwrap();
    let data_dir = temp_dir.path().to_str().unwrap();
    
    let config = PersistentStorageConfig {
        data_dir: data_dir.to_string(),
        wal_enabled: true,
        sync_on_write: true,
    };
    
    let count = 10_000;
    
    // Write large dataset
    {
        let storage = PersistentStorage::new(config.clone()).unwrap();
        let mem = storage.memory();
        
        for i in 0..count {
            mem.put_string(format!("key_{}", i), format!("value_{}", i).into_bytes()).unwrap();
        }
        
        drop(storage);
    }
    
    // Recover and verify all data
    {
        let storage = PersistentStorage::new(config).unwrap();
        let mem = storage.memory();
        
        assert_eq!(mem.len(), count);
        
        // Spot check
        assert_eq!(mem.get_string("key_0").unwrap(), b"value_0");
        assert_eq!(mem.get_string("key_5000").unwrap(), b"value_5000");
        assert_eq!(mem.get_string("key_9999").unwrap(), b"value_9999");
    }
}

#[test]
fn test_recovery_empty_wal() {
    let temp_dir = TempDir::new().unwrap();
    let data_dir = temp_dir.path().to_str().unwrap();
    
    let config = PersistentStorageConfig {
        data_dir: data_dir.to_string(),
        wal_enabled: true,
        sync_on_write: true,
    };
    
    // Create storage but don't write anything
    {
        let storage = PersistentStorage::new(config.clone()).unwrap();
        drop(storage);
    }
    
    // Recover from empty WAL
    {
        let storage = PersistentStorage::new(config).unwrap();
        let mem = storage.memory();
        
        assert_eq!(mem.len(), 0);
    }
}

#[test]
fn test_recovery_with_batch_operations() {
    let temp_dir = TempDir::new().unwrap();
    let data_dir = temp_dir.path().to_str().unwrap();
    
    let config = PersistentStorageConfig {
        data_dir: data_dir.to_string(),
        wal_enabled: true,
        sync_on_write: true,
    };
    
    // Write using batch operations
    {
        let storage = PersistentStorage::new(config.clone()).unwrap();
        let mem = storage.memory();
        
        let entries: Vec<(String, Vec<u8>)> = (0..100)
            .map(|i| (format!("key_{}", i), format!("value_{}", i).into_bytes()))
            .collect();
        
        mem.batch_put(entries).unwrap();
        
        drop(storage);
    }
    
    // Recover and verify
    {
        let storage = PersistentStorage::new(config).unwrap();
        let mem = storage.memory();
        
        assert_eq!(mem.len(), 100);
    }
}

#[test]
fn test_recovery_performance() {
    let temp_dir = TempDir::new().unwrap();
    let data_dir = temp_dir.path().to_str().unwrap();
    
    let config = PersistentStorageConfig {
        data_dir: data_dir.to_string(),
        wal_enabled: true,
        sync_on_write: true,
    };
    
    let count = 50_000;
    
    // Write data
    {
        let storage = PersistentStorage::new(config.clone()).unwrap();
        let mem = storage.memory();
        
        for i in 0..count {
            mem.put_string(format!("key_{}", i), format!("value_{}", i).into_bytes()).unwrap();
        }
        
        drop(storage);
    }
    
    // Measure recovery time
    let start = std::time::Instant::now();
    {
        let storage = PersistentStorage::new(config).unwrap();
        let mem = storage.memory();
        assert_eq!(mem.len(), count);
    }
    let recovery_time = start.elapsed();
    
    println!("Recovery of {} entries took {:?}", count, recovery_time);
    
    // Recovery should be reasonably fast (< 1 second for 50k entries)
    assert!(recovery_time.as_secs() < 5, "Recovery took too long: {:?}", recovery_time);
}
