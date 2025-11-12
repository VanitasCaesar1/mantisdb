use criterion::{criterion_group, criterion_main, Criterion};
use mantisdb_core::storage::LockFreeStorage;
use rand::rngs::StdRng;
use rand::{Rng, SeedableRng};
use std::sync::Arc;
use std::thread;

fn mixed_extreme_workload(c: &mut Criterion) {
    let mut group = c.benchmark_group("mixed_extreme_workload");
    group.sample_size(10);

    group.bench_function("concurrent_mixed_ops", |b| {
        b.iter(|| {
            let storage = Arc::new(LockFreeStorage::default());
            let threads = std::cmp::min(num_cpus::get() * 4, 32);
            let mut handles = Vec::with_capacity(threads);

            for t in 0..threads {
                let s = Arc::clone(&storage);
                let handle = thread::spawn(move || {
                    let mut rng = StdRng::seed_from_u64(0xDEADBEEF ^ (t as u64));
                    for i in 0..10_000u32 {
                        let op = rng.gen::<u8>() % 4;
                        match op {
                            0 | 1 => {
                                // put
                                let key = format!("m:{}:{}", t, i);
                                let val: Vec<u8> = (0..256).map(|_| rng.gen::<u8>()).collect();
                                let _ = s.put(key.as_bytes(), &val);
                            }
                            2 => {
                                // get
                                let key = format!("m:{}:{}", t, rng.gen::<u32>() % 10_000);
                                let _ = s.get(key.as_bytes());
                            }
                            _ => {
                                // delete
                                let key = format!("m:{}:{}", t, rng.gen::<u32>() % 10_000);
                                let _ = s.delete(key.as_bytes());
                            }
                        }
                    }
                });
                handles.push(handle);
            }

            for h in handles {
                let _ = h.join();
            }
        })
    });

    group.finish();
}

criterion_group!(benches, mixed_extreme_workload);
criterion_main!(benches);
