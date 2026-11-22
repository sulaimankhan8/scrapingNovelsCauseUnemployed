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
# Low-memory machines: compile one package at a time.
if (-not $env:GOFLAGS) { $env:GOFLAGS = "-p=2" }

# 1. Go core -> android/app/libs/core.aar (same engine as the desktop app)
Write-Host "[BUILD] Go core (gomobile bind)..." -ForegroundColor Cyan
New-Item -ItemType Directory -Force "android\app\libs" | Out-Null
& $gomobile bind -target=android/arm64,android/amd64 -androidapi 24 -javapkg com.novelarchiver.core -o android\app\libs\core.aar ./mobile
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
Write-Host "[BUILD] Android APK (Gradle)..." -ForegroundColor Cyan
& $gradleBat -p android assembleDebug

if ($LASTEXITCODE -eq 0 -and (Test-Path "android\app\build\outputs\apk\debug\app-debug.apk")) {
    Copy-Item "android\app\build\outputs\apk\debug\app-debug.apk" "NovelArchiver.apk" -Force
    $sizeMb = [math]::Round((Get-Item "NovelArchiver.apk").Length / 1MB, 2)
    Write-Host ""
    Write-Host "[SUCCESS] NovelArchiver.apk ($sizeMb MB)" -ForegroundColor Green
} else {
    Write-Host "[ERROR] Android APK build failed!" -ForegroundColor Red
    exit 1
}
