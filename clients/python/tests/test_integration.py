"""
Integration tests for MantisDB Python client
"""

import asyncio
import os
import pytest
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from typing import Dict, Any

from mantisdb.client import Client, AsyncClient, MantisConfig, MantisError
from mantisdb.auth import BasicAuthProvider, APIKeyAuthProvider, JWTAuthProvider


# Test configuration
TEST_HOST = os.getenv("MANTISDB_TEST_HOST", "localhost")
TEST_PORT = int(os.getenv("MANTISDB_TEST_PORT", "8080"))
TEST_USERNAME = os.getenv("MANTISDB_TEST_USERNAME", "admin")
TEST_PASSWORD = os.getenv("MANTISDB_TEST_PASSWORD", "password")
TEST_API_KEY = os.getenv("MANTISDB_TEST_API_KEY", "")


def create_test_config() -> MantisConfig:
    """Create test configuration"""
    return MantisConfig(
        host=TEST_HOST,
        port=TEST_PORT,
        username=TEST_USERNAME,
        password=TEST_PASSWORD,
        request_timeout=10.0,
        connection_timeout=5.0
    )


def create_test_client() -> Client:
    """Create test client"""
    config = create_test_config()
    return Client(config)


def create_test_async_client() -> AsyncClient:
    """Create test async client"""
    config = create_test_config()
    return AsyncClient(config)


class TestSyncClient:
    """Test cases for synchronous client"""

    def test_connection(self):
        """Test basic connection"""
        client = create_test_client()
        try:
            client.ping()
        finally:
            client.close()

    def test_basic_operations(self):
        """Test basic CRUD operations"""
        client = create_test_client()
        table_name = f"test_users_{int(time.time())}"
        
        try:
            # Create table
            create_table_sql = f"""
                CREATE TABLE {table_name} (
                    id INTEGER PRIMARY KEY,
                    name TEXT NOT NULL,
                    email TEXT UNIQUE,
                    age INTEGER,
                    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                )
            """
            client.query(create_table_sql)

            # Insert data
            user_data = {
                "name": "John Doe",
                "email": "john@example.com",
                "age": 30
            }
            client.insert(table_name, user_data)

            # Query data
            result = client.query(f"SELECT * FROM {table_name} WHERE name = 'John Doe'")
            assert result.row_count == 1
            assert result.rows[0]["name"] == "John Doe"

            # Get data with filters
            filters = {"age": 30}
            result = client.get(table_name, filters)
            assert result.row_count == 1

            # Update data
            user_id = str(result.rows[0]["id"])
            update_data = {"age": 31}
            client.update(table_name, user_id, update_data)

            # Verify update
            result = client.query(f"SELECT age FROM {table_name} WHERE id = {user_id}")
            assert result.rows[0]["age"] == 31

            # Delete data
            client.delete(table_name, user_id)

            # Verify deletion
            result = client.query(f"SELECT COUNT(*) as count FROM {table_name}")
            assert result.rows[0]["count"] == 0

        finally:
            try:
                client.query(f"DROP TABLE {table_name}")
            except:
                pass
            client.close()

    def test_transactions(self):
        """Test transaction operations"""
        client = create_test_client()
        table_name = f"test_transactions_{int(time.time())}"
        
        try:
            # Create table
            create_table_sql = f"""
                CREATE TABLE {table_name} (
                    id INTEGER PRIMARY KEY,
                    name TEXT NOT NULL,
                    balance INTEGER DEFAULT 0
                )
            """
            client.query(create_table_sql)

            # Test successful transaction
            with client.begin_transaction() as tx:
                user_data1 = {"name": "Alice", "balance": 1000}
                user_data2 = {"name": "Bob", "balance": 500}
                
                tx.insert(table_name, user_data1)
                tx.insert(table_name, user_data2)
                
                # Query within transaction
                result = tx.query(f"SELECT COUNT(*) as count FROM {table_name}")
                assert result.rows[0]["count"] == 2

            # Verify data persisted
            result = client.query(f"SELECT COUNT(*) as count FROM {table_name}")
            assert result.rows[0]["count"] == 2

            # Test rollback transaction
            try:
                with client.begin_transaction() as tx:
                    user_data3 = {"name": "Charlie", "balance": 750}
                    tx.insert(table_name, user_data3)
                    raise Exception("Force rollback")
            except Exception:
                pass

            # Verify data was not persisted
            result = client.query(f"SELECT COUNT(*) as count FROM {table_name}")
            assert result.rows[0]["count"] == 2

        finally:
            try:
                client.query(f"DROP TABLE {table_name}")
            except:
                pass
            client.close()

    def test_authentication(self):
        """Test different authentication methods"""
        # Test Basic Auth
        config = create_test_config()
        client = Client(config)
        try:
            client.ping()
        finally:
            client.close()

        # Test API Key Auth (if API key is provided)
        if TEST_API_KEY:
            config = MantisConfig(
                host=TEST_HOST,
                port=TEST_PORT,
                api_key=TEST_API_KEY
            )
            client = Client(config)
            try:
                client.ping()
            finally:
                client.close()

    def test_error_handling(self):
        """Test error handling"""
        client = create_test_client()
        
        try:
            # Test invalid SQL
            with pytest.raises(MantisError) as exc_info:
                client.query("INVALID SQL STATEMENT")
            
            assert exc_info.value.code != ""

            # Test non-existent table
            with pytest.raises(MantisError):
                client.query("SELECT * FROM non_existent_table")

            # Test invalid insert
            with pytest.raises(MantisError):
                client.insert("non_existent_table", {"field": "value"})

        finally:
            client.close()

    def test_concurrent_operations(self):
        """Test concurrent operations"""
        client = create_test_client()
        table_name = f"test_concurrency_{int(time.time())}"
        
        try:
            # Create table
            create_table_sql = f"""
                CREATE TABLE {table_name} (
                    id INTEGER PRIMARY KEY,
                    worker_id INTEGER,
                    value INTEGER
                )
            """
            client.query(create_table_sql)

            # Test concurrent operations
            num_workers = 5
            num_operations_per_worker = 3
            
            def worker_task(worker_id: int):
                errors = []
                for j in range(num_operations_per_worker):
                    try:
                        data = {
                            "worker_id": worker_id,
                            "value": j
                        }
                        client.insert(table_name, data)
                    except Exception as e:
                        errors.append(f"Worker {worker_id} operation {j} failed: {e}")
                return errors

            with ThreadPoolExecutor(max_workers=num_workers) as executor:
                futures = [executor.submit(worker_task, i) for i in range(num_workers)]
                all_errors = []
                
                for future in as_completed(futures):
                    errors = future.result()
                    all_errors.extend(errors)

            # Check for errors
            assert len(all_errors) == 0, f"Concurrent operation errors: {all_errors}"

            # Verify all data was inserted
            result = client.query(f"SELECT COUNT(*) as count FROM {table_name}")
            expected_count = num_workers * num_operations_per_worker
            assert result.rows[0]["count"] == expected_count

        finally:
            try:
                client.query(f"DROP TABLE {table_name}")
            except:
                pass
            client.close()

    def test_connection_string_parsing(self):
        """Test connection string parsing"""
        connection_string = f"mantisdb://{TEST_USERNAME}:{TEST_PASSWORD}@{TEST_HOST}:{TEST_PORT}"
        client = Client(connection_string)
        
        try:
            client.ping()
        finally:
            client.close()

    def test_retry_mechanism(self):
        """Test retry mechanism"""
        config = create_test_config()
        config.retry_attempts = 3
        config.retry_delay = 0.1
        
        client = Client(config)
        
        try:
            # Test that normal operations work with retry enabled
            client.ping()
        finally:
            client.close()


class TestAsyncClient:
    """Test cases for asynchronous client"""

    @pytest.mark.asyncio
    async def test_connection(self):
        """Test basic async connection"""
        client = create_test_async_client()
        try:
            await client.ping()
        finally:
            await client.close()

    @pytest.mark.asyncio
    async def test_basic_operations(self):
        """Test basic async CRUD operations"""
        client = create_test_async_client()
        table_name = f"test_async_users_{int(time.time())}"
        
        try:
            # Create table
            create_table_sql = f"""
                CREATE TABLE {table_name} (
                    id INTEGER PRIMARY KEY,
                    name TEXT NOT NULL,
                    email TEXT UNIQUE,
                    age INTEGER
                )
            """
            await client.query(create_table_sql)

            # Insert data
            user_data = {
                "name": "Jane Doe",
                "email": "jane@example.com",
                "age": 25
            }
            await client.insert(table_name, user_data)

            # Query data
            result = await client.query(f"SELECT * FROM {table_name} WHERE name = 'Jane Doe'")
            assert result.row_count == 1
            assert result.rows[0]["name"] == "Jane Doe"

            # Get data with filters
            filters = {"age": 25}
            result = await client.get(table_name, filters)
            assert result.row_count == 1

            # Update data
            user_id = str(result.rows[0]["id"])
            update_data = {"age": 26}
            await client.update(table_name, user_id, update_data)

            # Verify update
            result = await client.query(f"SELECT age FROM {table_name} WHERE id = {user_id}")
            assert result.rows[0]["age"] == 26

            # Delete data
            await client.delete(table_name, user_id)

            # Verify deletion
            result = await client.query(f"SELECT COUNT(*) as count FROM {table_name}")
            assert result.rows[0]["count"] == 0

        finally:
            try:
                await client.query(f"DROP TABLE {table_name}")
            except:
                pass
            await client.close()

    @pytest.mark.asyncio
    async def test_transactions(self):
        """Test async transaction operations"""
        client = create_test_async_client()
        table_name = f"test_async_transactions_{int(time.time())}"
        
        try:
            # Create table
            create_table_sql = f"""
                CREATE TABLE {table_name} (
                    id INTEGER PRIMARY KEY,
                    name TEXT NOT NULL,
                    balance INTEGER DEFAULT 0
                )
            """
            await client.query(create_table_sql)

            # Test successful transaction
            async with client.begin_transaction() as tx:
                user_data1 = {"name": "Alice", "balance": 1000}
                user_data2 = {"name": "Bob", "balance": 500}
                
                await tx.insert(table_name, user_data1)
                await tx.insert(table_name, user_data2)
                
                # Query within transaction
                result = await tx.query(f"SELECT COUNT(*) as count FROM {table_name}")
                assert result.rows[0]["count"] == 2

            # Verify data persisted
            result = await client.query(f"SELECT COUNT(*) as count FROM {table_name}")
            assert result.rows[0]["count"] == 2

            # Test rollback transaction
            try:
                async with client.begin_transaction() as tx:
                    user_data3 = {"name": "Charlie", "balance": 750}
                    await tx.insert(table_name, user_data3)
                    raise Exception("Force rollback")
            except Exception:
                pass

            # Verify data was not persisted
            result = await client.query(f"SELECT COUNT(*) as count FROM {table_name}")
            assert result.rows[0]["count"] == 2

        finally:
            try:
                await client.query(f"DROP TABLE {table_name}")
            except:
                pass
            await client.close()

    @pytest.mark.asyncio
    async def test_concurrent_operations(self):
        """Test async concurrent operations"""
        client = create_test_async_client()
        table_name = f"test_async_concurrency_{int(time.time())}"
        
        try:
            # Create table
            create_table_sql = f"""
                CREATE TABLE {table_name} (
                    id INTEGER PRIMARY KEY,
                    worker_id INTEGER,
                    value INTEGER
                )
            """
            await client.query(create_table_sql)

            # Test concurrent operations
            num_workers = 5
            num_operations_per_worker = 3
            
            async def worker_task(worker_id: int):
                errors = []
                for j in range(num_operations_per_worker):
                    try:
                        data = {
                            "worker_id": worker_id,
                            "value": j
                        }
                        await client.insert(table_name, data)
                    except Exception as e:
                        errors.append(f"Worker {worker_id} operation {j} failed: {e}")
                return errors

            # Run concurrent tasks
            tasks = [worker_task(i) for i in range(num_workers)]
            results = await asyncio.gather(*tasks)
            
            # Check for errors
            all_errors = []
            for errors in results:
                all_errors.extend(errors)
            
            assert len(all_errors) == 0, f"Concurrent operation errors: {all_errors}"

            # Verify all data was inserted
            result = await client.query(f"SELECT COUNT(*) as count FROM {table_name}")
            expected_count = num_workers * num_operations_per_worker
            assert result.rows[0]["count"] == expected_count

        finally:
            try:
                await client.query(f"DROP TABLE {table_name}")
            except:
                pass
            await client.close()

    @pytest.mark.asyncio
    async def test_error_handling(self):
        """Test async error handling"""
        client = create_test_async_client()
        
        try:
            # Test invalid SQL
            with pytest.raises(MantisError) as exc_info:
                await client.query("INVALID SQL STATEMENT")
            
            assert exc_info.value.code != ""

            # Test non-existent table
            with pytest.raises(MantisError):
                await client.query("SELECT * FROM non_existent_table")

            # Test invalid insert
            with pytest.raises(MantisError):
                await client.insert("non_existent_table", {"field": "value"})

        finally:
            await client.close()


class TestAuthProviders:
    """Test authentication providers"""

    def test_basic_auth_provider(self):
        """Test BasicAuthProvider"""
        provider = BasicAuthProvider(TEST_USERNAME, TEST_PASSWORD)
        
        # Test authentication
        import requests
        session = requests.Session()
        token = provider.authenticate(session, f"http://{TEST_HOST}:{TEST_PORT}")
        
        assert token.access_token == "basic_auth"
        assert token.token_type == "Basic"
        
        # Test headers
        headers = provider.get_auth_headers(token)
        assert "Authorization" in headers
        assert headers["Authorization"].startswith("Basic ")

    def test_api_key_auth_provider(self):
        """Test APIKeyAuthProvider"""
        api_key = "test-api-key"
        provider = APIKeyAuthProvider(api_key)
        
        # Test authentication
        import requests
        session = requests.Session()
        token = provider.authenticate(session, f"http://{TEST_HOST}:{TEST_PORT}")
        
        assert token.access_token == api_key
        assert token.token_type == "ApiKey"
        
        # Test headers
        headers = provider.get_auth_headers(token)
        assert "X-API-Key" in headers
        assert headers["X-API-Key"] == api_key

    @pytest.mark.asyncio
    async def test_async_basic_auth_provider(self):
        """Test async BasicAuthProvider"""
        provider = BasicAuthProvider(TEST_USERNAME, TEST_PASSWORD)
        
        # Test async authentication
        import aiohttp
        async with aiohttp.ClientSession() as session:
            token = await provider.authenticate_async(session, f"http://{TEST_HOST}:{TEST_PORT}")
            
            assert token.access_token == "basic_auth"
            assert token.token_type == "Basic"


class TestPerformance:
    """Performance and load tests"""

    def test_query_performance(self):
        """Test query performance"""
        client = create_test_client()
        
        try:
            # Warm up
            for _ in range(5):
                client.query("SELECT 1")
            
            # Measure performance
            start_time = time.time()
            num_queries = 100
            
            for _ in range(num_queries):
                result = client.query("SELECT 1")
                assert result.row_count == 1
            
            end_time = time.time()
            duration = end_time - start_time
            queries_per_second = num_queries / duration
            
            print(f"Query performance: {queries_per_second:.2f} queries/second")
            
            # Basic performance assertion (should be able to do at least 10 queries/second)
            assert queries_per_second > 10

        finally:
            client.close()

    @pytest.mark.asyncio
    async def test_async_query_performance(self):
        """Test async query performance"""
        client = create_test_async_client()
        
        try:
            # Warm up
            for _ in range(5):
                await client.query("SELECT 1")
            
            # Measure performance
            start_time = time.time()
            num_queries = 100
            
            # Run queries concurrently
            tasks = [client.query("SELECT 1") for _ in range(num_queries)]
            results = await asyncio.gather(*tasks)
            
            end_time = time.time()
            duration = end_time - start_time
            queries_per_second = num_queries / duration
            
            print(f"Async query performance: {queries_per_second:.2f} queries/second")
            
            # Async should be faster than sync
            assert queries_per_second > 20
            
            # Verify all queries succeeded
            for result in results:
                assert result.row_count == 1

        finally:
            await client.close()


class TestCrossPlatformCompatibility:
    """Cross-platform compatibility tests"""

    def test_unicode_handling(self):
        """Test Unicode string handling"""
        client = create_test_client()
        table_name = f"test_unicode_{int(time.time())}"
        
        try:
            # Create table
            create_table_sql = f"""
                CREATE TABLE {table_name} (
                    id INTEGER PRIMARY KEY,
                    name TEXT,
                    description TEXT
                )
            """
            client.query(create_table_sql)

            # Insert Unicode data
            unicode_data = {
                "name": "测试用户",  # Chinese
                "description": "Тестовый пользователь"  # Russian
            }
            client.insert(table_name, unicode_data)

            # Query Unicode data
            result = client.query(f"SELECT * FROM {table_name}")
            assert result.row_count == 1
            assert result.rows[0]["name"] == "测试用户"
            assert result.rows[0]["description"] == "Тестовый пользователь"

        finally:
            try:
                client.query(f"DROP TABLE {table_name}")
            except:
                pass
            client.close()

    def test_large_data_handling(self):
        """Test handling of large data"""
        client = create_test_client()
        table_name = f"test_large_data_{int(time.time())}"
        
        try:
            # Create table
            create_table_sql = f"""
                CREATE TABLE {table_name} (
                    id INTEGER PRIMARY KEY,
                    large_text TEXT
                )
            """
            client.query(create_table_sql)

            # Insert large text data (1MB)
            large_text = "A" * (1024 * 1024)  # 1MB of 'A' characters
            large_data = {
                "large_text": large_text
            }
            client.insert(table_name, large_data)

            # Query large data
            result = client.query(f"SELECT large_text FROM {table_name}")
            assert result.row_count == 1
            assert len(result.rows[0]["large_text"]) == 1024 * 1024

        finally:
            try:
                client.query(f"DROP TABLE {table_name}")
            except:
                pass
            client.close()


if __name__ == "__main__":
    pytest.main([__file__, "-v"])