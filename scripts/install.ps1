$ErrorActionPreference = 'Stop'

$processor = $env:PROCESSOR_ARCHITEW6432
if (-not $processor) {
    $processor = $env:PROCESSOR_ARCHITECTURE
}
switch ($processor) {
    'AMD64' { $arch = 'x86_64' }
    'ARM64' { $arch = 'arm64' }
    default { throw 'vl supports x86_64 and arm64 on Windows.' }
}

$tag = '@RELEASE_TAG@'
if ($tag -cnotmatch '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?$') {
    throw 'Download the installer from a vl release.'
}
$version = $tag.Substring(1)
$archive = "vl_${version}_windows_${arch}.zip"
$download = "https://github.com/foursmith/vl/releases/download/$tag"

$installDir = $env:INSTALL_DIR
if (-not $installDir) {
    $installDir = Join-Path $env:LOCALAPPDATA 'vl\bin'
}
$installDir = [IO.Directory]::CreateDirectory($installDir).FullName
$target = Join-Path $installDir 'vl.exe'
if ([IO.Directory]::Exists($target)) {
    throw "$target is a directory."
}
$stage = Join-Path $installDir ('.vl-install-' + [guid]::NewGuid())
[void][IO.Directory]::CreateDirectory($stage)

try {
    $archivePath = Join-Path $stage $archive
    $checksums = Join-Path $stage 'checksums.txt'
    Invoke-WebRequest -UseBasicParsing "$download/$archive" -OutFile $archivePath
    Invoke-WebRequest -UseBasicParsing "$download/checksums.txt" -OutFile $checksums

    $expected = $null
    $pattern = '^([0-9a-fA-F]{64})\s+\*?' + [regex]::Escape($archive) + '$'
    foreach ($line in Get-Content -LiteralPath $checksums) {
        if ($line -cmatch $pattern) {
            $expected = $Matches[1]
            break
        }
    }
    if (-not $expected) {
        throw "No checksum found for $archive."
    }
    if ((Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash -ne $expected) {
        throw "SHA-256 mismatch for $archive."
    }

    $unpacked = Join-Path $stage 'unpacked'
    Expand-Archive -LiteralPath $archivePath -DestinationPath $unpacked
    $binary = Join-Path $unpacked 'vl.exe'
    if (-not [IO.File]::Exists($binary)) {
        throw "The release archive does not contain vl.exe: $archive"
    }
    try {
        $binaryVersion = & $binary --version
        if ($LASTEXITCODE -ne 0 -or ($binaryVersion -join "`n").Trim() -ne "vl $version") {
            throw "Expected 'vl $version', received '$binaryVersion' (exit code $LASTEXITCODE)."
        }
    } catch {
        throw "Cannot run the downloaded vl.exe: $($_.Exception.Message)"
    }
    if ([IO.File]::Exists($target)) {
        [IO.File]::Replace($binary, $target, (Join-Path $stage 'previous-vl.exe'))
    } else {
        [IO.File]::Move($binary, $target)
    }
    if (-not [IO.File]::Exists($target)) {
        throw "Installation failed: vl.exe is missing at $target"
    }
} finally {
    try {
        Remove-Item -LiteralPath $stage -Recurse -Force
    } catch {
        Write-Warning "Could not remove installer temporary directory ${stage}: $($_.Exception.Message)"
    }
}

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (($userPath -split ';') -notcontains $installDir) {
    $userPath = ($userPath, $installDir | Where-Object { $_ }) -join ';'
    [Environment]::SetEnvironmentVariable('Path', $userPath, 'User')
}
if (($env:Path -split ';')[0] -ne $installDir) {
    $env:Path = ($installDir, $env:Path | Where-Object { $_ }) -join ';'
}
$command = Get-Command vl -ErrorAction SilentlyContinue
if (-not $command -or $command.CommandType -ne 'Application' -or $command.Source -ne $target) {
    throw "Installed vl.exe at $target, but PowerShell cannot resolve 'vl' to it. Check PATH, PATHEXT (.EXE), and any alias or function named vl."
}
try {
    $installedVersion = vl --version
    if ($LASTEXITCODE -ne 0 -or ($installedVersion -join "`n").Trim() -ne "vl $version") {
        throw "Expected 'vl $version', received '$installedVersion' (exit code $LASTEXITCODE)."
    }
} catch {
    throw "Installed vl.exe at $target, but 'vl --version' failed: $($_.Exception.Message)"
}
Write-Host "Installed vl $version to $target"
Write-Host "Verified: vl --version -> $installedVersion"
Write-Host 'Run vl in this PowerShell session to get started.'
Write-Host 'For other terminal sessions, fully exit and reopen the terminal application to reload PATH.'
Write-Host ("Or run directly: & '" + $target.Replace("'", "''") + "'")
