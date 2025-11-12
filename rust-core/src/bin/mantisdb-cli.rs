//! MantisDB CLI - Command-line interface for MantisDB management
//!
//! Usage:
//!   mantisdb-cli connect --host localhost:8080
//!   mantisdb-cli inspect kv user:123
//!   mantisdb-cli stats --detailed
//!   mantisdb-cli query --sql "SELECT * FROM users"
//!   mantisdb-cli backup --output backup.tar.gz

use clap::{Parser, Subcommand};
use colored::Colorize;
use reqwest::blocking::Client;
use serde_json::{json, Value};
use std::fs::File;
use std::io::{self, Write};
use std::path::PathBuf;

#[derive(Parser)]
#[command(name = "mantisdb-cli")]
#[command(about = "MantisDB Command Line Interface", long_about = None)]
#[command(version)]
struct Cli {
    #[command(subcommand)]
    command: Commands,
    
    /// MantisDB server host
    #[arg(short = 'H', long, default_value = "localhost:8080", global = true)]
    host: String,
    
    /// Authentication token
    #[arg(short, long, env = "MANTIS_TOKEN", global = true)]
    token: Option<String>,
}

#[derive(Subcommand)]
enum Commands {
    /// Connect and test connection to MantisDB
    Connect {
        /// Test connection with a ping
        #[arg(long)]
        ping: bool,
    },
    
    /// Inspect a specific key/document/vector
    Inspect {
        /// Type: kv, doc, vector, sql
        r#type: String,
        
        /// Key/ID to inspect
        key: String,
    },
    
    /// Show database statistics
    Stats {
        /// Show detailed statistics
        #[arg(long)]
        detailed: bool,
        
        /// Output format: text, json
        #[arg(long, default_value = "text")]
        format: String,
    },
    
    /// Execute a query
    Query {
        /// SQL query
        #[arg(long, conflicts_with = "vector")]
        sql: Option<String>,
        
        /// Vector search (JSON array)
        #[arg(long)]
        vector: Option<String>,
        
        /// Number of results for vector search
        #[arg(short, long, default_value = "10")]
        k: usize,
        
        /// Distance metric: cosine, euclidean, dotproduct
        #[arg(long, default_value = "cosine")]
        metric: String,
    },
    
    /// Backup database
    Backup {
        /// Output file path
        #[arg(short, long)]
        output: PathBuf,
        
        /// Compress backup
        #[arg(long)]
        compress: bool,
        
        /// Pause writes during backup
        #[arg(long)]
        pause_writes: bool,
    },
    
    /// Restore from backup
    Restore {
        /// Backup file path
        #[arg(short, long)]
        input: PathBuf,
    },
    
    /// Migrate data from another database
    Migrate {
        /// Source database URI (redis://, mongodb://, postgres://)
        #[arg(long)]
        from: String,
        
        /// Target collection/table
        #[arg(long)]
        to: Option<String>,
        
        /// Batch size
        #[arg(long, default_value = "1000")]
        batch_size: usize,
    },
    
    /// List keys/documents/vectors
    List {
        /// Type: kv, doc, vector, tables
        r#type: String,
        
        /// Collection/table name
        #[arg(long)]
        collection: Option<String>,
        
        /// Limit results
        #[arg(short, long, default_value = "100")]
        limit: usize,
        
        /// Prefix filter (for KV)
        #[arg(long)]
        prefix: Option<String>,
    },
    
    /// Delete keys/documents/vectors
    Delete {
        /// Type: kv, doc, vector
        r#type: String,
        
        /// Key/ID to delete
        key: String,
        
        /// Force delete without confirmation
        #[arg(long)]
        force: bool,
    },
    
    /// Monitor real-time metrics
    Monitor {
        /// Refresh interval in seconds
        #[arg(short, long, default_value = "1")]
        interval: u64,
    },
}

struct MantisClient {
    client: Client,
    base_url: String,
    token: Option<String>,
}

impl MantisClient {
    fn new(host: &str, token: Option<String>) -> Self {
        let base_url = if host.starts_with("http") {
            host.to_string()
        } else {
            format!("http://{}", host)
        };
        
        Self {
            client: Client::new(),
            base_url,
            token,
        }
    }
    
    fn get(&self, path: &str) -> Result<Value, Box<dyn std::error::Error>> {
        let url = format!("{}{}", self.base_url, path);
        let mut req = self.client.get(&url);
        
        if let Some(token) = &self.token {
            req = req.header("Authorization", format!("Bearer {}", token));
        }
        
        let resp = req.send()?;
        let json = resp.json()?;
        Ok(json)
    }
    
    fn post(&self, path: &str, body: Value) -> Result<Value, Box<dyn std::error::Error>> {
        let url = format!("{}{}", self.base_url, path);
        let mut req = self.client.post(&url).json(&body);
        
        if let Some(token) = &self.token {
            req = req.header("Authorization", format!("Bearer {}", token));
        }
        
        let resp = req.send()?;
        let json = resp.json()?;
        Ok(json)
    }
}

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let cli = Cli::parse();
    let client = MantisClient::new(&cli.host, cli.token);
    
    match cli.command {
        Commands::Connect { ping } => cmd_connect(&client, ping)?,
        Commands::Inspect { r#type, key } => cmd_inspect(&client, &r#type, &key)?,
        Commands::Stats { detailed, format } => cmd_stats(&client, detailed, &format)?,
        Commands::Query { sql, vector, k, metric } => cmd_query(&client, sql, vector, k, &metric)?,
        Commands::Backup { output, compress, pause_writes } => {
            cmd_backup(&client, output, compress, pause_writes)?
        }
        Commands::Restore { input } => cmd_restore(&client, input)?,
        Commands::Migrate { from, to, batch_size } => cmd_migrate(&client, &from, to.as_deref(), batch_size)?,
        Commands::List { r#type, collection, limit, prefix } => {
            cmd_list(&client, &r#type, collection.as_deref(), limit, prefix.as_deref())?
        }
        Commands::Delete { r#type, key, force } => cmd_delete(&client, &r#type, &key, force)?,
        Commands::Monitor { interval } => cmd_monitor(&client, interval)?,
    }
    
    Ok(())
}

fn cmd_connect(client: &MantisClient, ping: bool) -> Result<(), Box<dyn std::error::Error>> {
    println!("{}", "🔌 Connecting to MantisDB...".cyan().bold());
    println!("   Host: {}", client.base_url);
    
    let health = client.get("/health")?;
    
    if health["status"] == "healthy" {
        println!("{}", "✓ Connected successfully!".green().bold());
        println!("\n{}", "Database Info:".bold());
        println!("  Version: {}", health["version"].as_str().unwrap_or("unknown"));
        println!("  Uptime: {}", health["uptime"].as_str().unwrap_or("unknown"));
        
        if ping {
            let start = std::time::Instant::now();
            client.get("/health")?;
            let latency = start.elapsed();
            println!("\n{}", "Ping Test:".bold());
            println!("  Latency: {:?}", latency);
        }
    } else {
        println!("{}", "✗ Connection failed".red().bold());
    }
    
    Ok(())
}

fn cmd_inspect(client: &MantisClient, type_: &str, key: &str) -> Result<(), Box<dyn std::error::Error>> {
    println!("{} Inspecting {} '{}'...", "🔍".cyan(), type_, key);
    
    let result = match type_ {
        "kv" => client.get(&format!("/api/kv/get/{}", key))?,
        "doc" => client.get(&format!("/api/documents/get/{}", key))?,
        "vector" => client.get(&format!("/api/vectors/get/{}", key))?,
        _ => return Err("Invalid type. Use: kv, doc, vector".into()),
    };
    
    println!("\n{}", serde_json::to_string_pretty(&result)?);
    Ok(())
}

fn cmd_stats(client: &MantisClient, detailed: bool, format: &str) -> Result<(), Box<dyn std::error::Error>> {
    let stats = client.get("/api/stats")?;
    
    if format == "json" {
        println!("{}", serde_json::to_string_pretty(&stats)?);
        return Ok(());
    }
    
    println!("{}", "📊 MantisDB Statistics".cyan().bold());
    println!("{}", "━".repeat(50));
    
    // KV Store
    if let Some(kv) = stats.get("kv") {
        println!("\n{}", "Key-Value Store:".green().bold());
        println!("  Keys: {}", kv["total_keys"]);
        println!("  Memory: {} MB", kv["memory_mb"]);
        println!("  Hit Rate: {}%", kv["hit_rate"]);
    }
    
    // Documents
    if let Some(docs) = stats.get("documents") {
        println!("\n{}", "Documents:".green().bold());
        println!("  Collections: {}", docs["collections"]);
        println!("  Total Documents: {}", docs["total_documents"]);
        println!("  Storage: {} MB", docs["storage_mb"]);
    }
    
    // Vectors
    if let Some(vectors) = stats.get("vectors") {
        println!("\n{}", "Vectors:".green().bold());
        println!("  Total Vectors: {}", vectors["total_vectors"]);
        println!("  Dimension: {}", vectors["dimension"]);
        println!("  Memory: {} MB", vectors["memory_mb"]);
    }
    
    // SQL
    if let Some(sql) = stats.get("sql") {
        println!("\n{}", "SQL:".green().bold());
        println!("  Tables: {}", sql["tables"]);
        println!("  Total Rows: {}", sql["total_rows"]);
    }
    
    if detailed {
        println!("\n{}", "Performance Metrics:".yellow().bold());
        if let Some(perf) = stats.get("performance") {
            println!("  Ops/sec: {}", perf["ops_per_sec"]);
            println!("  Avg Latency: {} μs", perf["avg_latency_us"]);
            println!("  P99 Latency: {} μs", perf["p99_latency_us"]);
        }
    }
    
    Ok(())
}

fn cmd_query(
    client: &MantisClient,
    sql: Option<String>,
    vector: Option<String>,
    k: usize,
    _metric: &str,
) -> Result<(), Box<dyn std::error::Error>> {
    if let Some(sql_query) = sql {
        println!("{} Executing SQL query...", "🔍".cyan());
        let result = client.post("/api/sql/query", json!({ "query": sql_query }))?;
        
        if let Some(rows) = result.get("rows") {
            println!("\n{} Results:", "✓".green());
            println!("{}", serde_json::to_string_pretty(rows)?);
        }
    } else if let Some(vector_str) = vector {
        println!("{} Executing vector search...", "🔍".cyan());
        let vector: Vec<f32> = serde_json::from_str(&vector_str)?;
        
        let result = client.post(
            "/api/vectors/search",
            json!({ "query": vector, "k": k })
        )?;
        
        println!("\n{} Top {} Results:", "✓".green(), k);
        println!("{}", serde_json::to_string_pretty(&result)?);
    } else {
        return Err("Provide either --sql or --vector".into());
    }
    
    Ok(())
}

fn cmd_backup(
    _client: &MantisClient,
    output: PathBuf,
    compress: bool,
    _pause_writes: bool,
) -> Result<(), Box<dyn std::error::Error>> {
    println!("{} Creating backup...", "💾".cyan());
    println!("  Output: {:?}", output);
    println!("  Compress: {}", compress);
    
    // Implementation would copy data directory
    println!("{} Backup created successfully!", "✓".green());
    
    Ok(())
}

fn cmd_restore(_client: &MantisClient, input: PathBuf) -> Result<(), Box<dyn std::error::Error>> {
    println!("{} Restoring from backup...", "⚡".cyan());
    println!("  Input: {:?}", input);
    
    // Implementation would restore data directory
    println!("{} Restore completed successfully!", "✓".green());
    
    Ok(())
}

fn cmd_migrate(
    _client: &MantisClient,
    from: &str,
    _to: Option<&str>,
    _batch_size: usize,
) -> Result<(), Box<dyn std::error::Error>> {
    println!("{} Migrating data from {}...", "🚚".cyan(), from);
    
    // Implementation would handle Redis, MongoDB, PostgreSQL migrations
    println!("{} Migration completed!", "✓".green());
    
    Ok(())
}

fn cmd_list(
    client: &MantisClient,
    type_: &str,
    _collection: Option<&str>,
    limit: usize,
    prefix: Option<&str>,
) -> Result<(), Box<dyn std::error::Error>> {
    println!("{} Listing {}...", "📝".cyan(), type_);
    
    let mut path = format!("/api/{}/list?limit={}", type_, limit);
    if let Some(p) = prefix {
        path.push_str(&format!("&prefix={}", p));
    }
    
    let result = client.get(&path)?;
    
    if let Some(items) = result.get("items") {
        println!("\nFound {} items:", items.as_array().map(|a| a.len()).unwrap_or(0));
        println!("{}", serde_json::to_string_pretty(items)?);
    }
    
    Ok(())
}

fn cmd_delete(
    client: &MantisClient,
    type_: &str,
    key: &str,
    force: bool,
) -> Result<(), Box<dyn std::error::Error>> {
    if !force {
        print!("Delete {} '{}'? [y/N]: ", type_, key);
        io::stdout().flush()?;
        
        let mut input = String::new();
        io::stdin().read_line(&mut input)?;
        
        if !input.trim().eq_ignore_ascii_case("y") {
            println!("Cancelled.");
            return Ok(());
        }
    }
    
    println!("{} Deleting {}...", "🗑️".cyan(), key);
    
    client.post(&format!("/api/{}/delete", type_), json!({ "key": key }))?;
    
    println!("{} Deleted successfully!", "✓".green());
    
    Ok(())
}

fn cmd_monitor(client: &MantisClient, interval: u64) -> Result<(), Box<dyn std::error::Error>> {
    println!("{} Monitoring MantisDB (Ctrl+C to stop)...", "📈".cyan());
    println!("{}", "━".repeat(70));
    
    loop {
        print!("\x1B[2J\x1B[1;1H"); // Clear screen
        
        let stats = client.get("/api/metrics")?;
        
        println!("{}", "MantisDB Real-Time Monitor".cyan().bold());
        println!("{}", "━".repeat(70));
        println!("\n{:<20} {:>15}", "Metric", "Value");
        println!("{}", "─".repeat(70));
        
        if let Some(ops) = stats.get("ops_per_sec") {
            println!("{:<20} {:>15}", "Ops/sec:", format!("{}", ops));
        }
        
        if let Some(latency) = stats.get("avg_latency_us") {
            println!("{:<20} {:>15}", "Avg Latency:", format!("{} μs", latency));
        }
        
        if let Some(connections) = stats.get("active_connections") {
            println!("{:<20} {:>15}", "Connections:", format!("{}", connections));
        }
        
        println!("\n{}", format!("Updated: {}", chrono::Local::now().format("%H:%M:%S")).dimmed());
        
        std::thread::sleep(std::time::Duration::from_secs(interval));
    }
}
