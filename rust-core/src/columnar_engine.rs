//! Column-oriented storage engine
//! 
//! Stores data by column for efficient analytics and scans.
//! Includes per-column compression and vectorized operations.

use crate::error::{Error, Result};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::Arc;
use parking_lot::RwLock;

/// Column data with compression
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ColumnData {
    pub name: String,
    pub data_type: ColumnType,
    /// Raw values stored as bytes
    pub values: Vec<u8>,
    /// Compression codec used
    pub compression: CompressionType,
    /// Number of rows
    pub row_count: usize,
    /// Null bitmap (one bit per row)
    pub null_bitmap: Vec<u8>,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
pub enum ColumnType {
    Int64,
    Float64,
    String,
    Boolean,
    Timestamp,
    Binary,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
pub enum CompressionType {
    None,
    /// Run-length encoding (good for repeated values)
    RLE,
    /// Dictionary encoding (good for low-cardinality strings)
    Dictionary,
    /// Delta encoding (good for sorted/sequential integers)
    Delta,
    /// Bit-packing for integers
    BitPacked,
}

impl ColumnData {
    pub fn new(name: String, data_type: ColumnType) -> Self {
        Self {
            name,
            data_type,
            values: Vec::new(),
            compression: CompressionType::None,
            row_count: 0,
            null_bitmap: Vec::new(),
        }
    }
    
    /// Append a value to the column
    pub fn append_i64(&mut self, value: Option<i64>) -> Result<()> {
        if self.data_type != ColumnType::Int64 {
            return Err(Error::StorageError("Type mismatch".to_string()));
        }
        
        let row_idx = self.row_count;
        self.row_count += 1;
        
        // Update null bitmap
        self.ensure_null_bitmap_size();
        if let Some(v) = value {
            self.values.extend_from_slice(&v.to_le_bytes());
        } else {
            self.set_null_bit(row_idx);
            self.values.extend_from_slice(&0i64.to_le_bytes());
        }
        
        Ok(())
    }
    
    /// Append a string value
    pub fn append_string(&mut self, value: Option<&str>) -> Result<()> {
        if self.data_type != ColumnType::String {
            return Err(Error::StorageError("Type mismatch".to_string()));
        }
        
        let row_idx = self.row_count;
        self.row_count += 1;
        
        self.ensure_null_bitmap_size();
        
        if let Some(s) = value {
            // Length-prefixed string
            let len = s.len() as u32;
            self.values.extend_from_slice(&len.to_le_bytes());
            self.values.extend_from_slice(s.as_bytes());
        } else {
            self.set_null_bit(row_idx);
            self.values.extend_from_slice(&0u32.to_le_bytes());
        }
        
        Ok(())
    }
    
    /// Get i64 value at index
    pub fn get_i64(&self, index: usize) -> Result<Option<i64>> {
        if index >= self.row_count {
            return Err(Error::StorageError("Index out of bounds".to_string()));
        }
        
        if self.is_null(index) {
            return Ok(None);
        }
        
        let offset = index * 8;
        if offset + 8 > self.values.len() {
            return Err(Error::StorageError("Corrupt column data".to_string()));
        }
        
        let mut bytes = [0u8; 8];
        bytes.copy_from_slice(&self.values[offset..offset + 8]);
        Ok(Some(i64::from_le_bytes(bytes)))
    }
    
    /// Get string value at index
    pub fn get_string(&self, index: usize) -> Result<Option<String>> {
        if index >= self.row_count {
            return Err(Error::StorageError("Index out of bounds".to_string()));
        }
        
        if self.is_null(index) {
            return Ok(None);
        }
        
        // Scan to find the string at index
        let mut offset = 0;
        for _ in 0..index {
            if offset + 4 > self.values.len() {
                return Err(Error::StorageError("Corrupt column data".to_string()));
            }
            let mut len_bytes = [0u8; 4];
            len_bytes.copy_from_slice(&self.values[offset..offset + 4]);
            let len = u32::from_le_bytes(len_bytes) as usize;
            offset += 4 + len;
        }
        
        if offset + 4 > self.values.len() {
            return Err(Error::StorageError("Corrupt column data".to_string()));
        }
        
        let mut len_bytes = [0u8; 4];
        len_bytes.copy_from_slice(&self.values[offset..offset + 4]);
        let len = u32::from_le_bytes(len_bytes) as usize;
        offset += 4;
        
        if offset + len > self.values.len() {
            return Err(Error::StorageError("Corrupt column data".to_string()));
        }
        
        let s = String::from_utf8(self.values[offset..offset + len].to_vec())
            .map_err(|e| Error::StorageError(format!("Invalid UTF-8: {}", e)))?;
        
        Ok(Some(s))
    }
    
    /// Check if value at index is null
    pub fn is_null(&self, index: usize) -> bool {
        if index >= self.row_count {
            return false;
        }
        let byte_idx = index / 8;
        let bit_idx = index % 8;
        if byte_idx < self.null_bitmap.len() {
            (self.null_bitmap[byte_idx] & (1 << bit_idx)) != 0
        } else {
            false
        }
    }
    
    fn ensure_null_bitmap_size(&mut self) {
        let needed_bytes = (self.row_count + 8) / 8;
        while self.null_bitmap.len() < needed_bytes {
            self.null_bitmap.push(0);
        }
    }
    
    fn set_null_bit(&mut self, index: usize) {
        let byte_idx = index / 8;
        let bit_idx = index % 8;
        self.null_bitmap[byte_idx] |= 1 << bit_idx;
    }
    
    /// Apply RLE compression (simple implementation)
    pub fn compress_rle(&mut self) -> Result<()> {
        if self.compression != CompressionType::None {
            return Ok(());
        }
        
        // Only compress Int64 for now
        if self.data_type != ColumnType::Int64 {
            return Ok(());
        }
        
        let mut compressed = Vec::new();
        let mut i = 0;
        
        while i < self.row_count {
            if let Ok(Some(value)) = self.get_i64(i) {
                let mut count = 1u32;
                while i + count < self.row_count {
                    if let Ok(Some(next)) = self.get_i64(i + count) {
                        if next == value {
                            count += 1;
                        } else {
                            break;
                        }
                    } else {
                        break;
                    }
                }
                
                // Write run: count + value
                compressed.extend_from_slice(&count.to_le_bytes());
                compressed.extend_from_slice(&value.to_le_bytes());
                i += count as usize;
            } else {
                i += 1;
            }
        }
        
        if compressed.len() < self.values.len() {
            self.values = compressed;
            self.compression = CompressionType::RLE;
        }
        
        Ok(())
    }
    
    /// Decompress RLE data
    pub fn decompress(&mut self) -> Result<()> {
        if self.compression == CompressionType::None {
            return Ok(());
        }
        
        match self.compression {
            CompressionType::RLE => self.decompress_rle(),
            _ => Err(Error::StorageError("Unsupported compression".to_string())),
        }
    }
    
    fn decompress_rle(&mut self) -> Result<()> {
        let mut decompressed = Vec::new();
        let mut offset = 0;
        
        while offset + 12 <= self.values.len() {
            let mut count_bytes = [0u8; 4];
            count_bytes.copy_from_slice(&self.values[offset..offset + 4]);
            let count = u32::from_le_bytes(count_bytes);
            offset += 4;
            
            let mut value_bytes = [0u8; 8];
            value_bytes.copy_from_slice(&self.values[offset..offset + 8]);
            offset += 8;
            
            for _ in 0..count {
                decompressed.extend_from_slice(&value_bytes);
            }
        }
        
        self.values = decompressed;
        self.compression = CompressionType::None;
        Ok(())
    }
}

/// Columnar table with multiple columns
pub struct ColumnarTable {
    pub name: String,
    pub columns: HashMap<String, ColumnData>,
    pub row_count: usize,
}

impl ColumnarTable {
    pub fn new(name: String) -> Self {
        Self {
            name,
            columns: HashMap::new(),
            row_count: 0,
        }
    }
    
    /// Add a column
    pub fn add_column(&mut self, name: String, data_type: ColumnType) {
        self.columns.insert(name.clone(), ColumnData::new(name, data_type));
    }
    
    /// Append a row (all columns must be present)
    pub fn append_row(&mut self, values: HashMap<String, ColumnValue>) -> Result<()> {
        // Verify all columns are present
        for col_name in self.columns.keys() {
            if !values.contains_key(col_name) {
                return Err(Error::StorageError(format!("Missing column: {}", col_name)));
            }
        }
        
        // Append to each column
        for (col_name, value) in values {
            if let Some(column) = self.columns.get_mut(&col_name) {
                match value {
                    ColumnValue::Int64(v) => column.append_i64(v)?,
                    ColumnValue::String(v) => column.append_string(v.as_deref())?,
                    ColumnValue::Null => match column.data_type {
                        ColumnType::Int64 => column.append_i64(None)?,
                        ColumnType::String => column.append_string(None)?,
                        _ => return Err(Error::StorageError("Unsupported type".to_string())),
                    },
                }
            }
        }
        
        self.row_count += 1;
        Ok(())
    }
    
    /// Get column by name
    pub fn get_column(&self, name: &str) -> Option<&ColumnData> {
        self.columns.get(name)
    }
    
    /// Compress all columns
    pub fn compress_all(&mut self) -> Result<()> {
        for column in self.columns.values_mut() {
            column.compress_rle()?;
        }
        Ok(())
    }
}

#[derive(Debug, Clone)]
pub enum ColumnValue {
    Int64(Option<i64>),
    String(Option<String>),
    Null,
}

/// Column store managing multiple tables
pub struct ColumnStore {
    tables: Arc<RwLock<HashMap<String, ColumnarTable>>>,
}

impl ColumnStore {
    pub fn new() -> Self {
        Self {
            tables: Arc::new(RwLock::new(HashMap::new())),
        }
    }
    
    /// Create a new table
    pub fn create_table(&self, name: String) -> Result<()> {
        let mut tables = self.tables.write();
        if tables.contains_key(&name) {
            return Err(Error::StorageError("Table already exists".to_string()));
        }
        tables.insert(name.clone(), ColumnarTable::new(name));
        Ok(())
    }
    
    /// Get a table (read-only)
    pub fn get_table<F, R>(&self, name: &str, f: F) -> Result<R>
    where
        F: FnOnce(&ColumnarTable) -> R,
    {
        let tables = self.tables.read();
        tables.get(name)
            .map(f)
            .ok_or_else(|| Error::StorageError("Table not found".to_string()))
    }
    
    /// Get a table (mutable)
    pub fn get_table_mut<F, R>(&self, name: &str, f: F) -> Result<R>
    where
        F: FnOnce(&mut ColumnarTable) -> R,
    {
        let mut tables = self.tables.write();
        tables.get_mut(name)
            .map(f)
            .ok_or_else(|| Error::StorageError("Table not found".to_string()))
    }
}

impl Default for ColumnStore {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_column_data_i64() {
        let mut col = ColumnData::new("test".to_string(), ColumnType::Int64);
        
        col.append_i64(Some(42)).unwrap();
        col.append_i64(None).unwrap();
        col.append_i64(Some(100)).unwrap();
        
        assert_eq!(col.get_i64(0).unwrap(), Some(42));
        assert_eq!(col.get_i64(1).unwrap(), None);
        assert_eq!(col.get_i64(2).unwrap(), Some(100));
    }
    
    #[test]
    fn test_column_data_string() {
        let mut col = ColumnData::new("test".to_string(), ColumnType::String);
        
        col.append_string(Some("hello")).unwrap();
        col.append_string(None).unwrap();
        col.append_string(Some("world")).unwrap();
        
        assert_eq!(col.get_string(0).unwrap(), Some("hello".to_string()));
        assert_eq!(col.get_string(1).unwrap(), None);
        assert_eq!(col.get_string(2).unwrap(), Some("world".to_string()));
    }
    
    #[test]
    fn test_rle_compression() {
        let mut col = ColumnData::new("test".to_string(), ColumnType::Int64);
        
        // Add repeated values
        for _ in 0..100 {
            col.append_i64(Some(42)).unwrap();
        }
        for _ in 0..50 {
            col.append_i64(Some(99)).unwrap();
        }
        
        let original_size = col.values.len();
        col.compress_rle().unwrap();
        let compressed_size = col.values.len();
        
        assert!(compressed_size < original_size);
        
        // Decompress and verify
        col.decompress().unwrap();
        for i in 0..100 {
            assert_eq!(col.get_i64(i).unwrap(), Some(42));
        }
        for i in 100..150 {
            assert_eq!(col.get_i64(i).unwrap(), Some(99));
        }
    }
}
