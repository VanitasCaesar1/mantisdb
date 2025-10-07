# Installation Guide

Complete installation instructions for MantisDB across all supported platforms and deployment methods.

## 📋 System Requirements

### Minimum Requirements
- **CPU**: 2 cores
- **Memory**: 2GB RAM
- **Storage**: 1GB free space
- **OS**: Linux, macOS, or Windows

### Recommended for Production
- **CPU**: 4+ cores
- **Memory**: 8GB+ RAM (4GB for cache, 4GB for system)
- **Storage**: SSD with 10GB+ free space
- **Network**: 1Gbps+ for distributed deployments

## 🚀 Installation Methods

### 1. Pre-built Binaries (Recommended)

Download the latest release for your platform:

#### Linux/macOS One-liner
```bash
curl -L https://github.com/mantisdb/mantisdb/releases/latest/download/mantisdb-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m).tar.gz | tar xz
cd mantisdb-*
sudo ./install.sh
```

#### Manual Download
1. Visit [GitHub Releases](https://github.com/mantisdb/mantisdb/releases)
2. Download the appropriate binary:
   - `mantisdb-linux-amd64.tar.gz` - Linux x86_64
   - `mantisdb-linux-arm64.tar.gz` - Linux ARM64
   - `mantisdb-darwin-amd64.tar.gz` - macOS Intel
   - `mantisdb-darwin-arm64.tar.gz` - macOS Apple Silicon
   - `mantisdb-windows-amd64.zip` - Windows x86_64

3. Extract and install:
   ```bash
   # Linux/macOS
   tar -xzf mantisdb-*.tar.gz
   cd mantisdb-*
   sudo ./install.sh
   
   # Windows (PowerShell as Administrator)
   Expand-Archive -Path "mantisdb-windows-amd64.zip" -DestinationPath "."
   cd mantisdb-windows-amd64
   .\install.ps1
   ```

### 2. Docker

#### Quick Start
```bash
docker run -d \
  --name mantisdb \
  -p 8080:8080 \
  -p 8081:8081 \
  -v mantisdb_data:/var/lib/mantisdb \
  mantisdb/mantisdb:latest
```

#### Docker Compose (Recommended)
```yaml
# docker-compose.yml
version: "3.8"

services:
  mantisdb:
    image: mantisdb/mantisdb:latest
    container_name: mantisdb
    restart: unless-stopped
    ports:
      - "8080:8080"
      - "8081:8081"
    volumes:
      - mantisdb_data:/var/lib/mantisdb
      - ./config.yaml:/etc/mantisdb/config.yaml:ro
    environment:
      - MANTIS_ADMIN_TOKEN=${MANTIS_ADMIN_TOKEN}
    healthcheck:
      test: ["CMD", "wget", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3

volumes:
  mantisdb_data:
```

Start with:
```bash
export MANTIS_ADMIN_TOKEN="$(openssl rand -hex 32)"
docker-compose up -d
```

### 3. Build from Source

#### Prerequisites
- **Go**: 1.21 or later
- **Node.js**: 18 or later (for admin dashboard)
- **Git**: Latest version

#### Build Steps
```bash
# Clone repository
git clone https://github.com/mantisdb/mantisdb.git
cd mantisdb

# Install dependencies
make deps

# Build for development
make build

# Build for production (all platforms)
make production

# Install locally
make install
```

### 4. Package Managers

#### Homebrew (macOS/Linux)
```bash
# Add tap
brew tap mantisdb/tap
brew install mantisdb
```

#### Chocolatey (Windows)
```powershell
choco install mantisdb
```

#### APT (Ubuntu/Debian)
```bash
# Add repository
curl -fsSL https://packages.mantisdb.com/gpg | sudo apt-key add -
echo "deb https://packages.mantisdb.com/apt stable main" | sudo tee /etc/apt/sources.list.d/mantisdb.list
sudo apt update
sudo apt install mantisdb
```

#### YUM/DNF (RHEL/CentOS/Fedora)
```bash
# Add repository
sudo tee /etc/yum.repos.d/mantisdb.repo << EOF
[mantisdb]
name=MantisDB Repository
baseurl=https://packages.mantisdb.com/rpm
enabled=1
gpgcheck=1
gpgkey=https://packages.mantisdb.com/gpg
EOF

# Install
sudo yum install mantisdb  # RHEL/CentOS
sudo dnf install mantisdb  # Fedora
```

## ⚙️ Post-Installation Setup

### 1. Create Configuration

#### Default Locations
- **Linux**: `/etc/mantisdb/config.yaml`
- **macOS**: `/usr/local/etc/mantisdb/config.yaml`
- **Windows**: `%PROGRAMDATA%\MantisDB\config.yaml`

#### Basic Configuration
```yaml
server:
  port: 8080
  admin_port: 8081
  host: "0.0.0.0"

database:
  data_dir: "/var/lib/mantisdb"
  cache_size: "512MB"
  buffer_size: "128MB"
  sync_writes: true

security:
  admin_token: "${MANTIS_ADMIN_TOKEN}"
  enable_cors: false

logging:
  level: "info"
  format: "json"
  output: "stdout"

backup:
  enabled: true
  schedule: "0 2 * * *"  # Daily at 2 AM
  retention_days: 30
```

### 2. Set Environment Variables

```bash
# Essential settings
export MANTIS_ADMIN_TOKEN="$(openssl rand -hex 32)"
export MANTIS_DATA_DIR="/var/lib/mantisdb"
export MANTIS_LOG_LEVEL="info"

# Optional settings
export MANTIS_PORT="8080"
export MANTIS_ADMIN_PORT="8081"
export MANTIS_CACHE_SIZE="512MB"
```

### 3. Create Data Directory

```bash
# Linux/macOS
sudo mkdir -p /var/lib/mantisdb
sudo chown mantisdb:mantisdb /var/lib/mantisdb
sudo chmod 755 /var/lib/mantisdb

# Windows
mkdir C:\ProgramData\MantisDB\data
```

## 🔧 Service Management

### Systemd (Linux)

The installer automatically creates a systemd service:

```bash
# Enable auto-start
sudo systemctl enable mantisdb

# Start service
sudo systemctl start mantisdb

# Check status
sudo systemctl status mantisdb

# View logs
sudo journalctl -u mantisdb -f

# Stop service
sudo systemctl stop mantisdb
```

#### Manual Service Creation
```bash
# Create service file
sudo tee /etc/systemd/system/mantisdb.service << EOF
[Unit]
Description=MantisDB Database Server
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

# Reload systemd and enable
sudo systemctl daemon-reload
sudo systemctl enable mantisdb
sudo systemctl start mantisdb
```

### Launchd (macOS)

```bash
# Create launch daemon
sudo tee /Library/LaunchDaemons/com.mantisdb.plist << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.mantisdb</string>
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
    <string>/var/log/mantisdb/mantisdb.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/mantisdb/mantisdb.log</string>
</dict>
</plist>
EOF

# Load and start
sudo launchctl load /Library/LaunchDaemons/com.mantisdb.plist
sudo launchctl start com.mantisdb
```

### Windows Service

```powershell
# Install as Windows service
sc create MantisDB binPath= "C:\Program Files\MantisDB\mantisdb.exe --config=C:\ProgramData\MantisDB\config.yaml"
sc config MantisDB start= auto
sc start MantisDB

# Or use PowerShell
New-Service -Name "MantisDB" -BinaryPathName "C:\Program Files\MantisDB\mantisdb.exe --config=C:\ProgramData\MantisDB\config.yaml" -StartupType Automatic
Start-Service -Name "MantisDB"
```

## 🌐 Kubernetes Deployment

### Namespace and ConfigMap
```yaml
# namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: mantisdb

---
# configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: mantisdb-config
  namespace: mantisdb
data:
  config.yaml: |
    server:
      port: 8080
      admin_port: 8081
      host: "0.0.0.0"
    database:
      data_dir: "/var/lib/mantisdb"
      cache_size: "1GB"
      buffer_size: "256MB"
      sync_writes: true
    security:
      admin_token: "${MANTIS_ADMIN_TOKEN}"
    logging:
      level: "info"
      format: "json"
```

### Secret
```yaml
# secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: mantisdb-secret
  namespace: mantisdb
type: Opaque
data:
  admin-token: <base64-encoded-token>
```

### StatefulSet
```yaml
# statefulset.yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: mantisdb
  namespace: mantisdb
spec:
  serviceName: mantisdb
  replicas: 1
  selector:
    matchLabels:
      app: mantisdb
  template:
    metadata:
      labels:
        app: mantisdb
    spec:
      containers:
        - name: mantisdb
          image: mantisdb/mantisdb:latest
          ports:
            - containerPort: 8080
              name: api
            - containerPort: 8081
              name: admin
          env:
            - name: MANTIS_ADMIN_TOKEN
              valueFrom:
                secretKeyRef:
                  name: mantisdb-secret
                  key: admin-token
          volumeMounts:
            - name: data
              mountPath: /var/lib/mantisdb
            - name: config
              mountPath: /etc/mantisdb
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 30
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 5
          resources:
            requests:
              memory: "1Gi"
              cpu: "500m"
            limits:
              memory: "4Gi"
              cpu: "2"
      volumes:
        - name: config
          configMap:
            name: mantisdb-config
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 10Gi
```

### Service and Ingress
```yaml
# service.yaml
apiVersion: v1
kind: Service
metadata:
  name: mantisdb
  namespace: mantisdb
spec:
  selector:
    app: mantisdb
  ports:
    - name: api
      port: 8080
      targetPort: 8080
    - name: admin
      port: 8081
      targetPort: 8081
  type: ClusterIP

---
# ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: mantisdb-ingress
  namespace: mantisdb
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
spec:
  rules:
    - host: mantisdb.yourdomain.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: mantisdb
                port:
                  number: 8081
    - host: api.mantisdb.yourdomain.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: mantisdb
                port:
                  number: 8080
```

Deploy with:
```bash
kubectl apply -f namespace.yaml
kubectl apply -f configmap.yaml
kubectl apply -f secret.yaml
kubectl apply -f statefulset.yaml
kubectl apply -f service.yaml
kubectl apply -f ingress.yaml
```

## ✅ Verification

### Check Installation
```bash
# Check version
mantisdb --version

# Test configuration
mantisdb --config=/etc/mantisdb/config.yaml --help

# Start in foreground (for testing)
mantisdb --config=/etc/mantisdb/config.yaml
```

### Health Check
```bash
# Start MantisDB
mantisdb &

# Wait for startup
sleep 5

# Check health
curl http://localhost:8080/health

# Check admin dashboard
curl http://localhost:8081

# Stop MantisDB
pkill mantisdb
```

### Service Status
```bash
# Systemd
sudo systemctl status mantisdb

# Docker
docker ps | grep mantisdb
docker logs mantisdb

# Kubernetes
kubectl get pods -n mantisdb
kubectl logs -n mantisdb mantisdb-0
```

## 🚨 Troubleshooting

### Common Issues

#### Port Already in Use
```bash
# Check what's using the port
sudo lsof -i :8080
sudo lsof -i :8081

# Change ports in configuration
# Edit /etc/mantisdb/config.yaml
```

#### Permission Denied
```bash
# Fix data directory permissions
sudo chown -R mantisdb:mantisdb /var/lib/mantisdb
sudo chmod 755 /var/lib/mantisdb

# Fix binary permissions
sudo chmod +x /usr/local/bin/mantisdb
```

#### Service Won't Start
```bash
# Check logs
sudo journalctl -u mantisdb -f

# Check configuration
mantisdb --config=/etc/mantisdb/config.yaml --help

# Test manually
sudo -u mantisdb mantisdb --config=/etc/mantisdb/config.yaml
```

#### Docker Issues
```bash
# Check container logs
docker logs mantisdb

# Check container status
docker inspect mantisdb

# Restart container
docker restart mantisdb
```

### Log Locations

- **Systemd**: `sudo journalctl -u mantisdb -f`
- **Docker**: `docker logs mantisdb`
- **File**: `/var/log/mantisdb/mantisdb.log`
- **Kubernetes**: `kubectl logs -n mantisdb mantisdb-0`

## 🔄 Upgrading

### Binary Upgrade
```bash
# Stop service
sudo systemctl stop mantisdb

# Backup data (recommended)
sudo cp -r /var/lib/mantisdb /var/lib/mantisdb.backup

# Download and install new version
curl -L https://github.com/mantisdb/mantisdb/releases/latest/download/mantisdb-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m).tar.gz | tar xz
cd mantisdb-*
sudo ./install.sh

# Start service
sudo systemctl start mantisdb
```

### Docker Upgrade
```bash
# Pull new image
docker pull mantisdb/mantisdb:latest

# Stop and remove old container
docker stop mantisdb
docker rm mantisdb

# Start new container
docker-compose up -d
```

### Kubernetes Upgrade
```bash
# Update image
kubectl set image statefulset/mantisdb mantisdb=mantisdb/mantisdb:latest -n mantisdb

# Check rollout status
kubectl rollout status statefulset/mantisdb -n mantisdb
```

## 🗑️ Uninstallation

### Remove Binary Installation
```bash
# Stop service
sudo systemctl stop mantisdb
sudo systemctl disable mantisdb

# Remove files
sudo rm -f /usr/local/bin/mantisdb
sudo rm -rf /etc/mantisdb
sudo rm -f /etc/systemd/system/mantisdb.service

# Remove data (optional)
sudo rm -rf /var/lib/mantisdb

# Remove user (optional)
sudo userdel mantisdb
```

### Remove Docker Installation
```bash
# Stop and remove containers
docker-compose down

# Remove volumes (optional)
docker volume rm mantisdb_mantisdb_data

# Remove images
docker rmi mantisdb/mantisdb:latest
```

## 📚 Next Steps

After installation:

1. **[Configuration Guide](configuration.md)** - Customize your setup
2. **[Quick Start Guide](quickstart.md)** - Basic operations
3. **[Admin Dashboard](../admin/dashboard.md)** - Web interface
4. **[API Reference](../api/rest.md)** - HTTP API documentation
5. **[Production Setup](../deployment/production.md)** - Production deployment

---

For more detailed information, visit the [MantisDB Documentation](../README.md).