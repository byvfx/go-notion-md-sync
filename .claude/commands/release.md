# Release Command

I will help you prepare and document a new release version $RELEASE for the notion-md-sync project. Here's what I'll do:

## Release Preparation for version $RELEASE

**1. Create Release Documentation**
- Create `/docs/releases/v$RELEASE.md` with detailed changelog
- Document new features, improvements, and bug fixes
- Include breaking changes and migration notes if any

**2. Update Project Documentation**
- Update README.md with latest features and version info
- Update CLAUDE.md with new release notes and session memories
- Update any API documentation or usage examples
- Review and update installation instructions

**3. Update Version References**
- Update version numbers in relevant files
- Update examples and code snippets with latest version
- Review and update any hardcoded version references

**4. Pre-Release Cleanup**
- Remove temporary test files (test_*.go, benchmark_*.go)
- Clean up generated documentation files (.env, temp configs)
- Remove development artifacts (*.tmp, *.backup, debug logs)
- Delete IDE-specific files (.vscode/settings.json if present)
- Clean up any build artifacts in bin/ or dist/ directories
- Remove any performance test outputs or debug files
- Verify .gitignore is properly excluding temporary files

**5. Testing & Code Quality**
- Run full test suite with `go test ./...`
- Execute linting with `golangci-lint run`
- Run code formatting with `go fmt ./...`
- Verify module dependencies with `go mod tidy`
- Check for any test failures or linting issues
- Ensure code coverage meets project standards
- Run final build to verify compilation

**6. Quality Assurance**
- Review all documentation for accuracy
- Ensure all links work correctly
- Verify code examples are up to date
- Check for any outdated information
- Confirm all tests pass and linting is clean

**7. Release Notes**
- Summarize key changes and improvements
- Highlight performance improvements (like our recent concurrent processing)
- Document any breaking changes or migration steps
- Include upgrade instructions

**Context for this release:**
- Recent integration of concurrent processing for significant performance improvements
- 10-minute timeout fix for slow Notion API responses
- Enhanced TUI with proper config integration and progress reporting
- Performance analysis showing ~2x speed improvement for pull operations

## Pre-Release Cleanup Checklist

I will automatically check for and remove the following files/patterns:
- `test_*.go` - Temporary performance test files
- `benchmark_*.go` - Temporary benchmark files  
- `debug_*.log` - Debug log files
- `*.tmp` - Temporary files
- `*.backup` - Backup files
- `.env` - Environment files (keep .env.example)
- `perf_test.md` - Temporary performance test markdown
- Any files in `bin/` or `dist/` directories
- IDE-specific temporary files

## Cleanup Commands I'll Execute:
```bash
# Remove temporary test files
rm -f test_*.go benchmark_*.go debug_*.log *.tmp *.backup perf_test.md

# Clean up build artifacts
rm -rf bin/ dist/

# Remove any accidentally committed .env files (keep .env.example)
rm -f .env

# Verify gitignore is working
git status --porcelain | grep -E "\.(tmp|backup|log)$" || echo "✅ No unwanted files found"
```

## Testing & Quality Commands I'll Execute:
```bash
# Format code
echo "🔧 Formatting Go code..."
go fmt ./...

# Tidy dependencies
echo "📦 Tidying Go modules..."
go mod tidy

# Run full test suite
echo "🧪 Running all tests..."
go test ./... -v

# Run linting
echo "🔍 Running linter..."
golangci-lint run

# Final build verification
echo "🏗️  Verifying build..."
go build -o notion-md-sync ./cmd/notion-md-sync

# Check for any uncommitted changes after formatting
echo "📋 Checking for uncommitted changes..."
git status --porcelain || echo "✅ Repository is clean"

echo "✅ All quality checks completed!"
```

Please confirm you want me to proceed with preparing release $RELEASE documentation, cleanup, and updates.