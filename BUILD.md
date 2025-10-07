# MantisDB Build Guide

Complete guide for building MantisDB from source.

## Quick Start

```bash
# Clone the repository
git clone https://github.com/mantisdb/mantisdb.git
cd mantisdb

# Install dependencies
make deps

# Build for current platform
make build

# Run
./mantisdb
```

## Prerequisites

### Required Tools

- **Go**: 1.21 or later ([download](https://golang.org/dl/))
- **Node.js**: 18 or later ([download](https://nodejs.org/))
- **npm**: 9 or later (comes with Node.js)
- **Git**: 2.0 or later
- **Make**: GNU Make 4.0 or later

### Optional Tools (for installers)

- **Linux**: `dpkg-deb`, `rpmbuild`, or `fpm`
- **macOS**: `hdiutil`, `pkgbuild`, `lipo`
- **Windows**: `makensis` (NSIS), `wix` (WiX Toolset)

### Verify Prerequisites

```bash
# Check Go
go version

# Check Node.js
node --version

# Check npm
npm --version

# Check Git
git --version

# Check Make
make --version
```

## Build Commands

### Basic Builds

```bash
# Build for current platform
make build

# Build everything (frontend + admin + clients + binary)
make build-all

# Build only the frontend
make build-frontend

# Build only the admin API
make build-admin

# Build only client libraries
make build-clients
```

### Cross-Platform Builds

```bash
# Build for all platforms
make cross-platform

# Builds will be in ./dist/ directory:
# - mantisdb-linux-amd64
# - mantisdb-linux-arm64
# - mantisdb-darwin-amd64
# - mantisdb-darwin-arm64
# - mantisdb-windows-amd64.exe
```

### Production Builds

```bash
# Build with installers for all platforms
make production

# Or build specific components
make installers          # Create installers only
make release            # Full release build
```

### Custom Builds

```bash
# Build with specific version
make build VERSION=1.2.3

# Build with CGO enabled
CGO_ENABLED=1 make build

# Build with custom ldflags
make build LDFLAGS="-X main.CustomFlag=value"

# Build without frontend
make build SKIP_FRONTEND=true
```

## Build Targets

### Main Targets

| Target | Description |
|--------|-------------|
| `make build` | Build MantisDB binary for current platform |
| `make build-all` | Build everything (frontend, admin, clients, binary) |
| `make cross-platform` | Build for all supported platforms |
| `make installers` | Create platform-specific installers |
| `make production` | Full production build with installers |
| `make release` | Create GitHub release |
| `make install` | Install locally to ~/.local/bin |

### Development Targets

| Target | Description |
|--------|-------------|
| `make run` | Build and run with admin dashboard |
| `make run-dev` | Run in development mode with hot reload |
| `make test` | Run test suite |
| `make benchmark` | Run benchmarks |
| `make clean` | Clean build artifacts |

### Component Targets

| Target | Description |
|--------|-------------|
| `make build-frontend` | Build React admin dashboard |
| `make build-admin` | Build standalone admin API server |
| `make build-client-go` | Build Go client library |
| `make build-client-python` | Build Python client library |
| `make build-client-js` | Build JavaScript client library |

## Build Process Details

### 1. Frontend Build

The admin dashboard is built using React and Vite:

```bash
cd admin/frontend
npm install
npm run build
```

Output: `admin/assets/dist/`

### 2. Binary Build

The Go binary embeds the frontend assets:

```bash
go build -ldflags="-s -w -X main.Version=1.0.0" -o mantisdb cmd/mantisDB/main.go
```

Build flags:
- `-s -w`: Strip debug information (smaller binary)
- `-X main.Version=...`: Set version at compile time
- `-X main.BuildTime=...`: Set build timestamp
- `-X main.GitCommit=...`: Set git commit hash

### 3. Cross-Platform Build

Uses Go's cross-compilation:

```bash
GOOS=linux GOARCH=amd64 go build -o mantisdb-linux-amd64 cmd/mantisDB/main.go
GOOS=darwin GOARCH=arm64 go build -o mantisdb-darwin-arm64 cmd/mantisDB/main.go
GOOS=windows GOARCH=amd64 go build -o mantisdb-windows-amd64.exe cmd/mantisDB/main.go
```

### 4. Installer Creation

Platform-specific installers are created from binaries:

**Linux:**
- `.deb` package (Debian/Ubuntu)
- `.rpm` package (RHEL/CentOS/Fedora)
- Install script

**macOS:**
- `.dmg` disk image
- `.pkg` installer
- Homebrew formula

**Windows:**
- `.msi` installer (WiX)
- `.exe` installer (NSIS)
- PowerShell install script

## Build Configuration

### Environment Variables

```bash
# Version
export VERSION=1.2.3

# Build optimization
export BUILD_OPTIMIZATION=size  # size, speed, debug

# CGO
export CGO_ENABLED=0  # 0 or 1

# Build cache
export BUILD_CACHE=true

# Parallel builds
export PARALLEL_BUILDS=true
export MAX_PARALLEL_JOBS=4
```

### Build Config File

Create `build.config.yaml`:

```yaml
version: 1.2.3
platforms:
  - linux/amd64
  - linux/arm64
  - darwin/amd64
  - darwin/arm64
  - windows/amd64

optimization: size  # size, speed, debug
cgo_enabled: false
strip_symbols: true
compress_binaries: false

build_cache:
  enabled: true
  directory: .build-cache

parallel:
  enabled: true
  max_jobs: 4
```

## Platform-Specific Instructions

### Building on Linux

```bash
# Install dependencies
sudo apt-get update
sudo apt-get install -y build-essential git golang nodejs npm

# For creating packages
sudo apt-get install -y dpkg-dev rpm

# Or use fpm
sudo gem install fpm

# Build
make production
```

### Building on macOS

```bash
# Install Xcode Command Line Tools
xcode-select --install

# Install Homebrew
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# Install dependencies
brew install go node

# Build
make production
```

### Building on Windows

```powershell
# Install Chocolatey
Set-ExecutionPolicy Bypass -Scope Process -Force
[System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072
iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))

# Install dependencies
choco install -y golang nodejs git make

# For creating installers
choco install -y nsis wix

# Build
make production
```

## Docker Build

Build using Docker (no local dependencies needed):

```bash
# Build image
docker build -t mantisdb:latest .

# Run
docker run -p 8080:8080 -p 8081:8081 mantisdb:latest

# Or use docker-compose
docker-compose up
```

## CI/CD Integration

### GitHub Actions

See `.github/workflows/ci.yml` for the complete CI/CD pipeline.

Key steps:
1. Checkout code
2. Setup Go and Node.js
3. Install dependencies
4. Run tests
5. Build for all platforms
6. Create installers
7. Upload artifacts
8. Create GitHub release

### GitLab CI

```yaml
build:
  image: golang:1.21
  script:
    - make deps
    - make production
  artifacts:
    paths:
      - dist/
```

## Troubleshooting

### Common Issues

**Issue: `npm install` fails**
```bash
# Clear npm cache
npm cache clean --force

# Delete node_modules and reinstall
rm -rf admin/frontend/node_modules
cd admin/frontend && npm install
```

**Issue: Go build fails with missing dependencies**
```bash
# Download dependencies
go mod download

# Tidy up
go mod tidy
```

**Issue: Cross-compilation fails**
```bash
# Ensure CGO is disabled for cross-compilation
CGO_ENABLED=0 make cross-platform
```

**Issue: Permission denied on Linux**
```bash
# Make scripts executable
chmod +x scripts/*.sh

# Or run with sudo for system-wide install
sudo make install
```

### Build Cache Issues

```bash
# Clear Go build cache
go clean -cache -modcache -testcache

# Clear npm cache
npm cache clean --force

# Clear custom build cache
rm -rf .build-cache
```

### Debugging Build Issues

```bash
# Verbose build
make build VERBOSE=1

# Debug mode build
make build BUILD_OPTIMIZATION=debug

# Check build environment
go env
node --version
npm --version
```

## Performance Tips

### Faster Builds

```bash
# Use build cache
export BUILD_CACHE=true

# Parallel builds
export PARALLEL_BUILDS=true
export MAX_PARALLEL_JOBS=8

# Skip tests
make build SKIP_TESTS=true

# Skip frontend rebuild
make build SKIP_FRONTEND=true
```

### Smaller Binaries

```bash
# Strip symbols
make build STRIP_SYMBOLS=true

# Optimize for size
make build BUILD_OPTIMIZATION=size

# Compress with UPX (if available)
make build COMPRESS_BINARIES=true
```

## Advanced Topics

### Custom Build Tags

```bash
# Build with specific tags
go build -tags "cgo,production" -o mantisdb cmd/mantisDB/main.go
```

### Static Linking

```bash
# Fully static binary (Linux)
CGO_ENABLED=0 go build -ldflags="-s -w -extldflags '-static'" -o mantisdb cmd/mantisDB/main.go
```

### Reproducible Builds

```bash
# Ensure reproducible builds
export GOFLAGS="-buildvcs=false -trimpath"
make build
```

## Contributing

When contributing, ensure:

1. Code builds without errors: `make build`
2. Tests pass: `make test`
3. Code is formatted: `make fmt`
4. Linter passes: `make lint`

See [CONTRIBUTING.md](CONTRIBUTING.md) for more details.

## Support

- **Documentation**: https://mantisdb.com/docs
- **Build Issues**: https://github.com/mantisdb/mantisdb/issues
- **Discussions**: https://github.com/mantisdb/mantisdb/discussions
