param(
    [string]$Version = "latest",
    [string]$InstallDir = "$([Environment]::GetFolderPath('LocalApplicationData'))\Programs\claude-configurator\bin"
)

$ErrorActionPreference = "Stop"
$repository = "ex3lite/claude-configurator"

function Test-NerdFont {
    $fontDirectory = Join-Path $env:LOCALAPPDATA "Microsoft\Windows\Fonts"
    if (Test-Path $fontDirectory) {
        if (Get-ChildItem $fontDirectory -File -ErrorAction SilentlyContinue |
            Where-Object { $_.Name -match "NerdFont|Nerd Font" } |
            Select-Object -First 1) {
            return $true
        }
    }
    $fontKey = "HKCU:\Software\Microsoft\Windows NT\CurrentVersion\Fonts"
    if (Test-Path $fontKey) {
        $names = (Get-Item $fontKey).Property
        if ($names | Where-Object { $_ -match "Nerd Font|NerdFont| NFM($| )" } | Select-Object -First 1) {
            return $true
        }
    }
    return $false
}

function Install-NerdFont([string]$WorkingDirectory) {
    $fontRelease = Invoke-RestMethod "https://api.github.com/repos/ryanoasis/nerd-fonts/releases/latest"
    $fontBaseUrl = "https://github.com/ryanoasis/nerd-fonts/releases/download/$($fontRelease.tag_name)"
    $fontArchive = Join-Path $WorkingDirectory "Meslo.zip"
    $fontChecksums = Join-Path $WorkingDirectory "nerd-font-checksums.txt"
    Invoke-WebRequest "$fontBaseUrl/Meslo.zip" -OutFile $fontArchive
    Invoke-WebRequest "$fontBaseUrl/SHA-256.txt" -OutFile $fontChecksums
    $fontLine = Get-Content $fontChecksums | Where-Object { $_ -match "\sMeslo\.zip$" }
    if (-not $fontLine) { throw "Checksum for Meslo.zip was not found" }
    $fontExpected = ($fontLine -split "\s+")[0].ToLowerInvariant()
    $fontActual = (Get-FileHash $fontArchive -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($fontActual -ne $fontExpected) { throw "MesloLGS Nerd Font checksum verification failed" }

    $fontUnpack = Join-Path $WorkingDirectory "meslo"
    Expand-Archive $fontArchive -DestinationPath $fontUnpack
    $fontDirectory = Join-Path $env:LOCALAPPDATA "Microsoft\Windows\Fonts"
    New-Item -ItemType Directory -Force -Path $fontDirectory | Out-Null
    $fontKey = "HKCU:\Software\Microsoft\Windows NT\CurrentVersion\Fonts"
    New-Item -Path $fontKey -Force | Out-Null
    $fonts = Get-ChildItem $fontUnpack -Filter "MesloLGSNerdFontMono-*.ttf"
    if (-not $fonts) { throw "MesloLGS Nerd Font files were not found in the verified archive" }
    foreach ($font in $fonts) {
        $destination = Join-Path $fontDirectory $font.Name
        Copy-Item $font.FullName $destination -Force
        New-ItemProperty -Path $fontKey -Name "$($font.BaseName) (TrueType)" `
            -Value $destination -PropertyType String -Force | Out-Null
    }
    Write-Host "Installed MesloLGS Nerd Font Mono for the current user."
    Write-Host "Restart the terminal and select 'MesloLGS Nerd Font Mono' as its font."
}

function Offer-NerdFont([string]$WorkingDirectory) {
    if (Test-NerdFont) {
        Write-Host "Nerd Font detected. Claude Icons will be unlocked in the status-bar themes."
        return
    }
    $fontChoice = if ($env:CLAUDE_CONFIG_INSTALL_NERD_FONT) {
        $env:CLAUDE_CONFIG_INSTALL_NERD_FONT
    } else {
        "ask"
    }
    if ($fontChoice -eq "ask" -and $Host.Name -eq "ConsoleHost" -and -not $env:CI) {
        $answer = Read-Host "Install the recommended MesloLGS Nerd Font for the Claude Icons theme? [y/N]"
        $fontChoice = if ($answer -match "^(y|yes)$") { "1" } else { "0" }
    }
    if ($fontChoice -match "^(1|true|yes)$") {
        Install-NerdFont $WorkingDirectory
    } else {
        Write-Host "Nerd Font not installed. Set CLAUDE_CONFIG_INSTALL_NERD_FONT=1 to install it non-interactively."
    }
}

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
    $unpackedBinary = "$tempDir\unpacked\claude-config.exe"
    $unpackedVersion = (& $unpackedBinary --version).Trim()
    if ($unpackedVersion -ne $releaseVersion) {
        throw "Downloaded binary reports $unpackedVersion; expected $releaseVersion"
    }

    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Copy-Item $unpackedBinary "$InstallDir\claude-config.exe" -Force
    '@echo off
"%~dp0claude-config.exe" %*' | Set-Content "$InstallDir\claude-configurator.cmd" -Encoding Ascii
    '@echo off
"%~dp0claude-config.exe" %*' | Set-Content "$InstallDir\ccfg.cmd" -Encoding Ascii

    $installedBinary = "$InstallDir\claude-config.exe"
    $installedVersion = (& $installedBinary --version).Trim()
    if ($installedVersion -ne $releaseVersion) {
        throw "Installed binary verification failed: expected $releaseVersion, got $installedVersion"
    }

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $currentPathContainsInstallDir = ($env:Path -split ";") -contains $InstallDir
    $userPathContainsInstallDir = ($userPath -split ";") -contains $InstallDir
    Write-Host "Installed and verified claude-config $releaseTag in $InstallDir"
    Offer-NerdFont $tempDir
    if (-not $currentPathContainsInstallDir) {
        if ($userPathContainsInstallDir) {
            Write-Host "Open a new terminal, then run claude-config."
        } else {
            $escapedInstallDir = $InstallDir.Replace("'", "''")
            Write-Host ""
            Write-Host "The install directory is not on PATH in this PowerShell session."
            Write-Host "Run this exact command now:"
            Write-Host "  `$env:Path = '$escapedInstallDir;' + `$env:Path"
            Write-Host "Use Windows Environment Variables if you want that PATH entry to persist."
        }
    }
} finally {
    Remove-Item $tempDir -Recurse -Force -ErrorAction SilentlyContinue
}
