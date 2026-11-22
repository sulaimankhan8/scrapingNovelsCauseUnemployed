package com.novelarchiver.app;

import android.app.Activity;
import android.content.Intent;
import android.net.Uri;
import android.webkit.JavascriptInterface;
import android.webkit.WebView;
import org.json.JSONArray;
import org.json.JSONObject;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.List;

public class NovelBridge {

    private final Activity activity;
    private final WebView webView;
    private final File novelsDir;
    private ScraperEngine activeEngine;

    public NovelBridge(Activity activity, WebView webView, File novelsDir) {
        this.activity = activity;
        this.webView = webView;
        this.novelsDir = novelsDir;
    }

    public void emitEvent(String eventName, JSONObject data) {
        activity.runOnUiThread(() -> {
            String script = String.format("window.__emitAndroidEvent && window.__emitAndroidEvent('%s', %s);",
                    eventName, data.toString());
            webView.evaluateJavascript(script, null);
        });
    }

    public void emitEvent(String eventName, String dataJson) {
        activity.runOnUiThread(() -> {
            String script = String.format("window.__emitAndroidEvent && window.__emitAndroidEvent('%s', %s);",
                    eventName, dataJson);
            webView.evaluateJavascript(script, null);
        });
    }

    @JavascriptInterface
    public String getNovels() {
        try {
            JSONArray arr = new JSONArray();
            File[] files = novelsDir.listFiles();
            if (files != null) {
                for (File dir : files) {
                    if (dir.isDirectory()) {
                        String slug = dir.getName();
                        File metaFile = new File(dir, "metadata.json");
                        JSONObject meta = new JSONObject();
                        if (metaFile.exists()) {
                            try (InputStream is = new FileInputStream(metaFile)) {
                                byte[] buf = new byte[(int) metaFile.length()];
                                is.read(buf);
                                meta = new JSONObject(new String(buf, StandardCharsets.UTF_8));
                            } catch (Exception ignored) {}
                        }

                        int rawCount = countFiles(new File(dir, "raw"), "chapter_");
                        int cleanCount = countFiles(new File(dir, "clean"), "chapter_");
                        boolean hasIndex = new File(dir, "clean/index.html").exists();

                        JSONObject item = new JSONObject();
                        item.put("slug", slug);
                        item.put("title", meta.optString("title", slug.replace('-', ' ')));
                        item.put("author", meta.optString("author", "Unknown Author"));
                        item.put("cover", meta.optString("cover", ""));
                        item.put("description", meta.optString("description", ""));
                        item.put("url", meta.optString("url", ""));
                        item.put("totalChapters", meta.optInt("totalChapters", Math.max(cleanCount, rawCount)));
                        item.put("rawCount", rawCount);
                        item.put("cleanCount", cleanCount);
                        item.put("hasIndex", hasIndex);
                        item.put("catalogUrl", "/novels/" + slug + "/clean/index.html");
                        arr.put(item);
                    }
                }
            }
            return arr.toString();
        } catch (Exception e) {
            return "[]";
        }
    }

    @JavascriptInterface
    public String getNovel(String slug) {
        try {
            File metaFile = new File(novelsDir, slug + "/metadata.json");
            if (metaFile.exists()) {
                try (InputStream is = new FileInputStream(metaFile)) {
                    byte[] buf = new byte[(int) metaFile.length()];
                    is.read(buf);
                    return new String(buf, StandardCharsets.UTF_8);
                }
            }
        } catch (Exception ignored) {}
        return "{}";
    }

    @JavascriptInterface
    public String getSystemStats() {
        try {
            int totalNovels = 0;
            int totalRaw = 0;
            int totalClean = 0;
            long totalBytes = 0;

            File[] files = novelsDir.listFiles();
            if (files != null) {
                for (File dir : files) {
                    if (dir.isDirectory()) {
                        totalNovels++;
                        totalRaw += countFiles(new File(dir, "raw"), "chapter_");
                        totalClean += countFiles(new File(dir, "clean"), "chapter_");
                        totalBytes += calculateDirSize(dir);
                    }
                }
            }

            double mb = (double) totalBytes / (1024.0 * 1024.0);
            JSONObject stats = new JSONObject();
            stats.put("totalNovels", totalNovels);
            stats.put("totalRawChapters", totalRaw);
            stats.put("totalCleanChapters", totalClean);
            stats.put("totalBytes", totalBytes);
            stats.put("sizeMb", String.format("%.1f MB", mb));
            return stats.toString();
        } catch (Exception e) {
            return "{\"totalNovels\":0,\"totalCleanChapters\":0,\"sizeMb\":\"0 MB\"}";
        }
    }

    @JavascriptInterface
    public void discover(String url, String callback) {
        new Thread(() -> {
            try {
                JSONObject meta = ScraperEngine.discoverNovel(url);
                JSONObject res = new JSONObject();
                res.put("success", true);
                res.put("metadata", meta);
                final String resJson = res.toString();
                activity.runOnUiThread(() -> {
                    String script = String.format("window['%s'] && window['%s'](%s);", callback, callback, resJson);
                    webView.evaluateJavascript(script, null);
                });
            } catch (Exception e) {
                final String errJson = "{\"success\":false,\"error\":\"" + escapeJson(e.getMessage()) + "\"}";
                activity.runOnUiThread(() -> {
                    String script = String.format("window['%s'] && window['%s'](%s);", callback, callback, errJson);
                    webView.evaluateJavascript(script, null);
                });
            }
        }).start();
    }

    private String escapeJson(String s) {
        if (s == null) return "";
        return s.replace("\"", "\\\"").replace("\n", "\\n").replace("\r", "");
    }

    @JavascriptInterface
    public String startScrape(String optionsJson) {
        try {
            if (activeEngine != null && activeEngine.isRunning()) {
                return new JSONObject().put("error", "An archival task is already running.").toString();
            }

            JSONObject opts = new JSONObject(optionsJson);
            activeEngine = new ScraperEngine(this, novelsDir, opts);
            new Thread(activeEngine).start();

            return new JSONObject().put("status", "started").put("slug", activeEngine.getSlug()).toString();
        } catch (Exception e) {
            return "{\"error\":\"" + e.getMessage() + "\"}";
        }
    }

    @JavascriptInterface
    public String cleanNovel(String slug) {
        try {
            if (activeEngine != null && activeEngine.isRunning()) {
                return new JSONObject().put("error", "A task is already running.").toString();
            }

            JSONObject opts = new JSONObject();
            opts.put("slug", slug);
            opts.put("cleanOnly", true);
            activeEngine = new ScraperEngine(this, novelsDir, opts);
            new Thread(activeEngine).start();

            return new JSONObject().put("success", true).put("slug", slug).toString();
        } catch (Exception e) {
            return "{\"error\":\"" + e.getMessage() + "\"}";
        }
    }

    @JavascriptInterface
    public String auditNovel(String slug) {
        try {
            File metaFile = new File(novelsDir, slug + "/metadata.json");
            if (!metaFile.exists()) {
                return "{\"validCount\":0,\"invalidCount\":0}";
            }

            String metaStr;
            try (InputStream is = new FileInputStream(metaFile)) {
                byte[] buf = new byte[(int) metaFile.length()];
                is.read(buf);
                metaStr = new String(buf, StandardCharsets.UTF_8);
            }
            JSONObject meta = new JSONObject(metaStr);
            JSONArray chapters = meta.optJSONArray("chapters");
            int validCount = 0;
            int invalidCount = 0;
            JSONArray invalidArr = new JSONArray();

            if (chapters != null) {
                for (int i = 0; i < chapters.length(); i++) {
                    JSONObject ch = chapters.getJSONObject(i);
                    int num = ch.optInt("number", i + 1);
                    String fileName = String.format("chapter_%04d.html", num);
                    File f = new File(novelsDir, slug + "/raw/" + fileName);
                    if (!f.exists() || f.length() < 120) {
                        invalidCount++;
                        invalidArr.put(num);
                    } else {
                        validCount++;
                    }
                }
            }

            JSONObject res = new JSONObject();
            res.put("total", chapters != null ? chapters.length() : 0);
            res.put("validCount", validCount);
            res.put("invalidCount", invalidCount);
            res.put("invalid", invalidArr);
            return res.toString();
        } catch (Exception e) {
            return "{\"validCount\":0,\"invalidCount\":0}";
        }
    }

    @JavascriptInterface
    public String retryNovel(String slug) {
        try {
            String novelData = getNovel(slug);
            JSONObject meta = new JSONObject(novelData);
            String url = meta.optString("url");
            JSONObject opts = new JSONObject();
            opts.put("url", url);
            opts.put("force", true);
            opts.put("concurrency", 6);
            return startScrape(opts.toString());
        } catch (Exception e) {
            return "{\"error\":\"" + e.getMessage() + "\"}";
        }
    }

    @JavascriptInterface
    public String deleteNovel(String slug) {
        try {
            File dir = new File(novelsDir, slug);
            deleteRecursive(dir);
            return new JSONObject().put("success", true).put("slug", slug).toString();
        } catch (Exception e) {
            return "{\"error\":\"" + e.getMessage() + "\"}";
        }
    }

    @JavascriptInterface
    public void cancelTask() {
        if (activeEngine != null) {
            activeEngine.cancel();
        }
    }

    @JavascriptInterface
    public void openExternalURL(String url) {
        try {
            Intent intent = new Intent(Intent.ACTION_VIEW, Uri.parse(url));
            activity.startActivity(intent);
        } catch (Exception ignored) {}
    }

    private int countFiles(File dir, String prefix) {
        if (!dir.exists() || !dir.isDirectory()) return 0;
        File[] files = dir.listFiles((d, name) -> name.startsWith(prefix));
        return files != null ? files.length : 0;
    }

    private long calculateDirSize(File dir) {
        long size = 0;
        File[] files = dir.listFiles();
        if (files != null) {
            for (File f : files) {
                if (f.isDirectory()) size += calculateDirSize(f);
                else size += f.length();
            }
        }
        return size;
    }

    private void deleteRecursive(File f) {
        if (f.isDirectory()) {
            File[] children = f.listFiles();
            if (children != null) {
                for (File child : children) deleteRecursive(child);
            }
        }
        f.delete();
    }
}
