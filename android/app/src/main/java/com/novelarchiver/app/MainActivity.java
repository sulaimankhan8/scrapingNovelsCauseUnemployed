package com.novelarchiver.app;

import android.Manifest;
import android.annotation.SuppressLint;
import android.content.Intent;
import android.net.ConnectivityManager;
import android.net.Uri;
import android.os.Bundle;
import android.util.Log;
import android.view.Gravity;
import android.view.View;
import android.view.ViewGroup;
import android.webkit.MimeTypeMap;
import android.webkit.WebResourceRequest;
import android.webkit.WebResourceResponse;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.Button;
import android.widget.FrameLayout;
import android.widget.LinearLayout;
import android.widget.TextView;

import androidx.activity.OnBackPressedCallback;
import androidx.activity.result.ActivityResultLauncher;
import androidx.activity.result.contract.ActivityResultContracts;
import androidx.appcompat.app.AppCompatActivity;
import androidx.core.graphics.Insets;
import androidx.core.view.ViewCompat;
import androidx.core.view.WindowInsetsCompat;

import com.novelarchiver.core.mobile.Core;
import com.novelarchiver.core.mobile.Mobile;
import com.novelarchiver.core.mobile.Network;
import com.novelarchiver.core.mobile.Response;

import java.io.ByteArrayInputStream;
import java.io.File;
import java.io.InputStream;
import java.util.HashMap;
import java.util.Map;
import java.util.concurrent.CompletableFuture;

/**
 * Hosts the shared frontend in a WebView. All app logic runs in the Go core
 * (the same code as the desktop app): /library/... reader pages are rendered
 * by core.serve(), and page calls go through NovelBridge to core.call().
 */
public class MainActivity extends AppCompatActivity {
    static final String HOST = "appassets.androidplatform.net";
    private static final String TAG = "NovelArchiver";

    private WebView webView;
    private FrameLayout siteBrowserPanel;
    private SiteBrowser siteBrowser;
    private NovelBridge bridge;
    private Core core;
    private CompletableFuture<Uri> pendingPick;
    private final ActivityResultLauncher<String[]> openBackup =
            registerForActivityResult(new ActivityResultContracts.OpenDocument(), uri -> {
                if (pendingPick != null) {
                    pendingPick.complete(uri);
                    pendingPick = null;
                }
            });
    private final ActivityResultLauncher<String> notifyPermission =
            registerForActivityResult(new ActivityResultContracts.RequestPermission(), granted -> { });
    private boolean askedNotify;

    /** NovelBridge asks for a backup file to restore. */
    void pickBackup(CompletableFuture<Uri> result) {
        if (pendingPick != null) pendingPick.complete(null);
        pendingPick = result;
        try {
            openBackup.launch(new String[]{"application/zip", "application/x-zip-compressed", "application/octet-stream"});
        } catch (Exception e) {
            pendingPick = null;
            result.complete(null);
        }
    }

    /** Android 13+: ask once for permission to post notifications. */
    void askNotificationPermission() {
        if (askedNotify || android.os.Build.VERSION.SDK_INT < 33) return;
        askedNotify = true;
        notifyPermission.launch(Manifest.permission.POST_NOTIFICATIONS);
    }

    @SuppressLint({"SetJavaScriptEnabled", "AddJavascriptInterface"})
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        // Debug builds are inspectable automatically; release test builds
        // made with -PdebugSign turn it on through this resource.
        if (getResources().getBoolean(R.bool.webview_inspectable)) {
            WebView.setWebContentsDebuggingEnabled(true);
        }
        FrameLayout root = new FrameLayout(this);
        webView = new WebView(this);
        webView.setBackgroundColor(0xFF121417);
        root.addView(webView, new FrameLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));

        // Site browser overlay: hidden until a site needs the user (human check).
        siteBrowserPanel = new FrameLayout(this);
        siteBrowserPanel.setVisibility(View.GONE);
        siteBrowserPanel.setBackgroundColor(0xFF121417);
        LinearLayout column = new LinearLayout(this);
        column.setOrientation(LinearLayout.VERTICAL);
        LinearLayout bar = new LinearLayout(this);
        bar.setGravity(Gravity.CENTER_VERTICAL);
        bar.setPadding(24, 12, 12, 12);
        TextView label = new TextView(this);
        label.setText("Tick the check below. We’ll continue as soon as it passes.");
        label.setTextColor(0xFFE7E2D9);
        bar.addView(label, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f));
        Button hide = new Button(this);
        hide.setText("Done");
        hide.setOnClickListener(v -> siteBrowserPanel.setVisibility(View.GONE));
        bar.addView(hide);
        column.addView(bar);
        WebView siteView = new WebView(this);
        column.addView(siteView, new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, 0, 1f));
        siteBrowserPanel.addView(column);
        root.addView(siteBrowserPanel, new FrameLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));
        setContentView(root);
        // Android 15+ draws edge-to-edge: keep content clear of the status
        // and navigation bars.
        ViewCompat.setOnApplyWindowInsetsListener(root, (v, insets) -> {
            Insets bars = insets.getInsets(WindowInsetsCompat.Type.systemBars()
                    | WindowInsetsCompat.Type.displayCutout() | WindowInsetsCompat.Type.ime());
            v.setPadding(bars.left, bars.top, bars.right, bars.bottom);
            return WindowInsetsCompat.CONSUMED;
        });

        siteBrowser = new SiteBrowser(this, siteView, siteBrowserPanel);

        WebSettings s = webView.getSettings();
        s.setJavaScriptEnabled(true);
        s.setDomStorageEnabled(true);
        s.setAllowFileAccess(false);
        s.setAllowContentAccess(false);

        bridge = new NovelBridge(this, webView);
        try {
            File library = new File(getFilesDir(), "library");
            core = Mobile.new_(library.getAbsolutePath(), bridge, siteBrowser);
            bridge.setCore(core);
            // "Only on Wi-Fi": downloads pause while the connection is metered.
            final ConnectivityManager cm = getSystemService(ConnectivityManager.class);
            core.setNetwork(new Network() {
                @Override
                public boolean unmetered() {
                    return cm == null || !cm.isActiveNetworkMetered();
                }
            });
        } catch (Exception e) {
            Log.e(TAG, "core start failed", e);
        }
        webView.addJavascriptInterface(bridge, "NovelBridge");

        webView.setWebViewClient(new WebViewClient() {
            @Override
            public WebResourceResponse shouldInterceptRequest(WebView view, WebResourceRequest request) {
                Uri url = request.getUrl();
                if (!HOST.equals(url.getHost())) return null;
                String path = url.getPath() == null ? "/" : url.getPath();
                if (path.startsWith("/library/")) {
                    return serveLibrary(path);
                }
                return serveAsset(path);
            }

            @Override
            public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) {
                Uri url = request.getUrl();
                if (HOST.equals(url.getHost())) return false;
                try {
                    startActivity(new Intent(Intent.ACTION_VIEW, url));
                } catch (Exception ignored) {
                }
                return true;
            }
        });

        getOnBackPressedDispatcher().addCallback(this, new OnBackPressedCallback(true) {
            @Override
            public void handleOnBackPressed() {
                if (siteBrowserPanel.getVisibility() == View.VISIBLE) {
                    siteBrowserPanel.setVisibility(View.GONE);
                } else if (webView.canGoBack()) {
                    webView.goBack();
                } else {
                    finish();
                }
            }
        });

        webView.loadUrl("https://" + HOST + "/");
    }

    private WebResourceResponse serveLibrary(String path) {
        if (core == null) return text(503, "The library could not start.");
        try {
            Response r = core.serve(path);
            WebResourceResponse resp = new WebResourceResponse(r.getMimeType(), "UTF-8",
                    new ByteArrayInputStream(r.getBody()));
            resp.setStatusCodeAndReasonPhrase((int) r.getStatus(), r.getStatus() == 200 ? "OK" : "Error");
            return resp;
        } catch (Exception e) {
            return text(500, e.toString());
        }
    }

    /** The shared frontend/ folder is packaged as the APK's asset root. */
    private WebResourceResponse serveAsset(String path) {
        String asset = path.equals("/") ? "index.html" : path.substring(1);
        if (asset.contains("..")) return text(404, "Not found");
        try {
            InputStream in = getAssets().open(asset);
            return new WebResourceResponse(mimeFor(asset), "UTF-8", in);
        } catch (Exception e) {
            return text(404, "Not found");
        }
    }

    private static final Map<String, String> MIME = new HashMap<>();
    static {
        MIME.put("html", "text/html");
        MIME.put("js", "application/javascript");
        MIME.put("css", "text/css");
        MIME.put("json", "application/json");
        MIME.put("svg", "image/svg+xml");
    }

    private static String mimeFor(String name) {
        String ext = MimeTypeMap.getFileExtensionFromUrl(name);
        String m = MIME.get(ext);
        if (m == null) m = MimeTypeMap.getSingleton().getMimeTypeFromExtension(ext);
        return m == null ? "application/octet-stream" : m;
    }

    private static WebResourceResponse text(int code, String msg) {
        WebResourceResponse r = new WebResourceResponse("text/plain", "UTF-8",
                new ByteArrayInputStream(msg.getBytes()));
        r.setStatusCodeAndReasonPhrase(code, code == 404 ? "Not Found" : "Error");
        return r;
    }

    @Override
    protected void onDestroy() {
        if (core != null) core.shutdown();
        if (bridge != null) bridge.close();
        if (webView != null) webView.destroy();
        super.onDestroy();
    }
}
