/**
 * API Configuration
 * 
 * Dynamically detects the correct API endpoint based on environment
 * and running services. Avoids hardcoded ports.
 */

export interface ApiConfig {
  baseUrl: string;
  adminPort: number;
  apiPort: number;
}

/**
 * Detect which port the Rust admin-server is running on
 * by trying common ports in order
 */
async function detectAdminPort(): Promise<number> {
  const portsToTry = [8081, 8082, 8083, 8084, 8085, 3001, 3002];
  
  for (const port of portsToTry) {
    try {
      const response = await fetch(`http://localhost:${port}/api/health`, {
        method: 'GET',
        signal: AbortSignal.timeout(1000), // 1 second timeout
      });
      
      if (response.ok) {
        console.log(`✅ Detected Rust admin-server on port ${port}`);
        return port;
      }
    } catch (error) {
      // Port not available or server not responding, try next
      continue;
    }
  }
  
  // Fallback to default
  console.warn('⚠️  Could not detect admin server port, using default 8081');
  return 8081;
}

/**
 * Get API configuration based on environment
 */
export async function getApiConfig(): Promise<ApiConfig> {
  const hostname = window.location.hostname;
  const protocol = window.location.protocol;
  
  // In production, the admin UI is served by the same server
  if (hostname !== 'localhost' && hostname !== '127.0.0.1') {
    // Production: Use same host and port as the UI
    const port = parseInt(window.location.port) || (protocol === 'https:' ? 443 : 80);
    return {
      baseUrl: `${protocol}//${hostname}:${port}`,
      adminPort: port,
      apiPort: port,
    };
  }
  
  // Development: Detect the admin server port
  const adminPort = await detectAdminPort();
  
  return {
    baseUrl: `http://localhost:${adminPort}`,
    adminPort: adminPort,
    apiPort: 8080, // Go API server (if needed)
  };
}

/**
 * Get the base URL for API calls
 * Uses cached config if available, otherwise detects
 */
let cachedConfig: ApiConfig | null = null;

export async function getBaseUrl(): Promise<string> {
  if (!cachedConfig) {
    cachedConfig = await getApiConfig();
  }
  return cachedConfig.baseUrl;
}

/**
 * Get the admin port
 */
export async function getAdminPort(): Promise<number> {
  if (!cachedConfig) {
    cachedConfig = await getApiConfig();
  }
  return cachedConfig.adminPort;
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
export async function buildApiUrl(endpoint: string): Promise<string> {
  const baseUrl = await getBaseUrl();
  const path = endpoint.startsWith('/') ? endpoint : `/${endpoint}`;
  return `${baseUrl}${path}`;
}
