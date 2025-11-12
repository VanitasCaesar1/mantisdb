//! MantisDB simple migration CLI: export/import table rows to/from JSONL
//!
//! Usage:
//!   mantisdb-migrate export <data_dir> <table> <output.jsonl>
//!   mantisdb-migrate import <data_dir> <table> <input.jsonl>

use std::env;
use std::fs::{File};
use std::io::{self, BufRead, BufReader, BufWriter, Write};
use std::path::PathBuf;

use mantisdb_core::persistent_storage::{PersistentStorage, PersistentStorageConfig};

fn print_usage() {
    eprintln!("Usage:\n  mantisdb-migrate export <data_dir> <table> <output.jsonl>\n  mantisdb-migrate import <data_dir> <table> <input.jsonl>");
}

fn open_storage(data_dir: PathBuf) -> Result<PersistentStorage, Box<dyn std::error::Error>> {
    let config = PersistentStorageConfig { data_dir, wal_enabled: true, sync_on_write: true };
    let storage = PersistentStorage::new(config)?;
    Ok(storage)
}

fn export_table(mut storage: PersistentStorage, table: &str, output: &str) -> Result<(), Box<dyn std::error::Error>> {
    let prefix = format!("table/{}/", table);
    let entries = storage.memory().scan_prefix(&prefix);

    let file = File::create(output)?;
    let mut writer = BufWriter::new(file);

    let mut count = 0usize;
    for (_key, value) in entries.into_iter() {
        // Values are JSON; write one JSON row per line
        writer.write_all(&value)?;
        writer.write_all(b"\n")?;
        count += 1;
    }
    writer.flush()?;

    eprintln!("Exported {} rows from table '{}'", count, table);
    Ok(())
}

fn import_table(mut storage: PersistentStorage, table: &str, input: &str) -> Result<(), Box<dyn std::error::Error>> {
    let file = File::open(input)?;
    let reader = BufReader::new(file);

    let mut count = 0usize;
    for line in reader.lines() {
        let line = line?;
        if line.trim().is_empty() { continue; }

        // Ensure each row has an id; if not, generate one
        let mut value: serde_json::Value = serde_json::from_str(&line)?;
        let id = if let Some(id) = value.get("id").and_then(|v| v.as_str()) {
            id.to_string()
        } else {
            let new_id = uuid::Uuid::new_v4().to_string();
            if let serde_json::Value::Object(ref mut map) = value {
                map.insert("id".to_string(), serde_json::Value::String(new_id.clone()));
            }
            new_id
        };

        let key = format!("table/{}/{}", table, id);
        let bytes = serde_json::to_vec(&value)?;
        storage.put(key, bytes)?;
        count += 1;
    }

    eprintln!("Imported {} rows into table '{}'", count, table);
    Ok(())
}

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let args: Vec<String> = env::args().collect();
    if args.len() < 5 {
        print_usage();
        std::process::exit(1);
    }

    let cmd = args[1].as_str();
    let data_dir = PathBuf::from(&args[2]);
    let table = &args[3];
    let path = &args[4];

    match cmd {
        "export" => {
            let storage = open_storage(data_dir)?;
            export_table(storage, table, path)?;
        }
        "import" => {
            let storage = open_storage(data_dir)?;
            import_table(storage, table, path)?;
        }
        _ => {
            print_usage();
            std::process::exit(1);
        }
    }

    Ok(())
}
