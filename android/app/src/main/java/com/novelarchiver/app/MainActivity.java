package com.novelarchiver.app;

import android.annotation.SuppressLint;
import android.content.Intent;
import android.net.Uri;
import android.os.Bundle;
import android.view.View;
import android.webkit.WebResourceRequest;
import android.webkit.WebResourceResponse;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import androidx.annotation.NonNull;
import androidx.appcompat.app.AppCompatActivity;
import androidx.webkit.WebViewAssetLoader;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.io.OutputStream;

public class MainActivity extends AppCompatActivity {

    private WebView webView;
    private NovelBridge novelBridge;
    private WebViewAssetLoader assetLoader;

    @SuppressLint("SetJavaScriptEnabled")
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        // Ensure novels directory exists in internal storage
        File novelsDir = new File(getFilesDir(), "novels");
        if (!novelsDir.exists()) {
            novelsDir.mkdirs();
        }
        if (novelsDir.list() == null || novelsDir.list().length == 0) {
            copyAssetFolder("novels", novelsDir);
        }

        webView = new WebView(this);
        setContentView(webView);

        WebSettings settings = webView.getSettings();
        settings.setJavaScriptEnabled(true);
        settings.setDomStorageEnabled(true);
        settings.setDatabaseEnabled(true);
        settings.setAllowFileAccess(true);
        settings.setAllowContentAccess(true);
        settings.setLoadsImagesAutomatically(true);
        settings.setMediaPlaybackRequiresUserGesture(false);

        // Dark background matching bibliotheca theme
        webView.setBackgroundColor(0xFF0E1216);

        // Configure modern in-process WebViewAssetLoader (Zero open ports, zero localhost)
        assetLoader = new WebViewAssetLoader.Builder()
                .setDomain("appassets.androidplatform.net")
                .addPathHandler("/assets/", new WebViewAssetLoader.AssetsPathHandler(this))
                .addPathHandler("/novels/", new WebViewAssetLoader.InternalStoragePathHandler(this, novelsDir))
                .build();

        webView.setWebViewClient(new WebViewClient() {
            @Override
            public WebResourceResponse shouldInterceptRequest(WebView view, WebResourceRequest request) {
                Uri url = request.getUrl();
                String path = url.getPath();

                // Custom handler for /novels/... requests from relative reader links
                if (path != null && path.startsWith("/novels/")) {
                    File file = new File(novelsDir, path.substring("/novels/".length()));
                    if (file.exists() && file.isFile()) {
                        try {
                            String mime = "text/html";
                            if (path.endsWith(".css")) mime = "text/css";
                            else if (path.endsWith(".js")) mime = "application/javascript";
                            else if (path.endsWith(".json")) mime = "application/json";
                            else if (path.endsWith(".png")) mime = "image/png";
                            else if (path.endsWith(".jpg") || path.endsWith(".jpeg")) mime = "image/jpeg";
                            return new WebResourceResponse(mime, "UTF-8", new FileInputStream(file));
                        } catch (Exception ignored) {}
                    }
                }

                return assetLoader.shouldInterceptRequest(request.getUrl());
            }

            @Override
            public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) {
                String url = request.getUrl().toString();
                if (url.startsWith("https://appassets.androidplatform.net/")) {
                    return false; // Load inside WebView
                }
                // External links open in default browser
                try {
                    Intent intent = new Intent(Intent.ACTION_VIEW, Uri.parse(url));
                    startActivity(intent);
                } catch (Exception ignored) {}
                return true;
            }
        });

        novelBridge = new NovelBridge(this, webView, novelsDir);
        webView.addJavascriptInterface(novelBridge, "AndroidBridge");

        // Load embedded responsive frontend
        webView.loadUrl("https://appassets.androidplatform.net/assets/frontend/index.html");
    }

    @Override
    public void onBackPressed() {
        if (webView.canGoBack()) {
            webView.goBack();
        } else {
            super.onBackPressed();
        }
    }

    private void copyAssetFolder(String assetSubDir, File targetDir) {
        try {
            String[] files = getAssets().list(assetSubDir);
            if (files != null && files.length > 0) {
                targetDir.mkdirs();
                for (String file : files) {
                    String subPath = assetSubDir + "/" + file;
                    File outFile = new File(targetDir, file);
                    String[] subFiles = getAssets().list(subPath);
                    if (subFiles != null && subFiles.length > 0) {
                        copyAssetFolder(subPath, outFile);
                    } else {
                        try (InputStream in = getAssets().open(subPath);
                             OutputStream out = new FileOutputStream(outFile)) {
                            byte[] buffer = new byte[8192];
                            int read;
                            while ((read = in.read(buffer)) != -1) {
                                out.write(buffer, 0, read);
                            }
                        } catch (Exception ignored) {}
                    }
                }
            }
        } catch (Exception ignored) {}
    }

    @Override
    protected void onDestroy() {
        if (novelBridge != null) {
            novelBridge.cancelTask();
        }
        if (webView != null) {
            webView.destroy();
        }
        super.onDestroy();
    }
}
