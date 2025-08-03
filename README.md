# NetWeather

A powerful Go command-line application for scanning websites to identify JavaScript libraries, compute their checksums, and track security dependencies across web applications.

## Features

- 🔍 **Website Scanning**: Parses HTML pages for external JavaScript libraries
- 🔐 **Security Analysis**: Computes SHA-256 checksums for dependency verification
- 📚 **Library Identification**: Identifies JavaScript libraries using the publicdata.guru API
- 🗄️ **Database Storage**: Persistent MySQL/MariaDB storage for scan results
- 🐳 **Port Scanning**: Integrated Docker/NMAP support for network analysis
- 📊 **Statistics**: Comprehensive reporting with `-stats` flag
- 📝 **Logging**: Detailed logging for monitoring and debugging

## Quick Start

### Prerequisites

- Go 1.19 or higher
- MySQL or MariaDB server
- Docker (optional, for NMAP integration)

### Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd netweather
```

2. Build the application:
```bash
go build -o netweather
```

3. Set up your database and update the connection string in `main.go:22`:
```go
initDB("user:password@tcp(127.0.0.1:3306)/database")
```

### Basic Usage

1. Create a file with URLs to scan (one per line):
```
https://example.com
https://github.com
https://google.com
```

2. Run the scanner:
```bash
./netweather urls.txt
```

3. View statistics:
```bash
./netweather -stats
```

### Sample Output

```
NetWeather - URL Scanner
Scanning URL: https://example.com
  - Found script: https://code.jquery.com/jquery-3.6.0.min.js
    Checksum: sha256-/xUj+3OJU5yExlq6GSYGSHk7tPXikynS7ogEvDej/m4=
    Library: jQuery 3.6.0
Database updated successfully
```

## Advanced Features

### Port Scanning with NMAP

NetWeather includes integrated Docker/NMAP support for comprehensive network analysis:

```bash
# Build the NMAP container
./scripts/build-nmap-container.sh

# Test NMAP integration
./scripts/test_nmap.sh
```

### Database Management

Use the provided scripts for database operations:

```bash
# Create database tables
./scripts/create_tables.sh

# Create database user
./scripts/create_database_user.sh

# Clean up all entries
./scripts/delete_all_entries.sh
```

### Statistics and Reporting

```bash
# View comprehensive statistics
./netweather -stats

# Test statistics functionality
./scripts/test_stats.sh
```

## Project Structure

```
├── main.go              # Main application and URL scanning logic
├── database.go          # Database operations and schema management
├── api.go              # External API integration (publicdata.guru)
├── logger.go           # Logging configuration and utilities
├── nmap.go             # Docker/NMAP integration
├── cmd/
│   └── nmap-scanner/   # NMAP REST API service
├── docker/
│   └── nmap-scanner/   # Docker container definitions
├── scripts/            # Management and utility scripts
│   ├── build-nmap-container.sh
│   ├── create_database_user.sh
│   ├── create_tables.sh
│   ├── delete_all_entries.sh
│   ├── test_nmap.sh
│   └── test_stats.sh
└── README.md
```

## Database Schema

The application uses a simple but effective schema:

```sql
CREATE TABLE scan_results (
    id INT AUTO_INCREMENT PRIMARY KEY,
    url VARCHAR(255) NOT NULL,
    script_url VARCHAR(255) NOT NULL,
    checksum VARCHAR(64) NOT NULL,
    library_name VARCHAR(255),
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## Dependencies

- **Core**: Standard Go libraries for HTTP, HTML parsing, and cryptography
- **Database**: `github.com/go-sql-driver/mysql` for MySQL connectivity
- **HTML Parsing**: `golang.org/x/net` for robust HTML parsing
- **External API**: publicdata.guru for JavaScript library identification

## Security Considerations

- ✅ Uses parameterized queries to prevent SQL injection
- ✅ Validates URL formats and handles malformed inputs
- ✅ Comprehensive error handling for network failures
- ⚠️ Database credentials currently hardcoded (consider environment variables)

## Configuration

### Database Connection

Update the connection string in `main.go`:
```go
initDB("username:password@tcp(host:port)/database_name")
```

### Logging

Logs are written to `netweather.log` with the format:
```
netweather: 2024/01/15 10:30:45 Scanning URL: https://example.com
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Use Cases

- **Security Auditing**: Track JavaScript dependencies across web applications
- **Compliance Monitoring**: Verify library versions and identify vulnerabilities
- **Asset Discovery**: Catalog JavaScript libraries used in web properties
- **Change Detection**: Monitor for unauthorized script modifications
- **Network Analysis**: Combine with NMAP for comprehensive security assessment

## License

Apache License 2.0 - See [LICENSE](LICENSE) file for details.

## Support

For issues, feature requests, or questions, please open an issue in the repository.
