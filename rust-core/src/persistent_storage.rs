//! Persistent storage with disk backing and WAL
//! 
//! This module provides a persistent storage layer that writes to disk
//! and uses Write-Ahead Logging for durability.

use crate::error::{Error, Result};
use crate::storage::{LockFreeStorage, StorageEntry};
use serde::{Deserialize, Serialize};
use std::fs::{self, File, OpenOptions};
use std::io::{BufReader, BufWriter, Write};
use std::path::{Path, PathBuf};
use std::sync::Arc;

/// Persistent storage configuration
#[derive(Debug, Clone)]
pub struct PersistentStorageConfig {
    pub data_dir: PathBuf,
    pub wal_enabled: bool,
    pub sync_on_write: bool,
}

impl Default for PersistentStorageConfig {
    fn default() -> Self {
        Self {
            data_dir: PathBuf::from("./data"),
            wal_enabled: true,
            sync_on_write: true,
        }
    }
}

/// WAL entry for write-ahead logging
#[derive(Debug, Serialize, Deserialize)]
enum WalEntry {
    Put { key: String, value: Vec<u8> },
    Delete { key: String },
}

/// Persistent storage with disk backing
pub struct PersistentStorage {
    memory: Arc<LockFreeStorage>,
    config: PersistentStorageConfig,
    wal_file: Option<File>,
}

impl PersistentStorage {
    /// Create a new persistent storage instance
    pub fn new(config: PersistentStorageConfig) -> Result<Self> {
        // Create data directory if it doesn't exist
        fs::create_dir_all(&config.data_dir)
            .map_err(|e| Error::StorageError(format!("Failed to create data directory: {}", e)))?;

        // Initialize in-memory storage
        let memory = Arc::new(LockFreeStorage::new(10000)?);

        // Open WAL file if enabled
        let wal_file = if config.wal_enabled {
            let wal_path = config.data_dir.join("wal.log");
            Some(
                OpenOptions::new()
                    .create(true)
                    .append(true)
                    .open(&wal_path)
                    .map_err(|e| Error::StorageError(format!("Failed to open WAL: {}", e)))?,
            )
        } else {
            None
        };

        let mut storage = Self {
            memory,
            config,
            wal_file,
        };

        // Load existing data from disk
        storage.load_from_disk()?;

        Ok(storage)
    }

    /// Load data from disk on startup
    fn load_from_disk(&mut self) -> Result<()> {
        let snapshot_path = self.config.data_dir.join("snapshot.json");
        
        if snapshot_path.exists() {
            println!("📂 Loading database from disk: {:?}", snapshot_path);
            
            let file = File::open(&snapshot_path)
                .map_err(|e| Error::StorageError(format!("Failed to open snapshot: {}", e)))?;
            
            let reader = BufReader::new(file);
            let entries: Vec<(String, Vec<u8>)> = serde_json::from_reader(reader)
                .map_err(|e| Error::StorageError(format!("Failed to parse snapshot: {}", e)))?;
            
            println!("✅ Loaded {} entries from disk", entries.len());
            
            // Load into memory
            for (key, value) in entries {
                self.memory.put_string(key, value)?;
            }
        } else {
            println!("📂 No existing database found, starting fresh");
        }

        // Replay WAL if it exists
        self.replay_wal()?;

        Ok(())
    }

    /// Replay WAL entries
    fn replay_wal(&self) -> Result<()> {
        let wal_path = self.config.data_dir.join("wal.log");
        
        if !wal_path.exists() {
            return Ok(());
        }

        println!("📝 Replaying WAL...");
        
        let file = File::open(&wal_path)
            .map_err(|e| Error::StorageError(format!("Failed to open WAL: {}", e)))?;
        
        let reader = BufReader::new(file);
        let entries: Vec<WalEntry> = serde_json::Deserializer::from_reader(reader)
            .into_iter::<WalEntry>()
            .filter_map(|e| e.ok())
            .collect();

        for entry in entries {
            match entry {
                WalEntry::Put { key, value } => {
                    self.memory.put_string(key, value)?;
                }
                WalEntry::Delete { key } => {
                    self.memory.delete_string(&key)?;
                }
            }
        }

        println!("✅ WAL replay complete");
        Ok(())
    }

    /// Write to WAL
    fn write_wal(&mut self, entry: &WalEntry) -> Result<()> {
        if let Some(ref mut wal) = self.wal_file {
            let json = serde_json::to_string(entry)
                .map_err(|e| Error::StorageError(format!("Failed to serialize WAL entry: {}", e)))?;
            
            writeln!(wal, "{}", json)
                .map_err(|e| Error::StorageError(format!("Failed to write WAL: {}", e)))?;
            
            if self.config.sync_on_write {
                wal.sync_all()
                    .map_err(|e| Error::StorageError(format!("Failed to sync WAL: {}", e)))?;
            }
        }
        Ok(())
    }

    /// Put a key-value pair with persistence
    pub fn put(&mut self, key: String, value: Vec<u8>) -> Result<()> {
        // Write to WAL first
        self.write_wal(&WalEntry::Put {
            key: key.clone(),
            value: value.clone(),
        })?;

        // Then write to memory
        self.memory.put_string(key, value)?;

        Ok(())
    }

    /// Get a value by key
    pub fn get(&self, key: &str) -> Result<Vec<u8>> {
        self.memory.get_string(key)
    }

    /// Delete a key with persistence
    pub fn delete(&mut self, key: String) -> Result<()> {
        // Write to WAL first
        self.write_wal(&WalEntry::Delete { key: key.clone() })?;

        // Then delete from memory
        self.memory.delete_string(&key)?;

        Ok(())
    }

    /// Create a snapshot of current data to disk
    pub fn snapshot(&self) -> Result<()> {
        println!("💾 Creating database snapshot...");
        
        let snapshot_path = self.config.data_dir.join("snapshot.json");
        let temp_path = self.config.data_dir.join("snapshot.json.tmp");

        // Collect all data
        let entries: Vec<(String, Vec<u8>)> = self.memory
            .scan_prefix("")
            .into_iter()
            .collect();

        // Write to temporary file
        let file = File::create(&temp_path)
            .map_err(|e| Error::StorageError(format!("Failed to create snapshot: {}", e)))?;
        
        let writer = BufWriter::new(file);
        serde_json::to_writer(writer, &entries)
            .map_err(|e| Error::StorageError(format!("Failed to write snapshot: {}", e)))?;

        // Atomic rename
        fs::rename(&temp_path, &snapshot_path)
            .map_err(|e| Error::StorageError(format!("Failed to rename snapshot: {}", e)))?;

        // Clear WAL after successful snapshot
        if self.config.wal_enabled {
            let wal_path = self.config.data_dir.join("wal.log");
            let _ = fs::remove_file(&wal_path);
        }

        println!("✅ Snapshot created with {} entries", entries.len());
        Ok(())
    }

    /// Get the underlying in-memory storage (read-only access)
    pub fn memory(&self) -> &Arc<LockFreeStorage> {
        &self.memory
    }

    /// Return the data directory path
    pub fn data_dir(&self) -> PathBuf {
        self.config.data_dir.clone()
    }

    /// Reload from snapshot + WAL on disk, replacing in-memory state
    pub fn reload_from_disk(&mut self) -> Result<()> {
        // Clear current in-memory data
        self.memory.clear();
        // Reload snapshot and replay WAL
        self.load_from_disk()
    }

    /// Get statistics
    pub fn len(&self) -> usize {
        self.memory.len()
    }

    /// Check if empty
    pub fn is_empty(&self) -> bool {
        self.memory.is_empty()
    }
}

impl Drop for PersistentStorage {
    fn drop(&mut self) {
        // Create snapshot on shutdown
        if let Err(e) = self.snapshot() {
            eprintln!("⚠️  Failed to create snapshot on shutdown: {}", e);
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::TempDir;

    #[test]
    fn test_persistent_storage() {
        let temp_dir = TempDir::new().unwrap();
        let config = PersistentStorageConfig {
            data_dir: temp_dir.path().to_path_buf(),
            wal_enabled: true,
            sync_on_write: true,
        };

        // Create storage and write data
        {
            let mut storage = PersistentStorage::new(config.clone()).unwrap();
            storage.put("key1".to_string(), b"value1".to_vec()).unwrap();
            storage.put("key2".to_string(), b"value2".to_vec()).unwrap();
            storage.snapshot().unwrap();
        }

        // Reload and verify
        {
            let storage = PersistentStorage::new(config).unwrap();
            assert_eq!(storage.get("key1").unwrap(), b"value1");
            assert_eq!(storage.get("key2").unwrap(), b"value2");
        }
    }
}
