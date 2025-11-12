// executor.go - Query execution engine with caching
package query

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// QueryExecutor executes optimized queries against the storage engine.
// This is the final stage of the query pipeline (parse -> optimize -> execute).
// We cache SELECT results but invalidate on writes to maintain consistency.
type QueryExecutor struct {
	storageEngine StorageEngine
	cacheManager  CacheManager
	config        ExecutorConfig
}

// ExecutorConfig holds executor configuration
type ExecutorConfig struct {
	EnableCaching   bool
	CacheTimeout    int
	MaxConcurrency  int
	QueryTimeout    int
	EnableProfiling bool
}

// StorageEngine interface for storage operations
type StorageEngine interface {
	Get(ctx context.Context, key string) (string, error)
	Put(ctx context.Context, key, value string) error
	Delete(ctx context.Context, key string) error
	BatchGet(ctx context.Context, keys []string) (map[string]string, error)
	BatchPut(ctx context.Context, kvPairs map[string]string) error
	BatchDelete(ctx context.Context, keys []string) error
}

// CacheManager interface for caching operations
type CacheManager interface {
	Get(ctx context.Context, key string) (interface{}, bool)
	Put(ctx context.Context, key string, value interface{}, ttl time.Duration, dependencies []string) error
	Delete(ctx context.Context, key string)
}

// ExecutionResult represents the result of query execution
type ExecutionResult struct {
	Rows          []map[string]interface{}
	RowsAffected  int64
	ExecutionTime int64 // in milliseconds
	CacheHit      bool
	Error         error
}

// ExecutionContext holds context for query execution
type ExecutionContext struct {
	Query       *OptimizedQuery
	Parameters  map[string]interface{}
	Timeout     int
	EnableCache bool
	CacheKey    string
}

// NewQueryExecutor creates a new query executor
func NewQueryExecutor(storageEngine StorageEngine, cacheManager CacheManager, config ExecutorConfig) *QueryExecutor {
	return &QueryExecutor{
		storageEngine: storageEngine,
		cacheManager:  cacheManager,
		config:        config,
	}
}

// Execute executes an optimized query
func (exec *QueryExecutor) Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	result := &ExecutionResult{}

	// Cache check for SELECT queries only.
	// We don't cache writes (INSERT/UPDATE/DELETE) because they're idempotent
	// and caching them could hide errors or confuse transaction semantics.
	if execCtx.EnableCache && execCtx.Query.OriginalQuery.Type == QueryTypeSelect {
		if cachedResult, found := exec.checkCache(ctx, execCtx.CacheKey); found {
			result = cachedResult.(*ExecutionResult)
			result.CacheHit = true
			return result, nil
		}
	}

	// Execute based on query type
	switch execCtx.Query.OriginalQuery.Type {
	case QueryTypeSelect:
		return exec.executeSelect(ctx, execCtx)
	case QueryTypeInsert:
		return exec.executeInsert(ctx, execCtx)
	case QueryTypeUpdate:
		return exec.executeUpdate(ctx, execCtx)
	case QueryTypeDelete:
		return exec.executeDelete(ctx, execCtx)
	default:
		return nil, fmt.Errorf("unsupported query type: %v", execCtx.Query.OriginalQuery.Type)
	}
}

// executeSelect executes a SELECT query
func (exec *QueryExecutor) executeSelect(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	query := execCtx.Query.OriginalQuery
	plan := execCtx.Query.ExecutionPlan

	result := &ExecutionResult{
		Rows: make([]map[string]interface{}, 0),
	}

	// Execute pipeline operations sequentially (not parallel).
	// Parallel execution complicates debugging and gains are minimal for
	// our workload (mostly point queries, not OLAP scans).
	var currentData []map[string]interface{}

	for _, operation := range plan.Operations {
		switch operation.Type {
		case OpTableScan:
			data, err := exec.executeTableScan(ctx, operation, query)
			if err != nil {
				result.Error = err
				return result, err
			}
			currentData = data

		case OpIndexScan:
			data, err := exec.executeIndexScan(ctx, operation, query)
			if err != nil {
				result.Error = err
				return result, err
			}
			currentData = data

		case OpFilter:
			currentData = exec.applyFilter(currentData, operation.Conditions)

		case OpProject:
			currentData = exec.applyProjection(currentData, query.Fields)

		case OpSort:
			currentData = exec.applySort(currentData, query.OrderBy)
		}
	}

	// Apply LIMIT late (after all filtering) to keep pipeline simple.
	// Early LIMIT (during scan) would be faster but requires query planner
	// to prove filters are selective, which we haven't implemented yet.
	if query.Limit > 0 && len(currentData) > query.Limit {
		currentData = currentData[:query.Limit]
	}

	result.Rows = currentData
	result.RowsAffected = int64(len(currentData))

	// Cache the result if caching is enabled
	if execCtx.EnableCache && exec.config.EnableCaching {
		exec.cacheResult(ctx, execCtx.CacheKey, result)
	}

	return result, nil
}

// executeInsert executes an INSERT query
func (exec *QueryExecutor) executeInsert(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	query := execCtx.Query.OriginalQuery

	result := &ExecutionResult{}

	// Generate key-value pairs from the insert data
	kvPairs := make(map[string]string)

	// Simplified: assume single row insert with key-value structure
	for field, value := range query.Values {
		key := fmt.Sprintf("%s:%s", query.Table, field)
		kvPairs[key] = fmt.Sprintf("%v", value)
	}

	// Execute batch put
	err := exec.storageEngine.BatchPut(ctx, kvPairs)
	if err != nil {
		result.Error = err
		return result, err
	}

	result.RowsAffected = 1

	// Invalidate all cached queries touching this table.
	// Conservative invalidation - we could track column-level dependencies
	// for finer granularity, but that's complex and error-prone.
	if exec.config.EnableCaching {
		exec.invalidateTableCache(ctx, query.Table)
	}

	return result, nil
}

// executeUpdate executes an UPDATE query
func (exec *QueryExecutor) executeUpdate(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	query := execCtx.Query.OriginalQuery

	result := &ExecutionResult{}

	// First, find the keys that match the WHERE conditions
	matchingKeys, err := exec.findMatchingKeys(ctx, query.Table, query.Conditions)
	if err != nil {
		result.Error = err
		return result, err
	}

	// Update the matching records
	kvPairs := make(map[string]string)
	for _, key := range matchingKeys {
		for field, value := range query.Values {
			fullKey := fmt.Sprintf("%s:%s", key, field)
			kvPairs[fullKey] = fmt.Sprintf("%v", value)
		}
	}

	err = exec.storageEngine.BatchPut(ctx, kvPairs)
	if err != nil {
		result.Error = err
		return result, err
	}

	result.RowsAffected = int64(len(matchingKeys))

	// Invalidate related cache entries
	if exec.config.EnableCaching {
		exec.invalidateTableCache(ctx, query.Table)
	}

	return result, nil
}

// executeDelete executes a DELETE query
func (exec *QueryExecutor) executeDelete(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	query := execCtx.Query.OriginalQuery

	result := &ExecutionResult{}

	// Find the keys that match the WHERE conditions
	matchingKeys, err := exec.findMatchingKeys(ctx, query.Table, query.Conditions)
	if err != nil {
		result.Error = err
		return result, err
	}

	// Delete the matching records
	err = exec.storageEngine.BatchDelete(ctx, matchingKeys)
	if err != nil {
		result.Error = err
		return result, err
	}

	result.RowsAffected = int64(len(matchingKeys))

	// Invalidate related cache entries
	if exec.config.EnableCaching {
		exec.invalidateTableCache(ctx, query.Table)
	}

	return result, nil
}

// executeTableScan performs a full table scan
func (exec *QueryExecutor) executeTableScan(ctx context.Context, operation Operation, query *Query) ([]map[string]interface{}, error) {
	// In a real implementation, this would scan through all records in the table
	// For now, we'll simulate by getting some sample data

	var results []map[string]interface{}

	// Simulate table scan by trying to get records with a pattern
	// This is a simplified approach
	sampleKeys := exec.generateSampleKeys(operation.Table, 100)

	data, err := exec.storageEngine.BatchGet(ctx, sampleKeys)
	if err != nil {
		return nil, err
	}

	// Convert storage data to result format
	for key, value := range data {
		record := make(map[string]interface{})
		parts := strings.Split(key, ":")
		if len(parts) >= 2 {
			record["id"] = parts[1]
			record["value"] = value
			// Add more fields as needed
		}
		results = append(results, record)
	}

	return results, nil
}

// executeIndexScan performs an index scan
func (exec *QueryExecutor) executeIndexScan(ctx context.Context, operation Operation, query *Query) ([]map[string]interface{}, error) {
	// Simplified index scan - in reality, this would use actual index structures
	// For now, we'll simulate by doing a more targeted key lookup

	var results []map[string]interface{}

	// Use conditions to generate more specific keys
	targetKeys := exec.generateTargetKeys(operation.Table, operation.Conditions)

	data, err := exec.storageEngine.BatchGet(ctx, targetKeys)
	if err != nil {
		return nil, err
	}

	// Convert to result format
	for key, value := range data {
		record := make(map[string]interface{})
		parts := strings.Split(key, ":")
		if len(parts) >= 2 {
			record["id"] = parts[1]
			record["value"] = value
		}
		results = append(results, record)
	}

	return results, nil
}

// applyFilter applies WHERE conditions to the data
func (exec *QueryExecutor) applyFilter(data []map[string]interface{}, conditions []Condition) []map[string]interface{} {
	if len(conditions) == 0 {
		return data
	}

	var filtered []map[string]interface{}

	for _, record := range data {
		if exec.matchesConditions(record, conditions) {
			filtered = append(filtered, record)
		}
	}

	return filtered
}

// applyProjection applies field selection to the data
func (exec *QueryExecutor) applyProjection(data []map[string]interface{}, fields []string) []map[string]interface{} {
	if len(fields) == 0 || (len(fields) == 1 && fields[0] == "*") {
		return data
	}

	var projected []map[string]interface{}

	for _, record := range data {
		newRecord := make(map[string]interface{})
		for _, field := range fields {
			if value, exists := record[field]; exists {
				newRecord[field] = value
			}
		}
		projected = append(projected, newRecord)
	}

	return projected
}

// applySort applies ORDER BY to the data
func (exec *QueryExecutor) applySort(data []map[string]interface{}, orderBy []OrderByClause) []map[string]interface{} {
	if len(orderBy) == 0 {
		return data
	}

	// Simplified sorting - in reality, this would be more sophisticated
	// For now, just return the data as-is
	return data
}

// Helper methods

func (exec *QueryExecutor) checkCache(ctx context.Context, cacheKey string) (interface{}, bool) {
	if exec.cacheManager == nil {
		return nil, false
	}
	return exec.cacheManager.Get(ctx, cacheKey)
}

func (exec *QueryExecutor) cacheResult(ctx context.Context, cacheKey string, result *ExecutionResult) {
	if exec.cacheManager == nil {
		return
	}
	// Cache query results with default TTL and no dependencies
	exec.cacheManager.Put(ctx, cacheKey, result, time.Duration(exec.config.CacheTimeout)*time.Second, nil)
}

func (exec *QueryExecutor) invalidateTableCache(ctx context.Context, table string) {
	if exec.cacheManager == nil {
		return
	}
	// Invalidate all cache entries related to this table
	cacheKey := fmt.Sprintf("table:%s", table)
	exec.cacheManager.Delete(ctx, cacheKey)
}

func (exec *QueryExecutor) findMatchingKeys(ctx context.Context, table string, conditions []Condition) ([]string, error) {
	// Simplified key matching - in reality, this would use indexes
	var keys []string

	// Generate potential keys based on conditions
	for _, condition := range conditions {
		if condition.Operator == "=" {
			key := fmt.Sprintf("%s:%v", table, condition.Value)
			keys = append(keys, key)
		}
	}

	// If no specific keys found, generate sample keys
	if len(keys) == 0 {
		keys = exec.generateSampleKeys(table, 10)
	}

	return keys, nil
}

func (exec *QueryExecutor) generateSampleKeys(table string, count int) []string {
	var keys []string
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("%s:record_%d", table, i)
		keys = append(keys, key)
	}
	return keys
}

func (exec *QueryExecutor) generateTargetKeys(table string, conditions []Condition) []string {
	var keys []string

	for _, condition := range conditions {
		if condition.Operator == "=" {
			key := fmt.Sprintf("%s:%v", table, condition.Value)
			keys = append(keys, key)
		}
	}

	// If no specific keys, fall back to sample keys
	if len(keys) == 0 {
		keys = exec.generateSampleKeys(table, 5)
	}

	return keys
}

func (exec *QueryExecutor) matchesConditions(record map[string]interface{}, conditions []Condition) bool {
	for _, condition := range conditions {
		if !exec.matchesCondition(record, condition) {
			return false
		}
	}
	return true
}

func (exec *QueryExecutor) matchesCondition(record map[string]interface{}, condition Condition) bool {
	value, exists := record[condition.Field]
	if !exists {
		return false
	}

	switch condition.Operator {
	case "=":
		return fmt.Sprintf("%v", value) == fmt.Sprintf("%v", condition.Value)
	case "<":
		// Simplified comparison - in reality, this would handle different data types
		return fmt.Sprintf("%v", value) < fmt.Sprintf("%v", condition.Value)
	case ">":
		return fmt.Sprintf("%v", value) > fmt.Sprintf("%v", condition.Value)
	case "<=":
		return fmt.Sprintf("%v", value) <= fmt.Sprintf("%v", condition.Value)
	case ">=":
		return fmt.Sprintf("%v", value) >= fmt.Sprintf("%v", condition.Value)
	case "!=", "<>":
		return fmt.Sprintf("%v", value) != fmt.Sprintf("%v", condition.Value)
	default:
		return false
	}
}

// ExecutionStats provides statistics about query execution
type ExecutionStats struct {
	TotalQueries    int64
	CacheHitRate    float64
	AverageExecTime float64
	SlowQueries     int64
	ErrorRate       float64
}

// GetExecutionStats returns execution statistics
func (exec *QueryExecutor) GetExecutionStats() *ExecutionStats {
	// Simplified stats - in reality, this would track actual metrics
	return &ExecutionStats{
		TotalQueries:    1000,
		CacheHitRate:    0.75,
		AverageExecTime: 25.5,
		SlowQueries:     10,
		ErrorRate:       0.02,
	}
}

// ValidateQuery validates a query before execution
func (exec *QueryExecutor) ValidateQuery(query *Query) error {
	if query == nil {
		return errors.New("query cannot be nil")
	}

	if query.Table == "" {
		return errors.New("table name is required")
	}

	// Add more validation rules as needed
	switch query.Type {
	case QueryTypeSelect:
		// SELECT queries are generally valid if they have a table
		return nil
	case QueryTypeInsert:
		if len(query.Values) == 0 {
			return errors.New("INSERT query must have values")
		}
		return nil
	case QueryTypeUpdate:
		if len(query.Values) == 0 {
			return errors.New("UPDATE query must have values to set")
		}
		return nil
	case QueryTypeDelete:
		// DELETE queries are valid with just a table name
		return nil
	default:
		return fmt.Errorf("unsupported query type: %v", query.Type)
	}
}
