#!/bin/bash

# Validation script for notion-md-sync setup

echo "🔍 Validating notion-md-sync setup..."
echo

# Check if binary exists
if [ ! -f "bin/notion-md-sync" ]; then
    echo "❌ Binary not found. Run 'make build' first."
    exit 1
fi
echo "✅ Binary found: bin/notion-md-sync"

# Check if .env file exists
if [ ! -f ".env" ]; then
    echo "❌ .env file not found. Copy .env.example to .env and fill in your values."
    exit 1
fi
echo "✅ .env file found"

# Load environment variables
source .env

# Check if required environment variables are set
if [ -z "$NOTION_MD_SYNC_NOTION_TOKEN" ]; then
    echo "❌ NOTION_MD_SYNC_NOTION_TOKEN not set in .env"
    exit 1
fi
echo "✅ NOTION_MD_SYNC_NOTION_TOKEN is set"

if [ -z "$NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID" ]; then
    echo "❌ NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID not set in .env"
    exit 1
fi
echo "✅ NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID is set"

# Check if config file exists
if [ ! -f "config/config.yaml" ]; then
    echo "⚠️  config/config.yaml not found. Using default configuration."
else
    echo "✅ Config file found: config/config.yaml"
fi

# Check if docs directory exists
if [ ! -d "docs" ]; then
    echo "ℹ️  Creating docs directory..."
    mkdir -p docs
fi
echo "✅ Docs directory ready"

# Test connection to Notion
echo
echo "🔗 Testing Notion API connection..."

# Try to pull pages (this will test authentication and permissions)
# Make sure environment variables are exported for the subprocess
export NOTION_MD_SYNC_NOTION_TOKEN
export NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID
PULL_OUTPUT=$(./bin/notion-md-sync pull --verbose -c config/config.yaml 2>&1)
PULL_EXIT_CODE=$?

if echo "$PULL_OUTPUT" | grep -q "✓ Pull completed successfully"; then
    echo "✅ Successfully connected to Notion API!"
    echo "✅ Your integration has access to the parent page"
elif echo "$PULL_OUTPUT" | grep -q "notion.token is required"; then
    echo "❌ Authentication failed - token not being read properly"
    exit 1
elif echo "$PULL_OUTPUT" | grep -q "401\|403"; then
    echo "❌ Authentication/permission error"
    echo "   Check that:"
    echo "   - Your token is correct"
    echo "   - You've shared the page with your integration"
    exit 1
elif echo "$PULL_OUTPUT" | grep -q "404"; then
    echo "❌ Page not found"
    echo "   Check that your page ID is correct"
    exit 1
else
    echo "✅ Connected to Notion API (pages may be empty or have formatting issues)"
    echo "ℹ️  Pull output: $PULL_OUTPUT"
fi

echo
echo "🎉 Setup validation complete! You're ready to use notion-md-sync."
echo
echo "Quick commands to try:"
echo "  ./scripts/run-with-env.sh pull --verbose"
echo "  ./scripts/run-with-env.sh push docs/your-file.md --verbose"
echo "  ./scripts/run-with-env.sh watch --verbose"