use criterion::{black_box, criterion_group, criterion_main, BenchmarkId, Criterion, Throughput};
use mantisdb::cache::LockFreeCache;
use mantisdb::columnar_engine::ColumnStore;
use mantisdb::document_store::{Document, DocumentStore};
use mantisdb::storage::LockFreeStorage;
use mantisdb::vector_db::{DistanceMetric, Vector, VectorDB};
use std::sync::Arc;
use std::thread;

// KV Operations Benchmarks
fn bench_kv_operations(c: &mut Criterion) {
    let mut group = c.benchmark_group("kv_operations");

    // Single-threaded write
    group.bench_function("write_single", |b| {
        let storage = LockFreeStorage::new(1000000).unwrap();
        let mut counter = 0u64;

        b.iter(|| {
            let key = format!("key_{}", counter);
            let value = format!("value_{}", counter).into_bytes();
            storage.put_string(key, value).unwrap();
            counter += 1;
        });
    });

    // Single-threaded read
    group.bench_function("read_single", |b| {
        let storage = LockFreeStorage::new(1000000).unwrap();

        // Pre-populate
        for i in 0..10000 {
            let key = format!("key_{}", i);
            let value = format!("value_{}", i).into_bytes();
            storage.put_string(key, value).unwrap();
        }

        let mut counter = 0u64;
        b.iter(|| {
            let key = format!("key_{}", counter % 10000);
            black_box(storage.get_string(&key).ok());
            counter += 1;
        });
    });

    // Batch write
    group.throughput(Throughput::Elements(1000));
    group.bench_function("write_batch_1000", |b| {
        let storage = LockFreeStorage::new(1000000).unwrap();
        let mut counter = 0u64;

        b.iter(|| {
            let batch: Vec<_> = (0..1000)
                .map(|i| {
                    let key = format!("batch_key_{}_{}", counter, i);
                    let value = format!("batch_value_{}", i).into_bytes();
                    (key, value)
                })
                .collect();

            storage.batch_put(batch).unwrap();
            counter += 1;
        });
    });

    group.finish();
}

// Cache Operations Benchmarks
fn bench_cache_operations(c: &mut Criterion) {
    let mut group = c.benchmark_group("cache_operations");

    group.bench_function("cache_write", |b| {
        let cache = LockFreeCache::new(100000);
        let mut counter = 0u64;

        b.iter(|| {
            let key = format!("cache_key_{}", counter);
            let value = format!("cache_value_{}", counter).into_bytes();
            cache.put(key.as_bytes(), &value, 3600);
            counter += 1;
        });
    });

    group.bench_function("cache_read_hit", |b| {
        let cache = LockFreeCache::new(100000);

        // Pre-populate
        for i in 0..10000 {
            let key = format!("cache_key_{}", i);
            let value = format!("cache_value_{}", i).into_bytes();
            cache.put(key.as_bytes(), &value, 3600);
        }

        let mut counter = 0u64;
        b.iter(|| {
            let key = format!("cache_key_{}", counter % 10000);
            black_box(cache.get(key.as_bytes()));
            counter += 1;
        });
    });

    group.finish();
}

// Vector Database Benchmarks
fn bench_vector_operations(c: &mut Criterion) {
    let mut group = c.benchmark_group("vector_operations");

    // Insert vectors
    group.bench_function("vector_insert_128d", |b| {
        let db = VectorDB::new(128, DistanceMetric::Cosine);
        let mut counter = 0u64;

        b.iter(|| {
            let embedding: Vec<f32> = (0..128)
                .map(|i| (i as f32 + counter as f32) / 128.0)
                .collect();
            let vector = Vector::new(format!("vec_{}", counter), embedding);
            db.insert(vector).unwrap();
            counter += 1;
        });
    });

    // Search vectors
    group.bench_function("vector_search_cosine_k10", |b| {
        let db = VectorDB::new(128, DistanceMetric::Cosine);

        // Pre-populate with 10000 vectors
        for i in 0..10000 {
            let embedding: Vec<f32> = (0..128).map(|j| (i + j) as f32 / 128.0).collect();
            let vector = Vector::new(format!("vec_{}", i), embedding);
            db.insert(vector).unwrap();
        }

        let query: Vec<f32> = (0..128).map(|i| i as f32 / 128.0).collect();

        b.iter(|| {
            black_box(db.search(&query, 10).unwrap());
        });
    });

    group.finish();
}

// Document Store Benchmarks
fn bench_document_operations(c: &mut Criterion) {
    let mut group = c.benchmark_group("document_operations");

    group.bench_function("document_insert", |b| {
        let store = DocumentStore::new("test_collection");
        let mut counter = 0u64;

        b.iter(|| {
            let doc = Document::new(serde_json::json!({
                "id": counter,
                "name": format!("Document {}", counter),
                "value": counter * 10,
                "active": true
            }));

            store.insert_document(doc).unwrap();
            counter += 1;
        });
    });

    group.bench_function("document_query", |b| {
        let store = DocumentStore::new("test_collection");

        // Pre-populate
        for i in 0..10000 {
            let doc = Document::new(serde_json::json!({
                "id": i,
                "name": format!("Document {}", i),
                "value": i * 10,
                "active": i % 2 == 0
            }));
            store.insert_document(doc).unwrap();
        }

        b.iter(|| {
            black_box(store.find_documents(|doc| doc.data["active"].as_bool().unwrap_or(false)));
        });
    });

    group.finish();
}

// Columnar Store Benchmarks
fn bench_columnar_operations(c: &mut Criterion) {
    let mut group = c.benchmark_group("columnar_operations");

    group.throughput(Throughput::Elements(1000));
    group.bench_function("columnar_append_1000", |b| {
        let store = ColumnStore::new();

        b.iter(|| {
            for i in 0..1000 {
                store.append("col1", i as i64).unwrap();
                store.append("col2", format!("value_{}", i)).unwrap();
                store.append("col3", i as f64 * 1.5).unwrap();
            }
        });
    });

    group.bench_function("columnar_scan", |b| {
        let store = ColumnStore::new();

        // Pre-populate
        for i in 0..10000 {
            store.append("col1", i as i64).unwrap();
            store.append("col2", format!("value_{}", i)).unwrap();
        }

        b.iter(|| {
            black_box(store.get_column("col1"));
        });
    });

    group.finish();
}

// Concurrent Operations Benchmarks
fn bench_concurrent_operations(c: &mut Criterion) {
    let mut group = c.benchmark_group("concurrent_operations");

    group.bench_function("concurrent_writes_4_threads", |b| {
        let storage = Arc::new(LockFreeStorage::new(1000000).unwrap());

        b.iter(|| {
            let mut handles = vec![];

            for thread_id in 0..4 {
                let storage_clone = Arc::clone(&storage);

                let handle = thread::spawn(move || {
                    for i in 0..250 {
                        let key = format!("thread_{}_{}", thread_id, i);
                        let value = format!("value_{}", i).into_bytes();
                        storage_clone.put_string(key, value).unwrap();
                    }
                });

                handles.push(handle);
            }

            for handle in handles {
                handle.join().unwrap();
            }
        });
    });

    group.bench_function("concurrent_reads_4_threads", |b| {
        let storage = Arc::new(LockFreeStorage::new(1000000).unwrap());

        // Pre-populate
        for i in 0..10000 {
            let key = format!("key_{}", i);
            let value = format!("value_{}", i).into_bytes();
            storage.put_string(key, value).unwrap();
        }

        b.iter(|| {
            let mut handles = vec![];

            for thread_id in 0..4 {
                let storage_clone = Arc::clone(&storage);

                let handle = thread::spawn(move || {
                    for i in 0..250 {
                        let key = format!("key_{}", (thread_id * 250 + i) % 10000);
                        black_box(storage_clone.get_string(&key).ok());
                    }
                });

                handles.push(handle);
            }

            for handle in handles {
                handle.join().unwrap();
            }
        });
    });

    group.finish();
}

// Mixed Workload Benchmarks
fn bench_mixed_workload(c: &mut Criterion) {
    let mut group = c.benchmark_group("mixed_workload");

    group.bench_function("read_write_50_50", |b| {
        let storage = LockFreeStorage::new(1000000).unwrap();

        // Pre-populate
        for i in 0..10000 {
            let key = format!("key_{}", i);
            let value = format!("value_{}", i).into_bytes();
            storage.put_string(key, value).unwrap();
        }

        let mut counter = 0u64;
        b.iter(|| {
            if counter % 2 == 0 {
                // Write
                let key = format!("key_{}", counter);
                let value = format!("value_{}", counter).into_bytes();
                storage.put_string(key, value).unwrap();
            } else {
                // Read
                let key = format!("key_{}", counter % 10000);
                black_box(storage.get_string(&key).ok());
            }
            counter += 1;
        });
    });

    group.finish();
}

criterion_group!(
    benches,
    bench_kv_operations,
    bench_cache_operations,
    bench_vector_operations,
    bench_document_operations,
    bench_columnar_operations,
    bench_concurrent_operations,
    bench_mixed_workload
);

criterion_main!(benches);
