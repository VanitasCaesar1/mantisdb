/**
 * Type definitions for MantisDB JavaScript client
 */

// Import types that we need to reference
import type { QueryResult, Transaction, MantisClient } from "./index";

// Re-export main types from index (avoiding circular dependencies)
export type {
  ClientConfig,
  DefaultClientConfig,
  QueryResult,
  TransactionResponse,
  MantisErrorDetails,
  ErrorResponse,
  QueryRequest,
  InsertRequest,
  UpdateRequest,
  MantisError,
  Transaction,
  MantisClient,
} from "./index";

// Re-export auth types
export type {
  AuthToken,
  AuthProvider,
  BasicAuthProvider,
  APIKeyAuthProvider,
  JWTAuthProvider,
  AuthManager,
  ConnectionManager,
  HealthCheckResult,
} from "./auth";

// Additional utility types
export interface ConnectionStats {
  currentHost: string;
  totalHosts: number;
  currentHostIndex: number;
  maxConnections?: number;
  activeConnections?: number;
  idleConnections?: number;
}

export interface QueryOptions {
  timeout?: number;
  retryAttempts?: number;
  retryDelay?: number;
}

export interface InsertOptions extends QueryOptions {
  upsert?: boolean;
  onConflict?: "ignore" | "replace" | "fail";
}

export interface UpdateOptions extends QueryOptions {
  where?: Record<string, any>;
}

export interface DeleteOptions extends QueryOptions {
  where?: Record<string, any>;
  limit?: number;
}

export interface GetOptions extends QueryOptions {
  limit?: number;
  offset?: number;
  orderBy?: string | string[];
  orderDirection?: "ASC" | "DESC";
}

export interface TransactionOptions {
  isolationLevel?:
    | "READ_UNCOMMITTED"
    | "READ_COMMITTED"
    | "REPEATABLE_READ"
    | "SERIALIZABLE";
  timeout?: number;
}

// Metadata types
export interface TableMetadata {
  name: string;
  columns: ColumnMetadata[];
  indexes: IndexMetadata[];
  constraints: ConstraintMetadata[];
}

export interface ColumnMetadata {
  name: string;
  type: string;
  nullable: boolean;
  defaultValue?: any;
  isPrimaryKey: boolean;
  isUnique: boolean;
  isAutoIncrement: boolean;
}

export interface IndexMetadata {
  name: string;
  columns: string[];
  isUnique: boolean;
  isPrimary: boolean;
}

export interface ConstraintMetadata {
  name: string;
  type: "PRIMARY_KEY" | "FOREIGN_KEY" | "UNIQUE" | "CHECK" | "NOT_NULL";
  columns: string[];
  referencedTable?: string;
  referencedColumns?: string[];
}

// Performance monitoring types
export interface PerformanceMetrics {
  queryCount: number;
  averageQueryTime: number;
  minQueryTime: number;
  maxQueryTime: number;
  errorCount: number;
  connectionCount: number;
  cacheHitRatio?: number;
}

export interface QueryMetrics {
  sql: string;
  duration: number;
  rowsAffected: number;
  timestamp: Date;
  success: boolean;
  error?: string;
}

// Event types for monitoring
export type ClientEvent =
  | "query:start"
  | "query:success"
  | "query:error"
  | "transaction:start"
  | "transaction:commit"
  | "transaction:rollback"
  | "connection:open"
  | "connection:close"
  | "auth:success"
  | "auth:error"
  | "failover:start"
  | "failover:success"
  | "failover:error";

export interface ClientEventData {
  event: ClientEvent;
  timestamp: Date;
  data?: any;
  error?: Error;
}

// Callback types
export type EventCallback = (eventData: ClientEventData) => void;
export type QueryCallback = (error: Error | null, result?: QueryResult) => void;
export type TransactionCallback = (
  error: Error | null,
  transaction?: Transaction
) => void;

// Configuration validation types
export interface ConfigValidationResult {
  valid: boolean;
  errors: string[];
  warnings: string[];
}

// Utility types
export type DeepPartial<T> = {
  [P in keyof T]?: T[P] extends object ? DeepPartial<T[P]> : T[P];
};

export type RequiredKeys<T, K extends keyof T> = T & Required<Pick<T, K>>;

export type OptionalKeys<T, K extends keyof T> = Omit<T, K> &
  Partial<Pick<T, K>>;

// Database value types
export type DatabaseValue =
  | string
  | number
  | boolean
  | null
  | Date
  | Buffer
  | DatabaseValue[]
  | { [key: string]: DatabaseValue };

export type DatabaseRow = Record<string, DatabaseValue>;

// Query builder types (for future extension)
export interface QueryBuilder {
  select(columns?: string | string[]): QueryBuilder;
  from(table: string): QueryBuilder;
  where(condition: string | Record<string, any>): QueryBuilder;
  orderBy(column: string, direction?: "ASC" | "DESC"): QueryBuilder;
  limit(count: number): QueryBuilder;
  offset(count: number): QueryBuilder;
  join(table: string, condition: string): QueryBuilder;
  leftJoin(table: string, condition: string): QueryBuilder;
  rightJoin(table: string, condition: string): QueryBuilder;
  groupBy(columns: string | string[]): QueryBuilder;
  having(condition: string): QueryBuilder;
  toSQL(): string;
  execute(): Promise<QueryResult>;
}

// Migration types (for future extension)
export interface Migration {
  version: string;
  name: string;
  up: string | ((client: MantisClient) => Promise<void>);
  down: string | ((client: MantisClient) => Promise<void>);
}

export interface MigrationResult {
  version: string;
  name: string;
  success: boolean;
  duration: number;
  error?: string;
}

// Schema types (for future extension)
export interface Schema {
  tables: Record<string, TableSchema>;
  views: Record<string, ViewSchema>;
  indexes: Record<string, IndexSchema>;
}

export interface TableSchema {
  name: string;
  columns: Record<string, ColumnSchema>;
  primaryKey: string[];
  foreignKeys: ForeignKeySchema[];
  indexes: string[];
}

export interface ColumnSchema {
  name: string;
  type: string;
  nullable: boolean;
  defaultValue?: any;
  autoIncrement: boolean;
}

export interface ViewSchema {
  name: string;
  definition: string;
  columns: Record<string, ColumnSchema>;
}

export interface IndexSchema {
  name: string;
  table: string;
  columns: string[];
  unique: boolean;
  type: "BTREE" | "HASH" | "FULLTEXT";
}

export interface ForeignKeySchema {
  name: string;
  columns: string[];
  referencedTable: string;
  referencedColumns: string[];
  onUpdate: "CASCADE" | "SET_NULL" | "RESTRICT" | "NO_ACTION";
  onDelete: "CASCADE" | "SET_NULL" | "RESTRICT" | "NO_ACTION";
}
