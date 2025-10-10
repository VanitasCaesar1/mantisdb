# MantisDB Installation Guide

Complete installation instructions for all platforms.

## Table of Contents

- [Quick Install](#quick-install)
- [Platform-Specific Installation](#platform-specific-installation)
  - [Windows](#windows)
  - [macOS](#macos)
  - [Linux](#linux)
- [Building from Source](#building-from-source)
- [Configuration](#configuration)
- [Verification](#verification)
- [Troubleshooting](#troubleshooting)

---

## Quick Install

### Windows

**Option 1: MSI Installer (Recommended)**
```powershell
# Download and run the MSI installer
# This will install MantisDB to C:\Program Files\MantisDB
# and add it to your PATH
```

**Option 2: PowerShell Script**
```powershell
# Run as Administrator
.\install.ps1
```

**Option 3: Manual Installation**
```powershell
# Extract the ZIP file
# Copy mantisdb.exe to a directory in your PATH
# Or add the directory to PATH
```

### macOS

**Option 1: Homebrew (Recommended)**
```bash
brew tap mantisdb/tap
brew install mantisdb
```

**Option 2: DMG Installer**
```bash
# Download the .dmg file
# Double-click to mount
# Drag MantisDB.app to Applications
# Or run Install.command for CLI installation
```

**Option 3: Direct Binary**
```bash
# Download the binary for your architecture
curl -L https://github.com/mantisdb/mantisdb/releases/latest/download/mantisdb-darwin-$(uname -m).tar.gz | tar xz
sudo mv mantisdb /usr/local/bin/
```

### Linux

**Option 1: Package Manager**

**Ubuntu/Debian:**
```bash
wget https://github.com/mantisdb/mantisdb/releases/latest/download/mantisdb_VERSION_amd64.deb
sudo dpkg -i mantisdb_VERSION_amd64.deb
```

**RHEL/CentOS/Fedora:**
```bash
wget https://github.com/mantisdb/mantisdb/releases/latest/download/mantisdb-VERSION-1.x86_64.rpm
sudo rpm -i mantisdb-VERSION-1.x86_64.rpm
```

**Option 2: Install Script**
```bash
curl -fsSL https://raw.githubusercontent.com/mantisdb/mantisdb/main/scripts/install.sh | bash
```

**Option 3: Manual Installation**
```bash
# Download and extract
wget https://github.com/mantisdb/mantisdb/releases/latest/download/mantisdb-linux-amd64.tar.gz
tar -xzf mantisdb-linux-amd64.tar.gz
sudo mv mantisdb /usr/local/bin/
```

---

## Platform-Specific Installation

### Windows

#### System Requirements
- Windows 10 or later (64-bit)
- 2GB RAM minimum (4GB recommended)
- 500MB disk space

#### Installation Steps

1. **Download the Installer**
   - Visit [releases page](https://github.com/mantisdb/mantisdb/releases)
   - Download `MantisDB-VERSION-Windows-amd64-Installer.zip`

2. **Run the Installer**
   ```powershell
   # Extract the ZIP file
   Expand-Archive MantisDB-VERSION-Windows-amd64-Installer.zip
   
   # Run as Administrator
   cd MantisDB-VERSION-Windows-amd64-Installer
   .\install.ps1
   ```

3. **Verify Installation**
   ```powershell
   mantisdb --version
   ```

#### Default Locations
- **Binary**: `C:\Program Files\MantisDB\mantisdb.exe`
- **Config**: `C:\ProgramData\MantisDB\config.yaml`
- **Data**: `C:\ProgramData\MantisDB\data`
- **Logs**: `C:\ProgramData\MantisDB\logs`

#### Running as a Service

**Using NSSM (Non-Sucking Service Manager):**
```powershell
# Install NSSM
choco install nssm

# Create service
nssm install MantisDB "C:\Program Files\MantisDB\mantisdb.exe" --config="C:\ProgramData\MantisDB\config.yaml"

# Start service
nssm start MantisDB
```

**Using sc.exe:**
```powershell
sc.exe create MantisDB binPath= "C:\Program Files\MantisDB\mantisdb.exe --config=C:\ProgramData\MantisDB\config.yaml" start= auto
sc.exe start MantisDB
```

---

### macOS

#### System Requirements
- macOS 10.15 (Catalina) or later
- Intel or Apple Silicon
- 2GB RAM minimum (4GB recommended)
- 500MB disk space

#### Installation Steps

**Method 1: Homebrew (Recommended)**

```bash
# Add the MantisDB tap
brew tap mantisdb/tap

# Install MantisDB
brew install mantisdb

# Start as a service
brew services start mantisdb

# Or run manually
mantisdb
```

**Method 2: DMG Installer**

1. Download `MantisDB-VERSION-macOS-universal.dmg`
2. Double-click to mount the DMG
3. Drag `MantisDB.app` to Applications folder
4. Or run `Install.command` for CLI installation

**Method 3: Direct Binary**

```bash
# For Intel Macs
curl -L https://github.com/mantisdb/mantisdb/releases/latest/download/mantisdb-darwin-amd64.tar.gz | tar xz
sudo mv mantisdb /usr/local/bin/

# For Apple Silicon Macs
curl -L https://github.com/mantisdb/mantisdb/releases/latest/download/mantisdb-darwin-arm64.tar.gz | tar xz
sudo mv mantisdb /usr/local/bin/
```

#### Default Locations
- **Binary**: `/usr/local/bin/mantisdb` (Homebrew) or `/Applications/MantisDB.app` (DMG)
- **Config**: `/usr/local/etc/mantisdb/config.yaml` (Homebrew) or `~/.mantisdb/config.yaml`
- **Data**: `/usr/local/var/lib/mantisdb` (Homebrew) or `~/.mantisdb/data`
- **Logs**: `/usr/local/var/log/mantisdb` (Homebrew) or `~/.mantisdb/logs`

#### Running as a Service

**Using Homebrew:**
```bash
brew services start mantisdb
brew services stop mantisdb
brew services restart mantisdb
```

**Using launchd:**
```bash
# Create plist file
sudo tee /Library/LaunchDaemons/com.mantisdb.mantisdb.plist << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.mantisdb.mantisdb</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/mantisdb</string>
        <string>--config=/usr/local/etc/mantisdb/config.yaml</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/usr/local/var/log/mantisdb/mantisdb.log</string>
    <key>StandardErrorPath</key>
    <string>/usr/local/var/log/mantisdb/mantisdb.log</string>
</dict>
</plist>
EOF

# Load and start
sudo launchctl load /Library/LaunchDaemons/com.mantisdb.mantisdb.plist
```

---

### Linux

#### System Requirements
- Linux kernel 3.10 or later
- 2GB RAM minimum (4GB recommended)
- 500MB disk space
- systemd (for service management)

#### Installation Steps

**Method 1: DEB Package (Ubuntu/Debian)**

```bash
# Download the package
wget https://github.com/mantisdb/mantisdb/releases/latest/download/mantisdb_VERSION_amd64.deb

# Install
sudo dpkg -i mantisdb_VERSION_amd64.deb

# Start service
sudo systemctl start mantisdb
sudo systemctl enable mantisdb
```

**Method 2: RPM Package (RHEL/CentOS/Fedora)**

```bash
# Download the package
wget https://github.com/mantisdb/mantisdb/releases/latest/download/mantisdb-VERSION-1.x86_64.rpm

# Install
sudo rpm -i mantisdb-VERSION-1.x86_64.rpm

# Start service
sudo systemctl start mantisdb
sudo systemctl enable mantisdb
```

**Method 3: Install Script**

```bash
# Download and run install script
curl -fsSL https://raw.githubusercontent.com/mantisdb/mantisdb/main/scripts/install.sh | sudo bash

# Or download first and inspect
curl -fsSL https://raw.githubusercontent.com/mantisdb/mantisdb/main/scripts/install.sh -o install.sh
chmod +x install.sh
sudo ./install.sh
```

**Method 4: Manual Installation**

```bash
# Download binary
wget https://github.com/mantisdb/mantisdb/releases/latest/download/mantisdb-linux-amd64.tar.gz

# Extract
tar -xzf mantisdb-linux-amd64.tar.gz

# Install
sudo mv mantisdb /usr/local/bin/
sudo chmod +x /usr/local/bin/mantisdb

# Create directories
sudo mkdir -p /etc/mantisdb /var/lib/mantisdb /var/log/mantisdb

# Create config
sudo tee /etc/mantisdb/config.yaml << EOF
server:
  port: 8080
  admin_port: 8081
  host: "0.0.0.0"

storage:
  data_dir: "/var/lib/mantisdb"
  engine: "auto"

logging:
  level: "info"
  file: "/var/log/mantisdb/mantisdb.log"

cache:
  size: 268435456  # 256MB
EOF

# Create systemd service
sudo tee /etc/systemd/system/mantisdb.service << EOF
[Unit]
Description=MantisDB Multi-Model Database
After=network.target

[Service]
Type=simple
User=mantisdb
Group=mantisdb
ExecStart=/usr/local/bin/mantisdb --config=/etc/mantisdb/config.yaml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

# Create user
sudo useradd --system --home /var/lib/mantisdb --shell /bin/false mantisdb

# Set permissions
sudo chown -R mantisdb:mantisdb /var/lib/mantisdb /var/log/mantisdb

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable mantisdb
sudo systemctl start mantisdb
```

#### Default Locations
- **Binary**: `/usr/bin/mantisdb` or `/usr/local/bin/mantisdb`
- **Config**: `/etc/mantisdb/config.yaml`
- **Data**: `/var/lib/mantisdb`
- **Logs**: `/var/log/mantisdb/mantisdb.log`

#### Service Management

```bash
# Start service
sudo systemctl start mantisdb

# Stop service
sudo systemctl stop mantisdb

# Restart service
sudo systemctl restart mantisdb

# Check status
sudo systemctl status mantisdb

# View logs
sudo journalctl -u mantisdb -f

# Enable auto-start on boot
sudo systemctl enable mantisdb

# Disable auto-start
sudo systemctl disable mantisdb
```

---

## Building from Source

### Prerequisites

- **Go**: 1.21 or later
- **Node.js**: 18 or later
- **npm**: 9 or later
- **Git**: 2.0 or later
- **Make**: GNU Make 4.0 or later

### Build Steps

```bash
# Clone the repository
git clone https://github.com/mantisdb/mantisdb.git
cd mantisdb

# Install dependencies
make deps

# Build for current platform
make build

# Or build for all platforms
make cross-platform

# Create installers
make installers

# Full production build
make production
```

### Build Options

```bash
# Build with specific version
make build VERSION=1.2.3

# Build without frontend
make build SKIP_FRONTEND=true

# Build with CGO enabled
CGO_ENABLED=1 make build

# Build with custom flags
make build LDFLAGS="-X main.CustomFlag=value"
```

---

## Configuration

### Configuration File

MantisDB uses YAML for configuration. Default locations:

- **Linux**: `/etc/mantisdb/config.yaml`
- **macOS**: `/usr/local/etc/mantisdb/config.yaml` or `~/.mantisdb/config.yaml`
- **Windows**: `C:\ProgramData\MantisDB\config.yaml`

### Example Configuration

```yaml
# Server settings
server:
  port: 8080              # Main database port
  admin_port: 8081        # Admin dashboard port
  host: "0.0.0.0"         # Bind address (use 127.0.0.1 for local only)
  
  # TLS configuration (optional)
  tls:
    enabled: false
    cert_file: "/path/to/cert.pem"
    key_file: "/path/to/key.pem"

# Storage settings
storage:
  data_dir: "/var/lib/mantisdb"
  engine: "auto"          # auto, cgo, pure
  sync_writes: true       # Ensure durability
  
  # Backup configuration
  backup:
    enabled: true
    interval: "1h"
    retention: "168h"     # 7 days

# Logging settings
logging:
  level: "info"           # debug, info, warn, error
  format: "json"          # json, text
  file: "/var/log/mantisdb/mantisdb.log"
  max_size: 100           # MB
  max_backups: 10
  max_age: 30             # days

# Cache settings
cache:
  size: 268435456         # 256MB in bytes
  eviction_policy: "lru"  # lru, lfu

# Security settings
security:
  admin_token: ""         # Set for production
  enable_cors: false
  cors_origins:
    - "http://localhost:3000"
  
  # Authentication
  auth:
    enabled: false
    jwt_secret: "change-this-in-production"
    token_expiry: "24h"

# Performance tuning
performance:
  max_connections: 1000
  read_timeout: "30s"
  write_timeout: "30s"
  idle_timeout: "120s"
```

### Environment Variables

You can override configuration with environment variables:

```bash
# Server settings
export MANTISDB_PORT=8080
export MANTISDB_ADMIN_PORT=8081
export MANTISDB_HOST=0.0.0.0

# Storage settings
export MANTISDB_DATA_DIR=/var/lib/mantisdb
export MANTISDB_ENGINE=auto

# Logging
export MANTISDB_LOG_LEVEL=info
export MANTISDB_LOG_FILE=/var/log/mantisdb/mantisdb.log
```

---

## Verification

### Check Installation

```bash
# Check version
mantisdb --version

# Validate configuration
mantisdb --config=/path/to/config.yaml --validate

# Test connection
mantisdb --test-connection
```

### Access Admin Dashboard

After starting MantisDB, access the admin dashboard:

```
http://localhost:8081
```

Default credentials (if authentication is enabled):
- **Username**: admin
- **Password**: (set in config or first-run setup)

### Health Check

```bash
# Check health endpoint
curl http://localhost:8081/health

# Expected response
{"status":"ok","version":"1.0.0","uptime":"1h23m45s"}
```

---

## Troubleshooting

### Common Issues

#### Port Already in Use

```bash
# Check what's using the port
# Linux/macOS
sudo lsof -i :8080
sudo lsof -i :8081

# Windows
netstat -ano | findstr :8080
netstat -ano | findstr :8081

# Solution: Change ports in config or stop conflicting service
```

#### Permission Denied

```bash
# Linux/macOS: Fix data directory permissions
sudo chown -R mantisdb:mantisdb /var/lib/mantisdb
sudo chown -R mantisdb:mantisdb /var/log/mantisdb

# Or run as current user
mantisdb --config=~/.mantisdb/config.yaml
```

#### Service Won't Start

```bash
# Check service status
sudo systemctl status mantisdb

# View logs
sudo journalctl -u mantisdb -n 50

# Check config syntax
mantisdb --config=/etc/mantisdb/config.yaml --validate

# Try running manually
sudo -u mantisdb mantisdb --config=/etc/mantisdb/config.yaml
```

#### Connection Refused

```bash
# Check if MantisDB is running
ps aux | grep mantisdb

# Check firewall
# Linux
sudo ufw status
sudo ufw allow 8080
sudo ufw allow 8081

# macOS
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --add /usr/local/bin/mantisdb

# Windows
netsh advfirewall firewall add rule name="MantisDB" dir=in action=allow protocol=TCP localport=8080
netsh advfirewall firewall add rule name="MantisDB Admin" dir=in action=allow protocol=TCP localport=8081
```

### Getting Help

- **Documentation**: https://mantisdb.com/docs
- **GitHub Issues**: https://github.com/mantisdb/mantisdb/issues
- **Community Forum**: https://github.com/mantisdb/mantisdb/discussions
- **Email Support**: support@mantisdb.com

### Uninstallation

**Windows:**
```powershell
# Using installer
.\uninstall.bat

# Manual
Remove-Item "C:\Program Files\MantisDB" -Recurse
Remove-Item "C:\ProgramData\MantisDB" -Recurse
```

**macOS:**
```bash
# Homebrew
brew services stop mantisdb
brew uninstall mantisdb
brew untap mantisdb/tap

# Manual
sudo rm /usr/local/bin/mantisdb
rm -rf ~/.mantisdb
```

**Linux:**
```bash
# DEB
sudo systemctl stop mantisdb
sudo systemctl disable mantisdb
sudo dpkg -r mantisdb

# RPM
sudo systemctl stop mantisdb
sudo systemctl disable mantisdb
sudo rpm -e mantisdb

# Manual
sudo systemctl stop mantisdb
sudo systemctl disable mantisdb
sudo rm /usr/local/bin/mantisdb
sudo rm -rf /etc/mantisdb /var/lib/mantisdb /var/log/mantisdb
```

---

## Next Steps

After installation:

1. **Configure MantisDB** for your use case
2. **Start the service** or run manually
3. **Access the admin dashboard** at http://localhost:8081
4. **Install client libraries** for your programming language
5. **Read the documentation** at https://mantisdb.com/docs

For production deployments, see the [Production Deployment Guide](PRODUCTION.md).
