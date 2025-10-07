#!/bin/bash

# MantisDB Universal Installer
# Works on Linux and macOS
# Usage: curl -fsSL https://raw.githubusercontent.com/mantisdb/mantisdb/main/scripts/install.sh | bash

set -e

# Configuration
GITHUB_REPO="mantisdb/mantisdb"
BINARY_NAME="mantisdb"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/mantisdb"
DATA_DIR="/var/lib/mantisdb"
LOG_DIR="/var/log/mantisdb"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

# Print banner
print_banner() {
    echo -e "${BLUE}"
    echo "╔══════════════════════════════════════════════════════════════╗"
    echo "║                    MantisDB Installer                        ║"
    echo "║                                                              ║"
    echo "║  Multi-Model Database with Admin Dashboard                  ║"
    echo "╚══════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

# Detect OS and architecture
detect_platform() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)
    
    case "$os" in
        linux*)
            OS="linux"
            ;;
        darwin*)
            OS="darwin"
            ;;
        *)
            log_error "Unsupported operating system: $os"
            exit 1
            ;;
    esac
    
    case "$arch" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        *)
            log_error "Unsupported architecture: $arch"
            exit 1
            ;;
    esac
    
    log_info "Detected platform: $OS/$ARCH"
}

# Check if running as root
check_root() {
    if [ "$EUID" -eq 0 ]; then
        INSTALL_MODE="system"
        log_info "Installing system-wide (running as root)"
    else
        INSTALL_MODE="user"
        log_info "Installing for current user"
        
        # Update paths for user installation
        INSTALL_DIR="$HOME/.local/bin"
        CONFIG_DIR="$HOME/.config/mantisdb"
        DATA_DIR="$HOME/.local/share/mantisdb"
        LOG_DIR="$HOME/.local/share/mantisdb/logs"
    fi
}

# Get latest version from GitHub
get_latest_version() {
    log_info "Fetching latest version..."
    
    if command -v curl &> /dev/null; then
        VERSION=$(curl -fsSL "https://api.github.com/repos/$GITHUB_REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/')
    elif command -v wget &> /dev/null; then
        VERSION=$(wget -qO- "https://api.github.com/repos/$GITHUB_REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/')
    else
        log_error "Neither curl nor wget found. Please install one of them."
        exit 1
    fi
    
    if [ -z "$VERSION" ]; then
        log_warning "Could not fetch latest version, using 'latest'"
        VERSION="latest"
    else
        log_info "Latest version: $VERSION"
    fi
}

# Download binary
download_binary() {
    log_info "Downloading MantisDB binary..."
    
    local download_url
    if [ "$VERSION" = "latest" ]; then
        download_url="https://github.com/$GITHUB_REPO/releases/latest/download/${BINARY_NAME}-${OS}-${ARCH}.tar.gz"
    else
        download_url="https://github.com/$GITHUB_REPO/releases/download/v${VERSION}/${BINARY_NAME}-${OS}-${ARCH}.tar.gz"
    fi
    
    local temp_file="/tmp/${BINARY_NAME}-${OS}-${ARCH}.tar.gz"
    
    if command -v curl &> /dev/null; then
        curl -fsSL "$download_url" -o "$temp_file"
    elif command -v wget &> /dev/null; then
        wget -q "$download_url" -O "$temp_file"
    fi
    
    if [ ! -f "$temp_file" ]; then
        log_error "Failed to download binary"
        exit 1
    fi
    
    log_success "Downloaded binary"
    echo "$temp_file"
}

# Install binary
install_binary() {
    local temp_file=$1
    
    log_info "Installing binary to $INSTALL_DIR..."
    
    # Create install directory
    mkdir -p "$INSTALL_DIR"
    
    # Extract and install
    tar -xzf "$temp_file" -C /tmp
    
    if [ "$INSTALL_MODE" = "system" ]; then
        mv "/tmp/${BINARY_NAME}-${OS}-${ARCH}" "$INSTALL_DIR/$BINARY_NAME"
        chmod +x "$INSTALL_DIR/$BINARY_NAME"
    else
        mv "/tmp/${BINARY_NAME}-${OS}-${ARCH}" "$INSTALL_DIR/$BINARY_NAME"
        chmod +x "$INSTALL_DIR/$BINARY_NAME"
    fi
    
    # Clean up
    rm -f "$temp_file"
    
    log_success "Binary installed to $INSTALL_DIR/$BINARY_NAME"
}

# Create directories
create_directories() {
    log_info "Creating directories..."
    
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$DATA_DIR"
    mkdir -p "$LOG_DIR"
    
    if [ "$INSTALL_MODE" = "system" ] && [ "$OS" = "linux" ]; then
        # Create mantisdb user if it doesn't exist
        if ! id mantisdb &>/dev/null; then
            useradd --system --home "$DATA_DIR" --shell /bin/false mantisdb
            log_info "Created mantisdb user"
        fi
        
        # Set ownership
        chown -R mantisdb:mantisdb "$DATA_DIR" "$LOG_DIR"
    fi
    
    log_success "Directories created"
}

# Create default configuration
create_config() {
    log_info "Creating default configuration..."
    
    local config_file="$CONFIG_DIR/config.yaml"
    
    if [ -f "$config_file" ]; then
        log_warning "Configuration file already exists, skipping"
        return
    fi
    
    cat > "$config_file" << EOF
# MantisDB Configuration
server:
  port: 8080
  admin_port: 8081
  host: "127.0.0.1"

storage:
  data_dir: "$DATA_DIR"
  engine: "auto"
  sync_writes: true

logging:
  level: "info"
  format: "json"
  file: "$LOG_DIR/mantisdb.log"

cache:
  size: 268435456  # 256MB

security:
  admin_token: ""
  enable_cors: false
  cors_origins:
    - "http://localhost:3000"
EOF
    
    log_success "Configuration created at $config_file"
}

# Create systemd service (Linux only)
create_systemd_service() {
    if [ "$OS" != "linux" ]; then
        return
    fi
    
    if [ "$INSTALL_MODE" != "system" ]; then
        log_info "Skipping systemd service creation (user installation)"
        return
    fi
    
    if ! command -v systemctl &> /dev/null; then
        log_warning "systemd not found, skipping service creation"
        return
    fi
    
    log_info "Creating systemd service..."
    
    local service_file="/etc/systemd/system/mantisdb.service"
    
    cat > "$service_file" << EOF
[Unit]
Description=MantisDB Multi-Model Database
Documentation=https://mantisdb.com/docs
After=network.target

[Service]
Type=simple
User=mantisdb
Group=mantisdb
ExecStart=$INSTALL_DIR/$BINARY_NAME --config=$CONFIG_DIR/config.yaml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$DATA_DIR $LOG_DIR

[Install]
WantedBy=multi-user.target
EOF
    
    # Reload systemd
    systemctl daemon-reload
    
    log_success "Systemd service created"
    log_info "Enable with: sudo systemctl enable mantisdb"
    log_info "Start with: sudo systemctl start mantisdb"
}

# Create launchd service (macOS only)
create_launchd_service() {
    if [ "$OS" != "darwin" ]; then
        return
    fi
    
    if [ "$INSTALL_MODE" = "system" ]; then
        local plist_dir="/Library/LaunchDaemons"
        local plist_file="$plist_dir/com.mantisdb.mantisdb.plist"
    else
        local plist_dir="$HOME/Library/LaunchAgents"
        local plist_file="$plist_dir/com.mantisdb.mantisdb.plist"
    fi
    
    log_info "Creating launchd service..."
    
    mkdir -p "$plist_dir"
    
    cat > "$plist_file" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.mantisdb.mantisdb</string>
    <key>ProgramArguments</key>
    <array>
        <string>$INSTALL_DIR/$BINARY_NAME</string>
        <string>--config=$CONFIG_DIR/config.yaml</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>$LOG_DIR/mantisdb.log</string>
    <key>StandardErrorPath</key>
    <string>$LOG_DIR/mantisdb.log</string>
</dict>
</plist>
EOF
    
    log_success "Launchd service created"
    
    if [ "$INSTALL_MODE" = "system" ]; then
        log_info "Load with: sudo launchctl load $plist_file"
    else
        log_info "Load with: launchctl load $plist_file"
    fi
}

# Print post-install instructions
print_instructions() {
    echo ""
    echo -e "${GREEN}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║              Installation Complete!                          ║${NC}"
    echo -e "${GREEN}╚══════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "${BLUE}Installation Details:${NC}"
    echo "  Binary: $INSTALL_DIR/$BINARY_NAME"
    echo "  Config: $CONFIG_DIR/config.yaml"
    echo "  Data: $DATA_DIR"
    echo "  Logs: $LOG_DIR"
    echo ""
    
    if [ "$INSTALL_MODE" = "user" ]; then
        echo -e "${YELLOW}Note: Make sure $INSTALL_DIR is in your PATH${NC}"
        echo "Add to your shell profile:"
        echo "  export PATH=\"\$PATH:$INSTALL_DIR\""
        echo ""
    fi
    
    echo -e "${BLUE}Quick Start:${NC}"
    echo ""
    
    if [ "$OS" = "linux" ] && [ "$INSTALL_MODE" = "system" ]; then
        echo "1. Start as a service:"
        echo "   sudo systemctl enable mantisdb"
        echo "   sudo systemctl start mantisdb"
        echo ""
        echo "2. Or run manually:"
        echo "   mantisdb --config=$CONFIG_DIR/config.yaml"
    elif [ "$OS" = "darwin" ]; then
        echo "1. Start as a service:"
        if [ "$INSTALL_MODE" = "system" ]; then
            echo "   sudo launchctl load /Library/LaunchDaemons/com.mantisdb.mantisdb.plist"
        else
            echo "   launchctl load ~/Library/LaunchAgents/com.mantisdb.mantisdb.plist"
        fi
        echo ""
        echo "2. Or run manually:"
        echo "   mantisdb --config=$CONFIG_DIR/config.yaml"
    else
        echo "Run MantisDB:"
        echo "   mantisdb --config=$CONFIG_DIR/config.yaml"
    fi
    
    echo ""
    echo "3. Access the admin dashboard:"
    echo "   http://localhost:8081"
    echo ""
    echo -e "${BLUE}Documentation:${NC}"
    echo "  https://mantisdb.com/docs"
    echo ""
    echo -e "${BLUE}Support:${NC}"
    echo "  https://github.com/$GITHUB_REPO/issues"
    echo ""
}

# Main installation function
main() {
    print_banner
    
    # Check prerequisites
    detect_platform
    check_root
    
    # Get version and download
    get_latest_version
    local temp_file=$(download_binary)
    
    # Install
    install_binary "$temp_file"
    create_directories
    create_config
    
    # Create service
    if [ "$OS" = "linux" ]; then
        create_systemd_service
    elif [ "$OS" = "darwin" ]; then
        create_launchd_service
    fi
    
    # Print instructions
    print_instructions
}

# Run main function
main "$@"
