# NetWeather Scripts

This directory contains all scripts for NetWeather including database setup, Docker management, and testing.

## Scripts Overview

### Database Setup

1. **create_database_user.sh** / **create_database_user.sql**
   - Creates the `netweather` database
   - Creates the `netweather` user with password `netweather`
   - Grants all privileges on the netweather database to the user
   - Run as MySQL root/admin user

2. **create_tables.sh** / **create_tables.sql**
   - Creates the `scan_results` table with all necessary columns
   - Adds indexes for better query performance
   - Can be run multiple times safely (uses CREATE TABLE IF NOT EXISTS)

### Data Management

3. **delete_all_entries.sh** / **delete_all_entries.sql**
   - Deletes all entries from the scan_results table
   - Resets the auto-increment counter
   - **WARNING**: This permanently deletes all scan data!

### Docker Management

4. **build-nmap-container.sh**
   - Builds, starts, stops, and manages the NMAP scanner container
   - Supports multiple commands: build, start, stop, status, logs, clean
   - Includes health checking and automatic container management

### Testing Scripts

5. **test_nmap.sh**
   - Comprehensive test suite for NMAP integration
   - Validates Docker files, dependencies, and functionality
   - Tests both flags and container communication

6. **test_stats.sh**
   - Test suite for statistics functionality
   - Validates database requirements and help output
   - Ensures backward compatibility

## Usage

### Initial Setup

1. First, create the database and user:
   ```bash
   cd scripts
   ./create_database_user.sh
   # Enter MySQL root password when prompted
   ```

2. Then create the tables:
   ```bash
   ./create_tables.sh
   # Uses netweather credentials by default
   ```

### Running NetWeather with Database

After setup, you can run NetWeather with database storage:

```bash
# Using environment variables
export DB_USER=netweather
export DB_PASSWORD=netweather
export DB_NAME=netweather
./netweather -db urls.txt

# Or using command line options
./netweather -db -db-user netweather -db-password netweather -db-name netweather urls.txt

# Or using .env file
cp .env.example .env
# Edit .env with your credentials
./netweather -db urls.txt
```

### Clearing Data

To delete all scan results:
```bash
./delete_all_entries.sh
# Type 'yes' when prompted to confirm
```

## Database Schema

The `scan_results` table includes:
- `id` - Auto-incrementing primary key
- `url` - The base URL that was scanned
- `script_url` - The JavaScript file URL found
- `checksum` - SHA-256 checksum of the JavaScript file
- `library_name` - Identified library name from API
- `scanned_at` - Timestamp of the scan
- `date` - Date of the scan (for daily aggregation)

## Environment Variables

All database scripts support these environment variables:
- `DB_HOST` (default: localhost)
- `DB_PORT` (default: 3306)
- `DB_USER` (default: netweather)
- `DB_PASSWORD` (default: netweather)
- `DB_NAME` (default: netweather)

### Docker Container Management

Manage the NMAP scanner container:
```bash
# Build the container
./build-nmap-container.sh build

# Start the service
./build-nmap-container.sh start

# Check status
./build-nmap-container.sh status

# View logs
./build-nmap-container.sh logs

# Stop the service
./build-nmap-container.sh stop

# Clean up (remove container and image)
./build-nmap-container.sh clean
```

### Running Tests

Run the test suites to validate functionality:
```bash
# Test NMAP integration
./test_nmap.sh

# Test statistics functionality
./test_stats.sh
```