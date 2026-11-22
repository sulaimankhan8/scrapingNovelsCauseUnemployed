package com.novelarchiver.app;

import android.Manifest;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.content.ContentResolver;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.content.ContentValues;
import android.net.Uri;
import android.os.Build;
import android.os.Environment;
import android.provider.MediaStore;
import android.webkit.JavascriptInterface;
import android.webkit.WebView;

import androidx.core.app.NotificationCompat;
import androidx.core.app.NotificationManagerCompat;
import androidx.core.content.ContextCompat;

import com.novelarchiver.core.mobile.Core;
import com.novelarchiver.core.mobile.Events;

import org.json.JSONObject;

import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.io.OutputStream;

import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

/**
 * JavaScript <-> Go bridge. The page calls NovelBridge.call(id, method,
 * argsJson); the Go core runs on a background thread and the result comes
 * back through window.__naResolve. Engine events go to window.__naEmit.
 */
public class NovelBridge implements Events {
    private static final String CHANNEL = "downloads";
    private final MainActivity activity;
    private final WebView webView;
    private final ExecutorService pool = Executors.newFixedThreadPool(4);
    private volatile Core core;

    NovelBridge(MainActivity activity, WebView webView) {
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
                } else if ("BackupLibrary".equals(method)) {
                    payload = backupLibrary();
                } else if ("ChooseBackup".equals(method)) {
                    payload = chooseBackup();
                } else if ("Notify".equals(method)) {
                    org.json.JSONArray a = new org.json.JSONArray(argsJson);
                    notify(a.optString(0), a.optString(1));
                    payload = "";
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
        res.put("path", toDownloads(tmp, res.getString("fileName"), "application/epub+zip"));
        return res.toString();
    }

    /** The whole library as one zip in Downloads. */
    private String backupLibrary() throws Exception {
        File tmpDir = new File(activity.getCacheDir(), "backup");
        tmpDir.mkdirs();
        JSONObject res = new JSONObject(core.backup(tmpDir.getAbsolutePath()));
        File tmp = new File(res.getString("path"));
        res.put("path", toDownloads(tmp, res.getString("fileName"), "application/zip"));
        return res.toString();
    }

    /**
     * Lets the user pick a backup file, copies it into the cache and asks the
     * core what restoring would do. "" means the picker was cancelled.
     */
    private String chooseBackup() throws Exception {
        CompletableFuture<Uri> picked = new CompletableFuture<>();
        activity.runOnUiThread(() -> activity.pickBackup(picked));
        Uri uri = picked.get();
        if (uri == null) return "";
        File dir = new File(activity.getCacheDir(), "restore");
        dir.mkdirs();
        File dest = new File(dir, "backup.zip");
        try (InputStream in = activity.getContentResolver().openInputStream(uri);
             OutputStream out = new FileOutputStream(dest)) {
            if (in == null) throw new Exception("couldn't open that file");
            copy(in, out);
        }
        return core.call("InspectBackup", new org.json.JSONArray().put(dest.getAbsolutePath()).toString());
    }

    /** A system notification; tapping it opens the app. Asks for permission once on Android 13+. */
    private void notify(String title, String body) {
        if (!core.notifyEnabled()) return;
        if (Build.VERSION.SDK_INT >= 33 && ContextCompat.checkSelfPermission(activity,
                Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
            activity.runOnUiThread(activity::askNotificationPermission);
            return;
        }
        if (Build.VERSION.SDK_INT >= 26) {
            NotificationManager nm = activity.getSystemService(NotificationManager.class);
            if (nm != null && nm.getNotificationChannel(CHANNEL) == null) {
                nm.createNotificationChannel(new NotificationChannel(CHANNEL, "Downloads and updates",
                        NotificationManager.IMPORTANCE_DEFAULT));
            }
        }
        Intent open = new Intent(activity, MainActivity.class).addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP);
        PendingIntent pi = PendingIntent.getActivity(activity, 0, open,
                PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
        NotificationCompat.Builder b = new NotificationCompat.Builder(activity, CHANNEL)
                .setSmallIcon(android.R.drawable.stat_sys_download_done)
                .setContentTitle(title)
                .setContentText(body)
                .setStyle(new NotificationCompat.BigTextStyle().bigText(body))
                .setContentIntent(pi)
                .setAutoCancel(true);
        try {
            NotificationManagerCompat.from(activity).notify((int) (System.currentTimeMillis() & 0x7fffffff), b.build());
        } catch (SecurityException ignored) {
        }
    }

    /** Copies a finished file into the phone's Downloads folder; returns where it went. */
    private String toDownloads(File tmp, String name, String mime) throws Exception {
        String shown;
        if (Build.VERSION.SDK_INT >= 29) {
            ContentValues v = new ContentValues();
            v.put(MediaStore.Downloads.DISPLAY_NAME, name);
            v.put(MediaStore.Downloads.MIME_TYPE, mime);
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
        return shown;
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
