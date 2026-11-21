# Copyright (c) 2026 Tiago Vilas Boas.
# SPDX-License-Identifier: Apache-2.0
#
# Downshift Windows installer
# Usage:
#   irm https://raw.githubusercontent.com/tiagovilasboas/downshift-ai/main/install.ps1 | iex
# Or with parameters after downloading:
#   .\install.ps1 -Version v0.20.0

[CmdletBinding()]
param(
    [string]$Version = "",
    [string]$InstallDir = $env:DOWNSHIFT_INSTALL_DIR
)

$ErrorActionPreference = "Stop"
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$Repo = "tiagovilasboas/downshift-ai"
$BaseUrl = "https://github.com/$Repo"

function Resolve-LatestTag {
    try {
        $req = [System.Net.HttpWebRequest]::Create("$BaseUrl/releases/latest")
        $req.AllowAutoRedirect = $false
        $resp = $req.GetResponse()
        $loc = $resp.Headers["Location"]
        $resp.Close()
        if ($loc -match '/releases/tag/([^/?#]+)$') {
            return $Matches[1]
        }
    } catch [System.Net.WebException] {
        $r = $_.Exception.Response
        if ($null -ne $r) {
            $loc = $r.Headers["Location"]
            $r.Close()
            if ($loc -match '/releases/tag/([^/?#]+)$') {
                return $Matches[1]
            }
        }
    }
    try {
        $atom = (Invoke-WebRequest -Uri "$BaseUrl/releases.atom" -UseBasicParsing).Content
        if ($atom -match '/releases/tag/([^"<]+)') {
            return $Matches[1]
        }
    } catch {}
    return ""
}

$arch = $env:PROCESSOR_ARCHITECTURE
if ($env:PROCESSOR_ARCHITEW6432) { $arch = $env:PROCESSOR_ARCHITEW6432 }
if ($arch -ne "AMD64") {
    Write-Error "Unsupported Windows architecture '$arch'. Release archives target windows/amd64; build from source with: go install github.com/$Repo/cmd/downshift@latest"
}

if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = Resolve-LatestTag
}
if ([string]::IsNullOrWhiteSpace($Version)) {
    Write-Error "Could not determine latest version. Pass -Version (e.g. v0.20.0) or install with: go install github.com/$Repo/cmd/downshift@latest"
}
if ($Version -notmatch '^v\d+\.\d+\.\d+(-[0-9A-Za-z.]+)?$') {
    Write-Error "Invalid version '$Version' (expected vMAJOR.MINOR.PATCH[-prerelease])."
}

$verNoV = $Version.Substring(1)
$archive = "downshift_${verNoV}_windows_amd64.zip"
$archiveUrl = "$BaseUrl/releases/download/$Version/$archive"
$checksumsUrl = "$BaseUrl/releases/download/$Version/checksums.txt"

if ([string]::IsNullOrWhiteSpace($InstallDir)) {
    $existing = Get-Command downshift -ErrorAction SilentlyContinue
    if ($null -ne $existing -and $existing.Source -and (Test-Path $existing.Source)) {
        $InstallDir = Split-Path -Parent $existing.Source
        Write-Host "Updating existing downshift on PATH: $($existing.Source)"
    } else {
        $InstallDir = Join-Path $env:LOCALAPPDATA "downshift\bin"
    }
}
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("downshift-install-" + [System.Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null

try {
    $zipPath = Join-Path $tmp $archive
    Write-Host "Downloading downshift $Version (windows/amd64)..."
    Invoke-WebRequest -Uri $archiveUrl -OutFile $zipPath -UseBasicParsing

    if ($env:DOWNSHIFT_SKIP_CHECKSUM -eq "1") {
        Write-Warning "Checksum verification skipped (DOWNSHIFT_SKIP_CHECKSUM=1)."
    } else {
        $chkPath = Join-Path $tmp "checksums.txt"
        Invoke-WebRequest -Uri $checksumsUrl -OutFile $chkPath -UseBasicParsing
        $expected = ""
        foreach ($line in Get-Content $chkPath) {
            $parts = ($line -split '\s+') | Where-Object { $_ }
            if ($parts.Count -ge 2 -and $parts[1].TrimStart('*') -eq $archive) {
                $expected = $parts[0].ToLowerInvariant()
                break
            }
        }
        if (-not $expected) {
            Write-Error "$archive not listed in checksums.txt."
        }
        $actual = (Get-FileHash -Path $zipPath -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($actual -ne $expected) {
            Write-Error "Checksum mismatch for $archive (expected $expected, got $actual)."
        }
        Write-Host "Checksum verified (sha256)."
    }

    if ($env:DOWNSHIFT_SKIP_ATTESTATION -ne "1") {
        $gh = Get-Command gh -ErrorAction SilentlyContinue
        if ($null -ne $gh) {
            & $gh.Source attestation verify $zipPath --repo $Repo *> $null
            if ($LASTEXITCODE -eq 0) {
                Write-Host "Build provenance verified."
            } elseif ($env:DOWNSHIFT_REQUIRE_ATTESTATION -eq "1") {
                Write-Error "Build provenance attestation verification failed (DOWNSHIFT_REQUIRE_ATTESTATION=1)."
            } else {
                Write-Warning "Build provenance check skipped/unverified (sha256 passed)."
            }
        } elseif ($env:DOWNSHIFT_REQUIRE_ATTESTATION -eq "1") {
            Write-Error "gh CLI is required when DOWNSHIFT_REQUIRE_ATTESTATION=1."
        }
    }

    $extractDir = Join-Path $tmp "extract"
    Expand-Archive -Path $zipPath -DestinationPath $extractDir -Force
    $srcExe = Join-Path $extractDir "downshift.exe"
    if (-not (Test-Path $srcExe)) {
        Write-Error "downshift.exe missing from $archive."
    }
    $destExe = Join-Path $InstallDir "downshift.exe"
    Copy-Item -Path $srcExe -Destination $destExe -Force

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $segments = @()
    if ($userPath) { $segments = $userPath -split ';' }
    if ($segments -notcontains $InstallDir) {
        $newPath = if ($userPath) { "$userPath;$InstallDir" } else { $InstallDir }
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        $env:Path = "$env:Path;$InstallDir"
        Write-Host "Added $InstallDir to your User PATH (open a new terminal if needed)."
    }

    Write-Host "Installed $destExe ($Version)."
    Write-Host "Next: run 'downshift setup' and 'downshift doctor'."
} finally {
    Remove-Item -Recurse -Force -Path $tmp -ErrorAction SilentlyContinue
}
