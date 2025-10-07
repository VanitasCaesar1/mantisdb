package sql

import (
	"context"
	"fmt"
	"sync"
	"time"

	"mantisDB/transaction"
)

// DistributedTransactionCoordinator manages distributed transactions across multiple storage engines
type DistributedTransactionCoordinator struct {
	localTxnManager *SQLTransactionManager
	participants    map[string]TransactionParticipant
	activeDistTxns  map[string]*DistributedTransaction
	mutex           sync.RWMutex
	coordinatorID   string
	timeoutDuration time.Duration
}

// TransactionParticipant represents a participant in a distributed transaction
type TransactionParticipant interface {
	// Prepare phase of 2PC
	Prepare(ctx context.Context, txnID string) error
	// Commit phase of 2PC
	Commit(ctx context.Context, txnID string) error
	// Abort phase of 2PC
	Abort(ctx context.Context, txnID string) error
	// Get participant ID
	GetID() string
}

// DistributedTransaction represents a distributed transaction
type DistributedTransaction struct {
	ID           string
	LocalTxn     *SQLTransaction
	Participants map[string]TransactionParticipant
	State        DistributedTxnState
	StartTime    time.Time
	PrepareTime  time.Time
	EndTime      time.Time
	Tables       []string
	Operations   []DistributedOperation
	mutex        sync.RWMutex
}

// DistributedOperation represents an operation in a distributed transaction
type DistributedOperation struct {
	Type          DistributedOpType
	ParticipantID string
	TableName     string
	Statement     Statement
	Timestamp     time.Time
}

// DistributedTxnState represents the state of a distributed transaction
type DistributedTxnState int

const (
	DistTxnActive DistributedTxnState = iota
	DistTxnPreparing
	DistTxnPrepared
	DistTxnCommitting
	DistTxnCommitted
	DistTxnAborting
	DistTxnAborted
)

func (s DistributedTxnState) String() string {
	switch s {
	case DistTxnActive:
		return "ACTIVE"
	case DistTxnPreparing:
		return "PREPARING"
	case DistTxnPrepared:
		return "PREPARED"
	case DistTxnCommitting:
		return "COMMITTING"
	case DistTxnCommitted:
		return "COMMITTED"
	case DistTxnAborting:
		return "ABORTING"
	case DistTxnAborted:
		return "ABORTED"
	default:
		return "UNKNOWN"
	}
}

// DistributedOpType represents the type of distributed operation
type DistributedOpType int

const (
	DistOpRead DistributedOpType = iota
	DistOpWrite
	DistOpDelete
	DistOpCreate
	DistOpDrop
)

// StorageEngineParticipant implements TransactionParticipant for storage engines
type StorageEngineParticipant struct {
	id          string
	storageType StorageType
	engine      interface{} // KVStorageEngine, DocumentStorageEngine, or ColumnarStorageEngine
	txnManager  *transaction.TransactionSystem
	activeTxns  map[string]*transaction.Transaction
	mutex       sync.RWMutex
}

// NewDistributedTransactionCoordinator creates a new distributed transaction coordinator
func NewDistributedTransactionCoordinator(localTxnManager *SQLTransactionManager, coordinatorID string) *DistributedTransactionCoordinator {
	return &DistributedTransactionCoordinator{
		localTxnManager: localTxnManager,
		participants:    make(map[string]TransactionParticipant),
		activeDistTxns:  make(map[string]*DistributedTransaction),
		coordinatorID:   coordinatorID,
		timeoutDuration: 30 * time.Second,
	}
}

// RegisterParticipant registers a transaction participant
func (dtc *DistributedTransactionCoordinator) RegisterParticipant(participant TransactionParticipant) {
	dtc.mutex.Lock()
	defer dtc.mutex.Unlock()
	dtc.participants[participant.GetID()] = participant
}

// BeginDistributedTransaction starts a new distributed transaction
func (dtc *DistributedTransactionCoordinator) BeginDistributedTransaction(ctx context.Context, stmt *BeginTransactionStatement, tables []string) (*DistributedTransaction, error) {
	dtc.mutex.Lock()
	defer dtc.mutex.Unlock()

	// Start local transaction
	localTxn, err := dtc.localTxnManager.BeginTransaction(ctx, stmt)
	if err != nil {
		return nil, fmt.Errorf("failed to begin local transaction: %w", err)
	}

	// Create distributed transaction
	distTxn := &DistributedTransaction{
		ID:           fmt.Sprintf("dist_%s_%d", dtc.coordinatorID, time.Now().UnixNano()),
		LocalTxn:     localTxn,
		Participants: make(map[string]TransactionParticipant),
		State:        DistTxnActive,
		StartTime:    time.Now(),
		Tables:       tables,
		Operations:   make([]DistributedOperation, 0),
	}

	// Determine which participants are needed based on tables
	for _, table := range tables {
		participantID := dtc.getParticipantForTable(table)
		if participant, exists := dtc.participants[participantID]; exists {
			distTxn.Participants[participantID] = participant
		}
	}

	// Store distributed transaction
	dtc.activeDistTxns[distTxn.ID] = distTxn

	return distTxn, nil
}

// ExecuteInDistributedTransaction executes a statement in a distributed transaction
func (dtc *DistributedTransactionCoordinator) ExecuteInDistributedTransaction(ctx context.Context, distTxnID string, stmt Statement, executor *QueryExecutor) (*ResultSet, error) {
	dtc.mutex.RLock()
	distTxn, exists := dtc.activeDistTxns[distTxnID]
	dtc.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("distributed transaction %s not found", distTxnID)
	}

	if distTxn.State != DistTxnActive {
		return nil, fmt.Errorf("distributed transaction %s is not active (state: %s)", distTxnID, distTxn.State)
	}

	// Record the operation
	operation := DistributedOperation{
		Type:      dtc.getOperationType(stmt),
		Statement: stmt,
		Timestamp: time.Now(),
	}

	// Determine which participant should handle this operation
	tables := dtc.extractTablesFromStatement(stmt)
	if len(tables) == 0 {
		return nil, fmt.Errorf("no tables found in statement")
	}

	// For multi-table operations, we need to coordinate across participants
	if len(tables) > 1 {
		return dtc.executeMultiTableOperation(ctx, distTxn, stmt, tables, executor)
	}

	// Single table operation
	table := tables[0]
	participantID := dtc.getParticipantForTable(table)
	operation.ParticipantID = participantID
	operation.TableName = table

	// Add operation to transaction
	distTxn.mutex.Lock()
	distTxn.Operations = append(distTxn.Operations, operation)
	distTxn.mutex.Unlock()

	// Execute through local transaction manager
	return dtc.localTxnManager.ExecuteInTransaction(ctx, distTxn.LocalTxn.sqlID, stmt, executor)
}

// executeMultiTableOperation executes operations that span multiple tables/participants
func (dtc *DistributedTransactionCoordinator) executeMultiTableOperation(ctx context.Context, distTxn *DistributedTransaction, stmt Statement, tables []string, executor *QueryExecutor) (*ResultSet, error) {
	// For multi-table operations, we need to ensure atomicity across participants
	// This is where the distributed transaction coordination becomes critical

	// Record operations for each table
	for _, table := range tables {
		participantID := dtc.getParticipantForTable(table)
		operation := DistributedOperation{
			Type:          dtc.getOperationType(stmt),
			ParticipantID: participantID,
			TableName:     table,
			Statement:     stmt,
			Timestamp:     time.Now(),
		}

		distTxn.mutex.Lock()
		distTxn.Operations = append(distTxn.Operations, operation)
		distTxn.mutex.Unlock()
	}

	// For now, execute through local transaction manager
	// In a full implementation, this would coordinate across multiple participants
	return dtc.localTxnManager.ExecuteInTransaction(ctx, distTxn.LocalTxn.sqlID, stmt, executor)
}

// CommitDistributedTransaction commits a distributed transaction using 2PC
func (dtc *DistributedTransactionCoordinator) CommitDistributedTransaction(ctx context.Context, distTxnID string) error {
	dtc.mutex.Lock()
	distTxn, exists := dtc.activeDistTxns[distTxnID]
	if !exists {
		dtc.mutex.Unlock()
		return fmt.Errorf("distributed transaction %s not found", distTxnID)
	}
	dtc.mutex.Unlock()

	if distTxn.State != DistTxnActive {
		return fmt.Errorf("distributed transaction %s is not active (state: %s)", distTxnID, distTxn.State)
	}

	// Phase 1: Prepare
	if err := dtc.preparePhase(ctx, distTxn); err != nil {
		// If prepare fails, abort the transaction
		dtc.AbortDistributedTransaction(ctx, distTxnID)
		return fmt.Errorf("prepare phase failed: %w", err)
	}

	// Phase 2: Commit
	if err := dtc.commitPhase(ctx, distTxn); err != nil {
		// If commit fails, we're in an inconsistent state
		// In a real implementation, this would require recovery procedures
		return fmt.Errorf("commit phase failed: %w", err)
	}

	// Clean up
	dtc.mutex.Lock()
	delete(dtc.activeDistTxns, distTxnID)
	dtc.mutex.Unlock()

	return nil
}

// AbortDistributedTransaction aborts a distributed transaction
func (dtc *DistributedTransactionCoordinator) AbortDistributedTransaction(ctx context.Context, distTxnID string) error {
	dtc.mutex.Lock()
	distTxn, exists := dtc.activeDistTxns[distTxnID]
	if !exists {
		dtc.mutex.Unlock()
		return fmt.Errorf("distributed transaction %s not found", distTxnID)
	}
	dtc.mutex.Unlock()

	// Update state
	distTxn.mutex.Lock()
	distTxn.State = DistTxnAborting
	distTxn.mutex.Unlock()

	// Abort local transaction
	stmt := &RollbackTransactionStatement{}
	if err := dtc.localTxnManager.RollbackTransaction(ctx, distTxn.LocalTxn.sqlID, stmt); err != nil {
		return fmt.Errorf("failed to abort local transaction: %w", err)
	}

	// Abort all participants
	for participantID, participant := range distTxn.Participants {
		if err := participant.Abort(ctx, distTxnID); err != nil {
			// Log error but continue with other participants
			fmt.Printf("Warning: failed to abort participant %s: %v\n", participantID, err)
		}
	}

	// Update final state
	distTxn.mutex.Lock()
	distTxn.State = DistTxnAborted
	distTxn.EndTime = time.Now()
	distTxn.mutex.Unlock()

	// Clean up
	dtc.mutex.Lock()
	delete(dtc.activeDistTxns, distTxnID)
	dtc.mutex.Unlock()

	return nil
}

// preparePhase executes the prepare phase of 2PC
func (dtc *DistributedTransactionCoordinator) preparePhase(ctx context.Context, distTxn *DistributedTransaction) error {
	// Update state
	distTxn.mutex.Lock()
	distTxn.State = DistTxnPreparing
	distTxn.mutex.Unlock()

	// Create context with timeout
	prepareCtx, cancel := context.WithTimeout(ctx, dtc.timeoutDuration)
	defer cancel()

	// Prepare all participants
	var wg sync.WaitGroup
	errChan := make(chan error, len(distTxn.Participants))

	for participantID, participant := range distTxn.Participants {
		wg.Add(1)
		go func(id string, p TransactionParticipant) {
			defer wg.Done()
			if err := p.Prepare(prepareCtx, distTxn.ID); err != nil {
				errChan <- fmt.Errorf("participant %s prepare failed: %w", id, err)
			}
		}(participantID, participant)
	}

	// Wait for all participants to complete prepare
	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		return err
	}

	// Update state
	distTxn.mutex.Lock()
	distTxn.State = DistTxnPrepared
	distTxn.PrepareTime = time.Now()
	distTxn.mutex.Unlock()

	return nil
}

// commitPhase executes the commit phase of 2PC
func (dtc *DistributedTransactionCoordinator) commitPhase(ctx context.Context, distTxn *DistributedTransaction) error {
	// Update state
	distTxn.mutex.Lock()
	distTxn.State = DistTxnCommitting
	distTxn.mutex.Unlock()

	// Commit local transaction first
	stmt := &CommitTransactionStatement{}
	if err := dtc.localTxnManager.CommitTransaction(ctx, distTxn.LocalTxn.sqlID, stmt); err != nil {
		return fmt.Errorf("failed to commit local transaction: %w", err)
	}

	// Commit all participants
	// In 2PC, once we reach this phase, we must commit all participants
	// even if some fail (they would need to be recovered later)
	for participantID, participant := range distTxn.Participants {
		if err := participant.Commit(ctx, distTxn.ID); err != nil {
			// Log error but continue - in a real implementation, this would
			// require recovery procedures to ensure eventual consistency
			fmt.Printf("Warning: failed to commit participant %s: %v\n", participantID, err)
		}
	}

	// Update final state
	distTxn.mutex.Lock()
	distTxn.State = DistTxnCommitted
	distTxn.EndTime = time.Now()
	distTxn.mutex.Unlock()

	return nil
}

// Helper methods

func (dtc *DistributedTransactionCoordinator) getParticipantForTable(tableName string) string {
	// In a real implementation, this would use a routing table or metadata
	// to determine which participant handles which table
	// For now, we'll use simple heuristics based on table name prefixes
	if len(tableName) > 3 {
		switch tableName[:3] {
		case "kv_":
			return "kv_storage"
		case "doc":
			return "document_storage"
		case "col":
			return "columnar_storage"
		}
	}
	return "default_storage"
}

func (dtc *DistributedTransactionCoordinator) getOperationType(stmt Statement) DistributedOpType {
	switch stmt.(type) {
	case *SelectStatement:
		return DistOpRead
	case *InsertStatement:
		return DistOpWrite
	case *UpdateStatement:
		return DistOpWrite
	case *DeleteStatement:
		return DistOpDelete
	case *CreateTableStatement:
		return DistOpCreate
	case *DropTableStatement:
		return DistOpDrop
	default:
		return DistOpRead
	}
}

func (dtc *DistributedTransactionCoordinator) extractTablesFromStatement(stmt Statement) []string {
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

// GetActiveDistributedTransactions returns all active distributed transactions
func (dtc *DistributedTransactionCoordinator) GetActiveDistributedTransactions() []*DistributedTransaction {
	dtc.mutex.RLock()
	defer dtc.mutex.RUnlock()

	transactions := make([]*DistributedTransaction, 0, len(dtc.activeDistTxns))
	for _, txn := range dtc.activeDistTxns {
		transactions = append(transactions, txn)
	}

	return transactions
}

// RecoverDistributedTransactions recovers distributed transactions after a crash
func (dtc *DistributedTransactionCoordinator) RecoverDistributedTransactions(ctx context.Context) error {
	// In a real implementation, this would:
	// 1. Read transaction log to find transactions in PREPARED state
	// 2. Query participants to determine their state
	// 3. Complete or abort transactions based on participant responses
	// 4. Handle heuristic decisions for transactions where participants disagree

	// For now, this is a placeholder
	fmt.Println("Distributed transaction recovery not yet implemented")
	return nil
}

// Storage Engine Participant Implementation

func NewStorageEngineParticipant(id string, storageType StorageType, engine interface{}, txnManager *transaction.TransactionSystem) *StorageEngineParticipant {
	return &StorageEngineParticipant{
		id:          id,
		storageType: storageType,
		engine:      engine,
		txnManager:  txnManager,
		activeTxns:  make(map[string]*transaction.Transaction),
	}
}

func (sep *StorageEngineParticipant) GetID() string {
	return sep.id
}

func (sep *StorageEngineParticipant) Prepare(ctx context.Context, txnID string) error {
	sep.mutex.RLock()
	txn, exists := sep.activeTxns[txnID]
	sep.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("transaction %s not found in participant %s", txnID, sep.id)
	}

	// In a real implementation, this would:
	// 1. Flush all changes to stable storage
	// 2. Write a PREPARED record to the transaction log
	// 3. Ensure all locks are held
	// 4. Return success only if the transaction can be committed

	// For now, we'll just check if the transaction is still active
	if txn.Status != transaction.TxnActive {
		return fmt.Errorf("transaction %s is not active in participant %s", txnID, sep.id)
	}

	return nil
}

func (sep *StorageEngineParticipant) Commit(ctx context.Context, txnID string) error {
	sep.mutex.Lock()
	defer sep.mutex.Unlock()

	txn, exists := sep.activeTxns[txnID]
	if !exists {
		return fmt.Errorf("transaction %s not found in participant %s", txnID, sep.id)
	}

	// Commit the transaction
	if err := sep.txnManager.CommitTransaction(txn); err != nil {
		return fmt.Errorf("failed to commit transaction %s in participant %s: %w", txnID, sep.id, err)
	}

	// Remove from active transactions
	delete(sep.activeTxns, txnID)

	return nil
}

func (sep *StorageEngineParticipant) Abort(ctx context.Context, txnID string) error {
	sep.mutex.Lock()
	defer sep.mutex.Unlock()

	txn, exists := sep.activeTxns[txnID]
	if !exists {
		// Transaction might have already been cleaned up
		return nil
	}

	// Abort the transaction
	if err := sep.txnManager.AbortTransaction(txn); err != nil {
		return fmt.Errorf("failed to abort transaction %s in participant %s: %w", txnID, sep.id, err)
	}

	// Remove from active transactions
	delete(sep.activeTxns, txnID)

	return nil
}

// AddTransaction adds a transaction to the participant
func (sep *StorageEngineParticipant) AddTransaction(txnID string, txn *transaction.Transaction) {
	sep.mutex.Lock()
	defer sep.mutex.Unlock()
	sep.activeTxns[txnID] = txn
}

// RemoveTransaction removes a transaction from the participant
func (sep *StorageEngineParticipant) RemoveTransaction(txnID string) {
	sep.mutex.Lock()
	defer sep.mutex.Unlock()
	delete(sep.activeTxns, txnID)
}
