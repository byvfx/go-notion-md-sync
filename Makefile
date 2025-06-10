.PHONY: build test lint clean install deps fmt run dev-setup

# Binary name
BINARY_NAME=notion-md-sync

# Build the application
build:
	go build -o bin/$(BINARY_NAME) ./cmd/notion-md-sync

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Lint the code
lint:
	golangci-lint run

# Format code
fmt:
	go fmt ./...
	goimports -w .

# Clean build artifacts
clean:
	rm -rf bin/
	go clean

# Install dependencies
deps:
	go mod download
	go mod tidy

# Install the binary
install:
	go install ./cmd/notion-md-sync

# Run the application
run:
	go run ./cmd/notion-md-sync

# Development setup
dev-setup:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest

# Quick build and run
dev: build
	./bin/$(BINARY_NAME)

# Run with environment variables loaded from .env
run-env: build
	./scripts/run-with-env.sh

# Setup environment variables from .env for current shell
source-env:
	@echo "Run this command to load .env variables:"
	@echo "source <(cat .env | grep -v '^#' | sed 's/^/export /')"

# Validate setup and configuration
validate: build
	./scripts/validate-setup.sh

# Complete setup from scratch
setup: deps build
	@echo "Setting up notion-md-sync..."
	@if [ ! -f .env ]; then cp .env.example .env; echo "Created .env file - please edit with your values"; fi
	@if [ ! -f config/config.yaml ]; then cp config/config.template.yaml config/config.yaml; echo "Created config.yaml"; fi
	@mkdir -p docs
	@echo "Setup complete! Edit .env with your Notion token and page ID, then run 'make validate'"