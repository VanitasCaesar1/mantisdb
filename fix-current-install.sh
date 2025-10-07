#!/bin/bash

# Quick fix for current MantisDB installation

echo "Fixing MantisDB installation..."

# 1. Fix CLI command
echo "1. Creating CLI symlink..."
if [ -w "/usr/local/bin" ]; then
    sudo ln -sf /Applications/MantisDB.app/Contents/MacOS/mantisdb /usr/local/bin/mantisdb
    echo "✓ CLI symlink created"
else
    mkdir -p "$HOME/.local/bin"
    ln -sf /Applications/MantisDB.app/Contents/MacOS/mantisdb "$HOME/.local/bin/mantisdb"
    echo "✓ CLI symlink created in ~/.local/bin"
    echo "  Add to PATH: export PATH=\"\$HOME/.local/bin:\$PATH\""
fi

# 2. Create launcher script
echo "2. Creating GUI launcher..."
cat > /Applications/MantisDB.app/Contents/MacOS/mantisdb-launcher << 'EOF'
#!/bin/bash
# MantisDB GUI Launcher

# Set up environment
export MANTISDB_HOME="$HOME/.mantisdb"
mkdir -p "$MANTISDB_HOME"/{data,logs}

# Create default config
if [ ! -f "$MANTISDB_HOME/config.yaml" ]; then
    cat > "$MANTISDB_HOME/config.yaml" << 'EOFCONFIG'
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
EOFCONFIG
fi

# Launch MantisDB
cd "$MANTISDB_HOME"
/Applications/MantisDB.app/Contents/MacOS/mantisdb --config="$MANTISDB_HOME/config.yaml" &

# Open admin dashboard
sleep 2
open "http://localhost:8081" 2>/dev/null || true
EOF

chmod +x /Applications/MantisDB.app/Contents/MacOS/mantisdb-launcher

# 3. Update Info.plist to use launcher
echo "3. Updating app bundle..."
/usr/libexec/PlistBuddy -c "Set :CFBundleExecutable mantisdb-launcher" /Applications/MantisDB.app/Contents/Info.plist 2>/dev/null || true

# 4. Remove quarantine
echo "4. Removing quarantine..."
xattr -dr com.apple.quarantine /Applications/MantisDB.app 2>/dev/null || true

echo ""
echo "✅ Fix complete!"
echo ""
echo "Now you can:"
echo "  1. Open MantisDB.app from Applications (opens GUI + browser)"
echo "  2. Run 'mantisdb' in Terminal (CLI mode)"
echo ""
echo "Test it:"
echo "  mantisdb --version"
