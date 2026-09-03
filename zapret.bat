@echo off
:: One PowerShell process: host checks and GUI. Do not start powershell.exe three times.

setlocal
cd /d "%~dp0"

where powershell.exe >nul 2>&1
if errorlevel 1 (
    echo ERROR: powershell.exe is not found in PATH.
    echo Install Windows PowerShell 5.1 ^(WMF 5.1 on Windows 7^).
    pause
    exit /b 1
)

if not exist "%~dp0src\utils\gui.ps1" (
    echo ERROR: src\utils\gui.ps1 is not found.
    pause
    exit /b 1
)

powershell.exe -NoProfile -STA -ExecutionPolicy Bypass -File "%~dp0src\utils\gui.ps1"
if errorlevel 1 (
    echo ERROR: The GUI failed to start.
    pause
    exit /b 1
)

endlocal
exit /b 0
