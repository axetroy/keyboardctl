@echo off
::
:: uninstall.bat - remove the keyboardctl filter and undo install.bat.
::
:: Re-launches itself elevated (UAC) if needed, then:
::   1. Deletes the driver package from the Driver Store (pnputil).
::   2. Repairs the Keyboard class UpperFilters if "keyboardctl" is still listed,
::      because removing a driver package does not reliably undo the class-level
::      registry value the INF wrote.  A stale entry pointing at a driver that is
::      no longer installed is not a state to leave a keyboard stack in.
::   3. Removes this directory from the user PATH (undoing install.bat).
::
:: Leaves the Go binary and test-signing in place.
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

:: ── Remove the driver package from the Driver Store ────────────────────────
echo Removing the keyboardctl driver package...
powershell -NoProfile -Command ^
    "$pub=$null; $target=$null;" ^
    "foreach ($line in (pnputil /enum-drivers)) {" ^
    "  if ($line -match 'Published Name\s*:\s*(\S+)') { $pub=$Matches[1] }" ^
    "  elseif ($line -match 'Original Name\s*:\s*(\S+)') { if ($Matches[1] -ieq 'keyboardctl.inf') { $target=$pub } }" ^
    "}" ^
    "if ($target) { Write-Host ('Deleting ' + $target); pnputil /delete-driver $target /uninstall /force }" ^
    "else { Write-Host 'NOTE: no keyboardctl package found in the Driver Store - nothing to delete.' }"

:: ── Repair the Keyboard class UpperFilters ─────────────────────────────────
:: Remove our entry but keep kbdclass (and any other filter the user added).
:: kbdclass must stay: it is the keyboard class driver itself, not a filter we
:: added, so an empty list is never correct.
powershell -NoProfile -Command ^
    "$k='HKLM:\SYSTEM\CurrentControlSet\Control\Class\{4D36E96B-E325-11CE-BFC1-08002BE10318}';" ^
    "$c=@((Get-ItemProperty -Path $k -Name UpperFilters -ErrorAction SilentlyContinue).UpperFilters);" ^
    "if ($c -contains 'keyboardctl') {" ^
    "  Write-Host ('Current Keyboard class UpperFilters: ' + ($c -join ', '));" ^
    "  $n=@($c | Where-Object { $_ -ne 'keyboardctl' });" ^
    "  if ($n.Count -eq 0) { $n=@('kbdclass') };" ^
    "  Write-Host ('New Keyboard class UpperFilters: ' + ($n -join ', '));" ^
    "  Set-ItemProperty -Path $k -Name UpperFilters -Value $n -Type MultiString;" ^
    "  Write-Host 'A REBOOT is required for the keyboard stack to drop the filter.';" ^
    "} else { Write-Host 'keyboardctl is not in UpperFilters - no change needed.' }"

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
