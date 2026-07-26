param(
    [string]$AssetDirectory = "release-assets",
    [string]$Version = $env:GITHUB_REF_NAME
)

$ErrorActionPreference = "Stop"
if (-not $Version) { throw "Release version is required" }
$Version = $Version.TrimStart("v")

$assetOS = if ([Runtime.InteropServices.RuntimeInformation]::IsOSPlatform(
        [Runtime.InteropServices.OSPlatform]::Linux)) {
    "linux"
} elseif ([Runtime.InteropServices.RuntimeInformation]::IsOSPlatform(
        [Runtime.InteropServices.OSPlatform]::OSX)) {
    "darwin"
} elseif ([Runtime.InteropServices.RuntimeInformation]::IsOSPlatform(
        [Runtime.InteropServices.OSPlatform]::Windows)) {
    "windows"
} else {
    throw "Unsupported operating system"
}

$assetArch = switch ([Runtime.InteropServices.RuntimeInformation]::OSArchitecture) {
    "X64" { "amd64" }
    "Arm64" { "arm64" }
    default { throw "Unsupported architecture: $([Runtime.InteropServices.RuntimeInformation]::OSArchitecture)" }
}

$extension = if ($assetOS -eq "windows") { "zip" } else { "tar.gz" }
$asset = "claude-configurator_${Version}_${assetOS}_${assetArch}.${extension}"
$archive = Join-Path $AssetDirectory $asset
if (-not (Test-Path $archive)) { throw "Release archive not found: $asset" }

$checksumLine = @(Get-Content (Join-Path $AssetDirectory "checksums.txt") |
    Where-Object { $_ -match "\s$([regex]::Escape($asset))$" })
if ($checksumLine.Count -ne 1) { throw "Expected one checksum for $asset" }
$expected = ($checksumLine[0] -split "\s+")[0].ToLowerInvariant()
$actual = (Get-FileHash $archive -Algorithm SHA256).Hash.ToLowerInvariant()
if ($actual -ne $expected) { throw "Checksum mismatch for $asset" }

$unpackDirectory = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid())
try {
    New-Item -ItemType Directory -Path $unpackDirectory | Out-Null
    if ($assetOS -eq "windows") {
        Expand-Archive $archive -DestinationPath $unpackDirectory
        $binary = Join-Path $unpackDirectory "claude-config.exe"
    } else {
        & tar -xzf $archive -C $unpackDirectory
        if ($LASTEXITCODE -ne 0) { throw "Could not extract $asset" }
        $binary = Join-Path $unpackDirectory "claude-config"
        & chmod +x $binary
        if ($LASTEXITCODE -ne 0) { throw "Could not make $binary executable" }
    }

    $reportedVersion = (& $binary --version).Trim()
    if ($LASTEXITCODE -ne 0 -or $reportedVersion -ne $Version) {
        throw "Version smoke check failed: expected $Version, got $reportedVersion"
    }
    $help = (& $binary --help 2>&1 | Out-String)
    if ($LASTEXITCODE -ne 0 -or $help -notmatch "claude-config") {
        throw "Help smoke check failed"
    }
    Write-Host "Verified $asset"
} finally {
    Remove-Item $unpackDirectory -Recurse -Force -ErrorAction SilentlyContinue
}
