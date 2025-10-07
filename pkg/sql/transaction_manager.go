package sql

import (
	"context"
	"fmt"
	"sync"
	"time"

	"mantisDB/transaction"
)

// SQLTransactionManager integrates SQL operations with the existing transaction system
type SQLTransactionManager struct {
	txnSystem        *transaction.TransactionSystem
	activeTxns       map[string]*SQLTransaction
	savepoints       map[string]map[string]*transaction.Transaction // txnID -> savepoint name -> transaction
	mutex            sync.RWMutex
	defaultIsolation transaction.IsolationLevel
}

// SQLTransaction represents a SQL transaction with additional SQL-specific features
type SQLTransaction struct {
	*transaction.Transaction
	sqlID        string
	readOnly     bool
	deferrable   bool
	savepoints   map[string]*transaction.Transaction
	statements   []Statement
	startTime    time.Time
	lastActivity time.Time
	mutex        sync.RWMutex
}

// SQLTransactionConfig holds configuration for SQL transactions
type SQLTransactionConfig struct {
	DefaultIsolation transaction.IsolationLevel
	StatementTimeout time.Duration
	IdleTimeout      time.Duration
	MaxSavepoints    int
}

// DefaultSQLTransactionConfig returns default configuration
func DefaultSQLTransactionConfig() *SQLTransactionConfig {
	return &SQLTransactionConfig{
		DefaultIsolation: transaction.ReadCommitted,
		StatementTimeout: 30 * time.Second,
		IdleTimeout:      10 * time.Minute,
		MaxSavepoints:    100,
	}
}

// NewSQLTransactionManager creates a new SQL transaction manager
func NewSQLTransactionManager(txnSystem *transaction.TransactionSystem, config *SQLTransactionConfig) *SQLTransactionManager {
	if config == nil {
		config = DefaultSQLTransactionConfig()
	}

	return &SQLTransactionManager{
		txnSystem:        txnSystem,
		activeTxns:       make(map[string]*SQLTransaction),
		savepoints:       make(map[string]map[string]*transaction.Transaction),
		defaultIsolation: config.DefaultIsolation,
	}
}

// BeginTransaction starts a new SQL transaction
func (stm *SQLTransactionManager) BeginTransaction(ctx context.Context, stmt *BeginTransactionStatement) (*SQLTransaction, error) {
	stm.mutex.Lock()
	defer stm.mutex.Unlock()

	// Determine isolation level
	isolation := stm.defaultIsolation
	if stmt.IsolationLevel != nil {
		isolation = transaction.IsolationLevel(stmt.IsolationLevel.ToTransactionIsolationLevel())
	}

	// Begin transaction in the underlying system
	txn, err := stm.txnSystem.BeginTransaction(isolation)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Create SQL transaction wrapper
	sqlTxn := &SQLTransaction{
		Transaction:  txn,
		sqlID:        fmt.Sprintf("sql_%d", txn.ID),
		readOnly:     stmt.ReadOnly,
		deferrable:   stmt.Deferrable,
		savepoints:   make(map[string]*transaction.Transaction),
		statements:   make([]Statement, 0),
		startTime:    time.Now(),
		lastActivity: time.Now(),
	}

	// Store in active transactions
	stm.activeTxns[sqlTxn.sqlID] = sqlTxn
	stm.savepoints[sqlTxn.sqlID] = make(map[string]*transaction.Transaction)

	return sqlTxn, nil
}

// CommitTransaction commits a SQL transaction
func (stm *SQLTransactionManager) CommitTransaction(ctx context.Context, sqlTxnID string, stmt *CommitTransactionStatement) error {
	stm.mutex.Lock()
	defer stm.mutex.Unlock()

	sqlTxn, exists := stm.activeTxns[sqlTxnID]
	if !exists {
		return fmt.Errorf("transaction %s not found", sqlTxnID)
	}

	// Commit the underlying transaction
	if err := stm.txnSystem.CommitTransaction(sqlTxn.Transaction); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Clean up
	delete(stm.activeTxns, sqlTxnID)
	delete(stm.savepoints, sqlTxnID)

	// Handle chaining if requested
	if stmt.Chain {
		// Start a new transaction with the same characteristics
		newStmt := &BeginTransactionStatement{
			ReadOnly:   sqlTxn.readOnly,
			Deferrable: sqlTxn.deferrable,
		}
		isolation := SQLIsolationLevel(sqlTxn.Transaction.Isolation)
		newStmt.IsolationLevel = &isolation

		_, err := stm.BeginTransaction(ctx, newStmt)
		return err
	}

	return nil
}

// RollbackTransaction rolls back a SQL transaction
func (stm *SQLTransactionManager) RollbackTransaction(ctx context.Context, sqlTxnID string, stmt *RollbackTransactionStatement) error {
	stm.mutex.Lock()
	defer stm.mutex.Unlock()

	sqlTxn, exists := stm.activeTxns[sqlTxnID]
	if !exists {
		return fmt.Errorf("transaction %s not found", sqlTxnID)
	}

	// Handle rollback to savepoint
	if stmt.Savepoint != "" {
		return stm.rollbackToSavepoint(ctx, sqlTxn, stmt.Savepoint)
	}

	// Rollback the entire transaction
	if err := stm.txnSystem.AbortTransaction(sqlTxn.Transaction); err != nil {
		return fmt.Errorf("failed to rollback transaction: %w", err)
	}

	// Clean up
	delete(stm.activeTxns, sqlTxnID)
	delete(stm.savepoints, sqlTxnID)

	// Handle chaining if requested
	if stmt.Chain {
		// Start a new transaction with the same characteristics
		newStmt := &BeginTransactionStatement{
			ReadOnly:   sqlTxn.readOnly,
			Deferrable: sqlTxn.deferrable,
		}
		isolation := SQLIsolationLevel(sqlTxn.Transaction.Isolation)
		newStmt.IsolationLevel = &isolation

		_, err := stm.BeginTransaction(ctx, newStmt)
		return err
	}

	return nil
}

// CreateSavepoint creates a savepoint within a transaction
func (stm *SQLTransactionManager) CreateSavepoint(ctx context.Context, sqlTxnID string, stmt *SavepointStatement) error {
	stm.mutex.Lock()
	defer stm.mutex.Unlock()

	sqlTxn, exists := stm.activeTxns[sqlTxnID]
	if !exists {
		return fmt.Errorf("transaction %s not found", sqlTxnID)
	}

	sqlTxn.mutex.Lock()
	defer sqlTxn.mutex.Unlock()

	// Check if savepoint already exists
	if _, exists := sqlTxn.savepoints[stmt.Name]; exists {
		return fmt.Errorf("savepoint %s already exists", stmt.Name)
	}

	// Create a nested transaction as a savepoint
	// In a real implementation, this would create a proper savepoint
	// For now, we'll store the current transaction state
	savepointTxn, err := stm.txnSystem.BeginTransaction(sqlTxn.Transaction.Isolation)
	if err != nil {
		return fmt.Errorf("failed to create savepoint: %w", err)
	}

	sqlTxn.savepoints[stmt.Name] = savepointTxn
	stm.savepoints[sqlTxnID][stmt.Name] = savepointTxn

	return nil
}

// ReleaseSavepoint releases a savepoint
func (stm *SQLTransactionManager) ReleaseSavepoint(ctx context.Context, sqlTxnID string, stmt *ReleaseSavepointStatement) error {
	stm.mutex.Lock()
	defer stm.mutex.Unlock()

	sqlTxn, exists := stm.activeTxns[sqlTxnID]
	if !exists {
		return fmt.Errorf("transaction %s not found", sqlTxnID)
	}

	sqlTxn.mutex.Lock()
	defer sqlTxn.mutex.Unlock()

	// Check if savepoint exists
	savepointTxn, exists := sqlTxn.savepoints[stmt.Name]
	if !exists {
		return fmt.Errorf("savepoint %s not found", stmt.Name)
	}

	// Commit the savepoint transaction
	if err := stm.txnSystem.CommitTransaction(savepointTxn); err != nil {
		return fmt.Errorf("failed to release savepoint: %w", err)
	}

	// Remove from maps
	delete(sqlTxn.savepoints, stmt.Name)
	delete(stm.savepoints[sqlTxnID], stmt.Name)

	return nil
}

// rollbackToSavepoint rolls back to a specific savepoint
func (stm *SQLTransactionManager) rollbackToSavepoint(ctx context.Context, sqlTxn *SQLTransaction, savepointName string) error {
	sqlTxn.mutex.Lock()
	defer sqlTxn.mutex.Unlock()

	// Check if savepoint exists
	_, exists := sqlTxn.savepoints[savepointName]
	if !exists {
		return fmt.Errorf("savepoint %s not found", savepointName)
	}

	// Rollback to the savepoint
	// In a real implementation, this would restore the transaction state to the savepoint
	// For now, we'll abort all transactions created after the savepoint
	for name, txn := range sqlTxn.savepoints {
		if name != savepointName {
			if err := stm.txnSystem.AbortTransaction(txn); err != nil {
				// Log error but continue
				fmt.Printf("Warning: failed to abort savepoint transaction %s: %v\n", name, err)
			}
			delete(sqlTxn.savepoints, name)
			delete(stm.savepoints[sqlTxn.sqlID], name)
		}
	}

	return nil
}

// GetTransaction retrieves a SQL transaction by ID
func (stm *SQLTransactionManager) GetTransaction(sqlTxnID string) (*SQLTransaction, error) {
	stm.mutex.RLock()
	defer stm.mutex.RUnlock()

	sqlTxn, exists := stm.activeTxns[sqlTxnID]
	if !exists {
		return nil, fmt.Errorf("transaction %s not found", sqlTxnID)
	}

	return sqlTxn, nil
}

// ExecuteInTransaction executes a statement within a transaction context
func (stm *SQLTransactionManager) ExecuteInTransaction(ctx context.Context, sqlTxnID string, stmt Statement, executor *QueryExecutor) (*ResultSet, error) {
	sqlTxn, err := stm.GetTransaction(sqlTxnID)
	if err != nil {
		return nil, err
	}

	// Update last activity
	sqlTxn.mutex.Lock()
	sqlTxn.lastActivity = time.Now()
	sqlTxn.statements = append(sqlTxn.statements, stmt)
	sqlTxn.mutex.Unlock()

	// Create execution context with transaction
	execCtx := &ExecutionContext{
		Context:        ctx,
		SQLTransaction: sqlTxn,
		IsolationLevel: IsolationLevel(sqlTxn.Transaction.Isolation),
		ReadOnly:       sqlTxn.readOnly,
		StartTime:      time.Now(),
	}

	// Execute based on statement type
	switch s := stmt.(type) {
	case *SelectStatement:
		return stm.executeSelect(execCtx, s, executor)
	case *InsertStatement:
		return stm.executeInsert(execCtx, s, executor)
	case *UpdateStatement:
		return stm.executeUpdate(execCtx, s, executor)
	case *DeleteStatement:
		return stm.executeDelete(execCtx, s, executor)
	default:
		return nil, fmt.Errorf("unsupported statement type in transaction: %T", stmt)
	}
}

// executeSelect executes a SELECT statement within a transaction
func (stm *SQLTransactionManager) executeSelect(ctx *ExecutionContext, stmt *SelectStatement, executor *QueryExecutor) (*ResultSet, error) {
	// Ensure proper isolation level handling
	if err := stm.enforceIsolationLevel(ctx, stmt); err != nil {
		return nil, err
	}

	// Create query plan (simplified - would use optimizer in real implementation)
	plan := &QueryPlan{
		Type:      PlanTypeSeqScan,
		TableName: stm.extractTableName(stmt),
		Qual:      stm.convertWhereClause(stmt.Where),
	}

	// Execute with transaction context
	return executor.Execute(ctx.Context, plan, nil)
}

// executeInsert executes an INSERT statement within a transaction
func (stm *SQLTransactionManager) executeInsert(ctx *ExecutionContext, stmt *InsertStatement, executor *QueryExecutor) (*ResultSet, error) {
	if ctx.ReadOnly {
		return nil, fmt.Errorf("cannot execute INSERT in read-only transaction")
	}

	// Acquire exclusive locks on the target table
	tableName := stmt.Table.Name
	if err := stm.acquireTableLock(ctx, tableName, transaction.ExclusiveLock); err != nil {
		return nil, fmt.Errorf("failed to acquire table lock: %w", err)
	}

	// Execute insert operation
	// This would integrate with the storage engines
	return &ResultSet{
		Columns: []ColumnInfo{{Name: "rows_affected", Type: DataType{Name: "INTEGER"}}},
		Rows:    []Row{{Values: []interface{}{1}}},
	}, nil
}

// executeUpdate executes an UPDATE statement within a transaction
func (stm *SQLTransactionManager) executeUpdate(ctx *ExecutionContext, stmt *UpdateStatement, executor *QueryExecutor) (*ResultSet, error) {
	if ctx.ReadOnly {
		return nil, fmt.Errorf("cannot execute UPDATE in read-only transaction")
	}

	// Acquire exclusive locks on the target table
	tableName := stmt.Table.Name
	if err := stm.acquireTableLock(ctx, tableName, transaction.ExclusiveLock); err != nil {
		return nil, fmt.Errorf("failed to acquire table lock: %w", err)
	}

	// Execute update operation
	// This would integrate with the storage engines
	return &ResultSet{
		Columns: []ColumnInfo{{Name: "rows_affected", Type: DataType{Name: "INTEGER"}}},
		Rows:    []Row{{Values: []interface{}{1}}},
	}, nil
}

// executeDelete executes a DELETE statement within a transaction
func (stm *SQLTransactionManager) executeDelete(ctx *ExecutionContext, stmt *DeleteStatement, executor *QueryExecutor) (*ResultSet, error) {
	if ctx.ReadOnly {
		return nil, fmt.Errorf("cannot execute DELETE in read-only transaction")
	}

	// Acquire exclusive locks on the target table
	tableName := stmt.From.Name
	if err := stm.acquireTableLock(ctx, tableName, transaction.ExclusiveLock); err != nil {
		return nil, fmt.Errorf("failed to acquire table lock: %w", err)
	}

	// Execute delete operation
	// This would integrate with the storage engines
	return &ResultSet{
		Columns: []ColumnInfo{{Name: "rows_affected", Type: DataType{Name: "INTEGER"}}},
		Rows:    []Row{{Values: []interface{}{1}}},
	}, nil
}

// enforceIsolationLevel ensures proper isolation level semantics
func (stm *SQLTransactionManager) enforceIsolationLevel(ctx *ExecutionContext, stmt *SelectStatement) error {
	switch ctx.IsolationLevel {
	case ReadUncommitted:
		// No additional locking needed
		return nil
	case ReadCommitted:
		// Acquire shared locks that are released after each statement
		return stm.acquireReadLocks(ctx, stmt)
	case RepeatableRead:
		// Acquire shared locks that are held until transaction end
		return stm.acquireReadLocks(ctx, stmt)
	case Serializable:
		// Acquire range locks to prevent phantom reads
		return stm.acquireSerializableLocks(ctx, stmt)
	default:
		return fmt.Errorf("unsupported isolation level: %v", ctx.IsolationLevel)
	}
}

// acquireTableLock acquires a lock on a table
func (stm *SQLTransactionManager) acquireTableLock(ctx *ExecutionContext, tableName string, lockType transaction.LockType) error {
	if ctx.SQLTransaction == nil {
		return fmt.Errorf("no SQL transaction in context")
	}

	// Use the transaction system to acquire the lock
	// Note: This would need to be implemented in the transaction system
	// For now, we'll return nil as a placeholder
	return nil
}

// acquireReadLocks acquires shared locks for read operations
func (stm *SQLTransactionManager) acquireReadLocks(ctx *ExecutionContext, stmt *SelectStatement) error {
	// Extract table names from the statement
	tableNames := stm.extractTableNames(stmt)

	for _, tableName := range tableNames {
		if err := stm.acquireTableLock(ctx, tableName, transaction.SharedLock); err != nil {
			return err
		}
	}

	return nil
}

// acquireSerializableLocks acquires locks for serializable isolation
func (stm *SQLTransactionManager) acquireSerializableLocks(ctx *ExecutionContext, stmt *SelectStatement) error {
	// For serializable isolation, we need to prevent phantom reads
	// This would involve range locking in a real implementation
	return stm.acquireReadLocks(ctx, stmt)
}

// Helper methods for extracting information from statements

func (stm *SQLTransactionManager) extractTableName(stmt *SelectStatement) string {
	if len(stmt.From) > 0 {
		return stmt.From[0].Name
	}
	return ""
}

func (stm *SQLTransactionManager) extractTableNames(stmt *SelectStatement) []string {
	var names []string
	for _, table := range stmt.From {
		if table.Name != "" {
			names = append(names, table.Name)
		}
	}
	return names
}

func (stm *SQLTransactionManager) convertWhereClause(where Expression) []Expression {
	if where == nil {
		return nil
	}
	return []Expression{where}
}

// GetActiveTransactions returns all active SQL transactions
func (stm *SQLTransactionManager) GetActiveTransactions() []*SQLTransaction {
	stm.mutex.RLock()
	defer stm.mutex.RUnlock()

	transactions := make([]*SQLTransaction, 0, len(stm.activeTxns))
	for _, txn := range stm.activeTxns {
		transactions = append(transactions, txn)
	}

	return transactions
}

// CleanupIdleTransactions cleans up idle transactions
func (stm *SQLTransactionManager) CleanupIdleTransactions(idleTimeout time.Duration) error {
	stm.mutex.Lock()
	defer stm.mutex.Unlock()

	now := time.Now()
	var toCleanup []string

	for id, sqlTxn := range stm.activeTxns {
		sqlTxn.mutex.RLock()
		idle := now.Sub(sqlTxn.lastActivity) > idleTimeout
		sqlTxn.mutex.RUnlock()

		if idle {
			toCleanup = append(toCleanup, id)
		}
	}

	// Cleanup idle transactions
	for _, id := range toCleanup {
		sqlTxn := stm.activeTxns[id]
		if err := stm.txnSystem.AbortTransaction(sqlTxn.Transaction); err != nil {
			fmt.Printf("Warning: failed to cleanup idle transaction %s: %v\n", id, err)
		}
		delete(stm.activeTxns, id)
		delete(stm.savepoints, id)
	}

	return nil
}

// Close closes the SQL transaction manager
func (stm *SQLTransactionManager) Close() error {
	stm.mutex.Lock()
	defer stm.mutex.Unlock()

	// Abort all active transactions
	for id, sqlTxn := range stm.activeTxns {
		if err := stm.txnSystem.AbortTransaction(sqlTxn.Transaction); err != nil {
			fmt.Printf("Warning: failed to abort transaction %s during shutdown: %v\n", id, err)
		}
	}

	// Clear all maps
	stm.activeTxns = make(map[string]*SQLTransaction)
	stm.savepoints = make(map[string]map[string]*transaction.Transaction)

	return nil
}

// SQL Transaction interface methods

func (st *SQLTransaction) GetID() string {
	return st.sqlID
}

func (st *SQLTransaction) IsReadOnly() bool {
	return st.readOnly
}

func (st *SQLTransaction) IsDeferrable() bool {
	return st.deferrable
}

func (st *SQLTransaction) GetSavepoints() []string {
	st.mutex.RLock()
	defer st.mutex.RUnlock()

	names := make([]string, 0, len(st.savepoints))
	for name := range st.savepoints {
		names = append(names, name)
	}
	return names
}

func (st *SQLTransaction) GetStatements() []Statement {
	st.mutex.RLock()
	defer st.mutex.RUnlock()

	statements := make([]Statement, len(st.statements))
	copy(statements, st.statements)
	return statements
}

func (st *SQLTransaction) GetStartTime() time.Time {
	return st.startTime
}

func (st *SQLTransaction) GetLastActivity() time.Time {
	st.mutex.RLock()
	defer st.mutex.RUnlock()
	return st.lastActivity
}
