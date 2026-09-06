# Console entry for strategy tests. Start: cli.bat tests. The runner is Invoke-ZapmanStrategyTests.
param(
    [ValidateSet('standard', 'dpi')]
    [string]$TestType,
    [string]$Strategies,
    [switch]$NoPause
)

Import-Module -Force -DisableNameChecking (Join-Path (Split-Path -Parent $PSScriptRoot) 'Zapman\Zapman.psd1')
[void](Initialize-ZapmanUiLanguage)

$servicePath = Join-Path $PSScriptRoot 'service.ps1'
if (-not (Test-Path -LiteralPath $servicePath)) {
    throw ("File not found: {0}" -f $servicePath)
}
. $servicePath

if (-not (Test-IsAdministrator)) {
    $pass = New-Object System.Collections.ArrayList
    if (-not [string]::IsNullOrWhiteSpace($TestType)) {
        [void]$pass.Add('-TestType')
        [void]$pass.Add($TestType)
    }
    if (-not [string]::IsNullOrWhiteSpace($Strategies)) {
        [void]$pass.Add('-Strategies')
        [void]$pass.Add($Strategies)
    }
    if ($NoPause) {
        [void]$pass.Add('-NoPause')
    }
    exit (Start-ZapmanElevatedPowerShellFile -FilePath $PSCommandPath -ArgumentList @($pass))
}

$askType = [string]::IsNullOrWhiteSpace($TestType)
$askNames = [string]::IsNullOrWhiteSpace($Strategies)
$code = Invoke-ZapmanCliTestRun -TestType $TestType -Strategies $Strategies -AskType:$askType -AskNames:$askNames
if ($null -eq $code) {
    $code = 1
}

$doPause = -not $NoPause
if ($doPause) {
    try {
        if ([Console]::IsInputRedirected) {
            $doPause = $false
        }
    } catch {
        $doPause = $false
    }
}
if ($doPause) {
    try {
        Write-Host "Press any key to close..." -ForegroundColor Yellow
        [void][System.Console]::ReadKey($true)
    } catch {
        $null = $_.Exception
    }
}

exit ([int]$code)
