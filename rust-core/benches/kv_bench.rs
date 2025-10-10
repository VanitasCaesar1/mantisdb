use criterion::{black_box, criterion_group, criterion_main, Criterion, BatchSize};
use mantisdb_core::storage::LockFreeStorage;
use rand::{Rng, SeedableRng};
use rand::rngs::StdRng;

fn bench_kv_put(c: &mut Criterion) {
    let mut group = c.benchmark_group("kv_put");
    group.sample_size(50);

    group.bench_function("put_1k_values", |b| {
        b.iter_batched(
            || LockFreeStorage::default(),
            |storage| {
                let mut rng = StdRng::seed_from_u64(42);
                for i in 0..1000u32 {
                    let key = format!("k:{}:{}", i, rng.gen::<u64>());
                    let val: Vec<u8> = (0..512).map(|_| rng.gen::<u8>()).collect();
                    storage.put(key.as_bytes(), &val).unwrap();
                    black_box(());
                }
            },
            BatchSize::SmallInput,
        )
    });

    group.finish();
}

fn bench_kv_get(c: &mut Criterion) {
    let mut group = c.benchmark_group("kv_get");
    group.sample_size(50);

    group.bench_function("get_1k_values", |b| {
        b.iter_batched(
            || {
                let storage = LockFreeStorage::default();
                let mut rng = StdRng::seed_from_u64(1337);
                let mut keys = Vec::with_capacity(1000);
                for i in 0..1000u32 {
                    let key = format!("k:{}:{}", i, rng.gen::<u64>());
                    let val: Vec<u8> = (0..512).map(|_| rng.gen::<u8>()).collect();
                    storage.put(key.as_bytes(), &val).unwrap();
                    keys.push(key);
                }
                (storage, keys)
            },
            |(storage, keys)| {
                for key in keys.iter() {
                    let _ = black_box(storage.get(key.as_bytes()).unwrap());
                }
            },
            BatchSize::SmallInput,
        )
    });

    group.finish();
}

fn bench_kv_batch_put(c: &mut Criterion) {
    let mut group = c.benchmark_group("kv_batch_put");
    group.sample_size(30);

    group.bench_function("batch_put_10k", |b| {
        b.iter_batched(
            || {
                let mut rng = StdRng::seed_from_u64(2024);
                let entries: Vec<(String, Vec<u8>)> = (0..10_000)
                    .map(|i| {
                        let key = format!("bk:{}:{}", i, rng.gen::<u64>());
                        let val: Vec<u8> = (0..256).map(|_| rng.gen::<u8>()).collect();
                        (key, val)
                    })
                    .collect();
                (LockFreeStorage::default(), entries)
            },
            |(storage, entries)| {
                storage.batch_put(entries).unwrap();
            },
            BatchSize::LargeInput,
        )
    });

    group.finish();
}

criterion_group!(benches, bench_kv_put, bench_kv_get, bench_kv_batch_put);
criterion_main!(benches);
