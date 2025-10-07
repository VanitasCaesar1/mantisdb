# Building MantisDB Installers

This guide covers building and customizing MantisDB installers for all supported platforms.

## Quick Start

### Build Everything

```bash
# Complete release build with all installers
./scripts/build-release.sh --version=1.0.0

# Or step by step
./scripts/build.sh --all --version=1.0.0
```

### Platform-Specific Builds

```bash
# Windows installer only
./scripts/build.sh --cross-platform --version=1.0.0
./scripts/create-installers.sh --platform=windows --version=1.0.0

# macOS DMG (requires macOS)
./scripts/create-dmg.sh --version=1.0.0

# Homebrew formula
./scripts/create-homebrew.sh --version=1.0.0

# Linux packages
./scripts/create-installers.sh --platform=linux --version=1.0.0
```

## Prerequisites

### All Platforms
- **Go**: 1.21 or later
- **Node.js**: 18 or later (for admin dashboard)
- **Git**: Latest version

### macOS
- Xcode Command Line Tools
- `hdiutil` (included with macOS)

### Windows
- NSIS (Nullsoft Scriptable Install System) for professional installers
- Or use the batch/PowerShell fallback installers

### Linux
- `fpm` (Effing Package Management): `gem install fpm`
- `dpkg-deb` for DEB packages
- `rpmbuild` for RPM packages

## Build Scripts

### `scripts/build.sh`

Main build script with cross-platform support.

```bash
# Usage
./scripts/build.sh [OPTIONS]

# Options
--version VERSION     # Set build version
--cross-platform      # Build for all platforms
--installers          # Create installers
--all                 # Build everything
```

**Examples:**
```bash
# Build for current platform
./scripts/build.sh --version=1.0.0

# Build for all platforms
./scripts/build.sh --cross-platform --version=1.0.0

# Build everything including installers
./scripts/build.sh --all --version=1.0.0
```

### `scripts/build-release.sh`

Complete release builder with tests and documentation.

```bash
# Usage
./scripts/build-release.sh [OPTIONS]

# Options
--version VERSION     # Release version (required)
--skip-tests          # Skip test execution
--skip-frontend       # Skip frontend build
--skip-clients        # Skip client builds
```

**Examples:**
```bash
# Full release build
./scripts/build-release.sh --version=1.0.0

# Quick build without tests
./scripts/build-release.sh --version=1.0.0 --skip-tests
```

### `scripts/create-installers.sh`

Platform-specific installer creator.

```bash
# Usage
./scripts/create-installers.sh [OPTIONS]

# Options
--version VERSION     # Installer version (required)
--platform PLATFORM   # windows, macos, linux, or all
```

**Examples:**
```bash
# Create all installers
./scripts/create-installers.sh --version=1.0.0 --platform=all

# Create Windows installer only
./scripts/create-installers.sh --version=1.0.0 --platform=windows
```

### `scripts/create-dmg.sh`

macOS DMG creator (macOS only).

```bash
# Usage (macOS only)
./scripts/create-dmg.sh [--version VERSION]
```

### `scripts/create-homebrew.sh`

Homebrew formula generator.

```bash
# Usage
./scripts/create-homebrew.sh [--version VERSION]
```

## Available Installer Types

### Windows

#### Batch Installer (`install.bat`)
- Simple, works everywhere
- No dependencies required
- Basic installation to Program Files
- Adds to system PATH

#### PowerShell Installer (`install.ps1`)
- Modern, feature-rich
- Advanced configuration options
- Service installation support
- Desktop shortcuts

#### NSIS Installer
- Professional GUI installer
- Uninstaller included
- Start Menu shortcuts
- Windows service utilities

**Features:**
- Installs to Program Files
- Adds to system PATH
- Creates Start Menu shortcuts
- Windows service utilities
- Uninstaller included

### macOS

#### DMG Disk Image
- Drag-and-drop installation
- App bundle with proper metadata
- Command-line symlink creation
- Universal binary support

#### Homebrew Formula
- Package manager integration
- Automatic dependency management
- Service integration via launchd
- Easy updates

**Features:**
- App bundle with proper metadata
- Command-line symlink creation
- Homebrew service integration
- Universal binary support (Intel + Apple Silicon)

### Linux

#### DEB Package (Debian/Ubuntu)
- Native package manager integration
- Automatic dependency resolution
- Systemd service integration
- FHS-compliant file locations

#### RPM Package (RHEL/CentOS/Fedora)
- Native package manager integration
- Automatic dependency resolution
- Systemd service integration
- FHS-compliant file locations

#### TAR.GZ Archive
- Universal Linux distribution
- No dependencies required
- Manual installation
- Portable

**Features:**
- Systemd service integration
- Proper user/group creation
- FHS-compliant file locations
- Package manager integration

## Build Process

### Step 1: Prepare Environment

```bash
# Clone repository
git clone https://github.com/mantisdb/mantisdb.git
cd mantisdb

# Install dependencies
make deps

# Verify tools
go version
node --version
npm --version
```

### Step 2: Build Binaries

```bash
# Build for current platform
make build

# Build for all platforms
make production

# Or use build script
./scripts/build.sh --cross-platform --version=1.0.0
```

### Step 3: Create Installers

```bash
# Create all installers
./scripts/create-installers.sh --version=1.0.0 --platform=all

# Or create specific platform installers
./scripts/create-installers.sh --version=1.0.0 --platform=windows
./scripts/create-installers.sh --version=1.0.0 --platform=macos
./scripts/create-installers.sh --version=1.0.0 --platform=linux
```

### Step 4: Test Installers

```bash
# Test on target platforms
# Windows: Run install.bat or install.ps1 as Administrator
# macOS: Mount DMG and test installation
# Linux: Install DEB/RPM package and verify service
```

### Step 5: Package for Release

```bash
# Create release archives
./scripts/build-release.sh --version=1.0.0

# Output will be in dist/ directory
ls -la dist/
```

## Customization

### Modifying Installation Paths

Edit `scripts/create-installers.sh`:

```bash
# Windows
INSTALL_DIR="C:\Program Files\MantisDB"
CONFIG_DIR="%PROGRAMDATA%\MantisDB"
DATA_DIR="%LOCALAPPDATA%\MantisDB\data"

# macOS
INSTALL_DIR="/Applications/MantisDB.app"
CONFIG_DIR="/usr/local/etc/mantisdb"
DATA_DIR="/usr/local/var/lib/mantisdb"

# Linux
INSTALL_DIR="/usr/bin"
CONFIG_DIR="/etc/mantisdb"
DATA_DIR="/var/lib/mantisdb"
```

### Customizing Service Configuration

Edit systemd service template in `scripts/create-installers.sh`:

```ini
[Unit]
Description=MantisDB Database Server
After=network.target

[Service]
Type=simple
User=mantisdb
Group=mantisdb
ExecStart=/usr/bin/mantisdb --config=/etc/mantisdb/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

### Adding Desktop Shortcuts

For Windows, edit the PowerShell installer:

```powershell
$WshShell = New-Object -ComObject WScript.Shell
$Shortcut = $WshShell.CreateShortcut("$env:USERPROFILE\Desktop\MantisDB.lnk")
$Shortcut.TargetPath = "C:\Program Files\MantisDB\mantisdb.exe"
$Shortcut.Save()
```

### Customizing Branding and Icons

1. Replace icons in `assets/icons/`
2. Update app metadata in build scripts
3. Modify DMG background image
4. Update NSIS installer graphics

## Testing

### Local Testing

```bash
# Test build process
./scripts/build.sh --version=1.0.0-test

# Test installer creation
./scripts/create-installers.sh --version=1.0.0-test --platform=all

# Verify output
ls -la dist/
```

### Platform-Specific Testing

**Windows:**
```powershell
# Extract installer package
Expand-Archive -Path dist/mantisdb-windows-amd64.zip -DestinationPath test/

# Run installer
cd test/mantisdb-windows-amd64
.\install.ps1 -Verbose

# Verify installation
mantisdb --version
```

**macOS:**
```bash
# Mount DMG
hdiutil attach dist/MantisDB-1.0.0-macOS-universal.dmg

# Test installation
/Volumes/MantisDB/Install.command

# Verify
mantisdb --version
```

**Linux:**
```bash
# Test DEB package
sudo dpkg -i dist/mantisdb_1.0.0_amd64.deb
systemctl status mantisdb

# Test RPM package
sudo rpm -i dist/mantisdb-1.0.0-1.x86_64.rpm
systemctl status mantisdb
```

## Troubleshooting

### Build Failures

**Issue: Go build fails**
```bash
# Solution: Update Go version
go version  # Should be 1.21+
go mod tidy
go mod download
```

**Issue: Frontend build fails**
```bash
# Solution: Clean and rebuild
cd admin/frontend
rm -rf node_modules package-lock.json
npm install
npm run build
```

**Issue: Cross-compilation fails**
```bash
# Solution: Install required tools
# macOS
xcode-select --install

# Linux
sudo apt-get install build-essential

# Windows
# Install MinGW-w64
```

### Installer Creation Failures

**Issue: NSIS not found (Windows)**
```bash
# Solution: Install NSIS
# Download from https://nsis.sourceforge.io/
# Or use Chocolatey
choco install nsis
```

**Issue: DMG creation fails (macOS)**
```bash
# Solution: Check permissions
sudo hdiutil create -volname "MantisDB" -srcfolder dist/macos -ov -format UDZO dist/MantisDB.dmg
```

**Issue: FPM not found (Linux)**
```bash
# Solution: Install FPM
gem install fpm

# Or use system package
sudo apt-get install ruby-dev build-essential
gem install fpm
```

### Installation Issues

**Windows:**
- Run installers as Administrator
- Disable antivirus temporarily
- Check Windows SmartScreen settings

**macOS:**
- Allow unsigned applications in Security preferences
- Grant Full Disk Access if needed
- Check Gatekeeper settings

**Linux:**
- Check package dependencies: `dpkg -I mantisdb.deb`
- Verify systemd is available
- Check SELinux/AppArmor policies

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Build Installers

on:
  push:
    tags:
      - 'v*'

jobs:
  build:
    runs-on: ${{ matrix.os }}
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
    
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Build
        run: ./scripts/build.sh --cross-platform --version=${{ github.ref_name }}
      
      - name: Create Installers
        run: ./scripts/create-installers.sh --version=${{ github.ref_name }} --platform=all
      
      - name: Upload Artifacts
        uses: actions/upload-artifact@v3
        with:
          name: installers-${{ matrix.os }}
          path: dist/
```

## Contributing

### Adding New Platforms

1. Add platform detection to `scripts/build.sh`
2. Create platform-specific installer in `scripts/create-installers.sh`
3. Update documentation
4. Test on target platform
5. Submit pull request

### Improving Installers

1. Modify the appropriate script in `scripts/`
2. Test on target platform
3. Update documentation
4. Submit pull request

## Best Practices

1. **Version Consistency**: Use semantic versioning (e.g., 1.0.0)
2. **Testing**: Test installers on clean systems
3. **Documentation**: Update docs for any changes
4. **Signing**: Sign installers for production releases
5. **Checksums**: Generate and publish checksums
6. **Release Notes**: Include detailed release notes

## Default Locations Reference

### Windows
- Binary: `C:\Program Files\MantisDB\mantisdb.exe`
- Config: `%PROGRAMDATA%\MantisDB\config.yaml`
- Data: `%LOCALAPPDATA%\MantisDB\data`
- Logs: `%LOCALAPPDATA%\MantisDB\logs`

### macOS
- Binary: `/Applications/MantisDB.app/Contents/MacOS/mantisdb`
- Config: `/usr/local/etc/mantisdb/config.yaml`
- Data: `/usr/local/var/lib/mantisdb`
- Logs: `/usr/local/var/log/mantisdb`

### Linux
- Binary: `/usr/bin/mantisdb`
- Config: `/etc/mantisdb/config.yaml`
- Data: `/var/lib/mantisdb`
- Logs: `/var/log/mantisdb`

## Resources

- [NSIS Documentation](https://nsis.sourceforge.io/Docs/)
- [FPM Documentation](https://fpm.readthedocs.io/)
- [Homebrew Formula Cookbook](https://docs.brew.sh/Formula-Cookbook)
- [Debian Packaging Guide](https://www.debian.org/doc/manuals/maint-guide/)
- [RPM Packaging Guide](https://rpm-packaging-guide.github.io/)

---

For installation instructions, see the [Installation Guide](../getting-started/installation.md).
