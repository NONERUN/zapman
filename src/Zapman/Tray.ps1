# Tray watcher host. Start with powershell.exe -STA. Do not dotsource this file in Zapman.psm1.

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Continue'

# Hide the console before the module load. A visible host flashes until Import-Module ends.
try {
    $hide = @'
using System;
using System.Runtime.InteropServices;
public static class ZapmanTrayConsole {
    [DllImport("kernel32.dll")] public static extern IntPtr GetConsoleWindow();
    [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);
}
'@
    if (-not ('ZapmanTrayConsole' -as [type])) {
        Add-Type -TypeDefinition $hide -ErrorAction Stop
    }
    $hwnd = [ZapmanTrayConsole]::GetConsoleWindow()
    if ($hwnd -ne [IntPtr]::Zero) {
        [void][ZapmanTrayConsole]::ShowWindow($hwnd, 0)
    }
} catch {
    $null = $_.Exception
}

Add-Type -AssemblyName System.Windows.Forms -ErrorAction Stop
Add-Type -AssemblyName System.Drawing -ErrorAction Stop

Import-Module -DisableNameChecking (Join-Path $PSScriptRoot 'Zapman.psd1')
[void](Initialize-ZapmanUiLanguage)

$script:trayMutex = $null
$script:trayMutexOwned = $false
$script:trayIcon = $null
$script:trayNotify = $null
$script:trayMenu = $null
$script:trayItemStatus = $null
$script:trayItemStart = $null
$script:trayItemStop = $null
$script:trayCtx = $null
$script:trayTipField = $null
$script:trayTipMethod = $null
$script:trayTipUseReflection = $null

function Get-ZapmanTrayStatusParts {
    $running = Test-ZapretBypassRunning
    $bypassName = Get-ZapretStatusBypassName
    $svc = Get-ZapretService
    $name = Get-ZapretRunningStrategyName
    if ([string]::IsNullOrWhiteSpace($name)) {
        $name = Get-ZapretInstalledStrategyName
    }
    $parts = New-Object System.Collections.ArrayList
    if ($running) {
        [void]$parts.Add((Get-ZapmanUiString -Key 'StatusBypassOn' -FormatArgs @($bypassName)))
    } else {
        [void]$parts.Add((Get-ZapmanUiString -Key 'StatusBypassOff'))
    }
    if ($svc) {
        [void]$parts.Add((Get-ZapmanUiString -Key 'StatusServiceOn' -FormatArgs @($svc.Status)))
    } else {
        [void]$parts.Add((Get-ZapmanUiString -Key 'StatusServiceOff'))
    }
    if (-not [string]::IsNullOrWhiteSpace($name)) {
        [void]$parts.Add((Get-ZapmanUiString -Key 'StatusStrategy' -FormatArgs @($name)))
    } else {
        [void]$parts.Add((Get-ZapmanUiString -Key 'StatusStrategyNone'))
    }
    return $parts
}

function Get-ZapmanTrayNotifyText {
    $text = (@(Get-ZapmanTrayStatusParts) -join "`n")
    if ($text.Length -gt 127) {
        return ($text.Substring(0, 124) + '...')
    }
    return $text
}

function Set-ZapmanNotifyIconTip {
    param(
        [System.Windows.Forms.NotifyIcon]$Notify,
        [string]$Text
    )
    if ($script:trayTipUseReflection -ne $false) {
        try {
            if ($null -eq $script:trayTipField) {
                $type = $Notify.GetType()
                $flags = [System.Reflection.BindingFlags]::Instance -bor [System.Reflection.BindingFlags]::NonPublic
                $script:trayTipField = $type.GetField('text', $flags)
                $script:trayTipMethod = $type.GetMethod('UpdateIcon', $flags)
            }
            if ($script:trayTipField -and $script:trayTipMethod) {
                $script:trayTipField.SetValue($Notify, $Text)
                [void]$script:trayTipMethod.Invoke($Notify, @($true))
                $script:trayTipUseReflection = $true
                return
            }
        } catch {
            $script:trayTipUseReflection = $false
        }
        $script:trayTipUseReflection = $false
    }
    $short = $Text
    if ($short.Length -gt 63) {
        $short = $short.Substring(0, 63)
    }
    $Notify.Text = $short
}

function Update-ZapmanTrayUi {
    if (-not $script:trayNotify) {
        return
    }
    Set-ZapmanNotifyIconTip -Notify $script:trayNotify -Text (Get-ZapmanTrayNotifyText)
    $parts = @(Get-ZapmanTrayStatusParts)
    $n = 0
    foreach ($item in @($script:trayItemStatus)) {
        if (-not $item) {
            continue
        }
        $text = ''
        if ($n -lt $parts.Count) {
            $text = [string]$parts[$n]
        }
        $item.Text = $text
        $n++
    }
    if ($script:trayItemStart -or $script:trayItemStop) {
        $svc = Get-ZapretService
        $svcNotStopped = $false
        if ($svc) {
            $svcNotStopped = ($svc.Status -ne 'Stopped')
        }
        $bypassOn = Test-ZapretBypassRunning
        if ($script:trayItemStart) {
            $script:trayItemStart.Enabled = [bool]($svc -and $svc.Status -eq 'Stopped')
        }
        if ($script:trayItemStop) {
            $script:trayItemStop.Enabled = [bool]($bypassOn -or $svcNotStopped)
        }
    }
}

function Show-ZapmanTrayError {
    param([string]$Text)
    $msg = [string]$Text
    if ([string]::IsNullOrWhiteSpace($msg)) {
        return
    }
    if ($script:trayNotify) {
        $script:trayNotify.BalloonTipTitle = (Get-ZapmanUiString -Key 'AppName')
        $script:trayNotify.BalloonTipText = $msg
        $script:trayNotify.BalloonTipIcon = [System.Windows.Forms.ToolTipIcon]::Error
        $script:trayNotify.ShowBalloonTip(4000)
    }
}

function Get-ZapmanTrayIcon {
    $ico = Join-Path (Get-ZapretLayout).Gui 'app.ico'
    if (Test-Path -LiteralPath $ico) {
        return New-Object System.Drawing.Icon $ico
    }
    return [System.Drawing.SystemIcons]::Application
}

try {
    $script:trayMutex = New-Object System.Threading.Mutex($false, (Get-ZapmanTrayMutexName))
    try {
        $script:trayMutexOwned = $script:trayMutex.WaitOne(0)
    } catch [System.Threading.AbandonedMutexException] {
        $script:trayMutexOwned = $true
    }
    if (-not $script:trayMutexOwned) {
        exit 0
    }

    [System.Windows.Forms.Application]::EnableVisualStyles()
    $script:trayIcon = Get-ZapmanTrayIcon
    $script:trayNotify = New-Object System.Windows.Forms.NotifyIcon
    $script:trayNotify.Icon = $script:trayIcon
    $script:trayNotify.Visible = $true

    $script:trayMenu = New-Object System.Windows.Forms.ContextMenuStrip
    $script:trayItemStatus = New-Object System.Collections.ArrayList
    $i = 0
    while ($i -lt 3) {
        $statusItem = New-Object System.Windows.Forms.ToolStripMenuItem
        $statusItem.Enabled = $false
        [void]$script:trayItemStatus.Add($statusItem)
        [void]$script:trayMenu.Items.Add($statusItem)
        $i++
    }
    $itemOpen = New-Object System.Windows.Forms.ToolStripMenuItem
    $itemOpen.Text = (Get-ZapmanUiString -Key 'TrayOpen')
    $script:trayItemStart = New-Object System.Windows.Forms.ToolStripMenuItem
    $script:trayItemStart.Text = (Get-ZapmanUiString -Key 'TrayStart')
    $script:trayItemStop = New-Object System.Windows.Forms.ToolStripMenuItem
    $script:trayItemStop.Text = (Get-ZapmanUiString -Key 'TrayStop')

    [void]$script:trayMenu.Items.Add((New-Object System.Windows.Forms.ToolStripSeparator))
    [void]$script:trayMenu.Items.Add($itemOpen)
    [void]$script:trayMenu.Items.Add($script:trayItemStart)
    [void]$script:trayMenu.Items.Add($script:trayItemStop)

    $script:trayMenu.Add_Opening({ Update-ZapmanTrayUi })
    $itemOpen.Add_Click({
        try {
            Open-ZapmanGui
        } catch {
            Show-ZapmanTrayError -Text (Get-ZapmanExceptionText $_)
        }
    })
    $script:trayItemStart.Add_Click({
        try {
            $script:trayItemStart.Enabled = $false
            $script:trayItemStop.Enabled = $false
            Start-ZapretServiceIfInstalled
            Update-ZapmanTrayUi
        } catch {
            Show-ZapmanTrayError -Text (Get-ZapmanExceptionText $_)
            Update-ZapmanTrayUi
        }
    })
    $script:trayItemStop.Add_Click({
        try {
            $script:trayItemStart.Enabled = $false
            $script:trayItemStop.Enabled = $false
            Stop-ZapretBypass
            Update-ZapmanTrayUi
        } catch {
            Show-ZapmanTrayError -Text (Get-ZapmanExceptionText $_)
            Update-ZapmanTrayUi
        }
    })
    $script:trayNotify.Add_DoubleClick({
        try {
            Open-ZapmanGui
        } catch {
            Show-ZapmanTrayError -Text (Get-ZapmanExceptionText $_)
        }
    })
    $script:trayNotify.ContextMenuStrip = $script:trayMenu
    Update-ZapmanTrayUi

    $timer = New-Object System.Windows.Forms.Timer
    $timer.Interval = 2000
    $timer.Add_Tick({ Update-ZapmanTrayUi })
    $timer.Start()

    $script:trayCtx = New-Object System.Windows.Forms.ApplicationContext
    $script:trayNotify.Add_Disposed({
        if ($script:trayCtx) {
            $script:trayCtx.ExitThread()
        }
    })
    [System.Windows.Forms.Application]::Run($script:trayCtx)
} catch {
    $err = $_
    $errFile = Join-Path $env:TEMP 'zapman-tray-error.txt'
    $text = [string]$err
    try {
        $text = Get-ZapmanExceptionText $err
    } catch {
        $null = $_.Exception
    }
    try {
        [System.IO.File]::WriteAllText($errFile, $text)
    } catch {
        $null = $_.Exception
    }
    throw
} finally {
    if ($script:trayNotify) {
        $script:trayNotify.Visible = $false
        $script:trayNotify.Dispose()
    }
    if ($script:trayIcon) {
        $script:trayIcon.Dispose()
    }
    if ($script:trayMutexOwned -and $script:trayMutex) {
        try {
            [void]$script:trayMutex.ReleaseMutex()
        } catch {
            $null = $_.Exception
        }
    }
    if ($script:trayMutex) {
        $script:trayMutex.Dispose()
    }
}
