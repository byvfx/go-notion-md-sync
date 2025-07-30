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
    Write-Host "📍 Installation Details:" -ForegroundColor Cyan
    Write-Host "   Binary location: $binaryPath" -ForegroundColor Gray
    Write-Host "   Added to PATH: $InstallDir" -ForegroundColor Gray
    Write-Host ""
    Write-Host "🎯 Next Steps:" -ForegroundColor Yellow
    Write-Host "   1. 🔄 Restart your terminal (or run: refreshenv)" -ForegroundColor White
    Write-Host "   2. 📁 Navigate to your project folder: cd C:\path\to\your\project" -ForegroundColor White
    Write-Host "   3. 🚀 Initialize project: notion-md-sync init" -ForegroundColor White
    Write-Host "      (This will guide you through setup and create config files)" -ForegroundColor Gray
    Write-Host "   4. 📥 Pull your Notion pages: notion-md-sync pull" -ForegroundColor White
    Write-Host ""
    Write-Host "💡 Tips:" -ForegroundColor Magenta
    Write-Host "   • Your config files will be created in your project directory" -ForegroundColor Gray
    Write-Host "   • You can copy/paste your Notion token and page ID during init" -ForegroundColor Gray
    Write-Host "   • Use 'notion-md-sync --help' for all commands" -ForegroundColor Gray
    Write-Host ""
    Write-Host "🗑️  To uninstall later, run the uninstall script:" -ForegroundColor DarkGray
    Write-Host "   iwr -useb https://raw.githubusercontent.com/byvfx/go-notion-md-sync/main/scripts/uninstall-windows.ps1 | iex" -ForegroundColor DarkGray
} else {
    Write-Error "Installation failed - binary not found at $binaryPath"
    exit 1
}