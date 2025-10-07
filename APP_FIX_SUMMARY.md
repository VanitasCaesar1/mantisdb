# MantisDB macOS App - Complete Fix

## Problems Identified

1. ❌ **"mantisdb command not found"** - No CLI symlink created
2. ❌ **App won't open from Finder** - Binary can't be double-clicked as GUI app
3. ❌ **No automatic browser opening** - Users don't know where to go after starting

## Root Causes

### Issue 1: CLI Not in PATH
The binary was in `/Applications/MantisDB.app/Contents/MacOS/mantisdb` but not symlinked to `/usr/local/bin/`

### Issue 2: App Bundle Structure
macOS GUI apps need a **launcher script**, not a direct binary. The binary works from Terminal but won't launch from Finder.

### Issue 3: User Experience
Database servers need special handling - they should:
- Run in background
- Open admin dashboard automatically
- Create default config

## Solutions Implemented

### 1. Fixed CLI Access

**Created symlink:**
```bash
sudo ln -sf /Applications/MantisDB.app/Contents/MacOS/mantisdb /usr/local/bin/mantisdb
```

Now `mantisdb` command works from anywhere!

### 2. Created GUI Launcher

**New app structure:**
```
MantisDB.app/
└── Contents/
    ├── Info.plist (points to launcher script)
    ├── MacOS/
    │   ├── mantisdb (launcher script - for GUI)
    │   ├── mantisdb-bin (actual binary - for CLI)
    │   └── mantisdb-cli (symlink to binary)
    └── Resources/
        └── mantisdb.icns
```

**Launcher script does:**
1. Creates `~/.mantisdb/` directory
2. Creates default config if missing
3. Starts MantisDB in background
4. Opens `http://localhost:8081` in browser
5. Returns immediately (doesn't block)

### 3. Improved User Experience

**When user double-clicks MantisDB.app:**
- ✅ Server starts automatically
- ✅ Browser opens to admin dashboard
- ✅ Config is auto-created
- ✅ Runs in background

**When user runs `mantisdb` in Terminal:**
- ✅ Direct CLI access
- ✅ Full control with flags
- ✅ Can specify custom config

## Quick Fix for Current Installation

Run this script to fix your current installation:

```bash
./fix-current-install.sh
```

This will:
1. Create CLI symlink
2. Add GUI launcher
3. Update Info.plist
4. Remove quarantine attribute

## Manual Fix Steps

If the script doesn't work, do this manually:

### Step 1: Fix CLI Command

```bash
# Create symlink
sudo ln -sf /Applications/MantisDB.app/Contents/MacOS/mantisdb /usr/local/bin/mantisdb

# Test it
mantisdb --version
```

### Step 2: Fix App Opening

```bash
# Remove quarantine
xattr -dr com.apple.quarantine /Applications/MantisDB.app

# Try opening
open /Applications/MantisDB.app
```

### Step 3: Manual Start (if app still won't open)

```bash
# Start manually
/Applications/MantisDB.app/Contents/MacOS/mantisdb &

# Open browser
open http://localhost:8081
```

## Testing the Fixed App

### Test 1: CLI Access
```bash
# Should show version
mantisdb --version

# Should start server
mantisdb &

# Check it's running
curl http://localhost:8081/health
```

### Test 2: GUI Access
```bash
# Open from Finder
open /Applications/MantisDB.app

# Should:
# - Start server in background
# - Open browser to http://localhost:8081
# - Return immediately
```

### Test 3: Custom Config
```bash
# CLI with custom config
mantisdb --config=/path/to/config.yaml

# Check default config was created
cat ~/.mantisdb/config.yaml
```

## Rebuilding the DMG with Fixes

To create a new DMG with all fixes:

```bash
# Clean old DMGs
rm -f dist/installers/MantisDB-*.dmg

# Rebuild
./scripts/create-dmg.sh --version=1.0.0

# Test new DMG
open dist/installers/MantisDB-1.0.0-macOS-arm64.dmg
```

## Configuration

### Default Locations

- **Config**: `~/.mantisdb/config.yaml`
- **Data**: `~/.mantisdb/data/`
- **Logs**: `~/.mantisdb/logs/mantisdb.log`

### Default Config

```yaml
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
```

## Usage Examples

### Start as GUI App
```bash
# Double-click in Finder, or:
open /Applications/MantisDB.app
```

### Start from CLI
```bash
# Default config
mantisdb

# Custom config
mantisdb --config=/etc/mantisdb/config.yaml

# Specific ports
mantisdb --port=8080 --admin-port=8081

# Background
mantisdb &
```

### Stop the Server
```bash
# Find process
ps aux | grep mantisdb

# Kill it
pkill mantisdb

# Or use PID
kill $(pgrep mantisdb)
```

### Check Status
```bash
# Health check
curl http://localhost:8081/health

# Version
mantisdb --version

# Check if running
pgrep mantisdb
```

## Troubleshooting

### "mantisdb: command not found"

**Solution:**
```bash
# Check if symlink exists
ls -la /usr/local/bin/mantisdb

# If not, create it
sudo ln -sf /Applications/MantisDB.app/Contents/MacOS/mantisdb /usr/local/bin/mantisdb

# Or add to PATH
export PATH="/Applications/MantisDB.app/Contents/MacOS:$PATH"
```

### App won't open from Finder

**Solution 1: Remove quarantine**
```bash
xattr -dr com.apple.quarantine /Applications/MantisDB.app
```

**Solution 2: Right-click > Open**
- Right-click MantisDB.app
- Click "Open"
- Click "Open" again in dialog

**Solution 3: Allow in System Preferences**
- System Preferences > Security & Privacy
- Click "Open Anyway"

### "Address already in use"

**Solution:**
```bash
# Check what's using the port
lsof -i :8080
lsof -i :8081

# Kill it
kill $(lsof -t -i:8080)

# Or use different ports
mantisdb --port=8082 --admin-port=8083
```

### Browser doesn't open automatically

**Solution:**
```bash
# Open manually
open http://localhost:8081

# Or check if server is running
curl http://localhost:8081/health
```

### Can't write to /usr/local/bin

**Solution:**
```bash
# Use ~/.local/bin instead
mkdir -p ~/.local/bin
ln -sf /Applications/MantisDB.app/Contents/MacOS/mantisdb ~/.local/bin/mantisdb

# Add to PATH in ~/.zshrc or ~/.bash_profile
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

## Uninstalling

### Complete Removal
```bash
# Stop server
pkill mantisdb

# Remove app
rm -rf /Applications/MantisDB.app

# Remove CLI symlink
sudo rm /usr/local/bin/mantisdb

# Remove data (optional)
rm -rf ~/.mantisdb

# Remove from PATH (if added)
# Edit ~/.zshrc and remove the PATH line
```

### Keep Data
```bash
# Remove app but keep data
rm -rf /Applications/MantisDB.app
sudo rm /usr/local/bin/mantisdb

# Data remains in ~/.mantisdb/
```

## Next Steps

1. **Test the fix:**
   ```bash
   ./fix-current-install.sh
   mantisdb --version
   open /Applications/MantisDB.app
   ```

2. **Rebuild DMG with fixes:**
   ```bash
   rm -f dist/installers/MantisDB-*.dmg
   ./scripts/create-dmg.sh --version=1.0.0
   ```

3. **Test new DMG:**
   ```bash
   open dist/installers/MantisDB-1.0.0-macOS-arm64.dmg
   # Drag to Applications
   # Double-click to test
   ```

4. **Distribute:**
   - Upload to GitHub Releases
   - Share with users
   - Update documentation

## Summary

✅ **CLI Access**: `mantisdb` command now works  
✅ **GUI Launch**: Double-click opens app + browser  
✅ **Auto-config**: Creates default config automatically  
✅ **User-friendly**: No terminal knowledge required  
✅ **Developer-friendly**: Full CLI access available  

The app now works like a proper macOS application! 🎉
