@echo off
::
:: install.bat - one-click installer for keyboardctl.
::
:: Does everything needed to go from a fresh checkout to a working
:: "keyboardctl -key a":
::   1. Re-launches itself elevated (UAC) if not already Administrator.
::   2. Enables Windows test-signing mode when the driver is unsigned
::      (dev/test builds) and reboots once if a reboot is required.
::   3. Installs and starts the KeyboardSimulator kernel driver.
::   4. Builds keyboardctl.exe with the Go toolchain (if Go is installed
::      and the binary is missing).
::   5. Adds this directory to the current user's PATH.
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

set "DRIVER_NAME=KbdFilter"
set "DRIVER_SYS=%ROOT%\driver\keyboardsimulator.sys"
set "BINARY=%ROOT%\keyboardctl.exe"

:: ── 2. Driver binary present? ─────────────────────────────────────────────
if not exist "%DRIVER_SYS%" (
    echo ERROR: driver binary not found:
    echo     %DRIVER_SYS%
    echo.
    echo Build the driver first (Visual Studio + WDK required):
    echo     cd /d "%ROOT%\driver"
    echo     msbuild driver.vcxproj /p:Configuration=Release /p:Platform=x64
    echo.
    echo See README.md ^("构建驱动"^) for details.
    pause
    exit /b 1
)

:: ── 3. Test signing (unsigned dev driver only) ────────────────────────────
:: If the .sys carries no signature, Windows refuses to load it unless
:: test-signing is on.  Detect and enable it automatically.
::
:: NOTE: use the runtime "if errorlevel N" form (not "%errorlevel%") because
:: these checks live inside parenthesised blocks, where %VAR% would be
:: expanded once at parse time and read a stale value.
powershell -NoProfile -Command ^
    "$s = Get-AuthenticodeSignature '%DRIVER_SYS%'; if ($s.Status -eq 'Valid') { exit 0 } else { exit 1 }" >nul 2>&1
if errorlevel 1 (
    echo Driver is not signed - enabling Windows test-signing mode...
    bcdedit /set testsigning on >nul 2>&1
    if errorlevel 1 (
        echo WARNING: could not enable test-signing automatically.
        echo          Run "bcdedit /set testsigning on" manually and reboot.
    ) else (
        echo Test-signing enabled. A REBOOT is required before the driver loads.
        set "NEED_REBOOT=1"
    )
) else (
    echo Driver signature is valid.
)

:: ── 4. Install + start the kernel service ─────────────────────────────────
echo.
echo Installing %DRIVER_NAME% driver service...
sc query %DRIVER_NAME% >nul 2>&1
if errorlevel 1 (
    sc create %DRIVER_NAME% type= kernel binPath= "%DRIVER_SYS%" start= demand DisplayName= "Keyboard Filter Driver"
    if errorlevel 1 (
        echo ERROR: sc create failed. Is this shell elevated?
        pause
        exit /b 1
    )
) else (
    echo Driver service already exists - updating binary path...
    sc config %DRIVER_NAME% binPath= "%DRIVER_SYS%" >nul
)

echo Starting %DRIVER_NAME%...
sc start %DRIVER_NAME% >nul 2>&1
if errorlevel 1 (
    echo WARNING: driver did not start ^(this is expected if a reboot is pending^).
)

:: ── 5. Build keyboardctl.exe ──────────────────────────────────────────────
echo.
if exist "%BINARY%" goto :binary_ok

where go >nul 2>&1
if %errorlevel% neq 0 (
    echo NOTE: Go toolchain not found - skipping keyboardctl.exe build.
    echo       Install Go ^(^>= 1.23^) then run: make build
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
echo keyboardctl.exe already present.

:: ── 6. Add to user PATH ───────────────────────────────────────────────────
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
    echo  After rebooting, open a new terminal and run:
    echo      keyboardctl -key a
) else (
    echo  Installation complete.
    echo  Open a NEW terminal and run:
    echo      keyboardctl -key a
)
echo ============================================================
echo.
pause
exit /b 0
