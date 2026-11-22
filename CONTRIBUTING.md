# Contributing to notion-md-sync

Thank you for your interest in contributing to notion-md-sync! This document provides guidelines and workflows for contributing to the project.

## Development Setup

### Prerequisites
- Go 1.23 or later (`go.mod` requires Go 1.23.0 and toolchain go1.24.0)
- golangci-lint (for linting)
- Make (optional, for using Makefile commands)

### Getting Started
1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/go-notion-md-sync.git
   cd go-notion-md-sync
   ```
3. Install dependencies:
   ```bash
   go mod download
   make dev-setup  # Installs development tools
   ```

## Development Workflow

### Code Style
- Follow standard Go conventions
- Use `gofmt` and `goimports` for formatting
- Run `golangci-lint` before committing
- Write table-driven tests for new functionality

### Making Changes
1. Create a feature branch:
   ```bash
   git checkout -b feature/your-feature-name
   ```
2. Make your changes following the code style guidelines
3. Write or update tests as needed
4. Run tests and linting:
   ```bash
   make test
   make lint
   # or manually:
   go test ./...
   golangci-lint run
   ```
5. Commit your changes with clear, descriptive messages

### Submitting Changes
1. Push to your fork:
   ```bash
   git push origin feature/your-feature-name
   ```
2. Open a Pull Request against the `main` branch
3. Ensure all CI checks pass
4. Wait for code review and address any feedback

## Testing

### Running Tests
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific package tests
go test ./pkg/sync
go test ./pkg/notion
go test ./pkg/cli
```

### Writing Tests
- Follow table-driven test patterns
- Mock external dependencies (Notion API, filesystem)
- Test both success and error cases
- Aim for high test coverage on new code

## CI/CD Pipeline

### Continuous Integration
Pushes to `main` and pull requests targeting `main` trigger `.github/workflows/ci.yml`, which:
- Runs `go test -race ./...`
- Checks `gofmt` and `go vet`
- Runs golangci-lint
- Requests Go 1.23 (`go.mod` then selects toolchain go1.24.0)
- Runs on Ubuntu

### Release Process
Publishing a GitHub release starts `.github/workflows/release.yml`. Pushing a tag alone does not.

1. **For maintainers**:
   - Bump `Version` in `pkg/cli/root.go` and add `docs/releases/vX.Y.Z.md`
   - Merge to `main` after CI is green
   - Tag that commit and push the tag
   - Publish the GitHub release from the tag. That runs the workflow

2. **Binary building**:
   - Triggered when a release is published, or by manual dispatch
   - Linux and macOS: amd64 and arm64. Windows: amd64
   - Attaches archives to the release. The workflow does not run the tests

## Project Structure

```
notion-md-sync/
├── cmd/notion-md-sync/    # CLI entry point
├── pkg/
│   ├── cli/              # Command implementations
│   ├── config/           # Configuration handling
│   ├── notion/           # Notion API client
│   ├── markdown/         # Markdown processing
│   ├── sync/             # Core sync logic
│   ├── staging/          # Git-like staging
│   └── watcher/          # File watching
├── internal/             # Private utilities
├── docs/                 # Documentation
│   └── releases/         # Release notes
└── scripts/              # Helper scripts
```

## Guidelines

### Do's
- Write clear, self-documenting code
- Add tests for new functionality
- Update documentation as needed
- Keep commits focused and atomic
- Use meaningful commit messages

### Don'ts
- Don't commit credentials or tokens
- Don't skip tests or linting
- Don't make breaking changes without discussion
- Don't commit generated files

## Getting Help

- Check existing issues and discussions
- Review the documentation in `/docs`
- Ask questions in GitHub Discussions
- Reference CLAUDE.md for project-specific guidelines

## Code of Conduct

- Be respectful and inclusive
- Welcome newcomers and help them get started
- Focus on constructive feedback
- Assume good intentions

Thank you for contributing to notion-md-sync!
