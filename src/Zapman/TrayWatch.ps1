# Zapret Manager: logon task and tray watcher process.

Set-StrictMode -Version Latest

function Get-ZapmanGuiMutexName {
    return 'Local\ZapmanGui'
}

function Get-ZapmanTrayMutexName {
    return 'Local\ZapmanTray'
}

function Get-ZapmanTrayTaskName {
    return 'zapman-tray'
}

function Get-ZapmanTrayScriptPath {
    return (Join-Path $PSScriptRoot 'Tray.ps1')
}

function Get-ZapmanPowershellExePath {
    $ps = Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'
    if (Test-Path -LiteralPath $ps) {
        return $ps
    }
    return 'powershell.exe'
}

function Test-ZapmanTrayWatchEnabled {
    return [bool]((Get-ZapmanConfig).trayWatch)
}

function Set-ZapmanTrayWatchEnabled {
    param([bool]$Enabled)
    [void](Update-ZapmanConfig -TrayWatch $Enabled)
    Sync-ZapmanTrayWatch
}

function Get-ZapmanWin32ProcessList {
    param([string]$Name)
    $filter = "Name = '$Name'"
    try {
        return @(Get-CimInstance -ClassName Win32_Process -Filter $filter -ErrorAction Stop)
    } catch {
        try {
            return @(Get-WmiObject -Class Win32_Process -Filter $filter -ErrorAction Stop)
        } catch {
            return @()
        }
    }
}

function Test-ZapmanNamedMutexHeld {
    param([string]$Name)
    $mutex = $null
    try {
        $mutex = New-Object System.Threading.Mutex($false, $Name)
        try {
            $owned = $mutex.WaitOne(0)
        } catch [System.Threading.AbandonedMutexException] {
            $owned = $true
        }
        if ($owned) {
            [void]$mutex.ReleaseMutex()
            return $false
        }
        return $true
    } catch {
        return $false
    } finally {
        if ($null -ne $mutex) {
            $mutex.Dispose()
        }
    }
}

function Initialize-ZapmanWin32WindowType {
    $code = @'
using System;
using System.Text;
using System.Runtime.InteropServices;
public static class ZapmanWin32Window {
    public delegate bool EnumProc(IntPtr hWnd, IntPtr lParam);
    [DllImport("user32.dll")] public static extern bool EnumWindows(EnumProc cb, IntPtr lp);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] public static extern int GetWindowText(IntPtr hWnd, StringBuilder sb, int max);
    [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);
    [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr hWnd);
}
'@
    if (-not ('ZapmanWin32Window' -as [type])) {
        Add-Type -TypeDefinition $code -ErrorAction Stop
    }
}

function Show-ZapmanGuiWindow {
    try {
        Initialize-ZapmanWin32WindowType
    } catch {
        return $false
    }
    $found = New-Object System.Collections.ArrayList
    $cb = [ZapmanWin32Window+EnumProc] {
        param([IntPtr]$hWnd, [IntPtr]$lp)
        [void]$lp
        if (-not [ZapmanWin32Window]::IsWindowVisible($hWnd)) {
            return $true
        }
        $sb = New-Object System.Text.StringBuilder 256
        [void][ZapmanWin32Window]::GetWindowText($hWnd, $sb, $sb.Capacity)
        $title = $sb.ToString()
        if ($title.StartsWith('Zapret Manager')) {
            [void]$found.Add($hWnd)
            return $false
        }
        return $true
    }
    [void][ZapmanWin32Window]::EnumWindows($cb, [IntPtr]::Zero)
    if ($found.Count -lt 1) {
        return $false
    }
    $h = [IntPtr]$found[0]
    [void][ZapmanWin32Window]::ShowWindow($h, 9)
    [void][ZapmanWin32Window]::SetForegroundWindow($h)
    return $true
}

function Open-ZapmanGui {
    if (Test-ZapmanNamedMutexHeld -Name (Get-ZapmanGuiMutexName)) {
        if (Show-ZapmanGuiWindow) {
            return
        }
        Start-Sleep -Milliseconds 400
        if (Show-ZapmanGuiWindow) {
            return
        }
        return
    }
    $boot = Join-Path $script:ZapmanGuiDir 'gui-boot.ps1'
    if (-not (Test-Path -LiteralPath $boot)) {
        throw ('GUI entry is missing: {0}' -f $boot)
    }
    $ps = Get-ZapmanPowershellExePath
    $argList = '-NoProfile -STA -WindowStyle Hidden -ExecutionPolicy Bypass -File "' + $boot + '"'
    Start-Process -FilePath $ps -ArgumentList $argList | Out-Null
}

function Invoke-ZapmanSchtasks {
    param([string]$Arguments)
    $exe = Join-Path $env:SystemRoot 'System32\schtasks.exe'
    if (-not (Test-Path -LiteralPath $exe)) {
        $exe = 'schtasks.exe'
    }
    $p = New-Object System.Diagnostics.Process
    $p.StartInfo.FileName = $exe
    $p.StartInfo.Arguments = $Arguments
    $p.StartInfo.UseShellExecute = $false
    $p.StartInfo.RedirectStandardOutput = $true
    $p.StartInfo.RedirectStandardError = $true
    $p.StartInfo.CreateNoWindow = $true
    [void]$p.Start()
    $out = $p.StandardOutput.ReadToEnd()
    $err = $p.StandardError.ReadToEnd()
    $p.WaitForExit()
    $text = ($out + $err).Trim()
    return New-Object PSObject -Property @{
        ExitCode = [int]$p.ExitCode
        Text     = $text
    }
}

function Register-ZapmanTrayTask {
    $scriptPath = Get-ZapmanTrayScriptPath
    if (-not (Test-Path -LiteralPath $scriptPath)) {
        throw ('Tray script is missing: {0}' -f $scriptPath)
    }
    $ps = Get-ZapmanPowershellExePath
    $tr = ('{0} -NoProfile -STA -WindowStyle Hidden -ExecutionPolicy Bypass -File {1}' -f $ps, $scriptPath)
    $name = Get-ZapmanTrayTaskName
    $taskArgs = '/Create /TN "' + $name + '" /SC ONLOGON /RL HIGHEST /F /TR "' + $tr + '"'
    $res = Invoke-ZapmanSchtasks -Arguments $taskArgs
    if ($res.ExitCode -ne 0) {
        $msg = $res.Text
        if ([string]::IsNullOrWhiteSpace($msg)) {
            $msg = ('schtasks exit {0}' -f $res.ExitCode)
        }
        throw $msg
    }
}

function Unregister-ZapmanTrayTask {
    $name = Get-ZapmanTrayTaskName
    $query = Invoke-ZapmanSchtasks -Arguments ('/Query /TN "' + $name + '"')
    if ($query.ExitCode -ne 0) {
        return
    }
    $res = Invoke-ZapmanSchtasks -Arguments ('/Delete /TN "' + $name + '" /F')
    if ($res.ExitCode -eq 0) {
        return
    }
    $text = [string]$res.Text
    if ([string]::IsNullOrWhiteSpace($text)) {
        throw ('schtasks delete exit {0}' -f $res.ExitCode)
    }
    throw $text
}

function Start-ZapmanTrayWatchProcess {
    if (Test-ZapmanNamedMutexHeld -Name (Get-ZapmanTrayMutexName)) {
        return
    }
    $scriptPath = Get-ZapmanTrayScriptPath
    if (-not (Test-Path -LiteralPath $scriptPath)) {
        throw ('Tray script is missing: {0}' -f $scriptPath)
    }
    $ps = Get-ZapmanPowershellExePath
    $argList = '-NoProfile -STA -WindowStyle Hidden -ExecutionPolicy Bypass -File "' + $scriptPath + '"'
    Start-Process -FilePath $ps -ArgumentList $argList | Out-Null
}

function Stop-ZapmanTrayWatchProcess {
    $self = [int]$PID
    $needle = 'Zapman\Tray.ps1'
    foreach ($proc in @(Get-ZapmanWin32ProcessList -Name 'powershell.exe')) {
        $cmd = ''
        if ($proc.PSObject.Properties['CommandLine'] -and $null -ne $proc.CommandLine) {
            $cmd = [string]$proc.CommandLine
        }
        if ([string]::IsNullOrWhiteSpace($cmd)) {
            continue
        }
        if ($cmd.IndexOf($needle, [StringComparison]::OrdinalIgnoreCase) -lt 0) {
            continue
        }
        $id = [int]$proc.ProcessId
        if ($id -eq $self) {
            continue
        }
        Stop-Process -Id $id -Force -ErrorAction SilentlyContinue
    }
}

function Sync-ZapmanTrayWatch {
    if (Test-ZapmanTrayWatchEnabled) {
        Register-ZapmanTrayTask
        Start-ZapmanTrayWatchProcess
    } else {
        Unregister-ZapmanTrayTask
        Stop-ZapmanTrayWatchProcess
    }
}
