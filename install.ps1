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

$release = Invoke-RestMethod -UseBasicParsing 'https://api.github.com/repos/foursmith/vl/releases/latest'
$tag = $release.tag_name
if ($tag -cnotmatch '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?$') {
    throw 'Could not find a published vl release.'
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
    if ([IO.File]::Exists($target)) {
        [IO.File]::Replace($binary, $target, (Join-Path $stage 'previous-vl.exe'))
    } else {
        [IO.File]::Move($binary, $target)
    }
} finally {
    Remove-Item -LiteralPath $stage -Recurse -Force
}

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (($userPath -split ';') -notcontains $installDir) {
    $userPath = ($userPath, $installDir | Where-Object { $_ }) -join ';'
    [Environment]::SetEnvironmentVariable('Path', $userPath, 'User')
}
if (($env:Path -split ';') -notcontains $installDir) {
    $env:Path = ($env:Path, $installDir | Where-Object { $_ }) -join ';'
}
Write-Host "Installed vl $version to $target"
