package query

import (
	"fmt"
	"strings"
)

// QueryOptimizer optimizes parsed queries for better performance
type QueryOptimizer struct {
	statistics *Statistics
	config     OptimizerConfig
}

// OptimizerConfig holds optimizer configuration
type OptimizerConfig struct {
	EnableIndexHints        bool
	EnableJoinReordering    bool
	EnablePredicatePushdown bool
	CostThreshold           float64
}

// Statistics holds table and index statistics
type Statistics struct {
	TableStats map[string]*TableStats
	IndexStats map[string]*IndexStats
}

// TableStats holds statistics for a table
type TableStats struct {
	RowCount    int64
	ColumnStats map[string]*ColumnStats
}

// ColumnStats holds statistics for a column
type ColumnStats struct {
	Cardinality int64
	MinValue    interface{}
	MaxValue    interface{}
	NullCount   int64
}

// IndexStats holds statistics for an index
type IndexStats struct {
	Name        string
	Table       string
	Columns     []string
	Cardinality int64
	Height      int
	LeafPages   int64
}

// OptimizedQuery represents an optimized query execution plan
type OptimizedQuery struct {
	OriginalQuery *Query
	ExecutionPlan *ExecutionPlan
	EstimatedCost float64
	Optimizations []string
}

// ExecutionPlan represents the execution plan for a query
type ExecutionPlan struct {
	Operations []Operation
	Indexes    []string
	JoinOrder  []string
}

// Operation represents a single operation in the execution plan
type Operation struct {
	Type          OperationType
	Table         string
	Index         string
	Conditions    []Condition
	EstimatedRows int64
	Cost          float64
}

// OperationType represents the type of operation
type OperationType int

const (
	OpTableScan OperationType = iota
	OpIndexScan
	OpIndexSeek
	OpNestedLoop
	OpHashJoin
	OpSortMerge
	OpSort
	OpFilter
	OpProject
)

// NewQueryOptimizer creates a new query optimizer
func NewQueryOptimizer(config OptimizerConfig) *QueryOptimizer {
	return &QueryOptimizer{
		statistics: &Statistics{
			TableStats: make(map[string]*TableStats),
			IndexStats: make(map[string]*IndexStats),
		},
		config: config,
	}
}

// Optimize optimizes a parsed query
func (opt *QueryOptimizer) Optimize(query *Query) (*OptimizedQuery, error) {
	optimizedQuery := &OptimizedQuery{
		OriginalQuery: query,
		ExecutionPlan: &ExecutionPlan{},
		Optimizations: make([]string, 0),
	}

	switch query.Type {
	case QueryTypeSelect:
		return opt.optimizeSelect(optimizedQuery)
	case QueryTypeInsert:
		return opt.optimizeInsert(optimizedQuery)
	case QueryTypeUpdate:
		return opt.optimizeUpdate(optimizedQuery)
	case QueryTypeDelete:
		return opt.optimizeDelete(optimizedQuery)
	default:
		return optimizedQuery, fmt.Errorf("unsupported query type for optimization")
	}
}

// optimizeSelect optimizes a SELECT query
func (opt *QueryOptimizer) optimizeSelect(optimizedQuery *OptimizedQuery) (*OptimizedQuery, error) {
	query := optimizedQuery.OriginalQuery
	plan := optimizedQuery.ExecutionPlan

	// 1. Predicate pushdown
	if opt.config.EnablePredicatePushdown {
		opt.applyPredicatePushdown(query, plan)
		optimizedQuery.Optimizations = append(optimizedQuery.Optimizations, "predicate_pushdown")
	}

	// 2. Index selection
	if opt.config.EnableIndexHints {
		selectedIndex := opt.selectBestIndex(query)
		if selectedIndex != "" {
			plan.Indexes = append(plan.Indexes, selectedIndex)
			optimizedQuery.Optimizations = append(optimizedQuery.Optimizations, "index_selection")
		}
	}

	// 3. Create execution operations
	operations := opt.createSelectOperations(query, plan)
	plan.Operations = operations

	// 4. Calculate estimated cost
	optimizedQuery.EstimatedCost = opt.calculateCost(plan)

	return optimizedQuery, nil
}

// optimizeInsert optimizes an INSERT query
func (opt *QueryOptimizer) optimizeInsert(optimizedQuery *OptimizedQuery) (*OptimizedQuery, error) {
	query := optimizedQuery.OriginalQuery
	plan := optimizedQuery.ExecutionPlan

	// Simple optimization for INSERT
	operation := Operation{
		Type:          OpTableScan, // Simplified
		Table:         query.Table,
		EstimatedRows: 1,
		Cost:          1.0,
	}

	plan.Operations = append(plan.Operations, operation)
	optimizedQuery.EstimatedCost = 1.0

	return optimizedQuery, nil
}

// optimizeUpdate optimizes an UPDATE query
func (opt *QueryOptimizer) optimizeUpdate(optimizedQuery *OptimizedQuery) (*OptimizedQuery, error) {
	query := optimizedQuery.OriginalQuery
	plan := optimizedQuery.ExecutionPlan

	// Check if we can use an index for WHERE conditions
	if len(query.Conditions) > 0 {
		selectedIndex := opt.selectBestIndex(query)
		if selectedIndex != "" {
			plan.Indexes = append(plan.Indexes, selectedIndex)
			optimizedQuery.Optimizations = append(optimizedQuery.Optimizations, "index_selection")
		}
	}

	// Create operations
	operations := opt.createUpdateOperations(query, plan)
	plan.Operations = operations

	optimizedQuery.EstimatedCost = opt.calculateCost(plan)

	return optimizedQuery, nil
}

// optimizeDelete optimizes a DELETE query
func (opt *QueryOptimizer) optimizeDelete(optimizedQuery *OptimizedQuery) (*OptimizedQuery, error) {
	query := optimizedQuery.OriginalQuery
	plan := optimizedQuery.ExecutionPlan

	// Similar to UPDATE optimization
	if len(query.Conditions) > 0 {
		selectedIndex := opt.selectBestIndex(query)
		if selectedIndex != "" {
			plan.Indexes = append(plan.Indexes, selectedIndex)
			optimizedQuery.Optimizations = append(optimizedQuery.Optimizations, "index_selection")
		}
	}

	// Create operations
	operations := opt.createDeleteOperations(query, plan)
	plan.Operations = operations

	optimizedQuery.EstimatedCost = opt.calculateCost(plan)

	return optimizedQuery, nil
}

// applyPredicatePushdown applies predicate pushdown optimization
func (opt *QueryOptimizer) applyPredicatePushdown(query *Query, plan *ExecutionPlan) {
	// Simplified predicate pushdown
	// In a real implementation, this would analyze the query structure
	// and push predicates down to the lowest possible level

	for _, condition := range query.Conditions {
		if opt.canPushDown(condition) {
			// Create a filter operation
			operation := Operation{
				Type:       OpFilter,
				Table:      query.Table,
				Conditions: []Condition{condition},
				Cost:       0.1,
			}
			plan.Operations = append(plan.Operations, operation)
		}
	}
}

// selectBestIndex selects the best index for the query
func (opt *QueryOptimizer) selectBestIndex(query *Query) string {
	if len(query.Conditions) == 0 {
		return ""
	}

	// Get available indexes for the table
	availableIndexes := opt.getAvailableIndexes(query.Table)

	bestIndex := ""
	bestScore := 0.0

	for _, indexName := range availableIndexes {
		score := opt.calculateIndexScore(indexName, query.Conditions)
		if score > bestScore {
			bestScore = score
			bestIndex = indexName
		}
	}

	return bestIndex
}

// createSelectOperations creates execution operations for SELECT query
func (opt *QueryOptimizer) createSelectOperations(query *Query, plan *ExecutionPlan) []Operation {
	var operations []Operation

	// Determine scan type
	if len(plan.Indexes) > 0 {
		// Index scan
		operation := Operation{
			Type:          OpIndexScan,
			Table:         query.Table,
			Index:         plan.Indexes[0],
			Conditions:    query.Conditions,
			EstimatedRows: opt.estimateRows(query.Table, query.Conditions),
			Cost:          opt.calculateIndexScanCost(plan.Indexes[0], query.Conditions),
		}
		operations = append(operations, operation)
	} else {
		// Table scan
		operation := Operation{
			Type:          OpTableScan,
			Table:         query.Table,
			Conditions:    query.Conditions,
			EstimatedRows: opt.estimateRows(query.Table, query.Conditions),
			Cost:          opt.calculateTableScanCost(query.Table),
		}
		operations = append(operations, operation)
	}

	// Add projection if specific fields are selected
	if len(query.Fields) > 0 && !(len(query.Fields) == 1 && query.Fields[0] == "*") {
		operation := Operation{
			Type:          OpProject,
			Table:         query.Table,
			EstimatedRows: operations[len(operations)-1].EstimatedRows,
			Cost:          0.1,
		}
		operations = append(operations, operation)
	}

	// Add sorting if ORDER BY is present
	if len(query.OrderBy) > 0 {
		operation := Operation{
			Type:          OpSort,
			Table:         query.Table,
			EstimatedRows: operations[len(operations)-1].EstimatedRows,
			Cost:          opt.calculateSortCost(operations[len(operations)-1].EstimatedRows),
		}
		operations = append(operations, operation)
	}

	return operations
}

// createUpdateOperations creates execution operations for UPDATE query
func (opt *QueryOptimizer) createUpdateOperations(query *Query, plan *ExecutionPlan) []Operation {
	var operations []Operation

	// First, find the rows to update
	if len(plan.Indexes) > 0 {
		operation := Operation{
			Type:          OpIndexSeek,
			Table:         query.Table,
			Index:         plan.Indexes[0],
			Conditions:    query.Conditions,
			EstimatedRows: opt.estimateRows(query.Table, query.Conditions),
			Cost:          opt.calculateIndexSeekCost(plan.Indexes[0], query.Conditions),
		}
		operations = append(operations, operation)
	} else {
		operation := Operation{
			Type:          OpTableScan,
			Table:         query.Table,
			Conditions:    query.Conditions,
			EstimatedRows: opt.estimateRows(query.Table, query.Conditions),
			Cost:          opt.calculateTableScanCost(query.Table),
		}
		operations = append(operations, operation)
	}

	return operations
}

// createDeleteOperations creates execution operations for DELETE query
func (opt *QueryOptimizer) createDeleteOperations(query *Query, plan *ExecutionPlan) []Operation {
	// Similar to UPDATE operations
	return opt.createUpdateOperations(query, plan)
}

// Helper methods for cost calculation and statistics

func (opt *QueryOptimizer) canPushDown(condition Condition) bool {
	// Simplified check - in reality, this would be more complex
	return condition.Operator == "=" || condition.Operator == "<" || condition.Operator == ">"
}

func (opt *QueryOptimizer) getAvailableIndexes(table string) []string {
	var indexes []string
	for indexName, indexStats := range opt.statistics.IndexStats {
		if indexStats.Table == table {
			indexes = append(indexes, indexName)
		}
	}
	return indexes
}

func (opt *QueryOptimizer) calculateIndexScore(indexName string, conditions []Condition) float64 {
	indexStats, exists := opt.statistics.IndexStats[indexName]
	if !exists {
		return 0.0
	}

	score := 0.0
	for _, condition := range conditions {
		for _, column := range indexStats.Columns {
			if strings.EqualFold(condition.Field, column) {
				// Higher score for exact matches
				if condition.Operator == "=" {
					score += 10.0
				} else {
					score += 5.0
				}
			}
		}
	}

	// Adjust score based on index selectivity
	if indexStats.Cardinality > 0 {
		score *= float64(indexStats.Cardinality) / 1000.0
	}

	return score
}

func (opt *QueryOptimizer) estimateRows(table string, conditions []Condition) int64 {
	tableStats, exists := opt.statistics.TableStats[table]
	if !exists {
		return 1000 // Default estimate
	}

	estimatedRows := tableStats.RowCount

	// Apply selectivity for each condition
	for _, condition := range conditions {
		selectivity := opt.calculateSelectivity(table, condition)
		estimatedRows = int64(float64(estimatedRows) * selectivity)
	}

	if estimatedRows < 1 {
		estimatedRows = 1
	}

	return estimatedRows
}

func (opt *QueryOptimizer) calculateSelectivity(table string, condition Condition) float64 {
	tableStats, exists := opt.statistics.TableStats[table]
	if !exists {
		return 0.1 // Default selectivity
	}

	columnStats, exists := tableStats.ColumnStats[condition.Field]
	if !exists {
		return 0.1 // Default selectivity
	}

	switch condition.Operator {
	case "=":
		if columnStats.Cardinality > 0 {
			return 1.0 / float64(columnStats.Cardinality)
		}
		return 0.1
	case "<", ">":
		return 0.33 // Assume 1/3 selectivity for range queries
	case "<=", ">=":
		return 0.33
	default:
		return 0.1
	}
}

func (opt *QueryOptimizer) calculateCost(plan *ExecutionPlan) float64 {
	totalCost := 0.0
	for _, operation := range plan.Operations {
		totalCost += operation.Cost
	}
	return totalCost
}

func (opt *QueryOptimizer) calculateTableScanCost(table string) float64 {
	tableStats, exists := opt.statistics.TableStats[table]
	if !exists {
		return 100.0 // Default cost
	}

	// Cost is proportional to number of rows
	return float64(tableStats.RowCount) * 0.01
}

func (opt *QueryOptimizer) calculateIndexScanCost(indexName string, conditions []Condition) float64 {
	indexStats, exists := opt.statistics.IndexStats[indexName]
	if !exists {
		return 10.0 // Default cost
	}

	// Cost is based on index height and selectivity
	baseCost := float64(indexStats.Height) * 2.0

	// Adjust for selectivity
	selectivity := 1.0
	for _, condition := range conditions {
		selectivity *= opt.calculateSelectivity(indexStats.Table, condition)
	}

	return baseCost + (float64(indexStats.LeafPages) * selectivity * 0.1)
}

func (opt *QueryOptimizer) calculateIndexSeekCost(indexName string, conditions []Condition) float64 {
	// Index seek is generally cheaper than scan
	return opt.calculateIndexScanCost(indexName, conditions) * 0.5
}

func (opt *QueryOptimizer) calculateSortCost(rows int64) float64 {
	if rows <= 1 {
		return 0.1
	}

	// Cost is O(n log n) for sorting
	logRows := 1.0
	tempRows := rows
	for tempRows > 1 {
		tempRows /= 2
		logRows++
	}

	return float64(rows) * logRows * 0.001
}

// UpdateStatistics updates table and index statistics
func (opt *QueryOptimizer) UpdateStatistics(table string, stats *TableStats) {
	opt.statistics.TableStats[table] = stats
}

// AddIndexStatistics adds index statistics
func (opt *QueryOptimizer) AddIndexStatistics(indexName string, stats *IndexStats) {
	opt.statistics.IndexStats[indexName] = stats
}

// GetExecutionPlan returns a string representation of the execution plan
func (plan *ExecutionPlan) String() string {
	var builder strings.Builder

	builder.WriteString("Execution Plan:\n")
	for i, op := range plan.Operations {
		builder.WriteString(fmt.Sprintf("%d. %s on %s", i+1, op.Type.String(), op.Table))
		if op.Index != "" {
			builder.WriteString(fmt.Sprintf(" using index %s", op.Index))
		}
		builder.WriteString(fmt.Sprintf(" (cost: %.2f, rows: %d)\n", op.Cost, op.EstimatedRows))
	}

	if len(plan.Indexes) > 0 {
		builder.WriteString(fmt.Sprintf("Indexes used: %s\n", strings.Join(plan.Indexes, ", ")))
	}

	return builder.String()
}

// String returns a string representation of the operation type
func (opType OperationType) String() string {
	switch opType {
	case OpTableScan:
		return "Table Scan"
	case OpIndexScan:
		return "Index Scan"
	case OpIndexSeek:
		return "Index Seek"
	case OpNestedLoop:
		return "Nested Loop"
	case OpHashJoin:
		return "Hash Join"
	case OpSortMerge:
		return "Sort Merge"
	case OpSort:
		return "Sort"
	case OpFilter:
		return "Filter"
	case OpProject:
		return "Project"
	default:
		return "Unknown"
	}
}
