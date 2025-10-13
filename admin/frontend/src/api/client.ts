import type { 
  SystemStats, 
  TableInfo, 
  QueryRequest, 
  QueryResponse, 
  BackupInfo, 
  BackupRequest, 
  LogEntry, 
  LogFilter 
} from '../types';

export interface ApiResponse<T = any> {
  success: boolean;
  data?: T;
  error?: string;
  message?: string;
}

export class AdminApiClient {
  private baseUrl: string;
  private token?: string;

  constructor(baseUrl: string = '', token?: string) {
    this.baseUrl = baseUrl;
    this.token = token;
  }

  private async request<T>(
    endpoint: string, 
    options: RequestInit = {}
  ): Promise<ApiResponse<T>> {
    const url = `${this.baseUrl}/api${endpoint}`;
    
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(options.headers as Record<string, string>),
    };

    // Always include token from memory or localStorage
    const token = this.token || (typeof window !== 'undefined' ? localStorage.getItem('mantisdb_token') || undefined : undefined);
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }

    try {
      const response = await fetch(url, {
        ...options,
        headers,
      });

      // Check if response has content before parsing JSON
      const contentType = response.headers.get('content-type');
      let data: any = null;
      
      if (contentType && contentType.includes('application/json')) {
        const text = await response.text();
        if (text && text.trim().length > 0) {
          try {
            data = JSON.parse(text);
          } catch (parseError) {
            console.error('JSON parse error:', parseError, 'Response text:', text);
            return {
              success: false,
              error: `Invalid JSON response: ${text.substring(0, 100)}`,
            };
          }
        }
      }

      if (!response.ok) {
        return {
          success: false,
          error: data?.error || `HTTP ${response.status}: ${response.statusText}`,
        };
      }

      return {
        success: true,
        data,
      };
    } catch (error) {
      console.error('API request error:', error);
      return {
        success: false,
        error: error instanceof Error ? error.message : 'Network error',
      };
    }
  }

  // Health and system endpoints
  async getHealth(): Promise<ApiResponse<{ status: string; timestamp: string; version: string; database: any }>> {
    return this.request('/health');
  }

  async getMetrics() {
    return this.request<{ metrics: any; timestamp: string }>('/metrics');
  }

  async getSystemStats() {
    return this.request<SystemStats>('/stats');
  }

  async getConfig() {
    return this.request('/config');
  }

  async updateConfig(config: Record<string, any>) {
    return this.request('/config', {
      method: 'PUT',
      body: JSON.stringify(config),
    });
  }

  // Table management endpoints
  async getTables() {
    return this.request<{ tables: TableInfo[]; total: number }>('/tables');
  }

  async createTable(name: string, type: string, columns: any[]) {
    return this.request('/tables/create', {
      method: 'POST',
      body: JSON.stringify({ name, type, columns }),
    });
  }

  // Columnar table endpoints
  async getColumnarTables(): Promise<ApiResponse<{ tables: TableInfo[]; count?: number }>> {
    return this.request<{ tables: TableInfo[]; count?: number }>('/columnar/tables');
  }

  async createColumnarTable(name: string, columns: any[], partition_key?: string[]): Promise<ApiResponse<{ table: any }>> {
    return this.request<{ table: any }>(`/columnar/tables`, {
      method: 'POST',
      body: JSON.stringify({ name, columns, partition_key }),
    });
  }

  async queryColumnarTable(
    table: string,
    body: Record<string, any>
  ): Promise<ApiResponse<{ rows: any[]; total?: number; limit?: number; offset?: number; has_more?: boolean }>> {
    return this.request<{ rows: any[]; total?: number; limit?: number; offset?: number; has_more?: boolean }>(`/columnar/tables/${table}/query`, {
      method: 'POST',
      body: JSON.stringify(body),
    });
  }

  async insertColumnarRows(
    table: string,
    rows: Record<string, any>[]
  ): Promise<ApiResponse<{ rows_inserted?: number }>> {
    return this.request<{ rows_inserted?: number }>(`/columnar/tables/${table}/rows`, {
      method: 'POST',
      body: JSON.stringify({ rows }),
    });
  }

  async deleteColumnarRows(
    table: string,
    filters: Record<string, any>
  ): Promise<ApiResponse<{ rows_affected?: number }>> {
    return this.request<{ rows_affected?: number }>(`/columnar/tables/${table}/delete`, {
      method: 'POST',
      body: JSON.stringify(filters),
    });
  }

  async executeCql(statement: string, params?: any[]): Promise<ApiResponse<{ rows?: any[] }>> {
    return this.request<{ rows?: any[] }>(`/columnar/cql`, {
      method: 'POST',
      body: JSON.stringify({ statement, params }),
    });
  }

  async getTableData(
    table: string, 
    options: { 
      limit?: number; 
      offset?: number; 
      type?: string; 
    } = {}
  ): Promise<ApiResponse<{ data: any[]; total_count: number; limit: number; offset: number; table: string; type: string }>> {
    const params = new URLSearchParams();
    if (options.limit) params.set('limit', options.limit.toString());
    if (options.offset) params.set('offset', options.offset.toString());
    if (options.type) params.set('type', options.type);

    const query = params.toString() ? `?${params.toString()}` : '';
    return this.request(`/tables/${table}/data${query}`);
  }

  async getTableSchema(table: string) {
    return this.request<{ success: boolean; columns: any[] }>(`/tables/${table}/schema`);
  }

  async updateTableSchema(table: string, columns: any[]) {
    return this.request(`/tables/${table}/schema`, {
      method: 'PUT',
      body: JSON.stringify({ columns }),
    });
  }

  async createTableData(table: string, data: Record<string, any>, type?: string) {
    const params = type ? `?type=${type}` : '';
    return this.request(`/tables/${table}/data${params}`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async updateTableData(
    table: string, 
    id: string, 
    data: Record<string, any>, 
    type?: string
  ) {
    const params = type ? `?type=${type}` : '';
    return this.request(`/tables/${table}/data/${id}${params}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async deleteTableData(table: string, id: string, type?: string) {
    const params = type ? `?type=${type}` : '';
    return this.request(`/tables/${table}/data/${id}${params}`, {
      method: 'DELETE',
    });
  }

  // Query endpoints
  async executeQuery(queryRequest: QueryRequest) {
    return this.request<QueryResponse>('/query', {
      method: 'POST',
      body: JSON.stringify(queryRequest),
    });
  }

  async getQueryHistory(limit?: number) {
    const params = limit ? `?limit=${limit}` : '';
    return this.request(`/query/history${params}`);
  }

  // Backup endpoints
  async getBackups() {
    return this.request<{ backups: BackupInfo[]; total: number }>('/backups');
  }

  async createBackup(request: BackupRequest) {
    return this.request('/backups', {
      method: 'POST',
      body: JSON.stringify(request),
    });
  }

  async getBackupStatus(backupId: string) {
    return this.request<{ backup: BackupInfo }>(`/backups/${backupId}`);
  }

  async deleteBackup(backupId: string) {
    return this.request(`/backups/${backupId}`, {
      method: 'DELETE',
    });
  }

  async restoreBackup(backupId: string, options: { target_path?: string; overwrite?: boolean } = {}) {
    return this.request(`/backups/${backupId}/restore`, {
      method: 'POST',
      body: JSON.stringify(options),
    });
  }

  // Log endpoints
  async getLogs(filter?: LogFilter) {
    const params = new URLSearchParams();
    if (filter?.level) params.set('level', filter.level);
    if (filter?.component) params.set('component', filter.component);
    if (filter?.limit) params.set('limit', filter.limit.toString());
    if (filter?.offset) params.set('offset', filter.offset.toString());

    const query = params.toString() ? `?${params.toString()}` : '';
    return this.request<{ logs: LogEntry[]; total: number }>(`/logs${query}`);
  }

  async searchLogs(filter: LogFilter) {
    return this.request<{ results: LogEntry[]; total: number; has_more: boolean }>('/logs/search', {
      method: 'POST',
      body: JSON.stringify(filter),
    });
  }

  // Storage endpoints
  async listStorage(path?: string) {
    const params = new URLSearchParams();
    if (path) params.set('path', path);
    const query = params.toString() ? `?${params.toString()}` : '';
    return this.request<{ files: any[]; total: number }>(`/storage/list${query}`);
  }

  getStorageDownloadUrl(path: string) {
    // Build absolute URL for download
    return `${this.baseUrl}/api/storage/download?path=${encodeURIComponent(path)}`;
  }

  // WebSocket connections for real-time updates
  createMetricsWebSocket(onMessage: (data: any) => void, onError?: (error: Event) => void) {
    return this.createEventSource('/ws/metrics', onMessage, onError);
  }

  createLogsWebSocket(onMessage: (data: any) => void, onError?: (error: Event) => void) {
    return this.createEventSource('/ws/logs', onMessage, onError);
  }

  createEventsWebSocket(onMessage: (data: any) => void, onError?: (error: Event) => void) {
    return this.createEventSource('/ws/events', onMessage, onError);
  }

  private createEventSource(
    endpoint: string, 
    onMessage: (data: any) => void, 
    onError?: (error: Event) => void
  ): EventSource {
    const url = `${this.baseUrl}/api${endpoint}`;
    const eventSource = new EventSource(url);

    eventSource.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        onMessage(data);
      } catch (error) {
        console.error('Failed to parse WebSocket message:', error);
      }
    };

    if (onError) {
      eventSource.onerror = onError;
    }

    return eventSource;
  }
}

// Import dynamic API configuration
import { getBaseUrl } from '../config/api';

// Create singleton client instance
let apiClientInstance: AdminApiClient | null = null;

function getLazyClient(): AdminApiClient {
  if (!apiClientInstance) {
    const baseUrl = getBaseUrl();
    apiClientInstance = new AdminApiClient(baseUrl);
  }
  return apiClientInstance;
}

// Export singleton API client
export const apiClient = getLazyClient();

// Hook for React components
export function useApiClient() {
  return apiClient;
}