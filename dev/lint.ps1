# Project lint: Zapret 5.1/WPF checks, then PSScriptAnalyzer and Blinter if installed.
# Usage: powershell.exe -NoProfile -ExecutionPolicy Bypass -File dev\lint.ps1
# This script re-enters -STA so XamlReader.Load can parse src\gui\*.xaml.
# Bat files: pipx install Blinter

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

if ([System.Threading.Thread]::CurrentThread.GetApartmentState() -ne 'STA') {
    $psExe = Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'
    $self = $PSCommandPath
    if ([string]::IsNullOrWhiteSpace($self)) {
        $self = $MyInvocation.MyCommand.Path
    }
    & $psExe -NoProfile -STA -ExecutionPolicy Bypass -File $self @args
    exit $LASTEXITCODE
}

$root = Split-Path -Parent $PSScriptRoot
$settings = Join-Path $root 'PSScriptAnalyzerSettings.psd1'

function Get-ZapretLintFiles {
    param([string]$RepoRoot)
    $found = New-Object System.Collections.ArrayList
    foreach ($item in @(Get-ChildItem -LiteralPath (Join-Path $RepoRoot 'dev') -Filter '*.ps1' -File)) {
        [void]$found.Add($item)
    }
    $cliDir = Join-Path $RepoRoot 'src\cli'
    if (Test-Path -LiteralPath $cliDir) {
        foreach ($item in @(Get-ChildItem -LiteralPath $cliDir -Filter '*.ps1' -File)) {
            [void]$found.Add($item)
        }
    }
    $guiDir = Join-Path $RepoRoot 'src\gui'
    if (Test-Path -LiteralPath $guiDir) {
        foreach ($item in @(Get-ChildItem -LiteralPath $guiDir -Filter '*.ps1' -File)) {
            [void]$found.Add($item)
        }
    }
    foreach ($item in @(Get-ChildItem -LiteralPath (Join-Path $RepoRoot 'src\Zapret') -File | Where-Object {
        $_.Extension -in @('.ps1', '.psm1', '.psd1')
    })) {
        [void]$found.Add($item)
    }
    foreach ($item in @(Get-ChildItem -LiteralPath (Join-Path $RepoRoot 'strategies') -Filter '*.ps1' -File)) {
        [void]$found.Add($item)
    }
    return @($found)
}

function Get-ZapretXamlFiles {
    param([string]$RepoRoot)
    $guiDir = Join-Path $RepoRoot 'src\gui'
    if (-not (Test-Path -LiteralPath $guiDir)) {
        return @()
    }
    return @(Get-ChildItem -LiteralPath $guiDir -Filter '*.xaml' -File)
}

function New-ZapretLintIssue {
    param(
        [string]$ScriptName,
        [int]$Line,
        [string]$RuleName,
        [string]$Message
    )
    return New-Object PSObject -Property @{
        ScriptName = $ScriptName
        Line       = $Line
        Severity   = 'Error'
        RuleName   = $RuleName
        Message    = $Message
    }
}

function Test-ZapretUtf8Bom {
    param([string]$Path)
    $bytes = [System.IO.File]::ReadAllBytes($Path)
    if ($bytes.Length -lt 3) {
        return $false
    }
    return ($bytes[0] -eq 239 -and $bytes[1] -eq 187 -and $bytes[2] -eq 191)
}

function Test-ZapretFileHasCyrillic {
    param([string]$Path)
    $enc = New-Object System.Text.UTF8Encoding $false
    $text = [System.IO.File]::ReadAllText($Path, $enc)
    return [bool]($text -match '\p{IsCyrillic}')
}

function Test-ZapretFileHasNonAscii {
    param([string]$Path)
    $enc = New-Object System.Text.UTF8Encoding $false
    $text = [System.IO.File]::ReadAllText($Path, $enc)
    foreach ($ch in $text.ToCharArray()) {
        if ([int]$ch -gt 127) {
            return $true
        }
    }
    return $false
}

function Get-ZapretRelPath {
    param([string]$FullName)
    return $FullName.Substring($root.Length).TrimStart('\', '/')
}

function Test-ZapretProjectRules {
    param([System.IO.FileInfo[]]$Files)

    $issues = New-Object System.Collections.ArrayList
    # Split the type name so this file does not match its own rule.
    $listObject = 'New-Object\s+(?:System\.)?Collections\.Generic\.List\[' + '(?:object|PSObject)\]'
    $reListObject = New-Object System.Text.RegularExpressions.Regex (
        $listObject,
        [System.Text.RegularExpressions.RegexOptions]::IgnoreCase
    )
    $reWindowShowDialog = New-Object System.Text.RegularExpressions.Regex (
        '\$window\.ShowDialog\s*\(',
        [System.Text.RegularExpressions.RegexOptions]::IgnoreCase
    )
    $reWindowHide = New-Object System.Text.RegularExpressions.Regex (
        '\$window\.Hide\s*\(',
        [System.Text.RegularExpressions.RegexOptions]::IgnoreCase
    )
    $reWindowVisibility = New-Object System.Text.RegularExpressions.Regex (
        '\$window\.Visibility\s*=\s*(''?Hidden''?|''?Collapsed''?|"Hidden"|"Collapsed"|\[System\.Windows\.Visibility\]::(Hidden|Collapsed))',
        [System.Text.RegularExpressions.RegexOptions]::IgnoreCase
    )
    $reWindowEnabled = New-Object System.Text.RegularExpressions.Regex (
        '\$window\.IsEnabled\s*=\s*\$false',
        [System.Text.RegularExpressions.RegexOptions]::IgnoreCase
    )
    $reWindowNoTaskbar = New-Object System.Text.RegularExpressions.Regex (
        '\$window\.ShowInTaskbar\s*=\s*\$false',
        [System.Text.RegularExpressions.RegexOptions]::IgnoreCase
    )
    $reWinForms = New-Object System.Text.RegularExpressions.Regex (
        'System\.Windows\.Forms',
        [System.Text.RegularExpressions.RegexOptions]::IgnoreCase
    )
    $reNotifyIcon = New-Object System.Text.RegularExpressions.Regex (
        'NotifyIcon',
        [System.Text.RegularExpressions.RegexOptions]::IgnoreCase
    )
    $reDoEvents = New-Object System.Text.RegularExpressions.Regex (
        'Application\]::DoEvents|Forms\.Application',
        [System.Text.RegularExpressions.RegexOptions]::IgnoreCase
    )
    $reFormsTimer = New-Object System.Text.RegularExpressions.Regex (
        'Windows\.Forms\.Timer',
        [System.Text.RegularExpressions.RegexOptions]::IgnoreCase
    )
    $reProcessEvents = New-Object System.Text.RegularExpressions.Regex (
        'Add_Exited|OutputDataReceived|ErrorDataReceived',
        [System.Text.RegularExpressions.RegexOptions]::IgnoreCase
    )
    $reNullCoalesce = New-Object System.Text.RegularExpressions.Regex '\?\?'
    $reParallel = New-Object System.Text.RegularExpressions.Regex (
        '\s-Parallel\b',
        [System.Text.RegularExpressions.RegexOptions]::IgnoreCase
    )
    $reAndAnd = New-Object System.Text.RegularExpressions.Regex '\s&&\s'

    foreach ($file in $Files) {
        $rel = Get-ZapretRelPath -FullName $file.FullName
        $norm = $rel.Replace('/', '\').ToLowerInvariant()
        $isGui = ($norm -eq 'src\gui\gui.ps1') -or $norm.EndsWith('\src\gui\gui.ps1')
        $inGuiDir = $norm.StartsWith('src\gui\') -or $norm.Contains('\src\gui\')
        $needsBom = ($file.Name -eq 'Ui.ps1')
        if (-not $needsBom) {
            $needsBom = Test-ZapretFileHasCyrillic -Path $file.FullName
        }
        if ($needsBom -and -not (Test-ZapretUtf8Bom -Path $file.FullName)) {
            [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line 1 -RuleName 'ZapretUtf8Bom' -Message 'Cyrillic or Ui.ps1 must be UTF-8 with BOM. PowerShell 5.1 breaks literals without a BOM.'))
        }

        $rawAll = [System.IO.File]::ReadAllText($file.FullName)
        if ($isGui) {
            $hasReader = $rawAll.IndexOf('XamlReader') -ge 0
            $hasXmlCast = $rawAll.IndexOf('[xml]') -ge 0
            $hasNodeReader = $rawAll.IndexOf('XmlNodeReader') -ge 0
            if ($hasReader -and ($hasXmlCast -or $hasNodeReader)) {
                [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line 1 -RuleName 'ZapretAvoidXamlXmlCast' -Message 'Load XAML with XamlReader.Load and a FileStream. Do not use [xml] or XmlNodeReader.'))
            }
        }

        $n = 0
        foreach ($raw in [System.IO.File]::ReadAllLines($file.FullName)) {
            $n++
            $line = [string]$raw
            $trim = $line.Trim()
            if ($trim.StartsWith('#')) {
                continue
            }
            if ($reListObject.IsMatch($line)) {
                [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line $n -RuleName 'ZapretAvoidNewObjectListObject' -Message 'PS 5.1 parses New-Object Generic.List[object] as an open List. Use ArrayList or a quoted type name.'))
            }
            if ($isGui -and $reWindowShowDialog.IsMatch($line)) {
                [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line $n -RuleName 'ZapretAvoidMainWindowShowDialog' -Message 'The main window must use Application.Run, not $window.ShowDialog. A child dialog disables the owner and ends the UI.'))
            }
            if ($isGui -and $reWindowHide.IsMatch($line)) {
                [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line $n -RuleName 'ZapretAvoidMainWindowHide' -Message 'Do not call $window.Hide() on the main window. There is no tray; hide loses the window.'))
            }
            if ($isGui -and $reWindowVisibility.IsMatch($line)) {
                [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line $n -RuleName 'ZapretAvoidMainWindowHide' -Message 'Do not set $window.Visibility to Hidden or Collapsed. There is no tray; hide loses the window.'))
            }
            if ($isGui -and $reWindowEnabled.IsMatch($line)) {
                [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line $n -RuleName 'ZapretAvoidMainWindowEnabledFalse' -Message 'Do not set $window.IsEnabled = $false. That ends the main window message loop.'))
            }
            if ($isGui -and $reWindowNoTaskbar.IsMatch($line)) {
                [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line $n -RuleName 'ZapretAvoidMainWindowNoTaskbar' -Message 'The main window must stay on the taskbar. There is no tray.'))
            }
            if ($inGuiDir -and $reWinForms.IsMatch($line)) {
                [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line $n -RuleName 'ZapretAvoidWinFormsInGui' -Message 'The GUI is WPF. Do not use System.Windows.Forms in src/gui/.'))
            }
            if ($inGuiDir -and $reNotifyIcon.IsMatch($line)) {
                [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line $n -RuleName 'ZapretAvoidNotifyIcon' -Message 'Do not use NotifyIcon. Tray support is not in this release.'))
            }
            if ($inGuiDir -and $reDoEvents.IsMatch($line)) {
                [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line $n -RuleName 'ZapretAvoidFormsDoEvents' -Message 'Do not call Forms Application.DoEvents. Use Dispatcher.Invoke with Background priority.'))
            }
            if ($inGuiDir -and $reFormsTimer.IsMatch($line)) {
                [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line $n -RuleName 'ZapretAvoidFormsTimer' -Message 'Do not use Windows.Forms.Timer. Use DispatcherTimer.'))
            }
            if ($isGui -and $reProcessEvents.IsMatch($line)) {
                [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line $n -RuleName 'ZapretAvoidProcessEventsInGui' -Message 'Do not attach Process events in GUI scriptblocks. Those callbacks run on a thread-pool thread and break STA.'))
            }
            if ($reNullCoalesce.IsMatch($line)) {
                [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line $n -RuleName 'ZapretAvoidPwshNullCoalesce' -Message 'The null-coalescing operator is PowerShell 7. Target is Windows PowerShell 5.1.'))
            }
            if ($reParallel.IsMatch($line)) {
                [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line $n -RuleName 'ZapretAvoidPwshParallel' -Message '-Parallel is PowerShell 7. Target is Windows PowerShell 5.1.'))
            }
            if ($reAndAnd.IsMatch($line)) {
                [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line $n -RuleName 'ZapretAvoidPwshAndAnd' -Message '&& is PowerShell 7. Use a separate statement or -and in 5.1.'))
            }
        }
    }
    return @($issues)
}

function Test-ZapretXamlRules {
    param([System.IO.FileInfo[]]$Files)

    $issues = New-Object System.Collections.ArrayList
    foreach ($file in $Files) {
        $rel = Get-ZapretRelPath -FullName $file.FullName
        $text = [System.IO.File]::ReadAllText($file.FullName)
        $needsBom = Test-ZapretFileHasCyrillic -Path $file.FullName
        if ($needsBom -and -not (Test-ZapretUtf8Bom -Path $file.FullName)) {
            [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line 1 -RuleName 'ZapretUtf8Bom' -Message 'Cyrillic in XAML must be UTF-8 with BOM.'))
        }
        if ((Test-ZapretFileHasNonAscii -Path $file.FullName) -and -not (Test-ZapretUtf8Bom -Path $file.FullName)) {
            [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line 1 -RuleName 'ZapretXamlAsciiOrBom' -Message 'XAML with non-ASCII characters must be UTF-8 with BOM. Prefer ASCII and Get-ZapretUiString.'))
        }
        $usesX = [regex]::IsMatch($text, '(?<!xmlns:)x:[A-Za-z]')
        if ($usesX -and ($text.IndexOf('xmlns:x=') -lt 0)) {
            [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line 1 -RuleName 'ZapretXamlPrefixX' -Message 'XAML uses the x: prefix but xmlns:x is missing.'))
        }
        if ($text.IndexOf('x:Class=') -ge 0) {
            [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line 1 -RuleName 'ZapretXamlNoClass' -Message 'Do not set x:Class in XAML. Load with XamlReader and attach handlers in gui.ps1.'))
        }

        $fs = $null
        try {
            $fs = New-Object System.IO.FileStream($file.FullName, [System.IO.FileMode]::Open, [System.IO.FileAccess]::Read)
            $obj = [System.Windows.Markup.XamlReader]::Load($fs)
            if ($obj -is [System.Windows.Window]) {
                # Do not Show. Load is the parser check.
            }
        } catch {
            $msg = $_.Exception.Message
            if ($_.Exception.InnerException) {
                $msg = $_.Exception.InnerException.Message
            }
            [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line 1 -RuleName 'ZapretXamlLoad' -Message $msg))
        } finally {
            if ($fs) {
                $fs.Dispose()
            }
        }
    }
    return @($issues)
}

function Test-ZapretBatRequiresSta {
    param([string]$RepoRoot)
    $issues = New-Object System.Collections.ArrayList
    $batPath = Join-Path $RepoRoot 'zapret.bat'
    if (-not (Test-Path -LiteralPath $batPath)) {
        [void]$issues.Add((New-ZapretLintIssue -ScriptName 'zapret.bat' -Line 1 -RuleName 'ZapretBatRequiresSta' -Message 'zapret.bat is missing.'))
        return @($issues)
    }
    $n = 0
    $sawLaunch = $false
    foreach ($raw in [System.IO.File]::ReadAllLines($batPath)) {
        $n++
        $line = [string]$raw
        $trim = $line.Trim()
        if ($trim.StartsWith('::') -or $trim.StartsWith('REM ') -or $trim.StartsWith('rem ')) {
            continue
        }
        if ($line -match 'gui-boot\.ps1') {
            $sawLaunch = $true
            if ($line -notmatch '(?i)-STA') {
                [void]$issues.Add((New-ZapretLintIssue -ScriptName 'zapret.bat' -Line $n -RuleName 'ZapretBatRequiresSta' -Message 'The GUI launch command must keep -STA.'))
            }
        }
    }
    if (-not $sawLaunch) {
        [void]$issues.Add((New-ZapretLintIssue -ScriptName 'zapret.bat' -Line 1 -RuleName 'ZapretBatRequiresSta' -Message 'zapret.bat must launch gui-boot.ps1 with powershell.exe -STA.'))
    }
    return @($issues)
}

$files = @(Get-ZapretLintFiles -RepoRoot $root)
if ($files.Count -lt 1) {
    throw 'No PowerShell files found to lint.'
}

Add-Type -AssemblyName PresentationFramework -ErrorAction Stop

$issues = New-Object System.Collections.ArrayList
foreach ($item in @(Test-ZapretProjectRules -Files $files)) {
    [void]$issues.Add($item)
}

$xamlFiles = @(Get-ZapretXamlFiles -RepoRoot $root)
foreach ($item in @(Test-ZapretXamlRules -Files $xamlFiles)) {
    [void]$issues.Add($item)
}
foreach ($item in @(Test-ZapretBatRequiresSta -RepoRoot $root)) {
    [void]$issues.Add($item)
}

function Get-ZapretBlinterPath {
    $cmd = Get-Command -Name 'blinter' -ErrorAction SilentlyContinue
    if ($cmd) {
        return [string]$cmd.Source
    }
    $cmd = Get-Command -Name 'blinter.exe' -ErrorAction SilentlyContinue
    if ($cmd) {
        return [string]$cmd.Source
    }
    return $null
}

function Invoke-ZapretBlinter {
    param(
        [string]$RepoRoot,
        [string]$BlinterPath
    )
    $outFile = Join-Path $env:TEMP ('zapret-blinter-{0}.json' -f [guid]::NewGuid().ToString('N'))
    $argList = @(
        '--no-recursive'
        '--format'
        'json'
        '--output'
        $outFile
        '.'
    )
    $oldEap = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        Push-Location -LiteralPath $RepoRoot
        & $BlinterPath @argList
    } finally {
        Pop-Location
        $ErrorActionPreference = $oldEap
    }
    if (-not (Test-Path -LiteralPath $outFile)) {
        throw 'Blinter produced no JSON report.'
    }
    try {
        $raw = Get-Content -LiteralPath $outFile -Raw -Encoding UTF8
        $report = $raw | ConvertFrom-Json
    } finally {
        Remove-Item -LiteralPath $outFile -Force -ErrorAction SilentlyContinue
    }
    $found = New-Object System.Collections.ArrayList
    foreach ($item in @($report.issues)) {
        $sev = 'Warning'
        if ($item.severity -eq 'Error' -or $item.severity -eq 'Security') {
            $sev = 'Error'
        }
        [void]$found.Add((New-Object PSObject -Property @{
            ScriptName = [string]$item.file
            Line       = [int]$item.line
            Severity   = $sev
            RuleName   = [string]$item.code
            Message    = [string]$item.name
        }))
    }
    return @($found)
}

$pssaRan = $false
if (Get-Module -ListAvailable -Name PSScriptAnalyzer) {
    if (-not (Test-Path -LiteralPath $settings)) {
        throw 'PSScriptAnalyzerSettings.psd1 is not found.'
    }
    Import-Module PSScriptAnalyzer -ErrorAction Stop
    foreach ($file in $files) {
        $found = @(Invoke-ScriptAnalyzer -Path $file.FullName -Settings $settings)
        foreach ($item in $found) {
            [void]$issues.Add($item)
        }
    }
    $pssaRan = $true
} else {
    Write-Host 'WARN: PSScriptAnalyzer is not installed. Project rules still ran. CI installs the module.'
}

$blinterRan = $false
$blinterPath = Get-ZapretBlinterPath
if ($blinterPath) {
    foreach ($item in @(Invoke-ZapretBlinter -RepoRoot $root -BlinterPath $blinterPath)) {
        [void]$issues.Add($item)
    }
    $blinterRan = $true
} else {
    Write-Host 'WARN: Blinter is not installed. Project rules still ran. Install: pipx install Blinter'
}

$fileCount = $files.Count + $xamlFiles.Count
if ($issues.Count -eq 0) {
    $extra = ''
    if ($pssaRan) {
        $extra = $extra + ', PSScriptAnalyzer clean'
    }
    if ($blinterRan) {
        $extra = $extra + ', Blinter clean'
    }
    Write-Host ("OK: {0} files, no Zapret lint issues{1}." -f $fileCount, $extra)
    exit 0
}

$issues |
    Sort-Object ScriptName, Line |
    Format-Table -AutoSize ScriptName, Line, Severity, RuleName, Message
Write-Host ("FAIL: {0} issue(s) in {1} files." -f $issues.Count, $fileCount)
exit 1
