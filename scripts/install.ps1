param(
    [string]$Version = "latest",
    [string]$InstallDir = "$([Environment]::GetFolderPath('LocalApplicationData'))\Programs\claude-configurator\bin"
)

$ErrorActionPreference = "Stop"
$repository = "ex3lite/claude-configurator"
$architecture = switch ([Runtime.InteropServices.RuntimeInformation]::OSArchitecture) {
    "Arm64" { "arm64" }
    "X64" { "amd64" }
    default { throw "Unsupported architecture: $([Runtime.InteropServices.RuntimeInformation]::OSArchitecture)" }
}

if ($Version -eq "latest") {
    $release = Invoke-RestMethod "https://api.github.com/repos/$repository/releases/latest"
    $releaseTag = $release.tag_name
} else {
    $releaseTag = if ($Version.StartsWith("v")) { $Version } else { "v$Version" }
}
$releaseVersion = $releaseTag.TrimStart("v")
$archive = "claude-configurator_${releaseVersion}_windows_${architecture}.zip"
$baseUrl = "https://github.com/$repository/releases/download/$releaseTag"
$tempDir = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid())

try {
    New-Item -ItemType Directory -Path $tempDir | Out-Null
    Invoke-WebRequest "$baseUrl/$archive" -OutFile "$tempDir\$archive"
    Invoke-WebRequest "$baseUrl/checksums.txt" -OutFile "$tempDir\checksums.txt"
    $checksumLine = Get-Content "$tempDir\checksums.txt" | Where-Object { $_ -match "\s$([regex]::Escape($archive))$" }
    if (-not $checksumLine) { throw "Checksum for $archive was not found" }
    $expected = ($checksumLine -split "\s+")[0].ToLowerInvariant()
    $actual = (Get-FileHash "$tempDir\$archive" -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $expected) { throw "Checksum verification failed" }

    Expand-Archive "$tempDir\$archive" -DestinationPath "$tempDir\unpacked"
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Copy-Item "$tempDir\unpacked\claude-config.exe" "$InstallDir\claude-config.exe" -Force
    '@echo off
"%~dp0claude-config.exe" %*' | Set-Content "$InstallDir\claude-configurator.cmd" -Encoding Ascii
    '@echo off
"%~dp0claude-config.exe" %*' | Set-Content "$InstallDir\ccfg.cmd" -Encoding Ascii

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if (($userPath -split ";") -notcontains $InstallDir) {
        [Environment]::SetEnvironmentVariable("Path", "$userPath;$InstallDir", "User")
    }
    Write-Host "Installed claude-config $releaseTag to $InstallDir"
    Write-Host "Open a new terminal, then run claude-config."
} finally {
    Remove-Item $tempDir -Recurse -Force -ErrorAction SilentlyContinue
}
