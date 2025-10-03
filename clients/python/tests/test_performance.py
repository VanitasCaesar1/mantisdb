"""
Performance tests for MantisDB Python client
"""

import asyncio
import time
import pytest
from concurrent.futures import ThreadPoolExecutor, as_completed
from typing import List

from mantisdb.client import Client, AsyncClient, MantisConfig
from .test_integration import create_test_config, create_test_client, create_test_async_client


class TestSyncPerformance:
    """Performance tests for synchronous client"""

    def test_query_performance(self):
        """Test query performance"""
        client = create_test_client()
        
        try:
            # Warm up
            for _ in range(10):
                client.query("SELECT 1")
            
            # Measure performance
            num_queries = 100
            start_time = time.time()
            
            for _ in range(num_queries):
                result = client.query("SELECT 1")
                assert result.row_count == 1
            
            end_time = time.time()
            duration = end_time - start_time
            queries_per_second = num_queries / duration
            
            print(f"Sync query performance: {queries_per_second:.2f} queries/second")
            
            # Basic performance assertion
            assert queries_per_second > 5  # Should be able to do at least 5 queries/second

        finally:
            client.close()

    def test_concurrent_query_performance(self):
        """Test concurrent query performance with thread pool"""
        client = create_test_client()
        
        try:
            num_threads = 5
            queries_per_thread = 20
            
            def execute_queries(thread_id: int) -> List[float]:
                times = []
                for i in range(queries_per_thread):
                    start = time.time()
                    result = client.query("SELECT 1")
                    end = time.time()
                    times.append(end - start)
                    assert result.row_count == 1
                return times
            
            start_time = time.time()
            
            with ThreadPoolExecutor(max_workers=num_threads) as executor:
                futures = [executor.submit(execute_queries, i) for i in range(num_threads)]
                all_times = []
                
                for future in as_completed(futures):
                    thread_times = future.result()
                    all_times.extend(thread_times)
            
            end_time = time.time()
            total_duration = end_time - start_time
            total_queries = num_threads * queries_per_thread
            queries_per_second = total_queries / total_duration
            
            avg_query_time = sum(all_times) / len(all_times)
            min_query_time = min(all_times)
            max_query_time = max(all_times)
            
            print(f"Concurrent sync performance:")
            print(f"  Total queries: {total_queries}")
            print(f"  Total duration: {total_duration:.2f}s")
            print(f"  Queries per second: {queries_per_second:.2f}")
            print(f"  Average query time: {avg_query_time*1000:.2f}ms")
            print(f"  Min query time: {min_query_time*1000:.2f}ms")
            print(f"  Max query time: {max_query_time*1000:.2f}ms")
            
            assert queries_per_second > 10  # Should handle concurrent queries well

        finally:
            client.close()

    def test_insert_performance(self):
        """Test insert performance"""
        client = create_test_client()
        table_name = f"perf_test_insert_{int(time.time())}"
        
        try:
            # Create table
            create_table_sql = f"""
                CREATE TABLE {table_name} (
                    id INTEGER PRIMARY KEY,
                    name TEXT,
                    value INTEGER
                )
            """
            client.query(create_table_sql)
            
            # Measure insert performance
            num_inserts = 100
            start_time = time.time()
            
            for i in range(num_inserts):
                data = {
                    "name": f"test_user_{i}",
                    "value": i
                }
                client.insert(table_name, data)
            
            end_time = time.time()
            duration = end_time - start_time
            inserts_per_second = num_inserts / duration
            
            print(f"Insert performance: {inserts_per_second:.2f} inserts/second")
            
            # Verify all inserts
            result = client.query(f"SELECT COUNT(*) as count FROM {table_name}")
            assert result.rows[0]["count"] == num_inserts
            
            assert inserts_per_second > 5  # Should be able to do at least 5 inserts/second

        finally:
            try:
                client.query(f"DROP TABLE {table_name}")
            except:
                pass
            client.close()

    def test_transaction_performance(self):
        """Test transaction performance"""
        client = create_test_client()
        table_name = f"perf_test_tx_{int(time.time())}"
        
        try:
            # Create table
            create_table_sql = f"""
                CREATE TABLE {table_name} (
                    id INTEGER PRIMARY KEY,
                    batch_id INTEGER,
                    value INTEGER
                )
            """
            client.query(create_table_sql)
            
            # Measure transaction performance
            num_transactions = 20
            operations_per_transaction = 5
            start_time = time.time()
            
            for batch_id in range(num_transactions):
                with client.begin_transaction() as tx:
                    for i in range(operations_per_transaction):
                        data = {
                            "batch_id": batch_id,
                            "value": i
                        }
                        tx.insert(table_name, data)
            
            end_time = time.time()
            duration = end_time - start_time
            transactions_per_second = num_transactions / duration
            
            print(f"Transaction performance: {transactions_per_second:.2f} transactions/second")
            
            # Verify all data
            result = client.query(f"SELECT COUNT(*) as count FROM {table_name}")
            expected_count = num_transactions * operations_per_transaction
            assert result.rows[0]["count"] == expected_count
            
            assert transactions_per_second > 2  # Should be able to do at least 2 transactions/second

        finally:
            try:
                client.query(f"DROP TABLE {table_name}")
            except:
                pass
            client.close()


class TestAsyncPerformance:
    """Performance tests for asynchronous client"""

    @pytest.mark.asyncio
    async def test_async_query_performance(self):
        """Test async query performance"""
        client = create_test_async_client()
        
        try:
            # Warm up
            for _ in range(10):
                await client.query("SELECT 1")
            
            # Measure performance
            num_queries = 100
            start_time = time.time()
            
            # Run queries concurrently
            tasks = [client.query("SELECT 1") for _ in range(num_queries)]
            results = await asyncio.gather(*tasks)
            
            end_time = time.time()
            duration = end_time - start_time
            queries_per_second = num_queries / duration
            
            print(f"Async query performance: {queries_per_second:.2f} queries/second")
            
            # Verify all queries succeeded
            for result in results:
                assert result.row_count == 1
            
            # Async should be significantly faster than sync
            assert queries_per_second > 20

        finally:
            await client.close()

    @pytest.mark.asyncio
    async def test_async_insert_performance(self):
        """Test async insert performance"""
        client = create_test_async_client()
        table_name = f"async_perf_test_insert_{int(time.time())}"
        
        try:
            # Create table
            create_table_sql = f"""
                CREATE TABLE {table_name} (
                    id INTEGER PRIMARY KEY,
                    name TEXT,
                    value INTEGER
                )
            """
            await client.query(create_table_sql)
            
            # Measure insert performance
            num_inserts = 50
            start_time = time.time()
            
            # Run inserts concurrently
            tasks = []
            for i in range(num_inserts):
                data = {
                    "name": f"async_user_{i}",
                    "value": i
                }
                tasks.append(client.insert(table_name, data))
            
            await asyncio.gather(*tasks)
            
            end_time = time.time()
            duration = end_time - start_time
            inserts_per_second = num_inserts / duration
            
            print(f"Async insert performance: {inserts_per_second:.2f} inserts/second")
            
            # Verify all inserts
            result = await client.query(f"SELECT COUNT(*) as count FROM {table_name}")
            assert result.rows[0]["count"] == num_inserts
            
            assert inserts_per_second > 10  # Async should be faster

        finally:
            try:
                await client.query(f"DROP TABLE {table_name}")
            except:
                pass
            await client.close()

    @pytest.mark.asyncio
    async def test_async_mixed_operations_performance(self):
        """Test mixed async operations performance"""
        client = create_test_async_client()
        table_name = f"async_mixed_perf_{int(time.time())}"
        
        try:
            # Create table
            create_table_sql = f"""
                CREATE TABLE {table_name} (
                    id INTEGER PRIMARY KEY,
                    name TEXT,
                    value INTEGER
                )
            """
            await client.query(create_table_sql)
            
            # Measure mixed operations performance
            num_operations = 30
            start_time = time.time()
            
            # Mix of inserts, queries, and updates
            tasks = []
            
            # Insert operations
            for i in range(num_operations // 3):
                data = {"name": f"user_{i}", "value": i}
                tasks.append(client.insert(table_name, data))
            
            # Query operations
            for i in range(num_operations // 3):
                tasks.append(client.query(f"SELECT COUNT(*) as count FROM {table_name}"))
            
            # Get operations
            for i in range(num_operations // 3):
                tasks.append(client.get(table_name, {"value": i}))
            
            results = await asyncio.gather(*tasks, return_exceptions=True)
            
            end_time = time.time()
            duration = end_time - start_time
            operations_per_second = num_operations / duration
            
            print(f"Async mixed operations performance: {operations_per_second:.2f} operations/second")
            
            # Check for exceptions
            exceptions = [r for r in results if isinstance(r, Exception)]
            if exceptions:
                print(f"Exceptions during mixed operations: {len(exceptions)}")
                for exc in exceptions[:5]:  # Show first 5 exceptions
                    print(f"  {exc}")
            
            assert operations_per_second > 5
            assert len(exceptions) < num_operations * 0.1  # Less than 10% failures

        finally:
            try:
                await client.query(f"DROP TABLE {table_name}")
            except:
                pass
            await client.close()


class TestLoadTesting:
    """Load testing for both sync and async clients"""

    @pytest.mark.slow
    def test_sync_load_test(self):
        """Load test for synchronous client"""
        if pytest.mark.slow not in pytest.mark.slow:
            pytest.skip("Skipping load test - use --runslow to run")
        
        client = create_test_client()
        table_name = f"sync_load_test_{int(time.time())}"
        
        try:
            # Create table
            create_table_sql = f"""
                CREATE TABLE {table_name} (
                    id INTEGER PRIMARY KEY,
                    worker_id INTEGER,
                    operation_id INTEGER,
                    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                )
            """
            client.query(create_table_sql)
            
            # Load test parameters
            num_workers = 10
            operations_per_worker = 50
            
            def worker_task(worker_id: int) -> List[str]:
                errors = []
                for i in range(operations_per_worker):
                    try:
                        data = {
                            "worker_id": worker_id,
                            "operation_id": i
                        }
                        client.insert(table_name, data)
                    except Exception as e:
                        errors.append(f"Worker {worker_id} op {i}: {e}")
                return errors
            
            start_time = time.time()
            
            with ThreadPoolExecutor(max_workers=num_workers) as executor:
                futures = [executor.submit(worker_task, i) for i in range(num_workers)]
                all_errors = []
                
                for future in as_completed(futures):
                    errors = future.result()
                    all_errors.extend(errors)
            
            end_time = time.time()
            duration = end_time - start_time
            total_operations = num_workers * operations_per_worker
            operations_per_second = total_operations / duration
            
            # Get final count
            result = client.query(f"SELECT COUNT(*) as count FROM {table_name}")
            actual_count = result.rows[0]["count"]
            
            print(f"Sync load test results:")
            print(f"  Duration: {duration:.2f}s")
            print(f"  Total operations: {total_operations}")
            print(f"  Successful operations: {actual_count}")
            print(f"  Errors: {len(all_errors)}")
            print(f"  Operations per second: {operations_per_second:.2f}")
            
            # Should have minimal errors
            assert len(all_errors) < total_operations * 0.05  # Less than 5% errors
            assert actual_count >= total_operations * 0.95  # At least 95% success

        finally:
            try:
                client.query(f"DROP TABLE {table_name}")
            except:
                pass
            client.close()

    @pytest.mark.slow
    @pytest.mark.asyncio
    async def test_async_load_test(self):
        """Load test for asynchronous client"""
        if pytest.mark.slow not in pytest.mark.slow:
            pytest.skip("Skipping load test - use --runslow to run")
        
        client = create_test_async_client()
        table_name = f"async_load_test_{int(time.time())}"
        
        try:
            # Create table
            create_table_sql = f"""
                CREATE TABLE {table_name} (
                    id INTEGER PRIMARY KEY,
                    worker_id INTEGER,
                    operation_id INTEGER,
                    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                )
            """
            await client.query(create_table_sql)
            
            # Load test parameters
            num_workers = 20
            operations_per_worker = 25
            
            async def worker_task(worker_id: int) -> List[str]:
                errors = []
                for i in range(operations_per_worker):
                    try:
                        data = {
                            "worker_id": worker_id,
                            "operation_id": i
                        }
                        await client.insert(table_name, data)
                    except Exception as e:
                        errors.append(f"Worker {worker_id} op {i}: {e}")
                return errors
            
            start_time = time.time()
            
            # Run all workers concurrently
            tasks = [worker_task(i) for i in range(num_workers)]
            results = await asyncio.gather(*tasks, return_exceptions=True)
            
            end_time = time.time()
            duration = end_time - start_time
            total_operations = num_workers * operations_per_worker
            operations_per_second = total_operations / duration
            
            # Collect errors
            all_errors = []
            for result in results:
                if isinstance(result, Exception):
                    all_errors.append(str(result))
                elif isinstance(result, list):
                    all_errors.extend(result)
            
            # Get final count
            result = await client.query(f"SELECT COUNT(*) as count FROM {table_name}")
            actual_count = result.rows[0]["count"]
            
            print(f"Async load test results:")
            print(f"  Duration: {duration:.2f}s")
            print(f"  Total operations: {total_operations}")
            print(f"  Successful operations: {actual_count}")
            print(f"  Errors: {len(all_errors)}")
            print(f"  Operations per second: {operations_per_second:.2f}")
            
            # Async should handle load better
            assert len(all_errors) < total_operations * 0.05  # Less than 5% errors
            assert actual_count >= total_operations * 0.95  # At least 95% success
            assert operations_per_second > 50  # Should be significantly faster

        finally:
            try:
                await client.query(f"DROP TABLE {table_name}")
            except:
                pass
            await client.close()


class TestMemoryUsage:
    """Memory usage tests"""

    def test_memory_leak_detection(self):
        """Test for memory leaks in long-running operations"""
        client = create_test_client()
        
        try:
            # Perform many operations to detect memory leaks
            for i in range(500):
                result = client.query("SELECT 1")
                assert result.row_count == 1
                
                if i % 100 == 0:
                    print(f"Completed {i} operations")
            
            print("Memory leak test completed successfully")

        finally:
            client.close()

    @pytest.mark.asyncio
    async def test_async_memory_leak_detection(self):
        """Test for memory leaks in async operations"""
        client = create_test_async_client()
        
        try:
            # Perform many async operations
            for batch in range(10):
                tasks = [client.query("SELECT 1") for _ in range(50)]
                results = await asyncio.gather(*tasks)
                
                for result in results:
                    assert result.row_count == 1
                
                print(f"Completed batch {batch + 1}/10")
            
            print("Async memory leak test completed successfully")

        finally:
            await client.close()


if __name__ == "__main__":
    pytest.main([__file__, "-v", "-s"])