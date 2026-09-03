# Download official zapret and zapret2 zips and copy Windows files into bin/.
# Does not touch fake *.bin files. Cache: .local/releases/
# Usage:
#   powershell.exe -NoProfile -ExecutionPolicy Bypass -File dev\update-zapret.ps1
#   ... -ZapretTag v72.13 -Zapret2Tag v1.0.4
#   ... -DryRun

[CmdletBinding()]
param(
    [string]$ZapretTag,
    [string]$Zapret2Tag,
    [switch]$DryRun,
    [switch]$Force
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
$binDir = Join-Path $root 'bin'
$cacheRoot = Join-Path $root '.local\releases'
$manifestPath = Join-Path $binDir 'versions.json'
$ua = 'zapret-dev-update'

[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

function Get-ZapretGitHubJson {
    param([string]$Url)
    $resp = Invoke-WebRequest -Uri $Url -UseBasicParsing -Headers @{
        'User-Agent' = $ua
        'Accept'     = 'application/vnd.github+json'
    }
    return ($resp.Content | ConvertFrom-Json)
}

function Get-ZapretRelease {
    param(
        [string]$Repo,
        [string]$Tag
    )
    if ([string]::IsNullOrWhiteSpace($Tag)) {
        return (Get-ZapretGitHubJson -Url ("https://api.github.com/repos/{0}/releases/latest" -f $Repo))
    }
    return (Get-ZapretGitHubJson -Url ("https://api.github.com/repos/{0}/releases/tags/{1}" -f $Repo, $Tag))
}

function Get-ZapretZipAsset {
    param(
        $Release,
        [string]$NamePrefix
    )
    $hits = New-Object System.Collections.ArrayList
    foreach ($asset in @($Release.assets)) {
        $name = [string]$asset.name
        if ($name -like ($NamePrefix + '-*.zip') -and $name -notlike '*openwrt*') {
            [void]$hits.Add($asset)
        }
    }
    if ($hits.Count -lt 1) {
        throw ("No zip asset for {0} in {1}." -f $NamePrefix, [string]$Release.tag_name)
    }
    return $hits[0]
}

function Get-ZapretCachedZip {
    param(
        [string]$Url,
        [string]$DestPath,
        [long]$ExpectedSize
    )
    $dir = Split-Path -Parent $DestPath
    if (-not (Test-Path -LiteralPath $dir)) {
        [void](New-Item -ItemType Directory -Path $dir -Force)
    }
    if ((Test-Path -LiteralPath $DestPath) -and -not $Force) {
        $len = [long]((Get-Item -LiteralPath $DestPath).Length)
        if ($ExpectedSize -lt 1 -or $len -eq $ExpectedSize) {
            Write-Host ("Cache hit: {0}" -f $DestPath)
            return
        }
    }
    Write-Host ("Download: {0}" -f $Url)
    Invoke-WebRequest -Uri $Url -OutFile $DestPath -UseBasicParsing -Headers @{ 'User-Agent' = $ua }
}

function Find-ZapretZipEntry {
    param(
        [System.IO.Compression.ZipArchive]$Zip,
        [string]$Suffix
    )
    $want = $Suffix.Replace('\', '/')
    foreach ($entry in $Zip.Entries) {
        $name = [string]$entry.FullName
        if ($name.EndsWith('/')) {
            continue
        }
        $norm = $name.Replace('\', '/')
        if ($norm.EndsWith($want, [StringComparison]::OrdinalIgnoreCase)) {
            return $entry
        }
    }
    return $null
}

function Copy-ZapretZipEntry {
    param(
        [string]$ZipPath,
        [string]$Suffix,
        [string]$DestPath
    )
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $zip = [System.IO.Compression.ZipFile]::OpenRead($ZipPath)
    try {
        $entry = Find-ZapretZipEntry -Zip $zip -Suffix $Suffix
        if ($null -eq $entry) {
            throw ("Zip has no entry ending with {0}: {1}" -f $Suffix, $ZipPath)
        }
        $destDir = Split-Path -Parent $DestPath
        if (-not (Test-Path -LiteralPath $destDir)) {
            [void](New-Item -ItemType Directory -Path $destDir -Force)
        }
        $out = [System.IO.File]::Create($DestPath)
        try {
            $src = $entry.Open()
            try {
                $src.CopyTo($out)
            } finally {
                $src.Dispose()
            }
        } finally {
            $out.Dispose()
        }
    } finally {
        $zip.Dispose()
    }
    return (Get-FileHash -LiteralPath $DestPath -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Get-ZapretPinnedManifest {
    if (-not (Test-Path -LiteralPath $manifestPath)) {
        return $null
    }
    try {
        return ([System.IO.File]::ReadAllText($manifestPath) | ConvertFrom-Json)
    } catch {
        return $null
    }
}

function Get-ZapretPinnedTag {
    param(
        $Manifest,
        [string]$Id
    )
    if ($null -eq $Manifest) {
        return ''
    }
    if (-not $Manifest.PSObject.Properties['sources']) {
        return ''
    }
    foreach ($src in @($Manifest.sources)) {
        if ([string]$src.id -eq $Id) {
            return [string]$src.tag
        }
    }
    return ''
}

function Stop-ZapretBinLocks {
    $svc = Get-Service -Name 'zapret' -ErrorAction SilentlyContinue
    if ($svc -and $svc.Status -ne 'Stopped') {
        Write-Host 'Stopping zapret service so bin files can be replaced.'
        & net.exe stop zapret 2>$null | Out-Null
    }
    $procs = @(Get-Process -Name 'winws' -ErrorAction SilentlyContinue)
    $procs2 = @(Get-Process -Name 'winws2' -ErrorAction SilentlyContinue)
    $all = @($procs) + @($procs2)
    if ($all.Count -gt 0) {
        Write-Host 'Stopping winws / winws2.'
        $all | Stop-Process -Force -ErrorAction SilentlyContinue
        Start-Sleep -Milliseconds 500
    }
}

function Write-ZapretBinManifest {
    param(
        [object[]]$Sources,
        [object[]]$Files
    )
    $sb = New-Object System.Text.StringBuilder
    [void]$sb.AppendLine('{')
    [void]$sb.AppendLine(('  "updated": "{0}",' -f (Get-Date -Format 'yyyy-MM-dd')))
    [void]$sb.AppendLine('  "sources": [')
    for ($i = 0; $i -lt $Sources.Count; $i++) {
        $s = $Sources[$i]
        $comma = ','
        if ($i -eq ($Sources.Count - 1)) {
            $comma = ''
        }
        [void]$sb.AppendLine('    {')
        [void]$sb.AppendLine(('      "id": "{0}",' -f $s.id))
        [void]$sb.AppendLine(('      "repo": "{0}",' -f $s.repo))
        [void]$sb.AppendLine(('      "tag": "{0}",' -f $s.tag))
        [void]$sb.AppendLine(('      "published": "{0}",' -f $s.published))
        [void]$sb.AppendLine(('      "url": "{0}"' -f $s.url))
        [void]$sb.AppendLine(('    }{0}' -f $comma))
    }
    [void]$sb.AppendLine('  ],')
    [void]$sb.AppendLine('  "files": [')
    for ($i = 0; $i -lt $Files.Count; $i++) {
        $f = $Files[$i]
        $comma = ','
        if ($i -eq ($Files.Count - 1)) {
            $comma = ''
        }
        [void]$sb.AppendLine('    {')
        [void]$sb.AppendLine(('      "path": "{0}",' -f $f.path))
        [void]$sb.AppendLine(('      "sha256": "{0}",' -f $f.sha256))
        [void]$sb.AppendLine(('      "source": "{0}"' -f $f.source))
        [void]$sb.AppendLine(('    }{0}' -f $comma))
    }
    [void]$sb.AppendLine('  ]')
    [void]$sb.AppendLine('}')
    $utf8 = New-Object System.Text.UTF8Encoding $false
    [System.IO.File]::WriteAllText($manifestPath, $sb.ToString(), $utf8)
}

if (-not (Test-Path -LiteralPath $binDir)) {
    throw 'The bin folder is not found.'
}

$pinned = Get-ZapretPinnedManifest
$rel1 = Get-ZapretRelease -Repo 'bol-van/zapret' -Tag $ZapretTag
$rel2 = Get-ZapretRelease -Repo 'bol-van/zapret2' -Tag $Zapret2Tag
$tag1 = [string]$rel1.tag_name
$tag2 = [string]$rel2.tag_name
$old1 = Get-ZapretPinnedTag -Manifest $pinned -Id 'zapret'
$old2 = Get-ZapretPinnedTag -Manifest $pinned -Id 'zapret2'

Write-Host ("zapret:  {0} -> {1}  {2}" -f $old1, $tag1, [string]$rel1.html_url)
Write-Host ("zapret2: {0} -> {1}  {2}" -f $old2, $tag2, [string]$rel2.html_url)

if ($DryRun) {
    Write-Host 'Dry run. No files copied.'
    exit 0
}

$sameTags = ($old1 -eq $tag1 -and $old2 -eq $tag2)
if ($sameTags -and -not $Force) {
    Write-Host 'Pinned tags already match. Use -Force to extract again and rewrite versions.json.'
    exit 0
}

$asset1 = Get-ZapretZipAsset -Release $rel1 -NamePrefix 'zapret'
$asset2 = Get-ZapretZipAsset -Release $rel2 -NamePrefix 'zapret2'
$zip1 = Join-Path $cacheRoot ("zapret-{0}\{1}" -f $tag1, [string]$asset1.name)
$zip2 = Join-Path $cacheRoot ("zapret2-{0}\{1}" -f $tag2, [string]$asset2.name)

Get-ZapretCachedZip -Url ([string]$asset1.browser_download_url) -DestPath $zip1 -ExpectedSize ([long]$asset1.size)
Get-ZapretCachedZip -Url ([string]$asset2.browser_download_url) -DestPath $zip2 -ExpectedSize ([long]$asset2.size)

Stop-ZapretBinLocks

$map = @(
    @{ Zip = $zip1; Suffix = '/binaries/windows-x86_64/winws.exe'; Dest = 'winws.exe'; Source = 'zapret' }
    @{ Zip = $zip1; Suffix = '/binaries/windows-x86_64/cygwin1.dll'; Dest = 'cygwin1.dll'; Source = 'zapret' }
    @{ Zip = $zip1; Suffix = '/binaries/windows-x86_64/WinDivert.dll'; Dest = 'WinDivert.dll'; Source = 'zapret' }
    @{ Zip = $zip1; Suffix = '/binaries/windows-x86_64/WinDivert64.sys'; Dest = 'WinDivert64.sys'; Source = 'zapret' }
    @{ Zip = $zip2; Suffix = '/binaries/windows-x86_64/winws2.exe'; Dest = 'winws2.exe'; Source = 'zapret2' }
    @{ Zip = $zip2; Suffix = '/lua/zapret-lib.lua'; Dest = 'lua/zapret-lib.lua'; Source = 'zapret2' }
    @{ Zip = $zip2; Suffix = '/lua/zapret-antidpi.lua'; Dest = 'lua/zapret-antidpi.lua'; Source = 'zapret2' }
)

$files = New-Object System.Collections.ArrayList
foreach ($item in $map) {
    $dest = Join-Path $binDir ($item.Dest -replace '/', '\')
    $hash = Copy-ZapretZipEntry -ZipPath $item.Zip -Suffix $item.Suffix -DestPath $dest
    Write-Host ("OK {0}  {1}" -f $item.Dest, $hash)
    [void]$files.Add((New-Object PSObject -Property @{
        path   = $item.Dest
        sha256 = $hash
        source = $item.Source
    }))
}

$sources = @(
    (New-Object PSObject -Property @{
        id        = 'zapret'
        repo      = 'https://github.com/bol-van/zapret'
        tag       = $tag1
        published = [string]$rel1.published_at
        url       = [string]$rel1.html_url
    })
    (New-Object PSObject -Property @{
        id        = 'zapret2'
        repo      = 'https://github.com/bol-van/zapret2'
        tag       = $tag2
        published = [string]$rel2.published_at
        url       = [string]$rel2.html_url
    })
)
Write-ZapretBinManifest -Sources $sources -Files @($files)
Write-Host ("Wrote {0}" -f $manifestPath)
Write-Host 'Fake *.bin files were not changed. Update README / PLAN / AGENTS if the tags changed.'
exit 0
