# Zapret engine helpers: winws start/stop, Windows service zapret, Game Filter, IPSet.
# Zapman.psm1 dotsources this file. Script paths come from src/Zapman/Core.ps1.

Set-StrictMode -Version Latest

$script:ZapretArgvCache = @{}
$script:ZapmanEngineProcess = $null
$script:ZapmanEngineErrTask = $null
$script:ZapmanEngineOutTask = $null
# Strategy name under HKLM\...\Services\zapret. Do not read the old value name.
$script:ZapretServiceStrategyValueName = 'zapman'
$script:ZapretServiceStrategyValueNameLegacy = 'zapret-discord-youtube'

function Invoke-ZapretOnWait {
    param([scriptblock]$OnWait)
    if ($null -ne $OnWait) {
        & $OnWait
    }
}

function Get-ZapretGameFilter {
    $modeName = 'disabled'
    $cfg = Get-ZapmanConfig
    if ($cfg.gameFilter) {
        $modeName = [string]$cfg.gameFilter
    }
    $tcp = '12'
    $udp = '12'
    $status = 'disabled'
    if ($modeName -eq 'all') {
        $tcp = '1024-65535'
        $udp = '1024-65535'
        $status = 'enabled (TCP and UDP)'
    } elseif ($modeName -eq 'tcp') {
        $tcp = '1024-65535'
        $udp = '12'
        $status = 'enabled (TCP)'
    } elseif ($modeName -eq 'udp') {
        $tcp = '12'
        $udp = '1024-65535'
        $status = 'enabled (UDP)'
    } else {
        $modeName = 'disabled'
    }
    return New-Object PSObject -Property @{
        Tcp    = $tcp
        Udp    = $udp
        Status = $status
        Mode   = $modeName
    }
}

function Set-ZapretGameFilterMode {
    param([string]$Mode)
    $value = 'disabled'
    if ($Mode -eq 'all' -or $Mode -eq 'tcp' -or $Mode -eq 'udp') {
        $value = $Mode
    }
    [void](Update-ZapmanConfig -GameFilter $value)
}

function Test-ZapretServiceRunning {
    $svc = Get-Service -Name 'zapret' -ErrorAction SilentlyContinue
    return [bool]($svc -and $svc.Status -eq 'Running')
}

function Test-ZapretNamedService {
    param([string]$Name)
    $svc = Get-Service -Name $Name -ErrorAction SilentlyContinue
    return [bool]$svc
}

function Get-ZapretService {
    return Get-Service -Name 'zapret' -ErrorAction SilentlyContinue
}

function Invoke-ZapretStrategyPrep {
    if (Test-ZapretServiceRunning) {
        Write-Host 'The zapret service is already running. Remove the service first if you want to run a strategy.' -ForegroundColor Yellow
        throw 'The zapret service is already running.'
    }
    Enable-ZapmanTcpTimestamps
    Initialize-ZapmanUserLists
}

function Get-ZapretStrategyFiles {
    if (-not (Test-Path -LiteralPath $script:ZapmanStrategiesDir)) {
        return @()
    }
    return @(
        Get-ChildItem -LiteralPath $script:ZapmanStrategiesDir -Filter '*.json' |
            Sort-Object { [Regex]::Replace($_.Name, '(\d+)', { $args[0].Value.PadLeft(8, '0') }) }
    )
}

function Test-ZapretStrategyFileName {
    param([string]$Name)
    if ([string]::IsNullOrWhiteSpace($Name)) {
        return $false
    }
    foreach ($file in @(Get-ZapretStrategyFiles)) {
        if ($file.BaseName -eq $Name) {
            return $true
        }
    }
    return $false
}

function ConvertTo-ZapretWinwsArgumentString {
    param([string]$ArgumentList)
    return @(
        $ArgumentList -split '\r?\n' |
            ForEach-Object { $_.Trim() } |
            Where-Object { $_ }
    ) -join ' '
}

function Get-ZapretNormalizedArgumentString {
    param([string]$Text)
    $t = [string]$Text
    $t = $t -replace '"', ''
    $t = $t -replace '/', '\'
    $t = [regex]::Replace($t.ToLowerInvariant(), '\s+', ' ').Trim()
    return $t
}

function Get-ZapretWinwsArgumentString {
    $image = Get-ZapretWinwsCommandLine
    if ([string]::IsNullOrWhiteSpace($image)) {
        return ''
    }
    if ($image -match '^"[^"]+"\s+(.*)$') {
        return $matches[1]
    }
    return ''
}

function Get-ZapretEngineExeName {
    param([string]$Engine)
    if ([string]::IsNullOrWhiteSpace($Engine)) {
        $Engine = Get-ZapretEngine
    }
    if ($Engine -eq 'winws2') {
        return 'winws2.exe'
    }
    return 'winws.exe'
}

function Get-ZapretEngineProcessName {
    param([string]$Engine)
    if ([string]::IsNullOrWhiteSpace($Engine)) {
        $Engine = Get-ZapretEngine
    }
    if ($Engine -eq 'winws2') {
        return 'winws2'
    }
    return 'winws'
}

function Get-ZapretRunningBypassName {
    $n1 = (@(Get-Process -Name 'winws' -ErrorAction SilentlyContinue)).Count
    $n2 = (@(Get-Process -Name 'winws2' -ErrorAction SilentlyContinue)).Count
    if ($n1 -gt 0 -and $n2 -gt 0) {
        return 'winws + winws2'
    }
    if ($n2 -gt 0) {
        return 'winws2'
    }
    if ($n1 -gt 0) {
        return 'winws'
    }
    return ''
}

function Get-ZapretServiceEngineName {
    $path = 'HKLM:\System\CurrentControlSet\Services\zapret'
    try {
        $item = Get-ItemProperty -LiteralPath $path -Name 'ImagePath' -ErrorAction Stop
        $image = [string]$item.ImagePath
    } catch {
        return ''
    }
    if ($image -match '(?i)winws2\.exe') {
        return 'winws2'
    }
    if ($image -match '(?i)winws\.exe') {
        return 'winws'
    }
    return ''
}

function Get-ZapretStatusBypassName {
    $live = Get-ZapretRunningBypassName
    if (-not [string]::IsNullOrWhiteSpace($live)) {
        return $live
    }
    $fromSvc = Get-ZapretServiceEngineName
    if (-not [string]::IsNullOrWhiteSpace($fromSvc)) {
        return $fromSvc
    }
    return (Get-ZapretEngineProcessName)
}

function Test-ZapretEngineFiles {
    param([string]$Engine)
    if ([string]::IsNullOrWhiteSpace($Engine)) {
        $Engine = Get-ZapretEngine
    }
    $exe = Join-Path $script:ZapmanBinDir (Get-ZapretEngineExeName -Engine $Engine)
    if (-not (Test-Path -LiteralPath $exe)) {
        return $false
    }
    if ($Engine -eq 'winws2') {
        $lib = Join-Path $script:ZapmanBinDir 'lua\zapret-lib.lua'
        $anti = Join-Path $script:ZapmanBinDir 'lua\zapret-antidpi.lua'
        if (-not (Test-Path -LiteralPath $lib)) {
            return $false
        }
        if (-not (Test-Path -LiteralPath $anti)) {
            return $false
        }
    }
    return $true
}

function Get-ZapretStrategyArgTemplate {
    param(
        [string]$Path,
        [string]$Engine
    )
    if (-not (Test-Path -LiteralPath $Path)) {
        return ''
    }
    if ([string]::IsNullOrWhiteSpace($Engine)) {
        $Engine = Get-ZapretEngine
    }
    $item = Get-Item -LiteralPath $Path
    $gf = Get-ZapretGameFilter
    $key = '{0}|{1}|{2}|{3}|{4}' -f $Path.ToLowerInvariant(), $Engine, [string]$gf.Tcp, [string]$gf.Udp, $item.LastWriteTimeUtc.Ticks
    if ($script:ZapretArgvCache.ContainsKey($key)) {
        return [string]$script:ZapretArgvCache[$key]
    }
    $layout = Get-ZapretLayout
    $spec = Get-ZapretStrategySpec -Path $Path
    $argv = ConvertTo-ZapretStrategyArgList -Spec $spec -Engine $Engine -Bin $layout.Bin -Lists $layout.Lists -User $layout.User -GameFilterTcp $gf.Tcp -GameFilterUdp $gf.Udp
    $script:ZapretArgvCache[$key] = $argv
    return $argv
}

function Test-ZapretStrategySupportsEngine {
    param(
        [string]$Path,
        [string]$Engine
    )
    if ([string]::IsNullOrWhiteSpace($Engine)) {
        $Engine = Get-ZapretEngine
    }
    if ($Engine -ne 'winws' -and $Engine -ne 'winws2') {
        return $false
    }
    # List UI only needs a valid spec. Do not generate argv here.
    try {
        [void](Get-ZapretStrategySpec -Path $Path)
    } catch {
        return $false
    }
    return $true
}

function Complete-ZapretEngineArgumentList {
    param(
        [string]$ArgumentList,
        [string]$Engine
    )
    $flat = ConvertTo-ZapretWinwsArgumentString -ArgumentList $ArgumentList
    if ([string]::IsNullOrWhiteSpace($Engine)) {
        $Engine = Get-ZapretEngine
    }
    # A Windows service starts in System32. winws2 loads @lua from the current directory.
    if ($Engine -eq 'winws2' -and $flat -notmatch '(?i)(^|\s)--chdir(=|\s|$)') {
        $flat = '--chdir="' + $script:ZapmanBinDir + '" ' + $flat
    }
    return $flat
}

function Get-ZapretStrategyServiceImagePath {
    param(
        [string]$Path,
        [string]$Engine
    )
    if ([string]::IsNullOrWhiteSpace($Engine)) {
        $Engine = Get-ZapretEngine
    }
    $tmpl = Get-ZapretStrategyArgTemplate -Path $Path -Engine $Engine
    if ([string]::IsNullOrWhiteSpace($tmpl)) {
        return ''
    }
    $exe = Join-Path $script:ZapmanBinDir (Get-ZapretEngineExeName -Engine $Engine)
    $flat = Complete-ZapretEngineArgumentList -ArgumentList $tmpl -Engine $Engine
    return '"' + $exe + '" ' + $flat
}

function Get-ZapretStrategyNameFromWinws {
    $live = Get-ZapretNormalizedArgumentString -Text (Get-ZapretWinwsArgumentString)
    if ([string]::IsNullOrWhiteSpace($live)) {
        return ''
    }
    $engines = New-Object System.Collections.ArrayList
    $running = Get-ZapretRunningBypassName
    if ($running -eq 'winws2') {
        [void]$engines.Add('winws2')
    } elseif ($running -eq 'winws') {
        [void]$engines.Add('winws')
    } else {
        [void]$engines.Add('winws')
        [void]$engines.Add('winws2')
    }
    $hits = New-Object System.Collections.ArrayList
    foreach ($file in @(Get-ZapretStrategyFiles)) {
        foreach ($eng in @($engines)) {
            try {
                $tmpl = Get-ZapretStrategyArgTemplate -Path $file.FullName -Engine $eng
            } catch {
                continue
            }
            if ([string]::IsNullOrWhiteSpace($tmpl)) {
                continue
            }
            $flat = Complete-ZapretEngineArgumentList -ArgumentList $tmpl -Engine $eng
            $norm = Get-ZapretNormalizedArgumentString -Text $flat
            if ($norm -and $norm -eq $live) {
                [void]$hits.Add($file.BaseName)
            }
        }
    }
    if ($hits.Count -eq 1) {
        return [string]$hits[0]
    }
    return ''
}

function Get-ZapretRunningStrategyName {
    if (-not (Test-ZapretBypassRunning)) {
        return ''
    }
    $svc = Get-ZapretService
    $installed = Get-ZapretInstalledStrategyName
    $svcRunning = $false
    if ($svc -and $svc.Status -eq 'Running') {
        $svcRunning = $true
    }
    if ($svcRunning -and -not [string]::IsNullOrWhiteSpace($installed)) {
        return $installed
    }
    return (Get-ZapretStrategyNameFromWinws)
}

function Close-ZapretEngineCapture {
    $script:ZapmanEngineErrTask = $null
    $script:ZapmanEngineOutTask = $null
    $proc = $script:ZapmanEngineProcess
    $script:ZapmanEngineProcess = $null
    if ($null -ne $proc) {
        try {
            $proc.Dispose()
        } catch {
            [void]$_
        }
    }
}

function Get-ZapretTaskText {
    param(
        $Task,
        [int]$TimeoutMs = 2000
    )
    if ($null -eq $Task) {
        return ''
    }
    try {
        if (-not $Task.Wait($TimeoutMs)) {
            return ''
        }
        return [string]$Task.Result
    } catch {
        return ''
    }
}

function Get-ZapretCapturedEngineOutput {
    $chunks = New-Object System.Collections.ArrayList
    $proc = $script:ZapmanEngineProcess
    if ($null -ne $proc) {
        try {
            if ($proc.HasExited) {
                $proc.WaitForExit()
                [void]$chunks.Add(('exit {0}' -f $proc.ExitCode))
            }
        } catch {
            [void]$_
        }
    }
    $err = (Get-ZapretTaskText -Task $script:ZapmanEngineErrTask).Trim()
    $out = (Get-ZapretTaskText -Task $script:ZapmanEngineOutTask).Trim()
    if (-not [string]::IsNullOrWhiteSpace($err)) {
        [void]$chunks.Add($err)
    }
    if (-not [string]::IsNullOrWhiteSpace($out)) {
        [void]$chunks.Add($out)
    }
    if ($chunks.Count -eq 0) {
        return ''
    }
    return (@($chunks) -join [Environment]::NewLine)
}

function New-ZapretEngineFailText {
    param([string]$Key)
    $head = Get-ZapmanUiString -Key $Key -FormatArgs @(Get-ZapretEngineExeName)
    $detail = Get-ZapretCapturedEngineOutput
    $full = $head
    if (-not [string]::IsNullOrWhiteSpace($detail)) {
        $full = $head + [Environment]::NewLine + $detail
    }
    Set-ZapmanLastError -Text $full
    return $full
}

function Start-ZapretWinws {
    param([string]$ArgumentList)
    $engine = Get-ZapretEngine
    $exeName = Get-ZapretEngineExeName -Engine $engine
    $exe = Join-Path $script:ZapmanBinDir $exeName
    Confirm-ZapretEngineReady -Engine $engine
    $flat = Complete-ZapretEngineArgumentList -ArgumentList $ArgumentList -Engine $engine
    Close-ZapretEngineCapture
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $exe
    $psi.Arguments = $flat
    $psi.WorkingDirectory = $script:ZapmanBinDir
    $psi.UseShellExecute = $false
    $psi.CreateNoWindow = $true
    $psi.RedirectStandardError = $true
    $psi.RedirectStandardOutput = $true
    $proc = New-Object System.Diagnostics.Process
    $proc.StartInfo = $psi
    try {
        [void]$proc.Start()
    } catch {
        Close-ZapretEngineCapture
        throw
    }
    $script:ZapmanEngineProcess = $proc
    try {
        $script:ZapmanEngineErrTask = $proc.StandardError.ReadToEndAsync()
        $script:ZapmanEngineOutTask = $proc.StandardOutput.ReadToEndAsync()
    } catch {
        $script:ZapmanEngineErrTask = $null
        $script:ZapmanEngineOutTask = $null
    }
}

function Start-ZapretStrategyFile {
    param([string]$Path)
    Invoke-ZapretStrategyPrep
    $engine = Get-ZapretEngine
    $tmpl = Get-ZapretStrategyArgTemplate -Path $Path -Engine $engine
    if ([string]::IsNullOrWhiteSpace($tmpl)) {
        throw (Get-ZapmanUiString -Key 'EngineNoFlags' -FormatArgs @($engine))
    }
    Start-ZapretWinws -ArgumentList $tmpl
}

function ConvertTo-ZapretServiceImagePath {
    param(
        [string]$CommandLine,
        [string]$ExecutablePath
    )

    if ([string]::IsNullOrWhiteSpace($CommandLine) -and [string]::IsNullOrWhiteSpace($ExecutablePath)) {
        return ''
    }

    $exe = $ExecutablePath
    $arguments = ''
    if ($CommandLine) {
        if ($exe) {
            $quotedExe = '"' + $exe + '"'
            if ($CommandLine.StartsWith($quotedExe)) {
                $arguments = $CommandLine.Substring($quotedExe.Length).Trim()
            } elseif ($CommandLine.StartsWith($exe)) {
                $arguments = $CommandLine.Substring($exe.Length).Trim()
            } elseif ($CommandLine -match '^"([^"]+)"\s*(.*)$') {
                $exe = $matches[1]
                $arguments = $matches[2]
            } else {
                return $CommandLine
            }
        } elseif ($CommandLine -match '^"([^"]+)"\s*(.*)$') {
            $exe = $matches[1]
            $arguments = $matches[2]
        } elseif ($CommandLine -match '^(\S+)\s*(.*)$') {
            $exe = $matches[1]
            $arguments = $matches[2]
        } else {
            return $CommandLine
        }
    }

    if ([string]::IsNullOrWhiteSpace($exe)) {
        return $CommandLine
    }
    if ($arguments) {
        return "`"$exe`" $arguments"
    }
    return "`"$exe`""
}

function Test-ZapretBypassRunning {
    $n1 = (@(Get-Process -Name 'winws' -ErrorAction SilentlyContinue)).Count
    $n2 = (@(Get-Process -Name 'winws2' -ErrorAction SilentlyContinue)).Count
    return ($n1 + $n2) -gt 0
}

function Get-ZapretInstalledStrategyName {
    $key = [Microsoft.Win32.Registry]::LocalMachine.OpenSubKey(
        'SYSTEM\CurrentControlSet\Services\zapret',
        $false
    )
    if ($null -eq $key) {
        return ''
    }
    try {
        $raw = $key.GetValue($script:ZapretServiceStrategyValueName)
        if ($null -eq $raw) {
            return ''
        }
        return [string]$raw
    } finally {
        $key.Close()
    }
}

function Get-ZapretIpsetStatus {
    $listFile = Join-Path $script:ZapmanUserDir 'ipset-all.txt'
    if (-not (Test-Path -LiteralPath $listFile)) {
        return 'none'
    }
    $raw = [System.IO.File]::ReadAllText($listFile).Trim()
    if ([string]::IsNullOrWhiteSpace($raw)) {
        return 'any'
    }
    if ($raw -eq '203.0.113.113/32') {
        return 'none'
    }
    return 'loaded'
}

function Set-ZapretIpsetMode {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Mode,
        [string]$BackupName = 'ipset-all.txt.backup',
        [switch]$CopyBackup
    )

    $listFile = Join-Path $script:ZapmanUserDir 'ipset-all.txt'
    $backupFile = Join-Path $script:ZapmanUserDir $BackupName

    if ($Mode -eq 'restore') {
        if (Test-Path -LiteralPath $backupFile) {
            Move-Item -LiteralPath $backupFile -Destination $listFile -Force
        }
        return
    }

    if ($CopyBackup) {
        if (Test-Path -LiteralPath $listFile) {
            Copy-Item -LiteralPath $listFile -Destination $backupFile -Force
        } else {
            [System.IO.File]::WriteAllText($backupFile, '')
        }
        if ($Mode -eq 'any') {
            [System.IO.File]::WriteAllText($listFile, '')
        } elseif ($Mode -eq 'none') {
            [System.IO.File]::WriteAllText($listFile, "203.0.113.113/32`r`n")
        } elseif ($Mode -eq 'loaded') {
            if (-not (Test-Path -LiteralPath $backupFile)) {
                throw 'No IPSet backup found. Use cli.bat service -> Update IPSet List first.'
            }
        } else {
            throw "Unknown IPSet mode: $Mode"
        }
        return
    }

    $current = Get-ZapretIpsetStatus
    if ($current -eq $Mode) {
        return
    }

    if ($current -eq 'loaded') {
        if (Test-Path -LiteralPath $backupFile) {
            Remove-Item -LiteralPath $backupFile -Force
        }
        Rename-Item -LiteralPath $listFile -NewName $BackupName
    }

    switch ($Mode) {
        'none' {
            [System.IO.File]::WriteAllText($listFile, "203.0.113.113/32`r`n")
        }
        'any' {
            [System.IO.File]::WriteAllText($listFile, '')
        }
        'loaded' {
            if (-not (Test-Path -LiteralPath $backupFile)) {
                throw 'No IPSet backup found. Use cli.bat service -> Update IPSet List first.'
            }
            if (Test-Path -LiteralPath $listFile) {
                Remove-Item -LiteralPath $listFile -Force
            }
            Rename-Item -LiteralPath $backupFile -NewName 'ipset-all.txt'
        }
        default {
            throw "Unknown IPSet mode: $Mode"
        }
    }
}

function Stop-ZapretWinwsProcess {
    $procs = @(Get-Process -Name 'winws' -ErrorAction SilentlyContinue)
    $procs2 = @(Get-Process -Name 'winws2' -ErrorAction SilentlyContinue)
    $all = @($procs) + @($procs2)
    if ($all.Count -gt 0) {
        $all | Stop-Process -Force -ErrorAction SilentlyContinue
        Start-Sleep -Milliseconds 400
    }
    Close-ZapretEngineCapture
}

function Wait-ZapretServiceStopped {
    param(
        [string]$Name,
        [int]$TimeoutSeconds = 8,
        [scriptblock]$OnWait
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        Invoke-ZapretOnWait -OnWait $OnWait
        $svc = Get-Service -Name $Name -ErrorAction SilentlyContinue
        if (-not $svc -or $svc.Status -eq 'Stopped') {
            return $true
        }
        Start-Sleep -Milliseconds 250
    } while ((Get-Date) -lt $deadline)
    return $false
}

function Wait-ZapretServiceGone {
    param(
        [string]$Name,
        [int]$TimeoutSeconds = 8,
        [scriptblock]$OnWait
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        Invoke-ZapretOnWait -OnWait $OnWait
        if (-not (Test-ZapretNamedService -Name $Name)) {
            return $true
        }
        Start-Sleep -Milliseconds 400
    } while ((Get-Date) -lt $deadline)
    return $false
}

function Stop-ZapretBypass {
    param([scriptblock]$OnWait)
    try {
        $svc = Get-ZapretService
        if ($svc -and $svc.Status -ne 'Stopped') {
            Stop-Service -Name 'zapret' -Force -ErrorAction SilentlyContinue
            & net.exe stop zapret 2>$null | Out-Null
            [void](Wait-ZapretServiceStopped -Name 'zapret' -TimeoutSeconds 8 -OnWait $OnWait)
        }
    } finally {
        Stop-ZapretWinwsProcess
    }
}

function Wait-ZapretWinws {
    param(
        [int]$TimeoutSeconds = 12,
        [switch]$RequireCommandLine,
        [scriptblock]$OnWait
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        Invoke-ZapretOnWait -OnWait $OnWait
        $owned = $script:ZapmanEngineProcess
        if ($null -ne $owned) {
            try {
                if ($owned.HasExited) {
                    return $false
                }
            } catch {
                return $false
            }
        }
        Start-Sleep -Milliseconds 250
        if (-not (Test-ZapretBypassRunning)) {
            continue
        }
        Start-Sleep -Milliseconds 300
        Invoke-ZapretOnWait -OnWait $OnWait
        if (-not (Test-ZapretBypassRunning)) {
            continue
        }
        if ($RequireCommandLine) {
            try {
                $cmd = Get-ZapretWinwsCommandLine
            } catch {
                continue
            }
            if ([string]::IsNullOrWhiteSpace($cmd)) {
                continue
            }
        }
        return $true
    } while ((Get-Date) -lt $deadline)
    return $false
}

function Get-ZapretWinwsCommandLine {
    $engine = Get-ZapretEngine
    $first = Get-ZapretEngineExeName -Engine $engine
    $second = 'winws.exe'
    if ($first -eq 'winws.exe') {
        $second = 'winws2.exe'
    }
    $proc = $null
    foreach ($name in @($first, $second)) {
        try {
            $proc = Get-CimInstance -ClassName Win32_Process -Filter "Name='$name'" -ErrorAction SilentlyContinue |
                Select-Object -First 1
        } catch {
            $proc = $null
        }
        if ($proc) {
            break
        }
    }
    if (-not $proc) {
        return ''
    }
    if ([string]::IsNullOrWhiteSpace([string]$proc.CommandLine)) {
        return ''
    }
    return ConvertTo-ZapretServiceImagePath -CommandLine ([string]$proc.CommandLine) -ExecutablePath ([string]$proc.ExecutablePath)
}

function Remove-ZapretServiceRecord {
    param([scriptblock]$OnWait)
    if (-not (Test-ZapretNamedService -Name 'zapret')) {
        return
    }
    & net.exe stop zapret 2>$null | Out-Null
    $deleteOut = & sc.exe delete zapret 2>&1
    if (-not (Wait-ZapretServiceGone -Name 'zapret' -TimeoutSeconds 8 -OnWait $OnWait)) {
        throw "Failed to delete the existing zapret service. $deleteOut"
    }
}

function Confirm-ZapretEngineReady {
    param([string]$Engine)
    if ([string]::IsNullOrWhiteSpace($Engine)) {
        $Engine = Get-ZapretEngine
    }
    if (-not (Test-ZapretEngineFiles -Engine $Engine)) {
        if ($Engine -eq 'winws2') {
            throw (Get-ZapmanUiString -Key 'EngineNoWinws2')
        }
        throw (Get-ZapmanUiString -Key 'EngineNoWinws')
    }
    $pinErrs = @(Test-ZapretBinVersions -Engine $Engine)
    if (@($pinErrs).Count -gt 0) {
        throw ($pinErrs -join [Environment]::NewLine)
    }
}

function Get-ZapretServiceRecordSnapshot {
    if (-not (Test-ZapretNamedService -Name 'zapret')) {
        return $null
    }
    $verifyKey = [Microsoft.Win32.Registry]::LocalMachine.OpenSubKey(
        'SYSTEM\CurrentControlSet\Services\zapret',
        $false
    )
    if ($null -eq $verifyKey) {
        return $null
    }
    try {
        $image = [string]$verifyKey.GetValue(
            'ImagePath',
            '',
            [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames
        )
        $name = [string]$verifyKey.GetValue($script:ZapretServiceStrategyValueName, '')
    } finally {
        $verifyKey.Close()
    }
    if ([string]::IsNullOrWhiteSpace($image)) {
        return $null
    }
    return New-Object PSObject -Property @{
        ImagePath     = $image
        StrategyName  = $name
    }
}

function Write-ZapretServiceImagePath {
    param(
        [string]$CommandLine,
        [string]$StrategyName
    )
    $writableKey = [Microsoft.Win32.Registry]::LocalMachine.OpenSubKey(
        'SYSTEM\CurrentControlSet\Services\zapret',
        $true
    )
    if ($null -eq $writableKey) {
        throw 'Cannot open the zapret service registry key for write.'
    }
    try {
        $writableKey.SetValue('ImagePath', $CommandLine, [Microsoft.Win32.RegistryValueKind]::ExpandString)
        if (-not [string]::IsNullOrWhiteSpace($StrategyName)) {
            $writableKey.SetValue($script:ZapretServiceStrategyValueName, $StrategyName, [Microsoft.Win32.RegistryValueKind]::String)
        }
        try {
            $writableKey.DeleteValue($script:ZapretServiceStrategyValueNameLegacy, $false)
        } catch {
            $null = $_.Exception
        }
    } finally {
        $writableKey.Close()
    }

    $verifyKey = [Microsoft.Win32.Registry]::LocalMachine.OpenSubKey(
        'SYSTEM\CurrentControlSet\Services\zapret',
        $false
    )
    if ($null -eq $verifyKey) {
        throw 'Cannot read the zapret service registry key.'
    }
    try {
        $written = [string]$verifyKey.GetValue(
            'ImagePath',
            '',
            [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames
        )
        $writtenName = [string]$verifyKey.GetValue($script:ZapretServiceStrategyValueName, '')
    } finally {
        $verifyKey.Close()
    }
    if ($written -ne $CommandLine) {
        throw 'Failed to write the service ImagePath.'
    }
    if (-not [string]::IsNullOrWhiteSpace($StrategyName) -and $writtenName -ne $StrategyName) {
        throw 'Failed to write the installed strategy name.'
    }
}

function New-ZapretServiceRecord {
    param(
        [string]$CommandLine,
        [string]$StrategyName,
        [scriptblock]$OnWait
    )
    $created = $false
    $createOut = $null
    for ($try = 1; $try -le 8; $try++) {
        $createOut = & sc.exe create zapret binPath= 'placeholder' DisplayName= 'zapret' start= auto 2>&1
        if ($LASTEXITCODE -eq 0) {
            $created = $true
            break
        }
        Start-Sleep -Milliseconds 400
        Invoke-ZapretOnWait -OnWait $OnWait
    }
    if (-not $created) {
        throw "Failed to create the service. $createOut"
    }
    Write-ZapretServiceImagePath -CommandLine $CommandLine -StrategyName $StrategyName
    & sc.exe description zapret 'Zapret DPI bypass software' | Out-Null
}

function Restore-ZapretServiceRecordSnapshot {
    param(
        $Snapshot,
        [scriptblock]$OnWait
    )
    if (-not $Snapshot) {
        return
    }
    $image = [string]$Snapshot.ImagePath
    if ([string]::IsNullOrWhiteSpace($image)) {
        return
    }
    $name = ''
    if ($Snapshot.PSObject.Properties['StrategyName'] -and $null -ne $Snapshot.StrategyName) {
        $name = [string]$Snapshot.StrategyName
    }
    if (Test-ZapretNamedService -Name 'zapret') {
        & net.exe stop zapret 2>$null | Out-Null
        & sc.exe delete zapret 2>$null | Out-Null
        [void](Wait-ZapretServiceGone -Name 'zapret' -TimeoutSeconds 8 -OnWait $OnWait)
    }
    New-ZapretServiceRecord -CommandLine $image -StrategyName $name -OnWait $OnWait
}

function Start-ZapretSelectedStrategy {
    param(
        [System.IO.FileInfo]$File,
        [scriptblock]$OnWait
    )

    if (-not $File) {
        throw 'Select a strategy first.'
    }

    $engine = Get-ZapretEngine
    if (-not (Test-ZapretStrategySupportsEngine -Path $File.FullName -Engine $engine)) {
        throw (Get-ZapmanUiString -Key 'EngineNoFlags' -FormatArgs @($engine))
    }
    if (-not (Test-ZapretEngineFiles -Engine $engine)) {
        if ($engine -eq 'winws2') {
            throw (Get-ZapmanUiString -Key 'EngineNoWinws2')
        }
        throw (Get-ZapmanUiString -Key 'EngineNoWinws')
    }

    $svc = Get-ZapretService
    if ($svc -or (Test-ZapretBypassRunning)) {
        Stop-ZapretBypass -OnWait $OnWait
    }

    Enable-ZapmanTcpTimestamps
    $env:NO_UPDATE_CHECK = '1'
    Start-ZapretStrategyFile -Path $File.FullName

    if (-not (Wait-ZapretWinws -TimeoutSeconds 12 -OnWait $OnWait)) {
        throw (New-ZapretEngineFailText -Key 'EngineStartFail')
    }
    Set-ZapmanLastError -Text ''
}

function Start-ZapretServiceIfInstalled {
    param([scriptblock]$OnWait)
    $svc = Get-ZapretService
    if (-not $svc) {
        throw 'The zapret service is not installed.'
    }
    if ($svc.Status -eq 'Running') {
        return
    }
    $engine = Get-ZapretServiceEngineName
    if ([string]::IsNullOrWhiteSpace($engine)) {
        $engine = Get-ZapretEngine
    }
    Confirm-ZapretEngineReady -Engine $engine
    Enable-ZapmanTcpTimestamps
    Start-Service -Name 'zapret' -ErrorAction Stop
    if (-not (Wait-ZapretWinws -TimeoutSeconds 12 -OnWait $OnWait)) {
        throw (New-ZapretEngineFailText -Key 'EngineServiceFail')
    }
    Set-ZapmanLastError -Text ''
}

function Install-ZapretService {
    param(
        [System.IO.FileInfo]$File,
        [scriptblock]$OnWait
    )

    if (-not $File) {
        throw 'Select a strategy first.'
    }

    $engine = Get-ZapretEngine
    if (-not (Test-ZapretStrategySupportsEngine -Path $File.FullName -Engine $engine)) {
        throw (Get-ZapmanUiString -Key 'EngineNoFlags' -FormatArgs @($engine))
    }
    Confirm-ZapretEngineReady -Engine $engine

    $commandLine = Get-ZapretStrategyServiceImagePath -Path $File.FullName -Engine $engine
    if ([string]::IsNullOrWhiteSpace($commandLine)) {
        throw (Get-ZapmanUiString -Key 'EngineCmdFail' -FormatArgs @(Get-ZapretEngineExeName -Engine $engine))
    }

    $snap = Get-ZapretServiceRecordSnapshot
    Stop-ZapretBypass -OnWait $OnWait
    Remove-ZapretServiceRecord -OnWait $OnWait

    $created = $false
    try {
        Enable-ZapmanTcpTimestamps
        $env:NO_UPDATE_CHECK = '1'
        New-ZapretServiceRecord -CommandLine $commandLine -StrategyName $File.BaseName -OnWait $OnWait
        $created = $true

        Start-Service -Name 'zapret' -ErrorAction Stop
        $waitSec = 12
        if ($engine -eq 'winws2') {
            $waitSec = 15
        }
        if (-not (Wait-ZapretWinws -TimeoutSeconds $waitSec -RequireCommandLine -OnWait $OnWait)) {
            throw (New-ZapretEngineFailText -Key 'EngineServiceFail')
        }
        Set-ZapmanLastError -Text ''
    } catch {
        if ($created -or (Test-ZapretNamedService -Name 'zapret')) {
            & net.exe stop zapret 2>$null | Out-Null
            & sc.exe delete zapret 2>$null | Out-Null
            [void](Wait-ZapretServiceGone -Name 'zapret' -TimeoutSeconds 8 -OnWait $OnWait)
        }
        if ($snap) {
            try {
                Restore-ZapretServiceRecordSnapshot -Snapshot $snap -OnWait $OnWait
            } catch {
                $null = $_.Exception
            }
        }
        throw
    }
}

function Get-ZapretInstalledStrategyFile {
    $name = Get-ZapretInstalledStrategyName
    if ([string]::IsNullOrWhiteSpace($name)) {
        return $null
    }
    foreach ($file in @(Get-ZapretStrategyFiles)) {
        if ($file.BaseName -eq $name) {
            return $file
        }
    }
    return $null
}

# Remember the installed service, then delete only zapret. Do not delete WinDivert.
function Suspend-ZapretServiceForTests {
    param([scriptblock]$OnWait)
    if (-not (Get-ZapretService)) {
        return $null
    }
    $engine = Get-ZapretServiceEngineName
    if ([string]::IsNullOrWhiteSpace($engine)) {
        $engine = Get-ZapretEngine
    }
    $snap = New-Object PSObject -Property @{
        File   = (Get-ZapretInstalledStrategyFile)
        Engine = $engine
    }
    Stop-ZapretBypass -OnWait $OnWait
    Remove-ZapretServiceRecord -OnWait $OnWait
    return $snap
}

function Restore-ZapretServiceAfterTests {
    param(
        $Snapshot,
        [scriptblock]$OnWait
    )
    if (-not $Snapshot) {
        return
    }
    $file = $null
    if ($Snapshot.PSObject.Properties['File']) {
        $file = $Snapshot.File
    }
    if (-not $file) {
        throw 'Cannot restore the zapret service: the strategy file is missing.'
    }
    if (-not (Test-Path -LiteralPath $file.FullName)) {
        throw ('Cannot restore the zapret service: the strategy file is missing: {0}' -f $file.FullName)
    }
    $engine = [string]$Snapshot.Engine
    if ($engine -ne 'winws' -and $engine -ne 'winws2') {
        $engine = Get-ZapretEngine
    }
    Set-ZapretEngine -Engine $engine | Out-Null
    Install-ZapretService -File $file -OnWait $OnWait
}

function Remove-ZapretServices {
    param([scriptblock]$OnWait)
    # Stop zapret, stop winws, then stop WinDivert. Do not stop the driver first.
    if (Test-ZapretNamedService -Name 'zapret') {
        & net.exe stop zapret 2>$null | Out-Null
        [void](Wait-ZapretServiceStopped -Name 'zapret' -TimeoutSeconds 8 -OnWait $OnWait)
        & sc.exe delete zapret | Out-Null
        [void](Wait-ZapretServiceGone -Name 'zapret' -TimeoutSeconds 8 -OnWait $OnWait)
    }

    Stop-ZapretWinwsProcess

    if (Test-ZapretNamedService -Name 'WinDivert') {
        & net.exe stop WinDivert 2>$null | Out-Null
        if (Test-ZapretNamedService -Name 'WinDivert') {
            & sc.exe delete WinDivert | Out-Null
        }
        [void](Wait-ZapretServiceGone -Name 'WinDivert' -TimeoutSeconds 8 -OnWait $OnWait)
    }

    if (Test-ZapretNamedService -Name 'WinDivert14') {
        & net.exe stop WinDivert14 2>$null | Out-Null
        if (Test-ZapretNamedService -Name 'WinDivert14') {
            & sc.exe delete WinDivert14 | Out-Null
        }
        [void](Wait-ZapretServiceGone -Name 'WinDivert14' -TimeoutSeconds 8 -OnWait $OnWait)
    }

    $left = New-Object System.Collections.Generic.List[string]
    foreach ($name in @('zapret', 'WinDivert', 'WinDivert14')) {
        if (Test-ZapretNamedService -Name $name) {
            [void]$left.Add($name)
        }
    }
    if (Test-ZapretBypassRunning) {
        [void]$left.Add('winws.exe/winws2.exe')
    }
    if ($left.Count -gt 0) {
        throw "Failed to remove: $($left -join ', ')"
    }
}
