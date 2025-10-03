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

    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`;
    }

    try {
      const response = await fetch(url, {
        ...options,
        headers,
      });

      const data = await response.json();

      if (!response.ok) {
        return {
          success: false,
          error: data.error || `HTTP ${response.status}: ${response.statusText}`,
        };
      }

      return {
        success: true,
        data,
      };
    } catch (error) {
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
    return this.request<SystemStats>('/system/stats');
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
    return this.request<{ logs: LogEntry[]; total: number }>('/logs/search', {
      method: 'POST',
      body: JSON.stringify(filter),
    });
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

// Create default client instance
export const apiClient = new AdminApiClient();

// Hook for React components
export function useApiClient() {
  return apiClient;
}