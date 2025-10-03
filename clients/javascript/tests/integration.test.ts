/**
 * Integration tests for MantisDB JavaScript client
 */

import { 
  MantisClient, 
  createClient, 
  parseConnectionString,
  MantisError,
  Transaction,
  ClientConfig
} from '../src/index';
import { 
  BasicAuthProvider, 
  APIKeyAuthProvider, 
  JWTAuthProvider 
} from '../src/auth';

// Test configuration
const TEST_HOST = process.env.MANTISDB_TEST_HOST || 'localhost';
const TEST_PORT = parseInt(process.env.MANTISDB_TEST_PORT || '8080');
const TEST_USERNAME = process.env.MANTISDB_TEST_USERNAME || 'admin';
const TEST_PASSWORD = process.env.MANTISDB_TEST_PASSWORD || 'password';
const TEST_API_KEY = process.env.MANTISDB_TEST_API_KEY || '';

function createTestConfig(): ClientConfig {
  return {
    host: TEST_HOST,
    port: TEST_PORT,
    username: TEST_USERNAME,
    password: TEST_PASSWORD,
    timeout: 10000,
    retryAttempts: 3,
    retryDelay: 100,
  };
}

function createTestClient(): MantisClient {
  const config = createTestConfig();
  return new MantisClient(config);
}

describe('MantisClient Connection', () => {
  test('should connect successfully', async () => {
    const client = createTestClient();
    
    try {
      await client.ping();
    } finally {
      await client.close();
    }
  });

  test('should handle connection errors gracefully', async () => {
    const config: ClientConfig = {
      host: 'nonexistent-host',
      port: 9999,
      timeout: 1000,
      retryAttempts: 1,
    };
    
    const client = new MantisClient(config);
    
    await expect(client.ping()).rejects.toThrow();
    await client.close();
  });
});

describe('MantisClient Basic Operations', () => {
  let client: MantisClient;
  let tableName: string;

  beforeEach(() => {
    client = createTestClient();
    tableName = `test_users_${Date.now()}`;
  });

  afterEach(async () => {
    try {
      await client.query(`DROP TABLE ${tableName}`);
    } catch (error) {
      // Ignore cleanup errors
    }
    await client.close();
  });

  test('should perform CRUD operations', async () => {
    // Create table
    const createTableSQL = `
      CREATE TABLE ${tableName} (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL,
        email TEXT UNIQUE,
        age INTEGER,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
      )
    `;
    await client.query(createTableSQL);

    // Insert data
    const userData = {
      name: 'John Doe',
      email: 'john@example.com',
      age: 30,
    };
    await client.insert(tableName, userData);

    // Query data
    let result = await client.query(`SELECT * FROM ${tableName} WHERE name = 'John Doe'`);
    expect(result.rowCount).toBe(1);
    expect(result.rows[0].name).toBe('John Doe');

    // Get data with filters
    const filters = { age: 30 };
    result = await client.get(tableName, filters);
    expect(result.rowCount).toBe(1);

    // Update data
    const userId = String(result.rows[0].id);
    const updateData = { age: 31 };
    await client.update(tableName, userId, updateData);

    // Verify update
    result = await client.query(`SELECT age FROM ${tableName} WHERE id = ${userId}`);
    expect(result.rows[0].age).toBe(31);

    // Delete data
    await client.delete(tableName, userId);

    // Verify deletion
    result = await client.query(`SELECT COUNT(*) as count FROM ${tableName}`);
    expect(result.rows[0].count).toBe(0);
  });

  test('should handle query errors', async () => {
    await expect(client.query('INVALID SQL STATEMENT')).rejects.toThrow(MantisError);
    await expect(client.query('SELECT * FROM non_existent_table')).rejects.toThrow(MantisError);
    await expect(client.insert('non_existent_table', { field: 'value' })).rejects.toThrow(MantisError);
  });
});

describe('MantisClient Transactions', () => {
  let client: MantisClient;
  let tableName: string;

  beforeEach(async () => {
    client = createTestClient();
    tableName = `test_transactions_${Date.now()}`;
    
    // Create table
    const createTableSQL = `
      CREATE TABLE ${tableName} (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL,
        balance INTEGER DEFAULT 0
      )
    `;
    await client.query(createTableSQL);
  });

  afterEach(async () => {
    try {
      await client.query(`DROP TABLE ${tableName}`);
    } catch (error) {
      // Ignore cleanup errors
    }
    await client.close();
  });

  test('should commit transactions successfully', async () => {
    const tx = await client.beginTransaction();
    
    try {
      // Insert data in transaction
      const userData1 = { name: 'Alice', balance: 1000 };
      const userData2 = { name: 'Bob', balance: 500 };
      
      await tx.insert(tableName, userData1);
      await tx.insert(tableName, userData2);
      
      // Query within transaction
      const result = await tx.query(`SELECT COUNT(*) as count FROM ${tableName}`);
      expect(result.rows[0].count).toBe(2);
      
      await tx.commit();
    } catch (error) {
      await tx.rollback();
      throw error;
    }

    // Verify data persisted
    const result = await client.query(`SELECT COUNT(*) as count FROM ${tableName}`);
    expect(result.rows[0].count).toBe(2);
  });

  test('should rollback transactions on error', async () => {
    const tx = await client.beginTransaction();
    
    try {
      const userData = { name: 'Charlie', balance: 750 };
      await tx.insert(tableName, userData);
      
      // Force rollback
      await tx.rollback();
    } catch (error) {
      await tx.rollback();
    }

    // Verify data was not persisted
    const result = await client.query(`SELECT COUNT(*) as count FROM ${tableName}`);
    expect(result.rows[0].count).toBe(0);
  });

  test('should handle transaction errors', async () => {
    const tx = await client.beginTransaction();
    
    await expect(tx.query('INVALID SQL')).rejects.toThrow(MantisError);
    
    // Transaction should still be usable after error
    expect(tx.isClosed).toBe(false);
    
    await tx.rollback();
    expect(tx.isClosed).toBe(true);
  });

  test('should prevent operations on closed transactions', async () => {
    const tx = await client.beginTransaction();
    await tx.commit();
    
    expect(tx.isClosed).toBe(true);
    
    await expect(tx.query('SELECT 1')).rejects.toThrow(MantisError);
    await expect(tx.insert(tableName, { name: 'test' })).rejects.toThrow(MantisError);
    await expect(tx.commit()).rejects.toThrow(MantisError);
    await expect(tx.rollback()).rejects.toThrow(MantisError);
  });
});

describe('MantisClient Authentication', () => {
  test('should authenticate with basic auth', async () => {
    const config = createTestConfig();
    const client = new MantisClient(config);
    
    try {
      await client.ping();
    } finally {
      await client.close();
    }
  });

  test('should authenticate with API key', async () => {
    if (!TEST_API_KEY) {
      console.log('Skipping API key test - no API key provided');
      return;
    }

    const config: ClientConfig = {
      host: TEST_HOST,
      port: TEST_PORT,
      apiKey: TEST_API_KEY,
    };
    
    const client = new MantisClient(config);
    
    try {
      await client.ping();
    } finally {
      await client.close();
    }
  });

  test('should handle authentication errors', async () => {
    const config: ClientConfig = {
      host: TEST_HOST,
      port: TEST_PORT,
      username: 'invalid_user',
      password: 'invalid_password',
      retryAttempts: 1,
    };
    
    const client = new MantisClient(config);
    
    await expect(client.ping()).rejects.toThrow();
    await client.close();
  });
});

describe('MantisClient Concurrent Operations', () => {
  let client: MantisClient;
  let tableName: string;

  beforeEach(async () => {
    client = createTestClient();
    tableName = `test_concurrency_${Date.now()}`;
    
    // Create table
    const createTableSQL = `
      CREATE TABLE ${tableName} (
        id INTEGER PRIMARY KEY,
        worker_id INTEGER,
        value INTEGER
      )
    `;
    await client.query(createTableSQL);
  });

  afterEach(async () => {
    try {
      await client.query(`DROP TABLE ${tableName}`);
    } catch (error) {
      // Ignore cleanup errors
    }
    await client.close();
  });

  test('should handle concurrent operations', async () => {
    const numWorkers = 5;
    const numOperationsPerWorker = 3;
    
    const workerTasks = Array.from({ length: numWorkers }, (_, workerId) =>
      Promise.all(
        Array.from({ length: numOperationsPerWorker }, (_, j) =>
          client.insert(tableName, {
            worker_id: workerId,
            value: j,
          })
        )
      )
    );

    await Promise.all(workerTasks);

    // Verify all data was inserted
    const result = await client.query(`SELECT COUNT(*) as count FROM ${tableName}`);
    const expectedCount = numWorkers * numOperationsPerWorker;
    expect(result.rows[0].count).toBe(expectedCount);
  });
});

describe('MantisClient Utility Functions', () => {
  test('should create client with createClient function', () => {
    const config = createTestConfig();
    const client = createClient(config);
    
    expect(client).toBeInstanceOf(MantisClient);
    expect(client.getConfig().host).toBe(TEST_HOST);
    expect(client.getConfig().port).toBe(TEST_PORT);
  });

  test('should parse connection strings', () => {
    const connectionString = `mantisdb://${TEST_USERNAME}:${TEST_PASSWORD}@${TEST_HOST}:${TEST_PORT}`;
    const config = parseConnectionString(connectionString);
    
    expect(config.host).toBe(TEST_HOST);
    expect(config.port).toBe(TEST_PORT);
    expect(config.username).toBe(TEST_USERNAME);
    expect(config.password).toBe(TEST_PASSWORD);
  });

  test('should parse secure connection strings', () => {
    const connectionString = `mantisdbs://${TEST_USERNAME}:${TEST_PASSWORD}@${TEST_HOST}:${TEST_PORT}`;
    const config = parseConnectionString(connectionString);
    
    expect(config.tlsEnabled).toBe(true);
  });
});

describe('MantisClient Health Check', () => {
  test('should perform health check', async () => {
    const client = createTestClient();
    
    try {
      const healthResult = await client.healthCheck();
      
      expect(healthResult.status).toMatch(/^(healthy|degraded)$/);
      expect(healthResult.host).toBe(TEST_HOST);
      expect(healthResult.port).toBe(TEST_PORT);
      expect(healthResult.duration).toBeGreaterThan(0);
      expect(healthResult.timestamp).toBeInstanceOf(Date);
      expect(healthResult.connectionStats).toBeDefined();
    } finally {
      await client.close();
    }
  });
});

describe('MantisClient Error Handling', () => {
  test('should create proper MantisError instances', async () => {
    const client = createTestClient();
    
    try {
      await expect(client.query('INVALID SQL')).rejects.toThrow(MantisError);
      
      try {
        await client.query('INVALID SQL');
      } catch (error) {
        expect(error).toBeInstanceOf(MantisError);
        expect((error as MantisError).code).toBeTruthy();
        expect((error as MantisError).message).toBeTruthy();
        expect((error as MantisError).toString()).toContain('MantisError');
      }
    } finally {
      await client.close();
    }
  });
});

describe('MantisClient Configuration', () => {
  test('should use default configuration values', () => {
    const config: ClientConfig = {
      host: TEST_HOST,
      port: TEST_PORT,
    };
    
    const client = new MantisClient(config);
    const clientConfig = client.getConfig();
    
    expect(clientConfig.maxConnections).toBe(10);
    expect(clientConfig.timeout).toBe(60000);
    expect(clientConfig.retryAttempts).toBe(3);
    expect(clientConfig.retryDelay).toBe(1000);
    expect(clientConfig.enableCompression).toBe(true);
    expect(clientConfig.tlsEnabled).toBe(false);
  });

  test('should override default configuration values', () => {
    const config: ClientConfig = {
      host: TEST_HOST,
      port: TEST_PORT,
      maxConnections: 20,
      timeout: 30000,
      retryAttempts: 5,
      retryDelay: 2000,
      enableCompression: false,
      tlsEnabled: true,
    };
    
    const client = new MantisClient(config);
    const clientConfig = client.getConfig();
    
    expect(clientConfig.maxConnections).toBe(20);
    expect(clientConfig.timeout).toBe(30000);
    expect(clientConfig.retryAttempts).toBe(5);
    expect(clientConfig.retryDelay).toBe(2000);
    expect(clientConfig.enableCompression).toBe(false);
    expect(clientConfig.tlsEnabled).toBe(true);
  });
});

describe('MantisClient Performance', () => {
  test('should handle multiple queries efficiently', async () => {
    const client = createTestClient();
    
    try {
      const numQueries = 50;
      const startTime = Date.now();
      
      const queryPromises = Array.from({ length: numQueries }, () =>
        client.query('SELECT 1 as test_value')
      );
      
      const results = await Promise.all(queryPromises);
      
      const endTime = Date.now();
      const duration = endTime - startTime;
      const queriesPerSecond = (numQueries / duration) * 1000;
      
      console.log(`Performance: ${queriesPerSecond.toFixed(2)} queries/second`);
      
      // Verify all queries succeeded
      results.forEach(result => {
        expect(result.rowCount).toBe(1);
        expect(result.rows[0].test_value).toBe(1);
      });
      
      // Basic performance assertion (should be able to do at least 10 queries/second)
      expect(queriesPerSecond).toBeGreaterThan(10);
    } finally {
      await client.close();
    }
  });
});

describe('MantisClient Cross-Platform Compatibility', () => {
  let client: MantisClient;
  let tableName: string;

  beforeEach(() => {
    client = createTestClient();
    tableName = `test_unicode_${Date.now()}`;
  });

  afterEach(async () => {
    try {
      await client.query(`DROP TABLE ${tableName}`);
    } catch (error) {
      // Ignore cleanup errors
    }
    await client.close();
  });

  test('should handle Unicode strings', async () => {
    // Create table
    const createTableSQL = `
      CREATE TABLE ${tableName} (
        id INTEGER PRIMARY KEY,
        name TEXT,
        description TEXT
      )
    `;
    await client.query(createTableSQL);

    // Insert Unicode data
    const unicodeData = {
      name: '测试用户', // Chinese
      description: 'Тестовый пользователь', // Russian
    };
    await client.insert(tableName, unicodeData);

    // Query Unicode data
    const result = await client.query(`SELECT * FROM ${tableName}`);
    expect(result.rowCount).toBe(1);
    expect(result.rows[0].name).toBe('测试用户');
    expect(result.rows[0].description).toBe('Тестовый пользователь');
  });

  test('should handle large data', async () => {
    // Create table
    const createTableSQL = `
      CREATE TABLE ${tableName} (
        id INTEGER PRIMARY KEY,
        large_text TEXT
      )
    `;
    await client.query(createTableSQL);

    // Insert large text data (100KB)
    const largeText = 'A'.repeat(100 * 1024); // 100KB of 'A' characters
    const largeData = {
      large_text: largeText,
    };
    await client.insert(tableName, largeData);

    // Query large data
    const result = await client.query(`SELECT large_text FROM ${tableName}`);
    expect(result.rowCount).toBe(1);
    expect(result.rows[0].large_text.length).toBe(100 * 1024);
  });

  test('should handle various data types', async () => {
    // Create table
    const createTableSQL = `
      CREATE TABLE ${tableName} (
        id INTEGER PRIMARY KEY,
        text_field TEXT,
        integer_field INTEGER,
        real_field REAL,
        boolean_field BOOLEAN,
        null_field TEXT
      )
    `;
    await client.query(createTableSQL);

    // Insert various data types
    const testData = {
      text_field: 'Hello World',
      integer_field: 42,
      real_field: 3.14159,
      boolean_field: true,
      null_field: null,
    };
    await client.insert(tableName, testData);

    // Query and verify data types
    const result = await client.query(`SELECT * FROM ${tableName}`);
    expect(result.rowCount).toBe(1);
    
    const row = result.rows[0];
    expect(typeof row.text_field).toBe('string');
    expect(typeof row.integer_field).toBe('number');
    expect(typeof row.real_field).toBe('number');
    expect(typeof row.boolean_field).toBe('boolean');
    expect(row.null_field).toBeNull();
  });
});

describe('Authentication Providers', () => {
  test('should create BasicAuthProvider', () => {
    const provider = new BasicAuthProvider(TEST_USERNAME, TEST_PASSWORD);
    expect(provider).toBeInstanceOf(BasicAuthProvider);
  });

  test('should create APIKeyAuthProvider', () => {
    const provider = new APIKeyAuthProvider('test-api-key');
    expect(provider).toBeInstanceOf(APIKeyAuthProvider);
  });

  test('should create JWTAuthProvider', () => {
    const provider = new JWTAuthProvider('client-id', 'client-secret');
    expect(provider).toBeInstanceOf(JWTAuthProvider);
  });
});

describe('MantisClient Failover', () => {
  test('should handle failover configuration', () => {
    const config: ClientConfig = {
      host: TEST_HOST,
      port: TEST_PORT,
      enableFailover: true,
      failoverHosts: ['localhost:8081', 'localhost:8082'],
    };
    
    const client = new MantisClient(config);
    const clientConfig = client.getConfig();
    
    expect(clientConfig.enableFailover).toBe(true);
    expect(clientConfig.failoverHosts).toEqual(['localhost:8081', 'localhost:8082']);
    
    const connectionStats = client.getConnectionStats();
    expect(connectionStats).toBeDefined();
    expect(connectionStats.totalHosts).toBeGreaterThan(1);
  });
});