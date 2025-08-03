#!/bin/bash

# NetWeather Start Script
# This script builds and runs the NetWeather application

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}NetWeather Start Script${NC}"
echo "=========================="

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo -e "${RED}Error: Go is not installed or not in PATH${NC}"
    exit 1
fi

# Build the application
echo -e "${YELLOW}Building NetWeather...${NC}"
if go build -o netweather; then
    echo -e "${GREEN}Build successful!${NC}"
else
    echo -e "${RED}Build failed!${NC}"
    exit 1
fi

# Make the binary executable
chmod +x netweather

# Run the application with provided arguments
echo -e "${YELLOW}Starting NetWeather...${NC}"
echo ""

# Pass all arguments to the netweather binary
./netweather "$@"