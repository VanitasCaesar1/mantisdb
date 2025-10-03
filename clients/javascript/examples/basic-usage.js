/**
 * Basic usage example for MantisDB JavaScript client
 */

const { MantisClient, parseConnectionString } = require('mantisdb-js');

async function basicExample() {
  // Create client with configuration object
  const client = new MantisClient({
    host: 'localhost',
    port: 8080,
    username: 'admin',
    password: 'password',
    timeout: 30000,
    retryAttempts: 3
  });

  try {
    // Test connection
    await client.ping();
    console.log('Connected to MantisDB successfully!');

    // Insert data
    const userData = {
      name: 'John Doe',
      email: 'john@example.com',
      age: 30,
      active: true
    };

    await client.insert('users', userData);
    console.log('User inserted successfully');

    // Query data
    const result = await client.query('SELECT * FROM users WHERE age > 25');
    console.log(`Found ${result.rowCount} users:`);
    
    result.rows.forEach(row => {
      console.log(`  - ${row.name} (${row.email}) - Age: ${row.age}`);
    });

    // Get data with filters
    const activeUsers = await client.get('users', { active: true });
    console.log(`Active users: ${activeUsers.rowCount}`);

    // Update data
    if (result.rows.length > 0) {
      const userId = result.rows[0].id;
      await client.update('users', userId, { age: 31 });
      console.log(`Updated user ${userId}`);
    }

  } catch (error) {
    if (error.name === 'MantisError') {
      console.error(`MantisDB Error [${error.code}]: ${error.message}`);
      if (error.details) {
        console.error('Details:', error.details);
      }
    } else {
      console.error('Unexpected error:', error.message);
    }
  } finally {
    await client.close();
    console.log('Connection closed');
  }
}

async function connectionStringExample() {
  // Create client with connection string
  const connectionString = 'mantisdb://admin:password@localhost:8080?timeout=30000&retryAttempts=3';
  const config = parseConnectionString(connectionString);
  const client = new MantisClient(config);

  try {
    await client.ping();
    console.log('Connected using connection string!');
  } catch (error) {
    console.error('Connection failed:', error.message);
  } finally {
    await client.close();
  }
}

// Run examples
async function main() {
  console.log('=== Basic Usage Example ===');
  await basicExample();
  
  console.log('\n=== Connection String Example ===');
  await connectionStringExample();
}

main().catch(console.error);