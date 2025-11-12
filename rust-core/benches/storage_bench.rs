use criterion::{black_box, criterion_group, criterion_main, BenchmarkId, Criterion, Throughput};
use mantisdb_core::storage::LockFreeStorage;
use std::sync::Arc;
use std::thread;

fn bench_sequential_writes(c: &mut Criterion) {
    let mut group = c.benchmark_group("storage_sequential_writes");

    for size in [100, 1000, 10000].iter() {
        group.throughput(Throughput::Elements(*size as u64));
        group.bench_with_input(BenchmarkId::from_parameter(size), size, |b, &size| {
            let storage = LockFreeStorage::new(1024 * 1024).expect("Failed to create storage");
            b.iter(|| {
                for i in 0..size {
                    let key = format!("key_{}", i);
                    let value = format!("value_{}", i).into_bytes();
                    storage.put(key.as_bytes(), &value).unwrap();
                }
            });
        });
    }
    group.finish();
}

fn bench_sequential_reads(c: &mut Criterion) {
    let mut group = c.benchmark_group("storage_sequential_reads");

    for size in [100, 1000, 10000].iter() {
        let storage = LockFreeStorage::new(1024 * 1024).expect("Failed to create storage");

        // Populate data
        for i in 0..*size {
            let key = format!("key_{}", i);
            let value = format!("value_{}", i).into_bytes();
            storage.put(key.as_bytes(), &value).unwrap();
        }

        group.throughput(Throughput::Elements(*size as u64));
        group.bench_with_input(BenchmarkId::from_parameter(size), size, |b, &size| {
            b.iter(|| {
                for i in 0..size {
                    let key = format!("key_{}", i);
                    black_box(storage.get(key.as_bytes()).unwrap());
                }
            });
        });
    }
    group.finish();
}

fn bench_concurrent_access(c: &mut Criterion) {
    let mut group = c.benchmark_group("storage_concurrent");

    for threads in [2, 4, 8, 16].iter() {
        group.throughput(Throughput::Elements(1000 * *threads as u64));
        group.bench_with_input(
            BenchmarkId::from_parameter(threads),
            threads,
            |b, &threads| {
                let storage =
                    Arc::new(LockFreeStorage::new(1024 * 1024).expect("Failed to create storage"));

                b.iter(|| {
                    let mut handles = vec![];

                    for t in 0..threads {
                        let storage = Arc::clone(&storage);
                        let handle = thread::spawn(move || {
                            for i in 0..1000 {
                                let key = format!("key_{}_{}", t, i);
                                let value = format!("value_{}_{}", t, i).into_bytes();
                                storage.put(key.as_bytes(), &value).unwrap();
                                black_box(storage.get(key.as_bytes()).unwrap());
                            }
                        });
                        handles.push(handle);
                    }

                    for handle in handles {
                        handle.join().unwrap();
                    }
                });
            },
        );
    }
    group.finish();
}

fn bench_mixed_workload(c: &mut Criterion) {
    let mut group = c.benchmark_group("storage_mixed_workload");

    let storage = Arc::new(LockFreeStorage::new(1024 * 1024).expect("Failed to create storage"));

    // Pre-populate
    for i in 0..10000 {
        let key = format!("key_{}", i);
        let value = format!("value_{}", i).into_bytes();
        storage.put(key.as_bytes(), &value).unwrap();
    }

    group.bench_function("80_read_20_write", |b| {
        b.iter(|| {
            for i in 0..1000 {
                if i % 5 == 0 {
                    // 20% writes
                    let key = format!("key_{}", i);
                    let value = format!("value_new_{}", i).into_bytes();
                    storage.put(key.as_bytes(), &value).unwrap();
                } else {
                    // 80% reads
                    let key = format!("key_{}", i);
                    black_box(storage.get(key.as_bytes()).unwrap());
                }
            }
        });
    });

    group.finish();
}

criterion_group!(
    benches,
    bench_sequential_writes,
    bench_sequential_reads,
    bench_concurrent_access,
    bench_mixed_workload
);
criterion_main!(benches);
