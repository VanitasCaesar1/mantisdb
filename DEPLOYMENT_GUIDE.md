# MantisDB Deployment Guide

Complete guide for deploying MantisDB in production environments.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation Methods](#installation-methods)
- [Configuration](#configuration)
- [Deployment Strategies](#deployment-strategies)
- [Monitoring & Maintenance](#monitoring--maintenance)
- [Troubleshooting](#troubleshooting)

## Prerequisites

### System Requirements

**Minimum:**
- CPU: 2 cores
- RAM: 4GB
- Disk: 20GB SSD
- OS: Linux (Ubuntu 20.04+, CentOS 8+), macOS 11+, Windows Server 2019+

**Recommended:**
- CPU: 8+ cores
- RAM: 16GB+
- Disk: 100GB+ NVMe SSD
- OS: Linux (Ubuntu 22.04 LTS)

### Software Dependencies

- **Go**: 1.20+ (for building from source)
- **Rust**: 1.75+ (for building from source)
- **Node.js**: 18+ (for Admin UI)
- **Docker**: 20.10+ (optional, for containerized deployment)

## Installation Methods

### Method 1: Binary Installation (Recommended)

```bash
# Download latest release
wget https://github.com/mantisdb/mantisdb/releases/download/v1.0.0/mantisdb-linux-amd64.tar.gz

# Extract
tar -xzf mantisdb-linux-amd64.tar.gz
cd mantisdb-linux-amd64

# Install
sudo cp mantisdb /usr/local/bin/
sudo chmod +x /usr/local/bin/mantisdb

# Verify installation
mantisdb --version
```

### Method 2: Build from Source

```bash
# Clone repository
git clone https://github.com/mantisdb/mantisdb.git
cd mantisdb

# Build production binaries
./scripts/build-production.sh --parallel --optimization size

# Build Admin UI
cd admin/frontend
npm ci
npm run build
cd ../..

# Install
sudo cp mantisdb /usr/local/bin/
```

### Method 3: Docker Deployment

```bash
# Pull image
docker pull mantisdb/mantisdb:latest

# Run container
docker run -d \
  --name mantisdb \
  -p 8080:8080 \
  -p 8081:8081 \
  -v mantisdb-data:/data \
  -v mantisdb-wal:/wal \
  -v mantisdb-backups:/backups \
  mantisdb/mantisdb:latest
```

### Method 4: Kubernetes Deployment

```bash
# Apply Kubernetes manifests
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/secret.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
kubectl apply -f k8s/ingress.yaml
```

## Configuration

### 1. Create Configuration Directory

```bash
sudo mkdir -p /etc/mantisdb
sudo mkdir -p /var/lib/mantisdb/{data,wal,backups}
sudo mkdir -p /var/log/mantisdb
```

### 2. Copy Configuration Files

```bash
# Copy production config
sudo cp configs/production.yaml /etc/mantisdb/config.yaml

# Copy environment template
sudo cp .env.production.template /etc/mantisdb/.env

# Edit configuration
sudo nano /etc/mantisdb/config.yaml
sudo nano /etc/mantisdb/.env
```

### 3. Set Permissions

```bash
# Create mantisdb user
sudo useradd -r -s /bin/false mantisdb

# Set ownership
sudo chown -R mantisdb:mantisdb /var/lib/mantisdb
sudo chown -R mantisdb:mantisdb /var/log/mantisdb
sudo chown -R mantisdb:mantisdb /etc/mantisdb

# Set permissions
sudo chmod 750 /var/lib/mantisdb
sudo chmod 750 /var/log/mantisdb
sudo chmod 640 /etc/mantisdb/.env
```

## Deployment Strategies

### Strategy 1: Systemd Service (Linux)

Create `/etc/systemd/system/mantisdb.service`:

```ini
[Unit]
Description=MantisDB High-Performance Database
After=network.target

[Service]
Type=simple
User=mantisdb
Group=mantisdb
WorkingDirectory=/var/lib/mantisdb
ExecStart=/usr/local/bin/mantisdb --config /etc/mantisdb/config.yaml
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=5s
LimitNOFILE=65536
LimitNPROC=4096

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/mantisdb /var/log/mantisdb

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable mantisdb
sudo systemctl start mantisdb
sudo systemctl status mantisdb
```

### Strategy 2: Docker Compose

Create `docker-compose.prod.yml`:

```yaml
version: '3.8'

services:
  mantisdb:
    image: mantisdb/mantisdb:1.0.0
    container_name: mantisdb
    restart: unless-stopped
    ports:
      - "8080:8080"
      - "8081:8081"
    environment:
      - MANTIS_ENV=production
      - MANTIS_LOG_LEVEL=info
    volumes:
      - mantisdb-data:/data
      - mantisdb-wal:/wal
      - mantisdb-backups:/backups
      - ./configs/production.yaml:/etc/mantisdb/config.yaml:ro
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    networks:
      - mantisdb-network

  nginx:
    image: nginx:alpine
    container_name: mantisdb-nginx
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx/nginx.conf:/etc/nginx/nginx.conf:ro
      - ./nginx/ssl:/etc/nginx/ssl:ro
    depends_on:
      - mantisdb
    networks:
      - mantisdb-network

volumes:
  mantisdb-data:
    driver: local
  mantisdb-wal:
    driver: local
  mantisdb-backups:
    driver: local

networks:
  mantisdb-network:
    driver: bridge
```

Deploy:

```bash
docker-compose -f docker-compose.prod.yml up -d
```

### Strategy 3: Kubernetes Deployment

Create `k8s/deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mantisdb
  namespace: mantisdb
spec:
  replicas: 3
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
        image: mantisdb/mantisdb:1.0.0
        ports:
        - containerPort: 8080
          name: api
        - containerPort: 8081
          name: admin
        env:
        - name: MANTIS_ENV
          value: "production"
        - name: MANTIS_LOG_LEVEL
          value: "info"
        volumeMounts:
        - name: data
          mountPath: /data
        - name: wal
          mountPath: /wal
        - name: config
          mountPath: /etc/mantisdb
        resources:
          requests:
            memory: "2Gi"
            cpu: "1000m"
          limits:
            memory: "4Gi"
            cpu: "2000m"
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
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: mantisdb-data-pvc
      - name: wal
        persistentVolumeClaim:
          claimName: mantisdb-wal-pvc
      - name: config
        configMap:
          name: mantisdb-config
```

### Strategy 4: High Availability Setup

```yaml
# Load Balancer Configuration (HAProxy)
frontend mantisdb_frontend
    bind *:8080
    mode tcp
    default_backend mantisdb_backend

backend mantisdb_backend
    mode tcp
    balance roundrobin
    option tcp-check
    server mantisdb1 10.0.1.10:8080 check
    server mantisdb2 10.0.1.11:8080 check
    server mantisdb3 10.0.1.12:8080 check
```

## Monitoring & Maintenance

### Health Checks

```bash
# Basic health check
curl http://localhost:8080/health

# Detailed status
curl http://localhost:8081/api/health

# Metrics endpoint
curl http://localhost:9090/metrics
```

### Log Management

```bash
# View logs
sudo journalctl -u mantisdb -f

# Rotate logs
sudo logrotate /etc/logrotate.d/mantisdb

# Search for errors
sudo grep ERROR /var/log/mantisdb/mantisdb.log
```

### Backup Automation

Create `/usr/local/bin/mantisdb-backup.sh`:

```bash
#!/bin/bash
set -e

BACKUP_DIR="/var/lib/mantisdb/backups"
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="$BACKUP_DIR/mantisdb_backup_$DATE.tar.gz"

# Create backup
tar -czf "$BACKUP_FILE" \
    /var/lib/mantisdb/data \
    /var/lib/mantisdb/wal

# Upload to S3 (optional)
# aws s3 cp "$BACKUP_FILE" s3://your-bucket/mantisdb-backups/

# Clean old backups (keep last 7 days)
find "$BACKUP_DIR" -name "mantisdb_backup_*.tar.gz" -mtime +7 -delete

echo "Backup completed: $BACKUP_FILE"
```

Add to crontab:

```bash
sudo crontab -e
# Add: 0 2 * * * /usr/local/bin/mantisdb-backup.sh
```

### Performance Monitoring

```bash
# Install monitoring tools
sudo apt-get install prometheus grafana

# Configure Prometheus scraping
# Add to prometheus.yml:
scrape_configs:
  - job_name: 'mantisdb'
    static_configs:
      - targets: ['localhost:9090']
```

## Troubleshooting

### Common Issues

#### Issue: Service won't start

```bash
# Check logs
sudo journalctl -u mantisdb -n 50

# Verify configuration
mantisdb --config /etc/mantisdb/config.yaml --validate

# Check permissions
ls -la /var/lib/mantisdb
```

#### Issue: High memory usage

```bash
# Check memory stats
curl http://localhost:8081/api/stats | jq '.memory'

# Reduce cache size
# Edit /etc/mantisdb/config.yaml
performance:
  cache_size: 536870912  # 512MB

# Restart service
sudo systemctl restart mantisdb
```

#### Issue: Connection refused

```bash
# Check if service is running
sudo systemctl status mantisdb

# Check listening ports
sudo netstat -tlnp | grep mantisdb

# Check firewall
sudo ufw status
sudo ufw allow 8080/tcp
sudo ufw allow 8081/tcp
```

#### Issue: Slow queries

```bash
# Enable query logging
# Edit /etc/mantisdb/config.yaml
logging:
  log_slow_queries: true
  slow_query_threshold: 100ms

# Analyze slow queries
grep "SLOW QUERY" /var/log/mantisdb/mantisdb.log
```

### Debug Mode

```bash
# Run in debug mode
MANTIS_LOG_LEVEL=debug mantisdb --config /etc/mantisdb/config.yaml

# Enable profiling
MANTIS_ENABLE_PROFILING=true mantisdb --config /etc/mantisdb/config.yaml

# Access pprof
go tool pprof http://localhost:6060/debug/pprof/profile
```

## Security Best Practices

### 1. Enable TLS/SSL

```bash
# Generate certificate
sudo certbot certonly --standalone -d yourdomain.com

# Update config
sudo nano /etc/mantisdb/config.yaml
# Set:
security:
  enable_tls: true
  tls_cert: "/etc/letsencrypt/live/yourdomain.com/fullchain.pem"
  tls_key: "/etc/letsencrypt/live/yourdomain.com/privkey.pem"
```

### 2. Configure Firewall

```bash
# Allow only necessary ports
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow ssh
sudo ufw allow 8080/tcp
sudo ufw allow 8081/tcp
sudo ufw allow 443/tcp
sudo ufw enable
```

### 3. Set Up Authentication

```bash
# Change default admin password
curl -X POST http://localhost:8081/api/auth/change-password \
  -H "Content-Type: application/json" \
  -d '{"old_password": "admin123", "new_password": "your-secure-password"}'
```

### 4. Enable Rate Limiting

Edit `/etc/mantisdb/config.yaml`:

```yaml
security:
  rate_limit: 1000  # requests per second
  enable_cors: true
  cors_origins:
    - "https://yourdomain.com"
```

## Scaling Strategies

### Vertical Scaling

```bash
# Increase resources
# Edit /etc/mantisdb/config.yaml
performance:
  max_connections: 2000
  pool_size: 200
  cache_size: 2147483648  # 2GB
  worker_threads: 16
```

### Horizontal Scaling

```bash
# Deploy multiple instances behind load balancer
# Use shared storage for data consistency
# Configure replication (if supported)
```

## Upgrade Process

```bash
# 1. Backup current installation
sudo /usr/local/bin/mantisdb-backup.sh

# 2. Download new version
wget https://github.com/mantisdb/mantisdb/releases/download/v1.1.0/mantisdb-linux-amd64.tar.gz

# 3. Stop service
sudo systemctl stop mantisdb

# 4. Replace binary
sudo tar -xzf mantisdb-linux-amd64.tar.gz
sudo cp mantisdb-linux-amd64/mantisdb /usr/local/bin/

# 5. Start service
sudo systemctl start mantisdb

# 6. Verify
curl http://localhost:8080/health
mantisdb --version
```

## Support

- **Documentation**: https://github.com/mantisdb/mantisdb/docs
- **Issues**: https://github.com/mantisdb/mantisdb/issues
- **Community**: https://discord.gg/mantisdb
- **Email**: support@mantisdb.io

---

**Last Updated**: 2025-10-08  
**Version**: 1.0.0
