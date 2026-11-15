@echo off
::
:: uninstall.bat - stop and remove the keyboardctl driver service.
::
:: Re-launches itself elevated (UAC) if needed, then stops and deletes the
:: KbdFilter kernel service and removes this directory from the user PATH
:: (undoing install.bat).  Leaves the Go binary in place.
::

setlocal EnableExtensions

set "ROOT=%~dp0"
if "%ROOT:~-1%"=="\" set "ROOT=%ROOT:~0,-1%"

:: ── Elevate ───────────────────────────────────────────────────────────────
net session >nul 2>&1
if %errorlevel% neq 0 (
    echo Requesting administrator privileges...
    powershell -NoProfile -ExecutionPolicy Bypass -Command ^
        "Start-Process -Verb RunAs -FilePath '%~f0'"
    exit /b 0
)

set "DRIVER_NAME=KbdFilter"

echo Stopping %DRIVER_NAME% driver...
sc stop %DRIVER_NAME% >nul 2>&1

echo Removing %DRIVER_NAME% driver service...
sc delete %DRIVER_NAME% >nul 2>&1
if %errorlevel% neq 0 (
    echo WARNING: service not found or already removed.
) else (
    echo Driver service removed.
)

:: ── Remove this directory from the user PATH (added by install.bat) ────────
echo.
echo Removing "%ROOT%" from your user PATH...
powershell -NoProfile -Command ^
    "$p = [Environment]::GetEnvironmentVariable('Path','User');" ^
    "$parts = $p -split ';' | Where-Object { $_ -and ($_ -ne '%ROOT%') };" ^
    "[Environment]::SetEnvironmentVariable('Path', ($parts -join ';'), 'User')"

echo.
echo Done. Test-signing mode (if enabled by install.bat) is left as-is.
echo To disable it: bcdedit /set testsigning off  ^(then reboot^).
echo.
pause
exit /b 0
