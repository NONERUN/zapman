# Strategy test runner. GUI calls Invoke-ZapmanStrategyTests and reads each line via -OnLine.

$script:ZapmanTestOnLine = $null
$script:ZapmanTestShouldStop = $null

function Write-ZapmanTestHost {
    param(
        [Parameter(Position = 0, ValueFromRemainingArguments = $true)]
        [object[]]$Object,
        [ConsoleColor]$ForegroundColor,
        [ConsoleColor]$BackgroundColor,
        [switch]$NoNewline
    )
    $text = ''
    if ($Object) {
        $text = (@($Object) | ForEach-Object { "$_" }) -join ' '
    }
    $splat = @{ Object = $text }
    if ($PSBoundParameters.ContainsKey('ForegroundColor')) {
        $splat.ForegroundColor = $ForegroundColor
    }
    if ($PSBoundParameters.ContainsKey('BackgroundColor')) {
        $splat.BackgroundColor = $BackgroundColor
    }
    if ($NoNewline) {
        $splat.NoNewline = $true
    }
    Microsoft.PowerShell.Utility\Write-Host @splat
    if ($script:ZapmanTestOnLine) {
        try {
            & $script:ZapmanTestOnLine $text ([bool]$NoNewline)
        } catch {
            $script:ZapmanTestOnLine = $null
            Microsoft.PowerShell.Utility\Write-Host ("[WARN] Test log callback failed: {0}" -f $_.Exception.Message) -ForegroundColor Yellow
        }
    }
}

function Test-ZapmanTestStopRequested {
    if (-not $script:ZapmanTestShouldStop) {
        return $false
    }
    try {
        return [bool](& $script:ZapmanTestShouldStop)
    } catch {
        return $false
    }
}

function Wait-WinwsReady {
    $name = Get-ZapretEngineProcessName
    $limitMs = 8000
    if ((Get-ZapretEngine) -eq 'winws2') {
        $limitMs = 12000
    }
    $timer = [Diagnostics.Stopwatch]::StartNew()
    while ($timer.ElapsedMilliseconds -lt $limitMs) {
        if (Get-Process -Name $name -ErrorAction SilentlyContinue) {
            Start-Sleep -Milliseconds 300
            return $true
        }
        Start-Sleep -Milliseconds 200
    }
    return $false
}

function New-OrderedDict { New-Object System.Collections.Specialized.OrderedDictionary }
function Add-OrSet {
    param($dict, $key, $val)
    if ($dict.Contains($key)) { $dict[$key] = $val } else { $dict.Add($key, $val) }
}

# Convert raw target value to structured target (supports PING:ip for ping-only targets)
function Convert-Target {
    param(
        [string]$Name,
        [string]$Value
    )

    if ($Value -like "PING:*") {
        $ping = $Value -replace '^PING:\s*', ''
        $url = $null
        $pingTarget = $ping
    } else {
        $url = $Value
        $pingTarget = $url -replace "^https?://", "" -replace "/.*$", ""
    }

    return (New-Object PSObject -Property @{
        Name       = $Name
        Url        = $url
        PingTarget = $pingTarget
    })
}

# DPI checker defaults (override via MONITOR_* env vars like in monitor.ps1)
$dpiTimeoutSeconds = 5
$dpiRangeBytes = 65536
$defaultMaxParallel = [Math]::Min(16, [Math]::Max(8, [Environment]::ProcessorCount * 2))
$dpiMaxParallel = $defaultMaxParallel
$dpiCustomHost = $env:MONITOR_HOST
if ($env:MONITOR_TIMEOUT) { [int]$dpiTimeoutSeconds = $env:MONITOR_TIMEOUT }
if ($env:MONITOR_RANGE) { [int]$dpiRangeBytes = $env:MONITOR_RANGE }
if ($env:MONITOR_MAX_PARALLEL) { [int]$dpiMaxParallel = $env:MONITOR_MAX_PARALLEL }
$standardCurlTimeout = 4
$standardMaxParallel = $defaultMaxParallel
if ($env:TEST_CURL_TIMEOUT) { [int]$standardCurlTimeout = $env:TEST_CURL_TIMEOUT }
if ($env:TEST_MAX_PARALLEL) { [int]$standardMaxParallel = $env:TEST_MAX_PARALLEL }

function Get-DpiSuite {
    # Suite sourced from https://github.com/hyperion-cs/dpi-checkers (Apache-2.0 license)
    # Original copyright retained from dpi-checkers repository
    $url = "https://hyperion-cs.github.io/dpi-checkers/ru/tcp-16-20/suite.v2.json"

    try {
        (Invoke-RestMethod -Uri $url -TimeoutSec $dpiTimeoutSeconds) |
            Select-Object `
                @{n='Id';       e={$_.id}},
                @{n='Provider'; e={$_.provider}},
                @{n='Country';  e={$_.country}},
                @{n='Host';     e={$_.host}}
    }
    catch {
        Write-ZapmanTestHost "[WARN] Fetch dpi suite failed." -ForegroundColor Yellow
        @()
    }
}

function Get-ZapretDpiTargets {
    param(
        [string]$CustomHost
    )

    $suite = Get-DpiSuite
    $targets = @()

    if ($CustomHost) {
        $targets += @{ Id = "CUSTOM"; Provider = "Custom"; Country = "💡"; Host = $CustomHost }
    } else {
        foreach ($entry in $suite) {
            $targets += @{ Id = $entry.Id; Country = $entry.Country; Provider = $entry.Provider; Host = $entry.Host }
        }
    }

    return $targets
}

function Invoke-DpiSuite {
    param(
        [array]$Targets,
        [int]$TimeoutSeconds,
        [int]$RangeBytes,
        [int]$MaxParallel
    )

    $tests = @(
        @{ Label = "HTTP";   Args = @("--http1.1") },
        @{ Label = "TLS1.2"; Args = @("--tlsv1.2", "--tls-max", "1.2") },
        @{ Label = "TLS1.3"; Args = @("--tlsv1.3", "--tls-max", "1.3") }
    )

    $rangeSpec = "0-$($RangeBytes - 1)"
    $warnDetected = $false

    Write-ZapmanTestHost "[INFO] Targets: $($Targets.Count) (custom URL overrides suite). Range: $rangeSpec bytes; Timeout: $($TimeoutSeconds)s" -ForegroundColor Cyan
    Write-ZapmanTestHost "[INFO] Starting DPI TCP 16-20 checks (parallel: $MaxParallel)..." -ForegroundColor DarkGray

    $runspacePool = [runspacefactory]::CreateRunspacePool(1, $MaxParallel)
    $runspacePool.Open()

    $payload = New-Object byte[] $RangeBytes
    [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($payload)

    $payloadFile = New-TemporaryFile
    [IO.File]::WriteAllBytes($payloadFile, $payload)

    $scriptBlock = {
        param($payloadFile, $target, $tests, $rangeSpec, $TimeoutSeconds)

        $warned = $false
        $lines = @()

        foreach ($test in $tests) {
            $curlArgs = @(
                "--range", $rangeSpec,
                "-m", $TimeoutSeconds,
                "--connect-timeout", ([Math]::Min(3, $TimeoutSeconds)),
                "-w", "%{http_code} %{size_upload} %{size_download} %{time_total}",
                "-o", "NUL",
                "-X", "POST",
                "--data-binary", "@$payloadFile",
                "-s"
            ) + $test.Args + @("https://$($target.Host)")

            $output = & curl.exe @curlArgs 2>&1
            $exit = $LASTEXITCODE
            $text = ($output | Out-String).Trim()

            $code = "NA"
            $upBytes = 0
            $downBytes = 0
            $time = -1

            if ($text -match '^(?<code>\d{3})\s+(?<up>\d+)\s+(?<down>\d+)\s+(?<time>[\d\.]+)$') {
                $code = $matches['code']
                $upBytes = [int64]$matches['up']
                $downBytes = [int64]$matches['down']
                $time = [double]$matches['time']
            } elseif (($exit -eq 35) -or ($text -match "not supported|does not support|protocol\s+'.+'\s+not\s+supported|protocol\s+.+\s+not\s+supported|unsupported protocol|TLS.not supported|Unrecognized option|Unknown option|unsupported option|unsupported feature|schannel|SSL")) {
                $code = "UNSUP"
            } elseif ($text) {
                $code = "ERR"
            }

            $upKB = [math]::Round($upBytes / 1024, 1)
            $downKB = [math]::Round($downBytes / 1024, 1)
            $status = "OK"
            $color = "Green"

            if ($code -eq "UNSUP") {
                $status = "UNSUPPORTED"
                $color = "Yellow"
            } elseif ($exit -ne 0 -or $code -eq "ERR" -or $code -eq "NA") {
                $status = "FAIL"
                $color = "Red"
            }

            if (($upBytes -gt 0) -and ($downBytes -eq 0) -and ($time -ge $TimeoutSeconds) -and ($exit -ne 0)) {
                $status = "LIKELY_BLOCKED"
                $color = "Yellow"
                $warned = $true
            }

            $lines += [PSCustomObject]@{
                TestLabel = $test.Label
                Code      = $code
                UpBytes   = $upBytes
                UpKB      = $upKB
                DownBytes = $downBytes
                DownKB    = $downKB
                Time      = $time
                Status    = $status
                Color     = $color
                Warned    = $warned
            }
        }

        return [PSCustomObject]@{
            TargetId = $target.Id
            Provider = $target.Provider
            Country   = $target.Country
            Lines    = $lines
            Warned   = $warned
        }
    }

    $runspaces = @()
    foreach ($target in $Targets) {
        $powershell = [powershell]::Create().AddScript($scriptBlock)
        [void]$powershell.AddArgument($payloadFile)
        [void]$powershell.AddArgument($target)
        [void]$powershell.AddArgument($tests)
        [void]$powershell.AddArgument($rangeSpec)
        [void]$powershell.AddArgument($TimeoutSeconds)
        $powershell.RunspacePool = $runspacePool

        $runspaces += [PSCustomObject]@{
            Powershell = $powershell
            Handle     = $powershell.BeginInvoke()
            TargetId   = $target.Id
        }
    }

    $results = @()
    foreach ($rs in $runspaces) {
        $completed = $true
        $stopFailed = $false
        try {
            $waitMs = (([int]$TimeoutSeconds * 3) + 5) * 1000
            $handle = $rs.Handle
            if ($handle -and $handle.AsyncWaitHandle) {
                $completed = $handle.AsyncWaitHandle.WaitOne($waitMs)
                if (-not $completed) {
                    Write-ZapmanTestHost "[WARN] Runspace for [$($rs.TargetId)] timed out after $waitMs ms; stopping runspace..." -ForegroundColor Yellow
                    try {
                        $rs.Powershell.Stop()
                    } catch {
                        $stopFailed = $true
                        Write-ZapmanTestHost "[WARN] Could not stop the timed-out runspace for [$($rs.TargetId)]." -ForegroundColor Yellow
                    }
                }
            }
        } catch {
            $completed = $false
            $stopFailed = $true
            Write-ZapmanTestHost "[WARN] Wait for runspace [$($rs.TargetId)] failed." -ForegroundColor Yellow
            try {
                $rs.Powershell.Stop()
            } catch {
                Write-ZapmanTestHost "[WARN] Could not stop the runspace for [$($rs.TargetId)] after a wait failure." -ForegroundColor Yellow
            }
        }

        $failedLine = [PSCustomObject]@{
            TestLabel = 'RUNSPACE'
            Code      = 'ERR'
            UpBytes   = 0
            UpKB      = 0
            DownBytes = 0
            DownKB    = 0
            Time      = 0
            Status    = 'FAIL'
            Color     = 'Red'
            Warned    = $false
        }

        if ((-not $completed) -and $stopFailed) {
            Write-ZapmanTestHost "[WARN] EndInvoke skipped for [$($rs.TargetId)]; treating as failure." -ForegroundColor Yellow
            $results += [PSCustomObject]@{
                TargetId = $rs.TargetId
                Provider = 'UNKNOWN'
                Country  = 'UNKNOWN'
                Lines    = @($failedLine)
                Warned   = $false
            }
        } else {
            try {
                $res = $rs.Powershell.EndInvoke($rs.Handle)
                $results += $res

                Write-ZapmanTestHost "`n=== [$($res.Country)][$($res.Provider)] $($res.TargetId) ===" -ForegroundColor DarkCyan
                foreach ($line in $res.Lines) {
                    $msg = "[{0}] code={1} buf_up={2} bytes ({3} KB) buf_down={4} bytes ({5} KB) time={6}s status={7}" -f $line.TestLabel, $line.Code, $line.UpBytes, $line.UpKB, $line.DownBytes, $line.DownKB, $line.Time, $line.Status
                    Write-ZapmanTestHost $msg -ForegroundColor $line.Color
                    if ($line.Status -eq "LIKELY_BLOCKED") {
                        Write-ZapmanTestHost "  Pattern matches 16-20KB freeze; censor likely cutting this strategy." -ForegroundColor Yellow
                    }
                }

                if ($res.Warned) {
                    $warnDetected = $true
                } else {
                    Write-ZapmanTestHost "  No 16-20KB freeze pattern for this target." -ForegroundColor Green
                }
            } catch {
                Write-ZapmanTestHost "[WARN] EndInvoke failed for [$($rs.TargetId)]; treating as failure." -ForegroundColor Yellow
                $results += [PSCustomObject]@{
                    TargetId = $rs.TargetId
                    Provider = 'UNKNOWN'
                    Country  = 'UNKNOWN'
                    Lines    = @($failedLine)
                    Warned   = $false
                }
            }
        }
        $rs.Powershell.Dispose()
    }
    $runspacePool.Close()
    $runspacePool.Dispose()
    Remove-Item -LiteralPath $payloadFile -Force -ErrorAction SilentlyContinue

    if ($warnDetected) {
        Write-ZapmanTestHost ""
        Write-ZapmanTestHost "[WARNING] Detected possible DPI TCP 16-20 blocking on one or more targets. Consider changing strategy/SNI/IP." -ForegroundColor Red
    } else {
        Write-ZapmanTestHost ""
        Write-ZapmanTestHost "[OK] No 16-20KB freeze pattern detected across targets." -ForegroundColor Green
    }

    return $results
}

function Test-ZapretServiceConflict {
    return [bool](Get-Service -Name "zapret" -ErrorAction SilentlyContinue)
}

function Read-TestType {
    while ($true) {
        Write-ZapmanTestHost ""
        Write-ZapmanTestHost "Select test type:" -ForegroundColor Cyan
        Write-ZapmanTestHost "  [1] Standard tests (HTTP/ping)" -ForegroundColor Gray
        Write-ZapmanTestHost "  [2] DPI checkers (TCP 16-20 freeze)" -ForegroundColor Gray
        $choice = Read-Host "Enter 1 or 2"
        switch ($choice) {
            '1' { return 'standard' }
            '2' { return 'dpi' }
            default { Write-ZapmanTestHost "Incorrect input. Please try again." -ForegroundColor Yellow }
        }
    }
}

# Select test mode: all configs or custom subset
function Read-ModeSelection {
    while ($true) {
        Write-ZapmanTestHost ""
        Write-ZapmanTestHost "Select test run mode:" -ForegroundColor Cyan
        Write-ZapmanTestHost "  [1] All configs" -ForegroundColor Gray
        Write-ZapmanTestHost "  [2] Selected configs" -ForegroundColor Gray
        $choice = Read-Host "Enter 1 or 2"
        switch ($choice) {
            '1' { return 'all' }
            '2' { return 'select' }
            default { Write-ZapmanTestHost "Incorrect input. Please try again." -ForegroundColor Yellow }
        }
    }
}

function Read-ConfigSelection {
    param([array]$allFiles)

    while ($true) {
        Write-ZapmanTestHost ""
        Write-ZapmanTestHost "Available configs:" -ForegroundColor Cyan
        for ($i = 0; $i -lt $allFiles.Count; $i++) {
            $idx = $i + 1
            Write-ZapmanTestHost "  [$idx] $($allFiles[$i].Name)" -ForegroundColor Gray
        }

        $selectionInput = Read-Host "Enter numbers (e.g. 1,3,5) , ranges (e.g. 2-7), or mixed (e.g. 1,5-10,12). '0' for all"
        $trimmed = $selectionInput.Trim()

        if ($trimmed -eq '0') {
            return $allFiles
        }

        $parts = $selectionInput -split '[,\s]+' | Where-Object { $_ -match '^\d+(-\d+)?$' }
        if ($parts.Count -eq 0) {
            Write-ZapmanTestHost ""
            Write-ZapmanTestHost "Invalid input format. Use numbers, ranges (1-5), or combinations (1,3-7,10). Try again." -ForegroundColor Yellow
            continue
        }
        $selectedIndices = @()
        $hasErrors = $false

        foreach ($part in $parts) {
            if ($part -match '^(\d+)-(\d+)$') {
                $start = [int]$matches[1]
                $end = [int]$matches[2]

                if ($start -gt $end) {
                    Write-ZapmanTestHost "  [WARN] Invalid range '$part' (start > end). Skipping." -ForegroundColor Yellow
                    $hasErrors = $true
                    continue
                }

                if ($start -lt 1 -or $end -gt $allFiles.Count) {
                    Write-ZapmanTestHost "  [WARN] Range '$part' out of bounds (valid: 1-$($allFiles.Count)). Skipping invalid parts." -ForegroundColor Yellow
                    $hasErrors = $true
                    $start = [Math]::Max($start, 1)
                    $end = [Math]::Min($end, $allFiles.Count)
                }

                for ($i = $start; $i -le $end; $i++) {
                    $selectedIndices += $i
                }
            } else {
                $num = [int]$part
                if ($num -ge 1 -and $num -le $allFiles.Count) {
                    $selectedIndices += $num
                } else {
                    Write-ZapmanTestHost "  [WARN] Number '$num' out of bounds (valid: 1-$($allFiles.Count)). Skipping." -ForegroundColor Yellow
                    $hasErrors = $true
                }
            }
        }
        $valid = $selectedIndices | Sort-Object -Unique | Where-Object { $_ -ge 1 -and $_ -le $allFiles.Count }
        if ($valid.Count -eq 0) {
            Write-ZapmanTestHost ""
            Write-ZapmanTestHost "No valid configs selected. Try again." -ForegroundColor Yellow
            continue
        }

        # Checker
         Write-ZapmanTestHost "Selected configs: $($valid -join ', ')" -ForegroundColor Green
        if ($hasErrors) {
            Write-ZapmanTestHost "Some entries were skipped due to errors (see warnings above)." -ForegroundColor Yellow
        }

        return $valid | ForEach-Object { $allFiles[$_ - 1] }
    }
}

function Stop-ZapretTestWinws {
    Stop-ZapretWinwsProcess
}

function Get-ZapretTestWinwsSnapshot {
    try {
        $a = @(Get-CimInstance Win32_Process -Filter "Name='winws.exe'" -ErrorAction SilentlyContinue)
        $b = @(Get-CimInstance Win32_Process -Filter "Name='winws2.exe'" -ErrorAction SilentlyContinue)
        return @($a + $b) | Select-Object ProcessId, CommandLine, ExecutablePath
    } catch {
        return @()
    }
}

function Restore-ZapretTestWinwsSnapshot {
    param($snapshot)

    if (-not $snapshot -or $snapshot.Count -eq 0) { return }

    $current = @()
    try { $current = (Get-ZapretTestWinwsSnapshot).CommandLine } catch { $current = @() }

    Write-ZapmanTestHost "[INFO] Restoring previously running winws instances..." -ForegroundColor DarkGray
    foreach ($p in $snapshot) {
        if (-not $p.ExecutablePath) { continue }

        if ($current -and $current -contains $p.CommandLine) { continue }

        $exe = $p.ExecutablePath
        $processArgs = ""
        if ($p.CommandLine) {
            $quotedExe = '"' + $exe + '"'
            if ($p.CommandLine.StartsWith($quotedExe)) {
                $processArgs = $p.CommandLine.Substring($quotedExe.Length).Trim()
            } elseif ($p.CommandLine.StartsWith($exe)) {
                $processArgs = $p.CommandLine.Substring($exe.Length).Trim()
            }
        }

        Start-Process -FilePath $exe -ArgumentList $processArgs -WorkingDirectory (Split-Path $exe -Parent) -WindowStyle Minimized | Out-Null
    }
}


function Invoke-ZapmanStrategyTestsCore {
    param(
        [string]$TestType,
        [string[]]$Names,
        [scriptblock]$OnLine,
        [scriptblock]$ShouldStop,
        [switch]$AskType,
        [switch]$AskNames
    )

    $script:ZapmanTestOnLine = $OnLine
    $script:ZapmanTestShouldStop = $ShouldStop
    $script:ZapmanTestExitCode = 1

    try {
        $hasErrors = $false
        $layout = Get-ZapretLayout
        $rootDir = $layout.Root
        $resultsDir = $layout.Results
        if (-not (Test-Path $resultsDir)) {
            New-Item -ItemType Directory -Path $resultsDir | Out-Null
        }

        $dpiTimeoutSeconds = 5
        $dpiRangeBytes = 65536
        $defaultMaxParallel = [Math]::Min(16, [Math]::Max(8, [Environment]::ProcessorCount * 2))
        $dpiMaxParallel = $defaultMaxParallel
        $dpiCustomHost = $env:MONITOR_HOST
        if ($env:MONITOR_TIMEOUT) { [int]$dpiTimeoutSeconds = $env:MONITOR_TIMEOUT }
        if ($env:MONITOR_RANGE) { [int]$dpiRangeBytes = $env:MONITOR_RANGE }
        if ($env:MONITOR_MAX_PARALLEL) { [int]$dpiMaxParallel = $env:MONITOR_MAX_PARALLEL }
        $standardCurlTimeout = 4
        $standardMaxParallel = $defaultMaxParallel
        if ($env:TEST_CURL_TIMEOUT) { [int]$standardCurlTimeout = $env:TEST_CURL_TIMEOUT }
        if ($env:TEST_MAX_PARALLEL) { [int]$standardMaxParallel = $env:TEST_MAX_PARALLEL }

        $currentPrincipal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
        if (-not $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
            Write-ZapmanTestHost "[ERROR] Run as Administrator to execute tests" -ForegroundColor Red
            $hasErrors = $true
        } else {
            Write-ZapmanTestHost "[OK] Administrator rights detected" -ForegroundColor Green
        }

        if (-not (Get-Command "curl.exe" -ErrorAction SilentlyContinue)) {
            Write-ZapmanTestHost "[ERROR] curl.exe not found" -ForegroundColor Red
            Write-ZapmanTestHost "Install curl or add it to PATH" -ForegroundColor Yellow
            $hasErrors = $true
        } else {
            Write-ZapmanTestHost "[OK] curl.exe found" -ForegroundColor Green
        }

        $ipsetFlagFile = Join-Path $layout.Results 'ipset_switched.flag'
        if (Test-Path $ipsetFlagFile) {
            Write-ZapmanTestHost "[INFO] Detected leftover ipset switch flag. Restoring ipset..." -ForegroundColor Yellow
            Set-ZapretIpsetMode -Mode restore -BackupName 'ipset-all.test-backup.txt'
            Remove-Item -Path $ipsetFlagFile -ErrorAction SilentlyContinue
        }

        $originalIpsetStatus = Get-ZapretIpsetStatus

        if ($originalIpsetStatus -ne "any") {
            Write-ZapmanTestHost "[INFO] Current ipset status: $originalIpsetStatus" -ForegroundColor Cyan
            Write-ZapmanTestHost "[WARNING] Ipset will be switched to 'any' for accurate DPI tests." -ForegroundColor Yellow
            Write-ZapmanTestHost "[WARNING] If you close the window with the X button, ipset will NOT restore immediately." -ForegroundColor Yellow
            Write-ZapmanTestHost "[WARNING] It will be restored automatically on the next script run." -ForegroundColor Yellow
        }

        if (Test-ZapretServiceConflict) {
            Write-ZapmanTestHost "[ERROR] Windows service 'zapret' is installed" -ForegroundColor Red
            Write-ZapmanTestHost "         Remove the service before running tests" -ForegroundColor Yellow
            Write-ZapmanTestHost "         Open cli.bat service and choose 'Remove Services'" -ForegroundColor Yellow
            $hasErrors = $true
        }

        if ($hasErrors) {
            Write-ZapmanTestHost ""
            Write-ZapmanTestHost "Fix the errors above and rerun." -ForegroundColor Yellow
            return 1
        }

        $dpiTargets = @()
        $engine = Get-ZapretEngine
        if (-not (Test-ZapretEngineFiles -Engine $engine)) {
            if ($engine -eq 'winws2') {
                Write-ZapmanTestHost ("[ERROR] {0}" -f (Get-ZapmanUiString -Key 'EngineNoWinws2')) -ForegroundColor Red
            } else {
                Write-ZapmanTestHost ("[ERROR] {0}" -f (Get-ZapmanUiString -Key 'EngineNoWinws')) -ForegroundColor Red
            }
            return 1
        }
        $allStrategyFiles = @(Get-ZapretStrategyFiles)
        $batFiles = @(
            $allStrategyFiles | Where-Object {
                Test-ZapretStrategySupportsEngine -Path $_.FullName -Engine $engine
            }
        )
        $skippedEngine = $allStrategyFiles.Count - $batFiles.Count
        Write-ZapmanTestHost ("Engine: {0}" -f $engine) -ForegroundColor Cyan
        if ($skippedEngine -gt 0) {
            Write-ZapmanTestHost ("[INFO] {0} strateg(ies) have no {1} flags and will not run." -f $skippedEngine, $engine) -ForegroundColor DarkGray
        }
        if ($batFiles.Count -lt 1) {
            Write-ZapmanTestHost ("[ERROR] No strategies have {0} flags." -f $engine) -ForegroundColor Red
            return 1
        }
        $globalResults = @()

        if ($AskType -or [string]::IsNullOrWhiteSpace($TestType)) {
            $TestType = Read-TestType
        }
        Write-ZapmanTestHost "Test type: $TestType" -ForegroundColor Cyan

        $nameList = @($Names)
        if ($AskNames) {
            $mode = Read-ModeSelection
            if ($mode -eq 'select') {
                $batFiles = @(Read-ConfigSelection -allFiles $batFiles)
            }
        } elseif ($nameList.Count -gt 0) {
            $want = @($nameList | ForEach-Object { $_.Trim().ToLowerInvariant() } | Where-Object { $_ })
            if ($want.Count -gt 0 -and $want[0] -ne 'all' -and $want[0] -ne '*') {
                $batFiles = @(
                    $batFiles | Where-Object {
                        ($want -contains $_.BaseName.ToLowerInvariant()) -or ($want -contains $_.Name.ToLowerInvariant())
                    }
                )
            }
        }

        if ($TestType -eq 'dpi') {
            $dpiTargets = Get-ZapretDpiTargets -CustomHost $dpiCustomHost
        }
# Load targets once for standard mode
$targetList = @()
$maxNameLen = 10
if ($TestType -eq 'standard') {
    $rawTargets = New-OrderedDict
    foreach ($item in @((Get-ZapmanConfig).testTargets)) {
        $tName = [string]$item.name
        $tVal = [string]$item.value
        if ([string]::IsNullOrWhiteSpace($tName) -or [string]::IsNullOrWhiteSpace($tVal)) {
            continue
        }
        Add-OrSet -dict $rawTargets -key $tName -val $tVal
    }

    if ($rawTargets.Count -eq 0) {
        Write-ZapmanTestHost "[INFO] config.json has no testTargets. Using built-in defaults." -ForegroundColor Gray
        $cfgDefaults = New-ZapmanConfigDefaults
        foreach ($item in @($cfgDefaults.testTargets)) {
            Add-OrSet -dict $rawTargets -key ([string]$item.name) -val ([string]$item.value)
        }
    } else {
        Write-ZapmanTestHost ""
        Write-ZapmanTestHost "[INFO] Loaded targets from config.json" -ForegroundColor Gray
        Write-ZapmanTestHost "[INFO] Targets loaded: $($rawTargets.Count)" -ForegroundColor Gray
    }

    foreach ($key in $rawTargets.Keys) {
        $targetList += Convert-Target -Name $key -Value $rawTargets[$key]
    }

    $maxNameLen = ($targetList | ForEach-Object { $_.Name.Length } | Measure-Object -Maximum).Maximum
    if (-not $maxNameLen -or $maxNameLen -lt 10) { $maxNameLen = 10 }
}

# Ensure we have configs to run
if (-not $batFiles -or $batFiles.Count -eq 0) {
    Write-ZapmanTestHost ("[ERROR] No strategies to test for {0}." -f $engine) -ForegroundColor Red
    return 1
}

$env:NO_UPDATE_CHECK = "1"
$originalWinws = Get-ZapretTestWinwsSnapshot

Write-ZapmanTestHost ""
Write-ZapmanTestHost "============================================================" -ForegroundColor Cyan
Write-ZapmanTestHost "                 ZAPRET CONFIG TESTS" -ForegroundColor Cyan
Write-ZapmanTestHost "                 Mode: $($TestType.ToUpper())" -ForegroundColor Cyan
Write-ZapmanTestHost ("                 Engine: {0}" -f $engine) -ForegroundColor Cyan
Write-ZapmanTestHost "                 Total configs: $($batFiles.Count.ToString().PadLeft(2))" -ForegroundColor Cyan
Write-ZapmanTestHost "============================================================" -ForegroundColor Cyan

try {
    # Save original ipset status and switch to 'any' for accurate DPI tests
    if (($originalIpsetStatus -ne "any") -and ($TestType -eq 'dpi')) {
        Write-ZapmanTestHost "[WARNING] Ipset is in '$originalIpsetStatus' mode. Switching to 'any' for accurate DPI tests..." -ForegroundColor Yellow
        Set-ZapretIpsetMode -Mode any -BackupName 'ipset-all.test-backup.txt' -CopyBackup
        # Create flag file to indicate ipset was switched
        "" | Out-File -FilePath $ipsetFlagFile -Encoding UTF8
    }
    Write-ZapmanTestHost "[WARNING] Tests may take several minutes to complete. Please wait..." -ForegroundColor Yellow

    $configNum = 0
    foreach ($file in $batFiles) {
    if (Test-ZapmanTestStopRequested) {
        Write-ZapmanTestHost "[INFO] Tests cancelled." -ForegroundColor Yellow
        break
    }
    $configNum++
    Write-ZapmanTestHost ""
    Write-ZapmanTestHost "------------------------------------------------------------" -ForegroundColor DarkCyan
    Write-ZapmanTestHost "  [$configNum/$($batFiles.Count)] $($file.Name)" -ForegroundColor Yellow
    Write-ZapmanTestHost "------------------------------------------------------------" -ForegroundColor DarkCyan

    # Cleanup
    Stop-ZapretTestWinws

    # Start config
    if (-not (Test-ZapretStrategySupportsEngine -Path $file.FullName -Engine $engine)) {
        Write-ZapmanTestHost ("  > No {0} flags. Skipping..." -f $engine) -ForegroundColor DarkGray
        continue
    }

    Write-ZapmanTestHost ("  > Starting config ({0})..." -f $engine) -ForegroundColor Cyan
    $proc = Start-Process -FilePath "powershell.exe" -ArgumentList "-NoProfile -ExecutionPolicy Bypass -File `"$($file.FullName)`"" -WorkingDirectory $rootDir -PassThru -WindowStyle Minimized

    # Wait init
    if (-not (Wait-WinwsReady)) {
        Write-ZapmanTestHost ("  > Strategy failed to start ({0} process not found). Skipping..." -f (Get-ZapretEngineExeName -Engine $engine)) -ForegroundColor Red
        if (-not $proc.HasExited) { Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue }
        continue
    }

    if ($TestType -eq 'standard') {
        $curlTimeoutSeconds = $standardCurlTimeout

        # Parallel target checks via runspace pool (faster than jobs)
        $maxParallel = $standardMaxParallel
        $runspacePool = [runspacefactory]::CreateRunspacePool(1, $maxParallel)
        $runspacePool.Open()

        $scriptBlock = {
            param($t, $curlTimeoutSeconds)

            $httpPieces = @()

            if ($t.Url) {
                $tests = @(
                    @{ Label = "HTTP";   Args = @("--http1.1") },
                    @{ Label = "TLS1.2"; Args = @("--tlsv1.2", "--tls-max", "1.2") },
                    @{ Label = "TLS1.3"; Args = @("--tlsv1.3", "--tls-max", "1.3") }
                )

                $baseArgs = @("-I", "-s", "-m", $curlTimeoutSeconds, "--connect-timeout", ([Math]::Min(2, $curlTimeoutSeconds)), "-o", "NUL", "-w", "%{http_code}", "--show-error")
                foreach ($test in $tests) {
                    try {
                        $curlArgs = $baseArgs + $test.Args
                        $rawOutput = & curl.exe @curlArgs $t.Url 2>&1
                        $exit = $LASTEXITCODE
                        $stderr = $null
                        $outputParts = New-Object System.Collections.ArrayList
                        foreach ($item in @($rawOutput)) {
                            if ($item -is [System.Management.Automation.ErrorRecord]) {
                                $stderr += $item.Exception.Message + " "
                            } else {
                                [void]$outputParts.Add($item)
                            }
                        }
                        $httpCode = ($outputParts | Out-String).Trim()

                        $dnsHijack = ($stderr -match "Could not resolve host|certificate|SSL certificate problem|self[- ]?signed|certificate verify failed|unable to get local issuer certificate")
                        if ($dnsHijack) {
                            $httpPieces += "$($test.Label):SSL  "
                            continue
                        }

                        $unsupported = (($exit -eq 35) -or ($stderr -match "does not support|not supported|protocol\s+'?.+'?\s+not\s+supported|unsupported protocol|TLS.*not supported|Unrecognized option|Unknown option|unsupported option|unsupported feature|schannel"))
                        if ($unsupported) {
                            $httpPieces += "$($test.Label):UNSUP"
                            continue
                        }

                        $ok = ($exit -eq 0) -and ($httpCode -match '^[1-5]\d{2}$')
                        if ($ok) {
                            $httpPieces += "$($test.Label):OK   "
                        } else {
                            $httpPieces += "$($test.Label):ERROR"
                        }
                    } catch {
                        $httpPieces += "$($test.Label):ERROR"
                    }
                }
            }

            $pingResult = "n/a"
            if ($t.PingTarget) {
                $ping = $null
                try {
                    $ping = New-Object System.Net.NetworkInformation.Ping
                    $reply = $ping.Send($t.PingTarget, 1000)
                    if ($reply.Status -eq [System.Net.NetworkInformation.IPStatus]::Success) {
                        $pingResult = "{0:N0} ms" -f $reply.RoundtripTime
                    } else {
                        $pingResult = "Timeout"
                    }
                } catch {
                    $pingResult = "Timeout"
                } finally {
                    if ($ping) { $ping.Dispose() }
                }
            }

            return (New-Object PSObject -Property @{
                Name       = $t.Name
                HttpTokens = $httpPieces
                PingResult = $pingResult
                IsUrl      = [bool]$t.Url
            })
        }

        $runspaces = @()
        foreach ($target in $targetList) {
            $ps = [powershell]::Create().AddScript($scriptBlock)
            [void]$ps.AddArgument($target)
            [void]$ps.AddArgument($curlTimeoutSeconds)
            $ps.RunspacePool = $runspacePool

            $runspaces += [PSCustomObject]@{
                Powershell = $ps
                Handle     = $ps.BeginInvoke()
                TargetName = $target.Name
            }
        }

        $script:currentLine = "  > Running tests..."
        Write-ZapmanTestHost $script:currentLine -ForegroundColor DarkGray

        $targetResults = @()
        foreach ($rs in $runspaces) {
            $completed = $true
            $stopFailed = $false
            try {
                $waitMs = (([int]$curlTimeoutSeconds * 3) + 5) * 1000
                $handle = $rs.Handle
                if ($handle -and $handle.AsyncWaitHandle) {
                    $completed = $handle.AsyncWaitHandle.WaitOne($waitMs)
                    if (-not $completed) {
                        Write-ZapmanTestHost "[WARN] Runspace for target timed out after $waitMs ms; stopping runspace..." -ForegroundColor Yellow
                        try {
                            $rs.Powershell.Stop()
                        } catch {
                            $stopFailed = $true
                            Write-ZapmanTestHost "[WARN] Could not stop the timed-out runspace." -ForegroundColor Yellow
                        }
                    }
                }
            } catch {
                $completed = $false
                $stopFailed = $true
                Write-ZapmanTestHost "[WARN] Wait for a test runspace failed." -ForegroundColor Yellow
                try {
                    $rs.Powershell.Stop()
                } catch {
                    Write-ZapmanTestHost "[WARN] Could not stop the runspace after a wait failure." -ForegroundColor Yellow
                }
            }

            if ((-not $completed) -and $stopFailed) {
                Write-ZapmanTestHost "[WARN] EndInvoke skipped; treating as failure." -ForegroundColor Yellow
                $targetResults += [PSCustomObject]@{
                    Name       = $rs.TargetName
                    HttpTokens = @('HTTP:ERROR')
                    PingResult = 'Timeout'
                    IsUrl      = $true
                }
            } else {
                try {
                    $targetResults += $rs.Powershell.EndInvoke($rs.Handle)
                } catch {
                    Write-ZapmanTestHost "[WARN] EndInvoke failed for a runspace; treating as failure." -ForegroundColor Yellow
                    $targetResults += [PSCustomObject]@{
                        Name       = $rs.TargetName
                        HttpTokens = @('HTTP:ERROR')
                        PingResult = 'Timeout'
                        IsUrl      = $true
                    }
                }
            }
            $rs.Powershell.Dispose()
        }

        $runspacePool.Close()
        $runspacePool.Dispose()

        $targetLookup = @{}
        foreach ($res in $targetResults) { $targetLookup[$res.Name] = $res }

        foreach ($target in $targetList) {
            $res = $targetLookup[$target.Name]
            if (-not $res) { continue }

            Write-ZapmanTestHost "  $($target.Name.PadRight($maxNameLen))    " -NoNewline

            if ($res.IsUrl -and $res.HttpTokens) {
                foreach ($tok in $res.HttpTokens) {
                    $tokColor = "Green"
                    if ($tok -match "UNSUP") { $tokColor = "Yellow" }
                    elseif ($tok -match "SSL") { $tokColor = "Red" }
                    elseif ($tok -match "ERR") { $tokColor = "Red" }
                    Write-ZapmanTestHost " $tok" -NoNewline -ForegroundColor $tokColor
                }
                Write-ZapmanTestHost " | Ping: " -NoNewline -ForegroundColor DarkGray
                if ($res.PingResult -eq "Timeout") {
                    $pingColor = "Yellow"
                } else {
                    $pingColor = "Cyan"
                }
                Write-ZapmanTestHost "$($res.PingResult)" -NoNewline -ForegroundColor $pingColor
                Write-ZapmanTestHost ""
            } else {
                # Ping-only target
                Write-ZapmanTestHost " Ping: " -NoNewline -ForegroundColor DarkGray
                if ($res.PingResult -eq "Timeout") {
                    $pingColor = "Red"
                } else {
                    $pingColor = "Cyan"
                }
                Write-ZapmanTestHost "$($res.PingResult)" -ForegroundColor $pingColor
            }

        }

        $globalResults += @{ Config = $file.Name; Type = 'standard'; Results = $targetResults }
    } else {
        Write-ZapmanTestHost "  > Running DPI checkers..." -ForegroundColor DarkGray
        $dpiResults = Invoke-DpiSuite -Targets $dpiTargets -TimeoutSeconds $dpiTimeoutSeconds -RangeBytes $dpiRangeBytes -MaxParallel $dpiMaxParallel
        $globalResults += @{ Config = $file.Name; Type = 'dpi'; Results = $dpiResults }
    }

    # Stop
    Stop-ZapretTestWinws
    if (-not $proc.HasExited) { Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue }
}

    Write-ZapmanTestHost ""
    Write-ZapmanTestHost "All tests finished." -ForegroundColor Green

    # Analytics
    $analytics = @{}
    foreach ($res in $globalResults) {
        if ($res.Type -eq 'standard') {
            foreach ($targetRes in $res.Results) {
                $config = $res.Config
                if (-not $analytics.ContainsKey($config)) { $analytics[$config] = @{ OK = 0; ERROR = 0; UNSUP = 0; PingOK = 0; PingFail = 0 } }
                if ($targetRes.IsUrl) {
                    foreach ($tok in $targetRes.HttpTokens) {
                        if ($tok -match "OK") { $analytics[$config].OK++ }
                        elseif ($tok -match "SSL") { $analytics[$config].ERROR++ }
                        elseif ($tok -match "ERROR") { $analytics[$config].ERROR++ }
                        elseif ($tok -match "UNSUP") { $analytics[$config].UNSUP++ }
                    }
                }
                if ($targetRes.PingResult -ne "Timeout" -and $targetRes.PingResult -ne "n/a") { $analytics[$config].PingOK++ } else { $analytics[$config].PingFail++ }
            }
        } elseif ($res.Type -eq 'dpi') {
            foreach ($targetRes in $res.Results) {
                $config = $res.Config
                if (-not $analytics.ContainsKey($config)) { $analytics[$config] = @{ OK = 0; FAIL = 0; UNSUPPORTED = 0; LIKELY_BLOCKED = 0 } }
                foreach ($line in $targetRes.Lines) {
                    if ($line.Status -eq "OK") { $analytics[$config].OK++ }
                    elseif ($line.Status -eq "FAIL") { $analytics[$config].FAIL++ }
                    elseif ($line.Status -eq "UNSUPPORTED") { $analytics[$config].UNSUPPORTED++ }
                    elseif ($line.Status -eq "LIKELY_BLOCKED") { $analytics[$config].LIKELY_BLOCKED++ }
                }
            }
        }
    }

    if (@($analytics.Keys).Count -eq 0) {
        Write-ZapmanTestHost "No completed strategy results." -ForegroundColor Yellow
        return 1
    }

    Write-ZapmanTestHost ""
    Write-ZapmanTestHost "=== ANALYTICS ===" -ForegroundColor Cyan
    $maxConfigLen = ($analytics.Keys | ForEach-Object { $_.Length } | Measure-Object -Maximum).Maximum
    foreach ($config in $analytics.Keys) {
        $a = $analytics[$config]
        $configPadded = $config.PadRight($maxConfigLen)
        if ($a.ContainsKey('PingOK')) {
            $line = "{0} : HTTP OK: {1,3}, ERR: {2,3}, UNSUP: {3,3}, Ping OK: {4,3}, Fail: {5,3}" -f `
                $configPadded, $a.OK, $a.ERROR, $a.UNSUP, $a.PingOK, $a.PingFail
        } else {
            $line = "{0} : OK: {1,3}, FAIL: {2,3}, UNSUP: {3,3}, BLOCKED: {4,3}" -f `
                $configPadded, $a.OK, $a.FAIL, $a.UNSUPPORTED, $a.LIKELY_BLOCKED
        }
        Write-ZapmanTestHost $line -ForegroundColor Yellow
    }

    # Determine best strategy
    $bestConfig = $null
    $maxScore = 0
    $maxPing = -1
    foreach ($config in $analytics.Keys) {
        $a = $analytics[$config]
        $score = $a.OK
        $pingScore = 0
        if ($a.ContainsKey('PingOK')) {
            $pingScore = $a.PingOK
        }
        if ($score -gt $maxScore) {
            $maxScore = $score
            $maxPing = $pingScore
            $bestConfig = $config
        } elseif ($score -eq $maxScore) {
            if ($pingScore -gt $maxPing) {
                $maxPing = $pingScore
                $bestConfig = $config
            }
        }
    }
    Write-ZapmanTestHost ""
    Write-ZapmanTestHost "Best config: $bestConfig" -ForegroundColor Green
    Write-ZapmanTestHost ""

    # Save to file
    $dateStr = Get-Date -Format "yyyy-MM-dd_HH-mm-ss"
    $resultFile = Join-Path $resultsDir "test_results_$dateStr.txt"
    $resultLines = New-Object System.Collections.Generic.List[string]
    foreach ($res in $globalResults) {
        $config = $res.Config
        $type = $res.Type
        $results = $res.Results
        [void]$resultLines.Add("Config: $config (Type: $type)")
        if ($type -eq 'standard') {
            foreach ($targetRes in $results) {
                $name = $targetRes.Name
                $http = $targetRes.HttpTokens -join ' '
                $ping = $targetRes.PingResult
                [void]$resultLines.Add("  $name : $http | Ping: $ping")
            }
        } elseif ($type -eq 'dpi') {
            foreach ($targetRes in $results) {
                $id = $targetRes.TargetId
                $provider = $targetRes.Provider
                $country = $targetRes.Country
                if ($country) {
                    [void]$resultLines.Add("  Target: [$country] $id ($provider)")
                } else {
                    [void]$resultLines.Add("  Target: $id ($provider)")
                }
                foreach ($line in $targetRes.Lines) {
                    $test = $line.TestLabel
                    $code = $line.Code
                    $up = $line.UpKB
                    $down = $line.DownKB
                    $time = $line.Time
                    $status = $line.Status
                    [void]$resultLines.Add("    ${test}: code=${code}  up=${up} KB  down=${down} KB  time=${time}s  status=${status}")
                }
            }
        }
        [void]$resultLines.Add("")
    }

    # Add analytics
    [void]$resultLines.Add("=== ANALYTICS ===")
    $maxConfigLen = ($analytics.Keys | ForEach-Object { $_.Length } | Measure-Object -Maximum).Maximum
    foreach ($config in $analytics.Keys) {
        $a = $analytics[$config]
        $configPadded = $config.PadRight($maxConfigLen)
        if ($a.ContainsKey('PingOK')) {
            $line = "{0} : HTTP OK: {1,3}, ERR: {2,3}, UNSUP: {3,3}, Ping OK: {4,3}, Fail: {5,3}" -f `
                $configPadded, $a.OK, $a.ERROR, $a.UNSUP, $a.PingOK, $a.PingFail
        } else {
            $line = "{0} : OK: {1,3}, FAIL: {2,3}, UNSUP: {3,3}, BLOCKED: {4,3}" -f `
                $configPadded, $a.OK, $a.FAIL, $a.UNSUPPORTED, $a.LIKELY_BLOCKED
        }
        [void]$resultLines.Add($line)
    }

    [void]$resultLines.Add("Best strategy: $bestConfig")
    $resultLines | Set-Content $resultFile -Encoding UTF8

    Write-ZapmanTestHost "Results saved to $resultFile" -ForegroundColor Green
    $script:ZapmanTestExitCode = 0

} catch {
    Write-ZapmanTestHost "[ERROR] An error occurred during tests. Restoring ipset..." -ForegroundColor Red
    if ($originalIpsetStatus -and $originalIpsetStatus -ne "any") {
    Set-ZapretIpsetMode -Mode restore -BackupName 'ipset-all.test-backup.txt'
    }
    Remove-Item -Path $ipsetFlagFile -ErrorAction SilentlyContinue
} finally {
    Stop-ZapretTestWinws
    Restore-ZapretTestWinwsSnapshot -snapshot $originalWinws
    if ($originalIpsetStatus -ne "any") {
        Write-ZapmanTestHost "[INFO] Restoring original ipset mode..." -ForegroundColor DarkGray
    Set-ZapretIpsetMode -Mode restore -BackupName 'ipset-all.test-backup.txt'
    }
    Remove-Item -Path $ipsetFlagFile -ErrorAction SilentlyContinue
}

    } finally {
        $script:ZapmanTestOnLine = $null
        $script:ZapmanTestShouldStop = $null
    }
    return [int]$script:ZapmanTestExitCode
}

