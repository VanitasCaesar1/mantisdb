// WAL Manager - Write-Ahead Logging
use super::entry::*;
use crate::error::MantisError;
use std::fs::{File, OpenOptions};
use std::io::{BufWriter, Write, BufReader, Read};
use std::path::{Path, PathBuf};
use std::sync::Arc;
use std::sync::atomic::{AtomicU64, Ordering};
use parking_lot::Mutex;

pub struct WalManager {
    wal_dir: PathBuf,
    current_segment: Arc<Mutex<WalSegment>>,
    current_lsn: Arc<AtomicU64>,
    segment_size: u64,
    sync_on_commit: bool,
}

struct WalSegment {
    file: BufWriter<File>,
    segment_id: u64,
    size: u64,
}

impl WalManager {
    pub fn new(wal_dir: impl AsRef<Path>, segment_size: u64) -> Result<Self, MantisError> {
        let wal_dir = wal_dir.as_ref().to_path_buf();
        
        // Create WAL directory if it doesn't exist
        std::fs::create_dir_all(&wal_dir)
            .map_err(|e| MantisError::IoError(format!("Failed to create WAL directory: {}", e)))?;
        
        // Find the latest segment or create a new one
        let (segment_id, current_lsn) = Self::find_latest_segment(&wal_dir)?;
        
        let segment_path = Self::segment_path(&wal_dir, segment_id);
        let file = OpenOptions::new()
            .create(true)
            .append(true)
            .open(&segment_path)
            .map_err(|e| MantisError::IoError(format!("Failed to open WAL segment: {}", e)))?;
        
        let size = file.metadata()
            .map_err(|e| MantisError::IoError(format!("Failed to get file metadata: {}", e)))?
            .len();
        
        let segment = WalSegment {
            file: BufWriter::new(file),
            segment_id,
            size,
        };
        
        Ok(WalManager {
            wal_dir,
            current_segment: Arc::new(Mutex::new(segment)),
            current_lsn: Arc::new(AtomicU64::new(current_lsn)),
            segment_size,
            sync_on_commit: true,
        })
    }
    
    pub fn append(&self, txn_id: u64, entry_type: WalEntryType) -> Result<LogSequenceNumber, MantisError> {
        let lsn = LogSequenceNumber::new(self.current_lsn.fetch_add(1, Ordering::SeqCst));
        let entry = WalEntry::new(txn_id, lsn, entry_type);
        
        let data = entry.serialize()
            .map_err(|e| MantisError::SerializationError(format!("Failed to serialize WAL entry: {}", e)))?;
        
        let mut segment = self.current_segment.lock();
        
        // Check if we need to rotate to a new segment
        if segment.size + data.len() as u64 > self.segment_size {
            self.rotate_segment(&mut segment)?;
        }
        
        // Write length prefix
        let len = data.len() as u32;
        segment.file.write_all(&len.to_le_bytes())
            .map_err(|e| MantisError::IoError(format!("Failed to write WAL entry length: {}", e)))?;
        
        // Write entry data
        segment.file.write_all(&data)
            .map_err(|e| MantisError::IoError(format!("Failed to write WAL entry: {}", e)))?;
        
        segment.size += 4 + data.len() as u64;
        
        Ok(lsn)
    }
    
    pub fn sync(&self) -> Result<(), MantisError> {
        let mut segment = self.current_segment.lock();
        segment.file.flush()
            .map_err(|e| MantisError::IoError(format!("Failed to flush WAL: {}", e)))?;
        
        segment.file.get_ref().sync_all()
            .map_err(|e| MantisError::IoError(format!("Failed to sync WAL: {}", e)))?;
        
        Ok(())
    }
    
    pub fn commit(&self, txn_id: u64) -> Result<LogSequenceNumber, MantisError> {
        let lsn = self.append(txn_id, WalEntryType::CommitTransaction)?;
        
        if self.sync_on_commit {
            self.sync()?;
        }
        
        Ok(lsn)
    }
    
    pub fn abort(&self, txn_id: u64) -> Result<LogSequenceNumber, MantisError> {
        self.append(txn_id, WalEntryType::AbortTransaction)
    }
    
    pub fn checkpoint(&self, active_txns: Vec<u64>) -> Result<LogSequenceNumber, MantisError> {
        let lsn = LogSequenceNumber::new(self.current_lsn.load(Ordering::SeqCst));
        self.append(0, WalEntryType::Checkpoint { lsn, active_txns })?;
        self.sync()?;
        Ok(lsn)
    }
    
    pub fn read_from(&self, start_lsn: LogSequenceNumber) -> Result<Vec<WalEntry>, MantisError> {
        let mut entries = Vec::new();
        
        // Find the segment containing start_lsn
        let segment_id = self.find_segment_for_lsn(start_lsn)?;
        
        // Read from that segment onwards
        for sid in segment_id.. {
            let segment_path = Self::segment_path(&self.wal_dir, sid);
            if !segment_path.exists() {
                break;
            }
            
            let file = File::open(&segment_path)
                .map_err(|e| MantisError::IoError(format!("Failed to open WAL segment: {}", e)))?;
            
            let mut reader = BufReader::new(file);
            
            loop {
                // Read length prefix
                let mut len_buf = [0u8; 4];
                match reader.read_exact(&mut len_buf) {
                    Ok(_) => {},
                    Err(e) if e.kind() == std::io::ErrorKind::UnexpectedEof => break,
                    Err(e) => return Err(MantisError::IoError(format!("Failed to read WAL entry length: {}", e))),
                }
                
                let len = u32::from_le_bytes(len_buf) as usize;
                
                // Read entry data
                let mut data = vec![0u8; len];
                reader.read_exact(&mut data)
                    .map_err(|e| MantisError::IoError(format!("Failed to read WAL entry: {}", e)))?;
                
                let entry = WalEntry::deserialize(&data)
                    .map_err(|e| MantisError::SerializationError(format!("Failed to deserialize WAL entry: {}", e)))?;
                
                if entry.lsn >= start_lsn {
                    entries.push(entry);
                }
            }
        }
        
        Ok(entries)
    }
    
    fn rotate_segment(&self, current: &mut WalSegment) -> Result<(), MantisError> {
        // Flush and close current segment
        current.file.flush()
            .map_err(|e| MantisError::IoError(format!("Failed to flush WAL: {}", e)))?;
        
        // Create new segment
        let new_segment_id = current.segment_id + 1;
        let segment_path = Self::segment_path(&self.wal_dir, new_segment_id);
        
        let file = OpenOptions::new()
            .create(true)
            .append(true)
            .open(&segment_path)
            .map_err(|e| MantisError::IoError(format!("Failed to create new WAL segment: {}", e)))?;
        
        *current = WalSegment {
            file: BufWriter::new(file),
            segment_id: new_segment_id,
            size: 0,
        };
        
        Ok(())
    }
    
    fn segment_path(wal_dir: &Path, segment_id: u64) -> PathBuf {
        wal_dir.join(format!("wal-{:016x}.log", segment_id))
    }
    
    fn find_latest_segment(wal_dir: &Path) -> Result<(u64, u64), MantisError> {
        let mut max_segment_id = 0u64;
        let mut max_lsn = 0u64;
        
        if let Ok(entries) = std::fs::read_dir(wal_dir) {
            for entry in entries.flatten() {
                if let Some(filename) = entry.file_name().to_str() {
                    if filename.starts_with("wal-") && filename.ends_with(".log") {
                        if let Ok(segment_id) = u64::from_str_radix(
                            &filename[4..20],
                            16
                        ) {
                            if segment_id > max_segment_id {
                                max_segment_id = segment_id;
                            }
                        }
                    }
                }
            }
        }
        
        // Scan the latest segment to find max LSN
        if max_segment_id > 0 {
            let segment_path = Self::segment_path(wal_dir, max_segment_id);
            if let Ok(file) = File::open(segment_path) {
                let mut reader = BufReader::new(file);
                
                loop {
                    let mut len_buf = [0u8; 4];
                    if reader.read_exact(&mut len_buf).is_err() {
                        break;
                    }
                    
                    let len = u32::from_le_bytes(len_buf) as usize;
                    let mut data = vec![0u8; len];
                    
                    if reader.read_exact(&mut data).is_err() {
                        break;
                    }
                    
                    if let Ok(entry) = WalEntry::deserialize(&data) {
                        if entry.lsn.as_u64() > max_lsn {
                            max_lsn = entry.lsn.as_u64();
                        }
                    }
                }
            }
        }
        
        Ok((max_segment_id, max_lsn + 1))
    }
    
    fn find_segment_for_lsn(&self, _lsn: LogSequenceNumber) -> Result<u64, MantisError> {
        // Simple implementation: start from segment 0
        // In production, you'd maintain an index
        Ok(0)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::TempDir;
    
    #[test]
    fn test_wal_append_and_read() {
        let temp_dir = TempDir::new().unwrap();
        let wal = WalManager::new(temp_dir.path(), 1024 * 1024).unwrap();
        
        let lsn = wal.append(1, WalEntryType::BeginTransaction).unwrap();
        wal.sync().unwrap();
        
        let entries = wal.read_from(lsn).unwrap();
        assert_eq!(entries.len(), 1);
    }
}
