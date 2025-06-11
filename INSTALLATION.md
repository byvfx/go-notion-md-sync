# Installation Guide

This guide covers different ways to install and set up notion-md-sync on your system.

## Quick Installation

### Windows (PowerShell)

Open PowerShell as Administrator and run:

```powershell
# Download and run the installer
iwr -useb https://raw.githubusercontent.com/byvfx/go-notion-md-sync/main/scripts/install-windows.ps1 | iex

# Or download a specific version
$env:VERSION="v0.1.0"; iwr -useb https://raw.githubusercontent.com/byvfx/go-notion-md-sync/main/scripts/install-windows.ps1 | iex
```

### Linux/macOS (Bash)

```bash
# Install latest version
curl -sSL https://raw.githubusercontent.com/byvfx/go-notion-md-sync/main/scripts/install-unix.sh | bash

# Or install specific version
VERSION=v0.1.0 curl -sSL https://raw.githubusercontent.com/byvfx/go-notion-md-sync/main/scripts/install-unix.sh | bash
```

## Manual Installation

### 1. Download Binary

Go to [GitHub Releases](https://github.com/byvfx/go-notion-md-sync/releases) and download the appropriate binary for your platform:

- **Windows**: `notion-md-sync-windows-amd64.zip`
- **Linux**: `notion-md-sync-linux-amd64.tar.gz` or `notion-md-sync-linux-arm64.tar.gz`
- **macOS**: `notion-md-sync-darwin-amd64.tar.gz` or `notion-md-sync-darwin-arm64.tar.gz`

### 2. Extract and Install

#### Windows
```cmd
# Extract the zip file
# Move notion-md-sync.exe to a directory in your PATH
# For example: C:\Users\YourName\bin\
```

#### Linux/macOS
```bash
# Extract the archive
tar -xzf notion-md-sync-*.tar.gz

# Move to a directory in your PATH
sudo mv notion-md-sync /usr/local/bin/
# OR for user-only install
mkdir -p ~/.local/bin
mv notion-md-sync ~/.local/bin/
```

### 3. Add to PATH (if needed)

#### Windows
1. Press `Win + R`, type `sysdm.cpl`, and press Enter
2. Click "Environment Variables"
3. Under "User variables", select "Path" and click "Edit"
4. Click "New" and add the directory containing `notion-md-sync.exe`
5. Click "OK" to save

#### Linux/macOS
Add this to your shell config file (`~/.bashrc`, `~/.zshrc`, etc.):
```bash
export PATH="$PATH:$HOME/.local/bin"
```

## Build from Source

### Prerequisites
- Go 1.21 or later
- Git

### Steps
```bash
# Clone the repository
git clone https://github.com/byvfx/go-notion-md-sync.git
cd go-notion-md-sync

# Build the binary
make build

# Install to system PATH
sudo cp bin/notion-md-sync /usr/local/bin/
# OR install to user PATH
mkdir -p ~/.local/bin
cp bin/notion-md-sync ~/.local/bin/
```

## Setting Up Your First Project

After installation, initialize a new project:

```bash
# Navigate to your project directory
cd my-notion-project

# Initialize the project
notion-md-sync init
```

This will:
- Create a `config.yaml` file
- Create a `.env` file for your credentials
- Create a sample markdown file
- Set up the markdown directory structure

## Configuration

### Method 1: Environment Variables (Recommended)

Edit the `.env` file created by `notion-md-sync init`:

```bash
# Your Notion integration token
NOTION_MD_SYNC_NOTION_TOKEN=ntn_your_token_here

# Your Notion parent page ID
NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID=your_page_id_here
```

### Method 2: Config File

Edit `config.yaml`:

```yaml
notion:
  token: ""  # Set via environment variable
  parent_page_id: ""  # Set via environment variable

sync:
  direction: push
  conflict_resolution: newer

directories:
  markdown_root: ./docs
  excluded_patterns:
    - "*.tmp"
    - "node_modules/**"
    - ".git/**"

mapping:
  strategy: frontmatter
```

## Getting Notion Credentials

### 1. Create a Notion Integration

1. Go to [https://www.notion.so/my-integrations](https://www.notion.so/my-integrations)
2. Click "Create new integration"
3. Give it a name (e.g., "Markdown Sync")
4. Copy the "Internal Integration Token"

### 2. Share a Notion Page

1. Create or open a Notion page that will be your "parent" page
2. Click "Share" → "Invite" → Add your integration
3. Copy the page ID from the URL (the long string after the last `/`)

## Verification

Test your installation:

```bash
# Check version
notion-md-sync --version

# Test configuration
notion-md-sync push --dry-run --verbose

# Push your first file
notion-md-sync push docs/welcome.md --verbose
```

## Troubleshooting

### Command not found
- Make sure the binary is in your PATH
- Restart your terminal after installation
- On Windows, you may need to restart PowerShell/Command Prompt

### Permission denied (Unix/Linux)
```bash
chmod +x /path/to/notion-md-sync
```

### Config file not found
- Make sure you're running commands from the project directory
- Use `notion-md-sync init` to create configuration files
- Specify config path with `-c config.yaml`

### Notion API errors
- Verify your integration token is correct
- Make sure you've shared the parent page with your integration
- Check that the parent page ID is correct

## Uninstallation

### Windows (if installed via script)
```powershell
# Remove from PATH via Environment Variables dialog
# Delete: %LOCALAPPDATA%\notion-md-sync\
```

### Linux/macOS
```bash
# Remove binary
rm ~/.local/bin/notion-md-sync
# OR if installed system-wide
sudo rm /usr/local/bin/notion-md-sync

# Remove from shell config
# Edit ~/.bashrc, ~/.zshrc, etc. and remove PATH addition
```

## Next Steps

Once installed and configured:

1. **Push files**: `notion-md-sync push --verbose`
2. **Pull from Notion**: `notion-md-sync pull --verbose`
3. **Watch for changes**: `notion-md-sync watch --verbose`
4. **Get help**: `notion-md-sync --help`

For detailed usage examples, see the [README.md](README.md).