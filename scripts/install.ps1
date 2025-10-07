# MantisDB Windows Installer
# PowerShell installation script for Windows
# Usage: Run as Administrator
#   .\install.ps1

#Requires -RunAsAdministrator

param(
    [string]$Version = "latest",
    [string]$InstallDir = "$env:ProgramFiles\MantisDB",
    [string]$DataDir = "$env:ProgramData\MantisDB",
    [switch]$NoService = $false,
    [switch]$Quiet = $false
)

# Configuration
$GitHubRepo = "mantisdb/mantisdb"
$BinaryName = "mantisdb.exe"
$ServiceName = "MantisDB"

# Colors for output
function Write-ColorOutput {
    param(
        [string]$Message,
        [string]$Color = "White"
    )
    
    if (-not $Quiet) {
        Write-Host $Message -ForegroundColor $Color
    }
}

function Write-Info {
    param([string]$Message)
    Write-ColorOutput "  [INFO] $Message" "Cyan"
}

function Write-Success {
    param([string]$Message)
    Write-ColorOutput "[SUCCESS] $Message" "Green"
}

function Write-Warning {
    param([string]$Message)
    Write-ColorOutput "[WARNING] $Message" "Yellow"
}

function Write-Error {
    param([string]$Message)
    Write-ColorOutput "  [ERROR] $Message" "Red"
}

# Print banner
function Show-Banner {
    Write-Host ""
    Write-ColorOutput "╔══════════════════════════════════════════════════════════════╗" "Cyan"
    Write-ColorOutput "║                    MantisDB Installer                        ║" "Cyan"
    Write-ColorOutput "║                                                              ║" "Cyan"
    Write-ColorOutput "║  Multi-Model Database with Admin Dashboard                  ║" "Cyan"
    Write-ColorOutput "╚══════════════════════════════════════════════════════════════╝" "Cyan"
    Write-Host ""
}

# Check if running as administrator
function Test-Administrator {
    $currentUser = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($currentUser)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

# Get latest version from GitHub
function Get-LatestVersion {
    Write-Info "Fetching latest version from GitHub..."
    
    try {
        $response = Invoke-RestMethod -Uri "https://api.github.com/repos/$GitHubRepo/releases/latest"
        $latestVersion = $response.tag_name -replace '^v', ''
        Write-Info "Latest version: $latestVersion"
        return $latestVersion
    }
    catch {
        Write-Warning "Could not fetch latest version, using 'latest'"
        return "latest"
    }
}

# Download binary
function Get-Binary {
    param(
        [string]$Version
    )
    
    Write-Info "Downloading MantisDB binary..."
    
    $downloadUrl = if ($Version -eq "latest") {
        "https://github.com/$GitHubRepo/releases/latest/download/mantisdb-windows-amd64.exe"
    } else {
        "https://github.com/$GitHubRepo/releases/download/v$Version/mantisdb-windows-amd64.exe"
    }
    
    $tempFile = "$env:TEMP\mantisdb.exe"
    
    try {
        Invoke-WebRequest -Uri $downloadUrl -OutFile $tempFile -UseBasicParsing
        Write-Success "Downloaded binary"
        return $tempFile
    }
    catch {
        Write-Error "Failed to download binary: $_"
        exit 1
    }
}

# Install binary
function Install-Binary {
    param(
        [string]$TempFile
    )
    
    Write-Info "Installing binary to $InstallDir..."
    
    # Create installation directory
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }
    
    # Copy binary
    Copy-Item -Path $TempFile -Destination "$InstallDir\$BinaryName" -Force
    
    # Clean up
    Remove-Item -Path $TempFile -Force
    
    Write-Success "Binary installed to $InstallDir\$BinaryName"
}

# Create directories
function New-Directories {
    Write-Info "Creating directories..."
    
    $directories = @(
        $DataDir,
        "$DataDir\data",
        "$DataDir\logs",
        "$DataDir\config"
    )
    
    foreach ($dir in $directories) {
        if (-not (Test-Path $dir)) {
            New-Item -ItemType Directory -Path $dir -Force | Out-Null
        }
    }
    
    Write-Success "Directories created"
}

# Create default configuration
function New-Configuration {
    Write-Info "Creating default configuration..."
    
    $configFile = "$DataDir\config\config.yaml"
    
    if (Test-Path $configFile) {
        Write-Warning "Configuration file already exists, skipping"
        return
    }
    
    $configContent = @"
# MantisDB Configuration
server:
  port: 8080
  admin_port: 8081
  host: "0.0.0.0"

storage:
  data_dir: "$($DataDir -replace '\\', '/')/data"
  engine: "auto"
  sync_writes: true

logging:
  level: "info"
  format: "json"
  file: "$($DataDir -replace '\\', '/')/logs/mantisdb.log"

cache:
  size: 268435456  # 256MB

security:
  admin_token: ""
  enable_cors: false
  cors_origins:
    - "http://localhost:3000"
"@
    
    $configContent | Out-File -FilePath $configFile -Encoding UTF8
    
    Write-Success "Configuration created at $configFile"
}

# Add to PATH
function Add-ToPath {
    Write-Info "Adding to system PATH..."
    
    $currentPath = [Environment]::GetEnvironmentVariable("PATH", "Machine")
    
    if ($currentPath -notlike "*$InstallDir*") {
        $newPath = "$currentPath;$InstallDir"
        [Environment]::SetEnvironmentVariable("PATH", $newPath, "Machine")
        Write-Success "Added to system PATH"
    } else {
        Write-Info "Already in system PATH"
    }
}

# Create Windows service
function New-Service {
    Write-Info "Creating Windows service..."
    
    # Check if service already exists
    $existingService = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    
    if ($existingService) {
        Write-Warning "Service already exists, stopping and removing..."
        Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
        sc.exe delete $ServiceName | Out-Null
        Start-Sleep -Seconds 2
    }
    
    # Create service using sc.exe
    $binaryPath = "`"$InstallDir\$BinaryName`" --config=`"$DataDir\config\config.yaml`""
    
    $result = sc.exe create $ServiceName binPath= $binaryPath start= auto DisplayName= "MantisDB Database"
    
    if ($LASTEXITCODE -eq 0) {
        # Set service description
        sc.exe description $ServiceName "MantisDB Multi-Model Database with Admin Dashboard"
        
        # Configure service recovery options
        sc.exe failure $ServiceName reset= 86400 actions= restart/5000/restart/10000/restart/30000
        
        Write-Success "Windows service created"
        Write-Info "Service name: $ServiceName"
    } else {
        Write-Error "Failed to create service: $result"
    }
}

# Create firewall rules
function New-FirewallRules {
    Write-Info "Creating firewall rules..."
    
    try {
        # Database port
        New-NetFirewallRule -DisplayName "MantisDB Database" `
            -Direction Inbound `
            -Protocol TCP `
            -LocalPort 8080 `
            -Action Allow `
            -ErrorAction SilentlyContinue | Out-Null
        
        # Admin port
        New-NetFirewallRule -DisplayName "MantisDB Admin Dashboard" `
            -Direction Inbound `
            -Protocol TCP `
            -LocalPort 8081 `
            -Action Allow `
            -ErrorAction SilentlyContinue | Out-Null
        
        Write-Success "Firewall rules created"
    }
    catch {
        Write-Warning "Could not create firewall rules: $_"
        Write-Info "You may need to manually configure firewall"
    }
}

# Create uninstaller
function New-Uninstaller {
    Write-Info "Creating uninstaller..."
    
    $uninstallScript = @"
# MantisDB Uninstaller
#Requires -RunAsAdministrator

Write-Host "Uninstalling MantisDB..." -ForegroundColor Yellow

# Stop and remove service
`$service = Get-Service -Name "$ServiceName" -ErrorAction SilentlyContinue
if (`$service) {
    Write-Host "Stopping service..." -ForegroundColor Cyan
    Stop-Service -Name "$ServiceName" -Force -ErrorAction SilentlyContinue
    sc.exe delete "$ServiceName" | Out-Null
}

# Remove from PATH
`$currentPath = [Environment]::GetEnvironmentVariable("PATH", "Machine")
`$newPath = `$currentPath -replace [regex]::Escape(";$InstallDir"), ""
[Environment]::SetEnvironmentVariable("PATH", `$newPath, "Machine")

# Remove firewall rules
Remove-NetFirewallRule -DisplayName "MantisDB Database" -ErrorAction SilentlyContinue
Remove-NetFirewallRule -DisplayName "MantisDB Admin Dashboard" -ErrorAction SilentlyContinue

# Remove installation directory
Write-Host "Removing installation directory..." -ForegroundColor Cyan
Remove-Item -Path "$InstallDir" -Recurse -Force -ErrorAction SilentlyContinue

# Ask about data directory
`$removeData = Read-Host "Remove data directory? (y/N)"
if (`$removeData -eq "y" -or `$removeData -eq "Y") {
    Remove-Item -Path "$DataDir" -Recurse -Force -ErrorAction SilentlyContinue
    Write-Host "Data directory removed" -ForegroundColor Green
}

Write-Host "MantisDB uninstalled successfully!" -ForegroundColor Green
Write-Host ""
Read-Host "Press Enter to exit"
"@
    
    $uninstallScript | Out-File -FilePath "$InstallDir\uninstall.ps1" -Encoding UTF8
    
    Write-Success "Uninstaller created at $InstallDir\uninstall.ps1"
}

# Print post-install instructions
function Show-Instructions {
    Write-Host ""
    Write-ColorOutput "╔══════════════════════════════════════════════════════════════╗" "Green"
    Write-ColorOutput "║              Installation Complete!                          ║" "Green"
    Write-ColorOutput "╚══════════════════════════════════════════════════════════════╝" "Green"
    Write-Host ""
    
    Write-ColorOutput "Installation Details:" "Cyan"
    Write-Host "  Binary: $InstallDir\$BinaryName"
    Write-Host "  Config: $DataDir\config\config.yaml"
    Write-Host "  Data: $DataDir\data"
    Write-Host "  Logs: $DataDir\logs"
    Write-Host ""
    
    Write-ColorOutput "Quick Start:" "Cyan"
    Write-Host ""
    
    if (-not $NoService) {
        Write-Host "1. Start the service:"
        Write-Host "   Start-Service $ServiceName" -ForegroundColor Yellow
        Write-Host ""
        Write-Host "2. Or run manually:"
        Write-Host "   mantisdb --config=`"$DataDir\config\config.yaml`"" -ForegroundColor Yellow
    } else {
        Write-Host "Run MantisDB:"
        Write-Host "   mantisdb --config=`"$DataDir\config\config.yaml`"" -ForegroundColor Yellow
    }
    
    Write-Host ""
    Write-Host "3. Access the admin dashboard:"
    Write-Host "   http://localhost:8081" -ForegroundColor Yellow
    Write-Host ""
    
    Write-ColorOutput "Service Management:" "Cyan"
    Write-Host "  Start:   Start-Service $ServiceName"
    Write-Host "  Stop:    Stop-Service $ServiceName"
    Write-Host "  Restart: Restart-Service $ServiceName"
    Write-Host "  Status:  Get-Service $ServiceName"
    Write-Host ""
    
    Write-ColorOutput "Documentation:" "Cyan"
    Write-Host "  https://mantisdb.com/docs"
    Write-Host ""
    
    Write-ColorOutput "Support:" "Cyan"
    Write-Host "  https://github.com/$GitHubRepo/issues"
    Write-Host ""
    
    Write-ColorOutput "To uninstall:" "Cyan"
    Write-Host "  Run: $InstallDir\uninstall.ps1"
    Write-Host ""
}

# Main installation function
function Install-MantisDB {
    Show-Banner
    
    # Check administrator privileges
    if (-not (Test-Administrator)) {
        Write-Error "This script must be run as Administrator"
        Write-Host "Right-click PowerShell and select 'Run as Administrator'"
        exit 1
    }
    
    Write-Info "Starting installation..."
    Write-Info "Install directory: $InstallDir"
    Write-Info "Data directory: $DataDir"
    Write-Host ""
    
    # Get version
    if ($Version -eq "latest") {
        $Version = Get-LatestVersion
    }
    
    # Download and install
    $tempFile = Get-Binary -Version $Version
    Install-Binary -TempFile $tempFile
    
    # Setup
    New-Directories
    New-Configuration
    Add-ToPath
    
    # Create service
    if (-not $NoService) {
        New-Service
        New-FirewallRules
    }
    
    # Create uninstaller
    New-Uninstaller
    
    # Show instructions
    Show-Instructions
    
    # Ask to start service
    if (-not $NoService -and -not $Quiet) {
        Write-Host ""
        $startNow = Read-Host "Start MantisDB service now? (Y/n)"
        if ($startNow -ne "n" -and $startNow -ne "N") {
            try {
                Start-Service -Name $ServiceName
                Write-Success "Service started successfully!"
                Write-Host ""
                Write-Host "Admin dashboard: http://localhost:8081" -ForegroundColor Green
            }
            catch {
                Write-Error "Failed to start service: $_"
                Write-Info "You can start it manually with: Start-Service $ServiceName"
            }
        }
    }
}

# Run installation
try {
    Install-MantisDB
}
catch {
    Write-Error "Installation failed: $_"
    Write-Host $_.ScriptStackTrace
    exit 1
}
