package com.novelarchiver.app;

import org.json.JSONArray;
import org.json.JSONObject;
import java.io.BufferedReader;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.net.HttpURLConnection;
import java.net.URL;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

public class ScraperEngine implements Runnable {

    private final NovelBridge bridge;
    private final File novelsDir;
    private final JSONObject options;
    private final AtomicBoolean isRunning = new AtomicBoolean(true);
    private final AtomicBoolean isCancelled = new AtomicBoolean(false);
    private String slug;

    public ScraperEngine(NovelBridge bridge, File novelsDir, JSONObject options) {
        this.bridge = bridge;
        this.novelsDir = novelsDir;
        this.options = options;
        String rawUrl = options.optString("url", options.optString("slug", "novel"));
        this.slug = slugify(rawUrl);
    }

    public boolean isRunning() {
        return isRunning.get();
    }

    public String getSlug() {
        return slug;
    }

    public void cancel() {
        isCancelled.set(true);
        isRunning.set(false);
        try {
            JSONObject status = new JSONObject();
            status.put("message", "Task cancelled by user.");
            bridge.emitEvent("status", status);
        } catch (Exception ignored) {}
    }

    @Override
    public void run() {
        try {
            boolean cleanOnly = options.optBoolean("cleanOnly", false);
            JSONObject meta = null;

            File metaFile = new File(novelsDir, slug + "/metadata.json");
            if (metaFile.exists()) {
                try (InputStream is = new FileInputStream(metaFile)) {
                    byte[] buf = new byte[(int) metaFile.length()];
                    is.read(buf);
                    meta = new JSONObject(new String(buf, StandardCharsets.UTF_8));
                }
            }

            if (!cleanOnly) {
                String inputUrl = options.optString("url");
                if (meta == null || !meta.optString("url").equals(inputUrl)) {
                    JSONObject status = new JSONObject();
                    status.put("message", "Resolving novel directory & table of contents...");
                    bridge.emitEvent("status", status);

                    meta = discoverNovel(inputUrl);
                    this.slug = meta.optString("slug", slug);

                    // Save metadata
                    File novelFolder = new File(novelsDir, slug);
                    novelFolder.mkdirs();
                    try (FileOutputStream fos = new FileOutputStream(new File(novelFolder, "metadata.json"))) {
                        fos.write(meta.toString(2).getBytes(StandardCharsets.UTF_8));
                    }
                }
            }

            if (meta == null) {
                throw new Exception("Metadata could not be loaded for " + slug);
            }

            String title = meta.optString("title", slug);
            JSONArray allChapters = meta.optJSONArray("chapters");
            if (allChapters == null || allChapters.length() == 0) {
                throw new Exception("No chapters discovered for " + title);
            }

            int startCh = options.optInt("startChapter", 0);
            int endCh = options.optInt("endChapter", 0);
            int concurrency = Math.min(Math.max(options.optInt("concurrency", 8), 1), 32);
            boolean force = options.optBoolean("force", false);

            List<JSONObject> targetList = new ArrayList<>();
            for (int i = 0; i < allChapters.length(); i++) {
                JSONObject ch = allChapters.getJSONObject(i);
                int num = ch.optInt("number", i + 1);
                if (startCh > 0 && num < startCh) continue;
                if (endCh > 0 && num > endCh) continue;
                targetList.add(ch);
            }
            if (targetList.isEmpty()) {
                for (int i = 0; i < allChapters.length(); i++) {
                    targetList.add(allChapters.getJSONObject(i));
                }
            }

            File rawDir = new File(novelsDir, slug + "/raw");
            File cleanDir = new File(novelsDir, slug + "/clean");
            rawDir.mkdirs();
            cleanDir.mkdirs();

            if (!cleanOnly) {
                JSONObject started = new JSONObject();
                started.put("slug", slug);
                started.put("title", title);
                started.put("stage", "scraping");
                bridge.emitEvent("started", started);

                int total = targetList.size();
                AtomicInteger scraped = new AtomicInteger(0);
                AtomicInteger cached = new AtomicInteger(0);
                AtomicInteger completed = new AtomicInteger(0);
                long startTime = System.currentTimeMillis();

                ExecutorService pool = Executors.newFixedThreadPool(concurrency);
                for (JSONObject ch : targetList) {
                    if (isCancelled.get()) break;
                    pool.submit(() -> {
                        if (isCancelled.get()) return;
                        try {
                            int num = ch.optInt("number", 1);
                            String fileName = String.format("chapter_%04d.html", num);
                            File chFile = new File(rawDir, fileName);

                            if (!force && chFile.exists() && chFile.length() > 100) {
                                cached.incrementAndGet();
                            } else {
                                String content = fetchChapterContent(ch);
                                try (FileOutputStream fos = new FileOutputStream(chFile)) {
                                    fos.write(content.getBytes(StandardCharsets.UTF_8));
                                }
                                scraped.incrementAndGet();
                            }

                            int c = completed.incrementAndGet();
                            long elapsed = Math.max(1, (System.currentTimeMillis() - startTime) / 1000);
                            double speed = (double) c / (double) elapsed;
                            int remaining = total - c;
                            int etaSec = speed > 0 ? (int) (remaining / speed) : 0;

                            if (c % 2 == 0 || c == total) {
                                JSONObject prog = new JSONObject();
                                prog.put("stage", "scraping");
                                prog.put("current", c);
                                prog.put("total", total);
                                prog.put("scraped", scraped.get());
                                prog.put("cached", cached.get());
                                prog.put("speed", String.format("%.1f ch/s", speed));
                                prog.put("eta", formatDuration(etaSec));
                                prog.put("percentage", (int) Math.round(((double) c / total) * 100));
                                prog.put("chapterNum", num);
                                prog.put("chapterTitle", ch.optString("title", "Chapter " + num));
                                prog.put("status", "ok");
                                bridge.emitEvent("progress", prog);
                            }
                        } catch (Exception ignored) {}
                    });
                }

                pool.shutdown();
                pool.awaitTermination(2, TimeUnit.HOURS);
            }

            if (isCancelled.get()) return;

            // Typesetting & Reader Generation
            JSONObject status = new JSONObject();
            status.put("message", "Typesetting prose & compiling offline readers...");
            bridge.emitEvent("status", status);

            int total = allChapters.length();
            for (int i = 0; i < total; i++) {
                if (isCancelled.get()) break;
                JSONObject ch = allChapters.getJSONObject(i);
                int num = ch.optInt("number", i + 1);
                String chTitle = ch.optString("title", "Chapter " + num);

                File rawFile = new File(rawDir, String.format("chapter_%04d.html", num));
                if (!rawFile.exists()) continue;

                String rawHtml;
                try (InputStream is = new FileInputStream(rawFile)) {
                    byte[] buf = new byte[(int) rawFile.length()];
                    is.read(buf);
                    rawHtml = new String(buf, StandardCharsets.UTF_8);
                }

                String cleanBody = sanitizeProse(rawHtml);
                int wordCount = cleanBody.split("\\s+").length;

                String prevLink = i > 0 ? String.format("chapter_%04d.html", allChapters.getJSONObject(i - 1).optInt("number", i)) : "";
                String nextLink = i < total - 1 ? String.format("chapter_%04d.html", allChapters.getJSONObject(i + 1).optInt("number", i + 2)) : "";

                String readerHtml = buildChapterHtml(title, slug, chTitle, num, total, wordCount, cleanBody, prevLink, nextLink);
                try (FileOutputStream fos = new FileOutputStream(new File(cleanDir, String.format("chapter_%04d.html", num)))) {
                    fos.write(readerHtml.getBytes(StandardCharsets.UTF_8));
                }

                if (i % 15 == 0 || i == total - 1) {
                    JSONObject prog = new JSONObject();
                    prog.put("stage", "cleaning");
                    prog.put("current", i + 1);
                    prog.put("total", total);
                    prog.put("cleaned", i + 1);
                    prog.put("percentage", (int) Math.round(((double) (i + 1) / total) * 100));
                    prog.put("chapterNum", num);
                    prog.put("chapterTitle", chTitle);
                    bridge.emitEvent("progress", prog);
                }
            }

            if (isCancelled.get()) return;

            // Generate Master Index
            String catalogHtml = buildCatalogHtml(meta);
            try (FileOutputStream fos = new FileOutputStream(new File(cleanDir, "index.html"))) {
                fos.write(catalogHtml.getBytes(StandardCharsets.UTF_8));
            }

            JSONObject done = new JSONObject();
            done.put("slug", slug);
            done.put("title", title);
            done.put("catalogUrl", "/novels/" + slug + "/clean/index.html");
            bridge.emitEvent("done", done);

        } catch (Exception e) {
            try {
                JSONObject err = new JSONObject();
                err.put("message", e.getMessage());
                bridge.emitEvent("error", err);
            } catch (Exception ignored) {}
        } finally {
            isRunning.set(false);
        }
    }

    public static JSONObject discoverNovel(String targetUrl) throws Exception {
        String slug = slugify(targetUrl);
        String cdnUrl = "https://boxnovela.b-cdn.net/novel/" + slug + ".json";

        String jsonResp = httpGet(cdnUrl, "https://novelbin.xyz/");
        if (jsonResp != null && !jsonResp.isEmpty()) {
            JSONObject data = new JSONObject(jsonResp);
            JSONObject meta = new JSONObject();
            meta.put("slug", slug);
            meta.put("title", data.optString("title", slug.replace('-', ' ')));
            meta.put("author", data.optString("author", "Unknown Author"));
            meta.put("cover", data.optString("image", ""));
            meta.put("description", data.optString("description", ""));
            meta.put("url", targetUrl);

            JSONArray cdnChapters = data.optJSONArray("chapters");
            JSONArray finalChapters = new JSONArray();
            if (cdnChapters != null) {
                for (int i = 0; i < cdnChapters.length(); i++) {
                    JSONObject ch = cdnChapters.getJSONObject(i);
                    JSONObject item = new JSONObject();
                    item.put("number", i + 1);
                    item.put("title", ch.optString("title", "Chapter " + (i + 1)));
                    item.put("url", "https://boxnovela.b-cdn.net/chapter/" + data.optString("id") + "/" + (i + 1) + ".json");
                    item.put("isClean", true);
                    finalChapters.put(item);
                }
            }
            meta.put("totalChapters", finalChapters.length());
            meta.put("chapters", finalChapters);
            meta.put("catalogUrl", "/novels/" + slug + "/clean/index.html");
            return meta;
        }

        // Web fallback
        String html = httpGet(targetUrl, targetUrl);
        JSONObject meta = new JSONObject();
        meta.put("slug", slug);
        meta.put("title", extractRegex(html, "<h3[^>]*class=\"title\"[^>]*>([^<]+)</h3>", slug));
        meta.put("author", extractRegex(html, "(?i)Author:[^<]*<a[^>]*>([^<]+)</a>", "Unknown Author"));
        meta.put("cover", extractRegex(html, "(?i)<div class=\"book\"[^>]*>\\s*<img[^>]+src=\"([^\"]+)\"", ""));
        meta.put("description", extractRegex(html, "(?i)<div class=\"desc-text\"[^>]*>([\\s\\S]*?)</div>", ""));
        meta.put("url", targetUrl);

        JSONArray chapters = new JSONArray();
        Pattern p = Pattern.compile("<a[^>]+href=\"([^\"]+chapter-([0-9]+)[^\"]*)\"[^>]*title=\"([^\"]+)\"");
        Matcher m = p.matcher(html);
        int idx = 1;
        while (m.find()) {
            JSONObject ch = new JSONObject();
            ch.put("number", Integer.parseInt(m.group(2)));
            ch.put("title", m.group(3));
            ch.put("url", m.group(1));
            ch.put("isClean", true);
            chapters.put(ch);
            idx++;
        }
        meta.put("totalChapters", chapters.length());
        meta.put("chapters", chapters);
        meta.put("catalogUrl", "/novels/" + slug + "/clean/index.html");
        return meta;
    }

    private String fetchChapterContent(JSONObject ch) throws Exception {
        String url = ch.optString("url");
        if (url.contains(".json") || url.contains("b-cdn.net")) {
            String jsonStr = httpGet(url, "https://novelbin.xyz/");
            JSONObject obj = new JSONObject(jsonStr);
            String title = obj.optString("title", ch.optString("title"));
            String content = obj.optString("content", "");
            return "<h3>" + title + "</h3>\n" + content;
        }

        String html = httpGet(url, url);
        Pattern p = Pattern.compile("(?i)<div[^>]+id=\"chr-content\"[^>]*>([\\s\\S]*?)</div>");
        Matcher m = p.matcher(html);
        if (m.find()) {
            return "<h3>" + ch.optString("title") + "</h3>\n" + m.group(1);
        }
        return "<h3>" + ch.optString("title") + "</h3>\n<p>" + html + "</p>";
    }

    private String sanitizeProse(String raw) {
        String s = raw.replaceAll("(?i)<(script|style|iframe)[^>]*>[\\s\\S]*?</\\1>", "");
        s = s.replaceAll("(?i)\\s*(style|onclick|onload|class|id)=\"[^\"]*\"", "");
        StringBuilder sb = new StringBuilder();
        Pattern p = Pattern.compile("(?i)<p[^>]*>([\\s\\S]*?)</p>");
        Matcher m = p.matcher(s);
        while (m.find()) {
            String text = m.group(1).replaceAll("<[^>]+>", "").trim();
            if (text.length() > 0 && !isAdPhrase(text)) {
                sb.append("<p>").append(text).append("</p>\n");
            }
        }
        return sb.length() > 0 ? sb.toString() : "<p>" + s.replaceAll("<[^>]+>", "").trim() + "</p>";
    }

    private boolean isAdPhrase(String t) {
        String l = t.toLowerCase();
        return (l.contains("novelbin") || l.contains("read light novel") || l.contains("support the author") || l.contains("pirate")) && t.length() < 120;
    }

    private String buildChapterHtml(String novelTitle, String slug, String chTitle, int chNum, int total, int words, String content, String prev, String next) {
        return "<!DOCTYPE html>\n<html lang=\"en\" data-theme=\"dark\">\n<head>\n"
                + "<meta charset=\"UTF-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n"
                + "<title>" + chTitle + " — " + novelTitle + "</title>\n"
                + "<link href=\"https://fonts.googleapis.com/css2?family=Newsreader:opsz,wght@6..72,400;6..72,500;6..72,600&family=Plus+Jakarta+Sans:wght@400;500;600;700&display=swap\" rel=\"stylesheet\">\n"
                + "<style>\n"
                + ":root { --bg: #0e1216; --text: #e6edf3; --muted: #7d8590; --accent: #c8411b; --card: #161b22; --border: #30363d; --content-size: 19px; }\n"
                + "[data-theme=\"light\"] { --bg: #ffffff; --text: #1f2328; --muted: #656d76; --card: #f6f8fa; --border: #d0d7de; }\n"
                + "[data-theme=\"sepia\"] { --bg: #fbf0d9; --text: #3c3226; --muted: #7f6e5d; --card: #f4e3c3; --border: #e2cca8; --accent: #9b3d1b; }\n"
                + "body { margin: 0; padding: 10px 14px 70px; background: var(--bg); color: var(--text); font-family: 'Newsreader', Georgia, serif; line-height: 1.8; }\n"
                + ".container { max-width: 820px; margin: 0 auto; }\n"
                + ".top-bar { display: flex; justify-content: space-between; align-items: center; padding: 12px 0; border-bottom: 1px solid var(--border); font-family: 'Plus Jakarta Sans', sans-serif; }\n"
                + ".novel-link { color: var(--accent); text-decoration: none; font-weight: 600; font-size: 14px; }\n"
                + ".btn-ctrl { background: var(--card); border: 1px solid var(--border); color: var(--text); padding: 4px 10px; border-radius: 4px; font-size: 12px; cursor: pointer; }\n"
                + "h1 { font-size: 24px; text-align: center; margin: 25px 0 10px; font-weight: 700; }\n"
                + ".meta { text-align: center; color: var(--muted); font-size: 13px; font-family: 'Plus Jakarta Sans', sans-serif; margin-bottom: 30px; }\n"
                + ".content { font-size: var(--content-size); text-align: justify; }\n"
                + ".content p { margin-bottom: 1.6em; text-indent: 1.5em; }\n"
                + ".nav-row { display: flex; justify-content: space-between; margin-top: 40px; padding-top: 20px; border-top: 1px solid var(--border); font-family: 'Plus Jakarta Sans', sans-serif; }\n"
                + ".btn { padding: 8px 16px; background: var(--accent); color: #fff; text-decoration: none; border-radius: 6px; font-weight: 600; font-size: 13px; }\n"
                + "</style></head><body>\n<div class=\"container\">\n"
                + "<div class=\"top-bar\"><a href=\"index.html\" class=\"novel-link\">📚 " + novelTitle + "</a>\n"
                + "<div><button class=\"btn-ctrl\" onclick=\"setTheme('light')\">☀️</button> <button class=\"btn-ctrl\" onclick=\"setTheme('sepia')\">📜</button> <button class=\"btn-ctrl\" onclick=\"setTheme('dark')\">🌙</button></div></div>\n"
                + "<h1>" + chTitle + "</h1>\n"
                + "<div class=\"meta\">Chapter " + chNum + " of " + total + " • ~" + (Math.max(1, words / 200)) + " min read</div>\n"
                + "<div class=\"content\">" + content + "</div>\n"
                + "<div class=\"nav-row\">"
                + (!prev.isEmpty() ? "<a href=\"" + prev + "\" class=\"btn\">← Previous</a>" : "<span></span>")
                + "<a href=\"index.html\" class=\"btn\" style=\"background: var(--card); color: var(--text); border: 1px solid var(--border);\">Catalog</a>"
                + (!next.isEmpty() ? "<a href=\"" + next + "\" class=\"btn\">Next →</a>" : "<span></span>")
                + "</div></div>\n"
                + "<script>function setTheme(t){document.documentElement.setAttribute('data-theme', t); localStorage.setItem('theme', t);} const st = localStorage.getItem('theme'); if(st) document.documentElement.setAttribute('data-theme', st);</script>\n"
                + "</body></html>";
    }

    private String buildCatalogHtml(JSONObject meta) {
        String title = meta.optString("title", "Novel");
        String author = meta.optString("author", "Unknown Author");
        String desc = meta.optString("description", "");
        JSONArray chapters = meta.optJSONArray("chapters");
        int total = chapters != null ? chapters.length() : 0;

        StringBuilder sb = new StringBuilder();
        sb.append("<!DOCTYPE html>\n<html lang=\"en\" data-theme=\"dark\"><head><meta charset=\"UTF-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n")
                .append("<title>").append(title).append(" — Master Catalog</title>\n")
                .append("<link href=\"https://fonts.googleapis.com/css2?family=Plus+Jakarta+Sans:wght@400;500;600;700&family=JetBrains+Mono:wght@500&display=swap\" rel=\"stylesheet\">\n")
                .append("<style>\n")
                .append(":root { --bg: #0e1216; --text: #e6edf3; --muted: #7d8590; --accent: #c8411b; --card: #161b22; --border: #30363d; }\n")
                .append("body { margin: 0; padding: 14px; background: var(--bg); color: var(--text); font-family: 'Plus Jakarta Sans', sans-serif; }\n")
                .append(".container { max-width: 900px; margin: 0 auto; }\n")
                .append(".back-btn { display: inline-flex; align-items: center; color: var(--muted); text-decoration: none; font-size: 13px; font-weight: 600; padding: 6px 12px; background: var(--card); border: 1px solid var(--border); border-radius: 4px; margin-bottom: 15px; }\n")
                .append(".header { background: var(--card); border: 1px solid var(--border); border-radius: 8px; padding: 20px; margin-bottom: 20px; }\n")
                .append("h1 { margin: 0 0 6px; font-size: 22px; }\n")
                .append(".author { color: var(--muted); font-size: 14px; margin-bottom: 12px; }\n")
                .append(".search { width: 100%; box-sizing: border-box; padding: 10px 14px; background: var(--bg); border: 1px solid var(--border); border-radius: 6px; color: #fff; font-size: 14px; margin-bottom: 15px; }\n")
                .append(".grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 10px; max-height: 700px; overflow-y: auto; }\n")
                .append(".card { background: var(--card); border: 1px solid var(--border); border-radius: 6px; padding: 10px 14px; color: var(--text); text-decoration: none; font-size: 13px; }\n")
                .append(".card:hover { border-color: var(--accent); }\n")
                .append("</style></head><body><div class=\"container\">\n")
                .append("<a href=\"/assets/frontend/index.html\" class=\"back-btn\">← Back to Bibliotheca</a>\n")
                .append("<div class=\"header\"><h1>").append(title).append("</h1><div class=\"author\">By ").append(author).append(" • ").append(total).append(" Chapters</div><div style=\"font-size: 13px; color: var(--muted); line-height: 1.5;\">").append(desc).append("</div></div>\n")
                .append("<input type=\"text\" id=\"s\" class=\"search\" placeholder=\"🔍 Search chapter number or title...\">\n")
                .append("<div class=\"grid\" id=\"g\">\n");

        if (chapters != null) {
            for (int i = 0; i < chapters.length(); i++) {
                JSONObject ch = chapters.optJSONObject(i);
                int num = ch != null ? ch.optInt("number", i + 1) : i + 1;
                String cTitle = ch != null ? ch.optString("title", "Chapter " + num) : "Chapter " + num;
                String fileName = String.format("chapter_%04d.html", num);
                sb.append("<a href=\"").append(fileName).append("\" class=\"card\" data-t=\"").append(cTitle.toLowerCase()).append(" ").append(num).append("\">")
                        .append("<b>#").append(num).append("</b> ").append(cTitle).append("</a>\n");
            }
        }

        sb.append("</div></div>\n<script>\n")
                .append("const s=document.getElementById('s'); const cards=document.querySelectorAll('.card');\n")
                .append("s.addEventListener('input', e=>{ const q=e.target.value.toLowerCase().trim(); cards.forEach(c=> c.style.display=(!q || c.getAttribute('data-t').includes(q))?'':'none'); });\n")
                .append("</script></body></html>");

        return sb.toString();
    }

    private static String httpGet(String urlStr, String referer) throws Exception {
        URL url = new URL(urlStr);
        HttpURLConnection conn = (HttpURLConnection) url.openConnection();
        conn.setRequestMethod("GET");
        conn.setConnectTimeout(15000);
        conn.setReadTimeout(15000);
        conn.setRequestProperty("User-Agent", "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36");
        if (referer != null) conn.setRequestProperty("Referer", referer);

        int code = conn.getResponseCode();
        if (code != 200) return null;

        StringBuilder sb = new StringBuilder();
        try (BufferedReader reader = new BufferedReader(new InputStreamReader(conn.getInputStream(), StandardCharsets.UTF_8))) {
            String line;
            while ((line = reader.readLine()) != null) {
                sb.append(line).append("\n");
            }
        }
        return sb.toString();
    }

    private static String slugify(String text) {
        String t = text.trim();
        t = t.replaceAll("^https?://[^/]+/b/", "");
        t = t.replaceAll("^https?://[^/]+/", "");
        t = t.replaceAll("/chapter-.*$", "");
        t = t.toLowerCase().replaceAll("[^a-z0-9]+", "-").replaceAll("^-+|-+$", "");
        return t.isEmpty() ? "novel" : t;
    }

    private static String extractRegex(String src, String regex, String def) {
        Pattern p = Pattern.compile(regex);
        Matcher m = p.matcher(src);
        if (m.find()) return m.group(1).trim();
        return def;
    }

    private static String formatDuration(int sec) {
        if (sec <= 0) return "--";
        if (sec < 60) return sec + "s";
        int m = sec / 60;
        int s = sec % 60;
        return m + "m " + s + "s";
    }
}
