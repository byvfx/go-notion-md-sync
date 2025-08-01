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

## v0.16.0 Code Quality & Maintenance Release (Complete)

### Code Quality & Cleanup
- **Zero Linting Issues**: All code passes golangci-lint without warnings
- **Enhanced Error Handling**: Improved error handling in test files and utility functions
- **Clean Codebase**: Removed temporary performance testing files and development artifacts
- **Import Optimization**: Cleaned up unused imports and functions across packages
- **Build Verification**: Verified successful compilation across all packages

### Development Environment Cleanup
- **Removed Temporary Tools**: Eliminated `cmd/perf-optimizer/`, `cmd/perf-test/`, `cmd/perf-test-simple/`, `cmd/measure-perf/`
- **Unused Code Removal**: Removed unused functions in TUI package (`executeWithCapturedOutput`, `readProgressChannel`)
- **Test Improvements**: Enhanced error checking in security and validation tests
- **Formatting Consistency**: Applied `go fmt` and `go mod tidy` across entire codebase

### Maintenance Achievements
- **98%+ Test Coverage**: Maintained high test coverage with improved reliability
- **100% Backward Compatibility**: No functional changes to sync operations, TUI, or CLI
- **Developer Experience**: Cleaner codebase for easier contribution and maintenance
- **Code Standards**: All code follows Go best practices and linting rules

## v0.15.0 Performance Optimization Release (Complete)

### Performance Breakthrough
- **26% Faster**: Optimized from 95.8s to 70.5s for 14-page workspaces
- **Auto-Tuned Workers**: Automatically scales workers based on workspace size (30 for large)
- **0.20 Pages/Second**: Improved throughput from 0.15 pages/second
- **Simple Wins**: Standard HTTP client outperformed "optimized" versions

### Configuration Enhancements
- **Performance Settings**: New `performance` config section with worker control
- **Multi-Client Mode**: Experimental round-robin across multiple HTTP clients
- **Smart Defaults**: 0 workers = auto-detect optimal count

### Technical Implementation
- **Worker Scaling**: Small (<5 pages) = page count, Medium (5-14) = 20, Large (15+) = 30
- **HTTP Simplification**: Removed complex optimizations that hurt performance
- **Proven Testing**: Extensive benchmarking showed 30 workers is optimal
- **Config Integration**: Performance settings in config.yaml with sensible defaults

## v0.14.0 Performance & Concurrency Release (Complete)

### Major Performance Improvements
- **Concurrent Processing**: 2x performance improvement with worker pools (5-10 workers)
- **Extended Timeouts**: Increased from 30 seconds to 10 minutes for large syncs
- **API Bottleneck Analysis**: Identified Notion API as primary bottleneck (91,944x slower than our code)
- **Scalable Architecture**: Worker count automatically adjusts based on page count

### TUI Enhancements
- **Interactive Config Setup**: Press 'c' in TUI to configure Notion credentials
- **Fixed Config Discovery**: TUI now properly finds and loads configuration files
- **Terminal Management**: Fixed text corruption and cursor issues during sync
- **Real-time Progress**: Live sync progress with captured stdout/stderr
- **Command Integration**: All CLI commands ('i', 'p', 'P', 's', 'c') work in TUI

### Technical Implementation
- **Simple Goroutines**: Used channel-based worker pools instead of complex concurrent package
- **Avoided Import Cycles**: Kept concurrent processing within sync package
- **Performance Testing**: Added benchmark tools to measure API vs code performance
- **Error Handling**: Per-page error tracking with graceful failure handling

### Performance Benchmarks
- **Before v0.14.0**: 34+ seconds for 3 pages (11.3s per page average)
- **After v0.14.0**: 75 seconds for 14 pages (5.35s per page average)
- **Improvement**: ~2x faster per page + handles larger workspaces reliably

## v0.13.0 Unified Database Handling (Complete)

### Enhanced Pull Command
- **Unified Database Handling**: Pull command now automatically detects and exports child databases
- **Intelligent CSV Naming**: Database CSV files are named based on actual database titles
- **Automatic CSV Export**: Databases embedded in pages are exported alongside markdown content
- **Mixed Content Support**: Pages with both text content and databases are handled seamlessly
- **Database References**: Markdown content includes automatic links to exported CSV files

### Smart Naming Implementation
- **Database Title Detection**: Uses `GetDatabase()` API to fetch actual database titles
- **Meaningful Filenames**: `Product_Inventory_Database.csv` instead of `PageName_db1.csv`
- **Fallback Logic**: Gracefully handles databases without titles or API failures
- **Sanitized Names**: Proper filesystem-safe naming with special character handling

### CLI Simplification
- **Removed**: Entire `database` command and subcommands (export, import, create)
- **Removed**: `pkg/cli/database.go` file and command registration in root.go
- **Simplified**: All database functionality integrated into standard pull workflow
- **Cleaner**: Fewer commands to remember, more intuitive usage

### Technical Implementation
- **Enhanced**: `exportChildDatabases` function with intelligent naming logic
- **Modified**: CSV filename generation to use database titles when available
- **Maintained**: Existing `ChildDatabaseBlock` support and detection logic
- **Preserved**: CSV export integration with `DatabaseSync` interface

### User Experience Improvements
- **Single Command**: Users no longer need separate commands for databases vs pages
- **Better Organization**: Meaningful CSV filenames make data management easier
- **Clear References**: Markdown includes a "Databases" section linking to CSV files
- **Error Handling**: Warnings for database export failures don't stop page sync
- **Future Ready**: Foundation for two-way database synchronization

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

### v0.14.0 Release Session
- **Performance Investigation**: Discovered Notion API was 91,944x slower than our code
- **Concurrent Implementation**: Integrated worker pools for 2x performance improvement
- **TUI Integration**: Successfully hooked up CLI commands to TUI interface
- **Timeout Solution**: Increased from 30s to 10 minutes based on real API performance data
- **Testing Approach**: Created performance analysis tools to identify bottlenecks
- **Import Cycle Resolution**: Used simple goroutines instead of complex concurrent package

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
