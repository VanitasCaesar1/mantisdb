package concurrency

import (
	"context"
	"fmt"
	"log"
	"time"
)

// ExampleUsage demonstrates how to use the enhanced concurrency system
func ExampleUsage() {
	// Create and configure the concurrency system
	config := DefaultConcurrencySystemConfig()
	config.EnableProfiling = true
	config.EnableMetrics = true
	config.EnableGoroutineManagement = true

	system := NewEnhancedConcurrencySystem(config)

	// Start the system
	if err := system.Start(); err != nil {
		log.Fatalf("Failed to start concurrency system: %v", err)
	}
	defer system.Stop()

	// Example 1: Basic lock usage with monitoring
	fmt.Println("=== Example 1: Basic Lock Usage ===")
	txnID := uint64(1)
	resource := "user:123"

	// Acquire a read lock
	if err := system.AcquireLock(txnID, resource, ReadLock); err != nil {
		log.Printf("Failed to acquire lock: %v", err)
	} else {
		fmt.Printf("Successfully acquired read lock on %s\n", resource)

		// Simulate some work
		time.Sleep(100 * time.Millisecond)

		// Release the lock
		if err := system.ReleaseLock(txnID, resource); err != nil {
			log.Printf("Failed to release lock: %v", err)
		} else {
			fmt.Printf("Successfully released lock on %s\n", resource)
		}
	}

	// Example 2: Managed goroutine usage
	fmt.Println("\n=== Example 2: Managed Goroutine ===")
	goroutineInfo, err := system.SpawnManagedGoroutine("example-worker", func(ctx context.Context) {
		fmt.Println("Managed goroutine started")

		// Simulate some work with context cancellation support
		select {
		case <-time.After(2 * time.Second):
			fmt.Println("Managed goroutine completed work")
		case <-ctx.Done():
			fmt.Println("Managed goroutine cancelled")
		}
	})

	if err != nil {
		log.Printf("Failed to spawn goroutine: %v", err)
	} else {
		fmt.Printf("Spawned managed goroutine with ID: %d\n", goroutineInfo.ID)
	}

	// Example 3: Worker pool usage
	fmt.Println("\n=== Example 3: Worker Pool ===")
	workItem := WorkItem{
		ID: "work-1",
		Function: func(ctx context.Context) error {
			fmt.Println("Processing work item in worker pool")
			time.Sleep(500 * time.Millisecond)
			fmt.Println("Work item completed")
			return nil
		},
		Priority: 1,
		Timeout:  5 * time.Second,
		Callback: func(err error) {
			if err != nil {
				fmt.Printf("Work item failed: %v\n", err)
			} else {
				fmt.Println("Work item callback: success")
			}
		},
	}

	if err := system.SubmitWork(workItem); err != nil {
		log.Printf("Failed to submit work: %v", err)
	}

	// Example 4: System monitoring
	fmt.Println("\n=== Example 4: System Monitoring ===")

	// Wait a bit for some activity
	time.Sleep(1 * time.Second)

	// Get system statistics
	stats := system.GetSystemStats()
	fmt.Printf("System running: %v\n", stats.Running)
	fmt.Printf("System healthy: %v\n", system.IsHealthy())

	if stats.LockMetrics != nil {
		fmt.Printf("Locks acquired: %d\n", stats.LockMetrics.locksAcquired)
		fmt.Printf("Locks released: %d\n", stats.LockMetrics.locksReleased)
		fmt.Printf("Fast path hits: %d\n", stats.LockMetrics.fastPathHits)
		fmt.Printf("Fast path misses: %d\n", stats.LockMetrics.fastPathMisses)
	}

	if stats.GoroutineStats != nil {
		fmt.Printf("Active goroutines: %d\n", stats.GoroutineStats.ActiveGoroutines)
		fmt.Printf("Total spawned: %d\n", stats.GoroutineStats.TotalSpawned)
	}

	// Example 5: Lock contention simulation
	fmt.Println("\n=== Example 5: Lock Contention Simulation ===")

	// Spawn multiple goroutines that compete for the same resource
	contentionResource := "shared-resource"

	for i := 0; i < 5; i++ {
		txnID := uint64(i + 10)

		system.SpawnManagedGoroutine(fmt.Sprintf("contention-worker-%d", i), func(ctx context.Context) {
			// Try to acquire exclusive lock
			if err := system.AcquireLock(txnID, contentionResource, ExclusiveLock); err != nil {
				fmt.Printf("Worker %d failed to acquire lock: %v\n", i, err)
				return
			}

			fmt.Printf("Worker %d acquired exclusive lock\n", i)

			// Hold lock for a short time
			select {
			case <-time.After(200 * time.Millisecond):
			case <-ctx.Done():
				return
			}

			// Release lock
			if err := system.ReleaseLock(txnID, contentionResource); err != nil {
				fmt.Printf("Worker %d failed to release lock: %v\n", i, err)
			} else {
				fmt.Printf("Worker %d released lock\n", i)
			}
		})
	}

	// Wait for contention simulation to complete
	time.Sleep(3 * time.Second)

	// Show final statistics
	fmt.Println("\n=== Final Statistics ===")
	finalStats := system.GetSystemStats()

	if finalStats.LockMetrics != nil {
		fmt.Printf("Total locks acquired: %d\n", finalStats.LockMetrics.locksAcquired)
		fmt.Printf("Total locks released: %d\n", finalStats.LockMetrics.locksReleased)
		fmt.Printf("Total lock timeouts: %d\n", finalStats.LockMetrics.lockTimeouts)
		fmt.Printf("Total contention events: %d\n", finalStats.LockMetrics.contentionEvents)
	}

	if finalStats.DeadlockMetrics != nil {
		fmt.Printf("Deadlocks detected: %d\n", finalStats.DeadlockMetrics.deadlocksFound)
		fmt.Printf("Deadlocks resolved: %d\n", finalStats.DeadlockMetrics.deadlocksResolved)
	}

	// Get metrics snapshot if available
	if finalStats.MetricsSnapshot != nil {
		fmt.Printf("Metrics timestamp: %v\n", finalStats.MetricsSnapshot.Timestamp)
		if finalStats.MetricsSnapshot.GlobalMetrics != nil {
			fmt.Printf("System uptime: %.2f seconds\n", finalStats.MetricsSnapshot.GlobalMetrics.SystemUptime)
		}
	}

	fmt.Println("\n=== Example completed ===")
}

// BenchmarkLockPerformance demonstrates performance testing of the lock system
func BenchmarkLockPerformance() {
	fmt.Println("=== Lock Performance Benchmark ===")

	config := DefaultConcurrencySystemConfig()
	config.EnableProfiling = true
	config.EnableMetrics = false // Disable metrics for pure performance test

	system := NewEnhancedConcurrencySystem(config)
	if err := system.Start(); err != nil {
		log.Fatalf("Failed to start system: %v", err)
	}
	defer system.Stop()

	// Benchmark parameters
	numOperations := 10000
	numResources := 100

	fmt.Printf("Running %d lock operations across %d resources...\n", numOperations, numResources)

	startTime := time.Now()

	// Perform lock operations
	for i := 0; i < numOperations; i++ {
		txnID := uint64(i)
		resource := fmt.Sprintf("resource-%d", i%numResources)

		// Acquire lock
		if err := system.AcquireLock(txnID, resource, ReadLock); err != nil {
			continue // Skip on error
		}

		// Release lock immediately
		system.ReleaseLock(txnID, resource)
	}

	duration := time.Since(startTime)

	fmt.Printf("Completed %d operations in %v\n", numOperations, duration)
	fmt.Printf("Average operation time: %v\n", duration/time.Duration(numOperations))
	fmt.Printf("Operations per second: %.2f\n", float64(numOperations)/duration.Seconds())

	// Show final metrics
	stats := system.GetSystemStats()
	if stats.LockMetrics != nil {
		fmt.Printf("Fast path hit rate: %.2f%%\n",
			float64(stats.LockMetrics.fastPathHits)/float64(stats.LockMetrics.fastPathHits+stats.LockMetrics.fastPathMisses)*100)
	}
}
