# Tiny -File entry. Hide the console, elevate, show the main window, then parse gui.ps1.
# The visible shell is Main.xaml. Labels and actions fill after the module load.

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Continue'

$script:startupClock = [System.Diagnostics.Stopwatch]::StartNew()
$script:startupLastMs = 0
$script:startupMarks = New-Object System.Collections.ArrayList
$script:guiBootOwnsRun = $false
$script:guiShellWindow = $null
$script:guiApp = $null

function Add-GuiStartupMark {
    param([string]$Name)
    $now = [int]$script:startupClock.ElapsedMilliseconds
    $delta = $now - $script:startupLastMs
    $script:startupLastMs = $now
    [void]$script:startupMarks.Add(('{0}: {1} ms' -f $Name, $delta))
}

function Initialize-GuiBootConsoleType {
    $code = @'
using System;
using System.Runtime.InteropServices;
public static class GuiBootConsole {
    [DllImport("kernel32.dll")] public static extern IntPtr GetConsoleWindow();
    [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);
}
'@
    if (-not ('GuiBootConsole' -as [type])) {
        Add-Type -TypeDefinition $code -ErrorAction Stop
    }
}

function Hide-GuiBootConsole {
    try {
        Initialize-GuiBootConsoleType
        $hwnd = [GuiBootConsole]::GetConsoleWindow()
        if ($hwnd -ne [IntPtr]::Zero) {
            [void][GuiBootConsole]::ShowWindow($hwnd, 0)
        }
    } catch {
        return
    }
}

function Show-GuiBootConsole {
    try {
        Initialize-GuiBootConsoleType
        $hwnd = [GuiBootConsole]::GetConsoleWindow()
        if ($hwnd -ne [IntPtr]::Zero) {
            [void][GuiBootConsole]::ShowWindow($hwnd, 5)
        }
    } catch {
        return
    }
}

function Initialize-GuiBootWin32 {
    $code = @'
using System;
using System.Text;
using System.Runtime.InteropServices;
public static class GuiBootWin32 {
    public delegate bool EnumProc(IntPtr hWnd, IntPtr lParam);
    [DllImport("user32.dll")] public static extern bool EnumWindows(EnumProc cb, IntPtr lp);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] public static extern int GetWindowText(IntPtr hWnd, StringBuilder sb, int max);
    [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);
    [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr hWnd);
}
'@
    if (-not ('GuiBootWin32' -as [type])) {
        Add-Type -TypeDefinition $code -ErrorAction Stop
    }
}

function Show-GuiBootExistingWindow {
    try {
        Initialize-GuiBootWin32
    } catch {
        return
    }
    $found = New-Object System.Collections.ArrayList
    $cb = [GuiBootWin32+EnumProc] {
        param([IntPtr]$hWnd, [IntPtr]$lp)
        [void]$lp
        if (-not [GuiBootWin32]::IsWindowVisible($hWnd)) {
            return $true
        }
        $sb = New-Object System.Text.StringBuilder 256
        [void][GuiBootWin32]::GetWindowText($hWnd, $sb, $sb.Capacity)
        $title = $sb.ToString()
        if ($title.StartsWith('Zapret Manager')) {
            [void]$found.Add($hWnd)
            return $false
        }
        return $true
    }
    [void][GuiBootWin32]::EnumWindows($cb, [IntPtr]::Zero)
    if ($found.Count -lt 1) {
        return
    }
    $h = [IntPtr]$found[0]
    [void][GuiBootWin32]::ShowWindow($h, 9)
    [void][GuiBootWin32]::SetForegroundWindow($h)
}

function Test-GuiBootAdministrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Update-GuiBootWindowPaint {
    param($Window)
    if (-not $Window) {
        return
    }
    try {
        $Window.Dispatcher.Invoke(
            [System.Windows.Threading.DispatcherPriority]::Render,
            [System.Action]{ $null }
        )
    } catch {
        return
    }
}

Hide-GuiBootConsole
Add-GuiStartupMark 'hide-console'

if (-not (Test-GuiBootAdministrator)) {
    $boot = $PSCommandPath
    $argList = "-NoProfile -STA -ExecutionPolicy Bypass -WindowStyle Hidden -File `"$boot`""
    $ps = Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'
    if (-not (Test-Path -LiteralPath $ps)) {
        $ps = 'powershell.exe'
    }
    try {
        Start-Process -FilePath $ps -ArgumentList $argList -Verb RunAs | Out-Null
        exit 0
    } catch {
        Show-GuiBootConsole
        Write-Host 'ERROR: Administrator rights are required.'
        exit 1
    }
}

# Same name as Get-ZapmanGuiMutexName in src/Zapman/TrayWatch.ps1.
$script:guiMutex = New-Object System.Threading.Mutex($false, 'Local\ZapmanGui')
$script:guiMutexOwned = $false
try {
    $script:guiMutexOwned = $script:guiMutex.WaitOne(0)
} catch [System.Threading.AbandonedMutexException] {
    $script:guiMutexOwned = $true
}
if (-not $script:guiMutexOwned) {
    Show-GuiBootExistingWindow
    exit 0
}

try {
    Add-Type -AssemblyName PresentationFramework -ErrorAction Stop
    Add-Type -AssemblyName PresentationCore -ErrorAction Stop
    Add-Type -AssemblyName WindowsBase -ErrorAction Stop
} catch {
    Show-GuiBootConsole
    Write-Host $_.Exception.Message
    exit 1
}

$xamlPath = Join-Path $PSScriptRoot 'Main.xaml'
if (-not (Test-Path -LiteralPath $xamlPath)) {
    Show-GuiBootConsole
    Write-Host ('ERROR: XAML file is missing: {0}' -f $xamlPath)
    exit 1
}

try {
    $script:guiApp = New-Object System.Windows.Application
    $script:guiApp.ShutdownMode = [System.Windows.ShutdownMode]::OnMainWindowClose
    $fs = New-Object System.IO.FileStream($xamlPath, [System.IO.FileMode]::Open, [System.IO.FileAccess]::Read)
    try {
        $window = [System.Windows.Markup.XamlReader]::Load($fs)
    } finally {
        $fs.Dispose()
    }
    $window.Title = 'Zapret Manager'
    $title = $window.FindName('title')
    if ($null -ne $title) {
        $title.Text = 'Zapret Manager'
    }
    foreach ($name in @(
            'btnStart', 'btnStop', 'btnRemove', 'btnStrategy', 'btnFakes',
            'btnIpsetUpd', 'btnHosts', 'btnUpdates', 'btnDiag',
            'cmbGame', 'cmbIpset', 'chkAuto', 'chkTray'
        )) {
        $el = $window.FindName($name)
        if ($null -ne $el) {
            $el.IsEnabled = $false
        }
    }
    $script:guiApp.MainWindow = $window
    $window.Show()
    [void]$window.Activate()
    Update-GuiBootWindowPaint -Window $window
    $script:guiShellWindow = $window
    $script:guiBootOwnsRun = $true
    Add-GuiStartupMark 'shell'
} catch {
    Show-GuiBootConsole
    Write-Host $_.Exception.Message
    exit 1
}

. (Join-Path $PSScriptRoot 'gui.ps1')

try {
    $app = [System.Windows.Application]::Current
    if (-not $app) {
        $app = $script:guiApp
    }
    [void]$app.Run()
} catch {
    Show-GuiBootConsole
    Write-Host $_.Exception.Message
    exit 1
}
