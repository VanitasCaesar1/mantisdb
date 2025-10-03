/**
 * TypeScript usage example for MantisDB client
 */

import { 
  MantisClient, 
  ClientConfig, 
  QueryResult, 
  Transaction,
  MantisError,
  parseConnectionString,
  validateConfig
} from 'mantisdb-js';

// Define interfaces for our data
interface User {
  id?: string;
  name: string;
  email: string;
  age: number;
  active: boolean;
  createdAt?: Date;
}

interface UserFilters {
  active?: boolean;
  minAge?: number;
  maxAge?: number;
}

class UserService {
  private client: MantisClient;

  constructor(config: ClientConfig) {
    // Validate configuration
    const validation = validateConfig(config);
    if (!validation.valid) {
      throw new Error(`Invalid configuration: ${validation.errors.join(', ')}`);
    }

    if (validation.warnings.length > 0) {
      console.warn('Configuration warnings:', validation.warnings);
    }

    this.client = new MantisClient(config);
  }

  async connect(): Promise<void> {
    await this.client.ping();
    console.log('Connected to MantisDB');
  }

  async createUser(user: Omit<User, 'id' | 'createdAt'>): Promise<void> {
    const userData = {
      ...user,
      createdAt: new Date().toISOString()
    };

    await this.client.insert('users', userData);
    console.log(`Created user: ${user.name}`);
  }

  async getUsers(filters?: UserFilters): Promise<User[]> {
    let query = 'SELECT * FROM users WHERE 1=1';
    
    if (filters?.active !== undefined) {
      query += ` AND active = ${filters.active}`;
    }
    
    if (filters?.minAge !== undefined) {
      query += ` AND age >= ${filters.minAge}`;
    }
    
    if (filters?.maxAge !== undefined) {
      query += ` AND age <= ${filters.maxAge}`;
    }

    const result: QueryResult = await this.client.query(query);
    return result.rows as User[];
  }

  async updateUser(id: string, updates: Partial<Omit<User, 'id'>>): Promise<void> {
    await this.client.update('users', id, updates);
    console.log(`Updated user: ${id}`);
  }

  async deleteUser(id: string): Promise<void> {
    await this.client.delete('users', id);
    console.log(`Deleted user: ${id}`);
  }

  async createUsersInTransaction(users: Omit<User, 'id' | 'createdAt'>[]): Promise<void> {
    const tx: Transaction = await this.client.beginTransaction();

    try {
      for (const user of users) {
        const userData = {
          ...user,
          createdAt: new Date().toISOString()
        };
        
        await tx.insert('users', userData);
      }

      // Verify the transaction
      const result = await tx.query('SELECT COUNT(*) as count FROM users');
      console.log(`Total users after transaction: ${result.rows[0].count}`);

      await tx.commit();
      console.log(`Successfully created ${users.length} users in transaction`);

    } catch (error) {
      if (!tx.isClosed) {
        await tx.rollback();
        console.log('Transaction rolled back due to error');
      }
      throw error;
    }
  }

  async getUserStats(): Promise<{ total: number; active: number; averageAge: number }> {
    const result = await this.client.query(`
      SELECT 
        COUNT(*) as total,
        SUM(CASE WHEN active = true THEN 1 ELSE 0 END) as active,
        AVG(age) as averageAge
      FROM users
    `);

    const row = result.rows[0];
    return {
      total: row.total,
      active: row.active,
      averageAge: Math.round(row.averageAge * 100) / 100
    };
  }

  async close(): Promise<void> {
    await this.client.close();
    console.log('Connection closed');
  }
}

// Example usage
async function typeScriptExample(): Promise<void> {
  // Configuration with type safety
  const config: ClientConfig = {
    host: 'localhost',
    port: 8080,
    username: 'admin',
    password: 'password',
    timeout: 30000,
    retryAttempts: 3,
    enableCompression: true
  };

  const userService = new UserService(config);

  try {
    await userService.connect();

    // Create sample users with type safety
    const sampleUsers: Omit<User, 'id' | 'createdAt'>[] = [
      { name: 'Alice Johnson', email: 'alice@example.com', age: 28, active: true },
      { name: 'Bob Smith', email: 'bob@example.com', age: 35, active: true },
      { name: 'Charlie Brown', email: 'charlie@example.com', age: 22, active: false },
      { name: 'Diana Prince', email: 'diana@example.com', age: 30, active: true }
    ];

    // Create users in a transaction
    await userService.createUsersInTransaction(sampleUsers);

    // Query users with filters
    const activeUsers: User[] = await userService.getUsers({ active: true });
    console.log(`Active users: ${activeUsers.length}`);

    const youngUsers: User[] = await userService.getUsers({ maxAge: 25 });
    console.log(`Young users (≤25): ${youngUsers.length}`);

    // Get statistics
    const stats = await userService.getUserStats();
    console.log('User Statistics:', stats);

    // Update a user
    if (activeUsers.length > 0) {
      const userId = activeUsers[0].id!;
      await userService.updateUser(userId, { age: 29 });
    }

    // Demonstrate error handling with types
    try {
      await userService.getUsers({ minAge: -1 }); // This might cause an error
    } catch (error) {
      if (error instanceof MantisError) {
        console.error(`MantisDB Error [${error.code}]: ${error.message}`);
        if (error.details) {
          console.error('Error details:', error.details);
        }
      } else {
        console.error('Unexpected error:', error);
      }
    }

  } catch (error) {
    console.error('Example error:', error);
  } finally {
    await userService.close();
  }
}

// Connection string example with TypeScript
async function connectionStringTypeScriptExample(): Promise<void> {
  const connectionString = 'mantisdb://admin:password@localhost:8080?timeout=30000';
  
  try {
    const config: ClientConfig = parseConnectionString(connectionString);
    const client = new MantisClient(config);

    await client.ping();
    console.log('Connected using connection string with TypeScript!');

    // Type-safe query result
    const result: QueryResult = await client.query('SELECT COUNT(*) as userCount FROM users');
    const userCount: number = result.rows[0].userCount;
    console.log(`Total users: ${userCount}`);

    await client.close();

  } catch (error) {
    if (error instanceof MantisError) {
      console.error(`Connection failed [${error.code}]: ${error.message}`);
    } else {
      console.error('Connection failed:', error);
    }
  }
}

// Generic repository pattern example
class Repository<T extends Record<string, any>> {
  constructor(
    private client: MantisClient,
    private tableName: string
  ) {}

  async create(data: Omit<T, 'id'>): Promise<void> {
    await this.client.insert(this.tableName, data);
  }

  async findAll(): Promise<T[]> {
    const result = await this.client.query(`SELECT * FROM ${this.tableName}`);
    return result.rows as T[];
  }

  async findById(id: string): Promise<T | null> {
    const result = await this.client.query(
      `SELECT * FROM ${this.tableName} WHERE id = '${id}'`
    );
    return result.rows.length > 0 ? result.rows[0] as T : null;
  }

  async update(id: string, data: Partial<Omit<T, 'id'>>): Promise<void> {
    await this.client.update(this.tableName, id, data);
  }

  async delete(id: string): Promise<void> {
    await this.client.delete(this.tableName, id);
  }
}

async function repositoryPatternExample(): Promise<void> {
  const config: ClientConfig = {
    host: 'localhost',
    port: 8080,
    username: 'admin',
    password: 'password'
  };

  const client = new MantisClient(config);

  try {
    await client.ping();

    // Create a typed repository
    const userRepository = new Repository<User>(client, 'users');

    // Use the repository with full type safety
    await userRepository.create({
      name: 'Repository User',
      email: 'repo@example.com',
      age: 25,
      active: true
    });

    const users: User[] = await userRepository.findAll();
    console.log(`Repository found ${users.length} users`);

    if (users.length > 0) {
      const user: User | null = await userRepository.findById(users[0].id!);
      if (user) {
        console.log(`Found user: ${user.name}`);
        
        await userRepository.update(user.id!, { age: 26 });
        console.log('User updated via repository');
      }
    }

  } catch (error) {
    console.error('Repository example error:', error);
  } finally {
    await client.close();
  }
}

// Run examples
async function main(): Promise<void> {
  console.log('=== TypeScript Example ===');
  await typeScriptExample();

  console.log('\n=== Connection String TypeScript Example ===');
  await connectionStringTypeScriptExample();

  console.log('\n=== Repository Pattern Example ===');
  await repositoryPatternExample();
}

main().catch(console.error);