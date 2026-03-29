# Marketplace launcher for thimble (Windows).
# Auto-downloads the binary on first use, then proxies all args.
param([Parameter(ValueFromRemainingArguments=$true)]$Args)

$ErrorActionPreference = "Stop"

$Repo = "inovacc/thimble"
$InstallDir = if ($env:THIMBLE_INSTALL_DIR) { $env:THIMBLE_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "thimble" }
$Bin = Join-Path $InstallDir "thimble.exe"

# If binary exists, just run it.
if (Test-Path $Bin) {
    & $Bin @Args
    exit $LASTEXITCODE
}

# Detect architecture.
$Arch = if ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq "Arm64") { "arm64" } else { "x86_64" }
$AssetName = "thimble_Windows_${Arch}.zip"

# Resolve version.
if ($env:THIMBLE_VERSION) {
    $Version = $env:THIMBLE_VERSION -replace "^v", ""
} else {
    $Release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
    $Version = $Release.tag_name -replace "^v", ""
}

Write-Host "thimble: downloading v$Version for Windows/$Arch..." -ForegroundColor Cyan

$TmpDir = Join-Path ([System.IO.Path]::GetTempPath()) "thimble-install-$([guid]::NewGuid())"
New-Item -ItemType Directory -Path $TmpDir -Force | Out-Null

try {
    # Download archive + checksums.
    $ArchivePath = Join-Path $TmpDir $AssetName
    Invoke-WebRequest "https://github.com/$Repo/releases/download/v$Version/$AssetName" -OutFile $ArchivePath
    $ChecksumPath = Join-Path $TmpDir "checksums.txt"
    Invoke-WebRequest "https://github.com/$Repo/releases/download/v$Version/checksums.txt" -OutFile $ChecksumPath

    # Verify checksum.
    $Expected = (Get-Content $ChecksumPath | Where-Object { $_ -match $AssetName } | ForEach-Object { ($_ -split "\s+")[0] })
    $Actual = (Get-FileHash $ArchivePath -Algorithm SHA256).Hash.ToLower()

    if ($Expected -and $Expected -ne $Actual) {
        throw "Checksum mismatch: expected $Expected, got $Actual"
    }

    # Extract and install.
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Expand-Archive -Path $ArchivePath -DestinationPath $TmpDir -Force
    Copy-Item (Join-Path $TmpDir "thimble.exe") $Bin -Force

    Write-Host "thimble: installed to $Bin" -ForegroundColor Green

    # Run setup.
    try { & $Bin setup --client claude --plugin 2>$null } catch {}

} finally {
    Remove-Item $TmpDir -Recurse -Force -ErrorAction SilentlyContinue
}

# Run with original args.
& $Bin @Args
exit $LASTEXITCODE
