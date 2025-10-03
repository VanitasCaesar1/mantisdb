"""
Synchronous MantisDB client example
"""

import mantisdb
from mantisdb import MantisConfig


def main():
    # Create configuration
    config = MantisConfig(
        host="localhost",
        port=8080,
        username="admin",
        password="password",
        retry_attempts=3,
        request_timeout=30.0
    )
    
    # Create client
    client = mantisdb.Client(config)
    
    try:
        # Test connection
        client.ping()
        print("Connected to MantisDB successfully!")
        
        # Insert data
        user_data = {
            "name": "John Doe",
            "email": "john@example.com",
            "age": 30,
            "active": True
        }
        
        client.insert("users", user_data)
        print("User inserted successfully")
        
        # Query data
        result = client.query("SELECT * FROM users WHERE age > 25")
        print(f"Found {result.row_count} users:")
        
        for row in result.rows:
            print(f"  - {row['name']} ({row['email']}) - Age: {row['age']}")
        
        # Get data with filters
        filters = {"active": True}
        active_users = client.get("users", filters)
        print(f"Active users: {active_users.row_count}")
        
        # Transaction example
        with client.begin_transaction() as tx:
            # Insert multiple users in a transaction
            users = [
                {"name": "Alice Smith", "email": "alice@example.com", "age": 25},
                {"name": "Bob Johnson", "email": "bob@example.com", "age": 35}
            ]
            
            for user in users:
                tx.insert("users", user)
            
            # Query within transaction
            tx_result = tx.query("SELECT COUNT(*) as count FROM users")
            print(f"Total users in transaction: {tx_result.rows[0]['count']}")
            
            # Transaction will auto-commit when exiting the context
        
        print("Transaction completed successfully")
        
        # Update example
        if result.rows:
            user_id = result.rows[0].get("id")
            if user_id:
                client.update("users", user_id, {"age": 31})
                print(f"Updated user {user_id}")
        
    except mantisdb.MantisError as e:
        print(f"MantisDB Error [{e.code}]: {e.message}")
        if e.details:
            print(f"Details: {e.details}")
    
    except Exception as e:
        print(f"Unexpected error: {e}")
    
    finally:
        client.close()
        print("Connection closed")


if __name__ == "__main__":
    main()