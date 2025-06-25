# CLAUDE_go.md - Guidelines for notion-md-sync (Go Implementation)

## Commands
- Build: `go build -o notion-md-sync ./cmd/notion-md-sync`
- Dev/Run: `go run ./cmd/notion-md-sync`
- Test: `go test ./...`
- Single test: `go test -run TestName ./pkg/package`
- Lint: `golangci-lint run`
- Format: `go fmt ./...`
- Mod tidy: `go mod tidy`

## Code Style
- **Formatting**: Use `gofmt` and `goimports`
- **Naming**: Follow Go conventions (PascalCase for exported, camelCase for unexported)
- **Packages**: Short, lowercase names without underscores
- **Error Handling**: Always check and handle errors explicitly
- **Interfaces**: Keep them small and focused
- **Documentation**: Use godoc comments for exported functions
- **Tests**: Use table-driven tests where appropriate

## Project Structure
```
notion-md-sync/
├── cmd/
│   └── notion-md-sync/           # Main application entry point
│       └── main.go
├── pkg/
│   ├── config/                   # Configuration management
│   │   ├── config.go
│   │   └── config_test.go
│   ├── notion/                   # Notion API client
│   │   ├── client.go
│   │   ├── client_test.go
│   │   └── types.go
│   ├── markdown/                 # Markdown processing
│   │   ├── parser.go
│   │   ├── parser_test.go
│   │   └── frontmatter.go
│   ├── sync/                     # Core sync logic & conflict resolution
│   │   ├── engine.go
│   │   ├── engine_test.go
│   │   ├── converter.go
│   │   ├── conflict.go           # Conflict resolution with diff display
│   │   └── conflict_test.go
│   ├── staging/                  # Git-like staging area
│   │   ├── staging.go
│   │   └── staging_test.go
│   ├── watcher/                  # File system monitoring
│   │   ├── watcher.go
│   │   └── watcher_test.go
│   └── cli/                      # Command line interface
│       ├── root.go
│       ├── sync.go
│       ├── pull.go
│       ├── push.go
│       ├── add.go               # Git-like staging commands
│       ├── reset.go
│       ├── status.go
│       └── watch.go
├── internal/                     # Private application code
│   └── util/                     # Internal utilities
├── configs/
│   └── config.example.yaml
├── go.mod
├── go.sum
├── README.md
└── Makefile
```

## Session Memories

### Release and Update Processes
- Always update session_memory.md and CLAUDE.md after performing a release
- This ensures documentation is consistently tracked across project versions
- Capture key changes, improvements, and notable modifications in each release cycle

## GitHub Workflows

### CI/CD Pipeline
- **CI workflow** (`ci.yml`): Runs on all pushes and PRs to main branch
  - Executes tests with `go test ./...`
  - Runs linting with `golangci-lint`
  - Validates code quality before merge

- **Release workflow** (`release.yml`): Triggers only on version tags (`v*`)
  - Builds binaries for multiple platforms (Linux, Windows, macOS)
  - Supports both amd64 and arm64 architectures
  - Creates GitHub releases with artifacts
  - Uses release notes from `docs/releases/vX.Y.Z.md`

### Release Process
1. **Write Release Notes**: Create `docs/releases/vX.Y.Z.md` with changelog
2. **Tag the Version**: 
   ```bash
   git tag vX.Y.Z
   git push origin vX.Y.Z
   ```
3. **Automated Release**: GitHub Actions will:
   - Run all tests
   - Build cross-platform binaries
   - Create GitHub release using your markdown notes
   - Upload binary artifacts (.tar.gz for Unix, .zip for Windows)

### Development Workflow
- Push to main or create PRs → CI runs tests/linting
- Tag with version → Release workflow builds and publishes
- No binaries built on regular commits (only on tags)
