//! Full-Text Search Engine
//!
//! High-performance full-text search with inverted index, stemming, and relevance scoring

use crate::error::{Error, Result};
use parking_lot::RwLock;
use std::collections::{HashMap, HashSet, BTreeMap};
use std::sync::Arc;
use serde::{Serialize, Deserialize};

/// Full-text search engine
pub struct FullTextSearch {
    inner: Arc<RwLock<FTSInner>>,
}

struct FTSInner {
    indexes: HashMap<String, InvertedIndex>,
    stop_words: HashSet<String>,
}

/// Inverted index for a collection
struct InvertedIndex {
    // term -> document_id -> term frequency
    index: HashMap<String, HashMap<String, u32>>,
    // document_id -> total terms
    doc_lengths: HashMap<String, u32>,
    // document_id -> field values
    documents: HashMap<String, HashMap<String, String>>,
    // total documents
    total_docs: usize,
    // index configuration
    config: IndexConfig,
}

#[derive(Debug, Clone)]
pub struct IndexConfig {
    pub stemming_enabled: bool,
    pub case_sensitive: bool,
    pub min_term_length: usize,
    pub max_term_length: usize,
    pub boost_fields: HashMap<String, f64>,
}

impl Default for IndexConfig {
    fn default() -> Self {
        Self {
            stemming_enabled: true,
            case_sensitive: false,
            min_term_length: 2,
            max_term_length: 50,
            boost_fields: HashMap::new(),
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SearchQuery {
    pub query: String,
    pub fields: Vec<String>,
    pub limit: usize,
    pub offset: usize,
    pub boost: Option<HashMap<String, f64>>,
    pub phrase: bool,
}

#[derive(Debug, Clone, Serialize)]
pub struct SearchResult {
    pub doc_id: String,
    pub score: f64,
    pub highlights: HashMap<String, Vec<String>>,
    pub fields: HashMap<String, String>,
}

impl FullTextSearch {
    /// Create a new full-text search engine
    pub fn new() -> Self {
        let stop_words = Self::default_stop_words();
        Self {
            inner: Arc::new(RwLock::new(FTSInner {
                indexes: HashMap::new(),
                stop_words,
            })),
        }
    }
    
    /// Create an index for a collection
    pub fn create_index(&self, collection: &str, config: IndexConfig) -> Result<()> {
        let mut inner = self.inner.write();
        
        if inner.indexes.contains_key(collection) {
            return Err(Error::General(format!("Index '{}' already exists", collection)));
        }
        
        inner.indexes.insert(
            collection.to_string(),
            InvertedIndex {
                index: HashMap::new(),
                doc_lengths: HashMap::new(),
                documents: HashMap::new(),
                total_docs: 0,
                config,
            },
        );
        
        Ok(())
    }
    
    /// Index a document
    pub fn index_document(
        &self,
        collection: &str,
        doc_id: &str,
        fields: HashMap<String, String>,
    ) -> Result<()> {
        let mut inner = self.inner.write();
        
        let index = inner.indexes.get_mut(collection)
            .ok_or_else(|| Error::General(format!("Index '{}' not found", collection)))?;
        
        // Remove existing document if present
        if index.documents.contains_key(doc_id) {
            self.remove_doc_from_index(index, doc_id);
        }
        
        // Tokenize and index each field
        let mut total_terms = 0;
        for (field, value) in &fields {
            let tokens = self.tokenize(&value, &index.config, &inner.stop_words);
            total_terms += tokens.len() as u32;
            
            for token in tokens {
                let term_docs = index.index.entry(token.clone()).or_insert_with(HashMap::new);
                let term_freq = term_docs.entry(doc_id.to_string()).or_insert(0);
                *term_freq += 1;
            }
        }
        
        index.doc_lengths.insert(doc_id.to_string(), total_terms);
        index.documents.insert(doc_id.to_string(), fields);
        index.total_docs = index.documents.len();
        
        Ok(())
    }
    
    /// Delete a document from the index
    pub fn delete_document(&self, collection: &str, doc_id: &str) -> Result<()> {
        let mut inner = self.inner.write();
        
        let index = inner.indexes.get_mut(collection)
            .ok_or_else(|| Error::General(format!("Index '{}' not found", collection)))?;
        
        self.remove_doc_from_index(index, doc_id);
        
        Ok(())
    }
    
    /// Search the index
    pub fn search(&self, collection: &str, query: SearchQuery) -> Result<Vec<SearchResult>> {
        let inner = self.inner.read();
        
        let index = inner.indexes.get(collection)
            .ok_or_else(|| Error::General(format!("Index '{}' not found", collection)))?;
        
        // Tokenize query
        let query_tokens = self.tokenize(&query.query, &index.config, &inner.stop_words);
        
        if query_tokens.is_empty() {
            return Ok(Vec::new());
        }
        
        // Calculate BM25 scores for each document
        let mut scores: HashMap<String, f64> = HashMap::new();
        
        for token in &query_tokens {
            if let Some(doc_freqs) = index.index.get(token) {
                let df = doc_freqs.len() as f64;
                let idf = Self::calculate_idf(index.total_docs as f64, df);
                
                for (doc_id, term_freq) in doc_freqs {
                    let doc_length = *index.doc_lengths.get(doc_id).unwrap_or(&0) as f64;
                    let avg_doc_length = index.doc_lengths.values().sum::<u32>() as f64 
                        / index.doc_lengths.len() as f64;
                    
                    let bm25 = Self::calculate_bm25(
                        *term_freq as f64,
                        doc_length,
                        avg_doc_length,
                        idf,
                    );
                    
                    *scores.entry(doc_id.clone()).or_insert(0.0) += bm25;
                }
            }
        }
        
        // Apply field boosting
        if let Some(boost_fields) = &query.boost {
            for (doc_id, score) in scores.iter_mut() {
                if let Some(doc_fields) = index.documents.get(doc_id) {
                    for (field, boost) in boost_fields {
                        if doc_fields.contains_key(field) {
                            *score *= boost;
                        }
                    }
                }
            }
        }
        
        // Sort by score and apply pagination
        let mut results: Vec<_> = scores.into_iter().collect();
        results.sort_by(|a, b| b.1.partial_cmp(&a.1).unwrap());
        
        // Build search results
        let results: Vec<SearchResult> = results
            .into_iter()
            .skip(query.offset)
            .take(query.limit)
            .filter_map(|(doc_id, score)| {
                index.documents.get(&doc_id).map(|fields| {
                    let highlights = self.generate_highlights(&query_tokens, fields, &query.fields);
                    SearchResult {
                        doc_id,
                        score,
                        highlights,
                        fields: fields.clone(),
                    }
                })
            })
            .collect();
        
        Ok(results)
    }
    
    /// Get index statistics
    pub fn get_stats(&self, collection: &str) -> Result<IndexStats> {
        let inner = self.inner.read();
        
        let index = inner.indexes.get(collection)
            .ok_or_else(|| Error::General(format!("Index '{}' not found", collection)))?;
        
        Ok(IndexStats {
            total_documents: index.total_docs,
            total_terms: index.index.len(),
            avg_doc_length: if index.doc_lengths.is_empty() {
                0.0
            } else {
                index.doc_lengths.values().sum::<u32>() as f64 / index.doc_lengths.len() as f64
            },
        })
    }
    
    // Helper methods
    
    fn tokenize(&self, text: &str, config: &IndexConfig, stop_words: &HashSet<String>) -> Vec<String> {
        let text = if config.case_sensitive {
            text.to_string()
        } else {
            text.to_lowercase()
        };
        
        text.split(|c: char| !c.is_alphanumeric())
            .filter(|token| {
                let len = token.len();
                len >= config.min_term_length 
                    && len <= config.max_term_length
                    && !stop_words.contains(*token)
            })
            .map(|token| {
                if config.stemming_enabled {
                    Self::stem(token)
                } else {
                    token.to_string()
                }
            })
            .collect()
    }
    
    fn stem(word: &str) -> String {
        // Simple Porter-like stemming
        let word = word.to_lowercase();
        
        if word.ends_with("ing") && word.len() > 5 {
            return word[..word.len()-3].to_string();
        }
        if word.ends_with("ed") && word.len() > 4 {
            return word[..word.len()-2].to_string();
        }
        if word.ends_with("s") && word.len() > 3 && !word.ends_with("ss") {
            return word[..word.len()-1].to_string();
        }
        
        word
    }
    
    fn calculate_idf(total_docs: f64, doc_freq: f64) -> f64 {
        ((total_docs - doc_freq + 0.5) / (doc_freq + 0.5) + 1.0).ln()
    }
    
    fn calculate_bm25(term_freq: f64, doc_length: f64, avg_doc_length: f64, idf: f64) -> f64 {
        const K1: f64 = 1.2;
        const B: f64 = 0.75;
        
        let normalized_length = doc_length / avg_doc_length;
        let numerator = term_freq * (K1 + 1.0);
        let denominator = term_freq + K1 * (1.0 - B + B * normalized_length);
        
        idf * (numerator / denominator)
    }
    
    fn remove_doc_from_index(&self, index: &mut InvertedIndex, doc_id: &str) {
        // Remove from inverted index
        for (_term, doc_freqs) in index.index.iter_mut() {
            doc_freqs.remove(doc_id);
        }
        
        // Clean up empty term entries
        index.index.retain(|_term, doc_freqs| !doc_freqs.is_empty());
        
        // Remove document metadata
        index.doc_lengths.remove(doc_id);
        index.documents.remove(doc_id);
        index.total_docs = index.documents.len();
    }
    
    fn generate_highlights(
        &self,
        query_tokens: &[String],
        fields: &HashMap<String, String>,
        highlight_fields: &[String],
    ) -> HashMap<String, Vec<String>> {
        let mut highlights = HashMap::new();
        
        for field in highlight_fields {
            if let Some(value) = fields.get(field) {
                let mut snippets = Vec::new();
                let words: Vec<&str> = value.split_whitespace().collect();
                
                for (i, word) in words.iter().enumerate() {
                    let normalized = word.to_lowercase()
                        .trim_matches(|c: char| !c.is_alphanumeric())
                        .to_string();
                    
                    if query_tokens.iter().any(|t| normalized.contains(t)) {
                        // Create snippet with context
                        let start = i.saturating_sub(3);
                        let end = (i + 4).min(words.len());
                        let snippet = words[start..end].join(" ");
                        snippets.push(snippet);
                    }
                }
                
                if !snippets.is_empty() {
                    highlights.insert(field.clone(), snippets);
                }
            }
        }
        
        highlights
    }
    
    fn default_stop_words() -> HashSet<String> {
        vec![
            "a", "an", "and", "are", "as", "at", "be", "but", "by",
            "for", "if", "in", "into", "is", "it", "no", "not", "of",
            "on", "or", "such", "that", "the", "their", "then", "there",
            "these", "they", "this", "to", "was", "will", "with",
        ]
        .into_iter()
        .map(String::from)
        .collect()
    }
}

impl Clone for FullTextSearch {
    fn clone(&self) -> Self {
        Self {
            inner: Arc::clone(&self.inner),
        }
    }
}

#[derive(Debug, Serialize)]
pub struct IndexStats {
    pub total_documents: usize,
    pub total_terms: usize,
    pub avg_doc_length: f64,
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_create_index() {
        let fts = FullTextSearch::new();
        let result = fts.create_index("test", IndexConfig::default());
        assert!(result.is_ok());
    }
    
    #[test]
    fn test_index_and_search() {
        let fts = FullTextSearch::new();
        fts.create_index("docs", IndexConfig::default()).unwrap();
        
        let mut fields = HashMap::new();
        fields.insert("title".to_string(), "Rust Programming".to_string());
        fields.insert("content".to_string(), "Rust is a systems programming language".to_string());
        
        fts.index_document("docs", "doc1", fields).unwrap();
        
        let results = fts.search("docs", SearchQuery {
            query: "rust programming".to_string(),
            fields: vec!["title".to_string(), "content".to_string()],
            limit: 10,
            offset: 0,
            boost: None,
            phrase: false,
        }).unwrap();
        
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].doc_id, "doc1");
        assert!(results[0].score > 0.0);
    }
    
    #[test]
    fn test_stemming() {
        assert_eq!(FullTextSearch::stem("running"), "run");
        assert_eq!(FullTextSearch::stem("walked"), "walk");
        assert_eq!(FullTextSearch::stem("books"), "book");
    }
    
    #[test]
    fn test_stop_words() {
        let fts = FullTextSearch::new();
        let config = IndexConfig::default();
        let inner = fts.inner.read();
        
        let tokens = fts.tokenize("the quick brown fox", &config, &inner.stop_words);
        assert!(!tokens.contains(&"the".to_string()));
        assert!(tokens.contains(&"quick".to_string()));
    }
    
    #[test]
    fn test_delete_document() {
        let fts = FullTextSearch::new();
        fts.create_index("docs", IndexConfig::default()).unwrap();
        
        let mut fields = HashMap::new();
        fields.insert("title".to_string(), "Test Doc".to_string());
        
        fts.index_document("docs", "doc1", fields).unwrap();
        fts.delete_document("docs", "doc1").unwrap();
        
        let stats = fts.get_stats("docs").unwrap();
        assert_eq!(stats.total_documents, 0);
    }
}
