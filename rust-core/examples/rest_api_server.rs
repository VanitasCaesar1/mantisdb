//! Example REST API server with connection pooling
//!
//! This demonstrates how to use the high-performance REST API
//! with connection pooling for MantisDB.

use mantisdb_core::{
    ConnectionPool, PoolConfig, RestApiServer, RestApiConfig,
};
use mantisdb_core::storage::LockFreeStorage;
use std::sync::Arc;
use std::time::Duration;
use tracing_subscriber;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Initialize tracing
    tracing_subscriber::fmt::init();

    println!("🚀 Starting MantisDB REST API Server with Connection Pooling");
    println!("{}", "=".repeat(60));

    // Configure connection pool (like PgBouncer)
    let pool_config = PoolConfig {
        min_connections: 10,
        max_connections: 1000,
        max_idle_time: Duration::from_secs(300),
        connection_timeout: Duration::from_secs(10),
        max_lifetime: Duration::from_secs(3600),
        health_check_interval: Duration::from_secs(30),
        recycle_connections: true,
    };

    println!("\n📊 Connection Pool Configuration:");
    println!("  • Min Connections: {}", pool_config.min_connections);
    println!("  • Max Connections: {}", pool_config.max_connections);
    println!("  • Max Idle Time: {:?}", pool_config.max_idle_time);
    println!("  • Connection Timeout: {:?}", pool_config.connection_timeout);
    println!("  • Max Lifetime: {:?}", pool_config.max_lifetime);

    // Create connection pool
    let pool = ConnectionPool::new(pool_config, || {
        Ok(Arc::new(LockFreeStorage::new(1024 * 1024 * 100)?)) // 100MB per connection
    })
    .await?;

    let pool = Arc::new(pool);
    println!("\n✅ Connection pool initialized successfully");

    // Configure REST API (like PostgREST)
    let api_config = RestApiConfig {
        bind_addr: "0.0.0.0:8080".parse()?,
        enable_cors: true,
        enable_compression: true,
        enable_tracing: true,
        max_body_size: 10 * 1024 * 1024, // 10MB
        request_timeout: 30,
    };

    println!("\n🌐 REST API Configuration:");
    println!("  • Bind Address: {}", api_config.bind_addr);
    println!("  • CORS Enabled: {}", api_config.enable_cors);
    println!("  • Compression: {}", api_config.enable_compression);
    println!("  • Max Body Size: {} MB", api_config.max_body_size / 1024 / 1024);

    // Create and start REST API server
    let server = RestApiServer::new(api_config, pool.clone());

    println!("\n🎯 Available Endpoints:");
    println!("  Health & Stats:");
    println!("    GET  /health              - Health check");
    println!("    GET  /stats               - Pool statistics");
    println!("\n  Key-Value API:");
    println!("    GET  /api/v1/kv/:key      - Get value");
    println!("    PUT  /api/v1/kv/:key      - Set value");
    println!("    DELETE /api/v1/kv/:key    - Delete value");
    println!("    POST /api/v1/kv           - Batch operations");
    println!("    GET  /api/v1/kv           - List keys");
    println!("\n  Table API (PostgREST-like):");
    println!("    GET  /api/v1/tables/:table       - Query table");
    println!("    POST /api/v1/tables/:table       - Insert row");
    println!("    GET  /api/v1/tables/:table/:id   - Get row");
    println!("    PUT  /api/v1/tables/:table/:id   - Update row");
    println!("    DELETE /api/v1/tables/:table/:id - Delete row");

    println!();
    println!("{}", "=".repeat(60));
    println!("🎉 Server is running on http://0.0.0.0:8080");
    println!("{}", "=".repeat(60));
    println!("\n💡 Example requests:");
    println!("  curl http://localhost:8080/health");
    println!("  curl http://localhost:8080/stats");
    println!("  curl -X PUT http://localhost:8080/api/v1/kv/mykey \\");
    println!("       -H 'Content-Type: application/json' \\");
    println!("       -d '{{\"value\": [72,101,108,108,111]}}'");
    println!("  curl http://localhost:8080/api/v1/kv/mykey");
    println!();

    // Start the server (this will block)
    server.start().await?;

    Ok(())
}
