#!/bin/bash
# Unix installation script for notion-md-sync (Linux/macOS)
# Usage: curl -sSL https://raw.githubusercontent.com/byvfx/go-notion-md-sync/main/scripts/install-unix.sh | bash

set -e

# Configuration
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${VERSION:-latest}"
REPO="byvfx/go-notion-md-sync"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log() {
    echo -e "${GREEN}🚀 $1${NC}"
}

warn() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

error() {
    echo -e "${RED}❌ $1${NC}"
    exit 1
}

# Detect OS and architecture
detect_platform() {
    local os
    local arch
    
    case "$(uname -s)" in
        Linux*)     os="linux" ;;
        Darwin*)    os="darwin" ;;
        *)          error "Unsupported operating system: $(uname -s)" ;;
    esac
    
    case "$(uname -m)" in
        x86_64)     arch="amd64" ;;
        amd64)      arch="amd64" ;;
        arm64)      arch="arm64" ;;
        aarch64)    arch="arm64" ;;
        *)          error "Unsupported architecture: $(uname -m)" ;;
    esac
    
    echo "${os}-${arch}"
}

# Get latest version from GitHub API
get_latest_version() {
    if command -v curl >/dev/null 2>&1; then
        curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4
    else
        error "Neither curl nor wget is available. Please install one of them."
    fi
}

# Download and extract binary
install_binary() {
    local platform="$1"
    local version="$2"
    
    if [ "$version" = "latest" ]; then
        log "Getting latest release information..."
        version=$(get_latest_version)
        if [ -z "$version" ]; then
            error "Failed to get latest version"
        fi
    fi
    
    local download_url="https://github.com/${REPO}/releases/download/${version}/notion-md-sync-${platform}.tar.gz"
    local temp_dir=$(mktemp -d)
    local archive_file="${temp_dir}/notion-md-sync.tar.gz"
    
    log "Downloading notion-md-sync ${version} for ${platform}..."
    
    if command -v curl >/dev/null 2>&1; then
        curl -sL "$download_url" -o "$archive_file" || error "Failed to download from $download_url"
    elif command -v wget >/dev/null 2>&1; then
        wget -q "$download_url" -O "$archive_file" || error "Failed to download from $download_url"
    else
        error "Neither curl nor wget is available"
    fi
    
    log "Creating installation directory: $INSTALL_DIR"
    mkdir -p "$INSTALL_DIR"
    
    log "Extracting binary..."
    tar -xzf "$archive_file" -C "$temp_dir"
    
    # Find the binary (it might be in a subdirectory)
    local binary_path
    if [ -f "${temp_dir}/notion-md-sync" ]; then
        binary_path="${temp_dir}/notion-md-sync"
    else
        binary_path=$(find "$temp_dir" -name "notion-md-sync" -type f | head -n1)
    fi
    
    if [ -z "$binary_path" ] || [ ! -f "$binary_path" ]; then
        error "Binary not found in archive"
    fi
    
    # Install binary
    cp "$binary_path" "$INSTALL_DIR/notion-md-sync"
    chmod +x "$INSTALL_DIR/notion-md-sync"
    
    # Cleanup
    rm -rf "$temp_dir"
}

# Add to PATH if needed
setup_path() {
    # Check if already in PATH
    if echo "$PATH" | tr ':' '\n' | grep -Fxq "$INSTALL_DIR"; then
        return 0
    fi
    
    log "Adding $INSTALL_DIR to PATH..."
    
    # Determine shell config file
    local shell_config=""
    case "$SHELL" in
        */bash)
            if [ -f "$HOME/.bashrc" ]; then
                shell_config="$HOME/.bashrc"
            elif [ -f "$HOME/.bash_profile" ]; then
                shell_config="$HOME/.bash_profile"
            fi
            ;;
        */zsh)
            shell_config="$HOME/.zshrc"
            ;;
        */fish)
            shell_config="$HOME/.config/fish/config.fish"
            ;;
    esac
    
    if [ -n "$shell_config" ]; then
        echo "export PATH=\"\$PATH:$INSTALL_DIR\"" >> "$shell_config"
        warn "Added to $shell_config - restart your shell or run: source $shell_config"
    else
        warn "Could not detect shell config file. Please add $INSTALL_DIR to your PATH manually."
    fi
    
    # Add to current session
    export PATH="$PATH:$INSTALL_DIR"
}

# Verify installation
verify_installation() {
    if [ -f "$INSTALL_DIR/notion-md-sync" ]; then
        log "Installation successful!"
        echo ""
        echo "Binary installed at: $INSTALL_DIR/notion-md-sync"
        echo ""
        echo -e "${BLUE}🎯 Next steps:${NC}"
        echo "   1. Restart your terminal or source your shell config"
        echo "   2. Create a project: notion-md-sync init"
        echo "   3. Start syncing: notion-md-sync watch"
        echo ""
        echo -e "${BLUE}📚 For help, run: notion-md-sync --help${NC}"
        
        # Try to run version check
        if "$INSTALL_DIR/notion-md-sync" --version >/dev/null 2>&1; then
            echo ""
            echo "Version: $($INSTALL_DIR/notion-md-sync --version)"
        fi
    else
        error "Installation failed - binary not found at $INSTALL_DIR/notion-md-sync"
    fi
}

# Main installation flow
main() {
    log "Installing notion-md-sync..."
    
    local platform
    platform=$(detect_platform)
    log "Detected platform: $platform"
    
    install_binary "$platform" "$VERSION"
    setup_path
    verify_installation
}

# Run main function
main "$@"