# Repro Windows Installer Script
# Requires PowerShell 5.1 or newer

$ErrorActionPreference = 'Stop'

$Repo = "BaimPriyatna/repro"

# 1. Architecture Detection (amd64, arm64)
$rawArch = $env:PROCESSOR_ARCHITECTURE
if ($env:PROCESSOR_ARCHITEW6432) {
    $rawArch = $env:PROCESSOR_ARCHITEW6432
}
if (-not $rawArch -and [System.Environment]::Is64BitOperatingSystem) {
    $rawArch = "AMD64"
}

switch -Regex ($rawArch) {
    "^(AMD64|x86_64|x64)$" { $arch = "amd64" }
    "^(ARM64|aarch64)$"    { $arch = "arm64" }
    default {
        Write-Error "Unsupported architecture: '$rawArch'. Only amd64 and arm64 are supported."
        exit 1
    }
}

# 2. Determine Version
if ($env:REPRO_VERSION) {
    $version = $env:REPRO_VERSION
} else {
    Write-Host "Checking latest release of $Repo..."
    try {
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
        $releaseApiUrl = "https://api.github.com/repos/$Repo/releases/latest"
        $releaseData = Invoke-RestMethod -Uri $releaseApiUrl -Headers @{ "User-Agent" = "ReproInstaller" }
        $version = $releaseData.tag_name
    } catch {
        Write-Error "Failed to detect latest release from GitHub API: $_"
        Write-Host "You can specify the version explicitly by setting `$env:REPRO_VERSION = 'v1.1.0' prior to running the installer."
        exit 1
    }
}

if (-not $version.StartsWith("v")) {
    $version = "v$version"
}

$archiveName = "repro_windows_${arch}.zip"
$checksumsName = "checksums.txt"
$baseUrl = "https://github.com/$Repo/releases/download/$version"
$downloadUrl = "$baseUrl/$archiveName"
$checksumsUrl = "$baseUrl/$checksumsName"

# 3. Create Temporary Directory
$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("repro-install-" + [System.Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

try {
    # 4. Download Release Archive and Checksums
    $archivePath = Join-Path $tempDir $archiveName
    $checksumsPath = Join-Path $tempDir $checksumsName

    Write-Host "Downloading $archiveName ($version)..."
    Invoke-WebRequest -Uri $downloadUrl -OutFile $archivePath -UseBasicParsing

    Write-Host "Downloading $checksumsName..."
    Invoke-WebRequest -Uri $checksumsUrl -OutFile $checksumsPath -UseBasicParsing

    # 5. Checksum Verification
    Write-Host "Verifying SHA-256 checksum..."
    $actualHash = (Get-FileHash -Path $archivePath -Algorithm SHA256).Hash.ToLower()

    $matchingLine = Get-Content -Path $checksumsPath | Where-Object { $_ -match [regex]::Escape($archiveName) }
    if (-not $matchingLine) {
        Write-Error "Checksum entry for '$archiveName' not found in $checksumsName."
        exit 1
    }

    $expectedHash = ($matchingLine.Trim() -split '\s+')[0].ToLower()

    if ($actualHash -ne $expectedHash) {
        Write-Error "SHA-256 verification failed!`nExpected: $expectedHash`nActual:   $actualHash"
        exit 1
    }
    Write-Host "Checksum verified successfully."

    # 6. Extract Archive
    $extractDir = Join-Path $tempDir "extracted"
    Write-Host "Extracting $archiveName..."
    Expand-Archive -Path $archivePath -DestinationPath $extractDir -Force

    $binarySource = Join-Path $extractDir "repro.exe"
    if (-not (Test-Path $binarySource)) {
        Write-Error "Binary 'repro.exe' was not found in the extracted archive."
        exit 1
    }

    # 7. Install to User-Local Directory
    $homeDir = [System.Environment]::GetFolderPath('UserProfile')
    if (-not $homeDir) {
        $homeDir = $HOME
    }
    $installDir = Join-Path $homeDir ".repro\bin"

    if (-not (Test-Path $installDir)) {
        New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    }

    $targetExe = Join-Path $installDir "repro.exe"
    Write-Host "Installing repro.exe to $installDir..."
    Copy-Item -Path $binarySource -Destination $targetExe -Force
    Write-Host "Repro $version installed successfully to $targetExe"

    # 8. Check PATH Setup
    $userPath = [System.Environment]::GetEnvironmentVariable("Path", "User")
    $pathList = ($userPath -split ';') | ForEach-Object { $_.Trim().TrimEnd('\') }
    $normInstall = $installDir.Trim().TrimEnd('\')

    if ($pathList -notcontains $normInstall) {
        Write-Host ""
        Write-Host "Notice: '$installDir' is not currently in your User PATH." -ForegroundColor Yellow
        Write-Host "To add it permanently to your User PATH, run:" -ForegroundColor Cyan
        Write-Host "  [System.Environment]::SetEnvironmentVariable('Path', [System.Environment]::GetEnvironmentVariable('Path', 'User') + ';$installDir', 'User')" -ForegroundColor Cyan
        Write-Host "To use 'repro' in your current PowerShell session, run:" -ForegroundColor Cyan
        Write-Host "  `$env:Path += ';$installDir'" -ForegroundColor Cyan
    }
}
finally {
    # 9. Clean Temporary Files
    if (Test-Path $tempDir) {
        Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}
