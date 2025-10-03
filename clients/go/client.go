package mantisdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Client represents the MantisDB Go client
type Client struct {
	config     *Config
	httpClient *http.Client
	pool       *ConnectionPool
	baseURL    string
	authMgr    *AuthManager
	connMgr    *ConnectionManager
	mu         sync.RWMutex
}

// Config holds client configuration
type Config struct {
	Host              string
	Port              int
	Username          string
	Password          string
	APIKey            string
	ClientID          string
	ClientSecret      string
	TokenURL          string
	AuthProvider      AuthProvider
	MaxConnections    int
	ConnectionTimeout time.Duration
	RequestTimeout    time.Duration
	RetryAttempts     int
	RetryDelay        time.Duration
	EnableCompression bool
	TLSEnabled        bool
	EnableFailover    bool
	FailoverHosts     []string
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Host:              "localhost",
		Port:              8080,
		MaxConnections:    10,
		ConnectionTimeout: 30 * time.Second,
		RequestTimeout:    60 * time.Second,
		RetryAttempts:     3,
		RetryDelay:        1 * time.Second,
		EnableCompression: true,
		TLSEnabled:        false,
	}
}

// ConnectionPool manages HTTP connections
type ConnectionPool struct {
	client *http.Client
	config *Config
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(config *Config) *ConnectionPool {
	transport := &http.Transport{
		MaxIdleConns:        config.MaxConnections,
		MaxIdleConnsPerHost: config.MaxConnections,
		IdleConnTimeout:     config.ConnectionTimeout,
	}

	return &ConnectionPool{
		client: &http.Client{
			Transport: transport,
			Timeout:   config.RequestTimeout,
		},
		config: config,
	}
}

// NewClient creates a new MantisDB client
func NewClient(config *Config) (*Client, error) {
	if config == nil {
		config = DefaultConfig()
	}

	pool := NewConnectionPool(config)
	connMgr := NewConnectionManager(config)

	scheme := "http"
	if config.TLSEnabled {
		scheme = "https"
	}

	baseURL := fmt.Sprintf("%s://%s:%d", scheme, config.Host, config.Port)

	// Set up authentication
	var authProvider AuthProvider
	if config.AuthProvider != nil {
		authProvider = config.AuthProvider
	} else if config.APIKey != "" {
		authProvider = NewAPIKeyAuthProvider(config.APIKey)
	} else if config.ClientID != "" && config.ClientSecret != "" {
		tokenURL := config.TokenURL
		if tokenURL == "" {
			tokenURL = baseURL + "/oauth/token"
		}
		authProvider = NewJWTAuthProvider(config.ClientID, config.ClientSecret, tokenURL)
	} else if config.Username != "" && config.Password != "" {
		authProvider = &BasicAuthProvider{
			Username: config.Username,
			Password: config.Password,
		}
	}

	var authMgr *AuthManager
	if authProvider != nil {
		authMgr = NewAuthManager(authProvider)
	}

	client := &Client{
		config:     config,
		httpClient: pool.client,
		pool:       pool,
		baseURL:    baseURL,
		authMgr:    authMgr,
		connMgr:    connMgr,
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), config.ConnectionTimeout)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to MantisDB: %w", err)
	}

	return client, nil
}

// Ping tests the connection to the database
func (c *Client) Ping(ctx context.Context) error {
	req, err := c.newRequest(ctx, "GET", "/api/health", nil)
	if err != nil {
		return err
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	return nil
}

// Query executes a query and returns results
func (c *Client) Query(ctx context.Context, query string) (*Result, error) {
	queryReq := QueryRequest{
		SQL: query,
	}

	req, err := c.newRequest(ctx, "POST", "/api/query", queryReq)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequestWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp)
	}

	var result Result
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// Insert inserts data into a table
func (c *Client) Insert(ctx context.Context, table string, data interface{}) error {
	insertReq := InsertRequest{
		Table: table,
		Data:  data,
	}

	req, err := c.newRequest(ctx, "POST", fmt.Sprintf("/api/tables/%s/data", table), insertReq)
	if err != nil {
		return err
	}

	resp, err := c.doRequestWithRetry(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return c.handleErrorResponse(resp)
	}

	return nil
}

// Update updates data in a table
func (c *Client) Update(ctx context.Context, table string, id string, data interface{}) error {
	updateReq := UpdateRequest{
		Data: data,
	}

	req, err := c.newRequest(ctx, "PUT", fmt.Sprintf("/api/tables/%s/data/%s", table, id), updateReq)
	if err != nil {
		return err
	}

	resp, err := c.doRequestWithRetry(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.handleErrorResponse(resp)
	}

	return nil
}

// Delete deletes data from a table
func (c *Client) Delete(ctx context.Context, table string, id string) error {
	req, err := c.newRequest(ctx, "DELETE", fmt.Sprintf("/api/tables/%s/data/%s", table, id), nil)
	if err != nil {
		return err
	}

	resp, err := c.doRequestWithRetry(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return c.handleErrorResponse(resp)
	}

	return nil
}

// Get retrieves data from a table
func (c *Client) Get(ctx context.Context, table string, filters map[string]interface{}) (*Result, error) {
	u, err := url.Parse(fmt.Sprintf("%s/api/tables/%s/data", c.baseURL, table))
	if err != nil {
		return nil, err
	}

	// Add filters as query parameters
	q := u.Query()
	for key, value := range filters {
		q.Add(key, fmt.Sprintf("%v", value))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req)

	resp, err := c.doRequestWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp)
	}

	var result Result
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// BeginTransaction starts a new transaction
func (c *Client) BeginTransaction(ctx context.Context) (*Transaction, error) {
	req, err := c.newRequest(ctx, "POST", "/api/transactions", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequestWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, c.handleErrorResponse(resp)
	}

	var txResp TransactionResponse
	if err := json.NewDecoder(resp.Body).Decode(&txResp); err != nil {
		return nil, fmt.Errorf("failed to decode transaction response: %w", err)
	}

	return &Transaction{
		ID:     txResp.TransactionID,
		client: c,
	}, nil
}

// Close closes the client and releases resources
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}

	return nil
}

// newRequest creates a new HTTP request
func (c *Client) newRequest(ctx context.Context, method, path string, body interface{}) (*http.Request, error) {
	var buf io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		buf = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, buf)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req)
	return req, nil
}

// setHeaders sets common headers for requests
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "MantisDB-Go-Client/1.0")

	if c.config.EnableCompression {
		req.Header.Set("Accept-Encoding", "gzip, deflate")
	}

	// Use authentication manager if available
	if c.authMgr != nil {
		authHeaders, err := c.authMgr.GetAuthHeaders(req.Context(), c)
		if err == nil {
			for key, value := range authHeaders {
				req.Header.Set(key, value)
			}
		}
	}
}

// doRequest executes an HTTP request
func (c *Client) doRequest(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

// doRequestWithRetry executes an HTTP request with retry logic
func (c *Client) doRequestWithRetry(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt <= c.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(c.config.RetryDelay * time.Duration(attempt))
		}

		resp, err = c.doRequest(req)
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}

		if resp != nil {
			resp.Body.Close()
		}
	}

	return resp, err
}

// handleErrorResponse handles error responses from the server
func (c *Client) handleErrorResponse(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	var errorResp ErrorResponse
	if err := json.Unmarshal(body, &errorResp); err != nil {
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return &MantisError{
		Code:      errorResp.Error.Code,
		Message:   errorResp.Error.Message,
		Details:   errorResp.Error.Details,
		RequestID: errorResp.Error.RequestID,
	}
}

// Request/Response types
type QueryRequest struct {
	SQL string `json:"sql"`
}

type InsertRequest struct {
	Table string      `json:"table"`
	Data  interface{} `json:"data"`
}

type UpdateRequest struct {
	Data interface{} `json:"data"`
}

type TransactionResponse struct {
	TransactionID string `json:"transaction_id"`
}

type ErrorResponse struct {
	Error struct {
		Code      string                 `json:"code"`
		Message   string                 `json:"message"`
		Details   map[string]interface{} `json:"details,omitempty"`
		RequestID string                 `json:"request_id,omitempty"`
	} `json:"error"`
}

// Result represents query results
type Result struct {
	Rows     []map[string]interface{} `json:"rows"`
	Columns  []string                 `json:"columns"`
	RowCount int                      `json:"row_count"`
	Metadata map[string]interface{}   `json:"metadata,omitempty"`
}

// MantisError represents a MantisDB error
type MantisError struct {
	Code      string
	Message   string
	Details   map[string]interface{}
	RequestID string
}

func (e *MantisError) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("MantisDB error [%s] (request: %s): %s", e.Code, e.RequestID, e.Message)
	}
	return fmt.Sprintf("MantisDB error [%s]: %s", e.Code, e.Message)
}

// GetConnectionStats returns connection pool statistics
func (c *Client) GetConnectionStats() ConnectionStats {
	if c.connMgr != nil {
		return c.connMgr.Stats()
	}
	return ConnectionStats{}
}

// SetAuthProvider sets a new authentication provider
func (c *Client) SetAuthProvider(provider AuthProvider) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.authMgr = NewAuthManager(provider)
}

// RefreshAuth forces a refresh of authentication tokens
func (c *Client) RefreshAuth(ctx context.Context) error {
	if c.authMgr == nil {
		return fmt.Errorf("no authentication provider configured")
	}

	_, err := c.authMgr.GetAuthHeaders(ctx, c)
	return err
}

// Failover attempts to connect to failover hosts if the primary host fails
func (c *Client) Failover(ctx context.Context) error {
	if !c.config.EnableFailover || len(c.config.FailoverHosts) == 0 {
		return fmt.Errorf("failover not configured")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	originalHost := c.config.Host
	originalPort := c.config.Port

	for _, hostPort := range c.config.FailoverHosts {
		// Parse host:port
		host := hostPort
		port := c.config.Port

		if colonIndex := strings.LastIndex(hostPort, ":"); colonIndex != -1 {
			host = hostPort[:colonIndex]
			if p, err := strconv.Atoi(hostPort[colonIndex+1:]); err == nil {
				port = p
			}
		}

		// Update configuration
		c.config.Host = host
		c.config.Port = port

		scheme := "http"
		if c.config.TLSEnabled {
			scheme = "https"
		}
		c.baseURL = fmt.Sprintf("%s://%s:%d", scheme, host, port)

		// Test connection
		if err := c.Ping(ctx); err == nil {
			fmt.Printf("Failed over to %s:%d\n", host, port)
			return nil
		}
	}

	// Restore original configuration if all failovers failed
	c.config.Host = originalHost
	c.config.Port = originalPort
	scheme := "http"
	if c.config.TLSEnabled {
		scheme = "https"
	}
	c.baseURL = fmt.Sprintf("%s://%s:%d", scheme, originalHost, originalPort)

	return fmt.Errorf("all failover hosts failed")
}

// HealthCheck performs a comprehensive health check
func (c *Client) HealthCheck(ctx context.Context) (*HealthCheckResult, error) {
	start := time.Now()

	result := &HealthCheckResult{
		Timestamp: start,
		Host:      c.config.Host,
		Port:      c.config.Port,
	}

	// Test basic connectivity
	if err := c.Ping(ctx); err != nil {
		result.Status = "unhealthy"
		result.Error = err.Error()
		result.Duration = time.Since(start)
		return result, err
	}

	// Test authentication if configured
	if c.authMgr != nil {
		if _, err := c.authMgr.GetAuthHeaders(ctx, c); err != nil {
			result.Status = "degraded"
			result.AuthError = err.Error()
		}
	}

	// Get connection stats
	result.ConnectionStats = c.GetConnectionStats()

	result.Duration = time.Since(start)
	if result.Status == "" {
		result.Status = "healthy"
	}

	return result, nil
}

type HealthCheckResult struct {
	Timestamp       time.Time       `json:"timestamp"`
	Status          string          `json:"status"`
	Host            string          `json:"host"`
	Port            int             `json:"port"`
	Duration        time.Duration   `json:"duration"`
	Error           string          `json:"error,omitempty"`
	AuthError       string          `json:"auth_error,omitempty"`
	ConnectionStats ConnectionStats `json:"connection_stats"`
}
