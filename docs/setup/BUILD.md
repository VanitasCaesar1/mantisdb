# Building MantisDB

## Quick Start

```bash
./build.sh
```

That's it! This builds MantisDB with the high-performance Rust core.

## What Gets Built

- **Rust Core**: Lock-free storage engine and LRU cache (50K+ ops/sec)
- **Frontend**: React admin dashboard
- **Binary**: `./mantisdb` - Single executable with everything embedded

## Requirements

- **Go** 1.21+ 
- **Rust/Cargo** (for the high-performance core)
- **Node.js/npm** (for admin dashboard)

## Build Commands

```bash
# Build everything
./build.sh

# Or use make
make build

# Build and run
make run

# Clean everything
make clean
```

## Force Frontend Rebuild

```bash
REBUILD_FRONTEND=1 ./build.sh
```

## Running

```bash
# Start the database
./mantisdb

# With admin dashboard
./mantisdb --admin-port=8081

# Then visit http://localhost:8081
```

## Performance

MantisDB uses Rust for critical performance paths:
- **Storage Engine**: Lock-free concurrent data structures
- **Cache**: Lock-free LRU with high hit rates
- **Target**: 50,000+ operations/second

## Troubleshooting

### Rust not found
```bash
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
```

### Frontend build fails
```bash
cd admin/frontend
rm -rf node_modules
npm install
cd ../..
./build.sh
```

### Go build fails
```bash
go mod tidy
./build.sh
```

That's all you need to know! 🚀
