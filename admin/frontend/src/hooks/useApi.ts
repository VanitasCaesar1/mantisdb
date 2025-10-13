import { useState, useEffect, useCallback, useRef } from 'react';
import { apiClient, type ApiResponse } from '../api/client';

export interface UseApiState<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
  refetch: () => Promise<void>;
}

export function useApi<T>(
  apiCall: () => Promise<ApiResponse<T>>,
  options: { 
    dependencies?: any[];
    enabled?: boolean;
  } = {}
): UseApiState<T> {
  const { dependencies = [], enabled = true } = options;
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const mountedRef = useRef(true);

  const fetchData = useCallback(async () => {
    if (!enabled) {
      setLoading(false);
      return;
    }

    try {
      setLoading(true);
      setError(null);
      
      const response = await apiCall();
      
      // Only update state if component is still mounted
      if (!mountedRef.current) return;
      
      if (response.success && response.data) {
        setData(response.data);
      } else {
        setError(response.error || 'Unknown error occurred');
      }
    } catch (err) {
      if (!mountedRef.current) return;
      setError(err instanceof Error ? err.message : 'Network error');
    } finally {
      if (mountedRef.current) {
        setLoading(false);
      }
    }
  }, [enabled, ...dependencies]);

  useEffect(() => {
    mountedRef.current = true;
    fetchData();
    
    return () => {
      mountedRef.current = false;
    };
  }, [fetchData]);

  return {
    data,
    loading,
    error,
    refetch: fetchData,
  };
}

// Specific hooks for common API calls
export function useHealth() {
  return useApi<{ status: string; timestamp: string; version: string; database: any }>(() => apiClient.getHealth());
}

export function useMetrics() {
  return useApi(() => apiClient.getMetrics());
}

export function useSystemStats() {
  return useApi(() => apiClient.getSystemStats());
}

export function useTables() {
  return useApi(() => apiClient.getTables());
}

export function useTableData(table: string, options: { limit?: number; offset?: number; type?: string } = {}) {
  return useApi<{ data: any[]; total_count: number; limit: number; offset: number; table: string; type: string }>(
    () => apiClient.getTableData(table, options),
    { dependencies: [table, options.limit, options.offset, options.type] }
  );
}

export function useQueryHistory(limit?: number) {
  return useApi(() => apiClient.getQueryHistory(limit), { dependencies: [limit] });
}

export function useBackups() {
  return useApi(() => apiClient.getBackups(), {});
}

export function useLogs(filter?: any) {
  return useApi(() => apiClient.getLogs(filter), { dependencies: [filter] });
}

// Hook for real-time metrics
export function useRealTimeMetrics() {
  const [metrics, setMetrics] = useState<any>(null);
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    let es: EventSource | null = null;

    const connect = async () => {
      try {
        es = await apiClient.createMetricsWebSocket(
          (data) => {
            setMetrics(data);
            setConnected(true);
          },
          (error) => {
            console.error('Metrics WebSocket error:', error);
            setConnected(false);
          }
        );
      } catch (error) {
        console.error('Failed to create Metrics WebSocket:', error);
        setConnected(false);
      }
    };

    connect();

    return () => {
      if (es) {
        es.close();
      }
      setConnected(false);
    };
  }, []);

  return { metrics, connected };
}

// Hook for real-time logs
export function useRealTimeLogs() {
  const [logs, setLogs] = useState<any[]>([]);
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    let es: EventSource | null = null;

    const connect = async () => {
      try {
        es = await apiClient.createLogsWebSocket(
          (data) => {
            setLogs(prev => [...prev, data].slice(-100)); // Keep last 100 logs
            setConnected(true);
          },
          (error) => {
            console.error('Logs WebSocket error:', error);
            setConnected(false);
          }
        );
      } catch (error) {
        console.error('Failed to create Logs WebSocket:', error);
        setConnected(false);
      }
    };

    connect();

    return () => {
      if (es) {
        es.close();
      }
      setConnected(false);
    };
  }, []);

  return { logs, connected, clearLogs: () => setLogs([]) };
}