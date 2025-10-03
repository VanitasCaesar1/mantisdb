/**
 * MantisDB JavaScript/TypeScript Client Library
 * 
 * Official client for MantisDB supporting Node.js and browser environments.
 */

import axios, { AxiosInstance, AxiosRequestConfig, AxiosResponse } from 'axios';
import { 
  AuthProvider, 
  BasicAuthProvider, 
  APIKeyAuthProvider, 
  JWTAuthProvider,
  AuthManager, 
  ConnectionManager,
  HealthCheckResult
} from './auth';

// Configuration interfaces
export interface ClientConfig {
  host: string;
  port: number;
  username?: string;
  password?: string;
  apiKey?: string;
  clientId?: string;
  clientSecret?: string;
  tokenUrl?: string;
  authProvider?: AuthProvider;
  maxConnections?: number;
  timeout?: number;
  retryAttempts?: number;
  retryDelay?: number;
  enableCompression?: boolean;
  tlsEnabled?: boolean;
  enableFailover?: boolean;
  failoverHosts?: string[];
}

export interface DefaultClientConfig extends Required<Omit<ClientConfig, 'authProvider'>> {
  authProvider?: AuthProvider;
}

// Result interfaces
export interface QueryResult {
  rows: Record<string, any>[];
  columns: string[];
  rowCount: number;
  metadata?: Record<string, any>;
}

export interface TransactionResponse {
  transactionId: string;
}

// Error interfaces
export interface MantisErrorDetails {
  code: string;
  message: string;
  details?: Record<string, any>;
  requestId?: string;
}

export interface ErrorResponse {
  error: MantisErrorDetails;
}

// Request interfaces
export interface QueryRequest {
  sql: string;
}

export interface InsertRequest {
  table: string;
  data: Record<string, any>;
}

export interface UpdateRequest {
  data: Record<string, any>;
}

// Custom error class
export class MantisError extends Error {
  public readonly code: string;
  public readonly details?: Record<string, any>;
  public readonly requestId?: string;

  constructor(code: string, message: string, details?: Record<string, any>, requestId?: string) {
    super(message);
    this.name = 'MantisError';
    this.code = code;
    this.details = details;
    this.requestId = requestId;
  }

  toString(): string {
    if (this.requestId) {
      return `MantisError [${this.code}] (request: ${this.requestId}): ${this.message}`;
    }
    return `MantisError [${this.code}]: ${this.message}`;
  }
}

// Transaction class
export class Transaction {
  private id: string;
  private client: MantisClient;
  private closed: boolean = false;

  constructor(id: string, client: MantisClient) {
    this.id = id;
    this.client = client;
  }

  async query(sql: string): Promise<QueryResult> {
    if (this.closed) {
      throw new MantisError('TRANSACTION_CLOSED', 'Transaction is closed');
    }

    return this.client.executeTransactionQuery(this.id, sql);
  }

  async insert(table: string, data: Record<string, any>): Promise<void> {
    if (this.closed) {
      throw new MantisError('TRANSACTION_CLOSED', 'Transaction is closed');
    }

    return this.client.executeTransactionInsert(this.id, table, data);
  }

  async update(table: string, id: string, data: Record<string, any>): Promise<void> {
    if (this.closed) {
      throw new MantisError('TRANSACTION_CLOSED', 'Transaction is closed');
    }

    return this.client.executeTransactionUpdate(this.id, table, id, data);
  }

  async delete(table: string, id: string): Promise<void> {
    if (this.closed) {
      throw new MantisError('TRANSACTION_CLOSED', 'Transaction is closed');
    }

    return this.client.executeTransactionDelete(this.id, table, id);
  }

  async commit(): Promise<void> {
    if (this.closed) {
      throw new MantisError('TRANSACTION_CLOSED', 'Transaction is already closed');
    }

    await this.client.commitTransaction(this.id);
    this.closed = true;
  }

  async rollback(): Promise<void> {
    if (this.closed) {
      throw new MantisError('TRANSACTION_CLOSED', 'Transaction is already closed');
    }

    await this.client.rollbackTransaction(this.id);
    this.closed = true;
  }

  get isClosed(): boolean {
    return this.closed;
  }

  get transactionId(): string {
    return this.id;
  }
}

// Main client class
export class MantisClient {
  private config: DefaultClientConfig;
  private httpClient: AxiosInstance;
  private baseURL: string;
  private authManager?: AuthManager;
  private connectionManager: ConnectionManager;

  constructor(config: ClientConfig) {
    this.config = this.mergeWithDefaults(config);
    this.baseURL = this.buildBaseURL();
    this.httpClient = this.createHttpClient();
    this.setupAuth();
    this.connectionManager = new ConnectionManager(this.config);
  }

  private mergeWithDefaults(config: ClientConfig): DefaultClientConfig {
    return {
      host: config.host,
      port: config.port,
      username: config.username || '',
      password: config.password || '',
      apiKey: config.apiKey || '',
      clientId: config.clientId || '',
      clientSecret: config.clientSecret || '',
      tokenUrl: config.tokenUrl || '',
      authProvider: config.authProvider,
      maxConnections: config.maxConnections || 10,
      timeout: config.timeout || 60000,
      retryAttempts: config.retryAttempts || 3,
      retryDelay: config.retryDelay || 1000,
      enableCompression: config.enableCompression !== false,
      tlsEnabled: config.tlsEnabled || false,
      enableFailover: config.enableFailover || false,
      failoverHosts: config.failoverHosts || [],
    };
  }

  private setupAuth(): void {
    let authProvider: AuthProvider | undefined;

    if (this.config.authProvider) {
      authProvider = this.config.authProvider;
    } else if (this.config.apiKey) {
      authProvider = new APIKeyAuthProvider(this.config.apiKey);
    } else if (this.config.clientId && this.config.clientSecret) {
      authProvider = new JWTAuthProvider(
        this.config.clientId,
        this.config.clientSecret,
        this.config.tokenUrl
      );
    } else if (this.config.username && this.config.password) {
      authProvider = new BasicAuthProvider(this.config.username, this.config.password);
    }

    if (authProvider) {
      this.authManager = new AuthManager(authProvider);
    }
  }

  private buildBaseURL(): string {
    const scheme = this.config.tlsEnabled ? 'https' : 'http';
    return `${scheme}://${this.config.host}:${this.config.port}`;
  }

  private createHttpClient(): AxiosInstance {
    const client = axios.create({
      baseURL: this.baseURL,
      timeout: this.config.timeout,
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json',
        'User-Agent': 'MantisDB-JS-Client/1.0',
      },
    });

    // Add compression support
    if (this.config.enableCompression) {
      client.defaults.headers.common['Accept-Encoding'] = 'gzip, deflate';
    }

    // Add authentication
    if (this.config.username && this.config.password) {
      client.defaults.auth = {
        username: this.config.username,
        password: this.config.password,
      };
    }

    // Add response interceptor for error handling
    client.interceptors.response.use(
      (response) => response,
      (error) => {
        if (error.response?.data?.error) {
          const errorData = error.response.data.error;
          throw new MantisError(
            errorData.code || 'UNKNOWN_ERROR',
            errorData.message || 'Unknown error occurred',
            errorData.details,
            errorData.requestId
          );
        }
        throw new MantisError(
          'HTTP_ERROR',
          error.message || 'HTTP request failed'
        );
      }
    );

    return client;
  }

  private async makeRequestWithRetry<T>(
    requestConfig: AxiosRequestConfig
  ): Promise<AxiosResponse<T>> {
    let lastError: any;

    // Update base URL from connection manager
    const currentBaseURL = this.connectionManager.getCurrentBaseURL();
    requestConfig.baseURL = currentBaseURL;

    // Add authentication headers
    if (this.authManager) {
      try {
        const authHeaders = await this.authManager.getAuthHeaders(this.httpClient, currentBaseURL);
        requestConfig.headers = {
          ...requestConfig.headers,
          ...authHeaders,
        };
      } catch (authError) {
        console.warn('Authentication failed:', authError);
      }
    }

    for (let attempt = 0; attempt <= this.config.retryAttempts; attempt++) {
      try {
        if (attempt > 0) {
          await this.delay(this.config.retryDelay * attempt);
        }

        const response = await this.httpClient.request<T>(requestConfig);
        
        // Reset to primary host on successful request
        if (attempt > 0) {
          this.connectionManager.resetToPrimary();
        }
        
        return response;
      } catch (error: any) {
        lastError = error;
        
        // Try failover on connection errors
        if (this.config.enableFailover && 
            attempt === 0 && 
            this.isConnectionError(error) &&
            this.connectionManager.failover()) {
          const newBaseURL = this.connectionManager.getCurrentBaseURL();
          requestConfig.baseURL = newBaseURL;
          continue;
        }
        
        // Don't retry on client errors (4xx) or MantisError
        if (error instanceof MantisError || 
            (error.response && error.response.status < 500)) {
          throw error;
        }
      }
    }

    throw lastError;
  }

  private isConnectionError(error: any): boolean {
    return !error.response || error.code === 'ECONNREFUSED' || error.code === 'ENOTFOUND';
  }

  private delay(ms: number): Promise<void> {
    return new Promise(resolve => setTimeout(resolve, ms));
  }

  // Public API methods
  async ping(): Promise<void> {
    await this.makeRequestWithRetry({
      method: 'GET',
      url: '/api/health',
    });
  }

  async query(sql: string): Promise<QueryResult> {
    const response = await this.makeRequestWithRetry<QueryResult>({
      method: 'POST',
      url: '/api/query',
      data: { sql } as QueryRequest,
    });

    return response.data;
  }

  async insert(table: string, data: Record<string, any>): Promise<void> {
    await this.makeRequestWithRetry({
      method: 'POST',
      url: `/api/tables/${table}/data`,
      data: { table, data } as InsertRequest,
    });
  }

  async update(table: string, id: string, data: Record<string, any>): Promise<void> {
    await this.makeRequestWithRetry({
      method: 'PUT',
      url: `/api/tables/${table}/data/${id}`,
      data: { data } as UpdateRequest,
    });
  }

  async delete(table: string, id: string): Promise<void> {
    await this.makeRequestWithRetry({
      method: 'DELETE',
      url: `/api/tables/${table}/data/${id}`,
    });
  }

  async get(table: string, filters?: Record<string, any>): Promise<QueryResult> {
    const response = await this.makeRequestWithRetry<QueryResult>({
      method: 'GET',
      url: `/api/tables/${table}/data`,
      params: filters,
    });

    return response.data;
  }

  async beginTransaction(): Promise<Transaction> {
    const response = await this.makeRequestWithRetry<TransactionResponse>({
      method: 'POST',
      url: '/api/transactions',
    });

    return new Transaction(response.data.transactionId, this);
  }

  // Transaction helper methods (used by Transaction class)
  async executeTransactionQuery(txId: string, sql: string): Promise<QueryResult> {
    const response = await this.makeRequestWithRetry<QueryResult>({
      method: 'POST',
      url: `/api/transactions/${txId}/query`,
      data: { sql } as QueryRequest,
    });

    return response.data;
  }

  async executeTransactionInsert(txId: string, table: string, data: Record<string, any>): Promise<void> {
    await this.makeRequestWithRetry({
      method: 'POST',
      url: `/api/transactions/${txId}/tables/${table}/data`,
      data: { table, data } as InsertRequest,
    });
  }

  async executeTransactionUpdate(txId: string, table: string, id: string, data: Record<string, any>): Promise<void> {
    await this.makeRequestWithRetry({
      method: 'PUT',
      url: `/api/transactions/${txId}/tables/${table}/data/${id}`,
      data: { data } as UpdateRequest,
    });
  }

  async executeTransactionDelete(txId: string, table: string, id: string): Promise<void> {
    await this.makeRequestWithRetry({
      method: 'DELETE',
      url: `/api/transactions/${txId}/tables/${table}/data/${id}`,
    });
  }

  async commitTransaction(txId: string): Promise<void> {
    await this.makeRequestWithRetry({
      method: 'POST',
      url: `/api/transactions/${txId}/commit`,
    });
  }

  async rollbackTransaction(txId: string): Promise<void> {
    await this.makeRequestWithRetry({
      method: 'POST',
      url: `/api/transactions/${txId}/rollback`,
    });
  }

  async close(): Promise<void> {
    // Axios doesn't require explicit cleanup, but we can clear any pending requests
    // In a real implementation, you might want to cancel pending requests
  }

  // Utility methods
  getConfig(): Readonly<DefaultClientConfig> {
    return { ...this.config };
  }

  getBaseURL(): string {
    return this.connectionManager.getCurrentBaseURL();
  }

  // Authentication methods
  async refreshAuth(): Promise<void> {
    if (!this.authManager) {
      throw new MantisError('NO_AUTH_PROVIDER', 'No authentication provider configured');
    }
    
    await this.authManager.refreshAuth(this.httpClient, this.getBaseURL());
  }

  setAuthProvider(provider: AuthProvider): void {
    this.authManager = new AuthManager(provider);
  }

  clearAuth(): void {
    if (this.authManager) {
      this.authManager.clearToken();
    }
  }

  // Connection management methods
  getConnectionStats(): Record<string, any> {
    return this.connectionManager.getConnectionStats();
  }

  async failover(): Promise<boolean> {
    if (!this.config.enableFailover) {
      throw new MantisError('FAILOVER_DISABLED', 'Failover is not enabled');
    }
    
    const success = this.connectionManager.failover();
    if (success) {
      // Test the new connection
      try {
        await this.ping();
        return true;
      } catch (error) {
        // Failover host is also down, continue trying others
        return this.failover();
      }
    }
    
    return false;
  }

  resetToPrimaryHost(): void {
    this.connectionManager.resetToPrimary();
  }

  // Health check method
  async healthCheck(): Promise<HealthCheckResult> {
    const start = Date.now();
    
    const result: HealthCheckResult = {
      timestamp: new Date(),
      status: 'healthy',
      host: this.config.host,
      port: this.config.port,
      duration: 0,
      connectionStats: this.getConnectionStats(),
    };

    try {
      // Test basic connectivity
      await this.ping();
      
      // Test authentication if configured
      if (this.authManager) {
        try {
          await this.authManager.getAuthHeaders(this.httpClient, this.getBaseURL());
        } catch (authError) {
          result.status = 'degraded';
          result.authError = authError instanceof Error ? authError.message : String(authError);
        }
      }
      
    } catch (error) {
      result.status = 'unhealthy';
      result.error = error instanceof Error ? error.message : String(error);
    }

    result.duration = Date.now() - start;
    return result;
  }
}

// Convenience function for creating clients
export function createClient(config: ClientConfig): MantisClient {
  return new MantisClient(config);
}

// Connection string parser
export function parseConnectionString(connectionString: string): ClientConfig {
  // Parse connection string format: mantisdb://username:password@host:port
  const url = new URL(connectionString);
  
  return {
    host: url.hostname,
    port: parseInt(url.port) || 8080,
    username: url.username || undefined,
    password: url.password || undefined,
    tlsEnabled: url.protocol === 'mantisdbs:',
  };
}

// Default export
export default MantisClient;

// Re-export types and utilities
export * from './types';
export * from './utils';