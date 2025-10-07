# Automatic Port Management

## Overview

MantisDB now automatically finds available ports if the default ports are in use. This allows running multiple instances on the same machine without conflicts.

## How It Works

### Default Ports
- **API Server**: 8080
- **Admin Dashboard**: 8081

### Automatic Fallback

If ports are in use, MantisDB automatically tries alternative ports:

**API Server** (increment by 1):
- 8080 → 8081 → 8082 → 8083 → ... (up to 10 attempts)

**Admin Server** (increment by 2):
- 8081 → 8083 → 8085 → 8087 → ... (up to 10 attempts)

The admin server uses increment of 2 to avoid collisions with the API server.

## Examples

### Single Instance
```bash
./mantisdb
```
Output:
```
✓ MantisDB started successfully
  Admin: http://localhost:8081
  API:   http://localhost:8080/api/v1/
```

### Multiple Instances
```bash
# Terminal 1
./mantisdb
# Admin: 8081, API: 8080

# Terminal 2
./mantisdb
# Admin: 8083, API: 8082

# Terminal 3
./mantisdb
# Admin: 8085, API: 8084
```

## Port Selection Logic

```
Instance 1:
  API:   8080 (default)
  Admin: 8081 (default)

Instance 2:
  API:   8082 (8080+2, skipping 8081 which is admin)
  Admin: 8083 (8081+2)

Instance 3:
  API:   8084 (8080+4)
  Admin: 8085 (8081+4)
```

## Custom Ports

You can still specify custom ports:

```bash
./mantisdb --port=9000 --admin-port=9001
```

If those ports are in use, it will automatically try:
- API: 9000 → 9001 → 9002 → ...
- Admin: 9001 → 9003 → 9005 → ...

## Verification

Check which ports are in use:

```bash
# List all MantisDB instances
ps aux | grep mantisdb

# Check specific ports
lsof -i :8080
lsof -i :8081
lsof -i :8082
lsof -i :8083

# Test connectivity
curl http://localhost:8080/health
curl http://localhost:8081/api/health
```

## Benefits

✅ **No Manual Configuration** - Just run multiple instances  
✅ **No Port Conflicts** - Automatic detection and fallback  
✅ **Development Friendly** - Easy to test with multiple instances  
✅ **Production Ready** - Handles port exhaustion gracefully  

## Error Handling

If no ports are available after 10 attempts:

```
Error: failed to find available admin port: no available port found after 10 attempts starting from 8081
```

Solution:
1. Stop unused instances
2. Use custom ports: `--port=9000 --admin-port=9001`
3. Increase system port limits

## Implementation Details

### Port Finding Algorithm

```go
func findAvailablePort(startPort int, maxAttempts int) (int, error) {
    for i := 0; i < maxAttempts; i++ {
        port := startPort + i
        listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
        if err == nil {
            listener.Close()
            return port, nil
        }
    }
    return 0, fmt.Errorf("no available port found")
}
```

### Startup Sequence

1. **Storage Engine** initializes
2. **Health Checker** starts
3. **API Server** finds available port (8080+)
4. **Admin Server** finds available port (8081+)
5. **Startup Message** displays actual ports

## Configuration

### Environment Variables
```bash
export MANTIS_API_PORT=8080
export MANTIS_ADMIN_PORT=8081
./mantisdb
```

### Command Line
```bash
./mantisdb --port=8080 --admin-port=8081
```

### Config File
```yaml
server:
  port: 8080
  admin_port: 8081
```

All methods support automatic port fallback.

## Troubleshooting

### Ports still conflict
- Ensure you're using the latest build: `make build`
- Check for other services using the ports: `lsof -i :8080`

### Wrong ports displayed
- The startup message now shows the **actual** ports after fallback
- Wait for the full startup message (includes 500ms delay)

### Can't connect to dashboard
- Check the actual port in the startup message
- Verify with: `curl http://localhost:<port>/api/health`

## Summary

MantisDB now handles port conflicts automatically, making it easy to:
- Run multiple instances for testing
- Develop without port configuration
- Deploy in environments with dynamic port allocation
- Avoid "address already in use" errors

**Just run `./mantisdb` and it works!**
