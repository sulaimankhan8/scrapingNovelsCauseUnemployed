package com.novelarchiver.app;

import android.app.Activity;
import android.content.ContentResolver;
import android.content.ContentValues;
import android.net.Uri;
import android.os.Build;
import android.os.Environment;
import android.provider.MediaStore;
import android.webkit.JavascriptInterface;
import android.webkit.WebView;

import com.novelarchiver.core.mobile.Core;
import com.novelarchiver.core.mobile.Events;

import org.json.JSONObject;

import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.io.OutputStream;

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
                if ("ExportEPUB".equals(method)) {
                    payload = exportEPUB(new org.json.JSONArray(argsJson).getString(0));
                } else {
                    payload = core.call(method, argsJson);
                }
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

    /** Go writes the EPUB to the cache; it is then copied into Downloads. */
    private String exportEPUB(String slug) throws Exception {
        File tmpDir = new File(activity.getCacheDir(), "export");
        tmpDir.mkdirs();
        JSONObject res = new JSONObject(core.exportEPUB(slug, tmpDir.getAbsolutePath()));
        File tmp = new File(res.getString("path"));
        String name = res.getString("fileName");
        String shown;
        if (Build.VERSION.SDK_INT >= 29) {
            ContentValues v = new ContentValues();
            v.put(MediaStore.Downloads.DISPLAY_NAME, name);
            v.put(MediaStore.Downloads.MIME_TYPE, "application/epub+zip");
            v.put(MediaStore.Downloads.IS_PENDING, 1);
            ContentResolver cr = activity.getContentResolver();
            Uri uri = cr.insert(MediaStore.Downloads.EXTERNAL_CONTENT_URI, v);
            if (uri == null) throw new Exception("couldn't create the file in Downloads");
            try (InputStream in = new FileInputStream(tmp); OutputStream out = cr.openOutputStream(uri)) {
                copy(in, out);
            }
            v.clear();
            v.put(MediaStore.Downloads.IS_PENDING, 0);
            cr.update(uri, v, null, null);
            shown = "Downloads";
        } else {
            // Android 9 and older: app-specific Downloads folder (no permission needed).
            File dir = activity.getExternalFilesDir(Environment.DIRECTORY_DOWNLOADS);
            if (dir == null) throw new Exception("storage unavailable");
            File dest = new File(dir, name);
            try (InputStream in = new FileInputStream(tmp); OutputStream out = new FileOutputStream(dest)) {
                copy(in, out);
            }
            shown = dest.getParent();
        }
        tmp.delete();
        res.put("path", shown);
        return res.toString();
    }

    private static void copy(InputStream in, OutputStream out) throws Exception {
        byte[] buf = new byte[64 * 1024];
        int n;
        while ((n = in.read(buf)) > 0) out.write(buf, 0, n);
    }

    void close() {
        pool.shutdownNow();
    }
}
