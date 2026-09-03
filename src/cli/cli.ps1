# CLI hub: after cli.bat checks powershell.exe, this process stays in PowerShell.
# Start: cli.bat   or   cli.bat service|tests|env

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Continue'

try {
    $Host.UI.RawUI.WindowTitle = 'Zapret Manager CLI'
} catch {
    Write-Verbose $_.Exception.Message
}

if ($PSVersionTable.PSVersion.Major -lt 3) {
    Write-Host 'ERROR: Windows PowerShell is too old. This program needs 3.0 or newer. Target: 5.1.'
    Write-Host 'On Windows 7 install WMF 5.1 and .NET Framework 4.5 or newer.'
    exit 1
}

Import-Module -DisableNameChecking (Join-Path (Split-Path -Parent $PSScriptRoot) 'Zapman\Zapman.psd1')

Enable-ZapmanConsoleUtf8
[void](Initialize-ZapmanUiLanguage)

$servicePath = Join-Path $PSScriptRoot 'service.ps1'
$testsPath = Join-Path $PSScriptRoot 'test-zapret.ps1'

if (-not (Test-Path -LiteralPath $servicePath)) {
    throw ("File not found: {0}" -f $servicePath)
}
. $servicePath

function Invoke-ZapmanCliTests {
    param([string[]]$Extra = @())
    if (-not (Test-Path -LiteralPath $testsPath)) {
        throw ("File not found: {0}" -f $testsPath)
    }
    $extraList = @($Extra)
    if ($extraList.Count -gt 0) {
        & $testsPath @extraList
        return [int]$LASTEXITCODE
    }
    & $testsPath
    return [int]$LASTEXITCODE
}

function Show-ZapmanCliUsage {
    Write-Host (Get-ZapmanUiString -Key 'CliUsage')
}

function Show-ZapmanCliMenu {
    while ($true) {
        Write-Host ''
        Write-Host (Get-ZapmanUiString -Key 'CliTitle')
        Write-Host ("  1. {0}" -f (Get-ZapmanUiString -Key 'CliService'))
        Write-Host ("  2. {0}" -f (Get-ZapmanUiString -Key 'CliTests'))
        Write-Host ("  3. {0}" -f (Get-ZapmanUiString -Key 'CliEnv'))
        Write-Host ("  0. {0}" -f (Get-ZapmanUiString -Key 'MenuExit'))
        $choice = Read-Host
        switch ($choice) {
            '1' { [void](Start-ZapmanServiceConsole) }
            '2' { [void](Invoke-ZapmanCliTests) }
            '3' { [void](Show-ZapmanHostReadyReport) }
            '0' { return 0 }
            default { Write-Host (Get-ZapmanUiString -Key 'InvalidChoice') }
        }
    }
}

$tokens = @($args)
$command = ''
$extra = @()
if ($tokens.Count -gt 0) {
    $command = ([string]$tokens[0]).Trim().ToLowerInvariant()
    if ($tokens.Count -gt 1) {
        $extra = @($tokens[1..($tokens.Count - 1)])
    }
}

if ($command -eq 'help' -or $command -eq '-h' -or $command -eq '--help' -or $command -eq '/?') {
    Show-ZapmanCliUsage
    exit 0
}

if ($command -eq 'env' -or $command -eq 'check-env') {
    exit (Show-ZapmanHostReadyReport)
}

if ((Show-ZapmanHostReadyReport) -ne 0) {
    exit 1
}

if ([string]::IsNullOrWhiteSpace($command)) {
    [void](Show-ZapmanCliMenu)
    return
}

if ($command -eq 'service') {
    exit (Start-ZapmanServiceConsole)
}

if ($command -eq 'tests' -or $command -eq 'test') {
    exit (Invoke-ZapmanCliTests -Extra $extra)
}

Write-Host (Get-ZapmanUiString -Key 'CliUnknown' -FormatArgs @($command))
Show-ZapmanCliUsage
exit 1
