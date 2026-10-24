param([string]$RepoRoot = (Split-Path -Parent $PSScriptRoot))
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
Import-Module -Force -DisableNameChecking (Join-Path $RepoRoot 'src\ZapretSpec\ZapretSpec.psd1')

function Assert-SkyTest {
    param([bool]$Condition, [string]$Message)
    if (-not $Condition) { throw $Message }
}

$path = Join-Path $RepoRoot 'strategies\general (ALT11 Photon).json'
$spec = Get-ZapretStrategySpec -Path $path
$common = @{
    Bin = (Join-Path $RepoRoot 'bin')
    Lists = (Join-Path $RepoRoot 'lists')
    User = (Join-Path $RepoRoot 'user')
    GameFilterTcp = '12'
    GameFilterUdp = '12'
}
$z2 = ConvertTo-ZapretStrategyArgList -Spec $spec -Engine winws2 @common
$profiles = @($z2 -split '\r?\n' | Where-Object { $_ -match '^--filter-' })
Assert-SkyTest ($z2 -match '--wf-udp-out=[^\r\n]*5000-8000') 'Sky ports must be captured with Game Filter disabled.'
Assert-SkyTest ($profiles[0] -match '--ipset="[^"\r\n]*user\\ipset-sky.txt"') 'Sky UDP must use its own user list.'
Assert-SkyTest ($profiles[0] -match '--out-range=-d4') 'Sky UDP must stop fake injection after four packets.'
Assert-SkyTest ($profiles[0] -match '--lua-desync=fake:payload=all:blob=quic_initial_www_google_com:repeats=6:ip_ttl=8') 'Sky UDP must match the verified fake command.'
Assert-SkyTest (@([regex]::Matches($profiles[0], '--lua-desync=fake:')).Count -eq 1) 'Sky UDP must not acquire an extra default TLS fake.'
Assert-SkyTest ($profiles[1] -match '--hostlist-domains=live.ws.radiance.thatgamecompany.com') 'Sky TCP must only target the real WebSocket endpoint.'
Assert-SkyTest ($profiles[1] -match '--lua-desync=multisplit:payload=all:pos=1') 'WebSocket must process encrypted application payloads.'
Assert-SkyTest ($profiles[1] -notmatch '--out-range=') 'WebSocket processing must continue past the handshake.'
$photon = @($profiles | Where-Object { $_ -match '--filter-udp=5055-5058,27000-27002' })
Assert-SkyTest ($photon.Count -eq 2) 'Sker and common Photon profiles must both exist.'
Assert-SkyTest ($photon[0] -match '--ipset="[^"\r\n]*user\\ipset-sker.txt"') 'Sker profile must come before the common Photon profile.'
Assert-SkyTest ($photon[0] -match '--lua-desync=fake:payload=all:blob=ACTIVE_GAME_UDP:repeats=6:ip_ttl=8') 'Sker fakes must not reach the Photon server.'
Assert-SkyTest (@([regex]::Matches($photon[0], '--lua-desync=fake:')).Count -eq 1) 'Sker profile must not acquire an extra default TLS fake.'
$z1 = ConvertTo-ZapretStrategyArgList -Spec $spec -Engine winws @common
Assert-SkyTest ($z1 -match '--dpi-desync-ttl=8') 'winws must receive the Sky fake TTL.'
Assert-SkyTest ($z1 -match '--dpi-desync-any-protocol=1') 'winws must process unknown Sky payloads.'
Assert-SkyTest ($z1 -notmatch '--dpi-desync-cutoff=none|--lua-desync') 'winws must not receive Lua flags or the none sentinel.'
foreach ($badTtl in @(0, 256, '8:repeats=100', '1.5')) {
    $invalid = Get-ZapretStrategySpec -Path $path
    $invalid.profiles[0].ttl = $badTtl
    foreach ($engine in @('winws', 'winws2')) {
        $threw = $false
        try { $null = ConvertTo-ZapretStrategyArgList -Spec $invalid -Engine $engine @common } catch { $threw = $true }
        Assert-SkyTest $threw ('Invalid TTL was accepted by ' + $engine)
    }
}
$invalid = Get-ZapretStrategySpec -Path $path
$invalid.profiles[0].luaPayload = 'all:repeats=100'
$threw = $false
try { $null = ConvertTo-ZapretStrategyArgList -Spec $invalid -Engine winws2 @common } catch { $threw = $true }
Assert-SkyTest $threw 'luaPayload must reject Lua argument injection.'

Import-Module -Force -DisableNameChecking (Join-Path $RepoRoot 'src\Zapman\Zapman.psd1')
Assert-SkyTest ([bool](Get-Command -Name Update-ZapmanSkyIpList -Module Zapman -ErrorAction SilentlyContinue)) 'Sky updater must be exported through the module manifest.'
$temporaryDirectory = Join-Path ([IO.Path]::GetTempPath()) ('zapman-sky-test-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $temporaryDirectory | Out-Null
try {
    $logPath = Join-Path $temporaryDirectory 'Sky.log'
    $ipList = Join-Path $temporaryDirectory 'ipset-sky.txt'
    '203.0.113.1/32' | Set-Content -LiteralPath $ipList -Encoding ASCII
    @(
        'Connecting to server: [203.0.113.2:6000]'
        'Connected to game server: [203.0.113.2:6000] event data: 0'
        'Connecting to server: [192.168.1.5:6000]'
        'Connecting to server: [999.1.1.1:6000]'
        'Connecting to server: [203.0.113.3:443]'
        'Account data: must not enter the IP list'
    ) | Set-Content -LiteralPath $logPath -Encoding ASCII
    $handle = [IO.File]::Open($logPath, [IO.FileMode]::Open, [IO.FileAccess]::ReadWrite, [IO.FileShare]::ReadWrite)
    try { Update-ZapmanSkyIpList -LogPath $logPath -IpListPath $ipList } finally { $handle.Dispose() }
    $addresses = @(Get-Content -LiteralPath $ipList)
    Assert-SkyTest ($addresses.Count -eq 2) 'Only a valid public game IP must be added, once.'
    Assert-SkyTest ($addresses -contains '203.0.113.1/32' -and $addresses -contains '203.0.113.2/32') 'Existing IPs must be preserved.'
    $stamp = (Get-Item -LiteralPath $ipList).LastWriteTimeUtc
    Update-ZapmanSkyIpList -LogPath $logPath -IpListPath $ipList
    Assert-SkyTest ((Get-Item -LiteralPath $ipList).LastWriteTimeUtc -eq $stamp) 'An unchanged log must not rewrite the list.'
} finally {
    foreach ($file in @(Get-ChildItem -LiteralPath $temporaryDirectory -File)) { Remove-Item -LiteralPath $file.FullName }
    Remove-Item -LiteralPath $temporaryDirectory
}
Write-Output 'PASS: Sky argv on both engines, TTL validation, Lua payload validation, and IP updates with a live log.'
