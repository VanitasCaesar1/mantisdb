# Multi-stage build for MantisDB
FROM node:18-alpine AS frontend-builder

# Build admin frontend
WORKDIR /app/admin/frontend
COPY admin/frontend/package*.json ./
RUN npm ci --only=production

COPY admin/frontend/ ./
RUN npm run build

# Go builder stage
FROM golang:1.21-alpine AS go-builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy built frontend assets
COPY --from=frontend-builder /app/admin/frontend/dist ./admin/assets/dist/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o mantisdb \
    cmd/mantisDB/main.go

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 mantisdb && \
    adduser -D -s /bin/sh -u 1001 -G mantisdb mantisdb

# Create directories
RUN mkdir -p /var/lib/mantisdb /var/log/mantisdb /etc/mantisdb && \
    chown -R mantisdb:mantisdb /var/lib/mantisdb /var/log/mantisdb /etc/mantisdb

# Copy binary
COPY --from=go-builder /app/mantisdb /usr/local/bin/mantisdb

# Copy configuration
COPY configs/production.yaml /etc/mantisdb/config.yaml

# Set permissions
RUN chmod +x /usr/local/bin/mantisdb

# Switch to non-root user
USER mantisdb

# Set working directory
WORKDIR /var/lib/mantisdb

# Expose ports
EXPOSE 8080 8081

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Default command
CMD ["mantisdb", "--config=/etc/mantisdb/config.yaml"]