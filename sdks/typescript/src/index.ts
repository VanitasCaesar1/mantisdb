/**
 * MantisDB TypeScript SDK
 * 
 * Type-safe client library for MantisDB multimodal database.
 * 
 * @example
 * ```typescript
 * import { MantisDB } from '@mantisdb/client';
 * 
 * const db = new MantisDB('http://localhost:8080');
 * 
 * // Key-Value operations
 * await db.kv.set('user:123', 'John Doe');
 * const name = await db.kv.get('user:123');
 * 
 * // Document operations
 * await db.docs.insert('users', { name: 'Alice', age: 30 });
 * const users = await db.docs.find('users', { age: { $gt: 25 } });
 * 
 * // SQL operations
 * const results = await db.sql.execute('SELECT * FROM users WHERE age > 25');
 * 
 * // Vector operations
 * await db.vectors.insert('embeddings', [0.1, 0.2, 0.3], { text: 'hello' });
 * const similar = await db.vectors.search('embeddings', [0.15, 0.25, 0.35], 10);
 * ```
 */

import axios, { AxiosInstance, AxiosResponse } from 'axios';

// Types
export interface MantisDBConfig {
  baseURL: string;
  authToken?: string;
  timeout?: number;
}

export interface HealthResponse {
  status: string;
  timestamp: string;
  version: string;
}

export interface MetricsResponse {
  overview: {
    uptime_seconds: number;
    total_queries: number;
    queries_per_second: number;
    error_rate: number;
  };
  performance: {
    avg_latency_ms: number;
    p50_latency_ms: number;
    p95_latency_ms: number;
    p99_latency_ms: number;
    cache_hit_ratio: number;
  };
}

export interface Document {
  [key: string]: any;
}

export interface QueryFilter {
  [key: string]: any;
}

export interface VectorSearchResult {
  id: string;
  score: number;
  metadata: Record<string, any>;
  vector?: number[];
}

// Key-Value Store
export class KeyValueStore {
  constructor(private client: AxiosInstance) {}

  async get(key: string): Promise<string | null> {
    const response = await this.client.get(`/api/kv/${key}`);
    return response.data.value;
  }

  async set(key: string, value: string, ttl?: number): Promise<void> {
    await this.client.post('/api/kv', { key, value, ttl });
  }

  async delete(key: string): Promise<void> {
    await this.client.delete(`/api/kv/${key}`);
  }

  async exists(key: string): Promise<boolean> {
    try {
      await this.get(key);
      return true;
    } catch {
      return false;
    }
  }
}

// Document Store
export class DocumentStore {
  constructor(private client: AxiosInstance) {}

  async insert(collection: string, document: Document): Promise<string> {
    const response = await this.client.post(`/api/documents/${collection}`, document);
    return response.data.id;
  }

  async find(collection: string, query: QueryFilter): Promise<Document[]> {
    const response = await this.client.post(`/api/documents/${collection}/find`, query);
    return response.data.documents;
  }

  async update(collection: string, id: string, update: Document): Promise<void> {
    await this.client.put(`/api/documents/${collection}/${id}`, update);
  }

  async delete(collection: string, id: string): Promise<void> {
    await this.client.delete(`/api/documents/${collection}/${id}`);
  }
}

// SQL Database
export class SQLDatabase {
  constructor(private client: AxiosInstance) {}

  async execute(query: string): Promise<any[]> {
    const response = await this.client.post('/api/query', { query });
    return response.data.results;
  }

  async createTable(table: string, schema: Record<string, string>): Promise<void> {
    await this.client.post('/api/tables/create', { name: table, schema });
  }

  async listTables(): Promise<string[]> {
    const response = await this.client.get('/api/tables');
    return response.data.tables.map((t: any) => t.name);
  }
}

// Vector Database
export class VectorDatabase {
  constructor(private client: AxiosInstance) {}

  async insert(
    collection: string,
    vector: number[],
    metadata?: Record<string, any>
  ): Promise<string> {
    const response = await this.client.post(`/api/vectors/${collection}`, {
      vector,
      metadata: metadata || {},
    });
    return response.data.id;
  }

  async search(
    collection: string,
    queryVector: number[],
    k: number = 10,
    filter?: QueryFilter
  ): Promise<VectorSearchResult[]> {
    const response = await this.client.post(`/api/vectors/${collection}/search`, {
      vector: queryVector,
      k,
      filter,
    });
    return response.data.results;
  }

  async delete(collection: string, id: string): Promise<void> {
    await this.client.delete(`/api/vectors/${collection}/${id}`);
  }
}

// Query Builder
export class QueryBuilder {
  private selectCols: string[] = ['*'];
  private fromTable: string = '';
  private whereClauses: string[] = [];
  private orderByClauses: string[] = [];
  private limitVal?: number;
  private offsetVal?: number;

  constructor(private sql: SQLDatabase) {}

  select(...columns: string[]): this {
    this.selectCols = columns.length > 0 ? columns : ['*'];
    return this;
  }

  from(table: string): this {
    this.fromTable = table;
    return this;
  }

  where(condition: string): this {
    this.whereClauses.push(condition);
    return this;
  }

  orderBy(column: string, direction: 'ASC' | 'DESC' = 'ASC'): this {
    this.orderByClauses.push(`${column} ${direction}`);
    return this;
  }

  limit(n: number): this {
    this.limitVal = n;
    return this;
  }

  offset(n: number): this {
    this.offsetVal = n;
    return this;
  }

  build(): string {
    let query = `SELECT ${this.selectCols.join(', ')} FROM ${this.fromTable}`;
    if (this.whereClauses.length > 0) {
      query += ` WHERE ${this.whereClauses.join(' AND ')}`;
    }
    if (this.orderByClauses.length > 0) {
      query += ` ORDER BY ${this.orderByClauses.join(', ')}`;
    }
    if (this.limitVal) {
      query += ` LIMIT ${this.limitVal}`;
    }
    if (this.offsetVal) {
      query += ` OFFSET ${this.offsetVal}`;
    }
    return query;
  }

  async execute(): Promise<any[]> {
    return this.sql.execute(this.build());
  }
}

// Main Client
export class MantisDB {
  private client: AxiosInstance;
  public kv: KeyValueStore;
  public docs: DocumentStore;
  public sql: SQLDatabase;
  public vectors: VectorDatabase;

  constructor(config: string | MantisDBConfig) {
    const cfg = typeof config === 'string' ? { baseURL: config } : config;

    this.client = axios.create({
      baseURL: cfg.baseURL,
      timeout: cfg.timeout || 30000,
      headers: cfg.authToken ? { Authorization: `Bearer ${cfg.authToken}` } : {},
    });

    this.kv = new KeyValueStore(this.client);
    this.docs = new DocumentStore(this.client);
    this.sql = new SQLDatabase(this.client);
    this.vectors = new VectorDatabase(this.client);
  }

  async health(): Promise<HealthResponse> {
    const response = await this.client.get('/api/health');
    return response.data;
  }

  async metrics(): Promise<MetricsResponse> {
    const response = await this.client.get('/api/metrics/detailed');
    return response.data;
  }

  query(): QueryBuilder {
    return new QueryBuilder(this.sql);
  }
}

// Default export
export default MantisDB;
