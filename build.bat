@echo off
setlocal enabledelayedexpansion

echo =======================================================
echo   WorkBuddy AI CLIProxyAPI Plugin - Windows Build Tool
echo =======================================================

where go >nul 2>nul
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Go compiler not found in PATH! Please install Go 1.24+.
    pause
    exit /b 1
)

where gcc >nul 2>nul
if %ERRORLEVEL% neq 0 (
    echo [WARNING] GCC compiler not found in PATH.
    echo C-Shared dynamic libraries (.dll) require a C compiler (MinGW-w64).
    echo Please ensure MinGW-w64 is installed and added to PATH if build fails.
)

set CGO_ENABLED=1
if not exist "bin" mkdir "bin"

echo [INFO] Compiling bin\workbuddy.dll (C-Shared dynamic library)...
go build -buildmode=c-shared -ldflags="-s -w" -o bin\workbuddy.dll .\cmd\workbuddy-plugin

if %ERRORLEVEL% equ 0 (
    echo [SUCCESS] Successfully generated:
    echo   - bin\workbuddy.dll
    echo   - bin\workbuddy.h
    echo.
    echo Copy bin\workbuddy.dll to your CLIProxyAPI "plugins" directory to use.
) else (
    echo [ERROR] Build failed! Check compiler logs above.
)

pause
