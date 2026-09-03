@echo off
:: Start the GUI. If it fails, the error stays in this window.

setlocal
cd /d "%~dp0"

:: Inbox Windows PowerShell. Do not scan PATH.
set "PS=%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe"
"%PS%" -NoProfile -STA -ExecutionPolicy Bypass -File "%~dp0src\gui\gui-boot.ps1"
:: Keep the console open so the user can read the PowerShell error.
if errorlevel 1 (
    echo ERROR: The GUI failed to start.
    pause
    exit /b 1
)

endlocal
exit /b 0
