@echo off
::
:: install.bat - one-click installer for keyboardctl.
::
:: Takes a freshly unpacked release (keyboardctl.exe + keyboardsimulator.sys +
:: keyboardctl.inf + keyboardctl-test.cer) to a working "keyboardctl -key a":
::
::   1. Re-launches itself elevated (UAC) if not already Administrator.
::   2. Refuses to run if a third-party keyboard filter is already installed,
::      because the filter list is rewritten as a whole and that product would
::      stop working.
::   3. Trusts the bundled test certificate and enables Windows test-signing
::      (required because the driver is test-signed, not production-signed).
::   4. Installs the filter with pnputil, then makes sure it is registered ahead
::      of kbdclass in the Keyboard class UpperFilters.
::   5. Builds keyboardctl.exe with Go if it is missing and Go is available.
::   6. Adds this directory to the current user's PATH.
::   7. Self-tests the install.
::
:: Just double-click this file.
::

setlocal EnableExtensions

set "ROOT=%~dp0"
if "%ROOT:~-1%"=="\" set "ROOT=%ROOT:~0,-1%"

:: ── 1. Elevate ────────────────────────────────────────────────────────────
net session >nul 2>&1
if %errorlevel% neq 0 (
    echo Requesting administrator privileges...
    powershell -NoProfile -ExecutionPolicy Bypass -Command ^
        "Start-Process -Verb RunAs -FilePath '%~f0'"
    echo If no UAC prompt appeared, right-click install.bat and choose
    echo "Run as administrator".
    pause
    exit /b 0
)

echo ============================================================
echo  keyboardctl installer
echo ============================================================
echo.

set "DRIVER_SYS=%ROOT%\keyboardsimulator.sys"
set "DRIVER_INF=%ROOT%\keyboardctl.inf"
set "DRIVER_CAT=%ROOT%\keyboardctl.cat"
set "BINARY=%ROOT%\keyboardctl.exe"
set "CERT=%ROOT%\keyboardctl-test.cer"

:: The Keyboard device class.  Its UpperFilters value decides where our filter
:: sits: it must come before kbdclass so it is below kbdclass in the stack,
:: which is the only place injection can capture the connect callback.
set "KB_CLASS_KEY=HKLM\SYSTEM\CurrentControlSet\Control\Class\{4D36E96B-E325-11CE-BFC1-08002BE10318}"

:: ── 2. Driver files present? ──────────────────────────────────────────────
if not exist "%DRIVER_SYS%" (
    echo ERROR: driver binary not found:
    echo     %DRIVER_SYS%
    echo.
    echo If you built from source, build the driver first ^(VS 2022 + WDK^):
    echo     cd /d "%ROOT%"
    echo     msbuild driver\driver.vcxproj /p:Configuration=Release /p:Platform=x64
    echo.
    echo See README.md for details.
    pause
    exit /b 1
)
if not exist "%DRIVER_INF%" (
    echo ERROR: driver INF not found:
    echo     %DRIVER_INF%
    echo.
    echo A release package always ships keyboardctl.inf next to the .sys.
    pause
    exit /b 1
)
:: The INF declares CatalogFile=keyboardctl.cat, so pnputil refuses the package
:: without it.  Catch it here rather than let pnputil fail with a vague message.
if not exist "%DRIVER_CAT%" (
    echo ERROR: driver catalog not found:
    echo     %DRIVER_CAT%
    echo.
    echo The INF requires keyboardctl.cat.  A release package ships it next to the
    echo .sys and .inf; a source build produces it in driver\x64\Release.
    pause
    exit /b 1
)

:: ── 3. Refuse to clobber a third-party keyboard filter ────────────────────
:: UpperFilters is rewritten as a whole, so any existing entry that is not
:: kbdclass would be dropped and that product would break.  Bail out instead of
:: silently disabling somebody else's driver.
powershell -NoProfile -Command ^
    "$k='HKLM:\SYSTEM\CurrentControlSet\Control\Class\{4D36E96B-E325-11CE-BFC1-08002BE10318}';" ^
    "$c=@((Get-ItemProperty -Path $k -Name UpperFilters -ErrorAction SilentlyContinue).UpperFilters);" ^
    "Write-Host ('Current Keyboard class UpperFilters: ' + $(if ($c) { $c -join ', ' } else { '<none>' }));" ^
    "$foreign=@($c | Where-Object { $_ -and $_ -inotmatch '^(kbdclass|keyboardctl)$' });" ^
    "if ($foreign.Count -gt 0) { Write-Host ('FOREIGN: ' + ($foreign -join ', ')); exit 1 } else { exit 0 }"
if errorlevel 1 (
    echo.
    echo ERROR: another keyboard filter is already installed (see FOREIGN above).
    echo        Installing would remove it from UpperFilters and break it.
    echo        Add it to [keyboardctl.AddReg] in keyboardctl.inf by hand, keeping
    echo        the order, then install again.
    pause
    exit /b 1
)

:: Was our filter already registered before this run?  If so, a reboot is NOT
:: forced and the self-test below actually runs, which is the whole point: a
:: second run after rebooting should prove the install works.  A first-time
:: registration does need a reboot, because the filter only enters the keyboard
:: stack when the stack is built at boot.
reg query "%KB_CLASS_KEY%" /v UpperFilters 2>nul | findstr /i "keyboardctl" >nul
if errorlevel 1 (set "HAD_FILTER=0") else (set "HAD_FILTER=1")

:: ── 4. Trust the test certificate + enable test-signing ───────────────────
:: The release driver is signed with a self-signed test certificate.  Windows
:: loads it only if the certificate is trusted AND test-signing is on.
::
:: NOTE: use the runtime "if errorlevel N" form (not "%errorlevel%") because
:: these checks live inside parenthesised blocks, where %VAR% would be
:: expanded once at parse time and read a stale value.
if exist "%CERT%" (
    echo Trusting the bundled test certificate...
    certutil -addstore -f Root            "%CERT%" >nul 2>&1
    certutil -addstore -f TrustedPublisher "%CERT%" >nul 2>&1
) else (
    echo NOTE: %CERT% not found - assuming the driver is already trusted.
)

powershell -NoProfile -Command ^
    "$s = Get-AuthenticodeSignature '%DRIVER_SYS%'; if ($s.Status -eq 'Valid') { exit 0 } else { exit 1 }" >nul 2>&1
if errorlevel 1 (
    echo Driver is not trusted by this machine - enabling Windows test-signing...
    bcdedit /set testsigning on >nul 2>&1
    if errorlevel 1 (
        echo WARNING: could not enable test-signing automatically.
        echo          Run "bcdedit /set testsigning on" manually and reboot.
        echo          If Secure Boot blocks it, disable Secure Boot in the BIOS.
    ) else (
        echo Test-signing enabled. A REBOOT is required before the driver loads.
        set "NEED_REBOOT=1"
    )
) else (
    echo Driver signature is valid.
)

:: ── 5. Install the filter package ─────────────────────────────────────────
:: pnputil copies the package into the Driver Store, creates the keyboardctl
:: service and applies the INF's AddReg.  A keyboard filter is picked up at
:: boot, so a reboot is needed before it is in the keyboard stack.
echo.
echo Installing the keyboard filter package (pnputil)...
pnputil /add-driver "%DRIVER_INF%" /install
if errorlevel 1 (
    echo ERROR: pnputil failed. Is this shell elevated?
    pause
    exit /b 1
)

:: ── 6. Make sure UpperFilters has us ahead of kbdclass ────────────────────
:: The INF writes this, but pnputil's handling of a primitive driver's AddReg
:: is not guaranteed on every Windows build, so set it explicitly and verify.
:: The previous value is saved under ProgramData (the install directory may be
:: read-only) as a record of what was there.
echo.
echo Registering the filter in the Keyboard class UpperFilters...
powershell -NoProfile -Command ^
    "$k='HKLM:\SYSTEM\CurrentControlSet\Control\Class\{4D36E96B-E325-11CE-BFC1-08002BE10318}';" ^
    "$c=@((Get-ItemProperty -Path $k -Name UpperFilters -ErrorAction SilentlyContinue).UpperFilters);" ^
    "try {" ^
    "  $dir=Join-Path $env:ProgramData 'keyboardctl'; New-Item -ItemType Directory -Force -Path $dir | Out-Null;" ^
    "  Set-Content -Path (Join-Path $dir 'UpperFilters-backup.txt') -Value ($c -join \"`n\") -ErrorAction Stop;" ^
    "  Write-Host ('previous UpperFilters saved to ' + $dir);" ^
    "} catch { Write-Host 'NOTE: could not save a backup of the previous UpperFilters.' }" ^
    "$n=@('keyboardctl') + @($c | Where-Object { $_ -and $_ -ine 'keyboardctl' });" ^
    "Set-ItemProperty -Path $k -Name UpperFilters -Value $n -Type MultiString;" ^
    "Write-Host ('New Keyboard class UpperFilters: ' + ($n -join ', '))"
if errorlevel 1 (
    echo ERROR: could not update UpperFilters.
    pause
    exit /b 1
)

reg query "%KB_CLASS_KEY%" /v UpperFilters 2>nul | findstr /i "keyboardctl" >nul
if errorlevel 1 (
    echo ERROR: the filter is not in the Keyboard class UpperFilters.
    echo        Injection cannot work without it. Please report this.
    pause
    exit /b 1
)
echo Filter registered.
:: Only a first-time registration needs a reboot: the filter joins the keyboard
:: stack when it is built at boot.  On a re-run (e.g. after the user rebooted and
:: ran install.bat again) there is nothing pending, so let the self-test run.
if "%HAD_FILTER%"=="0" (
    echo A REBOOT is required to load the filter into the keyboard stack.
    set "NEED_REBOOT=1"
) else (
    echo The filter was already registered before this run.
)

:: ── 7. Build keyboardctl.exe (source builds only) ─────────────────────────
echo.
if exist "%BINARY%" goto :binary_ok

where go >nul 2>&1
if %errorlevel% neq 0 (
    echo NOTE: keyboardctl.exe not found and the Go toolchain is not installed.
    echo       Download a prebuilt release, or install Go ^(^>= 1.23^) and run:
    echo           make build
    goto :path_setup
)

echo Building keyboardctl.exe...
pushd "%ROOT%"
set "GOOS=windows" & set "GOARCH=amd64" & set "CGO_ENABLED=0"
go build -trimpath -ldflags "-s -w" -o keyboardctl.exe .\cmd\keyboardctl
set "GOOS=" & set "GOARCH=" & set "CGO_ENABLED="
popd
goto :path_setup

:binary_ok
echo keyboardctl.exe found.

:: ── 8. Add to user PATH ───────────────────────────────────────────────────
:path_setup
echo.
echo %PATH% | find /i "%ROOT%" >nul
if %errorlevel% neq 0 (
    echo Adding "%ROOT%" to your PATH...
    powershell -NoProfile -Command ^
        "$p = [Environment]::GetEnvironmentVariable('Path','User');" ^
        "if ($p -notlike '*%ROOT%*') { [Environment]::SetEnvironmentVariable('Path', $p + ';%ROOT%', 'User') }"
) else (
    echo "%ROOT%" is already on your PATH.
)

echo.
echo ============================================================
if defined NEED_REBOOT (
    echo  Installation complete - REBOOT REQUIRED.
    echo.
    echo  After rebooting, open a new terminal and run:
    echo      keyboardctl -selftest
    echo      keyboardctl -key a
) else (
    echo  Installation complete.
    echo.
    echo  Open a new terminal and run:
    echo      keyboardctl -key a
)
echo ============================================================
echo.

:: ── 9. Finish: offer the reboot, or self-test ──────────────────────────────
:: A reboot is unavoidable the first time: the filter only joins the keyboard
:: stack when it is built at boot.  Offer it here so the user does not have to
:: remember.  When no reboot is pending (a re-run), self-test instead, so a
:: broken install surfaces now rather than on the user's first real keystroke.
if defined NEED_REBOOT (
    echo.
    choice /C YN /N /M "Reboot now to load the filter into the keyboard stack? [Y/N] "
    if errorlevel 2 goto :done
    echo.
    echo Rebooting in 15 seconds - save your work now.
    echo Press Ctrl+C in this window to cancel the reboot.
    shutdown /r /t 15 /c "keyboardctl: a reboot is required to load the keyboard filter"
    exit /b 0
)

if not exist "%BINARY%" (
    echo NOTE: keyboardctl.exe not found - skipping the self-test.
    goto :done
)

echo Running self-test...
"%BINARY%" -selftest
if errorlevel 1 (
    echo.
    echo WARNING: the driver is not ready (see above).
    echo          If you just enabled test-signing, reboot and re-run install.bat.
    echo          Otherwise make sure you are in an elevated terminal.
) else (
    echo Self-test passed - the driver is ready.
)

:done
pause
exit /b 0
