# NovelArchiver ProGuard / R8 rules

-keepattributes *Annotation*

# Methods called from page JavaScript (NovelBridge.call, SiteBrowser's
# __naSB.result) are found by name at runtime.
-keepclassmembers class * {
    @android.webkit.JavascriptInterface <methods>;
}

# gomobile: the Go library calls these Java classes and methods through JNI
# by name, so R8 must not rename or remove them.
-keep class go.** { *; }
-keep class com.novelarchiver.core.** { *; }

# Our implementations of the Go interfaces (Events, BrowserDriver) are
# invoked from Go via JNI.
-keep class com.novelarchiver.app.NovelBridge { *; }
-keep class com.novelarchiver.app.SiteBrowser { *; }

# Any class implementing a Go interface (e.g. the Wi-Fi check passed to
# Core.setNetwork) is called from Go through JNI.
-keep class * implements com.novelarchiver.core.mobile.Network { *; }
-keep class * implements com.novelarchiver.core.mobile.Events { *; }
-keep class * implements com.novelarchiver.core.mobile.BrowserDriver { *; }
