#!/bin/bash

# Test script for URL reachability feature
# This script tests the new URL reachability checking functionality

echo "Testing NetWeather URL Reachability Feature"
echo "=========================================="

# Set test database credentials (update these as needed)
DB_USER="${DB_USER:-netweather}"
DB_PASSWORD="${DB_PASSWORD:-netweather}"
DB_NAME="${DB_NAME:-netweather}"
DB_HOST="${DB_HOST:-127.0.0.1}"
DB_PORT="${DB_PORT:-3306}"

# Create a test URL file with various scenarios
cat << EOF > test_reachability_urls.txt
http://google.com
https://github.com
example.com
www.cloudflare.com
httpstat.us/301
https://httpstat.us/200
http://httpstat.us/404
EOF

echo "Test URLs created in test_reachability_urls.txt"
echo ""

# Run the scanner with database storage
echo "Running NetWeather with URL reachability checking..."
../netweather -db \
    -db-user "$DB_USER" \
    -db-password "$DB_PASSWORD" \
    -db-name "$DB_NAME" \
    -db-host "$DB_HOST" \
    -db-port "$DB_PORT" \
    test_reachability_urls.txt

echo ""
echo "Scan complete!"
echo ""

# Query the database to show results
echo "Querying database for reachability results..."
mysql -u"$DB_USER" -p"$DB_PASSWORD" -h"$DB_HOST" -P"$DB_PORT" "$DB_NAME" << 'EOF'
SELECT 
    original_url,
    http_available,
    https_available,
    http_status_code,
    https_status_code,
    final_url,
    DATE_FORMAT(scanned_at, '%Y-%m-%d %H:%i:%s') as scan_time
FROM url_reachability 
ORDER BY scanned_at DESC 
LIMIT 10;
EOF

# Clean up
rm -f test_reachability_urls.txt

echo ""
echo "Test complete!"