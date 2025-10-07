# MantisDB Quick Start Guide

## For Developers

### Build Commands

```bash
# Build for current platform
make build

# Build for all platforms (Linux, macOS, Windows)
make cross-platform

# Create installers
make installers VERSION=1.0.0

# Full production build
make production VERSION=1.0.0

# Run locally
make run
```

### Development

```bash
# Install dependencies
make deps

# Run tests
make test

# Run with hot reload
make run-dev

# Format code
make fmt

# Lint code
make lint

# Clean build artifacts
make clean
```

## For End Users

### Quick Install

**Linux/macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/mantisdb/mantisdb/main/scripts/install.sh | bash
```

**Windows (PowerShell as Admin):**
```powershell
.\install.ps1
```

### Package Managers

**macOS (Homebrew):**
```bash
brew tap mantisdb/tap
brew install mantisdb
```

**Ubuntu/Debian:**
```bash
sudo dpkg -i mantisdb_1.0.0_amd64.deb
```

**RHEL/CentOS:**
```bash
sudo rpm -i mantisdb-1.0.0-1.x86_64.rpm
```

## Running MantisDB

### Start the Server

```bash
# Default (port 8080 for DB, 8081 for admin)
mantisdb

# With custom config
mantisdb --config=/path/to/config.yaml

# With specific ports
mantisdb --port=8080 --admin-port=8081
```

### As a Service

**Linux (systemd):**
```bash
sudo systemctl start mantisdb
sudo systemctl enable mantisdb
sudo systemctl status mantisdb
```

**macOS (Homebrew):**
```bash
brew services start mantisdb
brew services stop mantisdb
```

**Windows:**
```powershell
Start-Service MantisDB
Stop-Service MantisDB
Get-Service MantisDB
```

### Access Admin Dashboard

Open your browser to:
```
http://localhost:8081
```

## Configuration

### Default Locations

**Linux:**
- Config: `/etc/mantisdb/config.yaml`
- Data: `/var/lib/mantisdb`
- Logs: `/var/log/mantisdb`

**macOS (Homebrew):**
- Config: `/usr/local/etc/mantisdb/config.yaml`
- Data: `/usr/local/var/lib/mantisdb`
- Logs: `/usr/local/var/log/mantisdb`

**Windows:**
- Config: `C:\ProgramData\MantisDB\config\config.yaml`
- Data: `C:\ProgramData\MantisDB\data`
- Logs: `C:\ProgramData\MantisDB\logs`

### Basic Configuration

```yaml
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
```

## Troubleshooting

### Check Version
```bash
mantisdb --version
```

### Validate Config
```bash
mantisdb --config=/path/to/config.yaml --validate
```

### View Logs

**Linux:**
```bash
sudo journalctl -u mantisdb -f
tail -f /var/log/mantisdb/mantisdb.log
```

**macOS:**
```bash
tail -f /usr/local/var/log/mantisdb/mantisdb.log
```

**Windows:**
```powershell
Get-EventLog -LogName Application -Source MantisDB -Newest 50
```

### Port Already in Use

```bash
# Check what's using the port
lsof -i :8080
lsof -i :8081

# Or use different ports
mantisdb --port=8082 --admin-port=8083
```

## Documentation

- **Full Installation Guide**: [INSTALL.md](INSTALL.md)
- **Build Guide**: [BUILD.md](BUILD.md)
- **Installer System**: [INSTALLER_SUMMARY.md](INSTALLER_SUMMARY.md)
- **API Documentation**: https://mantisdb.com/docs

## Support

- **GitHub Issues**: https://github.com/mantisdb/mantisdb/issues
- **Discussions**: https://github.com/mantisdb/mantisdb/discussions
- **Website**: https://mantisdb.com

## Common Commands Cheat Sheet

| Task | Command |
|------|---------|
| Build | `make build` |
| Build all platforms | `make cross-platform` |
| Create installers | `make installers VERSION=1.0.0` |
| Run locally | `make run` |
| Run tests | `make test` |
| Clean | `make clean` |
| Install locally | `make install` |
| Start service (Linux) | `sudo systemctl start mantisdb` |
| Start service (macOS) | `brew services start mantisdb` |
| Start service (Windows) | `Start-Service MantisDB` |
| View logs (Linux) | `sudo journalctl -u mantisdb -f` |
| Check status | `mantisdb --version` |
| Validate config | `mantisdb --validate` |

---

**Need help?** Check the full documentation or open an issue on GitHub!
