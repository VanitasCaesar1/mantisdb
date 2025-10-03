package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"mantisDB/cache"
	"mantisDB/storage"
	"mantisDB/store"
)

func main() {
	fmt.Println("MantisDB Test Suite")
	fmt.Println("==================")

	// Test 1: Direct store operations
	fmt.Println("\n1. Testing Direct Store Operations")
	testDirectStore()

	// Test 2: API endpoints (requires server to be running)
	fmt.Println("\n2. Testing API Endpoints")
	testAPIEndpoints()

	fmt.Println("\nTest suite complete!")
}

func testDirectStore() {
	// Initialize storage and cache
	storageEngine := storage.NewPureGoStorageEngine(storage.StorageConfig{
		DataDir:    "./test_data",
		BufferSize: 1024 * 1024,
		CacheSize:  10 * 1024 * 1024,
		UseCGO:     false,
		SyncWrites: true,
	})

	cacheConfig := cache.CacheConfig{
		MaxSize:         10 * 1024 * 1024,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute * 5,
		EvictionPolicy:  "lru",
	}
	cacheManager := cache.NewCacheManager(cacheConfig)

	// Initialize store
	mantisStore := store.NewMantisStore(storageEngine, cacheManager)

	ctx := context.Background()

	// Test Key-Value operations
	fmt.Println("  Testing KV operations...")

	// Set a key
	err := mantisStore.KV().Set(ctx, "test_key", []byte("test_value"), time.Minute)
	if err != nil {
		fmt.Printf("    ❌ KV Set failed: %v\n", err)
	} else {
		fmt.Println("    ✅ KV Set successful")
	}

	// Get the key
	value, err := mantisStore.KV().Get(ctx, "test_key")
	if err != nil {
		fmt.Printf("    ❌ KV Get failed: %v\n", err)
	} else if string(value) == "test_value" {
		fmt.Println("    ✅ KV Get successful")
	} else {
		fmt.Printf("    ❌ KV Get returned wrong value: %s\n", string(value))
	}

	// Test cache hit (second get should be faster)
	start := time.Now()
	_, err = mantisStore.KV().Get(ctx, "test_key")
	duration := time.Since(start)
	if err != nil {
		fmt.Printf("    ❌ KV Cache test failed: %v\n", err)
	} else {
		fmt.Printf("    ✅ KV Cache test successful (latency: %v)\n", duration)
	}

	fmt.Println("  Direct store tests completed!")
}

func testAPIEndpoints() {
	baseURL := "http://localhost:8080/api/v1"

	// Test health endpoint
	fmt.Println("  Testing health endpoint...")
	resp, err := http.Get("http://localhost:8080/health")
	if err != nil {
		fmt.Printf("    ❌ Health check failed: %v\n", err)
		fmt.Println("    (Make sure MantisDB server is running with: go run cmd/mantisDB/main.go)")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		fmt.Println("    ✅ Health check successful")
	} else {
		fmt.Printf("    ❌ Health check failed with status: %d\n", resp.StatusCode)
		return
	}

	// Test KV API
	fmt.Println("  Testing KV API...")

	// Set a key via API
	kvData := map[string]interface{}{
		"value": "api_test_value",
		"ttl":   3600,
	}
	jsonData, _ := json.Marshal(kvData)

	resp, err = http.Post(baseURL+"/kv/api_test_key", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("    ❌ KV API Set failed: %v\n", err)
	} else {
		defer resp.Body.Close()
		if resp.StatusCode == 200 {
			fmt.Println("    ✅ KV API Set successful")
		} else {
			fmt.Printf("    ❌ KV API Set failed with status: %d\n", resp.StatusCode)
		}
	}

	// Get the key via API
	resp, err = http.Get(baseURL + "/kv/api_test_key")
	if err != nil {
		fmt.Printf("    ❌ KV API Get failed: %v\n", err)
	} else {
		defer resp.Body.Close()
		if resp.StatusCode == 200 {
			body, _ := io.ReadAll(resp.Body)
			var result map[string]interface{}
			json.Unmarshal(body, &result)
			if result["value"] == "api_test_value" {
				fmt.Println("    ✅ KV API Get successful")
			} else {
				fmt.Printf("    ❌ KV API Get returned wrong value: %v\n", result["value"])
			}
		} else {
			fmt.Printf("    ❌ KV API Get failed with status: %d\n", resp.StatusCode)
		}
	}

	// Test Document API
	fmt.Println("  Testing Document API...")

	// Create a document via API
	docData := map[string]interface{}{
		"id": "test_doc",
		"data": map[string]interface{}{
			"name":  "Test Document",
			"value": 42,
		},
	}
	jsonData, _ = json.Marshal(docData)

	resp, err = http.Post(baseURL+"/docs/test_collection", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("    ❌ Document API Create failed: %v\n", err)
	} else {
		defer resp.Body.Close()
		if resp.StatusCode == 200 {
			fmt.Println("    ✅ Document API Create successful")
		} else {
			body, _ := io.ReadAll(resp.Body)
			fmt.Printf("    ❌ Document API Create failed with status: %d, body: %s\n", resp.StatusCode, string(body))
		}
	}

	// Get the document via API
	resp, err = http.Get(baseURL + "/docs/test_collection/test_doc")
	if err != nil {
		fmt.Printf("    ❌ Document API Get failed: %v\n", err)
	} else {
		defer resp.Body.Close()
		if resp.StatusCode == 200 {
			fmt.Println("    ✅ Document API Get successful")
		} else {
			fmt.Printf("    ❌ Document API Get failed with status: %d\n", resp.StatusCode)
		}
	}

	// Test Stats API
	fmt.Println("  Testing Stats API...")
	resp, err = http.Get(baseURL + "/stats")
	if err != nil {
		fmt.Printf("    ❌ Stats API failed: %v\n", err)
	} else {
		defer resp.Body.Close()
		if resp.StatusCode == 200 {
			body, _ := io.ReadAll(resp.Body)
			var stats map[string]interface{}
			json.Unmarshal(body, &stats)
			fmt.Printf("    ✅ Stats API successful - Storage: %v, Cache entries: %.0f\n",
				stats["storage_engine"], stats["cache"].(map[string]interface{})["total_entries"])
		} else {
			fmt.Printf("    ❌ Stats API failed with status: %d\n", resp.StatusCode)
		}
	}

	fmt.Println("  API tests completed!")
}
