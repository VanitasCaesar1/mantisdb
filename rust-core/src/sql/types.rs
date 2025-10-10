// SQL Type System
use serde::{Deserialize, Serialize};
use std::fmt;

#[derive(Debug, Clone, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub enum SqlType {
    // Numeric types
    Integer,
    BigInt,
    SmallInt,
    Real,
    Double,
    Decimal { precision: u8, scale: u8 },
    
    // String types
    Char { length: u32 },
    Varchar { length: u32 },
    Text,
    
    // Binary types
    Binary { length: u32 },
    Varbinary { length: u32 },
    Blob,
    
    // Date/Time types
    Date,
    Time,
    Timestamp,
    Interval,
    
    // Boolean
    Boolean,
    
    // JSON
    Json,
    Jsonb,
    
    // Array
    Array(Box<SqlType>),
    
    // Null
    Null,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub enum SqlValue {
    Null,
    Integer(i32),
    BigInt(i64),
    SmallInt(i16),
    Real(f32),
    Double(f64),
    Decimal(String),
    Char(String),
    Varchar(String),
    Text(String),
    Binary(Vec<u8>),
    Blob(Vec<u8>),
    Date(i32), // Days since epoch
    Time(i64), // Microseconds since midnight
    Timestamp(i64), // Microseconds since epoch
    Boolean(bool),
    Json(serde_json::Value),
    Jsonb(Vec<u8>),
    Array(Vec<SqlValue>),
}

impl fmt::Display for SqlValue {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            SqlValue::Null => write!(f, "NULL"),
            SqlValue::Integer(v) => write!(f, "{}", v),
            SqlValue::BigInt(v) => write!(f, "{}", v),
            SqlValue::SmallInt(v) => write!(f, "{}", v),
            SqlValue::Real(v) => write!(f, "{}", v),
            SqlValue::Double(v) => write!(f, "{}", v),
            SqlValue::Decimal(v) => write!(f, "{}", v),
            SqlValue::Char(v) | SqlValue::Varchar(v) | SqlValue::Text(v) => write!(f, "'{}'", v),
            SqlValue::Boolean(v) => write!(f, "{}", v),
            SqlValue::Json(v) => write!(f, "{}", v),
            _ => write!(f, "<binary>"),
        }
    }
}

#[derive(Debug, Clone)]
pub struct Column {
    pub name: String,
    pub sql_type: SqlType,
    pub nullable: bool,
    pub default: Option<SqlValue>,
    pub primary_key: bool,
    pub unique: bool,
    pub auto_increment: bool,
}

#[derive(Debug, Clone)]
pub struct Table {
    pub name: String,
    pub columns: Vec<Column>,
    pub primary_keys: Vec<String>,
    pub indexes: Vec<Index>,
}

#[derive(Debug, Clone)]
pub struct Index {
    pub name: String,
    pub table: String,
    pub columns: Vec<String>,
    pub unique: bool,
    pub index_type: IndexType,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum IndexType {
    BTree,
    Hash,
    GIN,
    GiST,
}

#[derive(Debug, Clone)]
pub struct QueryResult {
    pub columns: Vec<String>,
    pub rows: Vec<Vec<SqlValue>>,
    pub rows_affected: u64,
}
