#!/bin/bash

# MantisDB macOS DMG Creator
# Creates a professional macOS disk image installer

set -e

# Configuration
VERSION=${VERSION:-"1.0.0"}
APP_NAME="MantisDB"
COMPANY="MantisDB Team"
DESCRIPTION="Multi-Model Database with Admin Dashboard"
WEBSITE="https://mantisdb.com"

# Directories
BUILD_DIR="dist"
DMG_DIR="$BUILD_DIR/installers/macos"
TEMP_DIR="/tmp/mantisdb-dmg-$$"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

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

# Check if we're on macOS
if [[ "$OSTYPE" != "darwin"* ]]; then
    log_error "This script must be run on macOS to create DMG files"
    exit 1
fi

# Check for required tools
check_dependencies() {
    local missing_tools=()
    
    if ! command -v hdiutil &> /dev/null; then
        missing_tools+=("hdiutil")
    fi
    
    if ! command -v SetFile &> /dev/null; then
        log_warning "SetFile not found - DMG customization will be limited"
    fi
    
    if [ ${#missing_tools[@]} -ne 0 ]; then
        log_error "Missing required tools: ${missing_tools[*]}"
        exit 1
    fi
}

# Create DMG for specific architecture
create_dmg() {
    local arch=$1
    local binary_path="$BUILD_DIR/mantisdb-darwin-$arch"
    
    if [ ! -f "$binary_path" ]; then
        log_warning "macOS binary not found for $arch: $binary_path"
        return 1
    fi
    
    log_info "Creating DMG installer for macOS $arch..."
    
    # Clean up any existing temp directory
    rm -rf "$TEMP_DIR"
    mkdir -p "$TEMP_DIR"
    
    local app_dir="$TEMP_DIR/$APP_NAME.app"
    local dmg_name="$APP_NAME-$VERSION-macOS-$arch.dmg"
    local dmg_path="$DMG_DIR/$dmg_name"
    
    # Create app bundle structure
    mkdir -p "$app_dir/Contents/MacOS"
    mkdir -p "$app_dir/Contents/Resources"
    
    # Copy binary
    cp "$binary_path" "$app_dir/Contents/MacOS/mantisdb-bin"
    chmod +x "$app_dir/Contents/MacOS/mantisdb-bin"
    
    # Create launcher script for GUI
    cat > "$app_dir/Contents/MacOS/mantisdb" << 'LAUNCHER'
#!/bin/bash
# MantisDB Launcher Script

# Get the directory where this script is located
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Set up environment
export MANTISDB_HOME="$HOME/.mantisdb"
mkdir -p "$MANTISDB_HOME"/{data,logs}

# Create default config if it doesn't exist
if [ ! -f "$MANTISDB_HOME/config.yaml" ]; then
    cat > "$MANTISDB_HOME/config.yaml" << 'EOF'
server:
  port: 8080
  admin_port: 8081
  host: 127.0.0.1

storage:
  data_dir: ~/.mantisdb/data
  engine: auto

logging:
  level: info
  file: ~/.mantisdb/logs/mantisdb.log

cache:
  size: 268435456
EOF
fi

# Launch MantisDB
cd "$MANTISDB_HOME"
"$DIR/mantisdb-bin" --config="$MANTISDB_HOME/config.yaml" "$@" &

# Open admin dashboard in default browser after a short delay
sleep 2
open "http://localhost:8081" 2>/dev/null || true
LAUNCHER
    chmod +x "$app_dir/Contents/MacOS/mantisdb"
    
    # Also create a symlink for CLI access
    ln -sf "$app_dir/Contents/MacOS/mantisdb-bin" "$app_dir/Contents/MacOS/mantisdb-cli"
    
    # Create Info.plist
    cat > "$app_dir/Contents/Info.plist" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>mantisdb</string>
    <key>CFBundleIdentifier</key>
    <string>com.mantisdb.mantisdb</string>
    <key>CFBundleName</key>
    <string>$APP_NAME</string>
    <key>CFBundleDisplayName</key>
    <string>$APP_NAME</string>
    <key>CFBundleVersion</key>
    <string>$VERSION</string>
    <key>CFBundleShortVersionString</key>
    <string>$VERSION</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleSignature</key>
    <string>MTDB</string>
    <key>LSMinimumSystemVersion</key>
    <string>10.15</string>
    <key>NSHighResolutionCapable</key>
    <true/>
    <key>LSApplicationCategoryType</key>
    <string>public.app-category.developer-tools</string>
    <key>NSHumanReadableCopyright</key>
    <string>Copyright © 2024 $COMPANY. All rights reserved.</string>
</dict>
</plist>
EOF
    
    # Create a simple app icon (text-based)
    if command -v sips &> /dev/null; then
        # Create a simple 512x512 icon
        cat > "$TEMP_DIR/create_icon.py" << 'EOF'
#!/usr/bin/env python3
from PIL import Image, ImageDraw, ImageFont
import sys

# Create a 512x512 image with MantisDB logo
img = Image.new('RGBA', (512, 512), (22, 163, 74, 255))  # Mantis green
draw = ImageDraw.Draw(img)

# Draw a simple database icon
# Outer ellipse (top)
draw.ellipse([100, 150, 412, 200], fill=(255, 255, 255, 255), outline=(0, 0, 0, 255), width=3)
# Middle section
draw.rectangle([100, 175, 412, 300], fill=(255, 255, 255, 255))
draw.line([100, 175, 100, 300], fill=(0, 0, 0, 255), width=3)
draw.line([412, 175, 412, 300], fill=(0, 0, 0, 255), width=3)
# Bottom ellipse
draw.ellipse([100, 275, 412, 325], fill=(255, 255, 255, 255), outline=(0, 0, 0, 255), width=3)

# Add "M" for MantisDB
try:
    font = ImageFont.truetype("/System/Library/Fonts/Helvetica.ttc", 120)
except:
    font = ImageFont.load_default()
    
draw.text((230, 200), "M", fill=(22, 163, 74, 255), font=font, anchor="mm")

# Save as PNG
img.save(sys.argv[1])
EOF
        
        if command -v python3 &> /dev/null && python3 -c "import PIL" 2>/dev/null; then
            python3 "$TEMP_DIR/create_icon.py" "$TEMP_DIR/icon.png"
            # Convert to icns
            mkdir -p "$TEMP_DIR/icon.iconset"
            sips -z 512 512 "$TEMP_DIR/icon.png" --out "$TEMP_DIR/icon.iconset/icon_512x512.png"
            sips -z 256 256 "$TEMP_DIR/icon.png" --out "$TEMP_DIR/icon.iconset/icon_256x256.png"
            sips -z 128 128 "$TEMP_DIR/icon.png" --out "$TEMP_DIR/icon.iconset/icon_128x128.png"
            sips -z 64 64 "$TEMP_DIR/icon.png" --out "$TEMP_DIR/icon.iconset/icon_64x64.png"
            sips -z 32 32 "$TEMP_DIR/icon.png" --out "$TEMP_DIR/icon.iconset/icon_32x32.png"
            sips -z 16 16 "$TEMP_DIR/icon.png" --out "$TEMP_DIR/icon.iconset/icon_16x16.png"
            iconutil -c icns "$TEMP_DIR/icon.iconset" -o "$app_dir/Contents/Resources/mantisdb.icns"
        fi
    fi
    
    # Create documentation files
    cat > "$TEMP_DIR/README.txt" << EOF
$APP_NAME $VERSION

$DESCRIPTION

INSTALLATION
============
1. Drag MantisDB.app to your Applications folder
2. Or run the Install.command script for command-line installation

GETTING STARTED
===============
After installation:

GUI Application:
- Open MantisDB.app from Applications
- The app will start the database server
- Admin dashboard opens automatically at http://localhost:8081

Command Line:
- Open Terminal
- Run: mantisdb
- Access dashboard at: http://localhost:8081

CONFIGURATION
=============
Default config location: ~/.mantisdb/config.yaml
Data directory: ~/.mantisdb/data
Logs: ~/.mantisdb/logs

To create custom config:
mantisdb --init-config

UNINSTALLATION
==============
- Move MantisDB.app to Trash
- Remove ~/.mantisdb directory if desired
- Remove /usr/local/bin/mantisdb symlink if created

For more information, visit: $WEBSITE
EOF
    
    cp LICENSE "$TEMP_DIR/LICENSE.txt" 2>/dev/null || cat > "$TEMP_DIR/LICENSE.txt" << 'EOF'
MIT License

Copyright (c) 2024 MantisDB Team

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
EOF
    
    # Create install script for command-line installation
    cat > "$TEMP_DIR/Install.command" << 'EOF'
#!/bin/bash
cd "$(dirname "$0")"

echo "=========================================="
echo "MantisDB Command-Line Installer"
echo "=========================================="
echo ""

# Check if running from DMG
if [[ "$PWD" == *"/Volumes/"* ]]; then
    echo "Installing MantisDB..."
    
    # Copy app to Applications
    if [ -w "/Applications" ]; then
        echo "Copying MantisDB.app to /Applications..."
        cp -R "MantisDB.app" "/Applications/"
        echo "✓ MantisDB.app installed to /Applications"
    else
        echo "Cannot write to /Applications, installing to ~/Applications"
        mkdir -p "$HOME/Applications"
        cp -R "MantisDB.app" "$HOME/Applications/"
        echo "✓ MantisDB.app installed to ~/Applications"
    fi
    
    # Create symlink for command line usage (use the binary, not the launcher)
    if [ -w "/usr/local/bin" ]; then
        ln -sf "/Applications/MantisDB.app/Contents/MacOS/mantisdb-bin" "/usr/local/bin/mantisdb"
        echo "✓ Command line tool available as 'mantisdb'"
    elif [ -w "$HOME/.local/bin" ]; then
        mkdir -p "$HOME/.local/bin"
        ln -sf "/Applications/MantisDB.app/Contents/MacOS/mantisdb-bin" "$HOME/.local/bin/mantisdb"
        echo "✓ Command line tool installed to ~/.local/bin/mantisdb"
        echo "  Add ~/.local/bin to your PATH to use 'mantisdb' command"
    else
        echo "⚠ Cannot create command line symlink"
        echo "  Run: sudo ln -s /Applications/MantisDB.app/Contents/MacOS/mantisdb-bin /usr/local/bin/mantisdb"
    fi
    
    # Create default config directory
    mkdir -p "$HOME/.mantisdb"
    
    if [ ! -f "$HOME/.mantisdb/config.yaml" ]; then
        cat > "$HOME/.mantisdb/config.yaml" << 'EOFCONFIG'
# MantisDB Configuration
server:
  port: 8080
  admin_port: 8081
  host: 127.0.0.1

storage:
  data_dir: ~/.mantisdb/data
  engine: auto

logging:
  level: info
  file: ~/.mantisdb/logs/mantisdb.log

cache:
  size: 268435456  # 256MB
EOFCONFIG
        echo "✓ Default configuration created at ~/.mantisdb/config.yaml"
    fi
    
    echo ""
    echo "Installation complete!"
    echo ""
    echo "To start MantisDB:"
    echo "  - Open MantisDB.app from Applications"
    echo "  - Or run 'mantisdb' in Terminal"
    echo ""
    echo "Admin Dashboard: http://localhost:8081"
    
else
    echo "Please run this installer from the mounted DMG"
fi

echo ""
read -p "Press Enter to close..."
EOF
    chmod +x "$TEMP_DIR/Install.command"
    
    # Create temporary DMG first (before adding symlink)
    local temp_dmg="/tmp/mantisdb-temp-$$.dmg"
    hdiutil create -srcfolder "$TEMP_DIR" -volname "$APP_NAME $VERSION" -fs HFS+ -fsargs "-c c=64,a=16,e=16" -format UDRW "$temp_dmg"
    
    # Mount and customize DMG
    local mount_point="/tmp/mantisdb-mount-$$"
    mkdir -p "$mount_point"
    hdiutil attach "$temp_dmg" -mountpoint "$mount_point" -nobrowse
    
    # Add Applications symlink for drag-and-drop
    ln -s /Applications "$mount_point/Applications"
    
    # Customize DMG appearance (with error handling)
    if command -v osascript &> /dev/null; then
        log_info "Customizing DMG appearance..."
        osascript << EOF || log_warning "DMG customization failed (non-critical)"
tell application "Finder"
    try
        tell disk "$APP_NAME $VERSION"
            open
            set current view of container window to icon view
            set toolbar visible of container window to false
            set statusbar visible of container window to false
            set the bounds of container window to {400, 100, 1000, 500}
            set viewOptions to the icon view options of container window
            set arrangement of viewOptions to not arranged
            set icon size of viewOptions to 128
            
            -- Position items (with error handling for each)
            try
                set position of item "$APP_NAME.app" of container window to {200, 190}
            end try
            try
                set position of item "Applications" of container window to {500, 190}
            end try
            try
                set position of item "Install.command" of container window to {350, 300}
            end try
            try
                set position of item "README.txt" of container window to {150, 300}
            end try
            try
                set position of item "LICENSE.txt" of container window to {450, 300}
            end try
            
            update without registering applications
            delay 1
            close
        end tell
    on error errMsg
        log "DMG customization error: " & errMsg
    end try
end tell
EOF
    fi
    
    # Set custom icon for the DMG volume if we created one
    if [ -f "$app_dir/Contents/Resources/mantisdb.icns" ]; then
        cp "$app_dir/Contents/Resources/mantisdb.icns" "$mount_point/.VolumeIcon.icns"
        SetFile -c icnC "$mount_point/.VolumeIcon.icns" 2>/dev/null || true
        SetFile -a C "$mount_point" 2>/dev/null || true
    fi
    
    # Unmount and convert to read-only
    hdiutil detach "$mount_point"
    
    # Create final DMG
    mkdir -p "$DMG_DIR"
    hdiutil convert "$temp_dmg" -format UDZO -imagekey zlib-level=9 -o "$dmg_path"
    
    # Clean up
    rm "$temp_dmg"
    rm -rf "$TEMP_DIR"
    
    log_success "Created DMG: $dmg_name ($(du -h "$dmg_path" | cut -f1))"
    return 0
}

# Create universal DMG if both architectures exist
create_universal_dmg() {
    if [ -f "$BUILD_DIR/mantisdb-darwin-amd64" ] && [ -f "$BUILD_DIR/mantisdb-darwin-arm64" ]; then
        log_info "Creating universal macOS binary..."
        
        # Create universal binary
        lipo -create -output "$BUILD_DIR/mantisdb-darwin-universal" \
            "$BUILD_DIR/mantisdb-darwin-amd64" \
            "$BUILD_DIR/mantisdb-darwin-arm64"
        
        # Create universal DMG
        create_dmg "universal"
        
        # Clean up universal binary
        rm "$BUILD_DIR/mantisdb-darwin-universal"
    fi
}

# Main execution
main() {
    echo -e "${BLUE}MantisDB macOS DMG Creator${NC}"
    echo -e "${BLUE}==========================${NC}"
    
    check_dependencies
    
    # Create individual architecture DMGs
    local created_any=false
    
    if [ -f "$BUILD_DIR/mantisdb-darwin-amd64" ]; then
        create_dmg "amd64" && created_any=true
    fi
    
    if [ -f "$BUILD_DIR/mantisdb-darwin-arm64" ]; then
        create_dmg "arm64" && created_any=true
    fi
    
    # Create universal DMG if both exist
    create_universal_dmg && created_any=true
    
    if [ "$created_any" = true ]; then
        log_success "macOS DMG creation complete!"
        echo ""
        echo "Created DMGs:"
        find "$DMG_DIR" -name "*.dmg" -exec basename {} \; 2>/dev/null | sort
    else
        log_error "No macOS binaries found to package"
        exit 1
    fi
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --version)
            VERSION="$2"
            shift 2
            ;;
        --version=*)
            VERSION="${1#*=}"
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [--version VERSION]"
            echo "Creates macOS DMG installers for MantisDB"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Run main function
main "$@"