#!/bin/bash

# Script to create tables for NetWeather application
# Usage: ./create_tables.sh

echo "NetWeather Table Creation"
echo "========================"
echo ""

# Default database credentials
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-3306}"
DB_USER="${DB_USER:-netweather}"
DB_PASSWORD="${DB_PASSWORD:-netweather}"
DB_NAME="${DB_NAME:-netweather}"

# Check if MySQL is available
if ! command -v mysql &> /dev/null; then
    echo "Error: MySQL client is not installed or not in PATH"
    exit 1
fi

echo "Using database connection:"
echo "  Host: $DB_HOST:$DB_PORT"
echo "  Database: $DB_NAME"
echo "  User: $DB_USER"
echo ""

# Execute SQL script
echo "Creating tables..."
mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASSWORD" "$DB_NAME" < create_tables.sql

if [ $? -eq 0 ]; then
    echo ""
    echo "✓ Tables created successfully!"
else
    echo ""
    echo "✗ Error creating tables."
    echo "Please check your database credentials and try again."
    echo ""
    echo "You can set credentials using environment variables:"
    echo "  export DB_HOST=localhost"
    echo "  export DB_PORT=3306"
    echo "  export DB_USER=netweather"
    echo "  export DB_PASSWORD=netweather"
    echo "  export DB_NAME=netweather"
    exit 1
fi