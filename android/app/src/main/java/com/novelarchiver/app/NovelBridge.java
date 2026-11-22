package com.novelarchiver.app;

import android.app.Activity;
import android.webkit.JavascriptInterface;
import android.webkit.WebView;

import com.novelarchiver.core.mobile.Core;
import com.novelarchiver.core.mobile.Events;

import org.json.JSONObject;

import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

/**
 * JavaScript <-> Go bridge. The page calls NovelBridge.call(id, method,
 * argsJson); the Go core runs on a background thread and the result comes
 * back through window.__naResolve. Engine events go to window.__naEmit.
 */
public class NovelBridge implements Events {
    private final Activity activity;
    private final WebView webView;
    private final ExecutorService pool = Executors.newFixedThreadPool(4);
    private volatile Core core;

    NovelBridge(Activity activity, WebView webView) {
        this.activity = activity;
        this.webView = webView;
    }

    void setCore(Core core) {
        this.core = core;
    }

    @JavascriptInterface
    public void call(final int id, final String method, final String argsJson) {
        pool.execute(() -> {
            boolean ok;
            String payload;
            try {
                if (core == null) throw new IllegalStateException("The library could not start.");
                payload = core.call(method, argsJson);
                ok = true;
            } catch (Exception e) {
                ok = false;
                payload = e.getMessage() == null ? e.toString() : e.getMessage();
            }
            String js = "window.__naResolve && window.__naResolve(" + id + "," + ok + ","
                    + JSONObject.quote(payload) + ")";
            activity.runOnUiThread(() -> webView.evaluateJavascript(js, null));
        });
    }

    /** Called by the Go core (any thread). */
    @Override
    public void emit(String event, String payloadJSON) {
        String js = "window.__naEmit && window.__naEmit(" + JSONObject.quote(event) + ","
                + JSONObject.quote(payloadJSON) + ")";
        activity.runOnUiThread(() -> webView.evaluateJavascript(js, null));
    }

    void close() {
        pool.shutdownNow();
    }
}
