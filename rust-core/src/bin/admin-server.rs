//! MantisDB Admin Server
//!
//! High-performance admin API server built with Rust and Axum

use mantisdb_core::admin_api::{AdminState, build_admin_router};
use std::net::SocketAddr;
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Initialize tracing
    tracing_subscriber::registry()
        .with(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| "admin_server=debug,tower_http=debug,axum=trace".into()),
        )
        .with(tracing_subscriber::fmt::layer())
        .init();

    // Create admin state
    let state = AdminState::new();

    // Build router
    let app = build_admin_router(state);

    // Start server
    let addr = SocketAddr::from(([0, 0, 0, 0], 8081));
    tracing::info!("🚀 MantisDB Admin Server listening on {}", addr);
    tracing::info!("📊 Dashboard: http://localhost:8081");
    tracing::info!("🔒 RLS Engine: Enabled");
    tracing::info!("⚡ High-performance Rust backend ready!");

    let listener = tokio::net::TcpListener::bind(addr).await?;
    axum::serve(listener, app).await?;

    Ok(())
}
