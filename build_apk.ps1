Write-Host "=======================================================" -ForegroundColor Cyan
Write-Host "         NovelArchiver — Android APK Build" -ForegroundColor Cyan
Write-Host "=======================================================" -ForegroundColor Cyan
Write-Host ""

$env:JAVA_HOME = "C:\Program Files\Java\jdk-17"
$env:ANDROID_HOME = "C:\Users\Sulaiman\AppData\Local\Android\Sdk"
$gradleBat = "C:\Users\Sulaiman\.gradle\wrapper\dists\gradle-8.14.3-all\10utluxaxniiv4wxiphsi49nj\gradle-8.14.3\bin\gradle.bat"

Write-Host "[BUILD] Compiling Android APK with Gradle..." -ForegroundColor Cyan
& $gradleBat -p android assembleDebug

if ($LASTEXITCODE -eq 0 -and (Test-Path "android\app\build\outputs\apk\debug\app-debug.apk")) {
    Copy-Item "android\app\build\outputs\apk\debug\app-debug.apk" "NovelArchiver.apk" -Force
    $size = (Get-Item "NovelArchiver.apk").Length
    $sizeMb = [math]::Round($size / 1MB, 2)
    Write-Host ""
    Write-Host "=======================================================" -ForegroundColor Green
    Write-Host "[SUCCESS] Android APK built successfully!" -ForegroundColor Green
    Write-Host "Output: NovelArchiver.apk ($sizeMb MB)" -ForegroundColor Green
    Write-Host "=======================================================" -ForegroundColor Green
} else {
    Write-Host "[ERROR] Android APK build failed!" -ForegroundColor Red
    exit 1
}
