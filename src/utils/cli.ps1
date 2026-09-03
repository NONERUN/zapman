# CLI hub: after cli.bat checks powershell.exe, this process stays in PowerShell.
# Start: cli.bat   or   cli.bat service|tests|env

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Continue'

try {
    $Host.UI.RawUI.WindowTitle = 'Zapret CLI'
} catch {
    Write-Verbose $_.Exception.Message
}

if ($PSVersionTable.PSVersion.Major -lt 3) {
    Write-Host 'ERROR: Windows PowerShell is too old. This program needs 3.0 or newer. Target: 5.1.'
    Write-Host 'On Windows 7 install WMF 5.1 and .NET Framework 4.5 or newer.'
    exit 1
}

Import-Module -DisableNameChecking (Join-Path (Split-Path -Parent $PSScriptRoot) 'Zapret\Zapret.psd1')

Enable-ZapretConsoleUtf8
[void](Initialize-ZapretUiLanguage)

$servicePath = Join-Path $PSScriptRoot 'service.ps1'
$testsPath = Join-Path $PSScriptRoot 'test zapret.ps1'

if (-not (Test-Path -LiteralPath $servicePath)) {
    throw ("File not found: {0}" -f $servicePath)
}
. $servicePath

function Invoke-ZapretCliTests {
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

function Show-ZapretCliUsage {
    Write-Host (Get-ZapretUiString -Key 'CliUsage')
}

function Show-ZapretCliMenu {
    while ($true) {
        Write-Host ''
        Write-Host (Get-ZapretUiString -Key 'CliTitle')
        Write-Host ("  1. {0}" -f (Get-ZapretUiString -Key 'CliService'))
        Write-Host ("  2. {0}" -f (Get-ZapretUiString -Key 'CliTests'))
        Write-Host ("  3. {0}" -f (Get-ZapretUiString -Key 'CliEnv'))
        Write-Host ("  0. {0}" -f (Get-ZapretUiString -Key 'MenuExit'))
        $choice = Read-Host
        switch ($choice) {
            '1' { [void](Start-ZapretServiceConsole) }
            '2' { [void](Invoke-ZapretCliTests) }
            '3' { [void](Show-ZapretHostReadyReport) }
            '0' { return 0 }
            default { Write-Host (Get-ZapretUiString -Key 'InvalidChoice') }
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
    Show-ZapretCliUsage
    exit 0
}

if ($command -eq 'env' -or $command -eq 'check-env') {
    exit (Show-ZapretHostReadyReport)
}

if ((Show-ZapretHostReadyReport) -ne 0) {
    exit 1
}

if ([string]::IsNullOrWhiteSpace($command)) {
    [void](Show-ZapretCliMenu)
    return
}

if ($command -eq 'service') {
    exit (Start-ZapretServiceConsole)
}

if ($command -eq 'tests' -or $command -eq 'test') {
    exit (Invoke-ZapretCliTests -Extra $extra)
}

Write-Host (Get-ZapretUiString -Key 'CliUnknown' -FormatArgs @($command))
Show-ZapretCliUsage
exit 1
