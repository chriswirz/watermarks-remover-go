@echo off
REM Build wmr. Usage: build.cmd [all]
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0build.ps1" %*
exit /b %ERRORLEVEL%
