package com.novelarchiver.app;

import android.Manifest;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.content.ActivityNotFoundException;
import android.content.ClipData;
import android.content.ContentResolver;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.content.ContentValues;
import android.net.Uri;
import android.os.Build;
import android.os.Environment;
import android.os.SystemClock;
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
import java.util.concurrent.ExecutionException;
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
        DownloadService.onPause = () -> {
            try {
                core.call("CancelTask", "[]");
            } catch (Exception ignored) {
            }
            try {
                core.call("Manga.CancelCurrent", "[]");
            } catch (Exception ignored) {
            }
        };
    }

    // ---- background service: follows the queue so downloads keep going
    // with the app in the background or the screen off.
    private volatile String jobTitle = "";
    private volatile String jobKind = "download";
    private volatile int queued = 0;
    private long lastShown = 0;
    // Novels and manga download independently; the service stays up while
    // either is busy.
    private volatile boolean novelBusy = false;
    private volatile boolean mangaBusy = false;
    private volatile int mangaQueued = 0;

    private void stopIfIdle() {
        if (!novelBusy && !mangaBusy) DownloadService.stop(activity);
    }

    private void track(String event, String json) {
        try {
            switch (event) {
                case "manga:queue": {
                    JSONObject a = new JSONObject(json);
                    JSONObject cur = a.optJSONObject("current");
                    org.json.JSONArray q = a.optJSONArray("queue");
                    mangaQueued = q == null ? 0 : q.length();
                    mangaBusy = cur != null || mangaQueued > 0;
                    if (!mangaBusy) {
                        stopIfIdle();
                        return;
                    }
                    if (cur != null && !novelBusy) {
                        jobTitle = cur.optString("title", "");
                        jobKind = cur.optString("kind", "download");
                        show("update".equals(jobKind) ? "Checking for new chapters…" : "Getting ready…", 0, 0, true);
                    }
                    break;
                }
                case "manga:progress": {
                    if (novelBusy) break; // the novel download owns the notification
                    JSONObject j = new JSONObject(json);
                    jobTitle = j.optString("title", jobTitle);
                    int done = j.optInt("done"), total = j.optInt("total");
                    String msg = j.optString("message", "");
                    String text = "update".equals(j.optString("kind"))
                            ? "Checking for new chapters…"
                            : (total > 0 ? "Manga · " + done + " of " + total + " chapters" : "Manga") + (msg.isEmpty() ? "" : " · " + msg);
                    show(text, done, total, false);
                    break;
                }
                case "manga:challenge":
                    show("Needs you: open the app and tick the check", 0, 0, true);
                    break;
                case "queue": {
                    JSONObject a = new JSONObject(json);
                    JSONObject cur = a.optJSONObject("current");
                    org.json.JSONArray q = a.optJSONArray("queue");
                    queued = q == null ? 0 : q.length();
                    novelBusy = cur != null || queued > 0;
                    if (!novelBusy) {
                        stopIfIdle();
                        return;
                    }
                    if (cur != null) {
                        jobTitle = cur.optString("title", "");
                        jobKind = cur.optString("kind", "download");
                    }
                    show("update".equals(jobKind) ? "Checking for new chapters…" : "Getting ready…", 0, 0, true);
                    break;
                }
                case "started": {
                    JSONObject d = new JSONObject(json);
                    if (!d.optString("title").isEmpty()) jobTitle = d.optString("title");
                    show("update".equals(jobKind) ? "Checking for new chapters…" : "Starting…", 0, 0, true);
                    break;
                }
                case "progress": {
                    JSONObject p = new JSONObject(json);
                    int c = p.optInt("current"), t = p.optInt("total");
                    String text = "revising".equals(p.optString("stage"))
                            ? "Checking saved chapters · " + c + " of " + t
                            : ("update".equals(jobKind) ? "Getting new chapters · " : "Downloading · ") + c + " of " + t;
                    show(text, c, t, false);
                    break;
                }
                case "challenge":
                    show("Needs you: open the app and tick the check", 0, 0, true);
                    break;
                default:
            }
        } catch (Exception ignored) {
        }
    }

    private void show(String text, int current, int total, boolean force) {
        long now = SystemClock.elapsedRealtime();
        if (!force && now - lastShown < 1000) return;
        lastShown = now;
        int waiting = queued + mangaQueued;
        String more = waiting > 0 ? " · " + waiting + " more in the queue" : "";
        DownloadService.show(activity, jobTitle.isEmpty() ? "InkShelfs" : jobTitle, text + more, current, total);
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
                } else if ("Manga.ExportCBZ".equals(method)) {
                    payload = exportCBZ(new org.json.JSONArray(argsJson).getString(0));
                } else if ("OpenFile".equals(method)) {
                    org.json.JSONArray a = new org.json.JSONArray(argsJson);
                    openFile(a.getString(0), a.getString(1));
                    payload = "";
                } else if ("ShareFile".equals(method)) {
                    org.json.JSONArray a = new org.json.JSONArray(argsJson);
                    shareFile(a.getString(0), a.getString(1), a.optString(2, ""));
                    payload = "";
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
        track(event, payloadJSON);
        String js = "window.__naEmit && window.__naEmit(" + JSONObject.quote(event) + ","
                + JSONObject.quote(payloadJSON) + ")";
        activity.runOnUiThread(() -> webView.evaluateJavascript(js, null));
    }

    static final String EPUB = "application/epub+zip";
    static final String CBZ = "application/vnd.comicbook+zip";

    /** Go writes the EPUB to the cache; it is then saved where the reader can get at it. */
    private String exportEPUB(String slug) throws Exception {
        File tmpDir = new File(activity.getCacheDir(), "export");
        tmpDir.mkdirs();
        JSONObject res = new JSONObject(core.exportEPUB(slug, tmpDir.getAbsolutePath()));
        return saveExport(res, EPUB);
    }

    /** Go writes a manga series as one CBZ to the cache; it is then saved where the reader can get at it. */
    private String exportCBZ(String slug) throws Exception {
        File tmpDir = new File(activity.getCacheDir(), "export");
        tmpDir.mkdirs();
        JSONObject res = new JSONObject(core.exportCBZ(slug, tmpDir.getAbsolutePath()));
        return saveExport(res, CBZ);
    }

    /**
     * Moves an exported file out of the cache. Android 10+: into the phone's
     * Downloads folder. Android 7–9: wherever the reader picks in the system's
     * Save dialog. Adds "uri" and "mime" so the page can offer Open and Share.
     * Returns "" (cancelled) if the reader closes the Save dialog.
     */
    private String saveExport(JSONObject res, String mime) throws Exception {
        File tmp = new File(res.getString("path"));
        String name = res.getString("fileName");
        Uri uri;
        if (Build.VERSION.SDK_INT >= 29) {
            uri = insertIntoDownloads(tmp, name, mime);
            res.put("path", "Downloads");
        } else {
            CompletableFuture<Uri> picked = new CompletableFuture<>();
            activity.runOnUiThread(() -> activity.pickSaveLocation(name, mime, picked));
            uri = picked.get();
            if (uri == null) {
                tmp.delete();
                return "";
            }
            try (InputStream in = new FileInputStream(tmp);
                 OutputStream out = activity.getContentResolver().openOutputStream(uri, "w")) {
                if (out == null) throw new Exception("couldn't write to that folder");
                copy(in, out);
            }
            res.put("path", "");
        }
        tmp.delete();
        res.put("uri", uri.toString());
        res.put("mime", mime);
        return res.toString();
    }

    /** Opens a saved export in whichever app the reader picks (their e-book or comic reader). */
    private void openFile(String uriStr, String mime) throws Exception {
        Uri uri = Uri.parse(uriStr);
        boolean epub = EPUB.equals(mime);
        // Comic readers register CBZ under several types; try the common ones.
        String[] types = epub ? new String[]{EPUB} : new String[]{CBZ, "application/x-cbz", "application/zip"};
        onUi(() -> {
            for (String t : types) {
                Intent view = new Intent(Intent.ACTION_VIEW)
                        .setDataAndType(uri, t)
                        .addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
                try {
                    activity.startActivity(view);
                    return;
                } catch (ActivityNotFoundException ignored) {
                }
            }
            throw new Exception(epub
                    ? "No app on this phone opens EPUB books. Install an e-book reader, or use Share to send it to one."
                    : "No app on this phone opens CBZ comics. Install a comic reader, or use Share to send it to one.");
        });
    }

    /** Hands a saved export to the system share sheet (Kindle, Drive, email, chat apps…). */
    private void shareFile(String uriStr, String mime, String name) throws Exception {
        Uri uri = Uri.parse(uriStr);
        Intent send = new Intent(Intent.ACTION_SEND)
                .setType(mime)
                .putExtra(Intent.EXTRA_STREAM, uri)
                .addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
        send.setClipData(ClipData.newRawUri(name, uri)); // carries the read grant through the chooser
        Intent chooser = Intent.createChooser(send, name.isEmpty() ? "Share" : "Share " + name);
        onUi(() -> activity.startActivity(chooser));
    }

    private interface UiAction {
        void run() throws Exception;
    }

    /** Runs on the UI thread and waits, passing any error back to the caller. */
    private void onUi(UiAction action) throws Exception {
        CompletableFuture<Void> done = new CompletableFuture<>();
        activity.runOnUiThread(() -> {
            try {
                action.run();
                done.complete(null);
            } catch (Exception e) {
                done.completeExceptionally(e);
            }
        });
        try {
            done.get();
        } catch (ExecutionException e) {
            Throwable c = e.getCause();
            throw c instanceof Exception ? (Exception) c : new Exception(c);
        }
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

    /** Android 10+: copies a file into the phone's shared Downloads folder and returns its content URI. */
    private Uri insertIntoDownloads(File tmp, String name, String mime) throws Exception {
        ContentValues v = new ContentValues();
        v.put(MediaStore.Downloads.DISPLAY_NAME, name);
        v.put(MediaStore.Downloads.MIME_TYPE, mime);
        v.put(MediaStore.Downloads.IS_PENDING, 1);
        ContentResolver cr = activity.getContentResolver();
        Uri uri = cr.insert(MediaStore.Downloads.EXTERNAL_CONTENT_URI, v);
        if (uri == null) throw new Exception("couldn't create the file in Downloads");
        try (InputStream in = new FileInputStream(tmp); OutputStream out = cr.openOutputStream(uri)) {
            if (out == null) throw new Exception("couldn't write the file in Downloads");
            copy(in, out);
        }
        v.clear();
        v.put(MediaStore.Downloads.IS_PENDING, 0);
        cr.update(uri, v, null, null);
        return uri;
    }

    /** Copies a finished file into the phone's Downloads folder; returns where it went. */
    private String toDownloads(File tmp, String name, String mime) throws Exception {
        String shown;
        if (Build.VERSION.SDK_INT >= 29) {
            insertIntoDownloads(tmp, name, mime);
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
