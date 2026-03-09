@echo off
::
:: uninstall.bat - Stop and remove the KeyboardSimulator kernel driver
::
:: Must be run as Administrator.
::

setlocal

set DRIVER_NAME=KeyboardSimulator

echo Stopping %DRIVER_NAME% driver...
sc stop %DRIVER_NAME%
if %errorlevel% neq 0 (
    echo WARNING: sc stop returned %errorlevel% ^(driver may not be running^)
)

echo Removing %DRIVER_NAME% driver service...
sc delete %DRIVER_NAME%
if %errorlevel% neq 0 (
    echo ERROR: Failed to remove driver service ^(error %errorlevel%^)
    exit /b %errorlevel%
)

echo.
echo Driver stopped and removed successfully.
exit /b 0
