/**
 * Utility functions for MantisDB JavaScript client
 */

import { ClientConfig, ConfigValidationResult, DatabaseValue } from './types';

/**
 * Validates client configuration
 */
export function validateConfig(config: ClientConfig): ConfigValidationResult {
  const errors: string[] = [];
  const warnings: string[] = [];

  // Required fields
  if (!config.host) {
    errors.push('Host is required');
  }

  if (!config.port || config.port <= 0 || config.port > 65535) {
    errors.push('Port must be a valid port number (1-65535)');
  }

  // Authentication validation
  const hasBasicAuth = config.username && config.password;
  const hasApiKey = config.apiKey;
  const hasJwtAuth = config.clientId && config.clientSecret;
  const hasCustomAuth = config.authProvider;

  if (!hasBasicAuth && !hasApiKey && !hasJwtAuth && !hasCustomAuth) {
    warnings.push('No authentication method configured');
  }

  // Timeout validation
  if (config.timeout && config.timeout <= 0) {
    errors.push('Timeout must be positive');
  }

  // Retry validation
  if (config.retryAttempts && config.retryAttempts < 0) {
    errors.push('Retry attempts must be non-negative');
  }

  if (config.retryDelay && config.retryDelay < 0) {
    errors.push('Retry delay must be non-negative');
  }

  // Connection pool validation
  if (config.maxConnections && config.maxConnections <= 0) {
    errors.push('Max connections must be positive');
  }

  // Failover validation
  if (config.enableFailover && (!config.failoverHosts || config.failoverHosts.length === 0)) {
    warnings.push('Failover enabled but no failover hosts configured');
  }

  return {
    valid: errors.length === 0,
    errors,
    warnings
  };
}

/**
 * Merges configuration with defaults
 */
export function mergeConfigWithDefaults(config: Partial<ClientConfig>): ClientConfig {
  return {
    host: 'localhost',
    port: 8080,
    username: '',
    password: '',
    apiKey: '',
    clientId: '',
    clientSecret: '',
    tokenUrl: '',
    maxConnections: 10,
    timeout: 60000,
    retryAttempts: 3,
    retryDelay: 1000,
    enableCompression: true,
    tlsEnabled: false,
    enableFailover: false,
    failoverHosts: [],
    ...config
  };
}

/**
 * Parses connection string into configuration
 */
export function parseConnectionString(connectionString: string): Partial<ClientConfig> {
  try {
    const url = new URL(connectionString);
    
    const config: Partial<ClientConfig> = {
      host: url.hostname,
      port: parseInt(url.port) || 8080,
      tlsEnabled: url.protocol === 'mantisdbs:',
    };

    if (url.username) {
      config.username = decodeURIComponent(url.username);
    }

    if (url.password) {
      config.password = decodeURIComponent(url.password);
    }

    // Parse query parameters for additional config
    const params = new URLSearchParams(url.search);
    
    if (params.has('timeout')) {
      config.timeout = parseInt(params.get('timeout')!);
    }

    if (params.has('retryAttempts')) {
      config.retryAttempts = parseInt(params.get('retryAttempts')!);
    }

    if (params.has('maxConnections')) {
      config.maxConnections = parseInt(params.get('maxConnections')!);
    }

    if (params.has('enableCompression')) {
      config.enableCompression = params.get('enableCompression') === 'true';
    }

    return config;
  } catch (error) {
    throw new Error(`Invalid connection string: ${error instanceof Error ? error.message : 'Unknown error'}`);
  }
}

/**
 * Formats connection string from configuration
 */
export function formatConnectionString(config: ClientConfig): string {
  const protocol = config.tlsEnabled ? 'mantisdbs:' : 'mantisdb:';
  let connectionString = `${protocol}//`;

  if (config.username && config.password) {
    connectionString += `${encodeURIComponent(config.username)}:${encodeURIComponent(config.password)}@`;
  }

  connectionString += `${config.host}:${config.port}`;

  // Add query parameters for non-default values
  const params = new URLSearchParams();
  
  if (config.timeout !== 60000) {
    params.set('timeout', config.timeout.toString());
  }

  if (config.retryAttempts !== 3) {
    params.set('retryAttempts', config.retryAttempts.toString());
  }

  if (config.maxConnections !== 10) {
    params.set('maxConnections', config.maxConnections.toString());
  }

  if (!config.enableCompression) {
    params.set('enableCompression', 'false');
  }

  const queryString = params.toString();
  if (queryString) {
    connectionString += `?${queryString}`;
  }

  return connectionString;
}

/**
 * Sanitizes SQL query for logging (removes sensitive data)
 */
export function sanitizeQuery(query: string): string {
  // Remove potential passwords, tokens, etc.
  return query
    .replace(/password\s*=\s*'[^']*'/gi, "password='***'")
    .replace(/password\s*=\s*"[^"]*"/gi, 'password="***"')
    .replace(/token\s*=\s*'[^']*'/gi, "token='***'")
    .replace(/token\s*=\s*"[^"]*"/gi, 'token="***"')
    .replace(/api_key\s*=\s*'[^']*'/gi, "api_key='***'")
    .replace(/api_key\s*=\s*"[^"]*"/gi, 'api_key="***"');
}

/**
 * Formats error message with context
 */
export function formatError(error: Error, context?: Record<string, any>): string {
  let message = error.message;
  
  if (context) {
    const contextStr = Object.entries(context)
      .map(([key, value]) => `${key}=${JSON.stringify(value)}`)
      .join(', ');
    message += ` (${contextStr})`;
  }

  return message;
}

/**
 * Retries an async operation with exponential backoff
 */
export async function retryWithBackoff<T>(
  operation: () => Promise<T>,
  maxAttempts: number = 3,
  baseDelay: number = 1000,
  maxDelay: number = 10000
): Promise<T> {
  let lastError: Error;

  for (let attempt = 1; attempt <= maxAttempts; attempt++) {
    try {
      return await operation();
    } catch (error) {
      lastError = error instanceof Error ? error : new Error(String(error));
      
      if (attempt === maxAttempts) {
        throw lastError;
      }

      // Calculate delay with exponential backoff and jitter
      const delay = Math.min(
        baseDelay * Math.pow(2, attempt - 1) + Math.random() * 1000,
        maxDelay
      );

      await sleep(delay);
    }
  }

  throw lastError!;
}

/**
 * Sleep utility function
 */
export function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms));
}

/**
 * Debounces a function call
 */
export function debounce<T extends (...args: any[]) => any>(
  func: T,
  wait: number
): (...args: Parameters<T>) => void {
  let timeout: NodeJS.Timeout | null = null;

  return (...args: Parameters<T>) => {
    if (timeout) {
      clearTimeout(timeout);
    }

    timeout = setTimeout(() => {
      func(...args);
    }, wait);
  };
}

/**
 * Throttles a function call
 */
export function throttle<T extends (...args: any[]) => any>(
  func: T,
  limit: number
): (...args: Parameters<T>) => void {
  let inThrottle: boolean = false;

  return (...args: Parameters<T>) => {
    if (!inThrottle) {
      func(...args);
      inThrottle = true;
      setTimeout(() => inThrottle = false, limit);
    }
  };
}

/**
 * Deep clones an object
 */
export function deepClone<T>(obj: T): T {
  if (obj === null || typeof obj !== 'object') {
    return obj;
  }

  if (obj instanceof Date) {
    return new Date(obj.getTime()) as unknown as T;
  }

  if (obj instanceof Array) {
    return obj.map(item => deepClone(item)) as unknown as T;
  }

  if (typeof obj === 'object') {
    const cloned = {} as T;
    for (const key in obj) {
      if (obj.hasOwnProperty(key)) {
        cloned[key] = deepClone(obj[key]);
      }
    }
    return cloned;
  }

  return obj;
}

/**
 * Checks if a value is a plain object
 */
export function isPlainObject(value: any): value is Record<string, any> {
  return (
    value !== null &&
    typeof value === 'object' &&
    value.constructor === Object &&
    Object.prototype.toString.call(value) === '[object Object]'
  );
}

/**
 * Converts database value to appropriate JavaScript type
 */
export function convertDatabaseValue(value: any): DatabaseValue {
  if (value === null || value === undefined) {
    return null;
  }

  if (typeof value === 'string') {
    // Try to parse ISO date strings
    if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}/.test(value)) {
      const date = new Date(value);
      if (!isNaN(date.getTime())) {
        return date;
      }
    }
    return value;
  }

  if (typeof value === 'number' || typeof value === 'boolean') {
    return value;
  }

  if (Array.isArray(value)) {
    return value.map(convertDatabaseValue);
  }

  if (isPlainObject(value)) {
    const converted: Record<string, DatabaseValue> = {};
    for (const [key, val] of Object.entries(value)) {
      converted[key] = convertDatabaseValue(val);
    }
    return converted;
  }

  return value;
}

/**
 * Escapes SQL identifier (table name, column name, etc.)
 */
export function escapeIdentifier(identifier: string): string {
  return `"${identifier.replace(/"/g, '""')}"`;
}

/**
 * Escapes SQL string literal
 */
export function escapeString(str: string): string {
  return `'${str.replace(/'/g, "''")}'`;
}

/**
 * Builds WHERE clause from object
 */
export function buildWhereClause(conditions: Record<string, any>): string {
  if (!conditions || Object.keys(conditions).length === 0) {
    return '';
  }

  const clauses = Object.entries(conditions).map(([key, value]) => {
    const escapedKey = escapeIdentifier(key);
    
    if (value === null) {
      return `${escapedKey} IS NULL`;
    }
    
    if (Array.isArray(value)) {
      const escapedValues = value.map(v => 
        typeof v === 'string' ? escapeString(v) : String(v)
      ).join(', ');
      return `${escapedKey} IN (${escapedValues})`;
    }
    
    if (typeof value === 'string') {
      return `${escapedKey} = ${escapeString(value)}`;
    }
    
    return `${escapedKey} = ${value}`;
  });

  return `WHERE ${clauses.join(' AND ')}`;
}

/**
 * Generates a unique request ID
 */
export function generateRequestId(): string {
  return `req_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
}

/**
 * Formats bytes to human readable string
 */
export function formatBytes(bytes: number, decimals: number = 2): string {
  if (bytes === 0) return '0 Bytes';

  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];

  const i = Math.floor(Math.log(bytes) / Math.log(k));

  return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
}

/**
 * Formats duration to human readable string
 */
export function formatDuration(ms: number): string {
  if (ms < 1000) {
    return `${ms}ms`;
  }
  
  if (ms < 60000) {
    return `${(ms / 1000).toFixed(1)}s`;
  }
  
  if (ms < 3600000) {
    return `${(ms / 60000).toFixed(1)}m`;
  }
  
  return `${(ms / 3600000).toFixed(1)}h`;
}

/**
 * Creates a timeout promise that rejects after specified time
 */
export function createTimeoutPromise<T>(ms: number, message?: string): Promise<T> {
  return new Promise((_, reject) => {
    setTimeout(() => {
      reject(new Error(message || `Operation timed out after ${ms}ms`));
    }, ms);
  });
}

/**
 * Races a promise against a timeout
 */
export function withTimeout<T>(promise: Promise<T>, timeoutMs: number, message?: string): Promise<T> {
  return Promise.race([
    promise,
    createTimeoutPromise<T>(timeoutMs, message)
  ]);
}