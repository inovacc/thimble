# Copyright (c) 2026 dyammarcano. All rights reserved.
#
# Redistribution and use in source and binary forms, with or without
# modification, are permitted provided that the following conditions are met:
#
# 1. Redistributions of source code must retain the above copyright notice,
#    this list of conditions and the following disclaimer.
#
# 2. Redistributions in binary form must reproduce the above copyright notice,
#    this list of conditions and the following disclaimer in the documentation
#    and/or other materials provided with the distribution.
#
# 3. Neither the name of the copyright holder nor the names of its
#    contributors may be used to endorse or promote products derived from
#    this software without specific prior written permission.
#
# THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
# AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
# IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
# ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
# LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
# CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
# SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
# INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
# CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
# ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
# POSSIBILITY OF SUCH DAMAGE.
#
# Install script for thimble (Windows PowerShell)
#
# Usage:
#   irm https://raw.githubusercontent.com/inovacc/thimble/main/install.ps1 | iex
#
# Environment variables:
#   THIMBLE_VERSION      - Install a specific version (e.g., "v1.0.0"). Default: latest.
#   THIMBLE_INSTALL_DIR  - Override install directory. Default: $env:LOCALAPPDATA\thimble.

$ErrorActionPreference = "Stop"

$Repo = "inovacc/thimble"
$GitHubApi = "https://api.github.com"

function Get-ThimbleArch {
    $arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
    switch ($arch) {
        "X64"   { return "x86_64" }
        "Arm64" { return "arm64" }
        default { throw "Unsupported architecture: $arch" }
    }
}

function Get-InstallDir {
    if ($env:THIMBLE_INSTALL_DIR) {
        return $env:THIMBLE_INSTALL_DIR
    }
    return Join-Path $env:LOCALAPPDATA "thimble"
}

function Resolve-ThimbleVersion {
    if ($env:THIMBLE_VERSION) {
        $v = $env:THIMBLE_VERSION
        if (-not $v.StartsWith("v")) { $v = "v$v" }
        return $v
    }

    Write-Host "Fetching latest release version..."
    $release = Invoke-RestMethod -Uri "$GitHubApi/repos/$Repo/releases/latest" -Headers @{ "User-Agent" = "thimble-installer" }
    if (-not $release.tag_name) {
        throw "Failed to determine latest version. Set THIMBLE_VERSION to install a specific version."
    }
    return $release.tag_name
}

function Get-ReleaseAssets {
    param([string]$Version)
    $release = Invoke-RestMethod -Uri "$GitHubApi/repos/$Repo/releases/tags/$Version" -Headers @{ "User-Agent" = "thimble-installer" }
    return $release.assets
}

function Get-AssetUrl {
    param([object[]]$Assets, [string]$FileName)
    $asset = $Assets | Where-Object { $_.name -eq $FileName } | Select-Object -First 1
    if (-not $asset) {
        throw "Could not find asset '$FileName' in release"
    }
    return $asset.browser_download_url
}

function Test-Checksum {
    param([string]$FilePath, [string]$ChecksumsPath)

    $fileName = Split-Path $FilePath -Leaf
    $checksumLine = Get-Content $ChecksumsPath | Where-Object { $_ -match $fileName } | Select-Object -First 1

    if (-not $checksumLine) {
        throw "Checksum not found for $fileName in checksums.txt"
    }

    $expected = ($checksumLine -split '\s+')[0]
    $actual = (Get-FileHash -Path $FilePath -Algorithm SHA256).Hash.ToLower()

    if ($actual -ne $expected) {
        throw "Checksum mismatch for $fileName`n  expected: $expected`n  actual:   $actual"
    }

    Write-Host "Checksum verified."
}

function Add-ToPath {
    param([string]$Dir)

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -split ";" | Where-Object { $_ -eq $Dir }) {
        return
    }

    $newPath = "$userPath;$Dir"
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    $env:Path = "$env:Path;$Dir"
    Write-Host "Added $Dir to user PATH."
}

function Install-Thimble {
    $arch = Get-ThimbleArch
    $installDir = Get-InstallDir
    $version = Resolve-ThimbleVersion

    $archiveName = "thimble_Windows_${arch}.zip"
    $checksumsName = "checksums.txt"

    Write-Host "Installing thimble $version (Windows/$arch)..."

    # Create temp directory
    $tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) "thimble-install-$([System.Guid]::NewGuid().ToString('N').Substring(0,8))"
    New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

    try {
        # Get release assets
        $assets = Get-ReleaseAssets -Version $version

        # Download archive and checksums
        $archiveUrl = Get-AssetUrl -Assets $assets -FileName $archiveName
        $checksumsUrl = Get-AssetUrl -Assets $assets -FileName $checksumsName

        $archivePath = Join-Path $tmpDir $archiveName
        $checksumsPath = Join-Path $tmpDir $checksumsName

        Write-Host "Downloading $archiveName..."
        Invoke-WebRequest -Uri $archiveUrl -OutFile $archivePath -UseBasicParsing

        Write-Host "Downloading checksums.txt..."
        Invoke-WebRequest -Uri $checksumsUrl -OutFile $checksumsPath -UseBasicParsing

        # Verify checksum
        Test-Checksum -FilePath $archivePath -ChecksumsPath $checksumsPath

        # Extract
        Write-Host "Extracting thimble binary..."
        $extractDir = Join-Path $tmpDir "extracted"
        Expand-Archive -Path $archivePath -DestinationPath $extractDir -Force

        $binaryPath = Join-Path $extractDir "thimble.exe"
        if (-not (Test-Path $binaryPath)) {
            throw "Binary 'thimble.exe' not found in archive"
        }

        # Install
        if (-not (Test-Path $installDir)) {
            New-Item -ItemType Directory -Path $installDir -Force | Out-Null
        }

        $destPath = Join-Path $installDir "thimble.exe"
        Copy-Item -Path $binaryPath -Destination $destPath -Force

        # Add to PATH if needed
        Add-ToPath -Dir $installDir

        Write-Host ""
        Write-Host "thimble $version installed successfully to $destPath"
    }
    finally {
        # Cleanup
        if (Test-Path $tmpDir) {
            Remove-Item -Path $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

Install-Thimble
