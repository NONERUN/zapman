# Zapret engine helpers: winws start/stop, Windows service zapret, Game Filter, IPSet.
# Zapman.psm1 dotsources this file. Script paths come from src/Zapman/Core.ps1.

Set-StrictMode -Version Latest

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
        Get-ChildItem -LiteralPath $script:ZapmanStrategiesDir -Filter '*.ps1' |
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
    $raw = [System.IO.File]::ReadAllText($Path)
    if ($Engine -eq 'winws2') {
        $m = [regex]::Match($raw, '(?s)\$argListWinws2\s*=\s*@"\r?\n(.+?)\r?\n"@')
        if ($m.Success) {
            return $m.Groups[1].Value
        }
        return ''
    }
    $m = [regex]::Match($raw, '(?s)\$argListWinws\s*=\s*@"\r?\n(.+?)\r?\n"@')
    if (-not $m.Success) {
        $m = [regex]::Match($raw, '(?s)\$argList\s*=\s*@"\r?\n(.+?)\r?\n"@')
    }
    if (-not $m.Success) {
        return ''
    }
    return $m.Groups[1].Value
}

function Test-ZapretStrategySupportsEngine {
    param(
        [string]$Path,
        [string]$Engine
    )
    $tmpl = Get-ZapretStrategyArgTemplate -Path $Path -Engine $Engine
    return -not [string]::IsNullOrWhiteSpace($tmpl)
}

function Expand-ZapretStrategyArgTemplate {
    param([string]$Template)
    $gf = Get-ZapretGameFilter
    $t = [string]$Template
    $t = $t.Replace('$($gf.Tcp)', [string]$gf.Tcp)
    $t = $t.Replace('$($gf.Udp)', [string]$gf.Udp)
    $t = $t.Replace('$bin', $script:ZapmanBinDir)
    $t = $t.Replace('$user', $script:ZapmanUserDir)
    $t = $t.Replace('$lists', $script:ZapmanListsDir)
    return $t
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
    $flat = Complete-ZapretEngineArgumentList -ArgumentList (Expand-ZapretStrategyArgTemplate -Template $tmpl) -Engine $Engine
    return '"' + $exe + '" ' + $flat
}

function Get-ZapretStrategyNameFromWinws {
    $live = Get-ZapretNormalizedArgumentString -Text (Get-ZapretWinwsArgumentString)
    if ([string]::IsNullOrWhiteSpace($live)) {
        return ''
    }
    $hits = New-Object System.Collections.ArrayList
    foreach ($file in @(Get-ZapretStrategyFiles)) {
        foreach ($eng in @('winws', 'winws2')) {
            $tmpl = Get-ZapretStrategyArgTemplate -Path $file.FullName -Engine $eng
            if ([string]::IsNullOrWhiteSpace($tmpl)) {
                continue
            }
            $flat = Complete-ZapretEngineArgumentList -ArgumentList (Expand-ZapretStrategyArgTemplate -Template $tmpl) -Engine $eng
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

function Start-ZapretWinws {
    param([string]$ArgumentList)
    $engine = Get-ZapretEngine
    $exeName = Get-ZapretEngineExeName -Engine $engine
    $exe = Join-Path $script:ZapmanBinDir $exeName
    if (-not (Test-ZapretEngineFiles -Engine $engine)) {
        if ($engine -eq 'winws2') {
            throw (Get-ZapmanUiString -Key 'EngineNoWinws2')
        }
        throw (Get-ZapmanUiString -Key 'EngineNoWinws')
    }
    $pinErrs = @(Test-ZapretBinVersions -Engine $engine)
    if (@($pinErrs).Count -gt 0) {
        throw ($pinErrs -join [Environment]::NewLine)
    }
    $hasLua = $ArgumentList -match '--lua-desync'
    $hasZ1 = $ArgumentList -match '--dpi-desync'
    if ($engine -eq 'winws2' -and $hasZ1 -and -not $hasLua) {
        throw (Get-ZapmanUiString -Key 'EngineNoFlags' -FormatArgs @('winws2'))
    }
    if ($engine -eq 'winws' -and $hasLua) {
        throw (Get-ZapmanUiString -Key 'EngineNoFlags' -FormatArgs @('winws'))
    }
    $flat = Complete-ZapretEngineArgumentList -ArgumentList $ArgumentList -Engine $engine
    Start-Process -FilePath $exe -ArgumentList $flat -WorkingDirectory $script:ZapmanBinDir -WindowStyle Minimized | Out-Null
}

function Start-ZapretStrategyFile {
    param([string]$Path)
    $argList = "-NoProfile -ExecutionPolicy Bypass -File `"$Path`""
    Start-Process -FilePath 'powershell.exe' -ArgumentList $argList -WorkingDirectory $script:ZapmanRoot -WindowStyle Minimized | Out-Null
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
    $path = 'HKLM:\System\CurrentControlSet\Services\zapret'
    try {
        $item = Get-ItemProperty -LiteralPath $path -Name 'zapret-discord-youtube' -ErrorAction Stop
        return [string]$item.'zapret-discord-youtube'
    } catch {
        return ''
    }
}

function Get-ZapretIpsetStatus {
    $listFile = Join-Path $script:ZapmanUserDir 'ipset-all.txt'
    if (-not (Test-Path -LiteralPath $listFile)) {
        return 'none'
    }
    $item = Get-Item -LiteralPath $listFile
    if ($item.Length -lt 1) {
        return 'any'
    }
    # none-mode writes one TEST-NET line. A loaded list is much larger. Do not read the full file.
    if ($item.Length -gt 64) {
        return 'loaded'
    }
    $raw = [System.IO.File]::ReadAllText($listFile)
    if ([string]::IsNullOrWhiteSpace($raw)) {
        return 'any'
    }
    if ($raw -like '*203.0.113.113/32*') {
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
    if ($all.Count -eq 0) {
        return
    }
    $all | Stop-Process -Force -ErrorAction SilentlyContinue
    Start-Sleep -Milliseconds 400
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
        $proc = Get-CimInstance -ClassName Win32_Process -Filter "Name='$name'" -ErrorAction Stop |
            Select-Object -First 1
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
        throw (Get-ZapmanUiString -Key 'EngineStartFail' -FormatArgs @(Get-ZapretEngineExeName -Engine $engine))
    }
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
    Enable-ZapmanTcpTimestamps
    Start-Service -Name 'zapret' -ErrorAction Stop
    if (-not (Wait-ZapretWinws -TimeoutSeconds 12 -OnWait $OnWait)) {
        throw (Get-ZapmanUiString -Key 'EngineServiceFail' -FormatArgs @(Get-ZapretEngineExeName))
    }
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
    if (-not (Test-ZapretEngineFiles -Engine $engine)) {
        if ($engine -eq 'winws2') {
            throw (Get-ZapmanUiString -Key 'EngineNoWinws2')
        }
        throw (Get-ZapmanUiString -Key 'EngineNoWinws')
    }

    Stop-ZapretBypass -OnWait $OnWait
    Remove-ZapretServiceRecord -OnWait $OnWait

    $commandLine = Get-ZapretStrategyServiceImagePath -Path $File.FullName -Engine $engine
    if ([string]::IsNullOrWhiteSpace($commandLine)) {
        throw (Get-ZapmanUiString -Key 'EngineCmdFail' -FormatArgs @(Get-ZapretEngineExeName -Engine $engine))
    }

    Enable-ZapmanTcpTimestamps
    $env:NO_UPDATE_CHECK = '1'
    Start-ZapretStrategyFile -Path $File.FullName

    if (-not (Wait-ZapretWinws -TimeoutSeconds 15 -RequireCommandLine -OnWait $OnWait)) {
        throw (Get-ZapmanUiString -Key 'EngineInstallFail' -FormatArgs @(Get-ZapretEngineExeName -Engine $engine))
    }

    Stop-ZapretBypass -OnWait $OnWait

    $created = $false
    try {
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

        $writableKey = [Microsoft.Win32.Registry]::LocalMachine.OpenSubKey(
            'SYSTEM\CurrentControlSet\Services\zapret',
            $true
        )
        if ($null -eq $writableKey) {
            throw 'Cannot open the zapret service registry key for write.'
        }
        try {
            $writableKey.SetValue('ImagePath', $commandLine, [Microsoft.Win32.RegistryValueKind]::ExpandString)
            $writableKey.SetValue('zapret-discord-youtube', $File.BaseName, [Microsoft.Win32.RegistryValueKind]::String)
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
            $writtenName = [string]$verifyKey.GetValue('zapret-discord-youtube')
        } finally {
            $verifyKey.Close()
        }
        if ($written -ne $commandLine) {
            throw 'Failed to write the service ImagePath.'
        }
        if ($writtenName -ne $File.BaseName) {
            throw 'Failed to write the installed strategy name.'
        }

        & sc.exe description zapret 'Zapret DPI bypass software' | Out-Null

        Start-Service -Name 'zapret' -ErrorAction Stop
        $waitSec = 12
        if ($engine -eq 'winws2') {
            $waitSec = 15
        }
        if (-not (Wait-ZapretWinws -TimeoutSeconds $waitSec -RequireCommandLine -OnWait $OnWait)) {
            throw (Get-ZapmanUiString -Key 'EngineServiceFail' -FormatArgs @(Get-ZapretEngineExeName -Engine $engine))
        }
    } catch {
        if ($created -or (Test-ZapretNamedService -Name 'zapret')) {
            & net.exe stop zapret 2>$null | Out-Null
            & sc.exe delete zapret 2>$null | Out-Null
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
        return
    }
    if (-not (Test-Path -LiteralPath $file.FullName)) {
        return
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
        & sc.exe delete zapret | Out-Null
        [void](Wait-ZapretServiceGone -Name 'zapret' -TimeoutSeconds 8 -OnWait $OnWait)
    }

    Stop-ZapretWinwsProcess

    if (Test-ZapretNamedService -Name 'WinDivert') {
        & net.exe stop WinDivert 2>$null | Out-Null
        if (Test-ZapretNamedService -Name 'WinDivert') {
            & sc.exe delete WinDivert | Out-Null
        }
        [void](Wait-ZapretServiceGone -Name 'WinDivert' -TimeoutSeconds 5 -OnWait $OnWait)
    }

    if (Test-ZapretNamedService -Name 'WinDivert14') {
        & net.exe stop WinDivert14 2>$null | Out-Null
        if (Test-ZapretNamedService -Name 'WinDivert14') {
            & sc.exe delete WinDivert14 | Out-Null
        }
        [void](Wait-ZapretServiceGone -Name 'WinDivert14' -TimeoutSeconds 5 -OnWait $OnWait)
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
