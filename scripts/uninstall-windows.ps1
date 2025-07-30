# Windows uninstall script for notion-md-sync
# Run with: iwr -useb https://raw.githubusercontent.com/byvfx/go-notion-md-sync/main/scripts/uninstall-windows.ps1 | iex

param(
    [string]$InstallDir = "$env:LOCALAPPDATA\notion-md-sync",
    [switch]$Force
)

$ErrorActionPreference = "Stop"

Write-Host "🗑️  Uninstalling notion-md-sync for Windows..." -ForegroundColor Yellow

# Check if installation exists
if (!(Test-Path $InstallDir)) {
    Write-Host "❌ notion-md-sync installation not found at: $InstallDir" -ForegroundColor Red
    Write-Host "It may have been installed in a different location or already removed." -ForegroundColor Gray
    exit 1
}

$binaryPath = Join-Path $InstallDir "notion-md-sync.exe"
if (!(Test-Path $binaryPath)) {
    Write-Host "❌ notion-md-sync.exe not found at expected location: $binaryPath" -ForegroundColor Red
    exit 1
}

# Confirm uninstallation unless Force is used
if (-not $Force) {
    Write-Host ""
    Write-Host "📍 Found installation at: $InstallDir" -ForegroundColor Cyan
    Write-Host ""
    $confirmation = Read-Host "Are you sure you want to uninstall notion-md-sync? (y/N)"
    if ($confirmation -notlike "y*" -and $confirmation -notlike "Y*") {
        Write-Host "❌ Uninstall cancelled." -ForegroundColor Yellow
        exit 0
    }
}

Write-Host ""
Write-Host "🔧 Removing from PATH..." -ForegroundColor Blue

# Remove from PATH
$currentPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($currentPath -like "*$InstallDir*") {
    $newPath = $currentPath -replace [regex]::Escape(";$InstallDir"), ""
    $newPath = $newPath -replace [regex]::Escape("$InstallDir;"), ""
    $newPath = $newPath -replace [regex]::Escape("$InstallDir"), ""
    [Environment]::SetEnvironmentVariable("PATH", $newPath, "User")
    Write-Host "✅ Removed from PATH" -ForegroundColor Green
} else {
    Write-Host "ℹ️  Not found in PATH (may have been removed already)" -ForegroundColor Gray
}

Write-Host "🗂️  Removing installation directory..." -ForegroundColor Blue

# Remove installation directory
try {
    Remove-Item -Path $InstallDir -Recurse -Force
    Write-Host "✅ Removed installation directory" -ForegroundColor Green
} catch {
    Write-Host "❌ Failed to remove installation directory: $_" -ForegroundColor Red
    Write-Host "You may need to remove it manually: $InstallDir" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "✅ Uninstall completed!" -ForegroundColor Green
Write-Host ""
Write-Host "📝 Notes:" -ForegroundColor Cyan
Write-Host "   • Restart your terminal to update PATH" -ForegroundColor Gray
Write-Host "   • Your project config files (.env, config.yaml) were NOT removed" -ForegroundColor Gray
Write-Host "   • To reinstall, run the install script again" -ForegroundColor Gray
Write-Host ""
Write-Host "Thanks for using notion-md-sync! 👋" -ForegroundColor Magenta