package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// Column represents a column in a columnar table
type Column struct {
	Name         string      `json:"name"`
	DataType     DataType    `json:"data_type"`
	Nullable     bool        `json:"nullable"`
	DefaultValue interface{} `json:"default_value,omitempty"`
	Indexed      bool        `json:"indexed"`
	Compressed   bool        `json:"compressed"`
	Encoding     Encoding    `json:"encoding"`
}

// DataType represents the data type of a column
type DataType string

const (
	DataTypeInt32    DataType = "int32"
	DataTypeInt64    DataType = "int64"
	DataTypeFloat32  DataType = "float32"
	DataTypeFloat64  DataType = "float64"
	DataTypeString   DataType = "string"
	DataTypeBool     DataType = "bool"
	DataTypeBytes    DataType = "bytes"
	DataTypeDate     DataType = "date"
	DataTypeDateTime DataType = "datetime"
	DataTypeDecimal  DataType = "decimal"
	DataTypeJSON     DataType = "json"
)

// Encoding represents the encoding type for column data
type Encoding string

const (
	EncodingPlain      Encoding = "plain"
	EncodingDictionary Encoding = "dictionary"
	EncodingRLE        Encoding = "rle"       // Run Length Encoding
	EncodingDelta      Encoding = "delta"     // Delta encoding
	EncodingBitPacked  Encoding = "bitpacked" // Bit packing
)

// Table represents a columnar table
type Table struct {
	Name       string           `json:"name"`
	Columns    []*Column        `json:"columns"`
	RowCount   int64            `json:"row_count"`
	Partitions []*Partition     `json:"partitions"`
	Indexes    []*ColumnarIndex `json:"indexes"`
	Metadata   TableMetadata    `json:"metadata"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
	Version    int64            `json:"version"`
}

// TableMetadata holds metadata about the table
type TableMetadata struct {
	Description  string            `json:"description"`
	Owner        string            `json:"owner"`
	Properties   map[string]string `json:"properties"`
	Tags         []string          `json:"tags"`
	Compression  string            `json:"compression"`
	PartitionKey []string          `json:"partition_key"`
	SortKey      []string          `json:"sort_key"`
}

// Partition represents a partition of a table
type Partition struct {
	ID        string                 `json:"id"`
	Table     string                 `json:"table"`
	Values    map[string]interface{} `json:"values"`
	RowCount  int64                  `json:"row_count"`
	Size      int64                  `json:"size"`
	MinValues map[string]interface{} `json:"min_values"`
	MaxValues map[string]interface{} `json:"max_values"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// ColumnarIndex represents an index on columnar data
type ColumnarIndex struct {
	Name      string    `json:"name"`
	Table     string    `json:"table"`
	Columns   []string  `json:"columns"`
	Type      IndexType `json:"type"`
	Unique    bool      `json:"unique"`
	Clustered bool      `json:"clustered"`
	CreatedAt time.Time `json:"created_at"`
}

// IndexType represents the type of index
type IndexType string

const (
	IndexTypeBTree    IndexType = "btree"
	IndexTypeHash     IndexType = "hash"
	IndexTypeBitmap   IndexType = "bitmap"
	IndexTypeBloom    IndexType = "bloom"
	IndexTypeInverted IndexType = "inverted"
)

// Row represents a row of data in columnar format
type Row struct {
	Values    map[string]interface{} `json:"values"`
	RowID     int64                  `json:"row_id"`
	Partition string                 `json:"partition"`
	Version   int64                  `json:"version"`
}

// ColumnarQuery represents a query against columnar data
type ColumnarQuery struct {
	Table      string       `json:"table"`
	Columns    []string     `json:"columns"`
	Filters    []*Filter    `json:"filters"`
	GroupBy    []string     `json:"group_by"`
	OrderBy    []*OrderBy   `json:"order_by"`
	Aggregates []*Aggregate `json:"aggregates"`
	Limit      int          `json:"limit"`
	Offset     int          `json:"offset"`
	Partitions []string     `json:"partitions"`
}

// Filter represents a filter condition
type Filter struct {
	Column   string        `json:"column"`
	Operator FilterOp      `json:"operator"`
	Value    interface{}   `json:"value"`
	Values   []interface{} `json:"values,omitempty"` // For IN operator
}

// FilterOp represents a filter operator
type FilterOp string

const (
	FilterOpEQ      FilterOp = "eq"       // Equal
	FilterOpNE      FilterOp = "ne"       // Not equal
	FilterOpLT      FilterOp = "lt"       // Less than
	FilterOpLE      FilterOp = "le"       // Less than or equal
	FilterOpGT      FilterOp = "gt"       // Greater than
	FilterOpGE      FilterOp = "ge"       // Greater than or equal
	FilterOpIN      FilterOp = "in"       // In list
	FilterOpNotIN   FilterOp = "not_in"   // Not in list
	FilterOpLike    FilterOp = "like"     // Pattern matching
	FilterOpIsNull  FilterOp = "is_null"  // Is null
	FilterOpNotNull FilterOp = "not_null" // Is not null
)

// OrderBy represents an order by clause
type OrderBy struct {
	Column string `json:"column"`
	Desc   bool   `json:"desc"`
}

// Aggregate represents an aggregation function
type Aggregate struct {
	Function AggregateFunc `json:"function"`
	Column   string        `json:"column"`
	Alias    string        `json:"alias"`
}

// AggregateFunc represents an aggregation function
type AggregateFunc string

const (
	AggFuncCount    AggregateFunc = "count"
	AggFuncSum      AggregateFunc = "sum"
	AggFuncAvg      AggregateFunc = "avg"
	AggFuncMin      AggregateFunc = "min"
	AggFuncMax      AggregateFunc = "max"
	AggFuncStdDev   AggregateFunc = "stddev"
	AggFuncVariance AggregateFunc = "variance"
)

// ColumnarResult represents the result of a columnar query
type ColumnarResult struct {
	Columns     []string                 `json:"columns"`
	Rows        []map[string]interface{} `json:"rows"`
	TotalRows   int64                    `json:"total_rows"`
	ScannedRows int64                    `json:"scanned_rows"`
	HasMore     bool                     `json:"has_more"`
	NextOffset  int                      `json:"next_offset"`
	Metadata    QueryMetadata            `json:"metadata"`
}

// QueryMetadata holds metadata about query execution
type QueryMetadata struct {
	ExecutionTime  int64    `json:"execution_time_ms"`
	PartitionsRead []string `json:"partitions_read"`
	IndexesUsed    []string `json:"indexes_used"`
	BytesScanned   int64    `json:"bytes_scanned"`
	BytesReturned  int64    `json:"bytes_returned"`
}

// NewTable creates a new columnar table
func NewTable(name string, columns []*Column) *Table {
	now := time.Now()
	return &Table{
		Name:       name,
		Columns:    columns,
		RowCount:   0,
		Partitions: make([]*Partition, 0),
		Indexes:    make([]*ColumnarIndex, 0),
		Metadata: TableMetadata{
			Properties: make(map[string]string),
			Tags:       make([]string, 0),
		},
		CreatedAt: now,
		UpdatedAt: now,
		Version:   1,
	}
}

// NewColumn creates a new column definition
func NewColumn(name string, dataType DataType) *Column {
	return &Column{
		Name:       name,
		DataType:   dataType,
		Nullable:   true,
		Indexed:    false,
		Compressed: false,
		Encoding:   EncodingPlain,
	}
}

// AddColumn adds a column to the table
func (t *Table) AddColumn(column *Column) error {
	// Check if column already exists
	for _, existingCol := range t.Columns {
		if existingCol.Name == column.Name {
			return fmt.Errorf("column %s already exists", column.Name)
		}
	}

	t.Columns = append(t.Columns, column)
	t.UpdatedAt = time.Now()
	t.Version++

	return nil
}

// RemoveColumn removes a column from the table
func (t *Table) RemoveColumn(columnName string) error {
	for i, col := range t.Columns {
		if col.Name == columnName {
			t.Columns = append(t.Columns[:i], t.Columns[i+1:]...)
			t.UpdatedAt = time.Now()
			t.Version++
			return nil
		}
	}
	return fmt.Errorf("column %s not found", columnName)
}

// GetColumn gets a column by name
func (t *Table) GetColumn(name string) (*Column, error) {
	for _, col := range t.Columns {
		if col.Name == name {
			return col, nil
		}
	}
	return nil, fmt.Errorf("column %s not found", name)
}

// HasColumn checks if a column exists
func (t *Table) HasColumn(name string) bool {
	_, err := t.GetColumn(name)
	return err == nil
}

// AddPartition adds a partition to the table
func (t *Table) AddPartition(partition *Partition) {
	t.Partitions = append(t.Partitions, partition)
	t.UpdatedAt = time.Now()
}

// GetPartition gets a partition by ID
func (t *Table) GetPartition(id string) (*Partition, error) {
	for _, partition := range t.Partitions {
		if partition.ID == id {
			return partition, nil
		}
	}
	return nil, fmt.Errorf("partition %s not found", id)
}

// AddIndex adds an index to the table
func (t *Table) AddIndex(index *ColumnarIndex) error {
	// Validate that all columns exist
	for _, colName := range index.Columns {
		if !t.HasColumn(colName) {
			return fmt.Errorf("column %s does not exist", colName)
		}
	}

	t.Indexes = append(t.Indexes, index)
	t.UpdatedAt = time.Now()

	return nil
}

// ValidateRow validates a row against the table schema
func (t *Table) ValidateRow(row *Row) error {
	for _, col := range t.Columns {
		value, exists := row.Values[col.Name]

		// Check for required columns
		if !col.Nullable && (!exists || value == nil) {
			return fmt.Errorf("column %s cannot be null", col.Name)
		}

		// Validate data types (simplified)
		if exists && value != nil {
			if err := t.validateDataType(col, value); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateDataType validates that a value matches the expected data type
func (t *Table) validateDataType(col *Column, value interface{}) error {
	switch col.DataType {
	case DataTypeInt32:
		if _, ok := value.(int32); !ok {
			if _, ok := value.(int); !ok {
				return fmt.Errorf("column %s expects int32, got %T", col.Name, value)
			}
		}
	case DataTypeInt64:
		if _, ok := value.(int64); !ok {
			if _, ok := value.(int); !ok {
				return fmt.Errorf("column %s expects int64, got %T", col.Name, value)
			}
		}
	case DataTypeString:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("column %s expects string, got %T", col.Name, value)
		}
	case DataTypeBool:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("column %s expects bool, got %T", col.Name, value)
		}
		// Add more type validations as needed
	}

	return nil
}

// ToJSON converts the table to JSON
func (t *Table) ToJSON() ([]byte, error) {
	return json.Marshal(t)
}

// TableFromJSON creates a table from JSON
func TableFromJSON(data []byte) (*Table, error) {
	var table Table
	err := json.Unmarshal(data, &table)
	if err != nil {
		return nil, err
	}
	return &table, nil
}

// ColumnarStore interface defines operations for columnar storage
type ColumnarStore interface {
	// Create a new table
	CreateTable(table *Table) error

	// Drop a table
	DropTable(name string) error

	// Get table metadata
	GetTable(name string) (*Table, error)

	// List all tables
	ListTables() ([]*Table, error)

	// Insert rows into a table
	Insert(tableName string, rows []*Row) error

	// Update rows in a table
	Update(tableName string, filters []*Filter, updates map[string]interface{}) (int64, error)

	// Delete rows from a table
	Delete(tableName string, filters []*Filter) (int64, error)

	// Query data from a table
	Query(query *ColumnarQuery) (*ColumnarResult, error)

	// Create an index
	CreateIndex(index *ColumnarIndex) error

	// Drop an index
	DropIndex(tableName, indexName string) error

	// Get table statistics
	GetTableStats(tableName string) (*TableStats, error)
}

// TableStats represents statistics about a table
type TableStats struct {
	TableName      string                 `json:"table_name"`
	RowCount       int64                  `json:"row_count"`
	Size           int64                  `json:"size"`
	PartitionCount int                    `json:"partition_count"`
	IndexCount     int                    `json:"index_count"`
	ColumnStats    map[string]*ColumnStat `json:"column_stats"`
	LastUpdated    time.Time              `json:"last_updated"`
}

// ColumnStat represents statistics about a column
type ColumnStat struct {
	ColumnName    string      `json:"column_name"`
	DataType      DataType    `json:"data_type"`
	NullCount     int64       `json:"null_count"`
	DistinctCount int64       `json:"distinct_count"`
	MinValue      interface{} `json:"min_value"`
	MaxValue      interface{} `json:"max_value"`
	AvgLength     float64     `json:"avg_length"`
	Compression   float64     `json:"compression_ratio"`
}

// ColumnarEvent represents an event that occurred on columnar data
type ColumnarEvent struct {
	Type         EventType   `json:"type"`
	Table        string      `json:"table"`
	Timestamp    time.Time   `json:"timestamp"`
	RowsAffected int64       `json:"rows_affected"`
	Data         interface{} `json:"data,omitempty"`
}

// ColumnarEventHandler handles columnar events
type ColumnarEventHandler interface {
	HandleEvent(event *ColumnarEvent) error
}
