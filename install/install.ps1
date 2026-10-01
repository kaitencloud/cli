<#
.SYNOPSIS
    Installs the Kaiten CLI from its GitHub releases on Windows.

.DESCRIPTION
    Latest release, into $Env:LOCALAPPDATA\Programs\kaiten (no administrator rights needed):

        irm https://raw.githubusercontent.com/kaitencloud/cli/main/install/install.ps1 | iex

    A specific version or directory, by saving the script and running it with parameters:

        .\install.ps1 -Version 1.2.3 -InstallDir C:\tools\kaiten

    The archive's SHA-256 is verified against the release's checksums.txt before anything is installed.

.PARAMETER Version
    Version to install, with or without the leading v. Defaults to $Env:KAITEN_VERSION, then the latest release.

.PARAMETER InstallDir
    Directory to install kaiten.exe into. Defaults to $Env:KAITEN_INSTALL_DIR, then $Env:LOCALAPPDATA\Programs\kaiten.
    The directory is added to the user PATH when it is not there yet.

.PARAMETER DownloadUrl
    Base URL serving the release assets, for mirrors. Defaults to $Env:KAITEN_DOWNLOAD_URL, then the GitHub release.
#>
param (
    [string]$Version = $Env:KAITEN_VERSION,
    [string]$InstallDir = $(if ($Env:KAITEN_INSTALL_DIR) { $Env:KAITEN_INSTALL_DIR } else { Join-Path $Env:LOCALAPPDATA 'Programs\kaiten' }),
    [string]$DownloadUrl = $Env:KAITEN_DOWNLOAD_URL
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$Repo = 'kaitencloud/cli'
$Binary = 'kaiten.exe'

$arch = switch ($Env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { 'amd64' }
    'ARM64' { 'arm64' }
    default { throw "Unsupported architecture: $Env:PROCESSOR_ARCHITECTURE (releases ship amd64 and arm64)" }
}

$asset = "kaiten_windows_$arch.zip"
if ($DownloadUrl) {
    $base = $DownloadUrl.TrimEnd('/')
} elseif ($Version) {
    if (-not $Version.StartsWith('v')) { $Version = "v$Version" }
    $base = "https://github.com/$Repo/releases/download/$Version"
} else {
    $base = "https://github.com/$Repo/releases/latest/download"
}

$tmp = Join-Path ([IO.Path]::GetTempPath()) ('kaiten-install-' + [IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
    Write-Host "Downloading $base/$asset"
    Invoke-WebRequest -UseBasicParsing -Uri "$base/$asset" -OutFile (Join-Path $tmp $asset)
    Invoke-WebRequest -UseBasicParsing -Uri "$base/checksums.txt" -OutFile (Join-Path $tmp 'checksums.txt')

    $expected = Get-Content (Join-Path $tmp 'checksums.txt') |
        Where-Object { ($_ -split '\s+')[1] -eq $asset } |
        ForEach-Object { ($_ -split '\s+')[0] } |
        Select-Object -First 1
    if (-not $expected) { throw "checksums.txt has no entry for $asset" }

    $actual = (Get-FileHash -Algorithm SHA256 (Join-Path $tmp $asset)).Hash
    if ($actual.ToLowerInvariant() -ne $expected.ToLowerInvariant()) {
        throw "Checksum mismatch for ${asset}: expected $expected, got $actual"
    }

    Expand-Archive -Force -Path (Join-Path $tmp $asset) -DestinationPath $tmp
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Copy-Item -Force (Join-Path $tmp $Binary) (Join-Path $InstallDir $Binary)
} finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}

$exe = Join-Path $InstallDir $Binary
Write-Host "Installed $(& $exe version) to $exe"

$userPath = [Environment]::GetEnvironmentVariable('PATH', 'User')
if (($userPath -split ';') -notcontains $InstallDir) {
    $newPath = if ($userPath) { "$userPath;$InstallDir" } else { $InstallDir }
    [Environment]::SetEnvironmentVariable('PATH', $newPath, 'User')
    $Env:PATH = "$Env:PATH;$InstallDir"
    Write-Host "Added $InstallDir to your user PATH. Open a new terminal for it to take effect."
}
Write-Host "Run 'kaiten config set base-url <url>' and 'kaiten doctor' to get started."
