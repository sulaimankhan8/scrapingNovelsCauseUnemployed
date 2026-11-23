//go:build windows

package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	webview2 "github.com/jchv/go-webview2"

	"novelarchiver/internal/fetch"
)

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	procShowWindow      = user32.NewProc("ShowWindow")
	procSetForeground   = user32.NewProc("SetForegroundWindow")
	procShowWindowAsync = user32.NewProc("ShowWindowAsync")
)

const (
	swShowMinNoActiv = 7
	swRestore        = 9
)

// Window is one WebView2 window running on its own OS thread.
type Window struct {
	opts Options

	startMu sync.Mutex
	view    *view // nil until started or after the user closes it

	navMu sync.RWMutex  // navigation is exclusive; in-page fetches share
	sem   chan struct{} // max concurrent in-page fetches
	gapMu sync.Mutex
	next  time.Time
}

type view struct {
	w      webview2.WebView
	done   chan struct{} // closed when the window's message loop ends
	seq    atomic.Int64
	mu     sync.Mutex
	wait   map[string]chan result
	loads  chan loadEvent
	curURL string
	curTtl string
}

type result struct {
	status int
	url    string
	body   string
	ct     string
	b64    bool
}

type loadEvent struct{ url, title string }

// New prepares a browser fetcher. The window opens on first use.
func New(opts Options) (*Window, error) {
	return &Window{opts: opts, sem: make(chan struct{}, 3)}, nil
}

func (b *Window) status(msg string) {
	if b.opts.OnStatus != nil {
		b.opts.OnStatus(msg)
	}
}

// ensure starts the window thread if it isn't running.
func (b *Window) ensure() (*view, error) {
	b.startMu.Lock()
	defer b.startMu.Unlock()
	if v := b.view; v != nil {
		select {
		case <-v.done:
		default:
			return v, nil
		}
	}
	v := &view{done: make(chan struct{}), wait: map[string]chan result{}, loads: make(chan loadEvent, 16)}
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer close(v.done)
		w := webview2.NewWithOptions(webview2.WebViewOptions{
			DataPath:  b.opts.DataPath,
			AutoFocus: true,
			WindowOptions: webview2.WindowOptions{
				Title: "NovelArchiver — site browser (leave open while downloading)", Width: 1000, Height: 760, Center: true,
			},
		})
		if w == nil {
			ready <- fmt.Errorf("could not start WebView2 (is the Edge WebView2 runtime installed?)")
			return
		}
		v.w = w
		_ = w.Bind("__naResult", func(id string, status int, u string, body string, ct string, b64 bool) {
			v.mu.Lock()
			ch := v.wait[id]
			delete(v.wait, id)
			v.mu.Unlock()
			if ch != nil {
				ch <- result{status, u, body, ct, b64}
			}
		})
		_ = w.Bind("__naLoaded", func(u, title string) {
			v.mu.Lock()
			v.curURL, v.curTtl = u, title
			v.mu.Unlock()
			select {
			case v.loads <- loadEvent{u, title}:
			default:
			}
		})
		w.Init(`window.addEventListener('load', function(){ setTimeout(function(){
  try { window.__naLoaded(location.href, document.title || ''); } catch (e) {} }, 400); });`)
		w.SetHtml(`<!doctype html><title>NovelArchiver</title><body style="font:15px system-ui;padding:24px;color:#333">
<h3>NovelArchiver site browser</h3><p>Pages from sites that need a real browser load here.
If a site asks you to verify you're human, complete it in this window and the download continues.</p>`)
		hwnd := uintptr(w.Window())
		procShowWindow.Call(hwnd, swShowMinNoActiv) // stays in the taskbar, out of the way
		ready <- nil
		w.Run()
		v.mu.Lock()
		for id, ch := range v.wait {
			close(ch)
			delete(v.wait, id)
		}
		v.mu.Unlock()
	}()
	if err := <-ready; err != nil {
		return nil, err
	}
	b.view = v
	return v, nil
}

func (v *view) drop(id string) {
	v.mu.Lock()
	delete(v.wait, id)
	v.mu.Unlock()
}

func (v *view) eval(js string) {
	v.w.Dispatch(func() { v.w.Eval(js) })
}

func (v *view) current() (string, string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.curURL, v.curTtl
}

// Show brings the window to the front, optionally opening a URL, so the
// user can sign in or complete a human check before downloading.
func (b *Window) Show(rawURL string) error {
	v, err := b.ensure()
	if err != nil {
		return err
	}
	if rawURL != "" {
		v.w.Dispatch(func() { v.w.Navigate(rawURL) })
	}
	b.front(v)
	return nil
}

func (b *Window) front(v *view) {
	hwnd := uintptr(v.w.Window())
	procShowWindowAsync.Call(hwnd, swRestore)
	procSetForeground.Call(hwnd)
}

func (b *Window) minimize(v *view) {
	procShowWindowAsync.Call(uintptr(v.w.Window()), swShowMinNoActiv)
}

// Close shuts the window down.
func (b *Window) Close() {
	b.startMu.Lock()
	v := b.view
	b.view = nil
	b.startMu.Unlock()
	if v != nil {
		v.w.Dispatch(func() { v.w.Destroy() })
		// Wait for the window thread to finish tearing down WebView2, so the
		// process never exits underneath it.
		select {
		case <-v.done:
		case <-time.After(5 * time.Second):
		}
	}
}

func (b *Window) polite(ctx context.Context) error {
	b.gapMu.Lock()
	at := b.next
	if now := time.Now(); at.Before(now) {
		at = now
	}
	b.next = at.Add(400 * time.Millisecond)
	b.gapMu.Unlock()
	select {
	case <-time.After(time.Until(at)):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Get implements fetch.Fetcher. If the window is already on the target's
// origin (or the referer's, for cross-origin API calls) it uses the page's
// own fetch() with its cookies; otherwise it navigates the window there.
func (b *Window) Get(ctx context.Context, rawURL, referer string) (*fetch.Page, error) {
	v, err := b.ensure()
	if err != nil {
		return nil, err
	}
	if err := b.polite(ctx); err != nil {
		return nil, err
	}
	target := origin(rawURL)
	base := target
	// Cross-origin calls and assets (images, JSON) are fetched from inside
	// the referer's page instead of being navigated to.
	isAPI := referer != "" && (origin(referer) != target || isAsset(rawURL))
	if isAPI {
		base = origin(referer)
	}

	cur, title := v.current()
	if origin(cur) != base || isChallenge(title) {
		start := rawURL
		if isAPI {
			start = referer
		}
		b.navMu.Lock()
		page, err := b.navigate(ctx, v, start, !isAPI)
		b.navMu.Unlock()
		if err != nil {
			return nil, err
		}
		if !isAPI {
			return page, nil
		}
	}

	page, err := b.inPageFetch(ctx, v, rawURL)
	if err != nil {
		return nil, err
	}
	if !isAPI && (looksBlocked(page.Status, string(page.Body)) || looksLikeLoader(string(page.Body))) {
		// Needs a real navigation (human check or JS redirect page).
		b.navMu.Lock()
		defer b.navMu.Unlock()
		return b.navigate(ctx, v, rawURL, true)
	}
	if page.Status >= 400 {
		return nil, &fetch.HTTPError{Status: page.Status, URL: rawURL}
	}
	return page, nil
}

// inPageFetch runs fetch() inside the page, retrying a few times because a
// single-page app may still be re-rendering or redirecting.
func (b *Window) inPageFetch(ctx context.Context, v *view, rawURL string) (*fetch.Page, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(attempt) * 1500 * time.Millisecond):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		page, err := b.fetchOnce(ctx, v, rawURL)
		if err == nil {
			return page, nil
		}
		if err == ErrClosed || ctx.Err() != nil {
			return nil, err
		}
		lastErr = err
	}
	return nil, lastErr
}

func (b *Window) fetchOnce(ctx context.Context, v *view, rawURL string) (*fetch.Page, error) {
	b.navMu.RLock()
	defer b.navMu.RUnlock()
	select {
	case b.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-b.sem }()

	id := "f" + strconv.FormatInt(v.seq.Add(1), 10)
	ch := make(chan result, 1)
	v.mu.Lock()
	v.wait[id] = ch
	v.mu.Unlock()
	u, _ := json.Marshal(rawURL)
	v.eval(fetchJS(string(u), fmt.Sprintf(`function(s, u, b, ct, b64){ window.__naResult(%q, s, u, b, ct, b64); }`, id)))
	select {
	case r, ok := <-ch:
		if !ok {
			return nil, ErrClosed
		}
		if r.status < 0 {
			return nil, fmt.Errorf("browser fetch failed for %s: %s", rawURL, r.body)
		}
		return pageFromResult(r.status, r.url, r.body, r.ct, r.b64)
	case <-time.After(45 * time.Second):
		v.drop(id)
		return nil, fmt.Errorf("browser fetch timed out: %s", rawURL)
	case <-v.done:
		return nil, ErrClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// navigate loads a URL in the window and returns the rendered HTML once the
// page has settled. Human checks bring the window forward and wait for the
// user; JS loader pages are waited out.
func (b *Window) navigate(ctx context.Context, v *view, rawURL string, wantHTML bool) (*fetch.Page, error) {
	// Drain stale load events.
	for len(v.loads) > 0 {
		<-v.loads
	}
	v.w.Dispatch(func() { v.w.Navigate(rawURL) })
	deadline := time.After(45 * time.Second)
	challenged := false
	for {
		select {
		case ev := <-v.loads:
			if isChallenge(ev.title) {
				if !challenged {
					challenged = true
					deadline = time.After(3 * time.Minute)
					b.front(v)
					b.status("This site wants you to confirm you're human. Complete the check in the NovelArchiver site browser window.")
					if b.opts.OnChallenge != nil {
						b.opts.OnChallenge(origin(rawURL))
					}
				}
				continue
			}
			if isInterstitial(ev.title) {
				continue // loader page that redirects by itself
			}
			time.Sleep(700 * time.Millisecond) // let client-side rendering finish
			if !wantHTML {
				if challenged {
					b.minimize(v)
					b.status("Check passed; continuing.")
				}
				return &fetch.Page{URL: ev.url, Status: 200}, nil
			}
			page, err := b.capture(ctx, v)
			if err == nil && challengeBody(string(page.Body)) {
				// Still the check (its title changed while verifying): wait
				// for the next load.
				if !challenged {
					challenged = true
					deadline = time.After(3 * time.Minute)
					b.front(v)
					b.status("This site wants you to confirm you're human. Complete the check in the NovelArchiver site browser window.")
					if b.opts.OnChallenge != nil {
						b.opts.OnChallenge(origin(rawURL))
					}
				}
				continue
			}
			if challenged {
				b.minimize(v)
				b.status("Check passed; continuing.")
			}
			return page, err
		case <-deadline:
			if challenged {
				return nil, ErrChallengeTimeout
			}
			return nil, fmt.Errorf("page took too long to load: %s", rawURL)
		case <-v.done:
			return nil, ErrClosed
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// capture reads the rendered HTML, retrying if the page was mid-redirect
// (bindings are briefly missing on a freshly created document).
func (b *Window) capture(ctx context.Context, v *view) (*fetch.Page, error) {
	for attempt := 0; attempt < 3; attempt++ {
		page, err := b.captureOnce(ctx, v)
		if err == nil || err == ErrClosed || ctx.Err() != nil {
			return page, err
		}
		time.Sleep(time.Second)
	}
	return nil, fmt.Errorf("couldn't read the page from the browser")
}

func (b *Window) captureOnce(ctx context.Context, v *view) (*fetch.Page, error) {
	id := "c" + strconv.FormatInt(v.seq.Add(1), 10)
	ch := make(chan result, 1)
	v.mu.Lock()
	v.wait[id] = ch
	v.mu.Unlock()
	v.eval(fmt.Sprintf(`window.__naResult(%q, 200, location.href, document.documentElement.outerHTML, 'text/html', false);`, id))
	select {
	case r, ok := <-ch:
		if !ok {
			return nil, ErrClosed
		}
		return &fetch.Page{URL: r.url, Status: 200, Body: []byte(r.body)}, nil
	case <-time.After(8 * time.Second):
		v.drop(id)
		return nil, fmt.Errorf("couldn't read the page from the browser")
	case <-v.done:
		return nil, ErrClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
