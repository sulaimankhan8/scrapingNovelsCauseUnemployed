# Builds the Android app.
#   .\build_apk.ps1                 debug APK (phones + emulators)          -> NovelArchiver.apk
#   .\build_apk.ps1 -Release        signed, shrunk APK for phones           -> NovelArchiver-release.apk
#   .\build_apk.ps1 -Release -DebugSign   same, signed with the debug key (local testing only)
# Release signing reads android\keystore.properties; see README "Android release".
param(
    [switch]$Release,
    [switch]$DebugSign
)

Write-Host "=======================================================" -ForegroundColor Cyan
Write-Host "         NovelArchiver - Android APK Build" -ForegroundColor Cyan
Write-Host "=======================================================" -ForegroundColor Cyan
Write-Host ""

# Tool locations (override with your own env vars if different)
if (-not $env:JAVA_HOME) { $env:JAVA_HOME = "C:\Program Files\Eclipse Adoptium\jdk-17.0.6.10-hotspot" }
if (-not $env:ANDROID_HOME) { $env:ANDROID_HOME = "$env:LOCALAPPDATA\Android\Sdk" }
if (-not $env:ANDROID_NDK_HOME) {
    $ndk = Get-ChildItem "$env:ANDROID_HOME\ndk" -Directory -ErrorAction SilentlyContinue | Sort-Object Name -Descending | Select-Object -First 1
    if (-not $ndk) {
        Write-Host "[ERROR] No Android NDK found. Install one: sdkmanager ndk/29.0.14206865" -ForegroundColor Red
        exit 1
    }
    $env:ANDROID_NDK_HOME = $ndk.FullName
}
$gomobile = "$HOME\go\bin\gomobile.exe"
if (-not (Test-Path $gomobile)) {
    Write-Host "[SETUP] Installing gomobile..." -ForegroundColor Yellow
    go install golang.org/x/mobile/cmd/gomobile@latest
    go install golang.org/x/mobile/cmd/gobind@latest
}
$env:PATH = "$HOME\go\bin;$env:PATH"
# Low-memory machines: compile two packages at a time.
if (-not $env:GOFLAGS) { $env:GOFLAGS = "-p=2" }

if ($Release -and -not $DebugSign -and -not (Test-Path "android\keystore.properties")) {
    Write-Host "[ERROR] Release builds need your signing key: create android\keystore.properties (see README, 'Android release')." -ForegroundColor Red
    Write-Host "        For a local test without a key, add -DebugSign." -ForegroundColor Yellow
    exit 1
}

# 1. Go core -> android/app/libs/core.aar (same engine as the desktop app)
if ($Release) {
    # Real phones only (64- and 32-bit ARM); debug info stripped.
    $targets = "android/arm64,android/arm"
    $ldflags = "-s -w"
} else {
    # Phones + x86_64 emulators.
    $targets = "android/arm64,android/amd64"
    $ldflags = ""
}
Write-Host "[BUILD] Go core for $targets (gomobile bind)..." -ForegroundColor Cyan
New-Item -ItemType Directory -Force "android\app\libs" | Out-Null
& $gomobile bind -target="$targets" -androidapi 24 -ldflags="$ldflags" -javapkg com.novelarchiver.core -o android\app\libs\core.aar ./mobile
if ($LASTEXITCODE -ne 0) {
    Write-Host "[ERROR] gomobile bind failed!" -ForegroundColor Red
    exit 1
}

# 2. APK
$gradleBat = Get-ChildItem "$HOME\.gradle\wrapper\dists" -Recurse -Filter gradle.bat -ErrorAction SilentlyContinue | Sort-Object FullName -Descending | Select-Object -First 1 -ExpandProperty FullName
if (-not $gradleBat) {
    Write-Host "[ERROR] Gradle not found under ~/.gradle/wrapper/dists" -ForegroundColor Red
    exit 1
}
if ($Release) {
    $task = "assembleRelease"
    $extra = @()
    if ($DebugSign) { $extra += "-PdebugSign" }
    $apk = "android\app\build\outputs\apk\release\app-release.apk"
    $out = "NovelArchiver-release.apk"
} else {
    $task = "assembleDebug"
    $extra = @()
    $apk = "android\app\build\outputs\apk\debug\app-debug.apk"
    $out = "NovelArchiver.apk"
}
Write-Host "[BUILD] Android APK ($task)..." -ForegroundColor Cyan
& $gradleBat -p android $task @extra

if ($LASTEXITCODE -eq 0 -and (Test-Path $apk)) {
    Copy-Item $apk $out -Force
    $sizeMb = [math]::Round((Get-Item $out).Length / 1MB, 2)
    Write-Host ""
    Write-Host "[SUCCESS] $out ($sizeMb MB)" -ForegroundColor Green
    if ($Release -and $DebugSign) {
        Write-Host "          Signed with the DEBUG key: fine for testing, not for sharing." -ForegroundColor Yellow
    }
} else {
    Write-Host "[ERROR] Android APK build failed!" -ForegroundColor Red
    exit 1
}
