package sql

import (
	"context"
	"fmt"
	"sync"
	"time"

	"mantisDB/transaction"
)

// SQLEngine is the main SQL processing engine that integrates all components
type SQLEngine struct {
	parser             *Parser
	optimizer          *QueryOptimizer
	executor           *QueryExecutor
	txnManager         *SQLTransactionManager
	distTxnCoordinator *DistributedTransactionCoordinator
	storageManager     *StorageManager
	config             *SQLEngineConfig
	activeConnections  map[string]*SQLConnection
	connectionMutex    sync.RWMutex
	shutdownChan       chan struct{}
	wg                 sync.WaitGroup
}

// SQLEngineConfig holds configuration for the SQL engine
type SQLEngineConfig struct {
	MaxConnections         int
	ConnectionTimeout      time.Duration
	StatementTimeout       time.Duration
	TransactionTimeout     time.Duration
	IdleTransactionTimeout time.Duration
	EnableDistributedTxns  bool
	DefaultIsolationLevel  transaction.IsolationLevel
}

// SQLConnection represents a SQL connection with transaction context
type SQLConnection struct {
	ID             string
	UserID         string
	Database       string
	CurrentTxn     *SQLTransaction
	CurrentDistTxn *DistributedTransaction
	CreatedAt      time.Time
	LastActivity   time.Time
	AutoCommit     bool
	IsolationLevel transaction.IsolationLevel
	ReadOnly       bool
	mutex          sync.RWMutex
}

// SQLResult represents the result of SQL execution
type SQLResult struct {
	ResultSet     *ResultSet
	RowsAffected  int64
	LastInsertID  int64
	ExecutionTime time.Duration
	TransactionID string
	Warnings      []string
	Error         error
}

// DefaultSQLEngineConfig returns default configuration
func DefaultSQLEngineConfig() *SQLEngineConfig {
	return &SQLEngineConfig{
		MaxConnections:         1000,
		ConnectionTimeout:      30 * time.Second,
		StatementTimeout:       30 * time.Second,
		TransactionTimeout:     10 * time.Minute,
		IdleTransactionTimeout: 5 * time.Minute,
		EnableDistributedTxns:  true,
		DefaultIsolationLevel:  transaction.ReadCommitted,
	}
}

// NewSQLEngine creates a new SQL engine
func NewSQLEngine(storageManager *StorageManager, txnSystem *transaction.TransactionSystem, config *SQLEngineConfig) *SQLEngine {
	if config == nil {
		config = DefaultSQLEngineConfig()
	}

	// Create SQL transaction manager
	sqlTxnConfig := &SQLTransactionConfig{
		DefaultIsolation: config.DefaultIsolationLevel,
		StatementTimeout: config.StatementTimeout,
		IdleTimeout:      config.IdleTransactionTimeout,
	}
	txnManager := NewSQLTransactionManager(txnSystem, sqlTxnConfig)

	// Create distributed transaction coordinator
	distTxnCoordinator := NewDistributedTransactionCoordinator(txnManager, "sql_engine_1")

	// Create query executor
	executor := NewQueryExecutor(storageManager)

	// Create query optimizer
	optimizer := NewQueryOptimizer()

	engine := &SQLEngine{
		optimizer:          optimizer,
		executor:           executor,
		txnManager:         txnManager,
		distTxnCoordinator: distTxnCoordinator,
		storageManager:     storageManager,
		config:             config,
		activeConnections:  make(map[string]*SQLConnection),
		shutdownChan:       make(chan struct{}),
	}

	// Register storage engine participants for distributed transactions
	if config.EnableDistributedTxns {
		engine.registerStorageParticipants()
	}

	// Start background tasks
	engine.startBackgroundTasks()

	return engine
}

// CreateConnection creates a new SQL connection
func (se *SQLEngine) CreateConnection(userID, database string) (*SQLConnection, error) {
	se.connectionMutex.Lock()
	defer se.connectionMutex.Unlock()

	if len(se.activeConnections) >= se.config.MaxConnections {
		return nil, fmt.Errorf("maximum number of connections reached")
	}

	conn := &SQLConnection{
		ID:             fmt.Sprintf("conn_%d_%s", time.Now().UnixNano(), userID),
		UserID:         userID,
		Database:       database,
		CreatedAt:      time.Now(),
		LastActivity:   time.Now(),
		AutoCommit:     true,
		IsolationLevel: se.config.DefaultIsolationLevel,
		ReadOnly:       false,
	}

	se.activeConnections[conn.ID] = conn
	return conn, nil
}

// CloseConnection closes a SQL connection
func (se *SQLEngine) CloseConnection(connectionID string) error {
	se.connectionMutex.Lock()
	defer se.connectionMutex.Unlock()

	conn, exists := se.activeConnections[connectionID]
	if !exists {
		return fmt.Errorf("connection %s not found", connectionID)
	}

	// Rollback any active transaction
	if conn.CurrentTxn != nil {
		stmt := &RollbackTransactionStatement{}
		se.txnManager.RollbackTransaction(context.Background(), conn.CurrentTxn.sqlID, stmt)
	}

	// Abort any active distributed transaction
	if conn.CurrentDistTxn != nil {
		se.distTxnCoordinator.AbortDistributedTransaction(context.Background(), conn.CurrentDistTxn.ID)
	}

	delete(se.activeConnections, connectionID)
	return nil
}

// ExecuteSQL executes a SQL statement
func (se *SQLEngine) ExecuteSQL(ctx context.Context, connectionID, sqlText string) (*SQLResult, error) {
	// Get connection
	se.connectionMutex.RLock()
	conn, exists := se.activeConnections[connectionID]
	se.connectionMutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("connection %s not found", connectionID)
	}

	// Update last activity
	conn.mutex.Lock()
	conn.LastActivity = time.Now()
	conn.mutex.Unlock()

	startTime := time.Now()

	// Parse SQL
	stmt, err := ParseSQL(sqlText)
	if err != nil {
		return &SQLResult{
			Error:         fmt.Errorf("parse error: %w", err),
			ExecutionTime: time.Since(startTime),
		}, err
	}

	// Execute based on statement type
	switch s := stmt.(type) {
	case *BeginTransactionStatement:
		return se.executeBeginTransaction(ctx, conn, s, startTime)
	case *CommitTransactionStatement:
		return se.executeCommitTransaction(ctx, conn, s, startTime)
	case *RollbackTransactionStatement:
		return se.executeRollbackTransaction(ctx, conn, s, startTime)
	case *SavepointStatement:
		return se.executeSavepoint(ctx, conn, s, startTime)
	case *ReleaseSavepointStatement:
		return se.executeReleaseSavepoint(ctx, conn, s, startTime)
	default:
		return se.executeDataStatement(ctx, conn, stmt, startTime)
	}
}

// executeBeginTransaction executes BEGIN TRANSACTION
func (se *SQLEngine) executeBeginTransaction(ctx context.Context, conn *SQLConnection, stmt *BeginTransactionStatement, startTime time.Time) (*SQLResult, error) {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()

	if conn.CurrentTxn != nil {
		return &SQLResult{
			Error:         fmt.Errorf("transaction already active"),
			ExecutionTime: time.Since(startTime),
		}, fmt.Errorf("transaction already active")
	}

	// Begin transaction
	txn, err := se.txnManager.BeginTransaction(ctx, stmt)
	if err != nil {
		return &SQLResult{
			Error:         err,
			ExecutionTime: time.Since(startTime),
		}, err
	}

	conn.CurrentTxn = txn
	conn.AutoCommit = false

	return &SQLResult{
		TransactionID: txn.sqlID,
		ExecutionTime: time.Since(startTime),
	}, nil
}

// executeCommitTransaction executes COMMIT TRANSACTION
func (se *SQLEngine) executeCommitTransaction(ctx context.Context, conn *SQLConnection, stmt *CommitTransactionStatement, startTime time.Time) (*SQLResult, error) {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()

	if conn.CurrentTxn == nil {
		return &SQLResult{
			Error:         fmt.Errorf("no active transaction"),
			ExecutionTime: time.Since(startTime),
		}, fmt.Errorf("no active transaction")
	}

	txnID := conn.CurrentTxn.sqlID

	// Check if this is a distributed transaction
	if conn.CurrentDistTxn != nil {
		err := se.distTxnCoordinator.CommitDistributedTransaction(ctx, conn.CurrentDistTxn.ID)
		conn.CurrentDistTxn = nil
		if err != nil {
			return &SQLResult{
				Error:         err,
				ExecutionTime: time.Since(startTime),
			}, err
		}
	} else {
		// Regular transaction commit
		err := se.txnManager.CommitTransaction(ctx, txnID, stmt)
		if err != nil {
			return &SQLResult{
				Error:         err,
				ExecutionTime: time.Since(startTime),
			}, err
		}
	}

	conn.CurrentTxn = nil
	conn.AutoCommit = true

	return &SQLResult{
		TransactionID: txnID,
		ExecutionTime: time.Since(startTime),
	}, nil
}

// executeRollbackTransaction executes ROLLBACK TRANSACTION
func (se *SQLEngine) executeRollbackTransaction(ctx context.Context, conn *SQLConnection, stmt *RollbackTransactionStatement, startTime time.Time) (*SQLResult, error) {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()

	if conn.CurrentTxn == nil {
		return &SQLResult{
			Error:         fmt.Errorf("no active transaction"),
			ExecutionTime: time.Since(startTime),
		}, fmt.Errorf("no active transaction")
	}

	txnID := conn.CurrentTxn.sqlID

	// Check if this is a distributed transaction
	if conn.CurrentDistTxn != nil {
		err := se.distTxnCoordinator.AbortDistributedTransaction(ctx, conn.CurrentDistTxn.ID)
		conn.CurrentDistTxn = nil
		if err != nil {
			return &SQLResult{
				Error:         err,
				ExecutionTime: time.Since(startTime),
			}, err
		}
	} else {
		// Regular transaction rollback
		err := se.txnManager.RollbackTransaction(ctx, txnID, stmt)
		if err != nil {
			return &SQLResult{
				Error:         err,
				ExecutionTime: time.Since(startTime),
			}, err
		}
	}

	conn.CurrentTxn = nil
	conn.AutoCommit = true

	return &SQLResult{
		TransactionID: txnID,
		ExecutionTime: time.Since(startTime),
	}, nil
}

// executeSavepoint executes SAVEPOINT
func (se *SQLEngine) executeSavepoint(ctx context.Context, conn *SQLConnection, stmt *SavepointStatement, startTime time.Time) (*SQLResult, error) {
	conn.mutex.RLock()
	txn := conn.CurrentTxn
	conn.mutex.RUnlock()

	if txn == nil {
		return &SQLResult{
			Error:         fmt.Errorf("no active transaction"),
			ExecutionTime: time.Since(startTime),
		}, fmt.Errorf("no active transaction")
	}

	err := se.txnManager.CreateSavepoint(ctx, txn.sqlID, stmt)
	if err != nil {
		return &SQLResult{
			Error:         err,
			ExecutionTime: time.Since(startTime),
		}, err
	}

	return &SQLResult{
		TransactionID: txn.sqlID,
		ExecutionTime: time.Since(startTime),
	}, nil
}

// executeReleaseSavepoint executes RELEASE SAVEPOINT
func (se *SQLEngine) executeReleaseSavepoint(ctx context.Context, conn *SQLConnection, stmt *ReleaseSavepointStatement, startTime time.Time) (*SQLResult, error) {
	conn.mutex.RLock()
	txn := conn.CurrentTxn
	conn.mutex.RUnlock()

	if txn == nil {
		return &SQLResult{
			Error:         fmt.Errorf("no active transaction"),
			ExecutionTime: time.Since(startTime),
		}, fmt.Errorf("no active transaction")
	}

	err := se.txnManager.ReleaseSavepoint(ctx, txn.sqlID, stmt)
	if err != nil {
		return &SQLResult{
			Error:         err,
			ExecutionTime: time.Since(startTime),
		}, err
	}

	return &SQLResult{
		TransactionID: txn.sqlID,
		ExecutionTime: time.Since(startTime),
	}, nil
}

// executeDataStatement executes data manipulation statements (SELECT, INSERT, UPDATE, DELETE)
func (se *SQLEngine) executeDataStatement(ctx context.Context, conn *SQLConnection, stmt Statement, startTime time.Time) (*SQLResult, error) {
	// Handle auto-commit mode
	var txn *SQLTransaction
	var distTxn *DistributedTransaction
	var autoCommitTxn bool

	conn.mutex.Lock()
	if conn.AutoCommit && conn.CurrentTxn == nil {
		// Start auto-commit transaction
		beginStmt := &BeginTransactionStatement{
			ReadOnly: conn.ReadOnly,
		}
		isolation := SQLIsolationLevel(conn.IsolationLevel)
		beginStmt.IsolationLevel = &isolation

		var err error
		txn, err = se.txnManager.BeginTransaction(ctx, beginStmt)
		if err != nil {
			conn.mutex.Unlock()
			return &SQLResult{
				Error:         fmt.Errorf("failed to begin auto-commit transaction: %w", err),
				ExecutionTime: time.Since(startTime),
			}, err
		}
		autoCommitTxn = true
	} else {
		txn = conn.CurrentTxn
		distTxn = conn.CurrentDistTxn
	}
	conn.mutex.Unlock()

	if txn == nil {
		return &SQLResult{
			Error:         fmt.Errorf("no active transaction"),
			ExecutionTime: time.Since(startTime),
		}, fmt.Errorf("no active transaction")
	}

	// Determine if this requires distributed transaction coordination
	tables := se.extractTablesFromStatement(stmt)
	requiresDistTxn := se.requiresDistributedTransaction(tables)

	var result *ResultSet
	var err error

	if requiresDistTxn && se.config.EnableDistributedTxns {
		// Use distributed transaction
		if distTxn == nil {
			beginStmt := &BeginTransactionStatement{
				ReadOnly: conn.ReadOnly,
			}
			isolation := SQLIsolationLevel(conn.IsolationLevel)
			beginStmt.IsolationLevel = &isolation

			distTxn, err = se.distTxnCoordinator.BeginDistributedTransaction(ctx, beginStmt, tables)
			if err != nil {
				return &SQLResult{
					Error:         fmt.Errorf("failed to begin distributed transaction: %w", err),
					ExecutionTime: time.Since(startTime),
				}, err
			}

			conn.mutex.Lock()
			conn.CurrentDistTxn = distTxn
			conn.mutex.Unlock()
		}

		result, err = se.distTxnCoordinator.ExecuteInDistributedTransaction(ctx, distTxn.ID, stmt, se.executor)
	} else {
		// Use regular transaction
		result, err = se.txnManager.ExecuteInTransaction(ctx, txn.sqlID, stmt, se.executor)
	}

	if err != nil {
		// Rollback auto-commit transaction on error
		if autoCommitTxn {
			rollbackStmt := &RollbackTransactionStatement{}
			se.txnManager.RollbackTransaction(ctx, txn.sqlID, rollbackStmt)
		}

		return &SQLResult{
			Error:         err,
			ExecutionTime: time.Since(startTime),
		}, err
	}

	// Commit auto-commit transaction
	if autoCommitTxn {
		commitStmt := &CommitTransactionStatement{}
		if distTxn != nil {
			err = se.distTxnCoordinator.CommitDistributedTransaction(ctx, distTxn.ID)
			conn.mutex.Lock()
			conn.CurrentDistTxn = nil
			conn.mutex.Unlock()
		} else {
			err = se.txnManager.CommitTransaction(ctx, txn.sqlID, commitStmt)
		}

		if err != nil {
			return &SQLResult{
				Error:         fmt.Errorf("failed to commit auto-commit transaction: %w", err),
				ExecutionTime: time.Since(startTime),
			}, err
		}
	}

	// Calculate rows affected
	var rowsAffected int64
	if result != nil {
		rowsAffected = int64(len(result.Rows))
	}

	return &SQLResult{
		ResultSet:     result,
		RowsAffected:  rowsAffected,
		ExecutionTime: time.Since(startTime),
		TransactionID: txn.sqlID,
	}, nil
}

// Helper methods

func (se *SQLEngine) extractTablesFromStatement(stmt Statement) []string {
	switch s := stmt.(type) {
	case *SelectStatement:
		var tables []string
		for _, table := range s.From {
			if table.Name != "" {
				tables = append(tables, table.Name)
			}
		}
		return tables
	case *InsertStatement:
		if s.Table != nil && s.Table.Name != "" {
			return []string{s.Table.Name}
		}
	case *UpdateStatement:
		if s.Table != nil && s.Table.Name != "" {
			return []string{s.Table.Name}
		}
	case *DeleteStatement:
		if s.From != nil && s.From.Name != "" {
			return []string{s.From.Name}
		}
	}
	return []string{}
}

func (se *SQLEngine) requiresDistributedTransaction(tables []string) bool {
	if len(tables) <= 1 {
		return false
	}

	// Check if tables are on different storage engines
	storageTypes := make(map[StorageType]bool)
	for _, table := range tables {
		storageType := se.getStorageTypeForTable(table)
		storageTypes[storageType] = true
	}

	return len(storageTypes) > 1
}

func (se *SQLEngine) getStorageTypeForTable(tableName string) StorageType {
	// This would use metadata to determine storage type
	// For now, use simple heuristics
	if len(tableName) > 3 {
		switch tableName[:3] {
		case "kv_":
			return StorageTypeKV
		case "doc":
			return StorageTypeDocument
		case "col":
			return StorageTypeColumnar
		}
	}
	return StorageTypeColumnar // Default
}

func (se *SQLEngine) registerStorageParticipants() {
	// Register KV storage participant
	if se.storageManager.kvStore != nil {
		kvParticipant := NewStorageEngineParticipant(
			"kv_storage",
			StorageTypeKV,
			se.storageManager.kvStore,
			se.txnManager.txnSystem,
		)
		se.distTxnCoordinator.RegisterParticipant(kvParticipant)
	}

	// Register document storage participant
	if se.storageManager.docStore != nil {
		docParticipant := NewStorageEngineParticipant(
			"document_storage",
			StorageTypeDocument,
			se.storageManager.docStore,
			se.txnManager.txnSystem,
		)
		se.distTxnCoordinator.RegisterParticipant(docParticipant)
	}

	// Register columnar storage participant
	if se.storageManager.columnarStore != nil {
		columnarParticipant := NewStorageEngineParticipant(
			"columnar_storage",
			StorageTypeColumnar,
			se.storageManager.columnarStore,
			se.txnManager.txnSystem,
		)
		se.distTxnCoordinator.RegisterParticipant(columnarParticipant)
	}
}

func (se *SQLEngine) startBackgroundTasks() {
	// Start transaction cleanup task
	se.wg.Add(1)
	go func() {
		defer se.wg.Done()
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				se.txnManager.CleanupIdleTransactions(se.config.IdleTransactionTimeout)
				se.cleanupIdleConnections()
			case <-se.shutdownChan:
				return
			}
		}
	}()
}

func (se *SQLEngine) cleanupIdleConnections() {
	se.connectionMutex.Lock()
	defer se.connectionMutex.Unlock()

	now := time.Now()
	var toCleanup []string

	for id, conn := range se.activeConnections {
		conn.mutex.RLock()
		idle := now.Sub(conn.LastActivity) > se.config.ConnectionTimeout
		conn.mutex.RUnlock()

		if idle {
			toCleanup = append(toCleanup, id)
		}
	}

	for _, id := range toCleanup {
		se.CloseConnection(id)
	}
}

// GetConnectionInfo returns information about a connection
func (se *SQLEngine) GetConnectionInfo(connectionID string) (*SQLConnection, error) {
	se.connectionMutex.RLock()
	defer se.connectionMutex.RUnlock()

	conn, exists := se.activeConnections[connectionID]
	if !exists {
		return nil, fmt.Errorf("connection %s not found", connectionID)
	}

	return conn, nil
}

// GetActiveConnections returns all active connections
func (se *SQLEngine) GetActiveConnections() []*SQLConnection {
	se.connectionMutex.RLock()
	defer se.connectionMutex.RUnlock()

	connections := make([]*SQLConnection, 0, len(se.activeConnections))
	for _, conn := range se.activeConnections {
		connections = append(connections, conn)
	}

	return connections
}

// Shutdown gracefully shuts down the SQL engine
func (se *SQLEngine) Shutdown(ctx context.Context) error {
	// Signal shutdown
	close(se.shutdownChan)

	// Wait for background tasks to complete
	done := make(chan struct{})
	go func() {
		se.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Background tasks completed
	case <-ctx.Done():
		// Timeout reached
		return ctx.Err()
	}

	// Close all connections
	se.connectionMutex.Lock()
	for id := range se.activeConnections {
		se.CloseConnection(id)
	}
	se.connectionMutex.Unlock()

	// Close transaction manager
	if err := se.txnManager.Close(); err != nil {
		return fmt.Errorf("failed to close transaction manager: %w", err)
	}

	return nil
}
