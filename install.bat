@echo off
::
:: install.bat - one-click installer for keyboardctl.
::
:: Takes a freshly unpacked release (keyboardctl.exe + keyboardsimulator.sys +
:: keyboardctl-test.cer) to a working "keyboardctl -key a":
::
::   1. Re-launches itself elevated (UAC) if not already Administrator.
::   2. Trusts the bundled test certificate and enables Windows test-signing
::      (required because the driver is test-signed, not production-signed).
::   3. Installs and starts the KeyboardSimulator kernel driver.
::   4. Builds keyboardctl.exe with Go if it is missing and Go is available.
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
set "DRIVER_SYS=%ROOT%\keyboardsimulator.sys"
set "BINARY=%ROOT%\keyboardctl.exe"
set "CERT=%ROOT%\keyboardctl-test.cer"

:: ── 2. Driver binary present? ─────────────────────────────────────────────
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

:: ── 3. Trust the test certificate + enable test-signing ───────────────────
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

:: ── 5. Build keyboardctl.exe (source builds only) ─────────────────────────
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
