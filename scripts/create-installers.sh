#!/bin/bash

# MantisDB Installer Creation Script
# Creates platform-specific installers (DMG, MSI, DEB, RPM, etc.)

set -e

# Configuration
VERSION=${VERSION:-"1.0.0"}
BUILD_DIR="dist"
INSTALLER_DIR="$BUILD_DIR/installers"
BINARY_NAME="mantisdb"
APP_NAME="MantisDB"
COMPANY="MantisDB"
DESCRIPTION="Multi-Model Database with Admin Dashboard"
WEBSITE="https://mantisdb.com"

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

# Check dependencies for installer creation
check_installer_dependencies() {
    log_info "Checking installer creation dependencies..."
    
    local missing_tools=()
    
    # Check for macOS installer tools
    if [[ "$OSTYPE" == "darwin"* ]]; then
        if ! command -v hdiutil &> /dev/null; then
            missing_tools+=("hdiutil (macOS disk utility)")
        fi
        if ! command -v pkgbuild &> /dev/null; then
            missing_tools+=("pkgbuild (macOS package builder)")
        fi
    fi
    
    # Check for Linux installer tools
    if [[ "$OSTYPE" == "linux"* ]]; then
        if ! command -v fpm &> /dev/null; then
            log_warning "fpm not found - will create basic installers only"
            log_info "Install fpm with: gem install fpm"
        fi
    fi
    
    # Check for Windows installer tools (when running on Windows or with Wine)
    if command -v makensis &> /dev/null; then
        log_info "NSIS found - can create Windows installers"
    else
        log_warning "NSIS not found - Windows installers will be basic"
    fi
    
    if [ ${#missing_tools[@]} -ne 0 ]; then
        log_warning "Some installer tools are missing: ${missing_tools[*]}"
        log_info "Basic installers will be created instead"
    fi
}

# Create directory structure for installers
setup_installer_environment() {
    log_info "Setting up installer environment..."
    
    mkdir -p "$INSTALLER_DIR"/{macos,windows,linux}/{pkg,scripts,resources}
    
    # Create common resources
    create_common_resources
    
    log_success "Installer environment ready"
}

# Create common resources (icons, license, etc.)
create_common_resources() {
    log_info "Creating common installer resources..."
    
    # Create a simple license file if it doesn't exist
    if [ ! -f "LICENSE" ]; then
        cat > "$INSTALLER_DIR/LICENSE.txt" << 'EOF'
MIT License

Copyright (c) 2024 MantisDB

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
    else
        cp LICENSE "$INSTALLER_DIR/LICENSE.txt"
    fi
    
    # Create README for installers
    cat > "$INSTALLER_DIR/README.txt" << EOF
$APP_NAME $VERSION

$DESCRIPTION

Installation Instructions:
- macOS: Double-click the .dmg file and drag MantisDB to Applications
- Windows: Run the .msi installer as administrator
- Linux: Install the .deb or .rpm package using your package manager

For more information, visit: $WEBSITE

Configuration:
After installation, MantisDB can be configured by editing:
- Linux/macOS: /etc/mantisdb/config.yaml
- Windows: %PROGRAMDATA%\\MantisDB\\config.yaml

Starting MantisDB:
- Command line: mantisdb --config=/path/to/config.yaml
- Service: systemctl start mantisdb (Linux) or net start mantisdb (Windows)

Admin Dashboard:
Access the web interface at http://localhost:8081 after starting MantisDB.
EOF
}

# Create macOS DMG installer
create_macos_dmg() {
    local arch=$1
    local binary_path="$BUILD_DIR/mantisdb-darwin-$arch"
    
    if [ ! -f "$binary_path" ]; then
        log_warning "macOS binary not found for $arch: $binary_path"
        return 1
    fi
    
    log_info "Creating macOS DMG installer for $arch..."
    
    local dmg_dir="$INSTALLER_DIR/macos/dmg-$arch"
    local app_dir="$dmg_dir/$APP_NAME.app"
    
    # Create app bundle structure
    mkdir -p "$app_dir/Contents/MacOS"
    mkdir -p "$app_dir/Contents/Resources"
    
    # Copy binary
    cp "$binary_path" "$app_dir/Contents/MacOS/$BINARY_NAME"
    chmod +x "$app_dir/Contents/MacOS/$BINARY_NAME"
    
    # Create Info.plist
    cat > "$app_dir/Contents/Info.plist" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>$BINARY_NAME</string>
    <key>CFBundleIdentifier</key>
    <string>com.mantisdb.mantisdb</string>
    <key>CFBundleName</key>
    <string>$APP_NAME</string>
    <key>CFBundleVersion</key>
    <string>$VERSION</string>
    <key>CFBundleShortVersionString</key>
    <string>$VERSION</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>LSMinimumSystemVersion</key>
    <string>10.15</string>
    <key>NSHighResolutionCapable</key>
    <true/>
</dict>
</plist>
EOF
    
    # Copy documentation
    cp "$INSTALLER_DIR/README.txt" "$dmg_dir/"
    cp "$INSTALLER_DIR/LICENSE.txt" "$dmg_dir/"
    
    # Create install script
    cat > "$dmg_dir/Install.command" << 'EOF'
#!/bin/bash
echo "Installing MantisDB..."

# Copy to Applications
if [ -w "/Applications" ]; then
    cp -R "MantisDB.app" "/Applications/"
    echo "✓ Installed to /Applications/MantisDB.app"
else
    echo "⚠ Cannot write to /Applications, installing to ~/Applications"
    mkdir -p "$HOME/Applications"
    cp -R "MantisDB.app" "$HOME/Applications/"
    echo "✓ Installed to ~/Applications/MantisDB.app"
fi

# Create symlink for command line usage
if [ -w "/usr/local/bin" ]; then
    ln -sf "/Applications/MantisDB.app/Contents/MacOS/mantisdb" "/usr/local/bin/mantisdb"
    echo "✓ Command line tool available as 'mantisdb'"
else
    echo "⚠ Cannot create symlink in /usr/local/bin"
    echo "  Add /Applications/MantisDB.app/Contents/MacOS to your PATH to use 'mantisdb' command"
fi

echo ""
echo "Installation complete!"
echo "Start MantisDB from Applications or run 'mantisdb' in terminal"

read -p "Press Enter to close..."
EOF
    chmod +x "$dmg_dir/Install.command"
    
    # Create DMG
    local dmg_name="$APP_NAME-$VERSION-macOS-$arch.dmg"
    local dmg_path="$INSTALLER_DIR/$dmg_name"
    
    if command -v hdiutil &> /dev/null; then
        # Create temporary DMG
        local temp_dmg="/tmp/mantisdb-temp.dmg"
        hdiutil create -srcfolder "$dmg_dir" -volname "$APP_NAME $VERSION" -fs HFS+ -fsargs "-c c=64,a=16,e=16" -format UDRW "$temp_dmg"
        
        # Mount and customize
        local mount_point="/tmp/mantisdb-dmg"
        mkdir -p "$mount_point"
        hdiutil attach "$temp_dmg" -mountpoint "$mount_point"
        
        # Set background and icon positions (if running on macOS)
        if [[ "$OSTYPE" == "darwin"* ]]; then
            osascript << EOF
tell application "Finder"
    tell disk "$APP_NAME $VERSION"
        open
        set current view of container window to icon view
        set toolbar visible of container window to false
        set statusbar visible of container window to false
        set the bounds of container window to {400, 100, 900, 400}
        set viewOptions to the icon view options of container window
        set arrangement of viewOptions to not arranged
        set icon size of viewOptions to 72
        set position of item "$APP_NAME.app" of container window to {150, 150}
        set position of item "Install.command" of container window to {350, 150}
        close
        open
        update without registering applications
        delay 2
    end tell
end tell
EOF
        fi
        
        # Unmount and convert to read-only
        hdiutil detach "$mount_point"
        hdiutil convert "$temp_dmg" -format UDZO -imagekey zlib-level=9 -o "$dmg_path"
        rm "$temp_dmg"
        
        log_success "Created DMG: $dmg_name"
    else
        # Fallback: create tar.gz
        local fallback_name="$APP_NAME-$VERSION-macOS-$arch.tar.gz"
        tar -czf "$INSTALLER_DIR/$fallback_name" -C "$dmg_dir" .
        log_warning "Created fallback archive: $fallback_name (hdiutil not available)"
    fi
}

# Create Windows installer
create_windows_installer() {
    local arch=$1
    local binary_path="$BUILD_DIR/mantisdb-windows-$arch.exe"
    
    if [ ! -f "$binary_path" ]; then
        log_warning "Windows binary not found for $arch: $binary_path"
        return 1
    fi
    
    log_info "Creating Windows installer for $arch..."
    
    local installer_dir="$INSTALLER_DIR/windows/msi-$arch"
    mkdir -p "$installer_dir"
    
    # Copy binary
    cp "$binary_path" "$installer_dir/mantisdb.exe"
    
    # Create NSIS installer script
    cat > "$installer_dir/mantisdb.nsi" << EOF
!define APP_NAME "$APP_NAME"
!define APP_VERSION "$VERSION"
!define APP_PUBLISHER "$COMPANY"
!define APP_URL "$WEBSITE"
!define APP_EXECUTABLE "mantisdb.exe"

Name "\${APP_NAME}"
OutFile "$APP_NAME-$VERSION-Windows-$arch.exe"
InstallDir "\$PROGRAMFILES64\\MantisDB"
RequestExecutionLevel admin

Page directory
Page instfiles

Section "Install"
    SetOutPath "\$INSTDIR"
    File "mantisdb.exe"
    File "$INSTALLER_DIR/README.txt"
    File "$INSTALLER_DIR/LICENSE.txt"
    
    # Create config directory
    CreateDirectory "\$PROGRAMDATA\\MantisDB"
    
    # Create default config
    FileOpen \$0 "\$PROGRAMDATA\\MantisDB\\config.yaml" w
    FileWrite \$0 "# MantisDB Configuration\r\n"
    FileWrite \$0 "server:\r\n"
    FileWrite \$0 "  port: 8080\r\n"
    FileWrite \$0 "  admin_port: 8081\r\n"
    FileWrite \$0 "storage:\r\n"
    FileWrite \$0 "  data_dir: \$PROGRAMDATA\\MantisDB\\data\r\n"
    FileClose \$0
    
    # Create data directory
    CreateDirectory "\$PROGRAMDATA\\MantisDB\\data"
    
    # Add to PATH
    EnVar::SetHKLM
    EnVar::AddValue "PATH" "\$INSTDIR"
    
    # Create uninstaller
    WriteUninstaller "\$INSTDIR\\Uninstall.exe"
    
    # Registry entries
    WriteRegStr HKLM "Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\MantisDB" "DisplayName" "\${APP_NAME}"
    WriteRegStr HKLM "Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\MantisDB" "UninstallString" "\$INSTDIR\\Uninstall.exe"
    WriteRegStr HKLM "Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\MantisDB" "DisplayVersion" "\${APP_VERSION}"
    WriteRegStr HKLM "Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\MantisDB" "Publisher" "\${APP_PUBLISHER}"
    WriteRegStr HKLM "Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\MantisDB" "URLInfoAbout" "\${APP_URL}"
    
    # Create Start Menu shortcuts
    CreateDirectory "\$SMPROGRAMS\\MantisDB"
    CreateShortCut "\$SMPROGRAMS\\MantisDB\\MantisDB.lnk" "\$INSTDIR\\mantisdb.exe"
    CreateShortCut "\$SMPROGRAMS\\MantisDB\\Uninstall.lnk" "\$INSTDIR\\Uninstall.exe"
SectionEnd

Section "Uninstall"
    Delete "\$INSTDIR\\mantisdb.exe"
    Delete "\$INSTDIR\\README.txt"
    Delete "\$INSTDIR\\LICENSE.txt"
    Delete "\$INSTDIR\\Uninstall.exe"
    RMDir "\$INSTDIR"
    
    # Remove from PATH
    EnVar::SetHKLM
    EnVar::DeleteValue "PATH" "\$INSTDIR"
    
    # Remove registry entries
    DeleteRegKey HKLM "Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\MantisDB"
    
    # Remove Start Menu shortcuts
    Delete "\$SMPROGRAMS\\MantisDB\\MantisDB.lnk"
    Delete "\$SMPROGRAMS\\MantisDB\\Uninstall.lnk"
    RMDir "\$SMPROGRAMS\\MantisDB"
SectionEnd
EOF
    
    # Create batch installer as fallback
    cat > "$installer_dir/install.bat" << 'EOF'
@echo off
echo Installing MantisDB...

REM Create installation directory
if not exist "%PROGRAMFILES%\MantisDB" mkdir "%PROGRAMFILES%\MantisDB"

REM Copy files
copy mantisdb.exe "%PROGRAMFILES%\MantisDB\"
copy README.txt "%PROGRAMFILES%\MantisDB\"
copy LICENSE.txt "%PROGRAMFILES%\MantisDB\"

REM Create config directory
if not exist "%PROGRAMDATA%\MantisDB" mkdir "%PROGRAMDATA%\MantisDB"

REM Create default config
echo # MantisDB Configuration > "%PROGRAMDATA%\MantisDB\config.yaml"
echo server: >> "%PROGRAMDATA%\MantisDB\config.yaml"
echo   port: 8080 >> "%PROGRAMDATA%\MantisDB\config.yaml"
echo   admin_port: 8081 >> "%PROGRAMDATA%\MantisDB\config.yaml"
echo storage: >> "%PROGRAMDATA%\MantisDB\config.yaml"
echo   data_dir: %PROGRAMDATA%\MantisDB\data >> "%PROGRAMDATA%\MantisDB\config.yaml"

REM Add to PATH (requires admin)
setx /M PATH "%PATH%;%PROGRAMFILES%\MantisDB"

echo.
echo Installation complete!
echo Start MantisDB by running: mantisdb
echo Admin dashboard: http://localhost:8081
pause
EOF
    
    # Try to build with NSIS if available
    if command -v makensis &> /dev/null; then
        cd "$installer_dir"
        makensis mantisdb.nsi
        cd - > /dev/null
        log_success "Created Windows installer: $APP_NAME-$VERSION-Windows-$arch.exe"
    else
        # Create ZIP with batch installer
        local zip_name="$APP_NAME-$VERSION-Windows-$arch-installer.zip"
        cd "$installer_dir"
        zip -r "../../../$zip_name" . -x "*.nsi"
        cd - > /dev/null
        log_warning "Created Windows installer ZIP: $zip_name (NSIS not available)"
    fi
}

# Create Linux DEB package
create_linux_deb() {
    local arch=$1
    local binary_path="$BUILD_DIR/mantisdb-linux-$arch"
    
    if [ ! -f "$binary_path" ]; then
        log_warning "Linux binary not found for $arch: $binary_path"
        return 1
    fi
    
    log_info "Creating DEB package for $arch..."
    
    local deb_dir="$INSTALLER_DIR/linux/deb-$arch"
    local pkg_dir="$deb_dir/mantisdb_${VERSION}_${arch}"
    
    # Create package structure
    mkdir -p "$pkg_dir"/{DEBIAN,usr/bin,etc/mantisdb,usr/share/doc/mantisdb,lib/systemd/system}
    
    # Copy binary
    cp "$binary_path" "$pkg_dir/usr/bin/mantisdb"
    chmod +x "$pkg_dir/usr/bin/mantisdb"
    
    # Create control file
    cat > "$pkg_dir/DEBIAN/control" << EOF
Package: mantisdb
Version: $VERSION
Section: database
Priority: optional
Architecture: $arch
Maintainer: MantisDB Team <support@mantisdb.com>
Description: $DESCRIPTION
 MantisDB is a high-performance multi-model database that supports
 Key-Value, Document, and Columnar data models with a built-in
 admin dashboard for easy management.
Homepage: $WEBSITE
EOF
    
    # Create default config
    cat > "$pkg_dir/etc/mantisdb/config.yaml" << 'EOF'
# MantisDB Configuration
server:
  port: 8080
  admin_port: 8081
  host: "0.0.0.0"

storage:
  data_dir: "/var/lib/mantisdb"
  engine: "auto"  # auto, cgo, pure

logging:
  level: "info"
  file: "/var/log/mantisdb/mantisdb.log"

cache:
  size: 268435456  # 256MB
EOF
    
    # Create systemd service
    cat > "$pkg_dir/lib/systemd/system/mantisdb.service" << 'EOF'
[Unit]
Description=MantisDB Multi-Model Database
After=network.target

[Service]
Type=simple
User=mantisdb
Group=mantisdb
ExecStart=/usr/bin/mantisdb --config=/etc/mantisdb/config.yaml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF
    
    # Create postinst script
    cat > "$pkg_dir/DEBIAN/postinst" << 'EOF'
#!/bin/bash
set -e

# Create mantisdb user
if ! id mantisdb >/dev/null 2>&1; then
    useradd --system --home /var/lib/mantisdb --shell /bin/false mantisdb
fi

# Create directories
mkdir -p /var/lib/mantisdb /var/log/mantisdb
chown mantisdb:mantisdb /var/lib/mantisdb /var/log/mantisdb
chmod 755 /var/lib/mantisdb /var/log/mantisdb

# Enable and start service
systemctl daemon-reload
systemctl enable mantisdb
systemctl start mantisdb

echo "MantisDB installed successfully!"
echo "Service started and enabled for auto-start"
echo "Admin dashboard: http://localhost:8081"
EOF
    chmod +x "$pkg_dir/DEBIAN/postinst"
    
    # Create prerm script
    cat > "$pkg_dir/DEBIAN/prerm" << 'EOF'
#!/bin/bash
set -e

# Stop service
systemctl stop mantisdb || true
systemctl disable mantisdb || true
EOF
    chmod +x "$pkg_dir/DEBIAN/prerm"
    
    # Copy documentation
    cp "$INSTALLER_DIR/README.txt" "$pkg_dir/usr/share/doc/mantisdb/"
    cp "$INSTALLER_DIR/LICENSE.txt" "$pkg_dir/usr/share/doc/mantisdb/"
    
    # Build package
    if command -v dpkg-deb &> /dev/null; then
        dpkg-deb --build "$pkg_dir" "$INSTALLER_DIR/mantisdb_${VERSION}_${arch}.deb"
        log_success "Created DEB package: mantisdb_${VERSION}_${arch}.deb"
    elif command -v fpm &> /dev/null; then
        fpm -s dir -t deb -n mantisdb -v "$VERSION" -a "$arch" \
            --description "$DESCRIPTION" \
            --url "$WEBSITE" \
            --maintainer "MantisDB Team <support@mantisdb.com>" \
            --after-install "$pkg_dir/DEBIAN/postinst" \
            --before-remove "$pkg_dir/DEBIAN/prerm" \
            -C "$pkg_dir" \
            --package "$INSTALLER_DIR/mantisdb_${VERSION}_${arch}.deb" \
            .
        log_success "Created DEB package with fpm: mantisdb_${VERSION}_${arch}.deb"
    else
        # Create tar.gz as fallback
        local tar_name="mantisdb_${VERSION}_${arch}.tar.gz"
        tar -czf "$INSTALLER_DIR/$tar_name" -C "$pkg_dir" .
        log_warning "Created fallback archive: $tar_name (dpkg-deb not available)"
    fi
}

# Create Linux RPM package
create_linux_rpm() {
    local arch=$1
    local binary_path="$BUILD_DIR/mantisdb-linux-$arch"
    
    if [ ! -f "$binary_path" ]; then
        log_warning "Linux binary not found for $arch: $binary_path"
        return 1
    fi
    
    log_info "Creating RPM package for $arch..."
    
    if command -v fpm &> /dev/null; then
        local rpm_dir="$INSTALLER_DIR/linux/rpm-$arch"
        mkdir -p "$rpm_dir"/{usr/bin,etc/mantisdb,usr/share/doc/mantisdb,lib/systemd/system}
        
        # Copy files (same structure as DEB)
        cp "$binary_path" "$rpm_dir/usr/bin/mantisdb"
        chmod +x "$rpm_dir/usr/bin/mantisdb"
        
        # Create config and service files (same as DEB)
        cat > "$rpm_dir/etc/mantisdb/config.yaml" << 'EOF'
# MantisDB Configuration
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
  size: 268435456
EOF
        
        cat > "$rpm_dir/lib/systemd/system/mantisdb.service" << 'EOF'
[Unit]
Description=MantisDB Multi-Model Database
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
EOF
        
        # Copy documentation
        cp "$INSTALLER_DIR/README.txt" "$rpm_dir/usr/share/doc/mantisdb/"
        cp "$INSTALLER_DIR/LICENSE.txt" "$rpm_dir/usr/share/doc/mantisdb/"
        
        # Create post-install script
        cat > "$INSTALLER_DIR/rpm-postinst.sh" << 'EOF'
#!/bin/bash
if ! id mantisdb >/dev/null 2>&1; then
    useradd --system --home /var/lib/mantisdb --shell /bin/false mantisdb
fi
mkdir -p /var/lib/mantisdb /var/log/mantisdb
chown mantisdb:mantisdb /var/lib/mantisdb /var/log/mantisdb
systemctl daemon-reload
systemctl enable mantisdb
systemctl start mantisdb
EOF
        
        # Build RPM
        fpm -s dir -t rpm -n mantisdb -v "$VERSION" -a "$arch" \
            --description "$DESCRIPTION" \
            --url "$WEBSITE" \
            --maintainer "MantisDB Team <support@mantisdb.com>" \
            --after-install "$INSTALLER_DIR/rpm-postinst.sh" \
            -C "$rpm_dir" \
            --package "$INSTALLER_DIR/mantisdb-${VERSION}-1.${arch}.rpm" \
            .
        
        log_success "Created RPM package: mantisdb-${VERSION}-1.${arch}.rpm"
    else
        log_warning "fpm not available, skipping RPM creation"
    fi
}

# Create Homebrew formula
create_homebrew_formula() {
    log_info "Creating Homebrew formula..."
    
    local formula_dir="$INSTALLER_DIR/homebrew"
    mkdir -p "$formula_dir"
    
    # Calculate SHA256 for macOS binaries
    local amd64_sha=""
    local arm64_sha=""
    
    if [ -f "$BUILD_DIR/mantisdb-darwin-amd64.tar.gz" ]; then
        amd64_sha=$(shasum -a 256 "$BUILD_DIR/mantisdb-darwin-amd64.tar.gz" | cut -d' ' -f1)
    fi
    
    if [ -f "$BUILD_DIR/mantisdb-darwin-arm64.tar.gz" ]; then
        arm64_sha=$(shasum -a 256 "$BUILD_DIR/mantisdb-darwin-arm64.tar.gz" | cut -d' ' -f1)
    fi
    
    cat > "$formula_dir/mantisdb.rb" << EOF
class Mantisdb < Formula
  desc "$DESCRIPTION"
  homepage "$WEBSITE"
  version "$VERSION"
  
  if Hardware::CPU.intel?
    url "https://github.com/mantisdb/mantisdb/releases/download/v#{version}/mantisdb-darwin-amd64.tar.gz"
    sha256 "$amd64_sha"
  elsif Hardware::CPU.arm?
    url "https://github.com/mantisdb/mantisdb/releases/download/v#{version}/mantisdb-darwin-arm64.tar.gz"
    sha256 "$arm64_sha"
  end
  
  def install
    bin.install "mantisdb"
    
    # Create config directory
    etc.mkpath "mantisdb"
    
    # Install default config
    (etc/"mantisdb/config.yaml").write <<~EOS
      server:
        port: 8080
        admin_port: 8081
        host: "127.0.0.1"
      storage:
        data_dir: "#{var}/lib/mantisdb"
        engine: "auto"
      logging:
        level: "info"
        file: "#{var}/log/mantisdb/mantisdb.log"
      cache:
        size: 268435456
    EOS
  end
  
  def post_install
    (var/"lib/mantisdb").mkpath
    (var/"log/mantisdb").mkpath
  end
  
  service do
    run [opt_bin/"mantisdb", "--config=#{etc}/mantisdb/config.yaml"]
    keep_alive true
    log_path var/"log/mantisdb/mantisdb.log"
    error_log_path var/"log/mantisdb/mantisdb.log"
  end
  
  test do
    system "#{bin}/mantisdb", "--version"
  end
end
EOF
    
    log_success "Created Homebrew formula: mantisdb.rb"
}

# Create installers for specific platform
create_platform_installers() {
    local platform="$1"
    
    case "$platform" in
        "windows")
            log_info "Creating Windows installers..."
            if [ -f "$BUILD_DIR/mantisdb-windows-amd64.exe" ]; then
                create_windows_installer "amd64"
            else
                log_warning "No Windows binaries found"
            fi
            ;;
        "macos"|"darwin")
            log_info "Creating macOS installers..."
            local created_any=false
            if [ -f "$BUILD_DIR/mantisdb-darwin-amd64" ]; then
                create_macos_dmg "amd64" && created_any=true
            fi
            if [ -f "$BUILD_DIR/mantisdb-darwin-arm64" ]; then
                create_macos_dmg "arm64" && created_any=true
            fi
            if [ "$created_any" = false ]; then
                log_warning "No macOS binaries found"
            fi
            ;;
        "linux")
            log_info "Creating Linux installers..."
            local created_any=false
            if [ -f "$BUILD_DIR/mantisdb-linux-amd64" ]; then
                create_linux_deb "amd64" && created_any=true
                create_linux_rpm "amd64" && created_any=true
            fi
            if [ -f "$BUILD_DIR/mantisdb-linux-arm64" ]; then
                create_linux_deb "arm64" && created_any=true
                create_linux_rpm "arm64" && created_any=true
            fi
            if [ "$created_any" = false ]; then
                log_warning "No Linux binaries found"
            fi
            ;;
        *)
            log_error "Unknown platform: $platform"
            return 1
            ;;
    esac
}

# Main installer creation function
create_all_installers() {
    log_info "Creating installers for all platforms..."
    
    # macOS installers
    if [ -f "$BUILD_DIR/mantisdb-darwin-amd64" ] || [ -f "$BUILD_DIR/mantisdb-darwin-arm64" ]; then
        create_platform_installers "macos"
    fi
    
    # Windows installers
    if [ -f "$BUILD_DIR/mantisdb-windows-amd64.exe" ]; then
        create_platform_installers "windows"
    fi
    
    # Linux installers
    if [ -f "$BUILD_DIR/mantisdb-linux-amd64" ] || [ -f "$BUILD_DIR/mantisdb-linux-arm64" ]; then
        create_platform_installers "linux"
    fi
    
    log_success "All installers created successfully!"
}

# Print installer summary
print_installer_summary() {
    echo ""
    echo -e "${BLUE}================================${NC}"
    echo -e "${BLUE}Installer Summary${NC}"
    echo -e "${BLUE}================================${NC}"
    echo "Version: $VERSION"
    echo "Installer Directory: $INSTALLER_DIR"
    echo ""
    echo "Created installers:"
    
    if [ -d "$INSTALLER_DIR" ]; then
        find "$INSTALLER_DIR" -name "*.dmg" -o -name "*.exe" -o -name "*.deb" -o -name "*.rpm" -o -name "*.zip" -o -name "*.tar.gz" | while read -r file; do
            local size=$(stat -f%z "$file" 2>/dev/null || stat -c%s "$file" 2>/dev/null || echo "unknown")
            echo "  $(basename "$file") (${size} bytes)"
        done
    fi
    
    echo ""
    echo "Installation instructions:"
    echo "  macOS: Double-click .dmg file"
    echo "  Windows: Run .exe installer as administrator"
    echo "  Ubuntu/Debian: sudo dpkg -i mantisdb_*.deb"
    echo "  RHEL/CentOS: sudo rpm -i mantisdb-*.rpm"
    echo "  Homebrew: brew install mantisdb.rb"
}

# Main execution
main() {
    echo -e "${BLUE}MantisDB Installer Creation Script${NC}"
    echo -e "${BLUE}==================================${NC}"
    
    check_installer_dependencies
    setup_installer_environment
    
    if [ -n "$PLATFORM" ] && [ "$PLATFORM" != "all" ]; then
        create_platform_installers "$PLATFORM"
    else
        create_all_installers
    fi
    
    print_installer_summary
    
    log_success "Installer creation complete!"
}

# Parse command line arguments
PLATFORM=""
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
        --platform)
            PLATFORM="$2"
            shift 2
            ;;
        --platform=*)
            PLATFORM="${1#*=}"
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [--version VERSION] [--platform PLATFORM]"
            echo "Creates platform-specific installers for MantisDB"
            echo ""
            echo "Options:"
            echo "  --version VERSION    Set version (default: 1.0.0)"
            echo "  --platform PLATFORM  Create installers for specific platform"
            echo "                       (windows, macos, linux, or all)"
            echo ""
            echo "Examples:"
            echo "  $0                           # Create all installers"
            echo "  $0 --platform=windows       # Windows installers only"
            echo "  $0 --version=1.2.0 --platform=macos"
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