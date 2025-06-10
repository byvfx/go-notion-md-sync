#!/bin/bash

# Script to run notion-md-sync with environment variables loaded from .env file

# Change to the script's directory
cd "$(dirname "$0")/.."

# Load environment variables from .env if it exists
if [ -f .env ]; then
    echo "Loading environment variables from .env..."
    export $(cat .env | grep -v '^#' | xargs)
fi

# Run the application with all arguments passed through
./bin/notion-md-sync "$@"