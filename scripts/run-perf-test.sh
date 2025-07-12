#!/bin/bash

# Performance testing script for notion-md-sync
# This script runs various performance tests against a real Notion workspace

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo "🚀 Notion-md-sync Performance Testing Suite"
echo "=========================================="
echo ""

# Check if parent page ID is provided
if [ -z "$1" ]; then
    echo -e "${RED}Error: Parent page ID required${NC}"
    echo "Usage: $0 <parent-page-id> [page-count] [workers]"
    echo ""
    echo "Example: $0 abc123def456 20 10"
    echo ""
    echo "This will create test pages under the specified parent page,"
    echo "run performance tests, and then clean up the test pages."
    exit 1
fi

PARENT_PAGE_ID=$1
PAGE_COUNT=${2:-20}  # Default to 20 pages
WORKERS=${3:-10}     # Default to 10 workers

# Check if NOTION_MD_SYNC_NOTION_TOKEN is set
if [ -z "$NOTION_MD_SYNC_NOTION_TOKEN" ]; then
    echo -e "${RED}Error: NOTION_MD_SYNC_NOTION_TOKEN environment variable not set${NC}"
    echo "Please set your Notion API token first."
    exit 1
fi

# Build the performance test tool
echo -e "${YELLOW}Building performance test tool...${NC}"
go build -o bin/perf-test ./cmd/perf-test

# Create output directory
mkdir -p perf-test-results

# Run tests with different configurations
echo -e "${GREEN}Running performance tests...${NC}"
echo ""

# Small test (10 pages)
echo "Test 1: Small workload (10 pages)"
./bin/perf-test --parent "$PARENT_PAGE_ID" --pages 10 --workers 5 --cache --output perf-test-results/small

echo ""
echo "Waiting 5 seconds before next test..."
sleep 5

# Medium test (default page count)
echo ""
echo "Test 2: Medium workload ($PAGE_COUNT pages)"
./bin/perf-test --parent "$PARENT_PAGE_ID" --pages "$PAGE_COUNT" --workers "$WORKERS" --cache --cache-size 2000 --output perf-test-results/medium

# Large test (optional, only if explicitly requested)
if [ "$4" == "large" ]; then
    echo ""
    echo "Waiting 5 seconds before large test..."
    sleep 5
    
    echo ""
    echo "Test 3: Large workload (100 pages)"
    echo -e "${YELLOW}Warning: This will create 100 test pages and may take several minutes${NC}"
    ./bin/perf-test --parent "$PARENT_PAGE_ID" --pages 100 --workers 15 --cache --cache-size 5000 --batch-size 50 --output perf-test-results/large
fi

echo ""
echo -e "${GREEN}Performance testing complete!${NC}"
echo "Results saved to: perf-test-results/"
echo ""
echo "To run a large test (100 pages), use: $0 $PARENT_PAGE_ID $PAGE_COUNT $WORKERS large"