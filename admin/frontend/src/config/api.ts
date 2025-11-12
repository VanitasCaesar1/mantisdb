/**
 * API Configuration
 * 
 * Simplified configuration for reliable API connectivity.
 * In development, uses Vite proxy. In production, uses same origin.
 */

export interface ApiConfig {
  baseUrl: string;
}

/**
 * Get API configuration based on environment
 */
export function getApiConfig(): ApiConfig {
  const hostname = window.location.hostname;
  const protocol = window.location.protocol;
  
  // In development with Vite proxy, use relative URLs
  if (import.meta.env.DEV) {
    return {
      baseUrl: '', // Empty string means same origin, Vite will proxy to backend
    };
  }
  
  // In production, the admin UI is served by the same server
  if (hostname !== 'localhost' && hostname !== '127.0.0.1') {
    // Production: Use same host and port as the UI
    const port = parseInt(window.location.port) || (protocol === 'https:' ? 443 : 80);
    const portStr = port === 80 || port === 443 ? '' : `:${port}`;
    return {
      baseUrl: `${protocol}//${hostname}${portStr}`,
    };
  }
  
  // Local production build: Use localhost:8081 (Rust admin server)
  return {
    baseUrl: 'http://localhost:8081',
  };
}

/**
 * Get the base URL for API calls
 */
let cachedConfig: ApiConfig | null = null;

export function getBaseUrl(): string {
  if (!cachedConfig) {
    cachedConfig = getApiConfig();
    console.log(`🔗 API Client configured with base URL: ${cachedConfig.baseUrl || '(same origin)'}`);
  }
  return cachedConfig.baseUrl;
}

/**
 * Reset cached configuration (useful for testing or reconnection)
 */
export function resetApiConfig(): void {
  cachedConfig = null;
}

/**
 * Build a full API URL
 */
export function buildApiUrl(endpoint: string): string {
  const baseUrl = getBaseUrl();
  const path = endpoint.startsWith('/') ? endpoint : `/${endpoint}`;
  return `${baseUrl}${path}`;
}
