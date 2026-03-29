# Thimble installer for Windows.
# Usage: irm https://raw.githubusercontent.com/inovacc/thimble/main/scripts/install.ps1 | iex
$ErrorActionPreference = 'Stop'

$Repo = "inovacc/thimble"
$AppDir = if ($env:THIMBLE_INSTALL_DIR) { $env:THIMBLE_INSTALL_DIR } else { "$env:LOCALAPPDATA\Thimble" }
$PluginDir = Join-Path $AppDir "plugins"

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

# Download plugin archive (binary + plugin assets).
$Asset = "thimble-plugin_Windows_${Arch}.zip"
$Url = "https://github.com/$Repo/releases/download/$Tag/$Asset"

$TmpDir = Join-Path $env:TEMP "thimble-install-$(Get-Random)"
New-Item -ItemType Directory -Path $TmpDir -Force | Out-Null

try {
    Write-Host "Downloading $Url..."
    Invoke-WebRequest -Uri $Url -OutFile (Join-Path $TmpDir $Asset)

    # Extract to temp (ignore harmless '.' entry warning from GoReleaser zips).
    $savedPref = $ErrorActionPreference
    $ErrorActionPreference = 'SilentlyContinue'
    & "$env:SystemRoot\System32\tar.exe" -xf (Join-Path $TmpDir $Asset) -C $TmpDir 2>&1 | Out-Null
    $ErrorActionPreference = $savedPref

    # Install binary to app dir.
    New-Item -ItemType Directory -Path $AppDir -Force | Out-Null
    Copy-Item -Path (Join-Path $TmpDir "thimble.exe") -Destination (Join-Path $AppDir "thimble.exe") -Force

    # Install plugin assets to plugins dir.
    New-Item -ItemType Directory -Path $PluginDir -Force | Out-Null
    foreach ($item in @(".claude-plugin", ".mcp.json", "hooks", "skills", "agents", "scripts", "LICENSE")) {
        $src = Join-Path $TmpDir $item
        if (Test-Path $src) {
            $dst = Join-Path $PluginDir $item
            if (Test-Path $src -PathType Container) {
                Copy-Item -Path $src -Destination $dst -Recurse -Force
            } else {
                Copy-Item -Path $src -Destination $dst -Force
            }
        }
    }
    # Copy binary into plugins dir too (needed for MCP server).
    Copy-Item -Path (Join-Path $AppDir "thimble.exe") -Destination (Join-Path $PluginDir "thimble.exe") -Force

    # Add app dir to PATH for CLI access.
    $UserPath = [Environment]::GetEnvironmentVariable('PATH', 'User')
    if ($UserPath -notlike "*$AppDir*") {
        [Environment]::SetEnvironmentVariable('PATH', "$UserPath;$AppDir", 'User')
        $env:PATH = "$env:PATH;$AppDir"
        Write-Host "Added $AppDir to user PATH."
    }

    Write-Host ""
    Write-Host "Installed thimble $Tag"
    Write-Host "  Binary:  $AppDir\thimble.exe"
    Write-Host "  Plugins: $PluginDir"
    Write-Host ""
    Write-Host "Run 'thimble setup' to configure your AI coding assistant."
}
finally {
    Remove-Item -Path $TmpDir -Recurse -Force -ErrorAction SilentlyContinue
}
