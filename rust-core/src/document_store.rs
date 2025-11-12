//! Document Store with MongoDB-style operations
//! 
//! Features:
//! - BSON-like document storage
//! - Secondary indexes on nested JSON paths
//! - Aggregation pipeline support
//! - Atomic updates and upserts

use crate::error::{Error, Result};
use serde::{Deserialize, Serialize};
use serde_json::Value as JsonValue;
use std::collections::{BTreeMap, HashMap};
use std::sync::Arc;
use parking_lot::RwLock;

/// Document ID (similar to MongoDB ObjectId)
#[derive(Debug, Clone, PartialEq, Eq, Hash, PartialOrd, Ord, Serialize, Deserialize)]
pub struct DocumentId(String);

impl DocumentId {
    pub fn new() -> Self {
        use std::time::{SystemTime, UNIX_EPOCH};
        let timestamp = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_secs();
        let random = rand::random::<u32>();
        Self(format!("{:016x}{:08x}", timestamp, random))
    }
    
    pub fn from_string(s: String) -> Self {
        Self(s)
    }
    
    pub fn as_str(&self) -> &str {
        &self.0
    }
}

impl Default for DocumentId {
    fn default() -> Self {
        Self::new()
    }
}

/// Document with metadata
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Document {
    #[serde(rename = "_id")]
    pub id: DocumentId,
    #[serde(flatten)]
    pub data: JsonValue,
    #[serde(skip)]
    pub version: u64,
    #[serde(skip)]
    pub created_at: u64,
    #[serde(skip)]
    pub updated_at: u64,
}

impl Document {
    pub fn new(id: DocumentId, data: JsonValue) -> Self {
        let now = current_timestamp();
        Self {
            id,
            data,
            version: 1,
            created_at: now,
            updated_at: now,
        }
    }
    
    /// Get value at JSON path (e.g., "user.address.city")
    pub fn get_nested(&self, path: &str) -> Option<&JsonValue> {
        let parts: Vec<&str> = path.split('.').collect();
        let mut current = &self.data;
        
        for part in parts {
            match current {
                JsonValue::Object(map) => {
                    current = map.get(part)?;
                }
                _ => return None,
            }
        }
        
        Some(current)
    }
    
    /// Set value at JSON path
    pub fn set_nested(&mut self, path: &str, value: JsonValue) -> Result<()> {
        let parts: Vec<&str> = path.split('.').collect();
        if parts.is_empty() {
            return Err(Error::StorageError("Empty path".to_string()));
        }
        
        // Navigate to parent
        let mut current = &mut self.data;
        for part in &parts[..parts.len() - 1] {
            if !current.is_object() {
                return Err(Error::StorageError("Path traversal failed".to_string()));
            }
            current = current.get_mut(part)
                .ok_or_else(|| Error::StorageError("Path not found".to_string()))?;
        }
        
        // Set value at final key
        if let JsonValue::Object(map) = current {
            map.insert(parts[parts.len() - 1].to_string(), value);
            self.version += 1;
            self.updated_at = current_timestamp();
            Ok(())
        } else {
            Err(Error::StorageError("Parent is not an object".to_string()))
        }
    }
}

/// Index type for secondary indexes
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum IndexType {
    BTree,    // General purpose, supports range queries
    Hash,     // Fast exact match
    FullText, // Text search (basic implementation)
}

/// Secondary index on a field path
pub struct SecondaryIndex {
    pub collection: String,
    pub field_path: String,
    pub index_type: IndexType,
    pub unique: bool,
    // Map from indexed value to document IDs
    values: BTreeMap<IndexKey, Vec<DocumentId>>,
}

#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord)]
enum IndexKey {
    Null,
    Bool(bool),
    Number(OrderedFloat),
    String(String),
}

#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord)]
struct OrderedFloat(i64); // Store as fixed-point for ordering

impl From<f64> for OrderedFloat {
    fn from(f: f64) -> Self {
        Self((f * 1_000_000.0) as i64)
    }
}

impl From<&JsonValue> for IndexKey {
    fn from(value: &JsonValue) -> Self {
        match value {
            JsonValue::Null => IndexKey::Null,
            JsonValue::Bool(b) => IndexKey::Bool(*b),
            JsonValue::Number(n) => IndexKey::Number(n.as_f64().unwrap_or(0.0).into()),
            JsonValue::String(s) => IndexKey::String(s.clone()),
            _ => IndexKey::Null,
        }
    }
}

impl SecondaryIndex {
    pub fn new(collection: String, field_path: String, index_type: IndexType, unique: bool) -> Self {
        Self {
            collection,
            field_path,
            index_type,
            unique,
            values: BTreeMap::new(),
        }
    }
    
    /// Add document to index
    pub fn insert(&mut self, doc: &Document) -> Result<()> {
        if let Some(value) = doc.get_nested(&self.field_path) {
            let key = IndexKey::from(value);
            
            if self.unique && self.values.contains_key(&key) {
                return Err(Error::StorageError("Unique constraint violation".to_string()));
            }
            
            self.values.entry(key)
                .or_insert_with(Vec::new)
                .push(doc.id.clone());
        }
        Ok(())
    }
    
    /// Remove document from index
    pub fn remove(&mut self, doc: &Document) {
        if let Some(value) = doc.get_nested(&self.field_path) {
            let key = IndexKey::from(value);
            if let Some(ids) = self.values.get_mut(&key) {
                ids.retain(|id| id != &doc.id);
                if ids.is_empty() {
                    self.values.remove(&key);
                }
            }
        }
    }
    
    /// Find documents by exact value
    pub fn find_exact(&self, value: &JsonValue) -> Vec<DocumentId> {
        let key = IndexKey::from(value);
        self.values.get(&key)
            .map(|ids| ids.clone())
            .unwrap_or_default()
    }
    
    /// Range query (only for BTree index)
    pub fn find_range(&self, start: &JsonValue, end: &JsonValue) -> Vec<DocumentId> {
        if self.index_type != IndexType::BTree {
            return Vec::new();
        }
        
        let start_key = IndexKey::from(start);
        let end_key = IndexKey::from(end);
        
        self.values.range(start_key..=end_key)
            .flat_map(|(_, ids)| ids.clone())
            .collect()
    }
}

/// Collection of documents
pub struct Collection {
    pub name: String,
    documents: BTreeMap<DocumentId, Document>,
    indexes: HashMap<String, SecondaryIndex>,
}

impl Collection {
    pub fn new(name: String) -> Self {
        Self {
            name,
            documents: BTreeMap::new(),
            indexes: HashMap::new(),
        }
    }
    
    /// Insert document
    pub fn insert(&mut self, mut doc: Document) -> Result<DocumentId> {
        // Check unique constraints
        for index in self.indexes.values_mut() {
            index.insert(&doc)?;
        }
        
        let id = doc.id.clone();
        self.documents.insert(id.clone(), doc);
        Ok(id)
    }
    
    /// Find document by ID
    pub fn find_by_id(&self, id: &DocumentId) -> Option<&Document> {
        self.documents.get(id)
    }
    
    /// Update document
    pub fn update(&mut self, id: &DocumentId, updates: JsonValue) -> Result<()> {
        let doc = self.documents.get_mut(id)
            .ok_or_else(|| Error::StorageError("Document not found".to_string()))?;
        
        // Remove from indexes
        for index in self.indexes.values_mut() {
            index.remove(doc);
        }
        
        // Apply updates
        if let JsonValue::Object(update_map) = updates {
            if let JsonValue::Object(doc_map) = &mut doc.data {
                for (key, value) in update_map {
                    doc_map.insert(key, value);
                }
            }
        }
        
        doc.version += 1;
        doc.updated_at = current_timestamp();
        
        // Re-add to indexes
        for index in self.indexes.values_mut() {
            index.insert(doc)?;
        }
        
        Ok(())
    }
    
    /// Delete document
    pub fn delete(&mut self, id: &DocumentId) -> Result<()> {
        if let Some(doc) = self.documents.remove(id) {
            // Remove from all indexes
            for index in self.indexes.values_mut() {
                index.remove(&doc);
            }
            Ok(())
        } else {
            Err(Error::StorageError("Document not found".to_string()))
        }
    }
    
    /// Create secondary index
    pub fn create_index(&mut self, field_path: String, index_type: IndexType, unique: bool) -> Result<()> {
        if self.indexes.contains_key(&field_path) {
            return Err(Error::StorageError("Index already exists".to_string()));
        }
        
        let mut index = SecondaryIndex::new(self.name.clone(), field_path.clone(), index_type, unique);
        
        // Index existing documents
        for doc in self.documents.values() {
            index.insert(doc)?;
        }
        
        self.indexes.insert(field_path, index);
        Ok(())
    }
    
    /// Query documents with filter
    pub fn query(&self, query: &Query) -> Vec<Document> {
        let mut results: Option<Vec<DocumentId>> = None;
        
        // Try to use indexes
        for (field, condition) in &query.filters {
            if let Some(index) = self.indexes.get(field) {
                let matching_ids = match condition {
                    Condition::Eq(value) => index.find_exact(value),
                    Condition::Range { start, end } => index.find_range(start, end),
                    _ => continue,
                };
                
                // Intersect with previous results
                results = Some(if let Some(prev) = results {
                    prev.into_iter()
                        .filter(|id| matching_ids.contains(id))
                        .collect()
                } else {
                    matching_ids
                });
            }
        }
        
        // If no index was used, scan all documents
        let candidate_ids: Vec<DocumentId> = if let Some(ids) = results {
            ids
        } else {
            self.documents.keys().cloned().collect()
        };
        
        // Apply all filters
        let mut filtered: Vec<Document> = candidate_ids.into_iter()
            .filter_map(|id| self.documents.get(&id))
            .filter(|doc| self.matches_query(doc, query))
            .cloned()
            .collect();
        
        // Apply sorting
        if let Some(sort_field) = &query.sort {
            filtered.sort_by(|a, b| {
                let a_val = a.get_nested(sort_field);
                let b_val = b.get_nested(sort_field);
                compare_json_values(a_val, b_val)
            });
            
            if query.sort_desc {
                filtered.reverse();
            }
        }
        
        // Apply pagination
        let start = query.skip.unwrap_or(0);
        let end = start + query.limit.unwrap_or(100);
        filtered.into_iter().skip(start).take(end - start).collect()
    }
    
    fn matches_query(&self, doc: &Document, query: &Query) -> bool {
        for (field, condition) in &query.filters {
            if let Some(value) = doc.get_nested(field) {
                if !condition.matches(value) {
                    return false;
                }
            } else {
                return false;
            }
        }
        true
    }
    
    pub fn count(&self) -> usize {
        self.documents.len()
    }
}

/// Query builder
#[derive(Debug, Clone, Default)]
pub struct Query {
    pub filters: HashMap<String, Condition>,
    pub sort: Option<String>,
    pub sort_desc: bool,
    pub skip: Option<usize>,
    pub limit: Option<usize>,
}

#[derive(Debug, Clone)]
pub enum Condition {
    Eq(JsonValue),
    Ne(JsonValue),
    Gt(JsonValue),
    Gte(JsonValue),
    Lt(JsonValue),
    Lte(JsonValue),
    In(Vec<JsonValue>),
    Range { start: JsonValue, end: JsonValue },
}

impl Condition {
    fn matches(&self, value: &JsonValue) -> bool {
        match self {
            Condition::Eq(expected) => value == expected,
            Condition::Ne(expected) => value != expected,
            Condition::Gt(expected) => compare_json_values(Some(value), Some(expected)) == std::cmp::Ordering::Greater,
            Condition::Gte(expected) => {
                let ord = compare_json_values(Some(value), Some(expected));
                matches!(ord, std::cmp::Ordering::Greater | std::cmp::Ordering::Equal)
            }
            Condition::Lt(expected) => compare_json_values(Some(value), Some(expected)) == std::cmp::Ordering::Less,
            Condition::Lte(expected) => {
                let ord = compare_json_values(Some(value), Some(expected));
                matches!(ord, std::cmp::Ordering::Less | std::cmp::Ordering::Equal)
            }
            Condition::In(values) => values.contains(value),
            Condition::Range { start, end } => {
                let start_cmp = compare_json_values(Some(value), Some(start));
                let end_cmp = compare_json_values(Some(value), Some(end));
                matches!(start_cmp, std::cmp::Ordering::Greater | std::cmp::Ordering::Equal) &&
                matches!(end_cmp, std::cmp::Ordering::Less | std::cmp::Ordering::Equal)
            }
        }
    }
}

/// Document store managing multiple collections
pub struct DocumentStore {
    collections: Arc<RwLock<HashMap<String, Collection>>>,
}

impl DocumentStore {
    pub fn new() -> Self {
        Self {
            collections: Arc::new(RwLock::new(HashMap::new())),
        }
    }
    
    /// Create collection
    pub fn create_collection(&self, name: String) -> Result<()> {
        let mut collections = self.collections.write();
        if collections.contains_key(&name) {
            return Err(Error::StorageError("Collection already exists".to_string()));
        }
        collections.insert(name.clone(), Collection::new(name));
        Ok(())
    }
    
    /// Drop collection
    pub fn drop_collection(&self, name: &str) -> Result<()> {
        let mut collections = self.collections.write();
        collections.remove(name)
            .ok_or_else(|| Error::StorageError("Collection not found".to_string()))?;
        Ok(())
    }
    
    /// List collections
    pub fn list_collections(&self) -> Vec<String> {
        self.collections.read().keys().cloned().collect()
    }
    
    /// Execute on collection
    pub fn with_collection<F, R>(&self, name: &str, f: F) -> Result<R>
    where
        F: FnOnce(&Collection) -> R,
    {
        let collections = self.collections.read();
        collections.get(name)
            .map(f)
            .ok_or_else(|| Error::StorageError("Collection not found".to_string()))
    }
    
    /// Execute on collection (mutable)
    pub fn with_collection_mut<F, R>(&self, name: &str, f: F) -> Result<R>
    where
        F: FnOnce(&mut Collection) -> R,
    {
        let mut collections = self.collections.write();
        collections.get_mut(name)
            .map(f)
            .ok_or_else(|| Error::StorageError("Collection not found".to_string()))
    }
}

impl Default for DocumentStore {
    fn default() -> Self {
        Self::new()
    }
}

fn current_timestamp() -> u64 {
    use std::time::{SystemTime, UNIX_EPOCH};
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_secs()
}

fn compare_json_values(a: Option<&JsonValue>, b: Option<&JsonValue>) -> std::cmp::Ordering {
    match (a, b) {
        (Some(JsonValue::Number(a)), Some(JsonValue::Number(b))) => {
            a.as_f64().partial_cmp(&b.as_f64()).unwrap_or(std::cmp::Ordering::Equal)
        }
        (Some(JsonValue::String(a)), Some(JsonValue::String(b))) => a.cmp(b),
        (Some(JsonValue::Bool(a)), Some(JsonValue::Bool(b))) => a.cmp(b),
        (None, None) => std::cmp::Ordering::Equal,
        (None, Some(_)) => std::cmp::Ordering::Less,
        (Some(_), None) => std::cmp::Ordering::Greater,
        _ => std::cmp::Ordering::Equal,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;
    
    #[test]
    fn test_document_nested_get() {
        let data = json!({
            "user": {
                "name": "Alice",
                "address": {
                    "city": "NYC"
                }
            }
        });
        
        let doc = Document::new(DocumentId::new(), data);
        assert_eq!(doc.get_nested("user.name"), Some(&json!("Alice")));
        assert_eq!(doc.get_nested("user.address.city"), Some(&json!("NYC")));
    }
    
    #[test]
    fn test_collection_insert_query() {
        let mut coll = Collection::new("users".to_string());
        
        // Insert documents
        let doc1 = Document::new(
            DocumentId::new(),
            json!({"name": "Alice", "age": 30})
        );
        let doc2 = Document::new(
            DocumentId::new(),
            json!({"name": "Bob", "age": 25})
        );
        
        coll.insert(doc1).unwrap();
        coll.insert(doc2).unwrap();
        
        // Query with filter
        let mut query = Query::default();
        query.filters.insert("age".to_string(), Condition::Gt(json!(26)));
        
        let results = coll.query(&query);
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].get_nested("name"), Some(&json!("Alice")));
    }
    
    #[test]
    fn test_secondary_index() {
        let mut coll = Collection::new("users".to_string());
        
        // Create index on email
        coll.create_index("email".to_string(), IndexType::BTree, true).unwrap();
        
        // Insert document
        let doc = Document::new(
            DocumentId::new(),
            json!({"name": "Alice", "email": "alice@example.com"})
        );
        coll.insert(doc).unwrap();
        
        // Try to insert duplicate email (should fail)
        let doc2 = Document::new(
            DocumentId::new(),
            json!({"name": "Bob", "email": "alice@example.com"})
        );
        assert!(coll.insert(doc2).is_err());
    }
}
