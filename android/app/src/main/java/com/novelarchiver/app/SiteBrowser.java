package com.novelarchiver.app;

import android.annotation.SuppressLint;
import android.app.Activity;
import android.graphics.Bitmap;
import android.view.View;
import android.webkit.CookieManager;
import android.webkit.JavascriptInterface;
import android.webkit.WebResourceError;
import android.webkit.WebResourceRequest;
import android.webkit.WebChromeClient;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;

import com.novelarchiver.core.mobile.BrowserDriver;

import org.json.JSONArray;
import org.json.JSONObject;

import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;

/**
 * The four browser primitives the Go core needs (load, current, fetch,
 * capture, show), on a real Android WebView. The decisions (when to navigate,
 * waiting for human checks) live in Go: internal/browser/driven.go. Human
 * checks are shown to the user, never automated.
 */
public class SiteBrowser implements BrowserDriver {
    private final Activity activity;
    private final WebView view;
    private final View panel;
    private final AtomicInteger seq = new AtomicInteger();
    private final Map<Integer, AtomicReference<String>> results = new ConcurrentHashMap<>();
    private final Map<Integer, CountDownLatch> latches = new ConcurrentHashMap<>();
    private volatile CountDownLatch loadLatch;
    private volatile String curUrl = "";
    private volatile String curTitle = "";
    private volatile String loadError = null;

    @SuppressLint({"SetJavaScriptEnabled", "AddJavascriptInterface"})
    SiteBrowser(Activity activity, WebView view, View panel) {
        this.activity = activity;
        this.view = view;
        this.panel = panel;
        WebSettings s = view.getSettings();
        s.setJavaScriptEnabled(true);
        s.setDomStorageEnabled(true);
        s.setLoadsImagesAutomatically(false); // text only; saves data
        CookieManager.getInstance().setAcceptCookie(true);
        CookieManager.getInstance().setAcceptThirdPartyCookies(view, true);
        view.addJavascriptInterface(new Object() {
            @JavascriptInterface
            public void result(int id, String json) {
                AtomicReference<String> r = results.get(id);
                CountDownLatch l = latches.get(id);
                if (r != null && l != null) {
                    r.set(json);
                    l.countDown();
                }
            }
        }, "__naSB");
        view.setWebChromeClient(new WebChromeClient() {
            @Override
            public void onReceivedTitle(WebView v, String title) {
                curTitle = title == null ? "" : title;
            }
        });
        view.setWebViewClient(new WebViewClient() {
            @Override
            public void onPageStarted(WebView v, String url, Bitmap favicon) {
                curUrl = url;
                curTitle = "";
            }

            @Override
            public void onReceivedError(WebView v, WebResourceRequest req, WebResourceError err) {
                if (req.isForMainFrame()) {
                    loadError = String.valueOf(err.getDescription());
                    // The view now shows Chrome's error page, not this site:
                    // forget it so the next request navigates again.
                    curUrl = "";
                    curTitle = "";
                }
            }

            @Override
            public void onPageFinished(WebView v, String url) {
                if (loadError != null || url.startsWith("chrome-error:")) {
                    curUrl = "";
                    curTitle = "";
                    CountDownLatch l = loadLatch;
                    if (l != null) l.countDown();
                    return;
                }
                curUrl = url;
                String t = v.getTitle();
                if (t != null && !t.equals(url)) curTitle = t;
                CountDownLatch l = loadLatch;
                if (l != null) l.countDown();
            }
        });
    }

    private static String info(String url, String title) {
        try {
            return new JSONObject().put("url", url).put("title", title).toString();
        } catch (Exception e) {
            return "{}";
        }
    }

    @Override
    public String load(String url) throws Exception {
        CountDownLatch l = new CountDownLatch(1);
        loadLatch = l;
        loadError = null;
        activity.runOnUiThread(() -> view.loadUrl(url));
        if (!l.await(45, TimeUnit.SECONDS)) {
            throw new Exception("page took too long to load: " + url);
        }
        if (loadError != null) {
            throw new Exception("couldn't load " + url + " (" + loadError + ")");
        }
        Thread.sleep(500); // titles and redirects settle
        return current();
    }

    @Override
    public String current() {
        return info(curUrl, curTitle);
    }

    @Override
    public String fetch(String url) throws Exception {
        int id = seq.incrementAndGet();
        AtomicReference<String> r = new AtomicReference<>();
        CountDownLatch l = new CountDownLatch(1);
        results.put(id, r);
        latches.put(id, l);
        // Same contract as internal/browser fetchJS: text as-is, binary
        // (images) as base64 so it survives the bridge.
        String js = "(async function(){const cb=function(s,u,b,ct,b64){__naSB.result(" + id
                + ",JSON.stringify({status:s,url:u,body:b,contentType:ct,b64:b64}));};try{"
                + "const r=await fetch(" + JSONObject.quote(url) + ",{credentials:'same-origin',redirect:'follow'});"
                + "const ct=(r.headers.get('content-type')||'').toLowerCase();"
                + "if(ct===''||/^text\\/|json|xml|javascript/.test(ct)){cb(r.status,r.url,await r.text(),ct,false);}"
                + "else{const a=new Uint8Array(await r.arrayBuffer());let s='';"
                + "for(let i=0;i<a.length;i+=0x8000)s+=String.fromCharCode.apply(null,a.subarray(i,i+0x8000));"
                + "cb(r.status,r.url,btoa(s),ct,true);}"
                + "}catch(e){cb(-1,'',String(e),'',false);}})();";
        activity.runOnUiThread(() -> view.evaluateJavascript(js, null));
        try {
            if (!l.await(45, TimeUnit.SECONDS)) throw new Exception("browser fetch timed out: " + url);
            return r.get();
        } finally {
            results.remove(id);
            latches.remove(id);
        }
    }

    @Override
    public String capture() throws Exception {
        CountDownLatch l = new CountDownLatch(1);
        AtomicReference<String> r = new AtomicReference<>();
        activity.runOnUiThread(() -> view.evaluateJavascript(
                "JSON.stringify({url:location.href,body:document.documentElement.outerHTML})",
                value -> {
                    r.set(value);
                    l.countDown();
                }));
        if (!l.await(20, TimeUnit.SECONDS)) throw new Exception("couldn't read the page");
        // evaluateJavascript returns the result JSON-encoded (a quoted string).
        String v = r.get();
        if (v == null || v.equals("null")) throw new Exception("page not ready");
        return new JSONArray("[" + v + "]").getString(0);
    }

    @Override
    public void show(boolean visible) {
        activity.runOnUiThread(() -> panel.setVisibility(visible ? View.VISIBLE : View.GONE));
    }
}
