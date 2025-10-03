/**
 * Transaction examples for MantisDB JavaScript client
 */

const { MantisClient } = require('mantisdb-js');

async function transactionExample() {
  const client = new MantisClient({
    host: 'localhost',
    port: 8080,
    username: 'admin',
    password: 'password'
  });

  try {
    await client.ping();
    console.log('Connected to MantisDB');

    // Manual transaction management
    console.log('\n=== Manual Transaction Management ===');
    const tx = await client.beginTransaction();
    
    try {
      // Insert multiple users in a transaction
      const users = [
        { name: 'Alice Smith', email: 'alice@example.com', age: 25 },
        { name: 'Bob Johnson', email: 'bob@example.com', age: 35 },
        { name: 'Charlie Brown', email: 'charlie@example.com', age: 22 }
      ];

      for (const user of users) {
        await tx.insert('users', user);
        console.log(`Inserted user: ${user.name}`);
      }

      // Query within transaction
      const result = await tx.query('SELECT COUNT(*) as count FROM users');
      console.log(`Total users in transaction: ${result.rows[0].count}`);

      // Update within transaction
      await tx.update('users', 'user-id-1', { age: 26 });
      console.log('Updated user age');

      // Commit transaction
      await tx.commit();
      console.log('Transaction committed successfully');

    } catch (error) {
      console.error('Transaction error:', error.message);
      if (!tx.isClosed) {
        await tx.rollback();
        console.log('Transaction rolled back');
      }
      throw error;
    }

    // Demonstrate rollback scenario
    console.log('\n=== Rollback Scenario ===');
    const tx2 = await client.beginTransaction();
    
    try {
      await tx2.insert('users', { name: 'Test User', email: 'test@example.com', age: 30 });
      console.log('Inserted test user');

      // Simulate an error condition
      throw new Error('Simulated error - rolling back');

    } catch (error) {
      console.error('Error occurred:', error.message);
      if (!tx2.isClosed) {
        await tx2.rollback();
        console.log('Transaction rolled back - test user not saved');
      }
    }

    // Verify rollback worked
    const finalResult = await client.query('SELECT * FROM users WHERE name = "Test User"');
    console.log(`Test user exists after rollback: ${finalResult.rowCount > 0}`);

  } catch (error) {
    console.error('Example error:', error.message);
  } finally {
    await client.close();
    console.log('Connection closed');
  }
}

async function concurrentTransactionsExample() {
  const client = new MantisClient({
    host: 'localhost',
    port: 8080,
    username: 'admin',
    password: 'password'
  });

  try {
    await client.ping();
    console.log('\n=== Concurrent Transactions Example ===');

    // Create multiple concurrent transactions
    const transactions = await Promise.all([
      client.beginTransaction(),
      client.beginTransaction(),
      client.beginTransaction()
    ]);

    const promises = transactions.map(async (tx, index) => {
      try {
        // Each transaction inserts different data
        await tx.insert('users', {
          name: `Concurrent User ${index + 1}`,
          email: `user${index + 1}@example.com`,
          age: 20 + index
        });

        console.log(`Transaction ${index + 1}: Inserted user`);

        // Simulate some processing time
        await new Promise(resolve => setTimeout(resolve, 100 * (index + 1)));

        await tx.commit();
        console.log(`Transaction ${index + 1}: Committed`);

      } catch (error) {
        console.error(`Transaction ${index + 1} error:`, error.message);
        if (!tx.isClosed) {
          await tx.rollback();
          console.log(`Transaction ${index + 1}: Rolled back`);
        }
      }
    });

    await Promise.all(promises);
    console.log('All concurrent transactions completed');

    // Verify all users were inserted
    const result = await client.query('SELECT * FROM users WHERE name LIKE "Concurrent User%"');
    console.log(`Concurrent users inserted: ${result.rowCount}`);

  } catch (error) {
    console.error('Concurrent transactions error:', error.message);
  } finally {
    await client.close();
    console.log('Connection closed');
  }
}

async function batchOperationsExample() {
  const client = new MantisClient({
    host: 'localhost',
    port: 8080,
    username: 'admin',
    password: 'password'
  });

  try {
    await client.ping();
    console.log('\n=== Batch Operations Example ===');

    const tx = await client.beginTransaction();

    try {
      // Batch insert many records
      const batchSize = 10;
      const users = [];
      
      for (let i = 1; i <= batchSize; i++) {
        users.push({
          name: `Batch User ${i}`,
          email: `batch${i}@example.com`,
          age: 20 + (i % 30)
        });
      }

      console.log(`Inserting ${batchSize} users in batch...`);
      const startTime = Date.now();

      for (const user of users) {
        await tx.insert('users', user);
      }

      const endTime = Date.now();
      console.log(`Batch insert completed in ${endTime - startTime}ms`);

      // Batch update
      console.log('Performing batch updates...');
      const updateStartTime = Date.now();

      for (let i = 1; i <= batchSize; i++) {
        await tx.query(`UPDATE users SET age = age + 1 WHERE name = "Batch User ${i}"`);
      }

      const updateEndTime = Date.now();
      console.log(`Batch update completed in ${updateEndTime - updateStartTime}ms`);

      await tx.commit();
      console.log('Batch operations committed');

    } catch (error) {
      console.error('Batch operations error:', error.message);
      if (!tx.isClosed) {
        await tx.rollback();
        console.log('Batch operations rolled back');
      }
      throw error;
    }

  } catch (error) {
    console.error('Batch example error:', error.message);
  } finally {
    await client.close();
    console.log('Connection closed');
  }
}

// Run examples
async function main() {
  await transactionExample();
  await concurrentTransactionsExample();
  await batchOperationsExample();
}

main().catch(console.error);