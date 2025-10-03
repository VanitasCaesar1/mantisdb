package mantisdb

import (
	"context"
	"encoding/json"
	"fmt"
)

// Transaction represents a database transaction
type Transaction struct {
	ID     string
	client *Client
	closed bool
}

// Query executes a query within the transaction
func (tx *Transaction) Query(ctx context.Context, query string) (*Result, error) {
	if tx.closed {
		return nil, fmt.Errorf("transaction is closed")
	}

	queryReq := QueryRequest{
		SQL: query,
	}

	req, err := tx.client.newRequest(ctx, "POST", fmt.Sprintf("/api/transactions/%s/query", tx.ID), queryReq)
	if err != nil {
		return nil, err
	}

	resp, err := tx.client.doRequestWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, tx.client.handleErrorResponse(resp)
	}

	var result Result
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// Insert inserts data within the transaction
func (tx *Transaction) Insert(ctx context.Context, table string, data interface{}) error {
	if tx.closed {
		return fmt.Errorf("transaction is closed")
	}

	insertReq := InsertRequest{
		Table: table,
		Data:  data,
	}

	req, err := tx.client.newRequest(ctx, "POST", fmt.Sprintf("/api/transactions/%s/tables/%s/data", tx.ID, table), insertReq)
	if err != nil {
		return err
	}

	resp, err := tx.client.doRequestWithRetry(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		return tx.client.handleErrorResponse(resp)
	}

	return nil
}

// Update updates data within the transaction
func (tx *Transaction) Update(ctx context.Context, table string, id string, data interface{}) error {
	if tx.closed {
		return fmt.Errorf("transaction is closed")
	}

	updateReq := UpdateRequest{
		Data: data,
	}

	req, err := tx.client.newRequest(ctx, "PUT", fmt.Sprintf("/api/transactions/%s/tables/%s/data/%s", tx.ID, table, id), updateReq)
	if err != nil {
		return err
	}

	resp, err := tx.client.doRequestWithRetry(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return tx.client.handleErrorResponse(resp)
	}

	return nil
}

// Delete deletes data within the transaction
func (tx *Transaction) Delete(ctx context.Context, table string, id string) error {
	if tx.closed {
		return fmt.Errorf("transaction is closed")
	}

	req, err := tx.client.newRequest(ctx, "DELETE", fmt.Sprintf("/api/transactions/%s/tables/%s/data/%s", tx.ID, table, id), nil)
	if err != nil {
		return err
	}

	resp, err := tx.client.doRequestWithRetry(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		return tx.client.handleErrorResponse(resp)
	}

	return nil
}

// Commit commits the transaction
func (tx *Transaction) Commit(ctx context.Context) error {
	if tx.closed {
		return fmt.Errorf("transaction is already closed")
	}

	req, err := tx.client.newRequest(ctx, "POST", fmt.Sprintf("/api/transactions/%s/commit", tx.ID), nil)
	if err != nil {
		return err
	}

	resp, err := tx.client.doRequestWithRetry(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	tx.closed = true

	if resp.StatusCode != 200 {
		return tx.client.handleErrorResponse(resp)
	}

	return nil
}

// Rollback rolls back the transaction
func (tx *Transaction) Rollback(ctx context.Context) error {
	if tx.closed {
		return fmt.Errorf("transaction is already closed")
	}

	req, err := tx.client.newRequest(ctx, "POST", fmt.Sprintf("/api/transactions/%s/rollback", tx.ID), nil)
	if err != nil {
		return err
	}

	resp, err := tx.client.doRequestWithRetry(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	tx.closed = true

	if resp.StatusCode != 200 {
		return tx.client.handleErrorResponse(resp)
	}

	return nil
}

// IsClosed returns whether the transaction is closed
func (tx *Transaction) IsClosed() bool {
	return tx.closed
}
