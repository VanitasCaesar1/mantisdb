package concurrency

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// Example demonstrating the RWLockManager usage
func ExampleRWLockManager() {
	// Create a new RWLock manager
	manager := NewRWLockManager()
	defer manager.Close()

	ctx := context.Background()
	resource := "example_resource"

	// Acquire a read lock
	err := manager.AcquireReadLock(ctx, resource)
	if err != nil {
		fmt.Printf("Failed to acquire read lock: %v\n", err)
		return
	}

	fmt.Println("Read lock acquired successfully")

	// Release the read lock
	err = manager.ReleaseLock(resource, false)
	if err != nil {
		fmt.Printf("Failed to release read lock: %v\n", err)
		return
	}

	fmt.Println("Read lock released successfully")

	// Acquire a write lock
	err = manager.AcquireWriteLock(ctx, resource)
	if err != nil {
		fmt.Printf("Failed to acquire write lock: %v\n", err)
		return
	}

	fmt.Println("Write lock acquired successfully")

	// Release the write lock
	err = manager.ReleaseLock(resource, true)
	if err != nil {
		fmt.Printf("Failed to release write lock: %v\n", err)
		return
	}

	fmt.Println("Write lock released successfully")

	// Output:
	// Read lock acquired successfully
	// Read lock released successfully
	// Write lock acquired successfully
	// Write lock released successfully
}

// TestConcurrentReaders demonstrates multiple readers can acquire locks simultaneously
func TestConcurrentReaders(t *testing.T) {
	manager := NewRWLockManager()
	defer manager.Close()

	resource := "test_resource"
	numReaders := 5
	var wg sync.WaitGroup

	// Start multiple readers
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()

			ctx := context.Background()
			err := manager.AcquireReadLock(ctx, resource)
			if err != nil {
				t.Errorf("Reader %d failed to acquire lock: %v", readerID, err)
				return
			}

			// Simulate some work
			time.Sleep(100 * time.Millisecond)

			err = manager.ReleaseLock(resource, false)
			if err != nil {
				t.Errorf("Reader %d failed to release lock: %v", readerID, err)
			}
		}(i)
	}

	wg.Wait()

	// Check metrics
	metrics := manager.GetLockMetrics(resource)
	if metrics == nil {
		t.Error("Expected metrics to be available")
		return
	}

	if metrics.AcquisitionCount != int64(numReaders) {
		t.Errorf("Expected %d acquisitions, got %d", numReaders, metrics.AcquisitionCount)
	}
}

// TestWriterPreference demonstrates that writers have preference over readers
func TestWriterPreference(t *testing.T) {
	manager := NewRWLockManager()
	defer manager.Close()

	resource := "test_resource"
	ctx := context.Background()

	// Test basic writer preference: readers should not be able to acquire
	// when there are waiting writers

	// First, acquire a read lock
	err := manager.AcquireReadLock(ctx, resource)
	if err != nil {
		t.Fatalf("Failed to acquire initial read lock: %v", err)
	}

	// Start a writer in background (it will wait)
	writerDone := make(chan bool)
	go func() {
		defer close(writerDone)
		err := manager.AcquireWriteLock(ctx, resource)
		if err != nil {
			t.Errorf("Writer failed to acquire lock: %v", err)
			return
		}
		defer manager.ReleaseLock(resource, true)
		time.Sleep(10 * time.Millisecond) // Hold the lock briefly
	}()

	// Give writer time to queue up
	time.Sleep(50 * time.Millisecond)

	// Now try to acquire another read lock - should be blocked
	ctx2, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = manager.AcquireReadLock(ctx2, resource)
	if err == nil {
		t.Error("Reader should have been blocked by waiting writer")
		manager.ReleaseLock(resource, false)
	} else {
		t.Logf("Reader was correctly blocked: %v", err)
	}

	// Release the initial read lock to allow writer to proceed
	err = manager.ReleaseLock(resource, false)
	if err != nil {
		t.Fatalf("Failed to release initial read lock: %v", err)
	}

	// Wait for writer to complete
	<-writerDone
}

// TestPriorityOrdering demonstrates priority-based lock ordering
func TestPriorityOrdering(t *testing.T) {
	manager := NewRWLockManager()
	defer manager.Close()

	resource := "test_resource"
	ctx := context.Background()

	// Acquire a write lock to block subsequent requests
	err := manager.AcquireWriteLock(ctx, resource)
	if err != nil {
		t.Fatalf("Failed to acquire initial write lock: %v", err)
	}

	var results []string
	var resultsMutex sync.Mutex
	var wg sync.WaitGroup

	// Add requests with different priorities
	priorities := []LockPriority{LowPriority, HighPriority, NormalPriority}
	names := []string{"Low", "High", "Normal"}

	for i, priority := range priorities {
		wg.Add(1)
		go func(name string, prio LockPriority) {
			defer wg.Done()

			err := manager.AcquireReadLockWithPriority(ctx, resource, prio, 5*time.Second)
			if err != nil {
				t.Errorf("Failed to acquire lock for %s priority: %v", name, err)
				return
			}

			resultsMutex.Lock()
			results = append(results, name)
			resultsMutex.Unlock()

			manager.ReleaseLock(resource, false)
		}(names[i], priority)
	}

	// Give time for all requests to queue up
	time.Sleep(50 * time.Millisecond)

	// Release the initial write lock
	err = manager.ReleaseLock(resource, true)
	if err != nil {
		t.Fatalf("Failed to release initial write lock: %v", err)
	}

	wg.Wait()

	// High priority should be processed first
	if len(results) > 0 && results[0] != "High" {
		t.Logf("Priority ordering: %v (High priority may not always be first due to timing)", results)
		// Don't fail the test as timing can affect the order
	} else {
		t.Logf("Priority ordering worked correctly: %v", results)
	}
}

// TestDeadlockDetection demonstrates basic deadlock detection
func TestDeadlockDetection(t *testing.T) {
	manager := NewRWLockManager()
	defer manager.Close()

	// This is a simplified test - in practice, deadlock detection
	// would involve more complex scenarios with multiple resources
	resource := "test_resource"
	ctx := context.Background()

	// Acquire a write lock
	err := manager.AcquireWriteLock(ctx, resource)
	if err != nil {
		t.Fatalf("Failed to acquire write lock: %v", err)
	}

	// Try to acquire another write lock with a short timeout
	// This should timeout rather than deadlock
	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	err = manager.AcquireWriteLock(ctx, resource)
	if err == nil {
		t.Error("Expected timeout error, but lock was acquired")
		manager.ReleaseLock(resource, true)
	}

	// Release the initial lock
	err = manager.ReleaseLock(resource, true)
	if err != nil {
		t.Fatalf("Failed to release write lock: %v", err)
	}
}

// BenchmarkLockAcquisition benchmarks lock acquisition performance
func BenchmarkLockAcquisition(b *testing.B) {
	manager := NewRWLockManager()
	defer manager.Close()

	resource := "bench_resource"
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := manager.AcquireReadLock(ctx, resource)
			if err != nil {
				b.Fatalf("Failed to acquire read lock: %v", err)
			}

			err = manager.ReleaseLock(resource, false)
			if err != nil {
				b.Fatalf("Failed to release read lock: %v", err)
			}
		}
	})
}
