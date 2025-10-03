package mantisdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// AuthProvider interface for different authentication methods
type AuthProvider interface {
	Authenticate(ctx context.Context, client *Client) (*AuthToken, error)
	RefreshToken(ctx context.Context, client *Client, token *AuthToken) (*AuthToken, error)
	GetAuthHeaders() map[string]string
}

// AuthToken represents an authentication token
type AuthToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	ExpiresAt    time.Time `json:"expires_at"`
	Scope        string    `json:"scope,omitempty"`
}

// IsExpired checks if the token is expired
func (t *AuthToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt.Add(-30 * time.Second)) // 30 second buffer
}

// BasicAuthProvider implements basic username/password authentication
type BasicAuthProvider struct {
	Username string
	Password string
}

func (p *BasicAuthProvider) Authenticate(ctx context.Context, client *Client) (*AuthToken, error) {
	// Basic auth doesn't use tokens, return a dummy token
	return &AuthToken{
		AccessToken: "basic_auth",
		TokenType:   "Basic",
		ExpiresAt:   time.Now().Add(24 * time.Hour), // Long expiry for basic auth
	}, nil
}

func (p *BasicAuthProvider) RefreshToken(ctx context.Context, client *Client, token *AuthToken) (*AuthToken, error) {
	// Basic auth doesn't need refresh
	return token, nil
}

func (p *BasicAuthProvider) GetAuthHeaders() map[string]string {
	return map[string]string{
		"Authorization": fmt.Sprintf("Basic %s", basicAuth(p.Username, p.Password)),
	}
}

// APIKeyAuthProvider implements API key authentication
type APIKeyAuthProvider struct {
	APIKey string
	Header string // Header name for the API key (default: "X-API-Key")
}

func NewAPIKeyAuthProvider(apiKey string) *APIKeyAuthProvider {
	return &APIKeyAuthProvider{
		APIKey: apiKey,
		Header: "X-API-Key",
	}
}

func (p *APIKeyAuthProvider) Authenticate(ctx context.Context, client *Client) (*AuthToken, error) {
	return &AuthToken{
		AccessToken: p.APIKey,
		TokenType:   "ApiKey",
		ExpiresAt:   time.Now().Add(24 * time.Hour), // Long expiry for API keys
	}, nil
}

func (p *APIKeyAuthProvider) RefreshToken(ctx context.Context, client *Client, token *AuthToken) (*AuthToken, error) {
	return token, nil
}

func (p *APIKeyAuthProvider) GetAuthHeaders() map[string]string {
	return map[string]string{
		p.Header: p.APIKey,
	}
}

// JWTAuthProvider implements JWT token authentication
type JWTAuthProvider struct {
	ClientID     string
	ClientSecret string
	TokenURL     string
	Scope        string
	token        *AuthToken
	mu           sync.RWMutex
}

func NewJWTAuthProvider(clientID, clientSecret, tokenURL string) *JWTAuthProvider {
	return &JWTAuthProvider{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     tokenURL,
		Scope:        "read write",
	}
}

func (p *JWTAuthProvider) Authenticate(ctx context.Context, client *Client) (*AuthToken, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.token != nil && !p.token.IsExpired() {
		return p.token, nil
	}

	token, err := p.requestToken(ctx, client)
	if err != nil {
		return nil, err
	}

	p.token = token
	return token, nil
}

func (p *JWTAuthProvider) RefreshToken(ctx context.Context, client *Client, token *AuthToken) (*AuthToken, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if token.RefreshToken == "" {
		// No refresh token, get a new token
		return p.requestToken(ctx, client)
	}

	newToken, err := p.refreshTokenRequest(ctx, client, token.RefreshToken)
	if err != nil {
		// Refresh failed, try to get a new token
		return p.requestToken(ctx, client)
	}

	p.token = newToken
	return newToken, nil
}

func (p *JWTAuthProvider) GetAuthHeaders() map[string]string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.token == nil {
		return map[string]string{}
	}

	return map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", p.token.AccessToken),
	}
}

func (p *JWTAuthProvider) requestToken(ctx context.Context, client *Client) (*AuthToken, error) {
	tokenReq := map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     p.ClientID,
		"client_secret": p.ClientSecret,
		"scope":         p.Scope,
	}

	req, err := client.newRequest(ctx, "POST", p.TokenURL, tokenReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	resp, err := client.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request failed with status: %d", resp.StatusCode)
	}

	var token AuthToken
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	// Calculate expiry time
	token.ExpiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)

	return &token, nil
}

func (p *JWTAuthProvider) refreshTokenRequest(ctx context.Context, client *Client, refreshToken string) (*AuthToken, error) {
	refreshReq := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"client_id":     p.ClientID,
		"client_secret": p.ClientSecret,
	}

	req, err := client.newRequest(ctx, "POST", p.TokenURL, refreshReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh request: %w", err)
	}

	resp, err := client.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("refresh request failed with status: %d", resp.StatusCode)
	}

	var token AuthToken
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("failed to decode refresh response: %w", err)
	}

	token.ExpiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)

	return &token, nil
}

// AuthManager manages authentication for the client
type AuthManager struct {
	provider AuthProvider
	token    *AuthToken
	mu       sync.RWMutex
}

func NewAuthManager(provider AuthProvider) *AuthManager {
	return &AuthManager{
		provider: provider,
	}
}

func (am *AuthManager) GetAuthHeaders(ctx context.Context, client *Client) (map[string]string, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Check if we need to authenticate or refresh
	if am.token == nil || am.token.IsExpired() {
		var err error
		if am.token == nil {
			am.token, err = am.provider.Authenticate(ctx, client)
		} else {
			am.token, err = am.provider.RefreshToken(ctx, client, am.token)
		}

		if err != nil {
			return nil, fmt.Errorf("authentication failed: %w", err)
		}
	}

	return am.provider.GetAuthHeaders(), nil
}

// Connection pool management
type ConnectionManager struct {
	maxConnections int
	activeConns    int
	idleConns      chan *http.Client
	mu             sync.Mutex
	config         *Config
}

func NewConnectionManager(config *Config) *ConnectionManager {
	cm := &ConnectionManager{
		maxConnections: config.MaxConnections,
		idleConns:      make(chan *http.Client, config.MaxConnections),
		config:         config,
	}

	// Pre-populate with connections
	for i := 0; i < config.MaxConnections; i++ {
		client := cm.createHTTPClient()
		cm.idleConns <- client
	}

	return cm
}

func (cm *ConnectionManager) createHTTPClient() *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        cm.maxConnections,
		MaxIdleConnsPerHost: cm.maxConnections,
		IdleConnTimeout:     cm.config.ConnectionTimeout,
		DisableKeepAlives:   false,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   cm.config.RequestTimeout,
	}
}

func (cm *ConnectionManager) GetConnection() *http.Client {
	select {
	case client := <-cm.idleConns:
		cm.mu.Lock()
		cm.activeConns++
		cm.mu.Unlock()
		return client
	default:
		// No idle connections available, create a new one
		return cm.createHTTPClient()
	}
}

func (cm *ConnectionManager) ReturnConnection(client *http.Client) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.activeConns--

	select {
	case cm.idleConns <- client:
		// Connection returned to pool
	default:
		// Pool is full, let the connection be garbage collected
	}
}

func (cm *ConnectionManager) Close() {
	close(cm.idleConns)
	for client := range cm.idleConns {
		client.CloseIdleConnections()
	}
}

func (cm *ConnectionManager) Stats() ConnectionStats {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	return ConnectionStats{
		MaxConnections:    cm.maxConnections,
		ActiveConnections: cm.activeConns,
		IdleConnections:   len(cm.idleConns),
	}
}

type ConnectionStats struct {
	MaxConnections    int `json:"max_connections"`
	ActiveConnections int `json:"active_connections"`
	IdleConnections   int `json:"idle_connections"`
}

// Helper functions
func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64Encode([]byte(auth))
}

func base64Encode(data []byte) string {
	const base64Table = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

	if len(data) == 0 {
		return ""
	}

	// Calculate output length
	outputLen := ((len(data) + 2) / 3) * 4
	output := make([]byte, outputLen)

	for i, j := 0, 0; i < len(data); i, j = i+3, j+4 {
		b := uint32(data[i]) << 16
		if i+1 < len(data) {
			b |= uint32(data[i+1]) << 8
		}
		if i+2 < len(data) {
			b |= uint32(data[i+2])
		}

		output[j] = base64Table[(b>>18)&63]
		output[j+1] = base64Table[(b>>12)&63]
		if i+1 < len(data) {
			output[j+2] = base64Table[(b>>6)&63]
		} else {
			output[j+2] = '='
		}
		if i+2 < len(data) {
			output[j+3] = base64Table[b&63]
		} else {
			output[j+3] = '='
		}
	}

	return string(output)
}
