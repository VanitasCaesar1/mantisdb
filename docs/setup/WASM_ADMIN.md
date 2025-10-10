# MantisDB WASM Admin Dashboard

## Overview

The admin dashboard is now built with **WebAssembly (WASM)** - the entire UI is written in Go and compiled to WASM. This provides:

✅ **Single Language** - Everything in Go (backend + frontend)  
✅ **Type Safety** - Full Go type system in the browser  
✅ **Performance** - Near-native speed in the browser  
✅ **Small Bundle** - ~3MB WASM module (no React/Node dependencies)  
✅ **Embedded** - Compiled into the binary using Go's `embed` package  

## Architecture

```
┌─────────────────────────────────────────┐
│         mantisdb (binary)               │
│                                         │
│  ┌───────────────────────────────────┐ │
│  │  Embedded WASM (embed.FS)        │ │
│  │  - index.html                     │ │
│  │  - admin.wasm (3MB)               │ │
│  │  - wasm_exec.js                   │ │
│  └───────────────────────────────────┘ │
│                                         │
│  ┌───────────────────────────────────┐ │
│  │  Admin API (Go HTTP)             │ │
│  │  - /api/health                    │ │
│  │  - /api/metrics                   │ │
│  │  - /api/tables                    │ │
│  │  - /api/query                     │ │
│  └───────────────────────────────────┘ │
└─────────────────────────────────────────┘
```

## How It Works

### 1. Go to WASM Compilation
```bash
cd admin/api/wasm
GOOS=js GOARCH=wasm go build -o admin.wasm main.go
```

This compiles Go code to WebAssembly that runs in the browser.

### 2. Browser Execution
```javascript
const go = new Go();
WebAssembly.instantiateStreaming(fetch("admin.wasm"), go.importObject)
  .then((result) => go.run(result.instance));
```

The browser downloads and executes the WASM module.

### 3. DOM Manipulation
```go
document := js.Global().Get("document")
root := document.Call("getElementById", "root")
root.Set("innerHTML", html)
```

Go code directly manipulates the DOM using `syscall/js`.

### 4. API Calls
```go
promise := js.Global().Call("fetch", "/api/metrics")
// Handle promise and parse JSON
```

WASM makes HTTP calls to the Go backend API.

## Building

### Build WASM Module Only
```bash
cd admin/api/wasm
./build.sh
```

### Build Complete Binary
```bash
make build
```

This will:
1. Build the WASM module
2. Embed it into the Go binary
3. Create a single executable

## Running

```bash
./mantisdb

# Access dashboard
open http://localhost:8081
```

## Features

### Current Features
- ✅ Real-time metrics display
- ✅ Table browser
- ✅ Query editor (SQL, Document, Key-Value)
- ✅ Query history
- ✅ Modern dark theme UI
- ✅ Responsive design

### Query Examples

**SQL:**
```sql
SELECT * FROM users WHERE age > 18
```

**Document:**
```json
{ "collection": "users", "filter": { "age": { "$gt": 18 } } }
```

**Key-Value:**
```
GET mykey
SET mykey myvalue
DELETE mykey
```

## Development

### Modify the UI
Edit `admin/api/wasm/main.go`:

```go
func renderUI() {
    // Change HTML/CSS here
    html := `<div>Your custom UI</div>`
    root.Set("innerHTML", html)
}
```

### Rebuild
```bash
cd admin/api/wasm
./build.sh
cd ../../..
go build -o mantisdb cmd/mantisDB/main.go
./mantisdb
```

### Hot Reload (Development)
```bash
# Terminal 1: Auto-rebuild WASM
cd admin/api/wasm
while true; do
    ./build.sh
    sleep 2
done

# Terminal 2: Run server from wasm directory
cd admin/api/wasm
python3 -m http.server 8081
```

Then edit `main.go` and refresh browser.

## API Integration

The WASM frontend calls these endpoints:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/health` | GET | Health check |
| `/api/metrics` | GET | System metrics |
| `/api/tables` | GET | List all tables |
| `/api/query` | POST | Execute query |
| `/api/query/history` | GET | Query history |
| `/api/backups` | GET | List backups |
| `/api/backups` | POST | Create backup |

## Browser Compatibility

WASM requires modern browsers:
- ✅ Chrome 57+
- ✅ Firefox 52+
- ✅ Safari 11+
- ✅ Edge 16+

## Performance

| Metric | Value |
|--------|-------|
| WASM Size | ~3MB |
| Load Time | <1s (first load) |
| Startup Time | <100ms |
| Memory Usage | ~10MB |
| API Response | <50ms |

## Advantages Over React

| Feature | WASM (Go) | React |
|---------|-----------|-------|
| Language | Go | JavaScript/TypeScript |
| Build Tool | Go compiler | npm, webpack, vite |
| Dependencies | 0 | 300+ packages |
| Bundle Size | 3MB | 200KB+ (min) |
| Type Safety | Full Go types | TypeScript (optional) |
| Performance | Near-native | JIT optimized |
| Learning Curve | Go only | JS + React + tooling |

## Troubleshooting

### Dashboard shows loading forever
1. Check browser console for errors
2. Verify WASM file loads: `curl -I http://localhost:8081/admin.wasm`
3. Check CORS headers are set correctly

### WASM compilation fails
```bash
# Verify Go version
go version  # Should be 1.16+

# Clean and rebuild
cd admin/api/wasm
rm -f admin.wasm wasm_exec.js
./build.sh
```

### API calls fail
1. Check API is running: `curl http://localhost:8081/api/health`
2. Verify CORS headers in browser console
3. Check authentication token if required

## File Structure

```
admin/api/
├── assets.go           # Embed directive for WASM files
├── server.go          # Admin API handlers
└── wasm/
    ├── main.go        # WASM frontend code
    ├── build.sh       # Build script
    ├── index.html     # HTML shell
    ├── admin.wasm     # Compiled WASM (generated)
    └── wasm_exec.js   # Go WASM runtime (generated)
```

## Next Steps

### Planned Features
- [ ] Real-time metrics streaming (WebSocket)
- [ ] Advanced query builder
- [ ] Schema visualization
- [ ] Backup/restore UI
- [ ] User management
- [ ] Performance profiling
- [ ] Log viewer

### Contributing

To add new features:

1. Edit `admin/api/wasm/main.go`
2. Add new functions for UI components
3. Wire up event listeners
4. Make API calls as needed
5. Rebuild and test

Example - Add a new section:
```go
func renderNewSection() {
    html := `
        <section class="new-section">
            <h2>New Feature</h2>
            <div id="newContent"></div>
        </section>
    `
    // Add to main renderUI()
}
```

## Resources

- [Go WASM Wiki](https://github.com/golang/go/wiki/WebAssembly)
- [syscall/js Package](https://pkg.go.dev/syscall/js)
- [WASM Specification](https://webassembly.org/)
- [Go Blog: WASM](https://go.dev/blog/wasm)

## Summary

The WASM-based admin dashboard provides a modern, performant, and maintainable solution using pure Go. No JavaScript frameworks, no npm dependencies, just Go compiled to run in the browser.

**Build once, run anywhere - truly portable database with embedded admin UI.**
