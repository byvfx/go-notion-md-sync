#!/bin/bash

# Simple script to measure actual Notion API performance

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo "📊 Notion API Performance Measurement"
echo "===================================="
echo ""

# Check if page ID is provided
if [ -z "$1" ]; then
    echo -e "${RED}Error: Page ID required${NC}"
    echo "Usage: $0 <page-id> [requests]"
    echo ""
    echo "Example: $0 abc123def456 20"
    echo ""
    echo "This will measure API response times for the specified page."
    exit 1
fi

PAGE_ID=$1
REQUESTS=${2:-20}  # Default to 20 requests

# Check if NOTION_MD_SYNC_NOTION_TOKEN is set
if [ -z "$NOTION_MD_SYNC_NOTION_TOKEN" ]; then
    echo -e "${RED}Error: NOTION_MD_SYNC_NOTION_TOKEN environment variable not set${NC}"
    echo "Please set your Notion API token first."
    exit 1
fi

# Build the measurement tool
echo -e "${YELLOW}Building measurement tool...${NC}"
go build -o bin/measure-perf ./cmd/measure-perf

# Run the measurement
echo -e "${GREEN}Measuring API performance...${NC}"
echo ""

./bin/measure-perf --page "$PAGE_ID" --requests "$REQUESTS" --verbose

echo ""
echo -e "${GREEN}Measurement complete!${NC}"