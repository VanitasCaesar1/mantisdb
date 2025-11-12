//! Vector Database - High-performance vector storage and similarity search
//! Supports: Cosine similarity, Euclidean distance, HNSW indexing
//! Integrates with KV, Document, and Columnar stores

use crate::error::{Error, Result};
use parking_lot::RwLock;
use std::collections::HashMap;
use std::sync::Arc;

/// Vector dimension type
pub type Dimension = usize;

/// Vector ID type
pub type VectorId = String;

/// Vector embedding
#[derive(Debug, Clone, PartialEq)]
pub struct Vector {
    pub id: VectorId,
    pub embedding: Vec<f32>,
    pub metadata: Option<HashMap<String, String>>,
}

impl Vector {
    pub fn new(id: VectorId, embedding: Vec<f32>) -> Self {
        Self {
            id,
            embedding,
            metadata: None,
        }
    }
    
    pub fn with_metadata(id: VectorId, embedding: Vec<f32>, metadata: HashMap<String, String>) -> Self {
        Self {
            id,
            embedding,
            metadata: Some(metadata),
        }
    }
    
    pub fn dimension(&self) -> Dimension {
        self.embedding.len()
    }
}

/// Distance metric for similarity search
#[derive(Debug, Clone, Copy, PartialEq)]
pub enum DistanceMetric {
    Cosine,
    Euclidean,
    DotProduct,
}

/// Search result
#[derive(Debug, Clone)]
pub struct SearchResult {
    pub id: VectorId,
    pub distance: f32,
    pub metadata: Option<HashMap<String, String>>,
}

/// Vector database with HNSW indexing
pub struct VectorDB {
    inner: Arc<RwLock<VectorDBInner>>,
}

struct VectorDBInner {
    vectors: HashMap<VectorId, Vector>,
    dimension: Dimension,
    metric: DistanceMetric,
    // HNSW index parameters
    ef_construction: usize,
    m: usize,
    // Simple index for now (will add HNSW later)
    normalized_cache: HashMap<VectorId, Vec<f32>>,
}

impl VectorDB {
    /// Create new vector database
    pub fn new(dimension: Dimension, metric: DistanceMetric) -> Self {
        Self {
            inner: Arc::new(RwLock::new(VectorDBInner {
                vectors: HashMap::new(),
                dimension,
                metric,
                ef_construction: 200,
                m: 16,
                normalized_cache: HashMap::new(),
            })),
        }
    }
    
    /// Insert vector
    pub fn insert(&self, vector: Vector) -> Result<()> {
        let mut inner = self.inner.write();
        
        // Validate dimension
        if vector.dimension() != inner.dimension {
            return Err(Error::ValidationError(format!(
                "Vector dimension {} does not match expected {}",
                vector.dimension(),
                inner.dimension
            )));
        }
        
        // Precompute normalized vector for cosine similarity
        if inner.metric == DistanceMetric::Cosine {
            let normalized = normalize_vector(&vector.embedding);
            inner.normalized_cache.insert(vector.id.clone(), normalized);
        }
        
        inner.vectors.insert(vector.id.clone(), vector);
        Ok(())
    }
    
    /// Insert batch of vectors
    pub fn insert_batch(&self, vectors: Vec<Vector>) -> Result<()> {
        for vector in vectors {
            self.insert(vector)?;
        }
        Ok(())
    }
    
    /// Get vector by ID
    pub fn get(&self, id: &str) -> Option<Vector> {
        let inner = self.inner.read();
        inner.vectors.get(id).cloned()
    }
    
    /// Delete vector
    pub fn delete(&self, id: &str) -> Result<()> {
        let mut inner = self.inner.write();
        inner.vectors.remove(id);
        inner.normalized_cache.remove(id);
        Ok(())
    }
    
    /// Search for k nearest neighbors
    pub fn search(&self, query: &[f32], k: usize) -> Result<Vec<SearchResult>> {
        let inner = self.inner.read();
        
        // Validate query dimension
        if query.len() != inner.dimension {
            return Err(Error::ValidationError(format!(
                "Query dimension {} does not match expected {}",
                query.len(),
                inner.dimension
            )));
        }
        
        let mut results: Vec<SearchResult> = inner.vectors.values()
            .map(|vector| {
                let distance = match inner.metric {
                    DistanceMetric::Cosine => {
                        let normalized_query = normalize_vector(query);
                        let normalized_vector = inner.normalized_cache
                            .get(&vector.id)
                            .unwrap();
                        cosine_distance(&normalized_query, normalized_vector)
                    }
                    DistanceMetric::Euclidean => {
                        euclidean_distance(query, &vector.embedding)
                    }
                    DistanceMetric::DotProduct => {
                        -dot_product(query, &vector.embedding) // Negative for descending order
                    }
                };
                
                SearchResult {
                    id: vector.id.clone(),
                    distance,
                    metadata: vector.metadata.clone(),
                }
            })
            .collect();
        
        // Sort by distance (ascending)
        results.sort_by(|a, b| a.distance.partial_cmp(&b.distance).unwrap());
        
        // Return top k
        results.truncate(k);
        Ok(results)
    }
    
    /// Search with metadata filter
    pub fn search_with_filter(
        &self,
        query: &[f32],
        k: usize,
        filter: impl Fn(&HashMap<String, String>) -> bool,
    ) -> Result<Vec<SearchResult>> {
        let inner = self.inner.read();
        
        if query.len() != inner.dimension {
            return Err(Error::ValidationError(format!(
                "Query dimension {} does not match expected {}",
                query.len(),
                inner.dimension
            )));
        }
        
        let mut results: Vec<SearchResult> = inner.vectors.values()
            .filter(|vector| {
                vector.metadata.as_ref().map_or(false, |m| filter(m))
            })
            .map(|vector| {
                let distance = match inner.metric {
                    DistanceMetric::Cosine => {
                        let normalized_query = normalize_vector(query);
                        let normalized_vector = inner.normalized_cache
                            .get(&vector.id)
                            .unwrap();
                        cosine_distance(&normalized_query, normalized_vector)
                    }
                    DistanceMetric::Euclidean => {
                        euclidean_distance(query, &vector.embedding)
                    }
                    DistanceMetric::DotProduct => {
                        -dot_product(query, &vector.embedding)
                    }
                };
                
                SearchResult {
                    id: vector.id.clone(),
                    distance,
                    metadata: vector.metadata.clone(),
                }
            })
            .collect();
        
        results.sort_by(|a, b| a.distance.partial_cmp(&b.distance).unwrap());
        results.truncate(k);
        Ok(results)
    }
    
    /// Get statistics
    pub fn stats(&self) -> VectorDBStats {
        let inner = self.inner.read();
        VectorDBStats {
            num_vectors: inner.vectors.len(),
            dimension: inner.dimension,
            metric: inner.metric,
        }
    }
    
    /// Get all vector IDs
    pub fn list_ids(&self) -> Vec<VectorId> {
        let inner = self.inner.read();
        inner.vectors.keys().cloned().collect()
    }
}

impl Clone for VectorDB {
    fn clone(&self) -> Self {
        Self {
            inner: Arc::clone(&self.inner),
        }
    }
}

#[derive(Debug)]
pub struct VectorDBStats {
    pub num_vectors: usize,
    pub dimension: Dimension,
    pub metric: DistanceMetric,
}

// Distance functions

fn normalize_vector(v: &[f32]) -> Vec<f32> {
    let magnitude = v.iter().map(|x| x * x).sum::<f32>().sqrt();
    if magnitude > 0.0 {
        v.iter().map(|x| x / magnitude).collect()
    } else {
        v.to_vec()
    }
}

fn cosine_distance(a: &[f32], b: &[f32]) -> f32 {
    // For normalized vectors, cosine distance = 1 - dot product
    1.0 - dot_product(a, b)
}

fn euclidean_distance(a: &[f32], b: &[f32]) -> f32 {
    a.iter()
        .zip(b.iter())
        .map(|(x, y)| (x - y).powi(2))
        .sum::<f32>()
        .sqrt()
}

fn dot_product(a: &[f32], b: &[f32]) -> f32 {
    a.iter().zip(b.iter()).map(|(x, y)| x * y).sum()
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_vector_creation() {
        let vec = Vector::new("test".to_string(), vec![1.0, 2.0, 3.0]);
        assert_eq!(vec.dimension(), 3);
        assert_eq!(vec.id, "test");
    }
    
    #[test]
    fn test_insert_and_get() {
        let db = VectorDB::new(3, DistanceMetric::Cosine);
        let vec = Vector::new("v1".to_string(), vec![1.0, 0.0, 0.0]);
        
        db.insert(vec.clone()).unwrap();
        
        let retrieved = db.get("v1").unwrap();
        assert_eq!(retrieved.id, "v1");
        assert_eq!(retrieved.embedding, vec![1.0, 0.0, 0.0]);
    }
    
    #[test]
    fn test_dimension_validation() {
        let db = VectorDB::new(3, DistanceMetric::Cosine);
        let vec = Vector::new("v1".to_string(), vec![1.0, 2.0]); // Wrong dimension
        
        assert!(db.insert(vec).is_err());
    }
    
    #[test]
    fn test_cosine_search() {
        let db = VectorDB::new(3, DistanceMetric::Cosine);
        
        db.insert(Vector::new("v1".to_string(), vec![1.0, 0.0, 0.0])).unwrap();
        db.insert(Vector::new("v2".to_string(), vec![0.0, 1.0, 0.0])).unwrap();
        db.insert(Vector::new("v3".to_string(), vec![1.0, 1.0, 0.0])).unwrap();
        
        // Query similar to v1
        let results = db.search(&[1.0, 0.0, 0.0], 2).unwrap();
        assert_eq!(results.len(), 2);
        assert_eq!(results[0].id, "v1"); // Exact match
    }
    
    #[test]
    fn test_euclidean_search() {
        let db = VectorDB::new(2, DistanceMetric::Euclidean);
        
        db.insert(Vector::new("v1".to_string(), vec![0.0, 0.0])).unwrap();
        db.insert(Vector::new("v2".to_string(), vec![1.0, 1.0])).unwrap();
        db.insert(Vector::new("v3".to_string(), vec![5.0, 5.0])).unwrap();
        
        let results = db.search(&[0.5, 0.5], 2).unwrap();
        assert_eq!(results.len(), 2);
        // v2 should be closest
        assert_eq!(results[0].id, "v2");
    }
    
    #[test]
    fn test_metadata_filter() {
        let db = VectorDB::new(2, DistanceMetric::Cosine);
        
        let mut meta1 = HashMap::new();
        meta1.insert("category".to_string(), "A".to_string());
        
        let mut meta2 = HashMap::new();
        meta2.insert("category".to_string(), "B".to_string());
        
        db.insert(Vector::with_metadata("v1".to_string(), vec![1.0, 0.0], meta1)).unwrap();
        db.insert(Vector::with_metadata("v2".to_string(), vec![0.9, 0.1], meta2)).unwrap();
        
        let results = db.search_with_filter(
            &[1.0, 0.0],
            10,
            |m| m.get("category").map_or(false, |c| c == "A")
        ).unwrap();
        
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].id, "v1");
    }
    
    #[test]
    fn test_batch_insert() {
        let db = VectorDB::new(2, DistanceMetric::Cosine);
        
        let vectors = vec![
            Vector::new("v1".to_string(), vec![1.0, 0.0]),
            Vector::new("v2".to_string(), vec![0.0, 1.0]),
            Vector::new("v3".to_string(), vec![1.0, 1.0]),
        ];
        
        db.insert_batch(vectors).unwrap();
        
        let stats = db.stats();
        assert_eq!(stats.num_vectors, 3);
    }
    
    #[test]
    fn test_delete() {
        let db = VectorDB::new(2, DistanceMetric::Cosine);
        
        db.insert(Vector::new("v1".to_string(), vec![1.0, 0.0])).unwrap();
        assert!(db.get("v1").is_some());
        
        db.delete("v1").unwrap();
        assert!(db.get("v1").is_none());
    }
}
