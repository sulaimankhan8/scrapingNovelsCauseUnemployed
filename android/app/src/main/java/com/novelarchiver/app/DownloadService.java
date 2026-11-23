package com.novelarchiver.app;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.Context;
import android.content.Intent;
import android.content.pm.ServiceInfo;
import android.net.wifi.WifiManager;
import android.os.Build;
import android.os.IBinder;
import android.os.PowerManager;

import androidx.core.app.NotificationCompat;
import androidx.core.app.ServiceCompat;
import androidx.core.content.ContextCompat;

/**
 * Keeps downloads and updates running while the app is in the background or
 * the screen is off. Android stops or freezes an app that has nothing on
 * screen; a foreground service with a visible progress notification tells it
 * the app is doing work the user asked for. It runs only while the queue has
 * a job, and stops itself when the queue is empty.
 */
public class DownloadService extends Service {
    static final String CHANNEL = "progress";
    static final int ID = 7001;
    static final String ACTION_UPDATE = "update";
    static final String ACTION_PAUSE = "pause";

    /** Set by NovelBridge: pauses the current job (the Go core's CancelTask). */
    static volatile Runnable onPause;

    private PowerManager.WakeLock wake;
    private WifiManager.WifiLock wifi;
    private boolean foreground;

    /** Starts the service, or updates its notification if it's running. */
    static void show(Context ctx, String title, String text, int current, int total) {
        Intent i = new Intent(ctx, DownloadService.class).setAction(ACTION_UPDATE)
                .putExtra("title", title).putExtra("text", text)
                .putExtra("current", current).putExtra("total", total);
        try {
            ContextCompat.startForegroundService(ctx, i);
        } catch (Exception ignored) {
            // Android 12+ refuses to start one from the background. Jobs are
            // started from the app, so the service is already running by then.
        }
    }

    static void stop(Context ctx) {
        ctx.stopService(new Intent(ctx, DownloadService.class));
    }

    @Override
    public void onCreate() {
        super.onCreate();
        if (Build.VERSION.SDK_INT >= 26) {
            NotificationManager nm = getSystemService(NotificationManager.class);
            if (nm != null && nm.getNotificationChannel(CHANNEL) == null) {
                NotificationChannel ch = new NotificationChannel(CHANNEL, "Download progress", NotificationManager.IMPORTANCE_LOW);
                ch.setDescription("Shows while novels download or update, so they keep going in the background.");
                ch.setShowBadge(false);
                nm.createNotificationChannel(ch);
            }
        }
        // The CPU and Wi-Fi stay awake only while a job runs; released in onDestroy.
        PowerManager pm = (PowerManager) getSystemService(Context.POWER_SERVICE);
        if (pm != null) {
            wake = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "NovelArchiver:download");
            wake.setReferenceCounted(false);
            wake.acquire(6 * 60 * 60 * 1000L);
        }
        WifiManager wm = (WifiManager) getApplicationContext().getSystemService(Context.WIFI_SERVICE);
        if (wm != null) {
            wifi = wm.createWifiLock(Build.VERSION.SDK_INT >= 29 ? WifiManager.WIFI_MODE_FULL_HIGH_PERF : WifiManager.WIFI_MODE_FULL, "NovelArchiver:download");
            wifi.setReferenceCounted(false);
            wifi.acquire();
        }
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        if (intent != null && ACTION_PAUSE.equals(intent.getAction())) {
            Runnable p = onPause;
            if (p != null) new Thread(p).start();
            if (!foreground) promote(build("NovelArchiver", "Pausing…", 0, 0));
            return START_NOT_STICKY;
        }
        String title = intent != null ? intent.getStringExtra("title") : null;
        String text = intent != null ? intent.getStringExtra("text") : null;
        int current = intent != null ? intent.getIntExtra("current", 0) : 0;
        int total = intent != null ? intent.getIntExtra("total", 0) : 0;
        Notification n = build(title == null ? "NovelArchiver" : title, text == null ? "Working…" : text, current, total);
        if (!foreground) {
            promote(n);
        } else {
            NotificationManager nm = getSystemService(NotificationManager.class);
            if (nm != null) nm.notify(ID, n);
        }
        // Not restarted if Android kills it: the queue lives in the app, and
        // Resume picks up where it stopped.
        return START_NOT_STICKY;
    }

    private void promote(Notification n) {
        try {
            ServiceCompat.startForeground(this, ID, n,
                    Build.VERSION.SDK_INT >= 29 ? ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC : 0);
            foreground = true;
        } catch (Exception e) {
            stopSelf();
        }
    }

    private Notification build(String title, String text, int current, int total) {
        Intent open = new Intent(this, MainActivity.class).addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP);
        PendingIntent openPi = PendingIntent.getActivity(this, 1, open, PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
        PendingIntent pausePi = PendingIntent.getService(this, 2,
                new Intent(this, DownloadService.class).setAction(ACTION_PAUSE),
                PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
        NotificationCompat.Builder b = new NotificationCompat.Builder(this, CHANNEL)
                .setSmallIcon(android.R.drawable.stat_sys_download)
                .setContentTitle(title)
                .setContentText(text)
                .setContentIntent(openPi)
                .setOngoing(true)
                .setOnlyAlertOnce(true)
                .setSilent(true)
                .setCategory(NotificationCompat.CATEGORY_PROGRESS)
                .setForegroundServiceBehavior(NotificationCompat.FOREGROUND_SERVICE_IMMEDIATE)
                .addAction(0, "Pause", pausePi);
        if (total > 0) b.setProgress(total, Math.min(current, total), false);
        else b.setProgress(0, 0, true);
        return b.build();
    }

    /** Android 15 limits background data jobs to 6 hours a day. */
    @Override
    public void onTimeout(int startId, int fgsType) {
        Runnable p = onPause;
        if (p != null) new Thread(p).start();
        stopSelf();
    }

    @Override
    public void onDestroy() {
        if (wake != null && wake.isHeld()) wake.release();
        if (wifi != null && wifi.isHeld()) wifi.release();
        super.onDestroy();
    }

    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }
}
