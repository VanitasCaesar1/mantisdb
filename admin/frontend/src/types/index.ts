// Common types for the MantisDB Admin Dashboard

export interface DatabaseConnection {
  id: string;
  name: string;
  host: string;
  port: number;
  database: string;
  status: 'connected' | 'disconnected' | 'error';
  lastConnected?: Date;
}

export interface SystemMetrics {
  cpu_usage: number;
  memory_usage: number;
  memory_total: number;
  disk_usage: number;
  disk_total: number;
  query_latency: number[];
  active_connections: number;
  cache_hit_ratio: number;
  queries_per_second: number;
  timestamp: Date;
}

export interface SystemStats {
  uptime_seconds: number;
  version: string;
  go_version: string;
  platform: string;
  cpu_usage_percent: number;
  memory_usage_bytes: number;
  disk_usage_bytes: number;
  network_stats: Record<string, any>;
  active_connections: number;
  database_stats: Record<string, any>;
}

export interface LogEntry {
  id: string;
  timestamp: Date;
  level: 'DEBUG' | 'INFO' | 'WARN' | 'ERROR' | 'FATAL';
  component: string;
  message: string;
  request_id?: string;
  user_id?: string;
  metadata?: Record<string, any>;
}

export interface BackupInfo {
  id: string;
  name?: string;
  status: 'pending' | 'running' | 'completed' | 'failed' | 'creating';
  created_at: Date;
  completed_at?: Date;
  size?: number;
  size_bytes?: number;
  record_count?: number;
  checksum?: string;
  progress?: number;
  progress_percent?: number;
  error_message?: string;
  error?: string;
  type?: 'manual' | 'scheduled';
  tags?: Record<string, string>;
}

export interface BackupRequest {
  tags?: Record<string, string>;
  description?: string;
}

export interface QueryResult {
  columns: string[];
  rows: any[][];
  rowCount: number;
  executionTime: number;
  query: string;
  timestamp: Date;
}

export interface QueryHistory {
  id: string;
  query: string;
  timestamp: Date;
  executionTime: number;
  rowCount: number;
  status: 'success' | 'error';
  error?: string;
}

export interface QueryRequest {
  query: string;
  query_type?: string;
  limit?: number;
  offset?: number;
}

export interface QueryResponse {
  success: boolean;
  data?: any;
  rows_affected: number;
  duration_ms: number;
  error?: string;
  query_id: string;
}

export interface LogFilter {
  level?: string;
  component?: string;
  request_id?: string;
  user_id?: string;
  start_time?: Date;
  end_time?: Date;
  search_query?: string;
  limit?: number;
  offset?: number;
}

export interface TableInfo {
  name: string;
  schema?: string;
  rowCount: number;
  row_count: number;
  size: number;
  size_bytes: number;
  type: string;
  columns?: ColumnInfo[];
  indexes?: IndexInfo[];
  lastModified?: Date;
  created_at: Date;
  updated_at: Date;
}

export interface ColumnInfo {
  name: string;
  type: string;
  nullable: boolean;
  defaultValue?: any;
  isPrimaryKey: boolean;
  isForeignKey: boolean;
  references?: {
    table: string;
    column: string;
  };
}

export interface IndexInfo {
  name: string;
  columns: string[];
  unique: boolean;
  type: string;
}

export interface DatabaseConfig {
  server: {
    port: number;
    admin_port: number;
    host: string;
  };
  database: {
    data_dir: string;
    wal_dir: string;
    cache_size: string;
  };
  backup: {
    enabled: boolean;
    schedule: string;
    retention_days: number;
    destination: string;
  };
  logging: {
    level: string;
    format: string;
    output: string;
  };
  memory: {
    cache_limit: string;
    eviction_policy: 'LRU' | 'LFU' | 'TTL';
  };
  compression: {
    enabled: boolean;
    algorithm: 'LZ4' | 'Snappy' | 'ZSTD';
    cold_data_threshold: string;
  };
}

export interface ApiResponse<T = any> {
  success: boolean;
  data?: T;
  error?: string;
  message?: string;
}

export interface PaginationParams {
  page: number;
  limit: number;
  sortBy?: string;
  sortOrder?: 'asc' | 'desc';
  search?: string;
}

export interface PaginatedResponse<T> {
  data: T[];
  pagination: {
    page: number;
    limit: number;
    total: number;
    totalPages: number;
    hasNext: boolean;
    hasPrev: boolean;
  };
}

// Component prop types
export interface ComponentWithChildren {
  children: React.ReactNode;
}

export interface ComponentWithClassName {
  className?: string;
}

export type LoadingState = 'idle' | 'loading' | 'success' | 'error';

export interface AsyncState<T = any> {
  data: T | null;
  loading: boolean;
  error: string | null;
}

// Navigation types
export interface NavigationItem {
  id: string;
  label: string;
  path: string;
  icon?: React.ReactNode;
  badge?: string;
  children?: NavigationItem[];
}

// Form types
export interface FormField {
  name: string;
  label: string;
  type: 'text' | 'number' | 'email' | 'password' | 'select' | 'textarea' | 'checkbox' | 'radio';
  required?: boolean;
  placeholder?: string;
  options?: Array<{ label: string; value: any }>;
  validation?: {
    min?: number;
    max?: number;
    pattern?: string;
    custom?: (value: any) => string | null;
  };
}

export interface FormData {
  [key: string]: any;
}

export interface FormErrors {
  [key: string]: string;
}