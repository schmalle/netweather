#!/bin/bash

echo "Testing NetWeather statistics feature"
echo "====================================="
echo ""

# Test 1: Help output includes stats option
echo "Test 1: Checking help output..."
./netweather -h 2>&1 | grep -q "stats" && echo "✓ Stats flag is in help output" || echo "✗ Stats flag missing from help"

# Test 2: Stats flag requires database credentials
echo ""
echo "Test 2: Testing stats without database credentials..."
output=$(./netweather -stats 2>&1)
if echo "$output" | grep -q "Database user and name must be provided"; then
    echo "✓ Stats correctly requires database credentials"
else
    echo "✗ Stats should require database credentials"
fi

# Test 3: Regular mode still works
echo ""
echo "Test 3: Testing regular mode without database..."
./netweather urls.txt > /dev/null 2>&1 && echo "✓ Regular mode works without database" || echo "✗ Regular mode failed"

echo ""
echo "All tests completed!"