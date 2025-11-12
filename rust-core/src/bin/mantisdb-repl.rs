//! MantisDB Interactive REPL
//!
//! Interactive shell for testing all database features

use std::io::{self, Write};
use std::fs;
use std::path::PathBuf;
use colored::Colorize;
use serde_json::Value;

const VERSION: &str = "1.0.0";
const HISTORY_FILE: &str = ".mantisdb_history";

struct ReplState {
    server_url: String,
    auth_token: Option<String>,
    history: Vec<String>,
    multiline_buffer: String,
    in_multiline: bool,
}

impl ReplState {
    fn new(server_url: String) -> Self {
        let mut state = Self {
            server_url,
            auth_token: None,
            history: Vec::new(),
            multiline_buffer: String::new(),
            in_multiline: false,
        };
        state.load_history();
        state
    }

    fn load_history(&mut self) {
        if let Ok(home) = std::env::var("HOME") {
            let history_path = PathBuf::from(home).join(HISTORY_FILE);
            if let Ok(content) = fs::read_to_string(history_path) {
                self.history = content.lines().map(String::from).collect();
            }
        }
    }

    fn save_history(&self) {
        if let Ok(home) = std::env::var("HOME") {
            let history_path = PathBuf::from(home).join(HISTORY_FILE);
            let content = self.history.join("\n");
            let _ = fs::write(history_path, content);
        }
    }

    fn add_to_history(&mut self, line: &str) {
        if !line.trim().is_empty() && self.history.last() != Some(&line.to_string()) {
            self.history.push(line.to_string());
            if self.history.len() > 1000 {
                self.history.remove(0);
            }
        }
    }
}

fn print_welcome() {
    println!("{}", "╔══════════════════════════════════════════════════════════╗".bright_cyan());
    println!("{}", "║           MantisDB Interactive REPL v1.0.0              ║".bright_cyan());
    println!("{}", "║                                                          ║".bright_cyan());
    println!("{}", "║  Type .help for commands, .exit to quit                 ║".bright_cyan());
    println!("{}", "╚══════════════════════════════════════════════════════════╝".bright_cyan());
    println!();
}

fn print_help() {
    println!("\n{}", "Available Commands:".bright_yellow().bold());
    println!("  {}  - Show this help message", ".help".green());
    println!("  {}  - Exit REPL", ".exit / .quit".green());
    println!("  {}  - Clear screen", ".clear".green());
    println!("  {}  - Show command history", ".history".green());
    println!("  {}  - Show server status", ".status".green());
    println!("  {}  - Show database tables", ".tables".green());
    println!("  {}  - Show current connection", ".connection".green());
    println!("  {}  - Set auth token", ".auth <token>".green());
    println!("  {}  - Switch database mode", ".mode <kv|doc|sql|vector>".green());
    println!();
    println!("{}", "Query Examples:".bright_yellow().bold());
    println!("  {}  - SQL query", "SELECT * FROM users;".cyan());
    println!("  {}  - Key-value get", "GET user:123".cyan());
    println!("  {}  - Key-value set", "SET user:123 'John Doe'".cyan());
    println!("  {}  - Document find", "FIND users WHERE age > 25".cyan());
    println!("  {}  - Vector search", "VSEARCH embeddings [0.1,0.2,0.3] k=10".cyan());
    println!();
    println!("{}", "Multi-line Mode:".bright_yellow().bold());
    println!("  End lines with {} to continue to next line", "\\".yellow());
    println!("  Empty line executes multi-line command");
    println!();
}

fn execute_command(state: &mut ReplState, command: &str) -> Result<(), Box<dyn std::error::Error>> {
    let trimmed = command.trim();
    
    // Handle dot commands
    if trimmed.starts_with('.') {
        let parts: Vec<&str> = trimmed.splitn(2, ' ').collect();
        let cmd = parts[0];
        let args = parts.get(1).copied().unwrap_or("");
        
        match cmd {
            ".help" => print_help(),
            ".exit" | ".quit" => std::process::exit(0),
            ".clear" => {
                print!("\x1B[2J\x1B[1;1H");
                print_welcome();
            },
            ".history" => {
                println!("\n{}", "Command History:".bright_yellow().bold());
                for (i, cmd) in state.history.iter().enumerate().rev().take(20) {
                    println!("  {} {}", format!("{:3}.", i + 1).dimmed(), cmd);
                }
                println!();
            },
            ".status" => {
                println!("\n{}", "Server Status:".bright_yellow().bold());
                match fetch_health(&state.server_url, &state.auth_token) {
                    Ok(status) => {
                        println!("  {} {}", "Status:".green(), status["status"].as_str().unwrap_or("unknown"));
                        println!("  {} {}", "Version:".green(), status["version"].as_str().unwrap_or("unknown"));
                        println!("  {} {}", "Timestamp:".green(), status["timestamp"].as_str().unwrap_or("unknown"));
                    },
                    Err(e) => println!("  {} {}", "Error:".red(), e),
                }
                println!();
            },
            ".tables" => {
                println!("\n{}", "Database Tables:".bright_yellow().bold());
                match fetch_tables(&state.server_url, &state.auth_token) {
                    Ok(tables) => {
                        if let Some(arr) = tables.as_array() {
                            for table in arr {
                                if let Some(name) = table["name"].as_str() {
                                    println!("  {} {}", "•".cyan(), name);
                                }
                            }
                        }
                    },
                    Err(e) => println!("  {} {}", "Error:".red(), e),
                }
                println!();
            },
            ".connection" => {
                println!("\n{}", "Connection Info:".bright_yellow().bold());
                println!("  {} {}", "Server:".green(), state.server_url);
                println!("  {} {}", "Auth:".green(), 
                    if state.auth_token.is_some() { "✓ Configured" } else { "✗ None" });
                println!();
            },
            ".auth" => {
                if args.is_empty() {
                    println!("{} Usage: .auth <token>", "Error:".red());
                } else {
                    state.auth_token = Some(args.to_string());
                    println!("{} Auth token set", "✓".green());
                }
            },
            ".mode" => {
                if args.is_empty() {
                    println!("{} Usage: .mode <kv|doc|sql|vector>", "Error:".red());
                } else {
                    println!("{} Switched to {} mode", "✓".green(), args.cyan());
                }
            },
            _ => println!("{} Unknown command: {}", "Error:".red(), cmd),
        }
        return Ok(());
    }
    
    // Execute query
    execute_query(state, trimmed)?;
    
    Ok(())
}

fn execute_query(state: &ReplState, query: &str) -> Result<(), Box<dyn std::error::Error>> {
    if query.is_empty() {
        return Ok(());
    }
    
    // Determine query type and execute
    let upper = query.to_uppercase();
    
    if upper.starts_with("SELECT") || upper.starts_with("INSERT") || 
       upper.starts_with("UPDATE") || upper.starts_with("DELETE") ||
       upper.starts_with("CREATE") || upper.starts_with("DROP") {
        // SQL query
        execute_sql(state, query)?;
    } else if upper.starts_with("GET ") {
        // Key-value GET
        let key = query[4..].trim();
        execute_kv_get(state, key)?;
    } else if upper.starts_with("SET ") {
        // Key-value SET
        let parts: Vec<&str> = query[4..].splitn(2, ' ').collect();
        if parts.len() == 2 {
            execute_kv_set(state, parts[0].trim(), parts[1].trim())?;
        } else {
            println!("{} Usage: SET <key> <value>", "Error:".red());
        }
    } else if upper.starts_with("FIND ") {
        // Document find
        println!("{} Document queries not yet implemented in REPL", "Info:".yellow());
    } else if upper.starts_with("VSEARCH ") {
        // Vector search
        println!("{} Vector queries not yet implemented in REPL", "Info:".yellow());
    } else {
        // Default to SQL
        execute_sql(state, query)?;
    }
    
    Ok(())
}

fn execute_sql(state: &ReplState, query: &str) -> Result<(), Box<dyn std::error::Error>> {
    let client = reqwest::blocking::Client::new();
    let mut req = client.post(format!("{}/api/query", state.server_url))
        .json(&serde_json::json!({ "query": query }));
    
    if let Some(token) = &state.auth_token {
        req = req.header("Authorization", format!("Bearer {}", token));
    }
    
    let response = req.send()?;
    let status = response.status();
    let body: Value = response.json()?;
    
    if status.is_success() {
        if let Some(results) = body["results"].as_array() {
            println!("\n{} Returned {} rows", "✓".green(), results.len());
            if !results.is_empty() {
                println!("{}", serde_json::to_string_pretty(&results)?);
            }
        } else {
            println!("\n{} Query executed successfully", "✓".green());
        }
    } else {
        println!("\n{} {}", "Error:".red(), body["error"].as_str().unwrap_or("Unknown error"));
    }
    println!();
    
    Ok(())
}

fn execute_kv_get(state: &ReplState, key: &str) -> Result<(), Box<dyn std::error::Error>> {
    let client = reqwest::blocking::Client::new();
    let mut req = client.get(format!("{}/api/kv/{}", state.server_url, key));
    
    if let Some(token) = &state.auth_token {
        req = req.header("Authorization", format!("Bearer {}", token));
    }
    
    let response = req.send()?;
    let status = response.status();
    let body: Value = response.json()?;
    
    if status.is_success() {
        if let Some(value) = body["value"].as_str() {
            println!("\n{} {}", key.cyan(), "=>".dimmed());
            println!("  {}", value);
        }
    } else {
        println!("\n{} Key not found", "✗".red());
    }
    println!();
    
    Ok(())
}

fn execute_kv_set(state: &ReplState, key: &str, value: &str) -> Result<(), Box<dyn std::error::Error>> {
    let client = reqwest::blocking::Client::new();
    let mut req = client.post(format!("{}/api/kv", state.server_url))
        .json(&serde_json::json!({ "key": key, "value": value }));
    
    if let Some(token) = &state.auth_token {
        req = req.header("Authorization", format!("Bearer {}", token));
    }
    
    let response = req.send()?;
    
    if response.status().is_success() {
        println!("\n{} Set {} = {}", "✓".green(), key.cyan(), value);
    } else {
        println!("\n{} Failed to set key", "✗".red());
    }
    println!();
    
    Ok(())
}

fn fetch_health(url: &str, token: &Option<String>) -> Result<Value, Box<dyn std::error::Error>> {
    let client = reqwest::blocking::Client::new();
    let mut req = client.get(format!("{}/api/health", url));
    
    if let Some(t) = token {
        req = req.header("Authorization", format!("Bearer {}", t));
    }
    
    Ok(req.send()?.json()?)
}

fn fetch_tables(url: &str, token: &Option<String>) -> Result<Value, Box<dyn std::error::Error>> {
    let client = reqwest::blocking::Client::new();
    let mut req = client.get(format!("{}/api/tables", url));
    
    if let Some(t) = token {
        req = req.header("Authorization", format!("Bearer {}", t));
    }
    
    let body: Value = req.send()?.json()?;
    Ok(body["tables"].clone())
}

fn main() {
    let server_url = std::env::args()
        .nth(1)
        .unwrap_or_else(|| "http://localhost:8080".to_string());
    
    let mut state = ReplState::new(server_url);
    
    print_welcome();
    
    loop {
        // Print prompt
        let prompt = if state.in_multiline {
            "   ... ".dimmed().to_string()
        } else {
            format!("{} ", "mantis>".bright_green().bold())
        };
        
        print!("{}", prompt);
        io::stdout().flush().unwrap();
        
        // Read input
        let mut input = String::new();
        if io::stdin().read_line(&mut input).is_err() {
            break;
        }
        
        let input = input.trim_end();
        
        // Handle multi-line mode
        if input.ends_with('\\') {
            state.in_multiline = true;
            state.multiline_buffer.push_str(&input[..input.len()-1]);
            state.multiline_buffer.push('\n');
            continue;
        }
        
        if state.in_multiline {
            state.multiline_buffer.push_str(input);
            if input.is_empty() {
                // Execute multi-line command
                let command = state.multiline_buffer.clone();
                state.multiline_buffer.clear();
                state.in_multiline = false;
                
                state.add_to_history(&command);
                if let Err(e) = execute_command(&mut state, &command) {
                    println!("{} {}", "Error:".red(), e);
                }
            } else {
                state.multiline_buffer.push('\n');
            }
            continue;
        }
        
        // Single line command
        state.add_to_history(input);
        if let Err(e) = execute_command(&mut state, input) {
            println!("{} {}", "Error:".red(), e);
        }
    }
    
    state.save_history();
    println!("\n{}", "Goodbye! 👋".bright_cyan());
}
