# Windows installation script for notion-md-sync
# Run with: PowerShell -ExecutionPolicy Bypass -File install-windows.ps1

param(
    [string]$InstallDir = "$env:LOCALAPPDATA\notion-md-sync",
    [string]$Version = "latest"
)

$ErrorActionPreference = "Stop"

Write-Host "🚀 Installing notion-md-sync for Windows..." -ForegroundColor Green

# Detect architecture
$arch = "amd64"
if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
    $arch = "arm64"
}

# Set download URL
if ($Version -eq "latest") {
    $apiUrl = "https://api.github.com/repos/byvfx/go-notion-md-sync/releases/latest"
    try {
        $release = Invoke-RestMethod -Uri $apiUrl
        $Version = $release.tag_name
    } catch {
        Write-Error "Failed to get latest release info. Please specify a version."
        exit 1
    }
}

$downloadUrl = "https://github.com/byvfx/go-notion-md-sync/releases/download/$Version/notion-md-sync-windows-$arch.zip"
$zipFile = "$env:TEMP\notion-md-sync-windows-$arch.zip"

Write-Host "📦 Downloading notion-md-sync $Version for windows-$arch..."
try {
    Invoke-WebRequest -Uri $downloadUrl -OutFile $zipFile
} catch {
    Write-Error "Failed to download from $downloadUrl"
    exit 1
}

# Create installation directory
Write-Host "📁 Creating installation directory: $InstallDir"
if (!(Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

# Extract binary
Write-Host "📦 Extracting binary..."
try {
    Expand-Archive -Path $zipFile -DestinationPath $InstallDir -Force
    Remove-Item $zipFile -Force
} catch {
    Write-Error "Failed to extract archive"
    exit 1
}

# Add to PATH if not already there
$currentPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($currentPath -notlike "*$InstallDir*") {
    Write-Host "🔧 Adding to PATH..."
    [Environment]::SetEnvironmentVariable("PATH", "$currentPath;$InstallDir", "User")
    $env:PATH = "$env:PATH;$InstallDir"
}

# Verify installation
$binaryPath = Join-Path $InstallDir "notion-md-sync.exe"
if (Test-Path $binaryPath) {
    Write-Host "✅ Installation successful!" -ForegroundColor Green
    Write-Host ""
    Write-Host "Binary installed at: $binaryPath"
    Write-Host ""
    Write-Host "🎯 Next steps:"
    Write-Host "   1. Restart your terminal or run: refreshenv"
    Write-Host "   2. Create a project: notion-md-sync init"
    Write-Host "   3. Pull down your notion page: notion-md-sync pull"
    Write-Host ""
    Write-Host "📚 For help, run: notion-md-sync --help"
} else {
    Write-Error "Installation failed - binary not found at $binaryPath"
    exit 1
}