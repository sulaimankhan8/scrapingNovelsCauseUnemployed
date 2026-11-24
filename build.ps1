Write-Host "=======================================================" -ForegroundColor Cyan
Write-Host "          InkShelfs - Production Build" -ForegroundColor Cyan
Write-Host "=======================================================" -ForegroundColor Cyan
Write-Host ""

# 1. Verify Go
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host "[ERROR] Go compiler not found on PATH! Install from https://go.dev/dl/" -ForegroundColor Red
    exit 1
}
$goVer = go version
Write-Host "[OK] Go compiler: $goVer" -ForegroundColor Green

# 2. Verify GCC for CGo
if (-not (Get-Command gcc -ErrorAction SilentlyContinue)) {
    Write-Host "[ERROR] GCC compiler not found on PATH! Install MSYS2 or MinGW-w64." -ForegroundColor Red
    exit 1
}
Write-Host "[OK] GCC CGo compiler detected." -ForegroundColor Green

# 3. Locate Wails CLI
$wailsCmd = "wails"
if (-not (Get-Command wails -ErrorAction SilentlyContinue)) {
    $userWails = "$HOME\go\bin\wails.exe"
    if (Test-Path $userWails) {
        $wailsCmd = $userWails
    } else {
        Write-Host "[INFO] Wails CLI not found. Installing via go install..." -ForegroundColor Yellow
        go install github.com/wailsapp/wails/v2/cmd/wails@latest
        if (Test-Path $userWails) {
            $wailsCmd = $userWails
        } else {
            Write-Host "[ERROR] Could not install Wails CLI!" -ForegroundColor Red
            exit 1
        }
    }
}
Write-Host "[OK] Using Wails CLI: $wailsCmd" -ForegroundColor Green
Write-Host ""
Write-Host "[BUILD] Compiling desktop binary with Wails..." -ForegroundColor Cyan

& $wailsCmd build -clean

if ($LASTEXITCODE -eq 0 -and (Test-Path "build\bin\InkShelfs.exe")) {
    Copy-Item "build\bin\InkShelfs.exe" "InkShelfs.exe" -Force
    $size = (Get-Item "InkShelfs.exe").Length
    $sizeMb = [math]::Round($size / 1MB, 2)
    Write-Host ""
    Write-Host "=======================================================" -ForegroundColor Green
    Write-Host "[SUCCESS] Binary compiled successfully!" -ForegroundColor Green
    Write-Host "Output: InkShelfs.exe ($sizeMb MB)" -ForegroundColor Green
    Write-Host "=======================================================" -ForegroundColor Green
} else {
    Write-Host "[ERROR] Build failed. Check compiler errors above." -ForegroundColor Red
    exit 1
}
