# Add only game server IPv4 addresses observed in the Sky log.
$script:ZapmanSkyLogStamp = ''

function Update-ZapmanSkyIpList {
    param([string]$LogPath, [string]$IpListPath)
    if ([string]::IsNullOrWhiteSpace($IpListPath)) {
        $IpListPath = Join-Path $script:ZapmanUserDir 'ipset-sky.txt'
    }
    if ([string]::IsNullOrWhiteSpace($LogPath)) {
        $games = @(Get-Process -Name Sky -ErrorAction SilentlyContinue)
        if ($games.Count -eq 0) { return }
        $gamePath = $games[0].Path
        if ([string]::IsNullOrWhiteSpace($gamePath)) { return }
        $LogPath = Join-Path (Split-Path -Parent $gamePath) 'Sky.log'
    }
    if (-not (Test-Path -LiteralPath $LogPath)) { return }
    if (-not (Test-Path -LiteralPath $IpListPath)) { Initialize-ZapmanUserLists }
    $item = Get-Item -LiteralPath $LogPath
    $stamp = $LogPath + '|' + $item.LastWriteTimeUtc.Ticks + '|' + $item.Length
    if ($stamp -eq $script:ZapmanSkyLogStamp) { return }
    $old = @(Get-Content -LiteralPath $IpListPath | Where-Object { $_ -match '^\d+\.\d+\.\d+\.\d+/32$' })
    $addresses = New-Object System.Collections.ArrayList
    $text = Get-Content -LiteralPath $LogPath -Raw
    $serverMatches = [regex]::Matches($text, '(?:Connecting to server|Connected to game server): \[(\d+\.\d+\.\d+\.\d+):(\d+)\]')
    foreach ($match in $serverMatches) {
        $port = [int]$match.Groups[2].Value
        if ($port -lt 5000 -or $port -gt 8000) { continue }
        $address = $null
        if (-not [Net.IPAddress]::TryParse($match.Groups[1].Value, [ref]$address)) { continue }
        if ($address.AddressFamily -ne [Net.Sockets.AddressFamily]::InterNetwork) { continue }
        $bytes = $address.GetAddressBytes()
        if ($bytes[0] -eq 0 -or $bytes[0] -eq 10 -or $bytes[0] -eq 127 -or $bytes[0] -ge 224) { continue }
        if ($bytes[0] -eq 169 -and $bytes[1] -eq 254) { continue }
        if ($bytes[0] -eq 172 -and $bytes[1] -ge 16 -and $bytes[1] -le 31) { continue }
        if ($bytes[0] -eq 192 -and $bytes[1] -eq 168) { continue }
        [void]$addresses.Add($address.ToString() + '/32')
    }
    $combined = @(@($old) + @($addresses) | Sort-Object -Unique)
    if ($combined.Count -gt 0 -and @(Compare-Object -ReferenceObject $old -DifferenceObject $combined).Count -gt 0) {
        $temporaryPath = $IpListPath + '.tmp'
        try {
            $combined | Set-Content -LiteralPath $temporaryPath -Encoding ASCII
            Move-Item -LiteralPath $temporaryPath -Destination $IpListPath -Force
        } finally {
            if (Test-Path -LiteralPath $temporaryPath) { Remove-Item -LiteralPath $temporaryPath }
        }
    }
    $script:ZapmanSkyLogStamp = $stamp
}
