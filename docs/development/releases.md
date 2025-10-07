# MantisDB Release Guide

This guide explains how to create a complete release of MantisDB with installers for all platforms.

## Quick Release

To create a complete release with all installers:

```bash
# Build everything (binaries + installers)
make build-release

# Create GitHub release
make release
```

## What Gets Built

### Binaries

- `mantisdb-linux-amd64` / `mantisdb-linux-amd64.tar.gz`
- `mantisdb-linux-arm64` / `mantisdb-linux-arm64.tar.gz`
- `mantisdb-darwin-amd64` / `mantisdb-darwin-amd64.tar.gz`
- `mantisdb-darwin-arm64` / `mantisdb-darwin-arm64.tar.gz`
- `mantisdb-windows-amd64.exe` / `mantisdb-windows-amd64.zip`

### Installers

- **macOS**: `.dmg` files with app bundles and install scripts
- **Windows**: `.exe` installers (NSIS) or `.zip` with batch installers
- **Linux**: `.deb` and `.rpm` packages with systemd services
- **Homebrew**: Formula file for macOS package manager

### Installation Scripts

- `install.sh` - Universal installer for Linux/macOS
- `install.ps1` - PowerShell installer for Windows
- `RELEASE_NOTES.md` - Comprehensive release documentation

## File Structure After Build

```
dist/
├── mantisdb-linux-amd64.tar.gz
├── mantisdb-linux-arm64.tar.gz
├── mantisdb-darwin-amd64.tar.gz
├── mantisdb-darwin-arm64.tar.gz
├── mantisdb-windows-amd64.zip
├── checksums.txt
├── install.sh
├── install.ps1
├── RELEASE_NOTES.md
└── installers/
    ├── MantisDB-1.0.0-macOS-amd64.dmg
    ├── MantisDB-1.0.0-macOS-arm64.dmg
    ├── MantisDB-1.0.0-Windows-amd64.exe
    ├── mantisdb_1.0.0_amd64.deb
    ├── mantisdb_1.0.0_arm64.deb
    ├── mantisdb-1.0.0-1.x86_64.rpm
    ├── mantisdb-1.0.0-1.aarch64.rpm
    └── homebrew/
        └── mantisdb.rb
```

## Installation Methods for Users

### 1. Quick Install (Recommended)

**Linux/macOS:**

```bash
curl -fsSL https://github.com/mantisdb/mantisdb/releases/latest/download/install.sh | bash
```

**Windows:**

```powershell
iwr -useb https://github.com/mantisdb/mantisdb/releases/latest/download/install.ps1 | iex
```

### 2. Package Managers

**Homebrew (macOS):**

```bash
brew tap mantisdb/tap
brew install mantisdb
```

**APT (Ubuntu/Debian):**

```bash
sudo apt install ./mantisdb_1.0.0_amd64.deb
```

**YUM/DNF (RHEL/CentOS):**

```bash
sudo rpm -i mantisdb-1.0.0-1.x86_64.rpm
```

### 3. Platform-Specific Installers

- **macOS**: Double-click `.dmg` file
- **Windows**: Run `.exe` installer as Administrator
- **Linux**: Use package manager or extract `.tar.gz`

## Build Configuration

### Environment Variables

```bash
# Set version (auto-detected from git tags)
export VERSION="1.0.0"

# Build type
export BUILD_TYPE="release"

# Enable/disable installer creation
export CREATE_INSTALLERS="true"

# Parallel builds
export PARALLEL_BUILDS="true"
export MAX_PARALLEL_JOBS="4"

# Optimization level
export BUILD_OPTIMIZATION="size"  # size|speed|debug

# CGO settings
export CGO_ENABLED="auto"  # auto|true|false
```

### Build Scripts

1. **`scripts/build.sh`** - Basic build script
2. **`scripts/build-production.sh`** - Production builds with optimization
3. **`scripts/create-installers.sh`** - Creates platform-specific installers
4. **`scripts/build-release.sh`** - Complete release build
5. **`scripts/release.sh`** - GitHub release creation

## Makefile Targets

```bash
# Basic builds
make build              # Build binary with admin dashboard
make build-all          # Build everything (frontend, clients, binary)
make cross-platform     # Build for all platforms

# Production builds
make production         # Production binaries with installers
make build-release      # Complete release with installers and scripts

# Release
make release           # Create GitHub release with all artifacts
```

## Customization

### Installer Branding

Edit `scripts/create-installers.sh` to customize:

- Company name and description
- Installation paths
- Service configuration
- Icons and branding

### Build Optimization

Edit `scripts/build-production.sh` for:

- Compiler flags
- CGO settings
- Binary compression
- Build caching

### Release Notes

The release notes are auto-generated but can be customized in:

- `scripts/build-release.sh` (template)
- `scripts/release.sh` (GitHub release)

## Prerequisites for Building

### Required Tools

- Go 1.19+
- Node.js 18+
- npm
- git
- tar/zip

### Optional Tools (for advanced installers)

- **macOS**: `hdiutil`, `pkgbuild`
- **Windows**: NSIS (`makensis`)
- **Linux**: `fpm`, `dpkg-deb`, `rpmbuild`

### Installation

**macOS:**

```bash
# Install Homebrew if needed
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# Install tools
brew install go node fpm
```

**Ubuntu/Debian:**

```bash
# Install tools
sudo apt update
sudo apt install golang-go nodejs npm build-essential
gem install fpm
```

**Windows:**

```powershell
# Install Chocolatey if needed
Set-ExecutionPolicy Bypass -Scope Process -Force; [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072; iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))

# Install tools
choco install golang nodejs nsis
```

## Testing Installers

### Local Testing

```bash
# Build and test locally
make build-release

# Test installation scripts
./dist/install.sh --dry-run  # Linux/macOS
# Or manually test installers on target platforms
```

### CI/CD Integration

The build scripts are designed to work with GitHub Actions:

```yaml
name: Release
on:
  push:
    tags: ["v*"]

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v3
        with:
          go-version: "1.19"
      - uses: actions/setup-node@v3
        with:
          node-version: "18"

      - name: Build Release
        run: make build-release

      - name: Create GitHub Release
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: make release
```

## Distribution

### GitHub Releases

The `make release` command automatically:

1. Creates a GitHub release
2. Uploads all artifacts
3. Generates release notes
4. Calculates checksums

### Package Repositories

For broader distribution, consider:

- **Homebrew Tap**: Submit the generated formula
- **APT Repository**: Host `.deb` packages
- **YUM Repository**: Host `.rpm` packages
- **Chocolatey**: Submit Windows package
- **Snap Store**: Create snap package
- **Docker Hub**: Publish container images

## Troubleshooting

### Build Issues

```bash
# Clean and rebuild
make clean
make build-release

# Check dependencies
./scripts/build-production.sh --help

# Verbose output
DEBUG=1 make build-release
```

### Installer Issues

```bash
# Test installer creation separately
./scripts/create-installers.sh --version=test

# Check installer dependencies
./scripts/create-installers.sh --check-deps
```

### Platform-Specific Issues

- **macOS**: Code signing may be required for distribution
- **Windows**: Antivirus software may flag unsigned executables
- **Linux**: Different distributions may have different requirements

## Security Considerations

### Code Signing

For production releases, consider code signing:

- **macOS**: Apple Developer Certificate
- **Windows**: Authenticode certificate
- **Linux**: GPG signing for packages

### Checksums

All releases include SHA256 checksums in `checksums.txt`:

```bash
# Verify integrity
sha256sum -c checksums.txt
```

### Reproducible Builds

The build system supports reproducible builds:

- Fixed build timestamps
- Consistent compiler flags
- Deterministic archive creation

## Support

For build and release issues:

1. Check the [build logs](https://github.com/mantisdb/mantisdb/actions)
2. Review [INSTALLERS.md](INSTALLERS.md) for detailed instructions
3. Create an issue with build environment details

## Next Steps

After creating a release:

1. Test installers on target platforms
2. Update documentation
3. Announce the release
4. Monitor for issues
5. Plan next release cycle
