---
notion_id: 212388d7-7461-8194-b62e-fb9d9b78a698
sync_enabled: true
updated_at: "2025-06-13T23:01:34-07:00"
---

# Release Notes

## v0.5.0 - Git-like Staging Workflow & Major Improvements

### 🚀 Major Features: Git-like Staging + Critical Fixes + Performance Boost

This release represents the biggest advancement since launch, introducing a revolutionary Git-like staging workflow alongside critical bug fixes, comprehensive testing, and significant performance improvements. This transforms notion-md-sync into a professional-grade tool suitable for production workflows.

#### 🎯 New Git-like Commands

**Check what's changed:**
```bash
notion-md-sync status
# Output:
# Changes staged for sync:
#   staged:    docs/ready-file.md
# 
# Changes not staged for sync:
#   modified:  docs/updated-file.md
#   new file:  docs/new-file.md
```

**Stage files for sync:**
```bash
notion-md-sync add docs/file.md       # Stage specific file
notion-md-sync add docs/              # Stage all files in directory  
notion-md-sync add .                  # Stage all changed files
notion-md-sync add *.md               # Stage with patterns
```

**Unstage files:**
```bash
notion-md-sync reset docs/file.md     # Unstage specific file
notion-md-sync reset .                # Unstage all files
```

**Push only staged changes:**
```bash
notion-md-sync push                   # Push only staged files
notion-md-sync push docs/file.md      # Auto-stage and push specific file
notion-md-sync push --dry-run         # Preview what would be pushed
```

#### 🧠 Intelligent Change Detection

- **Hybrid Detection**: Fast timestamp checking with SHA256 hash verification for accuracy
- **Performance Optimized**: Only calculates hashes when timestamps suggest changes
- **Reliable Tracking**: Persistent `.notion-sync/` directory stores file states
- **Auto-Update**: Smart timestamp updates prevent unnecessary future hash calculations

#### 🔄 Enhanced Push Behavior

- **Staging-First**: Push now only syncs staged files (no more accidental syncs!)
- **Auto-Staging**: Specific file pushes automatically stage the file first
- **Success Tracking**: Successfully pushed files are automatically unstaged
- **Partial Failure Handling**: Failed pushes remain staged for retry

#### 📊 Comprehensive Status Display

- **Color-Coded Output**: Green for staged, red for modified/new
- **Clear Instructions**: Helpful hints for next steps
- **File Categories**: Staged, modified, new, and deleted files clearly separated
- **Summary Messages**: Git-like guidance for common workflows

#### 💾 Persistent State Management

- **`.notion-sync/` Directory**: Stores staging index and file tracking data
- **JSON Index**: Human-readable staging state storage
- **Automatic Cleanup**: Successful syncs automatically update tracking
- **Cross-Session**: Staging state persists across CLI sessions

#### 🔧 Technical Improvements

- **Type-Safe Staging**: New `staging` package with proper error handling
- **Memory Efficient**: Incremental file processing prevents memory issues
- **Path Handling**: Robust relative/absolute path conversion
- **Error Recovery**: Graceful handling of file access issues

#### 📈 Workflow Examples

**Basic staging workflow:**
```bash
# See what's changed
notion-md-sync status

# Stage files you want to sync
notion-md-sync add docs/important.md docs/update.md

# Preview the sync
notion-md-sync push --dry-run

# Sync to Notion
notion-md-sync push
```

**Selective staging:**
```bash
# Stage only specific files
notion-md-sync add docs/chapter1.md docs/chapter2.md

# Unstage one file
notion-md-sync reset docs/chapter2.md  

# Push remaining staged files
notion-md-sync push
```

#### 🎉 Developer Experience

This brings the familiar Git workflow to Notion sync:
- **Predictable**: Only staged files sync, no surprises
- **Controlled**: Choose exactly what changes to publish
- **Efficient**: Skip unchanged files automatically
- **Professional**: Industry-standard version control patterns

#### ⚠️ Breaking Changes

- **Push behavior**: `notion-md-sync push` now only pushes staged files
- **New default**: Staging is now the default workflow (not opt-in)
- **Migration**: Existing users should run `notion-md-sync add .` to stage all files

### 🐛 Critical Bug Fixes

#### Fixed: Code Blocks Empty Content
**CRITICAL FIX**: Code blocks were appearing in Notion but with no content inside.

**What was broken:**
- Fenced code blocks (`` ```language ``) showed up as empty blocks in Notion
- Only the language label appeared, but no actual code content
- Affected all programming languages and code examples

**What's fixed:**
- **🔧 Rewrote Code Block Extraction**: Implemented proper AST content extraction for fenced code blocks
- **💻 Full Content Preservation**: Code blocks now sync with complete source code intact  
- **🎯 Language Detection**: Enhanced language mapping (js→javascript, py→python, 70+ languages)
- **✅ Notion Validation**: Automatic language validation with fallback to "plain text"

**Testing Impact**: All code examples now appear correctly in Notion with proper syntax highlighting.

#### Fixed: Sync Performance Issues  
**Major performance improvements for large documents and many files.**

**What was improved:**
- **⚡ Parallel Block Deletion**: Replaced sequential deletion with 5-worker concurrent processing
- **🚀 Optimized Update Process**: Reduced delays and improved block handling efficiency
- **📊 Performance Results**: 
  - Large documents (400+ lines): **4.5 seconds** (fast)
  - Small documents now process without corruption
  - Pull operations: **2-3 seconds** consistently

**Technical details:**
- Implemented `bulkDeleteBlocks()` with worker pools
- Added proper error handling for undeletable blocks
- Optimized sequential fallback for reliability

### 🧪 Comprehensive Testing Suite

**NEW**: Full test coverage with CI/CD integration for bulletproof reliability.

#### Test Coverage Added
- **`pkg/sync/`**: 27 test cases covering markdown↔blocks conversion, language mapping
- **`pkg/config/`**: 6 test cases covering configuration loading, environment variables  
- **`pkg/staging/`**: 5 test cases covering staging workflow, change detection
- **`pkg/markdown/`**: 4 test cases covering frontmatter, file parsing

#### CI/CD Integration
- **✅ Automated Testing**: GitHub Actions runs full test suite on every commit
- **🌍 Multi-Platform**: Tests run on Linux, macOS, and Windows  
- **🛡️ Build Validation**: Binaries only created if all tests pass
- **📊 Coverage Reports**: Comprehensive test coverage tracking

#### Developer Benefits
- **🐛 Bug Prevention**: Catch regressions before they reach users
- **🔒 Reliability**: Every function tested with edge cases
- **📈 Confidence**: Tests validate all critical workflows work correctly

### 🔐 Security Enhancements

**Major security audit and fixes implemented.**

#### Critical Fixes Applied
- **🚨 Removed Hardcoded Credentials**: Eliminated exposed API tokens from repository
- **🛡️ Fixed Shell Injection**: Secure `.env` file parsing in run scripts
- **🔒 Input Validation**: Enhanced path validation for file operations
- **📝 Security Guidelines**: Updated documentation with best practices

#### Security Features
- **✅ HTTPS-Only**: All API communications use secure protocols
- **✅ Environment Variables**: Proper secret management patterns
- **✅ File Permissions**: Secure file creation with appropriate permissions
- **✅ Error Handling**: No sensitive data leakage in error messages

### 🎯 Enhanced Markdown Support

#### Improved Code Block Handling
- **70+ Programming Languages**: Comprehensive language detection and mapping
- **Smart Aliases**: Common abbreviations automatically mapped (js→javascript, py→python)
- **Syntax Preservation**: Code formatting preserved exactly from markdown to Notion
- **Fallback Handling**: Unknown languages default to "plain text" with content intact

#### Better Block Processing  
- **H4+ Support**: Headers above H3 automatically convert to H3 (Notion's limit)
- **Rich Text Handling**: Improved bold, italic, and inline code processing
- **List Improvements**: Enhanced bulleted and numbered list conversion
- **Blockquote Support**: Proper quote block formatting

### ⚠️ Breaking Changes

- **Push behavior**: `notion-md-sync push` now only pushes staged files
- **New default**: Staging is now the default workflow (not opt-in)
- **Migration**: Existing users should run `notion-md-sync add .` to stage all files

### 🎉 Developer Experience Improvements

This release brings enterprise-grade reliability:
- **🔍 Predictable**: Comprehensive testing ensures consistent behavior
- **🛡️ Secure**: Security audit eliminates vulnerabilities  
- **⚡ Fast**: Performance optimizations for large-scale usage
- **🎯 Professional**: Git-like workflow familiar to developers
- **📚 Documented**: Enhanced documentation with testing and security guides

This is the biggest enhancement since the initial release, bringing notion-md-sync in line with modern development workflows while ensuring bulletproof reliability!

---

## v0.4.0 - Critical Pull Bug Fix

### 🐛 Major Fix: Pull Command Now Extracts Content Properly

This release fixes a critical bug where the `pull` command was only extracting metadata from Notion pages instead of the actual page content, resulting in nearly empty markdown files.

#### What Was Broken
- `notion-md-sync pull` would create markdown files with only frontmatter metadata
- Page content (paragraphs, headings, lists, etc.) was not being converted to markdown
- Users would get empty files despite having content in their Notion pages

#### What's Fixed
- **🔧 Complete Block Structure Rewrite**: Rebuilt the Notion API block handling from scratch
- **📄 Proper Content Extraction**: Pull command now correctly extracts all text content from Notion blocks
- **🎯 Type-Safe Block Parsing**: Replaced error-prone interface{} casting with proper typed structs
- **🔄 Python Parity**: Go implementation now matches the working Python version's approach

#### Technical Details
- **Fixed Block struct**: Removed incorrect `json:",inline"` and added proper typed fields for each block type
- **Rewrote BlocksToMarkdown converter**: Now directly accesses typed block fields (`block.Paragraph.RichText`)
- **Improved text extraction**: New `extractPlainTextFromRichText()` function works with proper Notion API types
- **Added support for**: Paragraphs, headings (H1-H3), lists, code blocks, quotes, and dividers

#### For Existing Users
**Critical Update Required**: This is a major fix that completely resolves the empty file issue.

**Reinstall to get the fix:**
```powershell
# Windows
iwr -useb https://raw.githubusercontent.com/byvfx/go-notion-md-sync/main/scripts/install-windows.ps1 | iex

# Linux/macOS  
curl -sSL https://raw.githubusercontent.com/byvfx/go-notion-md-sync/main/scripts/install-unix.sh | bash
```

**Test your pull commands:**
```bash
cd your-project-directory
notion-md-sync pull --verbose  # Should now extract full content!
```

#### Supported Notion Block Types
- ✅ **Headings**: H1, H2, H3 with proper markdown conversion
- ✅ **Paragraphs**: Full text content with formatting preservation
- ✅ **Lists**: Both bulleted and numbered lists
- ✅ **Code Blocks**: With language syntax highlighting
- ✅ **Quotes**: Blockquote formatting
- ✅ **Dividers**: Horizontal rule conversion

This fix ensures the complete bidirectional sync experience that was originally intended. Pull operations now work as expected, matching the functionality of the proven Python implementation.

---

## v0.3.0 - Configuration Bug Fix

### 🐛 Critical Fix: Automatic .env File Loading

This release fixes a critical issue where `.env` files created by `notion-md-sync init` weren't being automatically loaded, causing "notion.token is required" errors even when credentials were properly configured.

#### What's Fixed
- **🔧 Automatic .env Loading**: Environment variables from `.env` files are now automatically loaded before config validation
- **📁 Smart File Discovery**: Searches for `.env` files in current directory, parent directories, and `~/.notion-md-sync/`
- **🔄 Seamless Experience**: Commands now work immediately after running `notion-md-sync init` and editing `.env`

#### For Existing Users
If you're experiencing config errors after v0.2.0 installation:

**Reinstall to get the fix:**
```powershell
# Windows
iwr -useb https://raw.githubusercontent.com/byvfx/go-notion-md-sync/main/scripts/install-windows.ps1 | iex

# Linux/macOS  
curl -sSL https://raw.githubusercontent.com/byvfx/go-notion-md-sync/main/scripts/install-unix.sh | bash
```

**Then test your existing project:**
```bash
cd your-project-directory
notion-md-sync pull --verbose  # Should now work!
```

#### Technical Details
- Added automatic `.env` file loading using the existing `gotenv` dependency
- Environment variables are loaded before config validation
- Backwards compatible with manual environment variable setting
- No breaking changes to existing functionality

This fix ensures the seamless "install → init → use" experience that was intended in v0.2.0.

---

## v0.2.0 - Installation & Setup Improvements

### 🚀 Enhanced Installation Experience

We've made it significantly easier to install and get started with notion-md-sync!

#### New Features
- **📦 One-Line Installation Scripts**: Install on Windows, Linux, and macOS with a single command
- **🎯 Project Initialization**: New `notion-md-sync init` command for easy project setup
- **📁 Automatic PATH Management**: Installers automatically add the binary to your system PATH
- **📖 Comprehensive Installation Guide**: New [INSTALLATION.md](INSTALLATION.md) with detailed setup instructions

#### Installation Commands

**Windows (PowerShell):**
```powershell
iwr -useb https://raw.githubusercontent.com/byvfx/go-notion-md-sync/main/scripts/install-windows.ps1 | iex
```

**Linux/macOS:**
```bash
curl -sSL https://raw.githubusercontent.com/byvfx/go-notion-md-sync/main/scripts/install-unix.sh | bash
```

#### Quick Project Setup
```bash
# Navigate to your project directory
cd my-notion-project

# Initialize with interactive setup
notion-md-sync init

# Edit .env with your credentials and start syncing!
notion-md-sync push --verbose
```

#### What's New
- Automatic binary download and extraction
- Cross-platform PATH configuration
- Interactive credential setup
- Sample files and directory structure creation
- Improved error messages and help text

This release focuses on user experience and makes notion-md-sync as easy to install as popular CLI tools like ffmpeg or git.

---

## v0.1.0 - Initial Release

## 🎉 Initial Release

We're excited to announce the first release of **notion-md-sync** - a powerful CLI tool for synchronizing markdown files with Notion pages, built with Go for fast and reliable performance.

## ✨ Features

### Core Functionality
- **🔄 Bidirectional Sync**: Seamlessly sync between markdown files and Notion pages
- **📝 Frontmatter Support**: Automatic metadata management with YAML frontmatter
- **👀 File Watching**: Real-time auto-sync when files change
- **🎯 Flexible Mapping**: Choose between filename or frontmatter-based page mapping

### CLI Commands
- `pull` - Download Notion pages as markdown files
- `push` - Upload markdown files to Notion
- `sync` - Bidirectional synchronization with conflict resolution
- `watch` - Monitor file changes for automatic sync

### Configuration & Security
- **🔒 Secure Configuration**: Environment variable support for API tokens
- **📁 Directory Management**: Configurable markdown directories with exclusion patterns
- **⚙️ Flexible Config**: YAML configuration with environment variable overrides

## 🚀 Quick Start

### Installation
```bash
# Download from GitHub Releases
wget https://github.com/byvfx/go-notion-md-sync/releases/download/v0.1.0/notion-md-sync-linux-amd64.tar.gz
tar -xzf notion-md-sync-linux-amd64.tar.gz

# Or build from source
git clone https://github.com/byvfx/go-notion-md-sync.git
cd go-notion-md-sync
make build
```

### Basic Usage
```bash
# Pull pages from Notion
./notion-md-sync pull --verbose

# Push markdown to Notion
./notion-md-sync push docs/my-file.md --verbose

# Watch for changes
./notion-md-sync watch --verbose
```

## 📦 Binary Downloads

This release includes pre-built binaries for:
- **Linux**: AMD64, ARM64
- **macOS**: AMD64 (Intel), ARM64 (Apple Silicon)
- **Windows**: AMD64

## 🛠 Technical Details

### Architecture
- Built with Go 1.21+ for performance and reliability
- Uses Cobra for CLI interface and Viper for configuration
- Goldmark for markdown parsing with frontmatter support
- fsnotify for cross-platform file watching

### Supported Markdown Features
- Headings (H1, H2, H3)
- Paragraphs and text formatting
- Bullet and numbered lists
- Code blocks with syntax highlighting
- Bold and italic emphasis

## 🔧 Configuration

### Environment Variables
```bash
NOTION_MD_SYNC_NOTION_TOKEN=your_integration_token
NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID=your_parent_page_id
```

### Config File (config.yaml)
```yaml
directories:
  markdown_root: ./docs
  excluded_patterns:
    - '*.tmp'
    - 'node_modules/**'
    - '.git/**'
sync:
  direction: push
  conflict_resolution: newer
mapping:
  strategy: frontmatter
```

## 📋 What's Included

- Complete CLI application with all sync commands
- Comprehensive documentation and examples
- Automated setup and validation scripts
- GitHub Actions workflow for multi-platform builds
- Security guidelines and best practices

## 🐛 Known Issues

- This is the initial release - please report any issues on GitHub
- Some advanced Notion block types may not be fully supported yet
- Large file operations may need optimization in future releases

## 🤝 Contributing

We welcome contributions! Please see our contributing guidelines and feel free to:
- Report bugs and feature requests
- Submit pull requests
- Improve documentation
- Share usage examples

## 📚 Documentation

- [README.md](README.md) - Complete usage guide
- [QUICK_START.md](QUICK_START.md) - Get started quickly
- [SECURITY.md](SECURITY.md) - Security best practices
- [CLAUDE.md](CLAUDE.md) - Development guidelines

## 🎯 Next Steps

Future releases will focus on:
- Enhanced Notion block type support
- Performance optimizations
- Advanced sync conflict resolution
- Web UI for configuration
- Plugin system for extensibility

## 🙏 Acknowledgments

Special thanks to the Go community and the maintainers of the excellent libraries that make this project possible:
- Cobra & Viper for CLI framework
- Goldmark for markdown processing
- fsnotify for file watching

---

**Full Changelog**: https://github.com/byvfx/go-notion-md-sync/commits/v0.1.0

**Download**: https://github.com/byvfx/go-notion-md-sync/releases/tag/v0.1.0