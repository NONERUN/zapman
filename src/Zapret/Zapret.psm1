# Zapret local module. Load by path. Do not install this module in PSModulePath.

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Continue'

. (Join-Path $PSScriptRoot 'Core.ps1')
. (Join-Path $PSScriptRoot 'Ui.ps1')
. (Join-Path $PSScriptRoot 'Bypass.ps1')
. (Join-Path $PSScriptRoot 'Tools.ps1')
# Tests.ps1 loads on the first test run. Do not parse it at GUI start.
$script:ZapretTestsLoaded = $false

function Invoke-ZapretStrategyTests {
    param(
        [string]$TestType,
        [string[]]$Names,
        [scriptblock]$OnLine,
        [scriptblock]$ShouldStop,
        [switch]$AskType,
        [switch]$AskNames
    )
    if (-not $script:ZapretTestsLoaded) {
        . (Join-Path $PSScriptRoot 'Tests.ps1')
        $script:ZapretTestsLoaded = $true
    }
    Invoke-ZapretStrategyTestsCore @PSBoundParameters
}

Export-ModuleMember -Function @(
    'Get-ZapretLayout'
    'Get-ZapretConfig'
    'Get-ZapretLocalVersion'
    'Initialize-ZapretUserLists'
    'Test-ZapretHostReady'
    'Test-ZapretBinVersions'
    'Show-ZapretHostReadyReport'
    'Test-ZapretTcpTimestampsEnabled'
    'Enable-ZapretTcpTimestamps'
    'Enable-ZapretTls12'
    'Get-ZapretEngine'
    'Set-ZapretEngine'
    'Get-ZapretEngineExeName'
    'Test-ZapretEngineFiles'
    'Test-ZapretStrategySupportsEngine'
    'Get-ZapretGameFilter'
    'Set-ZapretGameFilterMode'
    'Test-ZapretServiceRunning'
    'Test-ZapretNamedService'
    'Get-ZapretService'
    'Invoke-ZapretStrategyPrep'
    'Get-ZapretStrategyFiles'
    'Start-ZapretWinws'
    'Start-ZapretStrategyFile'
    'ConvertTo-ZapretServiceImagePath'
    'Test-ZapretBypassRunning'
    'Get-ZapretStatusBypassName'
    'Get-ZapretInstalledStrategyName'
    'Get-ZapretRunningStrategyName'
    'Get-ZapretIpsetStatus'
    'Set-ZapretIpsetMode'
    'Stop-ZapretWinwsProcess'
    'Wait-ZapretServiceStopped'
    'Wait-ZapretServiceGone'
    'Stop-ZapretBypass'
    'Wait-ZapretWinws'
    'Get-ZapretWinwsCommandLine'
    'Remove-ZapretServiceRecord'
    'Start-ZapretSelectedStrategy'
    'Start-ZapretServiceIfInstalled'
    'Install-ZapretService'
    'Remove-ZapretServices'
    'Test-ZapretAutoUpdateEnabled'
    'Set-ZapretAutoUpdateEnabled'
    'Get-ZapretVersionCheckUrl'
    'Get-ZapretRemoteVersion'
    'Get-ZapretReleasePageUrl'
    'Get-ZapretIpsetListUrl'
    'Get-ZapretHostsSourceUrl'
    'Invoke-ZapretWebDownload'
    'Update-ZapretIpsetList'
    'Get-ZapretHostsUpdateInfo'
    'Open-ZapretHostsUpdate'
    'Get-ZapretFakeCatalog'
    'Get-ZapretFakeCurrentText'
    'Set-ZapretActiveFake'
    'Start-ZapretConfigTests'
    'Invoke-ZapretStrategyTests'
    'Get-ZapretStatusLines'
    'Clear-ZapretDiscordCache'
    'Remove-ZapretNamedServices'
    'Get-ZapretDiagnosticReport'
    'Invoke-ZapretDiagnosticAction'
    'Initialize-ZapretUiLanguage'
    'Get-ZapretUiLanguage'
    'Set-ZapretUiLanguage'
    'Get-ZapretUiString'
    'Enable-ZapretConsoleUtf8'
)
