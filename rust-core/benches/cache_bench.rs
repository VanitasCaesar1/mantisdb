use criterion::{black_box, criterion_group, criterion_main, BenchmarkId, Criterion, Throughput};
use mantisdb_core::cache::LockFreeCache;
use std::sync::Arc;
use std::thread;

fn bench_cache_puts(c: &mut Criterion) {
    let mut group = c.benchmark_group("cache_puts");

    for size in [100, 1000, 10000].iter() {
        group.throughput(Throughput::Elements(*size as u64));
        group.bench_with_input(BenchmarkId::from_parameter(size), size, |b, &size| {
            let cache = LockFreeCache::new(10 * 1024 * 1024); // 10MB
            b.iter(|| {
                for i in 0..size {
                    let key = format!("key_{}", i);
                    let value = format!("value_{}", i).into_bytes();
                    cache.put(key, value, 0).unwrap();
                }
            });
        });
    }
    group.finish();
}

fn bench_cache_gets(c: &mut Criterion) {
    let mut group = c.benchmark_group("cache_gets");

    for size in [100, 1000, 10000].iter() {
        let cache = LockFreeCache::new(10 * 1024 * 1024);

        // Populate cache
        for i in 0..*size {
            let key = format!("key_{}", i);
            let value = format!("value_{}", i).into_bytes();
            cache.put(key, value, 0).unwrap();
        }

        group.throughput(Throughput::Elements(*size as u64));
        group.bench_with_input(BenchmarkId::from_parameter(size), size, |b, &size| {
            b.iter(|| {
                for i in 0..size {
                    let key = format!("key_{}", i);
                    black_box(cache.get(&key).unwrap());
                }
            });
        });
    }
    group.finish();
}

fn bench_cache_concurrent(c: &mut Criterion) {
    let mut group = c.benchmark_group("cache_concurrent");

    for threads in [2, 4, 8, 16].iter() {
        group.throughput(Throughput::Elements(1000 * *threads as u64));
        group.bench_with_input(
            BenchmarkId::from_parameter(threads),
            threads,
            |b, &threads| {
                let cache = Arc::new(LockFreeCache::new(50 * 1024 * 1024)); // 50MB

                b.iter(|| {
                    let mut handles = vec![];

                    for t in 0..threads {
                        let cache = Arc::clone(&cache);
                        let handle = thread::spawn(move || {
                            for i in 0..1000 {
                                let key = format!("key_{}_{}", t, i);
                                let value = format!("value_{}_{}", t, i).into_bytes();
                                cache.put(key.clone(), value, 0).unwrap();
                                black_box(cache.get(&key).unwrap());
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

fn bench_cache_eviction(c: &mut Criterion) {
    let mut group = c.benchmark_group("cache_eviction");

    group.bench_function("lru_eviction", |b| {
        let cache = LockFreeCache::new(1024 * 100); // Small cache to trigger eviction

        b.iter(|| {
            for i in 0..1000 {
                let key = format!("key_{}", i);
                let value = vec![0u8; 100]; // 100 bytes each
                cache.put(key, value, 0).unwrap();
            }
        });
    });

    group.finish();
}

criterion_group!(
    benches,
    bench_cache_puts,
    bench_cache_gets,
    bench_cache_concurrent,
    bench_cache_eviction
);
criterion_main!(benches);
