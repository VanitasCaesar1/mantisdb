package sql

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// QueryExecutor executes optimized query plans against storage engines
type QueryExecutor struct {
	storageManager *StorageManager
	config         *ExecutorConfig
	stats          *ExecutionStats
	mu             sync.RWMutex
}

// ExecutorConfig contains configuration for the query executor
type ExecutorConfig struct {
	MaxWorkers          int
	WorkMem             int64 // Work memory in bytes
	TempBuffers         int64 // Temporary buffer size
	StatementTimeout    time.Duration
	LockTimeout         time.Duration
	EnableVectorization bool
	EnableParallel      bool
	EnableJIT           bool
	BatchSize           int
	HashTableSize       int64
	SortMemThreshold    int64
}

// DefaultExecutorConfig returns default executor configuration
func DefaultExecutorConfig() *ExecutorConfig {
	return &ExecutorConfig{
		MaxWorkers:          8,
		WorkMem:             4 * 1024 * 1024, // 4MB
		TempBuffers:         8 * 1024 * 1024, // 8MB
		StatementTimeout:    30 * time.Second,
		LockTimeout:         5 * time.Second,
		EnableVectorization: true,
		EnableParallel:      true,
		EnableJIT:           false, // Disabled by default
		BatchSize:           1000,
		HashTableSize:       64 * 1024 * 1024, // 64MB
		SortMemThreshold:    16 * 1024 * 1024, // 16MB
	}
}

// StorageManager manages different storage engines
type StorageManager struct {
	kvStore       KVStorageEngine
	docStore      DocumentStorageEngine
	columnarStore ColumnarStorageEngine
	indexManager  IndexManager
	txnManager    TransactionManager

	// Storage adapters for unified access
	kvAdapter       *KVStorageAdapter
	docAdapter      *DocumentStorageAdapter
	columnarAdapter *ColumnarStorageAdapter
}

// KVStorageEngine interface for key-value storage
type KVStorageEngine interface {
	Get(ctx context.Context, key []byte) ([]byte, error)
	Put(ctx context.Context, key, value []byte) error
	Delete(ctx context.Context, key []byte) error
	Scan(ctx context.Context, startKey, endKey []byte) (Iterator, error)
	BatchGet(ctx context.Context, keys [][]byte) ([][]byte, error)
	BatchPut(ctx context.Context, kvPairs []KVPair) error
}

// DocumentStorageEngine interface for document storage
type DocumentStorageEngine interface {
	GetDocument(ctx context.Context, collection, id string) (Document, error)
	PutDocument(ctx context.Context, collection, id string, doc Document) error
	DeleteDocument(ctx context.Context, collection, id string) error
	QueryDocuments(ctx context.Context, collection string, query DocumentQuery) (DocumentIterator, error)
	CreateIndex(ctx context.Context, collection string, index DocumentIndex) error
	DropIndex(ctx context.Context, collection, indexName string) error
}

// ColumnarStorageEngine interface for columnar storage
type ColumnarStorageEngine interface {
	GetColumn(ctx context.Context, table, column string, rowIDs []int64) (ColumnData, error)
	PutColumn(ctx context.Context, table, column string, data ColumnData) error
	ScanColumn(ctx context.Context, table, column string, predicate Predicate) (ColumnIterator, error)
	GetRowGroup(ctx context.Context, table string, rowGroupID int64) (RowGroup, error)
	CreateTable(ctx context.Context, table string, schema TableSchema) error
	DropTable(ctx context.Context, table string) error
}

// IndexManager manages indexes across storage engines
type IndexManager interface {
	CreateIndex(ctx context.Context, indexDef IndexDefinition) error
	DropIndex(ctx context.Context, indexName string) error
	GetIndex(ctx context.Context, indexName string) (Index, error)
	ListIndexes(ctx context.Context, tableName string) ([]IndexDefinition, error)
	IndexScan(ctx context.Context, indexName string, scanKey ScanKey) (Iterator, error)
}

// TransactionManager manages ACID transactions
type TransactionManager interface {
	BeginTransaction(ctx context.Context, isolation IsolationLevel) (Transaction, error)
	CommitTransaction(ctx context.Context, txn Transaction) error
	RollbackTransaction(ctx context.Context, txn Transaction) error
	GetTransaction(ctx context.Context) Transaction
}

// ExecutionStats tracks query execution statistics
type ExecutionStats struct {
	QueriesExecuted    int64
	TotalExecutionTime time.Duration
	RowsProcessed      int64
	BytesProcessed     int64
	CacheHits          int64
	CacheMisses        int64
	IndexScans         int64
	SeqScans           int64
	JoinsExecuted      int64
	SortsExecuted      int64
}

// ExecutionContext contains context for query execution
type ExecutionContext struct {
	Context        context.Context
	Transaction    Transaction
	SQLTransaction *SQLTransaction
	Parameters     []any
	User           string
	Database       string
	StartTime      time.Time
	WorkMem        int64
	TempDir        string
	Stats          *ExecutionStats
	IsolationLevel IsolationLevel
	ReadOnly       bool
}

// ResultSet represents query execution results
type ResultSet struct {
	Columns []ColumnInfo
	Rows    []Row
	Stats   ExecutionStats
	Error   error
}

// ColumnInfo describes a result column
type ColumnInfo struct {
	Name     string
	Type     DataType
	Nullable bool
	Length   int
}

// Row represents a single result row
type Row struct {
	Values []interface{}
}

// Iterator interface for scanning data
type Iterator interface {
	Next() bool
	Value() ([]byte, []byte, error)
	Error() error
	Close() error
}

// DocumentIterator interface for scanning documents
type DocumentIterator interface {
	Next() bool
	Document() (Document, error)
	Error() error
	Close() error
}

// ColumnIterator interface for scanning columns
type ColumnIterator interface {
	Next() bool
	Value() (interface{}, error)
	Error() error
	Close() error
}

// Supporting types
type KVPair struct {
	Key   []byte
	Value []byte
}

type Document map[string]interface{}

type DocumentQuery struct {
	Filter     map[string]interface{}
	Sort       []SortField
	Limit      int
	Offset     int
	Projection []string
}

type DocumentIndex struct {
	Name    string
	Fields  []IndexField
	Unique  bool
	Sparse  bool
	Options map[string]interface{}
}

type IndexField struct {
	Field string
	Order int // 1 for ascending, -1 for descending
}

type SortField struct {
	Field string
	Order int
}

type ColumnData struct {
	Type   DataType
	Values []interface{}
	Nulls  []bool
}

type Predicate struct {
	Column   string
	Operator string
	Value    interface{}
}

type RowGroup struct {
	ID       int64
	Columns  map[string]ColumnData
	RowCount int64
}

type TableSchema struct {
	Columns []ColumnSchema
	Indexes []IndexDefinition
}

type ColumnSchema struct {
	Name     string
	Type     DataType
	Nullable bool
	Default  interface{}
}

type IndexDefinition struct {
	Name    string
	Table   string
	Columns []string
	Type    IndexType
	Unique  bool
	Options map[string]interface{}
}

type IndexType int

const (
	IndexTypeBTree IndexType = iota
	IndexTypeHash
	IndexTypeGIN
	IndexTypeGiST
	IndexTypeBRIN
)

type Index interface {
	Scan(ctx context.Context, key ScanKey) (Iterator, error)
	Insert(ctx context.Context, key, value []byte) error
	Delete(ctx context.Context, key []byte) error
	Stats() IndexStats
}

type IndexStats struct {
	Pages    int64
	Tuples   int64
	Size     int64
	LastUsed time.Time
}

type IsolationLevel int

const (
	ReadUncommitted IsolationLevel = iota
	ReadCommitted
	RepeatableRead
	Serializable
)

type Transaction interface {
	ID() string
	IsolationLevel() IsolationLevel
	StartTime() time.Time
	IsReadOnly() bool
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// NewQueryExecutor creates a new query executor
func NewQueryExecutor(storageManager *StorageManager) *QueryExecutor {
	// Initialize storage adapters
	storageManager.kvAdapter = NewKVStorageAdapter(storageManager.kvStore)
	storageManager.docAdapter = NewDocumentStorageAdapter(storageManager.docStore)
	storageManager.columnarAdapter = NewColumnarStorageAdapter(storageManager.columnarStore)

	return &QueryExecutor{
		storageManager: storageManager,
		config:         DefaultExecutorConfig(),
		stats:          &ExecutionStats{},
	}
}

// Execute executes a query plan and returns results
func (qe *QueryExecutor) Execute(ctx context.Context, plan *QueryPlan, params []interface{}) (*ResultSet, error) {
	execCtx := &ExecutionContext{
		Context:    ctx,
		Parameters: params,
		StartTime:  time.Now(),
		WorkMem:    qe.config.WorkMem,
		Stats:      &ExecutionStats{},
	}

	// Set timeout
	if qe.config.StatementTimeout > 0 {
		var cancel context.CancelFunc
		execCtx.Context, cancel = context.WithTimeout(ctx, qe.config.StatementTimeout)
		defer cancel()
	}

	// Execute the plan
	result, err := qe.executePlan(execCtx, plan)
	if err != nil {
		return &ResultSet{Error: err}, err
	}

	// Update global stats
	qe.mu.Lock()
	qe.stats.QueriesExecuted++
	qe.stats.TotalExecutionTime += time.Since(execCtx.StartTime)
	qe.stats.RowsProcessed += execCtx.Stats.RowsProcessed
	qe.stats.BytesProcessed += execCtx.Stats.BytesProcessed
	qe.mu.Unlock()

	return result, nil
}

// executePlan executes a specific plan node
func (qe *QueryExecutor) executePlan(ctx *ExecutionContext, plan *QueryPlan) (*ResultSet, error) {
	switch plan.Type {
	case PlanTypeSeqScan:
		return qe.executeSeqScan(ctx, plan)
	case PlanTypeIndexScan:
		return qe.executeIndexScan(ctx, plan)
	case PlanTypeBitmapIndexScan:
		return qe.executeBitmapIndexScan(ctx, plan)
	case PlanTypeBitmapHeapScan:
		return qe.executeBitmapHeapScan(ctx, plan)
	case PlanTypeNestLoop:
		return qe.executeNestLoop(ctx, plan)
	case PlanTypeHashJoin:
		return qe.executeHashJoin(ctx, plan)
	case PlanTypeMergeJoin:
		return qe.executeMergeJoin(ctx, plan)
	case PlanTypeSort:
		return qe.executeSort(ctx, plan)
	case PlanTypeHash:
		return qe.executeHash(ctx, plan)
	case PlanTypeMaterial:
		return qe.executeMaterial(ctx, plan)
	case PlanTypeAggregate:
		return qe.executeAggregateEnhanced(ctx, plan)
	case PlanTypeGroup:
		return qe.executeGroup(ctx, plan)
	case PlanTypeLimit:
		return qe.executeLimit(ctx, plan)
	case PlanTypeSubqueryScan:
		return qe.executeSubqueryScan(ctx, plan)
	case PlanTypeParallelSeqScan:
		return qe.executeParallelSeqScan(ctx, plan)
	case PlanTypeGather:
		return qe.executeGather(ctx, plan)
	default:
		return nil, fmt.Errorf("unsupported plan type: %v", plan.Type)
	}
}

// executeSeqScan executes a sequential scan using unified storage adapters
func (qe *QueryExecutor) executeSeqScan(ctx *ExecutionContext, plan *QueryPlan) (*ResultSet, error) {
	// Check if vectorization should be used
	if qe.config.EnableVectorization && qe.shouldUseVectorization(plan) {
		return qe.executeVectorizedScan(ctx, plan)
	}

	// Determine storage engine based on table type
	storageType := qe.getStorageType(plan.TableName)

	var unifiedResult *UnifiedResultSet
	var err error

	switch storageType {
	case StorageTypeKV:
		unifiedResult, err = qe.storageManager.kvAdapter.Scan(ctx.Context, plan.TableName, plan.Qual)
	case StorageTypeDocument:
		unifiedResult, err = qe.storageManager.docAdapter.Scan(ctx.Context, plan.TableName, plan.Qual)
	case StorageTypeColumnar:
		unifiedResult, err = qe.storageManager.columnarAdapter.Scan(ctx.Context, plan.TableName, plan.Qual)
	default:
		return nil, fmt.Errorf("unknown storage type for table %s", plan.TableName)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to execute scan on %s storage: %w", storageType, err)
	}

	// Convert unified result to standard result set
	columns := qe.getTableColumns(plan.TableName)
	var rows []Row

	for _, unifiedRow := range unifiedResult.Rows {
		row := Row{Values: make([]any, len(columns))}
		for i, col := range columns {
			if val, exists := unifiedRow.Data[col.Name]; exists {
				row.Values[i] = val
			}
		}
		rows = append(rows, row)
	}

	// Update statistics
	ctx.Stats.RowsProcessed += unifiedResult.TotalRows
	ctx.Stats.SeqScans++

	return &ResultSet{
		Columns: columns,
		Rows:    rows,
		Stats:   *ctx.Stats,
	}, nil
}

// executeKVSeqScan executes a sequential scan on KV storage
func (qe *QueryExecutor) executeKVSeqScan(ctx *ExecutionContext, plan *QueryPlan) (*ResultSet, error) {
	startKey := []byte(plan.TableName + "/")
	endKey := []byte(plan.TableName + "/~")

	iter, err := qe.storageManager.kvStore.Scan(ctx.Context, startKey, endKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create KV iterator: %w", err)
	}
	defer iter.Close()

	var rows []Row
	columns := qe.getTableColumns(plan.TableName)

	for iter.Next() {
		key, value, err := iter.Value()
		if err != nil {
			return nil, fmt.Errorf("failed to read KV pair: %w", err)
		}

		// Apply qualifiers
		if qe.matchesQualifiers(key, value, plan.Qual) {
			row := qe.kvToRow(key, value, columns)
			rows = append(rows, row)
			ctx.Stats.RowsProcessed++
			ctx.Stats.BytesProcessed += int64(len(key) + len(value))
		}

		// Check for cancellation
		select {
		case <-ctx.Context.Done():
			return nil, ctx.Context.Err()
		default:
		}
	}

	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("iterator error: %w", err)
	}

	ctx.Stats.SeqScans++

	return &ResultSet{
		Columns: columns,
		Rows:    rows,
		Stats:   *ctx.Stats,
	}, nil
}

// executeDocumentSeqScan executes a sequential scan on document storage
func (qe *QueryExecutor) executeDocumentSeqScan(ctx *ExecutionContext, plan *QueryPlan) (*ResultSet, error) {
	query := DocumentQuery{
		Filter: qe.qualifiersToDocumentFilter(plan.Qual),
	}

	iter, err := qe.storageManager.docStore.QueryDocuments(ctx.Context, plan.TableName, query)
	if err != nil {
		return nil, fmt.Errorf("failed to create document iterator: %w", err)
	}
	defer iter.Close()

	var rows []Row
	columns := qe.getTableColumns(plan.TableName)

	for iter.Next() {
		doc, err := iter.Document()
		if err != nil {
			return nil, fmt.Errorf("failed to read document: %w", err)
		}

		row := qe.documentToRow(doc, columns)
		rows = append(rows, row)
		ctx.Stats.RowsProcessed++

		// Check for cancellation
		select {
		case <-ctx.Context.Done():
			return nil, ctx.Context.Err()
		default:
		}
	}

	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("iterator error: %w", err)
	}

	ctx.Stats.SeqScans++

	return &ResultSet{
		Columns: columns,
		Rows:    rows,
		Stats:   *ctx.Stats,
	}, nil
}

// executeColumnarSeqScan executes a sequential scan on columnar storage
func (qe *QueryExecutor) executeColumnarSeqScan(ctx *ExecutionContext, plan *QueryPlan) (*ResultSet, error) {
	columns := qe.getTableColumns(plan.TableName)
	columnNames := make([]string, len(columns))
	for i, col := range columns {
		columnNames[i] = col.Name
	}

	// Read all columns
	columnData := make(map[string]ColumnData)
	for _, colName := range columnNames {
		data, err := qe.storageManager.columnarStore.GetColumn(ctx.Context, plan.TableName, colName, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to read column %s: %w", colName, err)
		}
		columnData[colName] = data
	}

	// Convert columnar data to rows
	var rows []Row
	if len(columnData) > 0 {
		// Get row count from first column
		var rowCount int
		for _, data := range columnData {
			rowCount = len(data.Values)
			break
		}

		for i := 0; i < rowCount; i++ {
			row := Row{Values: make([]interface{}, len(columns))}

			// Apply qualifiers at row level
			if qe.matchesColumnarQualifiers(columnData, i, plan.Qual) {
				for j, col := range columns {
					if data, exists := columnData[col.Name]; exists {
						if i < len(data.Values) {
							if data.Nulls != nil && i < len(data.Nulls) && data.Nulls[i] {
								row.Values[j] = nil
							} else {
								row.Values[j] = data.Values[i]
							}
						}
					}
				}
				rows = append(rows, row)
				ctx.Stats.RowsProcessed++
			}

			// Check for cancellation
			select {
			case <-ctx.Context.Done():
				return nil, ctx.Context.Err()
			default:
			}
		}
	}

	ctx.Stats.SeqScans++

	return &ResultSet{
		Columns: columns,
		Rows:    rows,
		Stats:   *ctx.Stats,
	}, nil
}

// executeIndexScan executes an index scan
func (qe *QueryExecutor) executeIndexScan(ctx *ExecutionContext, plan *QueryPlan) (*ResultSet, error) {
	index, err := qe.storageManager.indexManager.GetIndex(ctx.Context, plan.IndexName)
	if err != nil {
		return nil, fmt.Errorf("failed to get index %s: %w", plan.IndexName, err)
	}

	// Build scan key from qualifiers
	scanKey := qe.buildScanKey(plan.ScanKeys)

	iter, err := index.Scan(ctx.Context, scanKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create index iterator: %w", err)
	}
	defer iter.Close()

	var rows []Row
	columns := qe.getTableColumns(plan.TableName)

	for iter.Next() {
		key, value, err := iter.Value()
		if err != nil {
			return nil, fmt.Errorf("failed to read index entry: %w", err)
		}

		// Fetch actual row data using the index value (row ID)
		row, err := qe.fetchRowByID(ctx, plan.TableName, value)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch row: %w", err)
		}

		rows = append(rows, row)
		ctx.Stats.RowsProcessed++
		ctx.Stats.BytesProcessed += int64(len(key) + len(value))

		// Check for cancellation
		select {
		case <-ctx.Context.Done():
			return nil, ctx.Context.Err()
		default:
		}
	}

	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("iterator error: %w", err)
	}

	ctx.Stats.IndexScans++

	return &ResultSet{
		Columns: columns,
		Rows:    rows,
		Stats:   *ctx.Stats,
	}, nil
}

// executeHashJoin executes a hash join with optional parallelization
func (qe *QueryExecutor) executeHashJoin(ctx *ExecutionContext, plan *QueryPlan) (*ResultSet, error) {
	// Check if parallel execution should be used
	if qe.config.EnableParallel && qe.shouldUseParallelJoin(plan) {
		return qe.executeParallelHashJoin(ctx, plan)
	}

	// Execute inner (build) side
	innerResult, err := qe.executePlan(ctx, plan.RightTree)
	if err != nil {
		return nil, fmt.Errorf("failed to execute inner side of hash join: %w", err)
	}

	// Build hash table
	hashTable := make(map[string][]Row)
	for _, row := range innerResult.Rows {
		// Extract join key (simplified - assumes first column)
		key := fmt.Sprintf("%v", row.Values[0])
		hashTable[key] = append(hashTable[key], row)
	}

	// Execute outer (probe) side
	outerResult, err := qe.executePlan(ctx, plan.LeftTree)
	if err != nil {
		return nil, fmt.Errorf("failed to execute outer side of hash join: %w", err)
	}

	// Probe hash table
	var joinedRows []Row
	combinedColumns := append(outerResult.Columns, innerResult.Columns...)

	for _, outerRow := range outerResult.Rows {
		// Extract join key (simplified - assumes first column)
		key := fmt.Sprintf("%v", outerRow.Values[0])

		if innerRows, exists := hashTable[key]; exists {
			for _, innerRow := range innerRows {
				// Combine rows
				joinedRow := Row{
					Values: append(outerRow.Values, innerRow.Values...),
				}
				joinedRows = append(joinedRows, joinedRow)
				ctx.Stats.RowsProcessed++
			}
		}

		// Check for cancellation
		select {
		case <-ctx.Context.Done():
			return nil, ctx.Context.Err()
		default:
		}
	}

	ctx.Stats.JoinsExecuted++

	return &ResultSet{
		Columns: combinedColumns,
		Rows:    joinedRows,
		Stats:   *ctx.Stats,
	}, nil
}

// executeParallelSeqScan executes a parallel sequential scan
func (qe *QueryExecutor) executeParallelSeqScan(ctx *ExecutionContext, plan *QueryPlan) (*ResultSet, error) {
	if !qe.config.EnableParallel || plan.Workers <= 1 {
		// Fall back to regular sequential scan
		return qe.executeSeqScan(ctx, plan)
	}

	// Create worker contexts
	workers := plan.Workers
	if workers > qe.config.MaxWorkers {
		workers = qe.config.MaxWorkers
	}

	// Channel for collecting results
	resultChan := make(chan *ResultSet, workers)
	errorChan := make(chan error, workers)

	// Launch workers
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// Create worker-specific plan (partition the data)
			workerPlan := *plan
			workerPlan.Type = PlanTypeSeqScan // Convert to regular seq scan

			// Execute worker scan
			result, err := qe.executeSeqScan(ctx, &workerPlan)
			if err != nil {
				errorChan <- err
				return
			}

			resultChan <- result
		}(i)
	}

	// Wait for workers to complete
	go func() {
		wg.Wait()
		close(resultChan)
		close(errorChan)
	}()

	// Collect results
	var allRows []Row
	var columns []ColumnInfo
	combinedStats := ExecutionStats{}

	for {
		select {
		case result, ok := <-resultChan:
			if !ok {
				// All workers done
				return &ResultSet{
					Columns: columns,
					Rows:    allRows,
					Stats:   combinedStats,
				}, nil
			}

			if len(columns) == 0 {
				columns = result.Columns
			}
			allRows = append(allRows, result.Rows...)

			// Combine stats
			combinedStats.RowsProcessed += result.Stats.RowsProcessed
			combinedStats.BytesProcessed += result.Stats.BytesProcessed
			combinedStats.SeqScans += result.Stats.SeqScans

		case err := <-errorChan:
			return nil, fmt.Errorf("parallel scan worker error: %w", err)

		case <-ctx.Context.Done():
			return nil, ctx.Context.Err()
		}
	}
}

// Helper methods

type StorageType int

const (
	StorageTypeKV StorageType = iota
	StorageTypeDocument
	StorageTypeColumnar
)

// getStorageType determines the storage type for a table
func (qe *QueryExecutor) getStorageType(tableName string) StorageType {
	// This would be determined by table metadata
	// For now, use simple heuristics
	if strings.HasPrefix(tableName, "kv_") {
		return StorageTypeKV
	} else if strings.HasPrefix(tableName, "doc_") {
		return StorageTypeDocument
	} else {
		return StorageTypeColumnar
	}
}

// getTableColumns returns column information for a table
func (qe *QueryExecutor) getTableColumns(tableName string) []ColumnInfo {
	// This would come from table metadata
	// For now, return dummy columns
	return []ColumnInfo{
		{Name: "id", Type: DataType{Name: "INTEGER"}, Nullable: false},
		{Name: "data", Type: DataType{Name: "TEXT"}, Nullable: true},
	}
}

// Placeholder implementations for remaining methods
func (qe *QueryExecutor) executeBitmapIndexScan(ctx *ExecutionContext, plan *QueryPlan) (*ResultSet, error) {
	return qe.executeIndexScan(ctx, plan)
}

func (qe *QueryExecutor) executeBitmapHeapScan(ctx *ExecutionContext, plan *QueryPlan) (*ResultSet, error) {
	return qe.executeSeqScan(ctx, plan)
}

func (qe *QueryExecutor) executeNestLoop(ctx *ExecutionContext, plan *QueryPlan) (*ResultSet, error) {
	return qe.executeHashJoin(ctx, plan) // Simplified
}

func (qe *QueryExecutor) executeMergeJoin(ctx *ExecutionContext, plan *QueryPlan) (*ResultSet, error) {
	return qe.executeHashJoin(ctx, plan) // Simplified
}

func (qe *QueryExecutor) executeSort(ctx *ExecutionContext, plan *QueryPlan) (*ResultSet, error) {
	result, err := qe.executePlan(ctx, plan.LeftTree)
	if err != nil {
		return nil, err
	}

	// Sort rows (simplified implementation)
	ctx.Stats.SortsExecuted++
	return result, nil
}

func (qe *QueryExecutor) executeHash(ctx *ExecutionContext, plan *QueryPlan) (*ResultSet, error) {
	return qe.executePlan(ctx, plan.LeftTree)
}

func (qe *QueryExecutor) executeMaterial(ctx *ExecutionContext, plan *QueryPlan) (*ResultSet, error) {
	return qe.executePlan(ctx, plan.LeftTree)
}

func (qe *QueryExecutor) executeAggregate(ctx *ExecutionContext, plan *QueryPlan) (*ResultSet, error) {
	return qe.executePlan(ctx, plan.LeftTree)
}

func (qe *QueryExecutor) executeGroup(ctx *ExecutionContext, plan *QueryPlan) (*ResultSet, error) {
	return qe.executePlan(ctx, plan.LeftTree)
}

func (qe *QueryExecutor) executeLimit(ctx *ExecutionContext, plan *QueryPlan) (*ResultSet, error) {
	result, err := qe.executePlan(ctx, plan.LeftTree)
	if err != nil {
		return nil, err
	}

	// Apply limit (simplified)
	if plan.Limit != nil {
		// Would extract limit value from expression
		limit := 100 // Placeholder
		if len(result.Rows) > limit {
			result.Rows = result.Rows[:limit]
		}
	}

	return result, nil
}

func (qe *QueryExecutor) executeSubqueryScan(ctx *ExecutionContext, plan *QueryPlan) (*ResultSet, error) {
	return qe.executePlan(ctx, plan.LeftTree)
}

func (qe *QueryExecutor) executeGather(ctx *ExecutionContext, plan *QueryPlan) (*ResultSet, error) {
	return qe.executePlan(ctx, plan.LeftTree)
}

func (qe *QueryExecutor) matchesQualifiers(key, value []byte, quals []Expression) bool {
	// Simplified qualifier matching
	return true
}

func (qe *QueryExecutor) matchesColumnarQualifiers(columnData map[string]ColumnData, rowIndex int, quals []Expression) bool {
	// Simplified qualifier matching for columnar data
	return true
}

func (qe *QueryExecutor) kvToRow(key, value []byte, columns []ColumnInfo) Row {
	// Convert KV pair to row
	return Row{Values: []interface{}{string(key), string(value)}}
}

func (qe *QueryExecutor) documentToRow(doc Document, columns []ColumnInfo) Row {
	// Convert document to row
	values := make([]interface{}, len(columns))
	for i, col := range columns {
		if val, exists := doc[col.Name]; exists {
			values[i] = val
		}
	}
	return Row{Values: values}
}

func (qe *QueryExecutor) qualifiersToDocumentFilter(quals []Expression) map[string]interface{} {
	// Convert SQL qualifiers to document filter
	return make(map[string]interface{})
}

func (qe *QueryExecutor) buildScanKey(scanKeys []ScanKey) ScanKey {
	// Build scan key from qualifiers
	if len(scanKeys) > 0 {
		return scanKeys[0]
	}
	return ScanKey{}
}

func (qe *QueryExecutor) fetchRowByID(ctx *ExecutionContext, tableName string, rowID []byte) (Row, error) {
	// Fetch row by ID from storage
	return Row{Values: []interface{}{"dummy", "data"}}, nil
}

// GetStats returns execution statistics
func (qe *QueryExecutor) GetStats() ExecutionStats {
	qe.mu.RLock()
	defer qe.mu.RUnlock()
	return *qe.stats
}

// ResetStats resets execution statistics
func (qe *QueryExecutor) ResetStats() {
	qe.mu.Lock()
	defer qe.mu.Unlock()
	qe.stats = &ExecutionStats{}
}

// Storage Adapters for unified access to different storage models

// KVStorageAdapter adapts key-value storage to unified interface
type KVStorageAdapter struct {
	store KVStorageEngine
}

// NewKVStorageAdapter creates a new KV storage adapter
func NewKVStorageAdapter(store KVStorageEngine) *KVStorageAdapter {
	return &KVStorageAdapter{store: store}
}

// Scan performs a unified scan operation on KV storage
func (adapter *KVStorageAdapter) Scan(ctx context.Context, tableName string, filters []Expression) (*UnifiedResultSet, error) {
	startKey := []byte(tableName + "/")
	endKey := []byte(tableName + "/~")

	iter, err := adapter.store.Scan(ctx, startKey, endKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create KV iterator: %w", err)
	}
	defer iter.Close()

	var rows []UnifiedRow
	for iter.Next() {
		key, value, err := iter.Value()
		if err != nil {
			return nil, fmt.Errorf("failed to read KV pair: %w", err)
		}

		// Convert KV pair to unified row format
		row := UnifiedRow{
			Data: map[string]any{
				"key":   string(key),
				"value": string(value),
			},
			RowID:     string(key),
			TableName: tableName,
		}

		// Apply filters
		if adapter.matchesFilters(row, filters) {
			rows = append(rows, row)
		}
	}

	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("iterator error: %w", err)
	}

	return &UnifiedResultSet{
		Rows:        rows,
		TotalRows:   int64(len(rows)),
		StorageType: StorageTypeKV,
	}, nil
}

// BatchGet performs batch get operations on KV storage
func (adapter *KVStorageAdapter) BatchGet(ctx context.Context, keys [][]byte) ([]UnifiedRow, error) {
	values, err := adapter.store.BatchGet(ctx, keys)
	if err != nil {
		return nil, err
	}

	var rows []UnifiedRow
	for i, key := range keys {
		if i < len(values) && values[i] != nil {
			row := UnifiedRow{
				Data: map[string]any{
					"key":   string(key),
					"value": string(values[i]),
				},
				RowID: string(key),
			}
			rows = append(rows, row)
		}
	}

	return rows, nil
}

// DocumentStorageAdapter adapts document storage to unified interface
type DocumentStorageAdapter struct {
	store DocumentStorageEngine
}

// NewDocumentStorageAdapter creates a new document storage adapter
func NewDocumentStorageAdapter(store DocumentStorageEngine) *DocumentStorageAdapter {
	return &DocumentStorageAdapter{store: store}
}

// Scan performs a unified scan operation on document storage
func (adapter *DocumentStorageAdapter) Scan(ctx context.Context, tableName string, filters []Expression) (*UnifiedResultSet, error) {
	query := DocumentQuery{
		Filter: adapter.convertFiltersToDocumentQuery(filters),
	}

	iter, err := adapter.store.QueryDocuments(ctx, tableName, query)
	if err != nil {
		return nil, fmt.Errorf("failed to create document iterator: %w", err)
	}
	defer iter.Close()

	var rows []UnifiedRow
	for iter.Next() {
		doc, err := iter.Document()
		if err != nil {
			return nil, fmt.Errorf("failed to read document: %w", err)
		}

		// Convert document to unified row format
		row := UnifiedRow{
			Data:      doc,
			RowID:     fmt.Sprintf("%s/%s", tableName, getDocumentID(doc)),
			TableName: tableName,
		}

		rows = append(rows, row)
	}

	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("iterator error: %w", err)
	}

	return &UnifiedResultSet{
		Rows:        rows,
		TotalRows:   int64(len(rows)),
		StorageType: StorageTypeDocument,
	}, nil
}

// ColumnarStorageAdapter adapts columnar storage to unified interface
type ColumnarStorageAdapter struct {
	store ColumnarStorageEngine
}

// NewColumnarStorageAdapter creates a new columnar storage adapter
func NewColumnarStorageAdapter(store ColumnarStorageEngine) *ColumnarStorageAdapter {
	return &ColumnarStorageAdapter{store: store}
}

// Scan performs a unified scan operation on columnar storage
func (adapter *ColumnarStorageAdapter) Scan(ctx context.Context, tableName string, filters []Expression) (*UnifiedResultSet, error) {
	// Get table schema to determine columns
	schema, err := adapter.getTableSchema(ctx, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get table schema: %w", err)
	}

	// Read all columns
	columnData := make(map[string]ColumnData)
	for _, col := range schema.Columns {
		data, err := adapter.store.GetColumn(ctx, tableName, col.Name, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to read column %s: %w", col.Name, err)
		}
		columnData[col.Name] = data
	}

	// Convert columnar data to unified rows
	var rows []UnifiedRow
	if len(columnData) > 0 {
		// Get row count from first column
		var rowCount int
		for _, data := range columnData {
			rowCount = len(data.Values)
			break
		}

		for i := 0; i < rowCount; i++ {
			rowData := make(map[string]any)

			// Apply filters at row level before creating the row
			if adapter.matchesColumnarFilters(columnData, i, filters) {
				for _, col := range schema.Columns {
					if data, exists := columnData[col.Name]; exists && i < len(data.Values) {
						if data.Nulls != nil && i < len(data.Nulls) && data.Nulls[i] {
							rowData[col.Name] = nil
						} else {
							rowData[col.Name] = data.Values[i]
						}
					}
				}

				row := UnifiedRow{
					Data:      rowData,
					RowID:     fmt.Sprintf("%s/%d", tableName, i),
					TableName: tableName,
				}

				rows = append(rows, row)
			}
		}
	}

	return &UnifiedResultSet{
		Rows:        rows,
		TotalRows:   int64(len(rows)),
		StorageType: StorageTypeColumnar,
	}, nil
}

// Unified data structures for cross-storage operations

// UnifiedRow represents a row that can come from any storage type
type UnifiedRow struct {
	Data      map[string]any `json:"data"`
	RowID     string         `json:"row_id"`
	TableName string         `json:"table_name"`
	Version   int64          `json:"version"`
}

// UnifiedResultSet represents results from any storage type
type UnifiedResultSet struct {
	Rows        []UnifiedRow   `json:"rows"`
	TotalRows   int64          `json:"total_rows"`
	StorageType StorageType    `json:"storage_type"`
	Metadata    map[string]any `json:"metadata"`
}

// Enhanced execution methods with vectorization and parallelization

// executeVectorizedScan performs vectorized scanning for analytical queries
func (qe *QueryExecutor) executeVectorizedScan(ctx *ExecutionContext, plan *QueryPlan) (*ResultSet, error) {
	if !qe.config.EnableVectorization {
		// Fall back to regular scan
		return qe.executeSeqScan(ctx, plan)
	}

	storageType := qe.getStorageType(plan.TableName)

	switch storageType {
	case StorageTypeColumnar:
		return qe.executeVectorizedColumnarScan(ctx, plan)
	default:
		// Vectorization not supported for this storage type
		return qe.executeSeqScan(ctx, plan)
	}
}

// executeVectorizedColumnarScan performs vectorized scanning on columnar data
func (qe *QueryExecutor) executeVectorizedColumnarScan(ctx *ExecutionContext, plan *QueryPlan) (*ResultSet, error) {
	adapter := qe.storageManager.columnarAdapter

	// Get unified result set
	unifiedResult, err := adapter.Scan(ctx.Context, plan.TableName, plan.Qual)
	if err != nil {
		return nil, err
	}

	// Convert to standard result set
	columns := qe.getTableColumns(plan.TableName)
	var rows []Row

	for _, unifiedRow := range unifiedResult.Rows {
		row := Row{Values: make([]any, len(columns))}
		for i, col := range columns {
			if val, exists := unifiedRow.Data[col.Name]; exists {
				row.Values[i] = val
			}
		}
		rows = append(rows, row)
		ctx.Stats.RowsProcessed++
	}

	ctx.Stats.SeqScans++

	return &ResultSet{
		Columns: columns,
		Rows:    rows,
		Stats:   *ctx.Stats,
	}, nil
}

// executeParallelHashJoin performs parallel hash join execution
func (qe *QueryExecutor) executeParallelHashJoin(ctx *ExecutionContext, plan *QueryPlan) (*ResultSet, error) {
	if !qe.config.EnableParallel || plan.Workers <= 1 {
		return qe.executeHashJoin(ctx, plan)
	}

	// Execute inner (build) side
	innerResult, err := qe.executePlan(ctx, plan.RightTree)
	if err != nil {
		return nil, fmt.Errorf("failed to execute inner side of parallel hash join: %w", err)
	}

	// Build hash table in parallel
	hashTable := qe.buildParallelHashTable(innerResult.Rows, plan.Workers)

	// Execute outer (probe) side in parallel
	outerResult, err := qe.executePlan(ctx, plan.LeftTree)
	if err != nil {
		return nil, fmt.Errorf("failed to execute outer side of parallel hash join: %w", err)
	}

	// Probe hash table in parallel
	joinedRows := qe.probeParallelHashTable(outerResult.Rows, hashTable, plan.Workers)

	combinedColumns := append(outerResult.Columns, innerResult.Columns...)
	ctx.Stats.JoinsExecuted++
	ctx.Stats.RowsProcessed += int64(len(joinedRows))

	return &ResultSet{
		Columns: combinedColumns,
		Rows:    joinedRows,
		Stats:   *ctx.Stats,
	}, nil
}

// buildParallelHashTable builds a hash table using multiple workers
func (qe *QueryExecutor) buildParallelHashTable(rows []Row, workers int) map[string][]Row {
	if workers <= 1 || len(rows) < 1000 {
		// Use single-threaded approach for small datasets
		hashTable := make(map[string][]Row)
		for _, row := range rows {
			key := fmt.Sprintf("%v", row.Values[0])
			hashTable[key] = append(hashTable[key], row)
		}
		return hashTable
	}

	// Parallel hash table building
	hashTable := make(map[string][]Row)
	chunkSize := len(rows) / workers
	if chunkSize == 0 {
		chunkSize = 1
	}

	type hashChunk struct {
		table map[string][]Row
		err   error
	}

	resultChan := make(chan hashChunk, workers)

	// Launch workers
	for i := 0; i < workers; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if i == workers-1 {
			end = len(rows) // Last worker takes remaining rows
		}

		go func(chunk []Row) {
			localTable := make(map[string][]Row)
			for _, row := range chunk {
				key := fmt.Sprintf("%v", row.Values[0])
				localTable[key] = append(localTable[key], row)
			}
			resultChan <- hashChunk{table: localTable}
		}(rows[start:end])
	}

	// Collect results and merge
	for i := 0; i < workers; i++ {
		chunk := <-resultChan
		if chunk.err != nil {
			continue // Skip errors for now
		}

		for key, chunkRows := range chunk.table {
			hashTable[key] = append(hashTable[key], chunkRows...)
		}
	}

	return hashTable
}

// probeParallelHashTable probes the hash table using multiple workers
func (qe *QueryExecutor) probeParallelHashTable(outerRows []Row, hashTable map[string][]Row, workers int) []Row {
	if workers <= 1 || len(outerRows) < 1000 {
		// Use single-threaded approach
		var joinedRows []Row
		for _, outerRow := range outerRows {
			key := fmt.Sprintf("%v", outerRow.Values[0])
			if innerRows, exists := hashTable[key]; exists {
				for _, innerRow := range innerRows {
					joinedRow := Row{
						Values: append(outerRow.Values, innerRow.Values...),
					}
					joinedRows = append(joinedRows, joinedRow)
				}
			}
		}
		return joinedRows
	}

	// Parallel probing
	chunkSize := len(outerRows) / workers
	if chunkSize == 0 {
		chunkSize = 1
	}

	type probeResult struct {
		rows []Row
		err  error
	}

	resultChan := make(chan probeResult, workers)

	// Launch workers
	for i := 0; i < workers; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if i == workers-1 {
			end = len(outerRows)
		}

		go func(chunk []Row) {
			var localJoinedRows []Row
			for _, outerRow := range chunk {
				key := fmt.Sprintf("%v", outerRow.Values[0])
				if innerRows, exists := hashTable[key]; exists {
					for _, innerRow := range innerRows {
						joinedRow := Row{
							Values: append(outerRow.Values, innerRow.Values...),
						}
						localJoinedRows = append(localJoinedRows, joinedRow)
					}
				}
			}
			resultChan <- probeResult{rows: localJoinedRows}
		}(outerRows[start:end])
	}

	// Collect results
	var allJoinedRows []Row
	for i := 0; i < workers; i++ {
		result := <-resultChan
		if result.err == nil {
			allJoinedRows = append(allJoinedRows, result.rows...)
		}
	}

	return allJoinedRows
}

// Helper methods for storage adapters

func (adapter *KVStorageAdapter) matchesFilters(row UnifiedRow, filters []Expression) bool {
	// Simplified filter matching for KV storage
	// In a real implementation, this would evaluate SQL expressions
	return true
}

func (adapter *DocumentStorageAdapter) convertFiltersToDocumentQuery(filters []Expression) map[string]any {
	// Convert SQL expressions to document query filters
	// This is a simplified implementation
	result := make(map[string]any)

	for _, filter := range filters {
		if binExpr, ok := filter.(*BinaryExpression); ok {
			if idExpr, ok := binExpr.Left.(*IdentifierExpression); ok {
				if litExpr, ok := binExpr.Right.(*LiteralExpression); ok {
					result[idExpr.Name] = litExpr.Value
				}
			}
		}
	}

	return result
}

func (adapter *DocumentStorageAdapter) matchesFilters(row UnifiedRow, filters []Expression) bool {
	// Simplified filter matching for document storage
	return true
}

func (adapter *ColumnarStorageAdapter) matchesColumnarFilters(columnData map[string]ColumnData, rowIndex int, filters []Expression) bool {
	// Simplified filter matching for columnar storage
	// In a real implementation, this would evaluate filters against column values
	for _, filter := range filters {
		if binExpr, ok := filter.(*BinaryExpression); ok {
			if idExpr, ok := binExpr.Left.(*IdentifierExpression); ok {
				if data, exists := columnData[idExpr.Name]; exists {
					if rowIndex < len(data.Values) {
						// Simple equality check for demonstration
						if litExpr, ok := binExpr.Right.(*LiteralExpression); ok {
							if binExpr.Operator == OpEqual {
								if data.Values[rowIndex] != litExpr.Value {
									return false
								}
							}
						}
					}
				}
			}
		}
	}
	return true
}

func (adapter *ColumnarStorageAdapter) getTableSchema(ctx context.Context, tableName string) (*TableSchema, error) {
	// This would normally come from metadata store
	// For now, return a simple schema
	return &TableSchema{
		Columns: []ColumnSchema{
			{Name: "id", Type: DataType{Name: "INTEGER"}},
			{Name: "name", Type: DataType{Name: "TEXT"}},
			{Name: "value", Type: DataType{Name: "TEXT"}},
			{Name: "created_at", Type: DataType{Name: "TIMESTAMP"}},
		},
	}, nil
}

func getDocumentID(doc Document) string {
	if id, exists := doc["_id"]; exists {
		return fmt.Sprintf("%v", id)
	}
	if id, exists := doc["id"]; exists {
		return fmt.Sprintf("%v", id)
	}
	return "unknown"
}

// Helper methods for execution optimization

// shouldUseVectorization determines if vectorization should be used for a plan
func (qe *QueryExecutor) shouldUseVectorization(plan *QueryPlan) bool {
	if !qe.config.EnableVectorization {
		return false
	}

	// Use vectorization for columnar storage with analytical queries
	storageType := qe.getStorageType(plan.TableName)
	if storageType != StorageTypeColumnar {
		return false
	}

	// Use vectorization for large scans or aggregations
	return plan.PlanRows > 10000 || qe.hasAggregation(plan)
}

// shouldUseParallelJoin determines if parallel join should be used
func (qe *QueryExecutor) shouldUseParallelJoin(plan *QueryPlan) bool {
	if !qe.config.EnableParallel {
		return false
	}

	// Use parallel join for large datasets
	leftRows := float64(1000)  // Default estimate
	rightRows := float64(1000) // Default estimate

	if plan.LeftTree != nil {
		leftRows = plan.LeftTree.PlanRows
	}
	if plan.RightTree != nil {
		rightRows = plan.RightTree.PlanRows
	}

	// Use parallel join if either side has more than 10k rows
	return leftRows > 10000 || rightRows > 10000
}

// hasAggregation checks if a plan contains aggregation operations
func (qe *QueryExecutor) hasAggregation(plan *QueryPlan) bool {
	if plan == nil {
		return false
	}

	// Check current plan node
	if plan.Type == PlanTypeAggregate || plan.Type == PlanTypeGroup {
		return true
	}

	// Check child nodes
	return qe.hasAggregation(plan.LeftTree) || qe.hasAggregation(plan.RightTree)
}

// Enhanced aggregate execution with vectorization support
func (qe *QueryExecutor) executeAggregateEnhanced(ctx *ExecutionContext, plan *QueryPlan) (*ResultSet, error) {
	// Execute child plan
	childResult, err := qe.executePlan(ctx, plan.LeftTree)
	if err != nil {
		return nil, fmt.Errorf("failed to execute child plan for aggregation: %w", err)
	}

	// Check if we can use vectorized aggregation
	if qe.config.EnableVectorization && len(childResult.Rows) > 1000 {
		return qe.executeVectorizedAggregate(ctx, plan, childResult)
	}

	// Fall back to regular aggregation
	return qe.executeRegularAggregate(ctx, plan, childResult)
}

// executeVectorizedAggregate performs vectorized aggregation
func (qe *QueryExecutor) executeVectorizedAggregate(ctx *ExecutionContext, plan *QueryPlan, input *ResultSet) (*ResultSet, error) {
	// Group rows by group keys
	groups := make(map[string][]Row)

	for _, row := range input.Rows {
		// Create group key (simplified - uses first column)
		groupKey := fmt.Sprintf("%v", row.Values[0])
		groups[groupKey] = append(groups[groupKey], row)
	}

	// Process aggregates for each group in parallel
	if qe.config.EnableParallel && len(groups) > 100 {
		return qe.executeParallelAggregate(ctx, plan, groups)
	}

	// Sequential aggregation
	var resultRows []Row
	for _, groupRows := range groups {
		aggregatedRow := qe.computeAggregates(groupRows, plan.GroupKeys)
		resultRows = append(resultRows, aggregatedRow)
	}

	ctx.Stats.RowsProcessed += int64(len(resultRows))

	return &ResultSet{
		Columns: qe.getAggregateColumns(plan),
		Rows:    resultRows,
		Stats:   *ctx.Stats,
	}, nil
}

// executeParallelAggregate performs parallel aggregation
func (qe *QueryExecutor) executeParallelAggregate(ctx *ExecutionContext, plan *QueryPlan, groups map[string][]Row) (*ResultSet, error) {
	workers := qe.config.MaxWorkers
	if workers > len(groups) {
		workers = len(groups)
	}

	// Distribute groups among workers
	groupKeys := make([]string, 0, len(groups))
	for key := range groups {
		groupKeys = append(groupKeys, key)
	}

	chunkSize := len(groupKeys) / workers
	if chunkSize == 0 {
		chunkSize = 1
	}

	type aggregateResult struct {
		rows []Row
		err  error
	}

	resultChan := make(chan aggregateResult, workers)

	// Launch workers
	for i := 0; i < workers; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if i == workers-1 {
			end = len(groupKeys)
		}

		go func(keys []string) {
			var workerRows []Row
			for _, key := range keys {
				if groupRows, exists := groups[key]; exists {
					aggregatedRow := qe.computeAggregates(groupRows, plan.GroupKeys)
					workerRows = append(workerRows, aggregatedRow)
				}
			}
			resultChan <- aggregateResult{rows: workerRows}
		}(groupKeys[start:end])
	}

	// Collect results
	var allRows []Row
	for i := 0; i < workers; i++ {
		result := <-resultChan
		if result.err == nil {
			allRows = append(allRows, result.rows...)
		}
	}

	ctx.Stats.RowsProcessed += int64(len(allRows))

	return &ResultSet{
		Columns: qe.getAggregateColumns(plan),
		Rows:    allRows,
		Stats:   *ctx.Stats,
	}, nil
}

// executeRegularAggregate performs standard aggregation
func (qe *QueryExecutor) executeRegularAggregate(ctx *ExecutionContext, plan *QueryPlan, input *ResultSet) (*ResultSet, error) {
	// Simple aggregation implementation
	var resultRows []Row

	if len(input.Rows) > 0 {
		// For simplicity, just return the first row with count
		firstRow := input.Rows[0]
		aggregatedRow := Row{
			Values: append(firstRow.Values, len(input.Rows)),
		}
		resultRows = append(resultRows, aggregatedRow)
	}

	ctx.Stats.RowsProcessed += int64(len(resultRows))

	return &ResultSet{
		Columns: qe.getAggregateColumns(plan),
		Rows:    resultRows,
		Stats:   *ctx.Stats,
	}, nil
}

// computeAggregates computes aggregate functions for a group of rows
func (qe *QueryExecutor) computeAggregates(rows []Row, groupKeys []Expression) Row {
	// Simplified aggregate computation
	// In a real implementation, this would handle SUM, AVG, COUNT, etc.

	if len(rows) == 0 {
		return Row{Values: []any{0}}
	}

	// Return first row with count as additional column
	firstRow := rows[0]
	result := Row{
		Values: append(firstRow.Values, len(rows)),
	}

	return result
}

// getAggregateColumns returns column information for aggregate results
func (qe *QueryExecutor) getAggregateColumns(plan *QueryPlan) []ColumnInfo {
	// This would normally be derived from the target list
	// For now, return basic columns
	baseColumns := qe.getTableColumns(plan.TableName)

	// Add aggregate columns
	aggregateColumns := []ColumnInfo{
		{Name: "count", Type: DataType{Name: "INTEGER"}, Nullable: false},
	}

	return append(baseColumns, aggregateColumns...)
}
