$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

$repository = "shurrman/vault-manager"
$tag = $env:VAULT_MANAGER_VERSION
if ([string]::IsNullOrWhiteSpace($tag)) {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$repository/releases/latest"
    $tag = $release.tag_name
}
if (-not $tag.StartsWith("v")) {
    $tag = "v$tag"
}
if ($tag -notmatch '^v[0-9]+\.[0-9]+$') {
    throw "vault-manager installer: invalid release version: $tag"
}
$version = $tag.Substring(1)

$processorArchitecture = $env:PROCESSOR_ARCHITEW6432
if ([string]::IsNullOrWhiteSpace($processorArchitecture)) {
    $processorArchitecture = $env:PROCESSOR_ARCHITECTURE
}
if ($processorArchitecture -ne "AMD64") {
    throw "vault-manager installer: unsupported Windows architecture: $processorArchitecture"
}

$asset = "vault-manager_${version}_windows_amd64.zip"
$baseUrl = $env:VAULT_MANAGER_DOWNLOAD_BASE_URL
if ([string]::IsNullOrWhiteSpace($baseUrl)) {
    $baseUrl = "https://github.com/$repository/releases/download/$tag"
}
$installDir = $env:VAULT_MANAGER_INSTALL_DIR
if ([string]::IsNullOrWhiteSpace($installDir)) {
    if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) {
        throw "vault-manager installer: LOCALAPPDATA is required"
    }
    $installDir = Join-Path $env:LOCALAPPDATA "Programs\vault-manager"
}

$tempDir = Join-Path ([IO.Path]::GetTempPath()) ("vault-manager-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tempDir | Out-Null
try {
    $archive = Join-Path $tempDir $asset
    $checksums = Join-Path $tempDir "checksums.txt"
    Invoke-WebRequest -Uri "$baseUrl/$asset" -OutFile $archive -UseBasicParsing
    Invoke-WebRequest -Uri "$baseUrl/checksums.txt" -OutFile $checksums -UseBasicParsing

    $assetPattern = [regex]::Escape($asset)
    $checksumLine = Get-Content -LiteralPath $checksums | Where-Object {
        $_ -match ("^[0-9A-Fa-f]{64}\s+\*?" + $assetPattern + '$')
    } | Select-Object -First 1
    if ([string]::IsNullOrWhiteSpace($checksumLine)) {
        throw "vault-manager installer: $asset is missing from checksums.txt"
    }
    $expected = ($checksumLine -split '\s+')[0].ToLowerInvariant()
    $actual = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $expected) {
        throw "vault-manager installer: SHA-256 verification failed for $asset"
    }

    Expand-Archive -LiteralPath $archive -DestinationPath $tempDir -Force
    $source = Join-Path $tempDir "vault-manager.exe"
    if (-not (Test-Path -LiteralPath $source -PathType Leaf)) {
        throw "vault-manager installer: archive does not contain vault-manager.exe"
    }
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    $destination = Join-Path $installDir "vault-manager.exe"
    Copy-Item -LiteralPath $source -Destination $destination -Force

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $inUserPath = $false
    foreach ($entry in ($userPath -split ';')) {
        if ($entry.TrimEnd('\') -ieq $installDir.TrimEnd('\')) {
            $inUserPath = $true
            break
        }
    }
    if (-not $inUserPath) {
        $newUserPath = if ([string]::IsNullOrWhiteSpace($userPath)) { $installDir } else { "$userPath;$installDir" }
        [Environment]::SetEnvironmentVariable("Path", $newUserPath, "User")
    }
    if (-not (($env:Path -split ';') | Where-Object { $_.TrimEnd('\') -ieq $installDir.TrimEnd('\') })) {
        $env:Path = "$installDir;$env:Path"
    }

    Write-Host "Installed vault-manager $version to $destination"
}
finally {
    Remove-Item -LiteralPath $tempDir -Recurse -Force -ErrorAction SilentlyContinue
}
