# notion-md-sync

[![CI Status](https://github.com/byvfx/go-notion-md-sync/workflows/CI/badge.svg)](https://github.com/byvfx/go-notion-md-sync/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/byvfx/go-notion-md-sync)](https://goreportcard.com/report/github.com/byvfx/go-notion-md-sync)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![GitHub release](https://img.shields.io/github/release/byvfx/go-notion-md-sync.svg)](https://github.com/byvfx/go-notion-md-sync/releases)
[![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.21-blue)](https://golang.org)

A powerful CLI tool for synchronizing markdown files with Notion pages. Built with Go for fast, reliable bidirectional sync between your local markdown files and Notion workspace.

## Features

- **Terminal User Interface (TUI)**: Interactive split-pane interface for visual file management and sync monitoring
- **Bidirectional Sync**: Push markdown to Notion or pull Notion pages to markdown
- **Git-like Staging**: Stage files for sync with `add`, `status`, and `reset` commands
- **Frontmatter Support**: Automatic metadata management with YAML frontmatter
- **Smart Change Detection**: Hybrid timestamp and content-based change tracking
- **File Watching**: Real-time auto-sync when files change
- **Secure Configuration**: Environment variable support for API tokens
- **Flexible Mapping**: Choose between filename or frontmatter-based page mapping
- **High Performance**: 2-6x faster sync with concurrent operations, caching, and batch processing
- **Comprehensive Testing**: Full test coverage with CI/CD validation
- **Fast & Reliable**: Built with Go for performance and reliability
- **Configuration Verification**: Check your setup is ready with `verify` command
- **Enhanced Pull Information**: See page titles and progress when pulling from Notion
- **Parent Page Context**: Status command shows current Notion parent page title
- **Table Support**: Full bidirectional sync of Notion tables to markdown tables
- **Single File Pull**: Pull specific pages by filename with `--page` flag
- **Extended Block Support**: Images, callouts, toggles, bookmarks, dividers, and more
- **LaTeX Math Equations**: Full support for mathematical expressions with `$$` blocks
- **Mermaid Diagrams**: Preserve and sync Mermaid diagram code blocks
- **CSV/Database Integration**: Export Notion databases to CSV and import CSV to databases
- **Enhanced Markdown**: Advanced formatting with proper caption and metadata handling
- **Nested Page Support**: Pull command creates proper directory hierarchy mirroring Notion page structure

## Quick Start

> **New to this?** Use our automated setup: `make setup` then `make validate`

### 1. Installation

#### Quick Install (Recommended)

**Windows (PowerShell):**
```powershell
iwr -useb https://raw.githubusercontent.com/byvfx/go-notion-md-sync/main/scripts/install-windows.ps1 | iex
```

**Linux/macOS:**
```bash
curl -sSL https://raw.githubusercontent.com/byvfx/go-notion-md-sync/main/scripts/install-unix.sh | bash
```

#### Manual Installation
Download from [GitHub Releases](https://github.com/byvfx/go-notion-md-sync/releases) and extract to your PATH.

#### Build from Source
```bash
git clone https://github.com/byvfx/go-notion-md-sync.git
cd go-notion-md-sync
make build
```

**Detailed installation guide**: [INSTALLATION.md](docs/guides/INSTALLATION.md)

### 2. Setup Notion Integration

1. **Create a Notion Integration**:
   - Go to [https://www.notion.so/my-integrations](https://www.notion.so/my-integrations)
   - Click "Create new integration"
   - Give it a name (e.g., "Markdown Sync")
   - Copy the "Internal Integration Token"

2. **Share a Notion Page**:
   - Create or open a Notion page that will be your "parent" page
   - Click "Share" → "Invite" → Add your integration
   - Copy the page ID from the URL (the long string after the last `/`)

### 3. Initialize Your Project

```bash
# Navigate to your project directory
cd my-notion-project

# Initialize configuration and sample files
notion-md-sync init
```

This creates:
- `config.yaml` - Main configuration
- `.env` - Your Notion credentials (edit this!)
- `docs/welcome.md` - Sample markdown file
- `.env.example` - Template for sharing

#### Edit Your Credentials
Edit the `.env` file with your actual Notion credentials:
```bash
NOTION_MD_SYNC_NOTION_TOKEN=ntn_your_token_here
NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID=your_page_id_here
```

#### Method 2: Config File

Copy the template and edit:
```bash
cp config/config.template.yaml config/config.yaml
```

Edit `config/config.yaml` (but use environment variables for secrets):
```yaml
directories:
  excluded_patterns:
  - '*.tmp'
  - 'node_modules/**'
  - '.git/**'
  markdown_root: ./docs
mapping:
  strategy: frontmatter  # or filename
notion:
  parent_page_id: "" # Set via NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID env var
  token: "" # Set via NOTION_MD_SYNC_NOTION_TOKEN env var  
sync:
  conflict_resolution: newer
  direction: push
```

## Usage

### Terminal User Interface (TUI)

Launch the interactive terminal interface for visual file management:

```bash
# Launch the TUI
notion-md-sync tui
```

The TUI provides a split-pane interface with:

**Left Pane - File Browser:**
- Interactive file listing with sync status indicators
- File selection with visual markers
- Status icons: synced, modified, error, pending, conflict
- Statistics showing file counts by status

**Right Pane - Sync Status:**
- Real-time sync operation monitoring
- Tree-style progress display
- Elapsed time tracking
- Daily sync statistics

**Navigation:**
- **Tab**: Switch focus between file list and sync status panes
- **Arrow Keys**: Navigate within the active pane
- **Space**: Select/deselect files for sync operations
- **s**: Initiate sync for selected files
- **c**: Open configuration view
- **q / Ctrl+C**: Quit the application

### Git-like Staging Workflow

The tool now includes a Git-like staging system for better control over which files to sync:

```bash
# Verify configuration is ready
notion-md-sync verify

# Check status of all markdown files
notion-md-sync status

# Stage specific files for sync
notion-md-sync add docs/my-file.md docs/another-file.md

# Stage all changed files
notion-md-sync add .

# Remove files from staging
notion-md-sync reset docs/my-file.md

# Clear all staged files
notion-md-sync reset

# Push only staged files
notion-md-sync push

# Pull changes from Notion
notion-md-sync pull
```

The staging system provides:
- **Smart change detection** using timestamps and content hashes
- **Selective syncing** - only sync the files you want
- **Status overview** showing which files are modified, staged, or synced
- **Parent page context** - status shows current Notion parent page title
- **Concurrent processing** for faster operations

#### Status Command Output
```bash
# Example status output:
On parent page: My Project Documentation

Changes not staged for sync:
  (use "notion-md-sync add <file>..." to stage changes)

        modified: docs/api-guide.md
        modified: docs/user-manual.md
        deleted:  docs/old-notes.md

Changes staged for sync:
  (use "notion-md-sync reset <file>..." to unstage)

        staged:   docs/overview.md
        staged:   docs/quickstart.md
```

### Basic Commands

#### Pull Pages from Notion
```bash
# Pull all pages from Notion to markdown files
# Shows page titles and progress for each page
./bin/notion-md-sync pull --verbose

# Example output:
# Pulling all pages from Notion parent page: 123e4567-e89b-12d3...
# Found 3 pages under parent 123e4567-e89b-12d3...
# 
# [1/3] Pulling page: Project Overview
#   Notion ID: abc123-def456-789012
#   Saving to: docs/Project Overview.md
#   ✓ Successfully pulled

# Pull a specific page by filename (new!)
./bin/notion-md-sync pull --page "My Document.md"

# Pull a specific page by page ID  
./bin/notion-md-sync pull --page-id PAGE_ID --output docs/my-page.md

# Pull to a specific directory
./bin/notion-md-sync pull --directory ./my-docs --verbose

# Dry run - see what would be pulled without making changes
./bin/notion-md-sync pull --dry-run --verbose
```

**Nested Page Support**: The pull command automatically creates directory hierarchies that mirror your Notion page structure. Each page gets its own directory containing the page's markdown file:

```
# Example: Notion workspace with nested pages
docs/
└── Parent Page/                    # Parent page directory
    ├── Parent Page.md              # Parent page content
    ├── Main Document/              # Child page directory
    │   ├── Main Document.md        # Child page content
    │   └── Sub Page 1/             # Nested sub-page directory
    │       ├── Sub Page 1.md       # Sub-page content
    │       └── Sub Page 2/         # Deeply nested directory
    │           └── Sub Page 2.md   # Deeply nested content
    └── Another Document/
        ├── Another Document.md
        └── Nested Content/
            └── Nested Content.md
```

This structure ensures:
- Each page has its own directory for better organization
- Page names are preserved exactly as in Notion (including spaces)
- The hierarchy mirrors your Notion workspace structure
- Round-trip syncing is simplified with consistent naming

#### Push Markdown to Notion
```bash
# Push a specific file
./bin/notion-md-sync push docs/my-document.md --verbose

# Push all markdown files in default directory
./bin/notion-md-sync push --verbose

# Push all files from a specific directory
./bin/notion-md-sync push --directory ./my-docs --verbose

# Dry run - see what would be pushed without making changes
./bin/notion-md-sync push --dry-run --verbose
```

#### Bidirectional Sync
```bash
# Sync in both directions
./bin/notion-md-sync sync bidirectional --verbose

# Push only (markdown → Notion)
./bin/notion-md-sync sync push --verbose

# Pull only (Notion → markdown)  
./bin/notion-md-sync sync pull --verbose

# Sync a specific file
./bin/notion-md-sync sync push --file docs/my-file.md --verbose

# Sync files in a specific directory
./bin/notion-md-sync sync push --directory ./my-docs --verbose

# Dry run - see what would be synced without making changes
./bin/notion-md-sync sync bidirectional --dry-run --verbose
```

#### Watch for Changes
```bash
# Auto-sync when files change
./bin/notion-md-sync watch --verbose
```

### Advanced Usage

#### Performance Optimization (v0.11.0+)

For large-scale operations, v0.11.0 introduces powerful performance optimizations:

**Concurrent Sync**: 2-3x faster with parallel processing
```bash
# Default: Uses optimal worker count automatically
./bin/notion-md-sync pull --concurrent

# Custom worker count for fine-tuning
./bin/notion-md-sync pull --workers 10
```

**Caching**: ~2x improvement for repeated operations
```bash
# Enable caching for API calls (enabled by default)
./bin/notion-md-sync pull --cache-ttl 15m

# Increase cache size for large workloads
./bin/notion-md-sync pull --cache-size 5000
```

**Batch Processing**: Optimal for 100+ pages
```bash
# Process in optimized batches
./bin/notion-md-sync pull --batch-size 50

# Full optimization stack (4-6x faster for large syncs)
./bin/notion-md-sync pull --concurrent --workers 15 --batch-size 50 --cache-size 5000
```

**Performance Testing**:
```bash
# Build performance tools
make build-perf

# Quick API measurement (no test data created)
./scripts/measure-api-perf.sh YOUR_PAGE_ID 20

# Comprehensive performance test (creates/deletes test pages)
./scripts/run-perf-test.sh YOUR_PARENT_PAGE_ID 20 10

# See docs/running-performance-tests.md for detailed instructions
```

#### Dry Run Mode
Test your operations without making any actual changes:

```bash
# See what files would be pushed
./bin/notion-md-sync push --dry-run

# See what files would be pulled  
./bin/notion-md-sync pull --dry-run

# See what would happen in bidirectional sync
./bin/notion-md-sync sync bidirectional --dry-run
```

#### Single File Operations
Work with individual files instead of entire directories:

```bash
# Push a specific file
./bin/notion-md-sync push docs/important-doc.md

# Sync a specific file (if it has notion_id in frontmatter, it can be pulled)
./bin/notion-md-sync sync pull --file docs/existing-doc.md
```

#### Database Operations
Export Notion databases to CSV or import CSV files to create/update databases:

```bash
# Export a Notion database to CSV
./bin/notion-md-sync database export DATABASE_ID output.csv

# Import CSV to create a new database
./bin/notion-md-sync database create input.csv PARENT_PAGE_ID

# Import CSV to update existing database
./bin/notion-md-sync database import input.csv DATABASE_ID

# Pull a specific page to a file
./bin/notion-md-sync pull --page-id PAGE_ID --output docs/new-doc.md
```

#### Directory Options
Specify custom directories for operations:

```bash
# Push all files from a custom directory
./bin/notion-md-sync push --directory ./my-notes

# Pull files to a custom directory
./bin/notion-md-sync pull --directory ./downloaded-notes

# Sync files in a custom directory
./bin/notion-md-sync sync push --directory ./project-docs
```

### Using with Environment Variables

#### Method 1: Helper Script (Easiest)
```bash
# The script automatically loads .env variables
./scripts/run-with-env.sh pull --verbose
./scripts/run-with-env.sh push docs/my-file.md
./scripts/run-with-env.sh watch
```

#### Method 2: Make Commands
```bash
# Load .env and run
make run-env

# Get command to load variables in current shell
make source-env
```

#### Method 3: Manual Export
```bash
export NOTION_MD_SYNC_NOTION_TOKEN="your_token"
export NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID="your_page_id"
./bin/notion-md-sync pull --verbose
```

#### Shell Completion
Enable command autocompletion for your shell:

```bash
# Bash
./bin/notion-md-sync completion bash > /etc/bash_completion.d/notion-md-sync

# Zsh
./bin/notion-md-sync completion zsh > ~/.zsh/completions/_notion-md-sync

# Fish
./bin/notion-md-sync completion fish > ~/.config/fish/completions/notion-md-sync.fish

# PowerShell
./bin/notion-md-sync completion powershell > notion-md-sync.ps1
```

## Enhanced Markdown Support

### LaTeX Math Equations
The tool supports LaTeX math equations using `$$` delimiters:

```markdown
$$x = \frac{-b \pm \sqrt{b^2 - 4ac}}{2a}$$

$$\begin{aligned}
\nabla \times \vec{E} &= -\frac{\partial \vec{B}}{\partial t} \\
\nabla \times \vec{B} &= \mu_0 \vec{J} + \mu_0 \varepsilon_0 \frac{\partial \vec{E}}{\partial t}
\end{aligned}$$
```

### Mermaid Diagrams
Mermaid diagrams are preserved as code blocks:

```markdown
```mermaid
graph TD
    A[Start] --> B{Decision}
    B -->|Yes| C[Process 1]
    B -->|No| D[Process 2]
```

### Extended Block Types
- **Images**: `![caption](url)` with full caption support
- **Callouts**: Blockquotes with emoji icons (`> 💡 Note: ...`)
- **Toggles**: Collapsible sections (via HTML details/summary)
- **Bookmarks**: Links with rich previews
- **Dividers**: Horizontal rules (`---`)

## Markdown Format

### Frontmatter Fields

The tool automatically manages these frontmatter fields:

```yaml
---
title: "My Document Title"
notion_id: "page-id-from-notion"
created_at: "2025-06-10T18:39:00Z"
updated_at: "2025-06-10T15:38:10-07:00"
sync_enabled: true
tags: ["tag1", "tag2"]
status: "published"
---

# Your Content Here

This is the markdown content that syncs with Notion.
```

### Supported Markdown Features

- **Headings**: `# ## ###` (H1, H2, H3) - H4+ automatically convert to H3
- **Paragraphs**: Regular text blocks with proper formatting
- **Lists**: Both bullet (`-`) and numbered (`1.`) lists
- **Code blocks**: Fenced code blocks (`` ```language ``) with language detection
  - Supports 70+ programming languages 
  - Auto-maps common aliases (`js` → `javascript`, `py` → `python`)
  - Preserves syntax highlighting in Notion
- **Tables**: Markdown tables with headers and data rows
  - Supports any number of columns
  - Preserves table structure and content
  - Header row detection and formatting
  - Full bidirectional sync between Notion and markdown
- **Blockquotes**: `> quoted text`
- **Emphasis**: `**bold**`, `*italic*`, and `inline code`
- **Dividers**: `---` horizontal rules

## Examples

### Example 1: Create and Push a New Document

```bash
# 1. Create a new markdown file
cat > docs/my-new-page.md << 'EOF'
---
title: "My New Page"
sync_enabled: true
---

# Welcome to My New Page

This is a test document that will be synced to Notion.

## Features
- Easy markdown editing
- Automatic sync to Notion
- Frontmatter metadata tracking
EOF

# 2. Stage and push to Notion
notion-md-sync add docs/my-new-page.md
notion-md-sync push --verbose

# 3. Check that notion_id was added to frontmatter
cat docs/my-new-page.md
```

### Example 2: Pull Existing Notion Content

```bash
# Pull all pages from your Notion workspace
notion-md-sync pull --verbose

# Check what was downloaded
ls -la docs/
```

### Example 3: Git-like Staging Workflow

```bash
# Verify your configuration is ready
notion-md-sync verify

# Check which files have changed
notion-md-sync status

# Stage specific files
notion-md-sync add docs/file1.md docs/file2.md

# Or stage all changed files
notion-md-sync add .

# Review what's staged and push
notion-md-sync status
notion-md-sync push --verbose
```

### Example 4: Set Up Auto-Sync

```bash
# Start watching for file changes (uses staging automatically)
notion-md-sync watch --verbose

# In another terminal, edit files:
echo "New content" >> docs/my-page.md
# The file will automatically be staged and synced to Notion!
```

### Example 5: Working with Tables

```bash
# 1. Create a markdown file with a table
cat > docs/sales-report.md << 'EOF'
---
title: "Q4 Sales Report"
sync_enabled: true
---

# Q4 Sales Report

## Regional Performance

| Region | Q1 Sales | Q2 Sales | Q3 Sales | Q4 Sales |
| --- | --- | --- | --- | --- |
| North | $125,000 | $142,000 | $158,000 | $167,000 |
| South | $98,000 | $115,000 | $128,000 | $135,000 |
| East | $110,000 | $125,000 | $140,000 | $149,000 |
| West | $87,000 | $95,000 | $108,000 | $118,000 |

Total revenue increased by 23% this quarter!
EOF

# 2. Push to Notion - table will be converted to Notion table format
notion-md-sync push docs/sales-report.md --verbose

# 3. Pull back to verify round-trip conversion works
notion-md-sync pull --page "Q4 Sales Report.md"

# 4. Check that table structure is preserved
cat docs/sales-report.md
```

## Configuration Options

### Directory Settings
- `markdown_root`: Directory containing markdown files (default: `./`)
- `excluded_patterns`: File patterns to ignore (e.g., `*.tmp`, `node_modules/**`)

### Sync Settings
- `direction`: Default sync direction (`push`, `pull`, `bidirectional`)
- `conflict_resolution`: How to handle conflicts (`newer`, `notion_wins`, `markdown_wins`)

### Mapping Strategy
- `filename`: Use filename as Notion page title
- `frontmatter`: Use `title` field from frontmatter

## Troubleshooting

### Common Issues

#### "notion.token is required"
- Ensure your environment variables are set correctly
- Use `./scripts/run-with-env.sh` to automatically load `.env`
- Check that `.env` file exists and has correct format

#### "Page not found" or 403 errors
- Make sure you've shared your Notion page with the integration
- Verify the parent page ID is correct
- Check that your integration token is valid

#### Files not syncing
- Check that `sync_enabled: true` is in the frontmatter
- Verify the file isn't matching an excluded pattern
- Use `--verbose` flag to see detailed output

#### Nested pages not pulling correctly
- Ensure your Notion integration has access to all sub-pages
- Check that parent-child relationships are properly set in Notion
- Use `--verbose` flag to see page hierarchy detection

#### Database operations failing
- Verify the database ID (not page ID) is correct
- Ensure integration has database access permissions
- Check CSV format matches expected column types
- For import operations, verify parent page exists and is accessible

### Getting Help

```bash
# Validate your setup
make validate

# Verify configuration is ready
./bin/notion-md-sync verify

# General help
./bin/notion-md-sync --help

# Command-specific help
./bin/notion-md-sync pull --help
./bin/notion-md-sync push --help
./bin/notion-md-sync sync --help
./bin/notion-md-sync watch --help
./bin/notion-md-sync status --help
./bin/notion-md-sync verify --help
./bin/notion-md-sync database --help
```

## Development

### Building
```bash
make build          # Build binary
make test           # Run tests
make lint           # Run linter
make fmt            # Format code
make clean          # Clean build artifacts
```

### Testing
The project includes comprehensive test coverage:

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific test packages
go test ./pkg/sync      # Converter and markdown processing
go test ./pkg/config    # Configuration loading
go test ./pkg/staging   # Git-like staging system
```

**Test Coverage:**
- **Converter**: Markdown ↔ Notion block conversion, language detection
- **Config**: Environment variables, YAML parsing, validation  
- **Staging**: File change detection, staging workflow, persistence
- **Parser**: Frontmatter extraction, markdown processing
- **CI/CD**: Automated testing on multiple platforms (Linux, macOS, Windows)

### Environment Setup
```bash
make dev-setup      # Install development tools
```

### Release Process

This project uses GitHub Actions for automated releases:

1. **Create Release Notes**: Write release notes in `docs/releases/vX.Y.Z.md`
2. **Tag the Release**: 
   ```bash
   git tag vX.Y.Z
   git push origin vX.Y.Z
   ```
3. **Automated Build**: GitHub Actions will:
   - Run all tests and linting
   - Build binaries for Linux, macOS, and Windows (amd64/arm64)
   - Create a GitHub release with your markdown notes
   - Upload all binary artifacts

**Note**: Binaries are only built on version tags (`v*`), not on regular commits.

### GitHub Workflows

- **CI** (`ci.yml`): Runs tests and linting on all pushes and PRs
- **Release** (`release.yml`): Builds binaries and creates releases on version tags
- **Claude Code** (`claude.yml`): Integrates with Claude for AI-assisted development

## Security

- **Never commit** `.env` files or config files with tokens
- Use environment variables for all sensitive data
- See [SECURITY.md](docs/guides/SECURITY.md) for detailed security guidelines

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Support

- [Documentation](docs/)
- [Issues](https://github.com/byvfx/go-notion-md-sync/issues)
- [Discussions](https://github.com/byvfx/go-notion-md-sync/discussions)