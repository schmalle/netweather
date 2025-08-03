#!/bin/bash

# Script to delete all entries from NetWeather database
# Usage: ./delete_all_entries.sh
# WARNING: This will permanently delete all scan results!

echo "NetWeather Data Deletion"
echo "======================="
echo ""
echo "⚠️  WARNING: This will permanently delete ALL scan results!"
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

# Confirmation prompt
read -p "Are you sure you want to delete ALL entries? (yes/no): " CONFIRM

if [ "$CONFIRM" != "yes" ]; then
    echo "Operation cancelled."
    exit 0
fi

# Execute SQL script
echo ""
echo "Deleting all entries..."
mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASSWORD" "$DB_NAME" < delete_all_entries.sql

if [ $? -eq 0 ]; then
    echo ""
    echo "✓ All entries deleted successfully!"
else
    echo ""
    echo "✗ Error deleting entries."
    echo "Please check your database credentials and try again."
    exit 1
fi