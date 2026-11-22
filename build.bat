@echo off
setlocal

echo =======================================================
echo           NovelArchiver - Production Build
echo =======================================================
echo.

echo [CHECK] Verifying Go...
where go >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Go compiler not found on PATH!
    echo Please install Go 1.22+ from https://go.dev/dl/
    exit /b 1
)
go version

echo.
echo [CHECK] Verifying GCC for CGo...
where gcc >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [ERROR] GCC C-compiler not found on PATH!
    echo Wails requires GCC for CGo linking on Windows.
    echo Please install MinGW-w64 or MSYS2.
    exit /b 1
)
echo [OK] GCC compiler detected.

echo.
echo [CHECK] Locating Wails CLI...
set "WAILS_CMD=wails"
where wails >nul 2>&1
if %ERRORLEVEL% neq 0 (
    if exist "%USERPROFILE%\go\bin\wails.exe" (
        set "WAILS_CMD=%USERPROFILE%\go\bin\wails.exe"
    ) else (
        echo [INFO] Wails CLI not found on PATH. Attempting automatic installation...
        go install github.com/wailsapp/wails/v2/cmd/wails@latest
        if exist "%USERPROFILE%\go\bin\wails.exe" (
            set "WAILS_CMD=%USERPROFILE%\go\bin\wails.exe"
        ) else (
            echo [ERROR] Failed to install or locate Wails CLI!
            exit /b 1
        )
    )
)
echo [OK] Using Wails CLI: %WAILS_CMD%

echo.
echo [BUILD] Compiling desktop binary with Wails...
echo [BUILD] Embedding frontend assets and linking Windows WebView2 runtime...
echo.

"%WAILS_CMD%" build -clean

if %ERRORLEVEL% neq 0 (
    echo.
    echo [ERROR] Build failed! Check compiler errors above.
    exit /b 1
)

if exist "build\bin\NovelArchiver.exe" (
    copy /y "build\bin\NovelArchiver.exe" "NovelArchiver.exe" >nul
    echo.
    echo =======================================================
    echo [SUCCESS] Binary built successfully!
    echo Output: NovelArchiver.exe and build\bin\NovelArchiver.exe
    echo =======================================================
) else (
    echo [ERROR] Output binary build\bin\NovelArchiver.exe not found!
)

echo.
