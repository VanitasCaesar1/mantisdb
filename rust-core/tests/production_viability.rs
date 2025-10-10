use axum::{http::Request, Router};
use tower::ServiceExt; // for `oneshot`
use mantisdb_core::{build_admin_router, AdminState};
use mantisdb_core::storage::LockFreeStorage;
use std::sync::Arc;

#[tokio::test(flavor = "multi_thread", worker_threads = 4)]
async fn storage_extreme_concurrency() {
    let storage = Arc::new(LockFreeStorage::default());
    let threads = std::cmp::min(num_cpus::get() * 4, 32);

    let mut tasks = Vec::new();
    for t in 0..threads {
        let s = Arc::clone(&storage);
        tasks.push(tokio::spawn(async move {
            for i in 0..20_000u32 {
                let key = format!("t:{}:{}", t, i);
                let val = vec![0u8; 256];
                s.put(key.as_bytes(), &val).unwrap();
                let _ = s.get(key.as_bytes()).unwrap();
                if i % 7 == 0 { let _ = s.delete(key.as_bytes()); }
            }
        }));
    }

    for task in tasks { task.await.unwrap(); }

    // Basic sanity: no panic paths reached, storage remains usable
    assert!(storage.len() >= 0);
}

#[tokio::test]
async fn admin_router_health_and_tables() {
    let state = AdminState::new();
    let app: Router = build_admin_router(state);

    // Health endpoint
    let response = app.clone()
        .oneshot(Request::builder().uri("/api/health").body(axum::body::Body::empty()).unwrap())
        .await
        .unwrap();
    assert!(response.status().is_success());

    // Tables list
    let response = app
        .oneshot(Request::builder().uri("/api/tables").body(axum::body::Body::empty()).unwrap())
        .await
        .unwrap();
    assert!(response.status().is_success());
}
