# NetWeather NMAP Integration

This directory contains Docker integration for NMAP port scanning functionality.

## Overview

The NMAP integration provides:
- Lightweight Docker container with NMAP and NSE scripts
- Go-based REST API service for managing scans
- Batch processing support
- XML result format
- Multi-architecture support (x64 and arm64)

## Architecture

```
NetWeather App → HTTP API → NMAP Scanner Container
                              ├── NMAP binary
                              ├── NSE scripts
                              └── Go API service
```

## Container Features

### NMAP Scanner Service
- **Base Image**: Alpine Linux 3.18
- **Security**: Non-root user execution
- **Health Checks**: Built-in health monitoring
- **Persistence**: Batch and result storage
- **API Port**: 8080

### Supported Operations
1. **Single URL Scan**: Immediate scan of one URL
2. **Batch Upload**: Queue multiple URLs for scanning
3. **Status Query**: Check progress of running batches
4. **Result Download**: Retrieve XML scan results
5. **Batch Listing**: View all batch jobs

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| POST | `/scan` | Single URL scan |
| POST | `/batch` | Create batch scan |
| GET | `/batch/{id}` | Get batch status |
| GET | `/batch/{id}/results` | Download results |
| GET | `/batches` | List all batches |

## Usage

### Building the Container
```bash
# Build manually
docker build -t netweather-nmap ./docker/nmap-scanner

# Or use docker-compose
docker-compose up -d
```

### NetWeather Integration
```bash
# Enable port scanning
./netweather -port-scan urls.txt

# Custom ports
./netweather -port-scan -scan-ports "22,80,443,8080" urls.txt

# Custom NMAP options
./netweather -port-scan -nmap-options "-sV --script=vuln" urls.txt
```

### Default Scan Options
- **Ports**: 80,443,8080,8443 (common web ports)
- **Scan Type**: SYN scan (-sS)
- **Service Detection**: Version detection (-sV)
- **Scripts**: Default and safe NSE scripts

## Data Persistence

The container uses volumes for persistent storage:
- `/app/batches`: Batch metadata (JSON)
- `/app/results`: Scan results (XML)

## Security Considerations

### Container Security
- Runs as non-root user (scanner:1000)
- Minimal attack surface (Alpine base)
- No privileged mode required
- Health check monitoring

### Network Security
- NMAP scans are performed from container
- Results stored in isolated volumes
- API accessible only on localhost:8080

### Scan Safety
- Default to safe NSE scripts only
- Common ports by default
- Configurable scan options
- Timeout protection

## Troubleshooting

### Container Issues
```bash
# Check container status
docker ps -a | grep netweather-nmap

# View logs
docker logs netweather-nmap-scanner

# Health check
curl http://localhost:8080/health
```

### Common Problems
1. **Port 8080 in use**: Change port mapping in docker-compose.yml
2. **Docker not installed**: Install Docker engine
3. **Permission denied**: Ensure Docker daemon is running
4. **Build failures**: Check network connectivity for package downloads

### Manual Container Management
```bash
# Start container
docker run -d --name netweather-nmap-scanner \
  -p 8080:8080 --rm netweather-nmap

# Stop container
docker stop netweather-nmap-scanner

# Remove container
docker rm netweather-nmap-scanner
```

## Database Integration

When database storage is enabled, NetWeather tracks:
- Batch IDs and status
- Scan timestamps
- URL associations
- Statistics for monitoring

## Batch Management

### Batch Lifecycle
1. **Created**: Batch queued for processing
2. **Running**: NMAP scan in progress
3. **Completed**: Results available for download
4. **Failed**: Scan encountered errors

### Batch Persistence
Batches are automatically restored on container restart, allowing long-running scans to continue after system restarts.

## Performance Considerations

- Container startup time: ~5-10 seconds
- Scan duration varies by target and options
- Concurrent batches supported
- Resource usage scales with scan complexity