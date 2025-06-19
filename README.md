# notion-md-sync

[![Build Status](https://github.com/byvfx/go-notion-md-sync/workflows/Build%20Binaries/badge.svg)](https://github.com/byvfx/go-notion-md-sync/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/byvfx/go-notion-md-sync)](https://goreportcard.com/report/github.com/byvfx/go-notion-md-sync)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![GitHub release](https://img.shields.io/github/release/byvfx/go-notion-md-sync.svg)](https://github.com/byvfx/go-notion-md-sync/releases)
[![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.21-blue)](https://golang.org)

A powerful CLI tool for synchronizing markdown files with Notion pages. Built with Go for fast, reliable bidirectional sync between your local markdown files and Notion workspace.

## Features

- 🔄 **Bidirectional Sync**: Push markdown to Notion or pull Notion pages to markdown
- 🎯 **Git-like Staging**: Stage files for sync with `add`, `status`, and `reset` commands
- 📝 **Frontmatter Support**: Automatic metadata management with YAML frontmatter
- 💾 **Smart Change Detection**: Hybrid timestamp and content-based change tracking
- 👀 **File Watching**: Real-time auto-sync when files change
- 🔒 **Secure Configuration**: Environment variable support for API tokens
- 🗂️ **Flexible Mapping**: Choose between filename or frontmatter-based page mapping
- 🚀 **High Performance**: Concurrent processing with optimized block operations
- 🧪 **Comprehensive Testing**: Full test coverage with CI/CD validation
- ⚡ **Fast & Reliable**: Built with Go for performance and reliability
- ✅ **Configuration Verification**: Check your setup is ready with `verify` command
- 📊 **Enhanced Pull Information**: See page titles and progress when pulling from Notion
- 🏷️ **Parent Page Context**: Status command shows current Notion parent page title

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

📖 **Detailed installation guide**: [INSTALLATION.md](docs/guides/INSTALLATION.md)

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

# Pull a specific page
./bin/notion-md-sync pull --page-id PAGE_ID --output docs/my-page.md

# Pull to a specific directory
./bin/notion-md-sync pull --directory ./my-docs --verbose

# Dry run - see what would be pulled without making changes
./bin/notion-md-sync pull --dry-run --verbose
```

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

- 📖 [Documentation](docs/)
- 🐛 [Issues](https://github.com/byvfx/go-notion-md-sync/issues)
- 💬 [Discussions](https://github.com/byvfx/go-notion-md-sync/discussions)