<#
.SYNOPSIS
    Downloads the ggufget single-file binary and puts it on your PATH.

.DESCRIPTION
    The only job of this script is installation: download the release asset for
    this machine, check its SHA256 against SHA256SUMS, copy it into a private
    folder and add that folder to the user PATH. No model logic lives here --
    the binary does all of that, so nothing has to be kept in sync.

    Works on Windows PowerShell 5.1 and PowerShell 7. No admin rights needed:
    everything lands under %LOCALAPPDATA%.

.PARAMETER Version
    Release version to install, without the leading "v". Default 1.2.2.

.PARAMETER Repo
    GitHub repository that publishes the releases ("owner/name").

.PARAMETER InstallDir
    Where to put ggufget.exe. Default %LOCALAPPDATA%\Programs\ggufget.

.PARAMETER NoPath
    Skip editing the user PATH (useful for CI).

.PARAMETER Force
    Reinstall even if the same version is already present.

.EXAMPLE
    irm https://raw.githubusercontent.com/usre13243296/ggufget/main/install/install.ps1 | iex

.EXAMPLE
    .\install.ps1 -Version 1.2.2 -InstallDir D:\tools\ggufget
#>
[CmdletBinding()]
param(
    [string]$Version = "1.2.2",
    [string]$Repo = "usre13243296/ggufget",
    [string]$InstallDir = "",
    [switch]$NoPath,
    [switch]$Force
)

$ErrorActionPreference = "Stop"
# Invoke-WebRequest is slow when it tries to draw progress bars in 5.1.
$ProgressPreference = "SilentlyContinue"
# Windows PowerShell 5.1 still defaults to TLS 1.0 on some builds.
try {
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
} catch { }

function Write-Step($message) { Write-Host "==> $message" -ForegroundColor Cyan }
function Write-Ok($message) { Write-Host "  ok $message" -ForegroundColor Green }
function Fail($message) {
    Write-Host "  x  $message" -ForegroundColor Red
    exit 1
}

if (-not $InstallDir) {
    $InstallDir = Join-Path $env:LOCALAPPDATA "Programs\ggufget"
}

# ---------------------------------------------------------------- architecture
$arch = "amd64"
if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64" -or $env:PROCESSOR_ARCHITEW6432 -eq "ARM64") {
    $arch = "arm64"
}

$asset = "ggufget_${Version}_windows_${arch}.exe"
$base = "https://github.com/$Repo/releases/download/v$Version"
$exe = Join-Path $InstallDir "ggufget.exe"

Write-Step "ggufget $Version for windows/$arch"

if ((Test-Path $exe) -and -not $Force) {
    $current = ""
    try { $current = (& $exe --version) 2>&1 | Out-String } catch { }
    if ($current -match [regex]::Escape($Version)) {
        Write-Ok "already installed at $exe"
        Write-Host "     re-run with -Force to replace it." -ForegroundColor DarkGray
        exit 0
    }
}

# ------------------------------------------------------------------- download
$tmp = Join-Path ([IO.Path]::GetTempPath()) ("ggufget-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tmp -Force | Out-Null
try {
    $assetPath = Join-Path $tmp $asset
    $sumsPath = Join-Path $tmp "SHA256SUMS"

    Write-Step "downloading $asset"
    try {
        Invoke-WebRequest -Uri "$base/$asset" -OutFile $assetPath -UseBasicParsing
    } catch {
        Fail "download failed: $($_.Exception.Message)`n     If github.com is slow or blocked, download the asset manually and copy it to $exe"
    }

    # ------------------------------------------------------------- verification
    $expected = ""
    try {
        Invoke-WebRequest -Uri "$base/SHA256SUMS" -OutFile $sumsPath -UseBasicParsing
        $line = Select-String -Path $sumsPath -Pattern ([regex]::Escape($asset) + '$') | Select-Object -First 1
        if ($line) {
            $expected = ($line.Line -split '\s+')[0]
        }
    } catch {
        Write-Host "  !  SHA256SUMS not available, skipping verification" -ForegroundColor Yellow
    }

    if ($expected) {
        $actual = (Get-FileHash -Path $assetPath -Algorithm SHA256).Hash
        if ($actual -ne $expected.ToUpperInvariant()) {
            Fail "checksum mismatch`n     expected $expected`n     actual   $actual"
        }
        Write-Ok "sha256 $($actual.Substring(0, 16))..."
    } else {
        Write-Host "  !  no checksum published for $asset" -ForegroundColor Yellow
    }

    # ---------------------------------------------------------------- install
    Write-Step "installing to $InstallDir"
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    if (Test-Path $exe) { Remove-Item $exe -Force }
    Copy-Item $assetPath $exe
    Unblock-File $exe -ErrorAction SilentlyContinue
    Write-Ok "$exe"

    # ------------------------------------------------------------------- PATH
    if (-not $NoPath) {
        $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
        $parts = @()
        if ($userPath) { $parts = $userPath -split ';' }
        $normalized = $parts | ForEach-Object { $_.TrimEnd('\') }
        if ($normalized -notcontains $InstallDir.TrimEnd('\')) {
            $newPath = $userPath
            if ($newPath -and -not $newPath.EndsWith(';')) { $newPath += ';' }
            $newPath += $InstallDir
            [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
            Write-Ok "added to user PATH (open a new terminal to use it)"
        } else {
            Write-Ok "already on user PATH"
        }
        # make it usable in this very session too
        $env:Path = $env:Path + ";" + $InstallDir
    }

    Write-Host ""
    Write-Host "Done. Try it:" -ForegroundColor Cyan
    Write-Host "  ggufget doctor"
    Write-Host "  ggufget list lmstudio-community/Qwen2.5-7B-Instruct-GGUF"
    Write-Host "  ggufget get  lmstudio-community/Qwen2.5-7B-Instruct-GGUF -f 2"
} finally {
    Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
}
