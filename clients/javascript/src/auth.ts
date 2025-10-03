/**
 * Authentication providers for MantisDB JavaScript client
 */

import axios, { AxiosInstance } from 'axios';

// Authentication token interface
export interface AuthToken {
  accessToken: string;
  refreshToken?: string;
  tokenType: string;
  expiresIn: number;
  expiresAt: number;
  scope?: string;
}

// Abstract authentication provider
export abstract class AuthProvider {
  abstract authenticate(httpClient: AxiosInstance, baseURL: string): Promise<AuthToken>;
  abstract refreshToken(httpClient: AxiosInstance, baseURL: string, token: AuthToken): Promise<AuthToken>;
  abstract getAuthHeaders(token: AuthToken): Record<string, string>;
}

// Basic authentication provider
export class BasicAuthProvider extends AuthProvider {
  constructor(private username: string, private password: string) {
    super();
  }

  async authenticate(httpClient: AxiosInstance, baseURL: string): Promise<AuthToken> {
    // Basic auth doesn't use tokens, return a dummy long-lived token
    return {
      accessToken: 'basic_auth',
      tokenType: 'Basic',
      expiresIn: 86400, // 24 hours
      expiresAt: Date.now() + 86400 * 1000,
    };
  }

  async refreshToken(httpClient: AxiosInstance, baseURL: string, token: AuthToken): Promise<AuthToken> {
    return token; // Basic auth doesn't need refresh
  }

  getAuthHeaders(token: AuthToken): Record<string, string> {
    const credentials = btoa(`${this.username}:${this.password}`);
    return {
      Authorization: `Basic ${credentials}`,
    };
  }
}

// API Key authentication provider
export class APIKeyAuthProvider extends AuthProvider {
  constructor(private apiKey: string, private headerName: string = 'X-API-Key') {
    super();
  }

  async authenticate(httpClient: AxiosInstance, baseURL: string): Promise<AuthToken> {
    return {
      accessToken: this.apiKey,
      tokenType: 'ApiKey',
      expiresIn: 86400,
      expiresAt: Date.now() + 86400 * 1000,
    };
  }

  async refreshToken(httpClient: AxiosInstance, baseURL: string, token: AuthToken): Promise<AuthToken> {
    return token;
  }

  getAuthHeaders(token: AuthToken): Record<string, string> {
    return {
      [this.headerName]: this.apiKey,
    };
  }
}

// JWT authentication provider using OAuth2 client credentials flow
export class JWTAuthProvider extends AuthProvider {
  constructor(
    private clientId: string,
    private clientSecret: string,
    private tokenUrl?: string,
    private scope: string = 'read write'
  ) {
    super();
  }

  async authenticate(httpClient: AxiosInstance, baseURL: string): Promise<AuthToken> {
    const tokenUrl = this.tokenUrl || `${baseURL}/oauth/token`;

    const data = new URLSearchParams({
      grant_type: 'client_credentials',
      client_id: this.clientId,
      client_secret: this.clientSecret,
      scope: this.scope,
    });

    const response = await httpClient.post(tokenUrl, data, {
      headers: {
        'Content-Type': 'application/x-www-form-urlencoded',
      },
    });

    const tokenData = response.data;
    return {
      accessToken: tokenData.access_token,
      refreshToken: tokenData.refresh_token,
      tokenType: tokenData.token_type || 'Bearer',
      expiresIn: tokenData.expires_in,
      expiresAt: Date.now() + tokenData.expires_in * 1000,
      scope: tokenData.scope,
    };
  }

  async refreshToken(httpClient: AxiosInstance, baseURL: string, token: AuthToken): Promise<AuthToken> {
    if (!token.refreshToken) {
      // No refresh token, get a new one
      return this.authenticate(httpClient, baseURL);
    }

    const tokenUrl = this.tokenUrl || `${baseURL}/oauth/token`;

    const data = new URLSearchParams({
      grant_type: 'refresh_token',
      refresh_token: token.refreshToken,
      client_id: this.clientId,
      client_secret: this.clientSecret,
    });

    try {
      const response = await httpClient.post(tokenUrl, data, {
        headers: {
          'Content-Type': 'application/x-www-form-urlencoded',
        },
      });

      const tokenData = response.data;
      return {
        accessToken: tokenData.access_token,
        refreshToken: tokenData.refresh_token || token.refreshToken,
        tokenType: tokenData.token_type || 'Bearer',
        expiresIn: tokenData.expires_in,
        expiresAt: Date.now() + tokenData.expires_in * 1000,
        scope: tokenData.scope,
      };
    } catch (error) {
      // Refresh failed, get a new token
      return this.authenticate(httpClient, baseURL);
    }
  }

  getAuthHeaders(token: AuthToken): Record<string, string> {
    return {
      Authorization: `Bearer ${token.accessToken}`,
    };
  }
}

// Authentication manager
export class AuthManager {
  private token: AuthToken | null = null;

  constructor(private provider: AuthProvider) {}

  async getAuthHeaders(httpClient: AxiosInstance, baseURL: string): Promise<Record<string, string>> {
    // Check if we need to authenticate or refresh
    if (!this.token || this.isTokenExpired(this.token)) {
      if (!this.token) {
        this.token = await this.provider.authenticate(httpClient, baseURL);
      } else {
        this.token = await this.provider.refreshToken(httpClient, baseURL, this.token);
      }
    }

    return this.provider.getAuthHeaders(this.token);
  }

  private isTokenExpired(token: AuthToken, bufferSeconds: number = 30): boolean {
    return Date.now() >= token.expiresAt - bufferSeconds * 1000;
  }

  async refreshAuth(httpClient: AxiosInstance, baseURL: string): Promise<void> {
    if (this.token) {
      this.token = await this.provider.refreshToken(httpClient, baseURL, this.token);
    } else {
      this.token = await this.provider.authenticate(httpClient, baseURL);
    }
  }

  clearToken(): void {
    this.token = null;
  }
}

// Connection manager for failover support
export class ConnectionManager {
  private currentHostIndex: number = 0;
  private allHosts: string[];

  constructor(private config: any) {
    this.allHosts = [`${config.host}:${config.port}`];
    
    if (config.failoverHosts && config.failoverHosts.length > 0) {
      this.allHosts.push(...config.failoverHosts);
    }
  }

  getCurrentBaseURL(): string {
    const hostPort = this.allHosts[this.currentHostIndex];
    const [host, port] = this.parseHostPort(hostPort);
    const scheme = this.config.tlsEnabled ? 'https' : 'http';
    return `${scheme}://${host}:${port}`;
  }

  failover(): boolean {
    if (this.allHosts.length <= 1) {
      return false;
    }

    this.currentHostIndex = (this.currentHostIndex + 1) % this.allHosts.length;
    return true;
  }

  resetToPrimary(): void {
    this.currentHostIndex = 0;
  }

  private parseHostPort(hostPort: string): [string, number] {
    const lastColonIndex = hostPort.lastIndexOf(':');
    if (lastColonIndex !== -1) {
      const host = hostPort.substring(0, lastColonIndex);
      const portStr = hostPort.substring(lastColonIndex + 1);
      const port = parseInt(portStr, 10);
      return [host, isNaN(port) ? this.config.port : port];
    }
    return [hostPort, this.config.port];
  }

  getConnectionStats(): Record<string, any> {
    return {
      currentHost: this.allHosts[this.currentHostIndex],
      totalHosts: this.allHosts.length,
      currentHostIndex: this.currentHostIndex,
    };
  }
}

// Health check result interface
export interface HealthCheckResult {
  timestamp: Date;
  status: 'healthy' | 'unhealthy' | 'degraded';
  host: string;
  port: number;
  duration: number;
  error?: string;
  authError?: string;
  connectionStats: Record<string, any>;
}