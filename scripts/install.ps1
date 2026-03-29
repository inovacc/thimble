# Thimble installer for Windows.
# Usage: irm https://raw.githubusercontent.com/inovacc/thimble/main/scripts/install.ps1 | iex
$ErrorActionPreference = 'Stop'

$Repo = "inovacc/thimble"
$Binary = "thimble"

# Default install directory.
$InstallDir = if ($env:THIMBLE_INSTALL_DIR) { $env:THIMBLE_INSTALL_DIR } else { "$env:LOCALAPPDATA\Thimble\bin" }

# Detect architecture.
$Arch = if ([Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }
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

# Build download URL.
$Version = $Tag.TrimStart('v')
$Asset = "${Binary}_${Version}_windows_${Arch}.zip"
$Url = "https://github.com/$Repo/releases/download/$Tag/$Asset"

# Download and extract.
$TmpDir = Join-Path $env:TEMP "thimble-install-$(Get-Random)"
New-Item -ItemType Directory -Path $TmpDir -Force | Out-Null

try {
    Write-Host "Downloading $Url..."
    Invoke-WebRequest -Uri $Url -OutFile (Join-Path $TmpDir $Asset)

    Write-Host "Extracting..."
    Expand-Archive -Path (Join-Path $TmpDir $Asset) -DestinationPath $TmpDir -Force

    # Install binary.
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Copy-Item -Path (Join-Path $TmpDir "$Binary.exe") -Destination (Join-Path $InstallDir "$Binary.exe") -Force

    # Add to PATH if not already present.
    $UserPath = [Environment]::GetEnvironmentVariable('PATH', 'User')
    if ($UserPath -notlike "*$InstallDir*") {
        [Environment]::SetEnvironmentVariable('PATH', "$UserPath;$InstallDir", 'User')
        $env:PATH = "$env:PATH;$InstallDir"
        Write-Host "Added $InstallDir to user PATH."
    }

    Write-Host ""
    Write-Host "Installed $Binary $Tag to $InstallDir\$Binary.exe"
    Write-Host ""

    # Configure npm for GitHub Packages if npm is available.
    if (Get-Command npm -ErrorAction SilentlyContinue) {
        $Npmrc = Join-Path $env:USERPROFILE ".npmrc"
        $NpmrcContent = if (Test-Path $Npmrc) { Get-Content $Npmrc -Raw } else { "" }
        if ($NpmrcContent -notlike "*@inovacc:registry*") {
            Add-Content $Npmrc "@inovacc:registry=https://npm.pkg.github.com"
            Write-Host "Configured npm for @inovacc GitHub Packages."
        }
    }

    Write-Host "Next steps:"
    Write-Host "  claude plugin install thimble@npm:@inovacc/thimble   # Register as Claude Code plugin"
    Write-Host "  thimble setup --client claude                        # Or configure hooks manually"
    Write-Host "  thimble doctor                                       # Run diagnostic checks"
    Write-Host "  thimble --help                                       # Show all commands"
}
finally {
    Remove-Item -Path $TmpDir -Recurse -Force -ErrorAction SilentlyContinue
}
