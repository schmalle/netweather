# NetWeather Project Documentation for Claude

## Project Overview

NetWeather is a Go command-line application that scans websites for embedded JavaScript libraries, computes their checksums, and identifies them using an external API. The primary use case is security monitoring and dependency tracking for web applications.

### Key Features
- Checks URL reachability via HTTP and HTTPS before scanning
- Handles URLs without protocol prefix (automatically tests both HTTP/HTTPS)
- Follows redirects and stores redirect information
- Records HTTP/HTTPS status codes and availability
- Parses command-line arguments for URL input files
- Scans HTML pages for `<script>` tags with external JavaScript sources
- Computes SHA-256 checksums of JavaScript libraries
- Identifies libraries using the publicdata.guru API
- Stores scan results and reachability data in a MySQL/MariaDB database
- Provides comprehensive logging and error handling
- Clean command-line output with progress feedback

## Project Structure

```
/Users/flake/sources/go/src/netweather/
├── main.go              # Main application entry point and URL scanning logic
├── database.go          # Database connection, table creation, and data storage
├── api.go              # External API integration for library identification
├── logger.go           # Logging configuration and utilities
├── nmap.go             # Docker/NMAP integration for port scanning
├── reachability.go     # URL reachability checking with HTTP/HTTPS support
├── go.mod              # Go module dependencies
├── go.sum              # Dependency checksums
├── urls.txt            # Sample input file with URLs to scan
├── netweather          # Compiled binary executable
├── netweather.log      # Application log file
├── docker-compose.yml  # Docker orchestration for NMAP service
├── cmd/
│   └── nmap-scanner/   # NMAP scanner service
│       ├── main.go     # REST API service for container
│       └── nmap-scanner # Compiled scanner binary
├── docker/
│   ├── README.md       # Docker integration documentation
│   └── nmap-scanner/
│       └── Dockerfile  # Container definition for NMAP service
├── scripts/            # All management and utility scripts
│   ├── README.md       # Script documentation
│   ├── build-nmap-container.sh  # Container management
│   ├── create_database_user.sh  # Database setup
│   ├── create_database_user.sql
│   ├── create_tables.sh          # Table creation
│   ├── create_tables.sql
│   ├── delete_all_entries.sh     # Data cleanup
│   ├── delete_all_entries.sql
│   ├── test_nmap.sh              # NMAP integration tests
│   └── test_stats.sh             # Statistics functionality tests
├── README.md           # Basic project description
└── LICENSE             # Apache License 2.0
```

## Core Components

### 1. Main Application (`main.go`)
- **Entry point**: Handles command-line arguments and orchestrates the scanning process
- **URL scanning**: Fetches web pages and parses HTML for script tags
- **Script processing**: Downloads JavaScript files and computes SHA-256 checksums
- **URL resolution**: Converts relative script URLs to absolute URLs
- **Data flow**: Coordinates between scanning, API calls, and database storage

### 2. Database Layer (`database.go`)
- **Connection management**: MySQL/MariaDB database connectivity
- **Schema**: 
  - `scan_results` table: Stores JavaScript library scan results
  - `url_reachability` table: Stores HTTP/HTTPS availability and redirect information
  - `nmap_batches` table: Stores port scanning results
- **Data models**: 
  - `ScanResult` struct for JavaScript library results
  - `URLReachability` struct for reachability data
- **Storage**: Persistent storage of all scan results with timestamps
- **Statistics**: Functions for retrieving scan and reachability statistics

### 3. External API Integration (`api.go`)
- **Library identification**: Uses publicdata.guru API to identify JavaScript libraries by checksum
- **API endpoint**: `https://api.publicdata.guru/v1/checksums/{checksum}`
- **Response handling**: Parses JSON responses and extracts library names
- **Fallback**: Returns "Unknown" for unidentified libraries

### 4. Logging System (`logger.go`)
- **File-based logging**: All events logged to `netweather.log`
- **Log format**: Timestamped entries with "netweather:" prefix
- **Coverage**: Application lifecycle, URL processing, errors, and API responses

### 5. URL Reachability Checker (`reachability.go`)
- **Protocol checking**: Tests URLs for HTTP and HTTPS availability
- **Redirect handling**: Follows redirects and records redirect chains
- **Smart URL parsing**: Handles URLs without protocol prefix by testing both variants
- **Status tracking**: Records HTTP status codes for each protocol
- **Timeout management**: Uses 10-second timeout for connection attempts
- **Data model**: `URLReachability` struct for storing reachability information

## Development Workflow

### Git Workflow
- **Main branch**: `main` (primary branch for PRs)
- **Development branch**: `devel` (current active branch)
- **Remote tracking**: Origin remote with both branches

### Build Process
```bash
# Build the application
go build -o netweather

# Run with a URL list file
./netweather urls.txt
```

### Database Setup
Before running the application, ensure:
1. MySQL/MariaDB server is running
2. Update the database connection string in `main.go` line 22:
   ```go
   initDB("user:password@tcp(127.0.0.1:3306)/database")
   ```
3. The application will automatically create the required `scan_results` table

## Usage Patterns

### Basic Usage
```bash
./netweather <url_file>
```

### Input Format
Create a text file with one URL per line:
```
http://google.com
https://www.github.com
https://example.com
```

### Output Example
```
NetWeather - URL Scanner
Scanning URL: http://google.com
  - Found script: https://example.com/script.js, Checksum: abc123...
    Library: jQuery 3.6.0
```

## Dependencies

### Go Modules
- **Standard library**: `net/http`, `html`, `crypto/sha256`, `database/sql`
- **External dependencies**:
  - `github.com/go-sql-driver/mysql v1.9.3` - MySQL driver
  - `golang.org/x/net v0.42.0` - HTML parsing
  - `filippo.io/edwards25519 v1.1.0` - Cryptographic dependency

### External Services
- **publicdata.guru API**: Free service for JavaScript library identification
- **MySQL/MariaDB**: Database for persistent storage

## Security Considerations

### Network Requests
- The application makes HTTP requests to scan target URLs
- Downloads JavaScript files for checksum computation
- Queries external API for library identification

### Database Security
- Uses parameterized queries to prevent SQL injection
- Database credentials currently hardcoded (TODO: move to environment variables)

### Input Validation
- Validates URL formats and handles malformed URLs gracefully
- Error handling for network failures and invalid responses

## Common Development Tasks

### Adding New Features
1. Most business logic should go in `main.go` for core scanning features
2. Database-related changes go in `database.go`
3. New external API integrations go in `api.go`
4. Update logging in `logger.go` if new log patterns are needed

### Debugging
- Check `netweather.log` for detailed application logs
- Use `go run .` for development builds
- Database issues: verify connection string and table schema

### Testing
- Create test URL files for different scenarios
- Test with various website structures and JavaScript libraries
- Verify database connectivity and data persistence

## Configuration Notes

### Database Configuration
- Connection string format: `user:password@tcp(host:port)/database`
- Automatic table creation on first run
- Timestamps in UTC using MySQL CURRENT_TIMESTAMP

### Logging Configuration
- Log file: `netweather.log` in current directory
- Append mode: logs accumulate across runs
- Format: `netweather: YYYY/MM/DD HH:MM:SS message`

## Potential Improvements

Based on the current implementation, future enhancements could include:
- Environment variable configuration for database credentials
- Command-line flags for output formats and verbosity
- Concurrent processing of multiple URLs
- Rate limiting for external API calls
- Support for additional JavaScript library identification services
- Web interface for viewing scan results
- Scheduled scanning capabilities

## License
Apache License 2.0 - See LICENSE file for full terms.