# Zapret Manager module. Load by path. Do not install this module in PSModulePath.
# Engine helpers live in src/Zapret and are dotsourced here.

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Continue'

. (Join-Path $PSScriptRoot 'Core.ps1')
. (Join-Path $PSScriptRoot 'Ui.ps1')
Import-Module -Force -DisableNameChecking (Join-Path (Split-Path -Parent $PSScriptRoot) 'ZapretSpec\ZapretSpec.psd1')
. (Join-Path (Split-Path -Parent $PSScriptRoot) 'Zapret\Bypass.ps1')
. (Join-Path $PSScriptRoot 'Tools.ps1')
. (Join-Path $PSScriptRoot 'TrayWatch.ps1')
. (Join-Path $PSScriptRoot 'Tests.ps1')

function Get-ZapmanSpecVersion {
    return (Get-ZapretSpecVersion)
}

function Invoke-ZapmanStrategyTests {
    param(
        [string]$TestType,
        [string[]]$Names,
        [scriptblock]$OnLine,
        [scriptblock]$ShouldStop,
        [scriptblock]$OnWait,
        [scriptblock]$OnStrategy,
        [scriptblock]$OnSummary,
        [switch]$AskType,
        [switch]$AskNames
    )
    Invoke-ZapmanStrategyTestsCore @PSBoundParameters
}

Export-ModuleMember -Function @(
    'Get-ZapretLayout'
    'Get-ZapmanConfig'
    'Get-ZapmanConfigError'
    'Get-ZapmanLocalVersion'
    'Get-ZapmanSpecVersion'
    'ConvertTo-ZapmanVersion'
    'Get-ZapmanVersionFromRemoteBody'
    'Initialize-ZapmanUserLists'
    'Test-ZapmanHostReady'
    'Test-ZapretBinVersions'
    'Show-ZapmanHostReadyReport'
    'Test-ZapmanTcpTimestampsEnabled'
    'Enable-ZapmanTcpTimestamps'
    'Enable-ZapmanTls12'
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
    'Suspend-ZapretServiceForTests'
    'Restore-ZapretServiceAfterTests'
    'Start-ZapretSelectedStrategy'
    'Start-ZapretServiceIfInstalled'
    'Install-ZapretService'
    'Remove-ZapretServices'
    'Test-ZapmanAutoUpdateEnabled'
    'Set-ZapmanAutoUpdateEnabled'
    'Test-ZapmanTrayWatchEnabled'
    'Set-ZapmanTrayWatchEnabled'
    'Sync-ZapmanTrayWatch'
    'Get-ZapmanTrayMutexName'
    'Get-ZapmanGuiMutexName'
    'Get-ZapmanPowershellExePath'
    'Restart-ZapmanTrayWatchProcess'
    'Open-ZapmanGui'
    'Get-ZapretVersionCheckUrl'
    'Get-ZapretRemoteVersion'
    'Get-ZapretReleasePageUrl'
    'Get-ZapretIpsetListUrl'
    'Get-ZapretHostsSourceUrl'
    'Get-ZapmanWebUserAgent'
    'Invoke-ZapmanWebDownload'
    'Update-ZapretIpsetList'
    'Get-ZapretHostsUpdateInfo'
    'Copy-ZapretHostsTemplate'
    'Open-ZapretSystemHosts'
    'Get-ZapretFakeCatalog'
    'Get-ZapretFakeCurrentText'
    'Set-ZapretActiveFake'
    'Invoke-ZapmanStrategyTests'
    'Get-ZapmanStatusLines'
    'Get-ZapmanLastError'
    'Set-ZapmanLastError'
    'Get-ZapmanExceptionText'
    'Clear-ZapmanDiscordCache'
    'Remove-ZapretNamedServices'
    'Get-ZapmanDiagnosticReport'
    'Invoke-ZapmanDiagnosticAction'
    'Invoke-ZapmanNetworkReset'
    'Initialize-ZapmanUiLanguage'
    'Get-ZapmanUiLanguage'
    'Set-ZapmanUiLanguage'
    'Get-ZapmanUiString'
    'Enable-ZapmanConsoleUtf8'
)
