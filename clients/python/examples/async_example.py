"""
Asynchronous MantisDB client example
"""

import asyncio
import mantisdb
from mantisdb import MantisConfig


async def main():
    # Create configuration
    config = MantisConfig(
        host="localhost",
        port=8080,
        username="admin",
        password="password",
        max_connections=20,
        retry_attempts=3,
        request_timeout=30.0
    )
    
    # Create async client
    client = mantisdb.AsyncClient(config)
    
    try:
        # Test connection
        await client.ping()
        print("Connected to MantisDB successfully!")
        
        # Insert data
        user_data = {
            "name": "Jane Doe",
            "email": "jane@example.com",
            "age": 28,
            "active": True
        }
        
        await client.insert("users", user_data)
        print("User inserted successfully")
        
        # Query data
        result = await client.query("SELECT * FROM users WHERE age > 25")
        print(f"Found {result.row_count} users:")
        
        for row in result.rows:
            print(f"  - {row['name']} ({row['email']}) - Age: {row['age']}")
        
        # Get data with filters
        filters = {"active": True}
        active_users = await client.get("users", filters)
        print(f"Active users: {active_users.row_count}")
        
        # Async transaction example
        async with await client.begin_transaction() as tx:
            # Insert multiple users in a transaction
            users = [
                {"name": "Charlie Brown", "email": "charlie@example.com", "age": 22},
                {"name": "Diana Prince", "email": "diana@example.com", "age": 32}
            ]
            
            for user in users:
                await tx.insert("users", user)
            
            # Query within transaction
            tx_result = await tx.query("SELECT COUNT(*) as count FROM users")
            print(f"Total users in transaction: {tx_result.rows[0]['count']}")
            
            # Transaction will auto-commit when exiting the context
        
        print("Async transaction completed successfully")
        
        # Concurrent operations example
        tasks = []
        for i in range(5):
            user = {
                "name": f"User {i}",
                "email": f"user{i}@example.com",
                "age": 20 + i
            }
            tasks.append(client.insert("users", user))
        
        # Execute all inserts concurrently
        await asyncio.gather(*tasks)
        print("Concurrent inserts completed")
        
        # Update example
        if result.rows:
            user_id = result.rows[0].get("id")
            if user_id:
                await client.update("users", user_id, {"age": 29})
                print(f"Updated user {user_id}")
        
    except mantisdb.MantisError as e:
        print(f"MantisDB Error [{e.code}]: {e.message}")
        if e.details:
            print(f"Details: {e.details}")
    
    except Exception as e:
        print(f"Unexpected error: {e}")
    
    finally:
        await client.close()
        print("Connection closed")


if __name__ == "__main__":
    asyncio.run(main())