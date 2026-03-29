# Thimble installer for Windows.
# Usage: irm https://raw.githubusercontent.com/inovacc/thimble/main/scripts/install.ps1 | iex
$ErrorActionPreference = 'Stop'

$Repo = "inovacc/thimble"
$PluginDir = if ($env:THIMBLE_PLUGIN_DIR) { $env:THIMBLE_PLUGIN_DIR } else { "$env:LOCALAPPDATA\Thimble\plugin" }

# Detect architecture.
$Arch = if ([Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'x86_64' }
} else {
    Write-Error "32-bit Windows is not supported."
    exit 1
}

# Get latest release tag.
Write-Host "Fetching latest release..."
$Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
$Tag = $Release.tag_name
if (-not $Tag) {
    Write-Error "Failed to determine latest release."
    exit 1
}
Write-Host "Latest release: $Tag"

# Download plugin archive (binary + .claude-plugin/ + skills/ + hooks/ + agents/ + .mcp.json).
$Asset = "thimble-plugin_Windows_${Arch}.zip"
$Url = "https://github.com/$Repo/releases/download/$Tag/$Asset"

$TmpDir = Join-Path $env:TEMP "thimble-install-$(Get-Random)"
New-Item -ItemType Directory -Path $TmpDir -Force | Out-Null

try {
    Write-Host "Downloading $Url..."
    Invoke-WebRequest -Uri $Url -OutFile (Join-Path $TmpDir $Asset)

    Write-Host "Extracting to $PluginDir..."
    New-Item -ItemType Directory -Path $PluginDir -Force | Out-Null
    & "$env:SystemRoot\System32\tar.exe" -xf (Join-Path $TmpDir $Asset) -C $PluginDir 2>$null

    # Add plugin dir to PATH for CLI access.
    $UserPath = [Environment]::GetEnvironmentVariable('PATH', 'User')
    if ($UserPath -notlike "*$PluginDir*") {
        [Environment]::SetEnvironmentVariable('PATH', "$UserPath;$PluginDir", 'User')
        $env:PATH = "$env:PATH;$PluginDir"
        Write-Host "Added $PluginDir to user PATH."
    }

    Write-Host ""
    Write-Host "Installed thimble $Tag to $PluginDir"
    Write-Host ""
    Write-Host "Activate in Claude Code:"
    Write-Host ""
    Write-Host "  claude --plugin-dir `"$PluginDir`""
    Write-Host ""
    Write-Host "Other commands:"
    Write-Host "  thimble doctor    # Run diagnostic checks"
    Write-Host "  thimble --help    # Show all commands"
}
finally {
    Remove-Item -Path $TmpDir -Recurse -Force -ErrorAction SilentlyContinue
}
