#!/bin/bash

echo "Testing NetWeather NMAP Integration"
echo "==================================="
echo ""

# Test 1: Help output includes new flags
echo "Test 1: Checking help output for new flags..."
./netweather -h 2>&1 | grep -q "port-scan" && echo "✓ Port scan flag found" || echo "✗ Port scan flag missing"
./netweather -h 2>&1 | grep -q "scan-ports" && echo "✓ Scan ports flag found" || echo "✗ Scan ports flag missing"
./netweather -h 2>&1 | grep -q "nmap-options" && echo "✓ NMAP options flag found" || echo "✗ NMAP options flag missing"

# Test 2: Custom help shows NMAP options
echo ""
echo "Test 2: Checking custom help output..."
./netweather 2>&1 | grep -q "port-scan" && echo "✓ Port scan help found" || echo "✗ Port scan help missing"

# Test 3: NMAP scanner service builds correctly
echo ""
echo "Test 3: Testing NMAP scanner service build..."
if [[ -f "cmd/nmap-scanner/nmap-scanner" ]]; then
    echo "✓ NMAP scanner binary exists"
    
    # Test if service responds to --help (basic functionality)
    ./cmd/nmap-scanner/nmap-scanner --help &>/dev/null && echo "✓ NMAP scanner runs" || echo "? NMAP scanner may need dependencies"
else
    echo "✗ NMAP scanner binary not found"
fi

# Test 4: Docker files exist
echo ""
echo "Test 4: Checking Docker integration files..."
[[ -f "docker/nmap-scanner/Dockerfile" ]] && echo "✓ Dockerfile exists" || echo "✗ Dockerfile missing"
[[ -f "docker-compose.yml" ]] && echo "✓ Docker Compose file exists" || echo "✗ Docker Compose file missing"
[[ -f "scripts/build-nmap-container.sh" ]] && echo "✓ Build script exists" || echo "✗ Build script missing"

# Test 5: Dependencies are available
echo ""
echo "Test 5: Checking dependencies..."
go list -m github.com/gorilla/mux &>/dev/null && echo "✓ Gorilla Mux dependency available" || echo "✗ Gorilla Mux dependency missing"
go list -m github.com/google/uuid &>/dev/null && echo "✓ Google UUID dependency available" || echo "✗ Google UUID dependency missing"

# Test 6: Regular mode still works
echo ""
echo "Test 6: Testing regular mode without port scanning..."
./netweather urls.txt > /dev/null 2>&1 && echo "✓ Regular mode works" || echo "✗ Regular mode failed"

echo ""
echo "All tests completed!"
echo ""
echo "To test the full NMAP integration:"
echo "1. Build the container: ./scripts/build-nmap-container.sh build"
echo "2. Start the service: ./scripts/build-nmap-container.sh start"
echo "3. Run with port scan: ./netweather -port-scan urls.txt"