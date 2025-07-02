# Release Notes

All release notes have been organized by version for easier navigation.

## Current Release

- [v0.10.1 - Bug Fix: Nested Page Pulling](docs/releases/v0.10.1.md) - Critical fix for pull command not fetching nested sub-pages

## Previous Releases

- [v0.10.0 - Feature Completeness (Phase 2)](docs/releases/v0.10.0.md) - Extended Notion blocks, LaTeX math, Mermaid diagrams, and CSV/database integration
- [v0.8.2 - Test Coverage & Code Quality Foundation](docs/releases/v0.8.2.md) - Major milestone in code quality with 74% test coverage and A+ Go Report Card
- [v0.8.1 - Watch Command Testing & Reliability Improvements](docs/releases/v0.8.1.md) - Comprehensive watch command tests and reliability improvements
- [v0.8.0 - Table Support & Enhanced Pull Commands](docs/releases/v0.8.0.md) - Full table support and single file pull
- [v0.7.0 - Enhanced User Experience & Visibility](docs/releases/v0.7.0.md) - Verify command and enhanced pull visibility

- [v0.6.0 - Interactive Conflict Resolution](docs/releases/v0.6.0.md) - Comprehensive conflict resolution system
- [v0.5.0 - Git-like Staging Workflow & Major Improvements](docs/releases/v0.5.0.md) - Revolutionary Git-like staging workflow
- [v0.4.0 - Critical Pull Bug Fix](docs/releases/v0.4.0.md) - Fixed empty content issue in pull command
- [v0.3.0 - Configuration Bug Fix](docs/releases/v0.3.0.md) - Fixed automatic .env file loading
- [v0.2.0 - Installation & Setup Improvements](docs/releases/v0.2.0.md) - Enhanced installation experience
- [v0.1.0 - Initial Release](docs/releases/v0.1.0.md) - First release of notion-md-sync

## Release Highlights

### Latest Bug Fix (v0.10.1)
- 🐛 **Critical Fix**: Pull command now correctly fetches nested sub-pages
- 📁 **Directory Structure**: Proper nested directory creation mirroring Notion hierarchy
- 🔄 **Infinite Loop Fix**: Resolved timeout issues during recursive page fetching
- 🛡️ **Enhanced Safety**: Improved cycle detection and error handling

### Recent Features (v0.10.0)
- 🖼️ **Extended Block Support**: Images, callouts, toggles, bookmarks, dividers
- 🧮 **LaTeX Math Equations**: Full support for mathematical expressions with `$$` blocks
- 📊 **Mermaid Diagrams**: Preserve and sync diagram code blocks
- 🗄️ **CSV/Database Integration**: Export/import Notion databases to/from CSV
- 🎨 **Enhanced Markdown**: Advanced formatting with proper metadata handling

### Core Features
- 🔄 Bidirectional sync between markdown and Notion
- 🎯 Git-like staging workflow (add, status, reset, push)
- 📝 Full frontmatter support with metadata tracking
- 👀 Real-time file watching for auto-sync
- 🔒 Secure configuration with environment variables
- 🚀 High performance with concurrent processing

## Installation

Get the latest release:

**Windows:**
```powershell
iwr -useb https://raw.githubusercontent.com/byvfx/go-notion-md-sync/main/scripts/install-windows.ps1 | iex
```

**Linux/macOS:**
```bash
curl -sSL https://raw.githubusercontent.com/byvfx/go-notion-md-sync/main/scripts/install-unix.sh | bash
```

For detailed release information, see the individual version files in [docs/releases/](docs/releases/).