# NetWeather Database Scripts

This directory contains scripts to set up and manage the NetWeather database.

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

All scripts support these environment variables:
- `DB_HOST` (default: localhost)
- `DB_PORT` (default: 3306)
- `DB_USER` (default: netweather)
- `DB_PASSWORD` (default: netweather)
- `DB_NAME` (default: netweather)