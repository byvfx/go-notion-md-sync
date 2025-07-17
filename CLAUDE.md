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
│   │   ├── converter.go         # Enhanced with math & extended blocks
│   │   ├── converter_test.go
│   │   ├── database.go          # CSV/Database sync functionality
│   │   ├── conflict.go          # Conflict resolution with diff display
│   │   └── conflict_test.go
│   ├── staging/                  # Git-like staging area
│   │   ├── staging.go
│   │   └── staging_test.go
│   ├── watcher/                  # File system monitoring
│   │   ├── watcher.go
│   │   └── watcher_test.go
│   ├── tui/                      # Terminal User Interface
│   │   ├── model.go             # Main TUI application model
│   │   ├── unified.go           # Split-pane unified view
│   │   ├── filelist.go          # File browser component
│   │   ├── syncstatus.go        # Sync status component
│   │   ├── dashboard.go         # Dashboard component
│   │   ├── config.go            # Configuration component
│   │   ├── search.go            # Search component
│   │   ├── conflict.go          # Conflict resolution component
│   │   └── *_test.go            # Comprehensive test suite
│   └── cli/                      # Command line interface
│       ├── root.go
│       ├── sync.go
│       ├── pull.go
│       ├── push.go
│       ├── add.go               # Git-like staging commands
│       ├── reset.go
│       ├── status.go
│       ├── watch.go
│       ├── tui.go               # TUI command
│       └── database.go          # Database export/import commands
├── internal/                     # Private application code
│   └── util/                     # Internal utilities
├── configs/
│   └── config.example.yaml
├── go.mod
├── go.sum
├── README.md
└── Makefile
```

## v0.12.0 Terminal User Interface (Phase 4 Complete)

### TUI Implementation with Bubble Tea
- **New Package**: `pkg/tui` with comprehensive terminal UI components
- **Framework**: Built using Bubble Tea for robust cross-platform UI
- **Architecture**: Proper Model-View-Update (MVU) pattern implementation
- **Split-Pane Design**: File browser and sync status side-by-side interface

### TUI Components
- **Unified View**: Main split-pane interface matching the roadmap mockup
- **File Browser**: Interactive file listing with sync status indicators
- **Sync Status**: Real-time operation monitoring with progress display
- **Navigation**: Full keyboard navigation with tab switching between panes
- **Professional Design**: Clean straight borders and focused pane highlighting

### TUI Features
- **Interactive Selection**: Visual file selection with space/enter navigation
- **Status Indicators**: File status icons (synced, modified, error, pending, conflict)
- **Real-time Updates**: Live sync progress with elapsed time tracking
- **Cross-Platform**: Works on Windows, macOS, and Linux terminals
- **Responsive Layout**: Adapts to terminal size changes

### TUI Usage
- **Command**: `notion-md-sync tui`
- **Navigation**: Tab (switch panes), Arrow keys (navigate), Space (select), 's' (sync), 'q' (quit)
- **Visual Feedback**: Colored borders for focused panes, selection indicators
- **Help Integration**: Keyboard shortcuts displayed in footer

## v0.11.0 Performance Improvements (Phase 3 Complete)

### Concurrent Operations with Worker Pools
- **New Package**: `pkg/concurrent` with robust worker pool implementation
- **Performance Gain**: 9x faster than sequential operations
- **Worker Management**: Configurable worker count (1-50 workers)
- **Job Processing**: Generic Job interface for any operation type
- **Retry Logic**: Built-in exponential backoff retry mechanism
- **Graceful Shutdown**: Clean termination with context cancellation support

### Intelligent Caching Layer
- **New Package**: `pkg/cache` with memory-based caching
- **Performance Gain**: 2.4x faster for repeated operations
- **Cache Interface**: `NotionCache` interface with comprehensive operations
- **Smart Invalidation**: Automatic cache invalidation on data updates
- **LRU Eviction**: Memory-efficient least-recently-used eviction
- **Statistics**: Built-in cache performance monitoring
- **Transparent Integration**: `CachedNotionClient` wrapper with zero API changes

### Advanced Batch Processing
- **Performance Gain**: Combined optimizations provide 58x speed improvement
- **Batch Operations**: `AdvancedBatchProcessor` for bulk operations
- **Intelligent Scheduling**: Priority-based operation scheduling
- **Bulk Sync Manager**: High-level interface for large-scale operations
- **Optimized Batching**: `OptimizedBatch` for mixed workload scenarios
- **Error Handling**: Per-operation error tracking and reporting

### Comprehensive Testing and Benchmarking
- **Test Coverage**: 95%+ coverage for concurrent package, 98%+ for cache package
- **Performance Benchmarks**: 15+ benchmark tests measuring real-world scenarios
- **Memory Profiling**: Detailed memory allocation analysis
- **Scaling Tests**: Worker pool performance across different worker counts
- **Cache Performance**: Comprehensive cache hit/miss scenario testing

### Architecture Enhancements
- **Package Structure**: Clean separation of concerns with new packages
- **Interface Design**: Generic interfaces supporting future extensibility
- **Configuration**: Comprehensive configuration options for tuning
- **Error Handling**: Enhanced error handling with retry logic and timeouts
- **Context Support**: Full context.Context support for cancellation

## v0.11.0 Directory Structure Enhancement

### Enhanced Pull Directory Structure
- **New Structure**: Each Notion page now gets its own directory containing its markdown file
- **Parent Page Inclusion**: Parent page is now pulled along with all descendants
- **Consistent Naming**: Page names preserved exactly as in Notion (including spaces)
- **Example Structure**:
  ```
  docs/
  └── Parent Page/
      ├── Parent Page.md
      ├── Child Page/
      │   ├── Child Page.md
      │   └── Sub Page/
      │       └── Sub Page.md
      └── Another Child/
          └── Another Child.md
  ```

### Implementation Details
- **Modified**: `syncAllNotionToMarkdown` now fetches parent page first
- **Updated**: `buildFilePathForPage` handles parent page specially
- **Benefits**: Better organization, easier navigation, simplified round-trip syncing

## v0.10.1 Bug Fix (Critical)

### Nested Page Pulling Fix
- **Critical Issue**: Pull command was failing to fetch nested sub-pages, causing timeouts
- **Root Cause**: Infinite loop in `buildFilePathForPage` function's safety check logic
- **Solution**: Implemented proper cycle detection using `visited` map for hierarchy traversal
- **Impact**: Now supports deeply nested Notion page structures with proper directory mirroring

### Enhanced Safety Features
- **Cycle Detection**: Prevents infinite loops in complex page hierarchies
- **Missing Parent Handling**: Graceful warnings for orphaned pages
- **Proper Path Construction**: Accurate nested directory structure creation
- **Timeout Prevention**: Eliminated blocking operations during recursive page fetching

## v0.10.0 Features (Phase 2 Complete)

### Extended Block Support
- **EquationBlock**: LaTeX math equations with `$$` delimiters
- **Enhanced Images**: Full caption and external URL support
- **Callouts**: Blockquotes with emoji prefixes
- **Toggles**: Collapsible sections via HTML details/summary
- **Bookmarks**: Link blocks with rich preview
- **Dividers**: Horizontal rule conversion

### Database Integration
- **DatabaseSync interface**: Export/import CSV functionality
- **Smart type inference**: Automatic property type detection
- **NotionDate type**: Flexible date parsing for multiple formats
- **Select properties**: Dropdown field support

### Enhanced Converter
- **Math block extraction**: Pre-processing pipeline for `$$` blocks
- **Placeholder system**: Maintains markdown structure during conversion
- **Improved AST walking**: Better block detection and handling

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
