package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"mantisDB/cache"
	"mantisDB/models"
	"mantisDB/storage"
)

// MantisStore provides a unified interface for all three data models
type MantisStore struct {
	storage       storage.StorageEngine
	cache         *cache.CacheManager
	kvStore       *KeyValueStore
	docStore      *DocumentStore
	columnarStore *ColumnarStore
}

// NewMantisStore creates a new unified store
func NewMantisStore(storageEngine storage.StorageEngine, cacheManager *cache.CacheManager) *MantisStore {
	store := &MantisStore{
		storage: storageEngine,
		cache:   cacheManager,
	}

	// Initialize sub-stores
	store.kvStore = NewKeyValueStore(storageEngine, cacheManager)
	store.docStore = NewDocumentStore(storageEngine, cacheManager)
	store.columnarStore = NewColumnarStore(storageEngine, cacheManager)

	return store
}

// KV returns the key-value store interface
func (ms *MantisStore) KV() *KeyValueStore {
	return ms.kvStore
}

// Documents returns the document store interface
func (ms *MantisStore) Documents() *DocumentStore {
	return ms.docStore
}

// Columnar returns the columnar store interface
func (ms *MantisStore) Columnar() *ColumnarStore {
	return ms.columnarStore
}

// KeyValueStore implements key-value operations with caching
type KeyValueStore struct {
	storage storage.StorageEngine
	cache   *cache.CacheManager
}

// NewKeyValueStore creates a new key-value store
func NewKeyValueStore(storageEngine storage.StorageEngine, cacheManager *cache.CacheManager) *KeyValueStore {
	return &KeyValueStore{
		storage: storageEngine,
		cache:   cacheManager,
	}
}

// Set stores a key-value pair with optional TTL and caching
func (kvs *KeyValueStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	// Create key-value model
	var kv *models.KeyValue
	if ttl > 0 {
		kv = models.NewKeyValueWithTTL(key, value, int64(ttl.Seconds()))
	} else {
		kv = models.NewKeyValue(key, value)
	}

	// Serialize and store
	data, err := kv.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize key-value: %v", err)
	}

	storageKey := fmt.Sprintf("kv:%s", key)
	if err := kvs.storage.Put(ctx, storageKey, string(data)); err != nil {
		return fmt.Errorf("failed to store key-value: %v", err)
	}

	// Cache the value with automatic invalidation
	cacheKey := fmt.Sprintf("cache:kv:%s", key)
	dependencies := []string{storageKey}

	if err := kvs.cache.Put(ctx, cacheKey, value, ttl, dependencies); err != nil {
		// Log error but don't fail the operation
		fmt.Printf("Warning: failed to cache key-value: %v\n", err)
	}

	return nil
}

// Get retrieves a value by key with automatic caching
func (kvs *KeyValueStore) Get(ctx context.Context, key string) ([]byte, error) {
	cacheKey := fmt.Sprintf("cache:kv:%s", key)

	// Check cache first
	if cached, found := kvs.cache.Get(ctx, cacheKey); found {
		if data, ok := cached.([]byte); ok {
			return data, nil
		}
	}

	// Get from storage
	storageKey := fmt.Sprintf("kv:%s", key)
	data, err := kvs.storage.Get(ctx, storageKey)
	if err != nil {
		return nil, fmt.Errorf("key not found: %s", key)
	}

	// Deserialize
	kv, err := models.KVFromJSON([]byte(data))
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize key-value: %v", err)
	}

	// Check if expired
	if kv.IsExpired() {
		kvs.Delete(ctx, key)
		return nil, fmt.Errorf("key expired: %s", key)
	}

	// Cache the result
	ttl := kv.TimeToExpiry()
	if ttl <= 0 {
		ttl = time.Hour // Default cache TTL
	}

	dependencies := []string{storageKey}
	kvs.cache.Put(ctx, cacheKey, kv.Value, ttl, dependencies)

	return kv.Value, nil
}

// Delete removes a key-value pair and invalidates cache
func (kvs *KeyValueStore) Delete(ctx context.Context, key string) error {
	storageKey := fmt.Sprintf("kv:%s", key)
	cacheKey := fmt.Sprintf("cache:kv:%s", key)

	// Delete from storage
	if err := kvs.storage.Delete(ctx, storageKey); err != nil {
		return fmt.Errorf("failed to delete key: %v", err)
	}

	// Invalidate cache
	kvs.cache.Delete(ctx, cacheKey)

	return nil
}

// Exists checks if a key exists
func (kvs *KeyValueStore) Exists(ctx context.Context, key string) (bool, error) {
	_, err := kvs.Get(ctx, key)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// BeginTransaction starts a new transaction for atomic operations
func (kvs *KeyValueStore) BeginTransaction(ctx context.Context) (TransactionWrapper, error) {
	tx, err := kvs.storage.BeginTransaction(ctx)
	if err != nil {
		return nil, err
	}
	return &transactionWrapper{tx: tx, cache: kvs.cache}, nil
}

// transactionWrapper wraps the storage transaction with cache invalidation
type transactionWrapper struct {
	tx    storage.Transaction
	cache *cache.CacheManager
}

func (tw *transactionWrapper) Put(key, value string) error {
	return tw.tx.Put(key, value)
}

func (tw *transactionWrapper) Get(key string) (string, error) {
	return tw.tx.Get(key)
}

func (tw *transactionWrapper) Delete(key string) error {
	return tw.tx.Delete(key)
}

func (tw *transactionWrapper) Commit() error {
	return tw.tx.Commit()
}

func (tw *transactionWrapper) Rollback() error {
	return tw.tx.Rollback()
}

// TransactionWrapper interface for the storage transaction
type TransactionWrapper interface {
	Put(key, value string) error
	Get(key string) (string, error)
	Delete(key string) error
	Commit() error
	Rollback() error
}

// DocumentStore implements document operations with caching
type DocumentStore struct {
	storage storage.StorageEngine
	cache   *cache.CacheManager
}

// NewDocumentStore creates a new document store
func NewDocumentStore(storageEngine storage.StorageEngine, cacheManager *cache.CacheManager) *DocumentStore {
	return &DocumentStore{
		storage: storageEngine,
		cache:   cacheManager,
	}
}

// Create creates a new document with caching
func (ds *DocumentStore) Create(ctx context.Context, doc *models.Document) error {
	if err := doc.Validate(); err != nil {
		return fmt.Errorf("document validation failed: %v", err)
	}

	// Update metadata
	doc.UpdateChecksum()

	// Serialize and store
	data, err := doc.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize document: %v", err)
	}

	storageKey := fmt.Sprintf("doc:%s:%s", doc.Collection, doc.ID)
	if err := ds.storage.Put(ctx, storageKey, string(data)); err != nil {
		return fmt.Errorf("failed to store document: %v", err)
	}

	// Cache the document
	cacheKey := fmt.Sprintf("cache:doc:%s:%s", doc.Collection, doc.ID)
	dependencies := []string{storageKey, fmt.Sprintf("collection:%s", doc.Collection)}

	if err := ds.cache.Put(ctx, cacheKey, doc, time.Hour, dependencies); err != nil {
		fmt.Printf("Warning: failed to cache document: %v\n", err)
	}

	return nil
}

// Get retrieves a document by ID with caching
func (ds *DocumentStore) Get(ctx context.Context, collection, id string) (*models.Document, error) {
	cacheKey := fmt.Sprintf("cache:doc:%s:%s", collection, id)

	// Check cache first
	if cached, found := ds.cache.Get(ctx, cacheKey); found {
		if doc, ok := cached.(*models.Document); ok {
			return doc, nil
		}
	}

	// Get from storage
	storageKey := fmt.Sprintf("doc:%s:%s", collection, id)
	data, err := ds.storage.Get(ctx, storageKey)
	if err != nil {
		return nil, fmt.Errorf("document not found: %s/%s", collection, id)
	}

	// Deserialize
	doc, err := models.FromJSON([]byte(data))
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize document: %v", err)
	}

	// Cache the result
	dependencies := []string{storageKey, fmt.Sprintf("collection:%s", collection)}
	ds.cache.Put(ctx, cacheKey, doc, time.Hour, dependencies)

	return doc, nil
}

// Update updates a document and invalidates related caches
func (ds *DocumentStore) Update(ctx context.Context, doc *models.Document) error {
	if err := doc.Validate(); err != nil {
		return fmt.Errorf("document validation failed: %v", err)
	}

	// Update metadata
	doc.UpdatedAt = time.Now()
	doc.Version++
	doc.UpdateChecksum()

	// Serialize and store
	data, err := doc.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize document: %v", err)
	}

	storageKey := fmt.Sprintf("doc:%s:%s", doc.Collection, doc.ID)
	if err := ds.storage.Put(ctx, storageKey, string(data)); err != nil {
		return fmt.Errorf("failed to update document: %v", err)
	}

	// Invalidate related caches
	ds.cache.InvalidateDependencies(ctx, storageKey)
	ds.cache.InvalidateDependencies(ctx, fmt.Sprintf("collection:%s", doc.Collection))

	return nil
}

// Delete removes a document and invalidates caches
func (ds *DocumentStore) Delete(ctx context.Context, collection, id string) error {
	storageKey := fmt.Sprintf("doc:%s:%s", collection, id)
	cacheKey := fmt.Sprintf("cache:doc:%s:%s", collection, id)

	// Delete from storage
	if err := ds.storage.Delete(ctx, storageKey); err != nil {
		return fmt.Errorf("failed to delete document: %v", err)
	}

	// Invalidate caches
	ds.cache.Delete(ctx, cacheKey)
	ds.cache.InvalidateDependencies(ctx, fmt.Sprintf("collection:%s", collection))

	return nil
}

// Query queries documents with caching
func (ds *DocumentStore) Query(ctx context.Context, query *models.DocumentQuery, cacheTTL time.Duration) (*models.DocumentResult, error) {
	// Generate cache key based on query
	queryData, _ := json.Marshal(query)
	cacheKey := fmt.Sprintf("cache:query:doc:%x", queryData)

	// Check cache first
	if cached, found := ds.cache.Get(ctx, cacheKey); found {
		if result, ok := cached.(*models.DocumentResult); ok {
			return result, nil
		}
	}

	// Execute query (simplified implementation)
	result := &models.DocumentResult{
		Documents:  make([]*models.Document, 0),
		TotalCount: 0,
		HasMore:    false,
		NextOffset: 0,
	}

	// Scan collection (simplified - in reality, use indexes)
	prefix := fmt.Sprintf("doc:%s:", query.Collection)
	iterator, err := ds.storage.NewIterator(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to create iterator: %v", err)
	}
	defer iterator.Close()

	count := 0
	for iterator.Next() && (query.Limit == 0 || count < query.Limit) {
		if count < query.Offset {
			count++
			continue
		}

		data := iterator.Value()
		doc, err := models.FromJSON([]byte(data))
		if err != nil {
			continue // Skip invalid documents
		}

		// Apply filters
		if doc.MatchesQuery(query) {
			result.Documents = append(result.Documents, doc)
			result.TotalCount++
		}
		count++
	}

	// Cache the result
	if cacheTTL > 0 {
		dependencies := []string{fmt.Sprintf("collection:%s", query.Collection)}
		ds.cache.Put(ctx, cacheKey, result, cacheTTL, dependencies)
	}

	return result, nil
}

// ColumnarStore implements columnar operations with caching
type ColumnarStore struct {
	storage storage.StorageEngine
	cache   *cache.CacheManager
}

// NewColumnarStore creates a new columnar store
func NewColumnarStore(storageEngine storage.StorageEngine, cacheManager *cache.CacheManager) *ColumnarStore {
	return &ColumnarStore{
		storage: storageEngine,
		cache:   cacheManager,
	}
}

// CreateTable creates a new columnar table
func (cs *ColumnarStore) CreateTable(ctx context.Context, table *models.Table) error {
	// Serialize and store table metadata
	data, err := table.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize table: %v", err)
	}

	storageKey := fmt.Sprintf("table:%s", table.Name)
	if err := cs.storage.Put(ctx, storageKey, string(data)); err != nil {
		return fmt.Errorf("failed to create table: %v", err)
	}

	return nil
}

// GetTable retrieves table metadata with caching
func (cs *ColumnarStore) GetTable(ctx context.Context, name string) (*models.Table, error) {
	cacheKey := fmt.Sprintf("cache:table:%s", name)

	// Check cache first
	if cached, found := cs.cache.Get(ctx, cacheKey); found {
		if table, ok := cached.(*models.Table); ok {
			return table, nil
		}
	}

	// Get from storage
	storageKey := fmt.Sprintf("table:%s", name)
	data, err := cs.storage.Get(ctx, storageKey)
	if err != nil {
		return nil, fmt.Errorf("table not found: %s", name)
	}

	// Deserialize
	table, err := models.TableFromJSON([]byte(data))
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize table: %v", err)
	}

	// Cache the result
	dependencies := []string{storageKey}
	cs.cache.Put(ctx, cacheKey, table, time.Hour, dependencies)

	return table, nil
}

// Insert inserts rows into a columnar table with cache invalidation
func (cs *ColumnarStore) Insert(ctx context.Context, tableName string, rows []*models.Row) error {
	// Get table metadata
	table, err := cs.GetTable(ctx, tableName)
	if err != nil {
		return fmt.Errorf("table not found: %s", tableName)
	}

	// Validate rows
	for _, row := range rows {
		if err := table.ValidateRow(row); err != nil {
			return fmt.Errorf("row validation failed: %v", err)
		}
	}

	// Store rows (simplified - in reality, use columnar format)
	for i, row := range rows {
		rowData, err := json.Marshal(row)
		if err != nil {
			return fmt.Errorf("failed to serialize row: %v", err)
		}

		storageKey := fmt.Sprintf("row:%s:%d", tableName, table.RowCount+int64(i))
		if err := cs.storage.Put(ctx, storageKey, string(rowData)); err != nil {
			return fmt.Errorf("failed to store row: %v", err)
		}
	}

	// Update table metadata
	table.RowCount += int64(len(rows))
	table.UpdatedAt = time.Now()

	tableData, _ := table.ToJSON()
	tableKey := fmt.Sprintf("table:%s", tableName)
	cs.storage.Put(ctx, tableKey, string(tableData))

	// Invalidate related caches
	cs.cache.InvalidateDependencies(ctx, fmt.Sprintf("table:%s", tableName))

	return nil
}

// Query executes a columnar query with caching
func (cs *ColumnarStore) Query(ctx context.Context, query *models.ColumnarQuery, cacheTTL time.Duration) (*models.ColumnarResult, error) {
	// Generate cache key based on query
	queryData, _ := json.Marshal(query)
	cacheKey := fmt.Sprintf("cache:query:col:%x", queryData)

	// Check cache first
	if cached, found := cs.cache.Get(ctx, cacheKey); found {
		if result, ok := cached.(*models.ColumnarResult); ok {
			return result, nil
		}
	}

	// Execute query (simplified implementation)
	result := &models.ColumnarResult{
		Columns:     query.Columns,
		Rows:        make([]map[string]interface{}, 0),
		TotalRows:   0,
		ScannedRows: 0,
		HasMore:     false,
		NextOffset:  0,
		Metadata: models.QueryMetadata{
			ExecutionTime:  0,
			PartitionsRead: make([]string, 0),
			IndexesUsed:    make([]string, 0),
			BytesScanned:   0,
			BytesReturned:  0,
		},
	}

	startTime := time.Now()

	// Scan rows (simplified - in reality, use columnar scanning)
	prefix := fmt.Sprintf("row:%s:", query.Table)
	iterator, err := cs.storage.NewIterator(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to create iterator: %v", err)
	}
	defer iterator.Close()

	count := 0
	for iterator.Next() && (query.Limit == 0 || count < query.Limit) {
		if count < query.Offset {
			count++
			continue
		}

		data := iterator.Value()
		var row models.Row
		if err := json.Unmarshal([]byte(data), &row); err != nil {
			continue // Skip invalid rows
		}

		// Apply filters
		if cs.matchesFilters(row.Values, query.Filters) {
			// Project columns
			projectedRow := make(map[string]interface{})
			if len(query.Columns) == 0 {
				// Select all columns
				projectedRow = row.Values
			} else {
				for _, col := range query.Columns {
					if value, exists := row.Values[col]; exists {
						projectedRow[col] = value
					}
				}
			}

			result.Rows = append(result.Rows, projectedRow)
			result.TotalRows++
		}
		result.ScannedRows++
		count++
	}

	// Update metadata
	result.Metadata.ExecutionTime = time.Since(startTime).Milliseconds()
	result.Metadata.BytesScanned = result.ScannedRows * 1024 // Simplified
	result.Metadata.BytesReturned = result.TotalRows * 512   // Simplified

	// Cache the result
	if cacheTTL > 0 {
		dependencies := []string{fmt.Sprintf("table:%s", query.Table)}
		cs.cache.Put(ctx, cacheKey, result, cacheTTL, dependencies)
	}

	return result, nil
}

// Helper method to match filters
func (cs *ColumnarStore) matchesFilters(values map[string]interface{}, filters []*models.Filter) bool {
	for _, filter := range filters {
		value, exists := values[filter.Column]
		if !exists {
			return false
		}

		if !cs.matchesFilter(value, filter) {
			return false
		}
	}
	return true
}

// Helper method to match a single filter
func (cs *ColumnarStore) matchesFilter(value interface{}, filter *models.Filter) bool {
	switch filter.Operator {
	case models.FilterOpEQ:
		return fmt.Sprintf("%v", value) == fmt.Sprintf("%v", filter.Value)
	case models.FilterOpNE:
		return fmt.Sprintf("%v", value) != fmt.Sprintf("%v", filter.Value)
	case models.FilterOpLT:
		return fmt.Sprintf("%v", value) < fmt.Sprintf("%v", filter.Value)
	case models.FilterOpLE:
		return fmt.Sprintf("%v", value) <= fmt.Sprintf("%v", filter.Value)
	case models.FilterOpGT:
		return fmt.Sprintf("%v", value) > fmt.Sprintf("%v", filter.Value)
	case models.FilterOpGE:
		return fmt.Sprintf("%v", value) >= fmt.Sprintf("%v", filter.Value)
	case models.FilterOpIN:
		valueStr := fmt.Sprintf("%v", value)
		for _, filterValue := range filter.Values {
			if valueStr == fmt.Sprintf("%v", filterValue) {
				return true
			}
		}
		return false
	case models.FilterOpIsNull:
		return value == nil
	case models.FilterOpNotNull:
		return value != nil
	default:
		return false
	}
}

// ListTables returns a list of all tables and collections
func (ms *MantisStore) ListTables(ctx context.Context) ([]map[string]interface{}, error) {
	tables := make([]map[string]interface{}, 0)
	seenNames := make(map[string]bool)

	// List columnar tables
	tableIter, err := ms.storage.NewIterator(ctx, "table:")
	if err == nil {
		defer tableIter.Close()
		for tableIter.Next() {
			data := tableIter.Value()
			var table models.Table
			if err := json.Unmarshal([]byte(data), &table); err == nil {
				if !seenNames[table.Name] {
					seenNames[table.Name] = true
					tables = append(tables, map[string]interface{}{
						"name":       table.Name,
						"type":       "table",
						"row_count":  table.RowCount,
						"created_at": table.CreatedAt,
						"updated_at": table.UpdatedAt,
						"size_bytes": table.RowCount * 512, // Estimate
					})
				}
			}
		}
	}

	// List document collections by scanning doc: keys
	docIter, err := ms.storage.NewIterator(ctx, "doc:")
	if err == nil {
		defer docIter.Close()
		collectionCounts := make(map[string]int64)
		for docIter.Next() {
			key := docIter.Key()
			// Extract collection name from key: doc:collectionName:documentID
			parts := []byte(key)
			if len(parts) > 4 {
				keyStr := string(parts)
				if len(keyStr) > 4 {
					remaining := keyStr[4:] // Skip "doc:"
					// Find next colon
					for i, char := range remaining {
						if char == ':' {
							collection := remaining[:i]
							collectionCounts[collection]++
							break
						}
					}
				}
			}
		}
		
		// Convert counts to table info
		for collection, count := range collectionCounts {
			if !seenNames[collection] {
				seenNames[collection] = true
				tables = append(tables, map[string]interface{}{
					"name":       collection,
					"type":       "collection",
					"row_count":  count,
					"created_at": time.Now().Add(-24 * time.Hour), // Unknown, use placeholder
					"updated_at": time.Now(),
					"size_bytes": count * 1024, // Estimate
				})
			}
		}
	}

	return tables, nil
}

// GetStats returns statistics about the store
func (ms *MantisStore) GetStats(ctx context.Context) map[string]interface{} {
	stats := make(map[string]interface{})

	// Cache stats
	cacheStats := ms.cache.GetStats()
	stats["cache"] = map[string]interface{}{
		"total_entries": cacheStats.TotalEntries,
		"total_size":    cacheStats.TotalSize,
		"max_size":      cacheStats.MaxSize,
		"hit_rate":      cacheStats.HitRate,
	}

	// Storage engine health
	if err := ms.storage.HealthCheck(ctx); err == nil {
		stats["storage_engine"] = "healthy"
	} else {
		stats["storage_engine"] = "unhealthy"
	}

	return stats
}
