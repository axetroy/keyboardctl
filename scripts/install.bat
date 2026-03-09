@echo off
::
:: install.bat - Install and start the KeyboardSimulator kernel driver
::
:: Must be run as Administrator.
:: The driver binary (keyboardsimulator.sys) must exist in the driver\ directory.
::

setlocal

set DRIVER_NAME=KeyboardSimulator
set DRIVER_BIN=%~dp0..\driver\keyboardsimulator.sys

if not exist "%DRIVER_BIN%" (
    echo ERROR: Driver binary not found: %DRIVER_BIN%
    echo Build the driver first:  cd driver ^&^& build -cZ
    exit /b 1
)

echo Installing %DRIVER_NAME% driver...
sc create %DRIVER_NAME% type= kernel binPath= "%DRIVER_BIN%" start= demand DisplayName= "Keyboard Simulator Driver"
if %errorlevel% neq 0 (
    echo WARNING: sc create returned %errorlevel% ^(driver may already be installed^)
)

echo Starting %DRIVER_NAME% driver...
sc start %DRIVER_NAME%
if %errorlevel% neq 0 (
    echo ERROR: Failed to start driver ^(error %errorlevel%^)
    echo Check Event Viewer for details.
    exit /b %errorlevel%
)

echo.
echo Driver installed and started successfully.
echo Device: \\.\KeyboardSimulator
exit /b 0
