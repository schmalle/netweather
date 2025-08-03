#!/bin/bash

# Script to create database and user for NetWeather application
# Usage: ./create_database_user.sh

echo "NetWeather Database Setup"
echo "========================="
echo ""
echo "This script will create:"
echo "- Database: netweather"
echo "- User: netweather@localhost"
echo "- Password: netweather"
echo ""

# Check if MySQL is available
if ! command -v mysql &> /dev/null; then
    echo "Error: MySQL client is not installed or not in PATH"
    exit 1
fi

# Prompt for MySQL root password
read -sp "Enter MySQL root password: " ROOT_PASSWORD
echo ""

# Execute SQL script
echo "Creating database and user..."
mysql -u root -p"$ROOT_PASSWORD" < create_database_user.sql

if [ $? -eq 0 ]; then
    echo ""
    echo "✓ Database and user created successfully!"
    echo ""
    echo "You can now connect to the database using:"
    echo "  Host: localhost"
    echo "  Database: netweather"
    echo "  User: netweather"
    echo "  Password: netweather"
else
    echo ""
    echo "✗ Error creating database and user."
    echo "Please check your MySQL root password and try again."
    exit 1
fi