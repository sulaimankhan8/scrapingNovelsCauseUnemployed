@echo off
setlocal

echo =======================================================
echo          NovelArchiver - Android APK Build
echo =======================================================
echo.

set "JAVA_HOME=C:\Program Files\Java\jdk-17"
set "ANDROID_HOME=C:\Users\Sulaiman\AppData\Local\Android\Sdk"
set "GRADLE_BAT=C:\Users\Sulaiman\.gradle\wrapper\dists\gradle-8.14.3-all\10utluxaxniiv4wxiphsi49nj\gradle-8.14.3\bin\gradle.bat"

echo [BUILD] Compiling Android APK with Gradle...
call "%GRADLE_BAT%" -p android assembleDebug

if %ERRORLEVEL% neq 0 (
    echo [ERROR] Gradle build failed!
    exit /b 1
)

if exist "android\app\build\outputs\apk\debug\app-debug.apk" (
    copy /y "android\app\build\outputs\apk\debug\app-debug.apk" "NovelArchiver.apk" >nul
    echo.
    echo =======================================================
    echo [SUCCESS] Android APK built successfully!
    echo Output: NovelArchiver.apk
    echo =======================================================
) else (
    echo [ERROR] Output APK not found!
)

echo.
