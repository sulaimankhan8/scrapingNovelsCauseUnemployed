// Package fetch downloads pages politely: per-host concurrency limits, a
// minimum delay between requests to the same host, and retry with
// exponential backoff on transient errors.
package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Page is a fetched document.
type Page struct {
	URL      string // final URL after redirects
	Status   int
	Body     []byte
	Header   http.Header
	FromHTTP bool
}

// Fetcher is implemented by the plain HTTP client and by browser-backed
// fetchers (WebView2 on desktop, Android WebView on phones).
type Fetcher interface {
	Get(ctx context.Context, rawURL string, referer string) (*Page, error)
}

// ErrBlocked means the site answered with a bot check or access denial. The
// caller should switch to the in-app browser so the user can deal with it;
// the app never tries to defeat the check itself.
var ErrBlocked = errors.New("site blocked the request (bot check or access denied)")

// ErrNeedsBrowser is returned when a profile requires a browser fetcher but
// none is available on this platform/build yet.
var ErrNeedsBrowser = errors.New("this site only works through the in-app browser")

// HTTPError is a non-retryable HTTP status.
type HTTPError struct {
	Status int
	URL    string
}

func (e *HTTPError) Error() string { return fmt.Sprintf("HTTP %d for %s", e.Status, e.URL) }

// Options tunes the HTTP client.
type Options struct {
	PerHostConcurrency int           // default 3
	MinDelay           time.Duration // default 350ms between requests to one host
	Retries            int           // default 4
	Timeout            time.Duration // default 30s
	UserAgent          string
}

// HTTP is the plain Go HTTP fetcher.
type HTTP struct {
	opts   Options
	client *http.Client

	mu    sync.Mutex
	hosts map[string]*hostGate
}

type hostGate struct {
	sem  chan struct{}
	mu   sync.Mutex
	next time.Time
	// extra is added between requests after the host said 429.
	extra time.Duration
}

const defaultUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36 NovelArchiver/2"

// NewHTTP builds an HTTP fetcher.
func NewHTTP(o Options) *HTTP {
	if o.PerHostConcurrency <= 0 {
		o.PerHostConcurrency = 3
	}
	if o.MinDelay <= 0 {
		o.MinDelay = 350 * time.Millisecond
	}
	if o.Retries <= 0 {
		o.Retries = 4
	}
	if o.Timeout <= 0 {
		o.Timeout = 30 * time.Second
	}
	if o.UserAgent == "" {
		o.UserAgent = defaultUA
	}
	return &HTTP{
		opts:   o,
		client: &http.Client{Timeout: o.Timeout},
		hosts:  map[string]*hostGate{},
	}
}

func (h *HTTP) gate(host string) *hostGate {
	h.mu.Lock()
	defer h.mu.Unlock()
	g, ok := h.hosts[host]
	if !ok {
		g = &hostGate{sem: make(chan struct{}, h.opts.PerHostConcurrency)}
		h.hosts[host] = g
	}
	return g
}

// wait blocks until this host may receive another request.
func (g *hostGate) wait(ctx context.Context, minDelay time.Duration) error {
	g.mu.Lock()
	now := time.Now()
	at := g.next
	if at.Before(now) {
		at = now
	}
	g.next = at.Add(minDelay + g.extra)
	g.mu.Unlock()
	select {
	case <-time.After(time.Until(at)):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// slower is called when the host answers 429: every request to it, from
// every worker, is spaced further apart (up to 8 s more). faster undoes
// that a step at a time as requests succeed.
func (g *hostGate) slower() {
	g.mu.Lock()
	switch {
	case g.extra == 0:
		g.extra = time.Second
	case g.extra < 8*time.Second:
		g.extra *= 2
	}
	g.mu.Unlock()
}

func (g *hostGate) faster() {
	g.mu.Lock()
	if g.extra > 0 {
		g.extra -= g.extra / 8
		if g.extra < 50*time.Millisecond {
			g.extra = 0
		}
	}
	g.mu.Unlock()
}

// Get fetches a URL with retries. 403/429/503 pages that look like bot checks
// return ErrBlocked; other 4xx are returned as *HTTPError without retrying.
func (h *HTTP) Get(ctx context.Context, rawURL, referer string) (*Page, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	g := h.gate(u.Host)
	select {
	case g.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-g.sem }()

	var lastErr error
	for attempt := 0; attempt <= h.opts.Retries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			backoff += time.Duration(rand.Intn(500)) * time.Millisecond
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		if err := g.wait(ctx, h.opts.MinDelay); err != nil {
			return nil, err
		}
		page, err := h.once(ctx, rawURL, referer)
		if err == nil {
			g.faster()
			return page, nil
		}
		lastErr = err
		var he *HTTPError
		if errors.As(err, &he) && he.Status == 429 {
			g.slower()
		}
		if errors.Is(err, ErrBlocked) || (errors.As(err, &he) && he.Status < 500 && he.Status != 429) {
			return nil, err
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}

func (h *HTTP) once(ctx context.Context, rawURL, referer string) (*Page, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", h.opts.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/json;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	body = ToUTF8(body, resp.Header.Get("Content-Type"))
	page := &Page{URL: resp.Request.URL.String(), Status: resp.StatusCode, Body: body, Header: resp.Header, FromHTTP: true}
	if resp.StatusCode == 429 {
		// Rate limited: honour Retry-After (capped) and let Get retry.
		if secs, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && secs > 0 {
			select {
			case <-time.After(time.Duration(min(secs, 30)) * time.Second):
			case <-ctx.Done():
			}
		}
		return nil, &HTTPError{Status: 429, URL: rawURL}
	}
	if LooksLikeBotCheck(resp.StatusCode, body) {
		return nil, fmt.Errorf("%w: %s", ErrBlocked, rawURL)
	}
	if resp.StatusCode >= 400 {
		return nil, &HTTPError{Status: resp.StatusCode, URL: rawURL}
	}
	return page, nil
}

// LooksLikeBotCheck detects Cloudflare-style challenge pages and CDN denials.
func LooksLikeBotCheck(status int, body []byte) bool {
	// Many normal 200 pages include Cloudflare's "challenge-platform" beacon
	// script, so only an interstitial page counts: its <title>, or an error
	// status carrying challenge markers.
	head := strings.ToLower(string(body[:min(len(body), 16384)]))
	if strings.Contains(head, "<title>just a moment...</title>") || strings.Contains(head, "<title>attention required") {
		return true
	}
	if status == 403 || status == 503 {
		return strings.Contains(head, "cf-chl") || strings.Contains(head, "challenge-platform") ||
			strings.Contains(head, "verify you are human") || strings.Contains(head, "<title>403 forbidden</title>")
	}
	return false
}
