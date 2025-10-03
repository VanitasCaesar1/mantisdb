#!/bin/bash

# MantisDB Production Build Script
# This script builds production-ready binaries for all platforms

set -e

# Configuration
VERSION=${VERSION:-"1.0.0"}
BUILD_DIR="dist"
BINARY_NAME="mantisdb"
PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}MantisDB Production Build${NC}"
echo -e "${BLUE}========================${NC}"
echo "Version: $VERSION"
echo "Build Directory: $BUILD_DIR"
echo ""

# Clean previous builds
echo -e "${YELLOW}Cleaning previous builds...${NC}"
rm -rf $BUILD_DIR
mkdir -p $BUILD_DIR

# Build frontend
echo -e "${YELLOW}Building admin dashboard frontend...${NC}"
cd admin/frontend
if [ ! -d "node_modules" ]; then
    echo "Installing frontend dependencies..."
    npm install
fi
npm run build
cd ../..

# Build for each platform
echo -e "${YELLOW}Building binaries for all platforms...${NC}"
for platform in "${PLATFORMS[@]}"; do
    platform_split=(${platform//\// })
    GOOS=${platform_split[0]}
    GOARCH=${platform_split[1]}
    
    output_name="${BINARY_NAME}-${GOOS}-${GOARCH}"
    if [ $GOOS = "windows" ]; then
        output_name+='.exe'
    fi
    
    echo -e "${GREEN}Building for $GOOS/$GOARCH...${NC}"
    
    # Build with optimizations
    env GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=0 go build \
        -ldflags="-s -w -X main.Version=$VERSION -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ) -X main.GitCommit=$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" \
        -o "$BUILD_DIR/$output_name" \
        cmd/mantisDB/main.go
    
    # Create platform-specific package
    package_dir="$BUILD_DIR/${BINARY_NAME}-${GOOS}-${GOARCH}"
    mkdir -p "$package_dir"
    
    # Copy binary
    cp "$BUILD_DIR/$output_name" "$package_dir/"
    
    # Copy documentation
    cp README.md "$package_dir/"
    cp LICENSE "$package_dir/" 2>/dev/null || echo "# License file not found" > "$package_dir/LICENSE"
    
    # Create platform-specific installer
    case $GOOS in
        "linux"|"darwin")
            create_unix_installer "$package_dir" "$GOOS" "$GOARCH"
            ;;
        "windows")
            create_windows_installer "$package_dir" "$GOARCH"
            ;;
    esac
    
    # Create archive
    cd "$BUILD_DIR"
    if [ $GOOS = "windows" ]; then
        zip -r "${BINARY_NAME}-${GOOS}-${GOARCH}.zip" "${BINARY_NAME}-${GOOS}-${GOARCH}/"
    else
        tar -czf "${BINARY_NAME}-${GOOS}-${GOARCH}.tar.gz" "${BINARY_NAME}-${GOOS}-${GOARCH}/"
    fi
    cd ..
    
    echo -e "${GREEN}✓ Built $output_name${NC}"
done

# Generate checksums
echo -e "${YELLOW}Generating checksums...${NC}"
cd $BUILD_DIR
sha256sum *.tar.gz *.zip > checksums.txt 2>/dev/null || shasum -a 256 *.tar.gz *.zip > checksums.txt
cd ..

echo -e "${GREEN}Build complete!${NC}"
echo "Artifacts available in: $BUILD_DIR/"
echo ""
echo "Files created:"
ls -la $BUILD_DIR/

# Function to create Unix installer
create_unix_installer() {
    local package_dir=$1
    local os=$2
    local arch=$3
    
    cat > "$package_dir/install.sh" << 'EOF'
#!/bin/bash

# MantisDB Installer Script

set -e

INSTALL_DIR="/usr/local/bin"
SERVICE_DIR="/etc/systemd/system"
DATA_DIR="/var/lib/mantisdb"
CONFIG_DIR="/etc/mantisdb"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}MantisDB Installer${NC}"
echo "=================="

# Check if running as root for system installation
if [ "$EUID" -eq 0 ]; then
    echo "Installing system-wide..."
    INSTALL_MODE="system"
else
    echo "Installing for current user..."
    INSTALL_DIR="$HOME/.local/bin"
    SERVICE_DIR="$HOME/.config/systemd/user"
    DATA_DIR="$HOME/.local/share/mantisdb"
    CONFIG_DIR="$HOME/.config/mantisdb"
    INSTALL_MODE="user"
fi

# Create directories
mkdir -p "$INSTALL_DIR"
mkdir -p "$DATA_DIR"
mkdir -p "$CONFIG_DIR"

# Copy binary
echo "Installing MantisDB binary..."
cp mantisdb* "$INSTALL_DIR/mantisdb"
chmod +x "$INSTALL_DIR/mantisdb"

# Create default configuration
echo "Creating default configuration..."
cat > "$CONFIG_DIR/config.yaml" << 'CONFIGEOF'
server:
  port: 8080
  admin_port: 8081
  host: "0.0.0.0"

database:
  data_dir: "/var/lib/mantisdb"
  cache_size: "256MB"
  buffer_size: "64MB"
  use_cgo: false
  sync_writes: true

security:
  admin_token: ""
  enable_cors: false
  cors_origins: ["http://localhost:3000"]

logging:
  level: "info"
  format: "json"
  output: "stdout"
CONFIGEOF

# Update data directory in config for user installation
if [ "$INSTALL_MODE" = "user" ]; then
    sed -i.bak "s|/var/lib/mantisdb|$DATA_DIR|g" "$CONFIG_DIR/config.yaml"
    rm "$CONFIG_DIR/config.yaml.bak"
fi

# Create systemd service (if systemd is available)
if command -v systemctl >/dev/null 2>&1; then
    echo "Creating systemd service..."
    mkdir -p "$SERVICE_DIR"
    
    cat > "$SERVICE_DIR/mantisdb.service" << SERVICEEOF
[Unit]
Description=MantisDB - Multi-Model Database
After=network.target

[Service]
Type=simple
User=$(whoami)
ExecStart=$INSTALL_DIR/mantisdb --config=$CONFIG_DIR/config.yaml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
SERVICEEOF

    if [ "$INSTALL_MODE" = "system" ]; then
        systemctl daemon-reload
        echo "Service created. Enable with: sudo systemctl enable mantisdb"
        echo "Start with: sudo systemctl start mantisdb"
    else
        systemctl --user daemon-reload
        echo "Service created. Enable with: systemctl --user enable mantisdb"
        echo "Start with: systemctl --user start mantisdb"
    fi
fi

echo -e "${GREEN}Installation complete!${NC}"
echo ""
echo "Configuration file: $CONFIG_DIR/config.yaml"
echo "Data directory: $DATA_DIR"
echo ""
echo "To start MantisDB manually:"
echo "  $INSTALL_DIR/mantisdb --config=$CONFIG_DIR/config.yaml"
echo ""
echo "Admin dashboard will be available at: http://localhost:8081"
EOF

    chmod +x "$package_dir/install.sh"
}

# Function to create Windows installer
create_windows_installer() {
    local package_dir=$1
    local arch=$2
    
    cat > "$package_dir/install.bat" << 'EOF'
@echo off
echo MantisDB Windows Installer
echo =========================

set INSTALL_DIR=%PROGRAMFILES%\MantisDB
set DATA_DIR=%PROGRAMDATA%\MantisDB
set CONFIG_DIR=%PROGRAMDATA%\MantisDB

echo Installing MantisDB...

REM Create directories
if not exist "%INSTALL_DIR%" mkdir "%INSTALL_DIR%"
if not exist "%DATA_DIR%" mkdir "%DATA_DIR%"
if not exist "%CONFIG_DIR%" mkdir "%CONFIG_DIR%"

REM Copy binary
copy mantisdb.exe "%INSTALL_DIR%\"

REM Create default configuration
echo server: > "%CONFIG_DIR%\config.yaml"
echo   port: 8080 >> "%CONFIG_DIR%\config.yaml"
echo   admin_port: 8081 >> "%CONFIG_DIR%\config.yaml"
echo   host: "0.0.0.0" >> "%CONFIG_DIR%\config.yaml"
echo. >> "%CONFIG_DIR%\config.yaml"
echo database: >> "%CONFIG_DIR%\config.yaml"
echo   data_dir: "%DATA_DIR%" >> "%CONFIG_DIR%\config.yaml"
echo   cache_size: "256MB" >> "%CONFIG_DIR%\config.yaml"
echo   buffer_size: "64MB" >> "%CONFIG_DIR%\config.yaml"
echo   use_cgo: false >> "%CONFIG_DIR%\config.yaml"
echo   sync_writes: true >> "%CONFIG_DIR%\config.yaml"

REM Add to PATH
setx PATH "%PATH%;%INSTALL_DIR%" /M

echo Installation complete!
echo.
echo Configuration file: %CONFIG_DIR%\config.yaml
echo Data directory: %DATA_DIR%
echo.
echo To start MantisDB:
echo   mantisdb --config="%CONFIG_DIR%\config.yaml"
echo.
echo Admin dashboard will be available at: http://localhost:8081

pause
EOF

    # Create PowerShell installer as well
    cat > "$package_dir/install.ps1" << 'EOF'
# MantisDB PowerShell Installer

Write-Host "MantisDB Windows Installer" -ForegroundColor Green
Write-Host "=========================" -ForegroundColor Green

$InstallDir = "$env:ProgramFiles\MantisDB"
$DataDir = "$env:ProgramData\MantisDB"
$ConfigDir = "$env:ProgramData\MantisDB"

Write-Host "Installing MantisDB..." -ForegroundColor Yellow

# Create directories
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
New-Item -ItemType Directory -Force -Path $DataDir | Out-Null
New-Item -ItemType Directory -Force -Path $ConfigDir | Out-Null

# Copy binary
Copy-Item "mantisdb.exe" -Destination $InstallDir

# Create default configuration
$ConfigContent = @"
server:
  port: 8080
  admin_port: 8081
  host: "0.0.0.0"

database:
  data_dir: "$DataDir"
  cache_size: "256MB"
  buffer_size: "64MB"
  use_cgo: false
  sync_writes: true

security:
  admin_token: ""
  enable_cors: false
  cors_origins: ["http://localhost:3000"]

logging:
  level: "info"
  format: "json"
  output: "stdout"
"@

$ConfigContent | Out-File -FilePath "$ConfigDir\config.yaml" -Encoding UTF8

# Add to PATH
$CurrentPath = [Environment]::GetEnvironmentVariable("PATH", "Machine")
if ($CurrentPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("PATH", "$CurrentPath;$InstallDir", "Machine")
}

Write-Host "Installation complete!" -ForegroundColor Green
Write-Host ""
Write-Host "Configuration file: $ConfigDir\config.yaml"
Write-Host "Data directory: $DataDir"
Write-Host ""
Write-Host "To start MantisDB:"
Write-Host "  mantisdb --config=`"$ConfigDir\config.yaml`""
Write-Host ""
Write-Host "Admin dashboard will be available at: http://localhost:8081"

Read-Host "Press Enter to continue..."
EOF
}

echo -e "${BLUE}Production build script created successfully!${NC}"