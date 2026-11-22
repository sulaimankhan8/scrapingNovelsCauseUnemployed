package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"novelarchiver/internal/fetch"
)

// Driver is the small set of primitives a platform browser must provide
// (the Android WebView implements it in Java). All methods may block; the
// platform marshals onto its UI thread. Results are JSON strings so the
// interface stays gomobile-friendly.
type Driver interface {
	// Load navigates and blocks until the page has loaded.
	// Returns {"url": "...", "title": "..."}.
	Load(url string) (string, error)
	// Current returns {"url","title"} of the page now shown.
	Current() string
	// Fetch runs fetch(url) inside the current page.
	// Returns {"status": 200, "url": "...", "body": "..."}; status -1 on a
	// network/CORS error.
	Fetch(url string) (string, error)
	// Capture returns {"url","body"} with the rendered document HTML.
	Capture() (string, error)
	// Show makes the browser visible (true) or hides it (false).
	Show(visible bool)
}

// Driven implements the same fetch strategy as the Windows window on top of
// a platform Driver.
type Driven struct {
	d    Driver
	opts Options

	navMu sync.RWMutex
	sem   chan struct{}
	gapMu sync.Mutex
	next  time.Time
}

// NewDriven wraps a platform driver.
func NewDriven(d Driver, opts Options) *Driven {
	return &Driven{d: d, opts: opts, sem: make(chan struct{}, 3)}
}

type pageInfo struct {
	URL    string `json:"url"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Body   string `json:"body"`
}

func decode(s string) pageInfo {
	var p pageInfo
	_ = json.Unmarshal([]byte(s), &p)
	return p
}

// Show opens the browser at url so the user can deal with a check or login.
func (b *Driven) Show(url string) error {
	if url != "" {
		go func() { _, _ = b.d.Load(url) }()
	}
	b.d.Show(true)
	return nil
}

// Close hides the browser.
func (b *Driven) Close() { b.d.Show(false) }

func (b *Driven) polite(ctx context.Context) error {
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

// Get implements fetch.Fetcher.
func (b *Driven) Get(ctx context.Context, rawURL, referer string) (*fetch.Page, error) {
	if err := b.polite(ctx); err != nil {
		return nil, err
	}
	target := origin(rawURL)
	base := target
	isAPI := referer != "" && origin(referer) != target
	if isAPI {
		base = origin(referer)
	}
	cur := decode(b.d.Current())
	if origin(cur.URL) != base || isChallenge(cur.Title) {
		start := rawURL
		if isAPI {
			start = referer
		}
		b.navMu.Lock()
		page, err := b.navigate(ctx, start, !isAPI)
		b.navMu.Unlock()
		if err != nil {
			return nil, err
		}
		if !isAPI {
			return page, nil
		}
	}
	page, err := b.inPageFetch(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	if !isAPI && (looksBlocked(page.Status, string(page.Body)) || looksLikeLoader(string(page.Body))) {
		b.navMu.Lock()
		defer b.navMu.Unlock()
		return b.navigate(ctx, rawURL, true)
	}
	if page.Status >= 400 {
		return nil, &fetch.HTTPError{Status: page.Status, URL: rawURL}
	}
	return page, nil
}

func (b *Driven) inPageFetch(ctx context.Context, rawURL string) (*fetch.Page, error) {
	b.navMu.RLock()
	defer b.navMu.RUnlock()
	select {
	case b.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-b.sem }()
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(attempt) * 1500 * time.Millisecond):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		raw, err := b.d.Fetch(rawURL)
		if err != nil {
			lastErr = err
			continue
		}
		p := decode(raw)
		if p.Status < 0 {
			lastErr = fmt.Errorf("browser fetch failed for %s: %s", rawURL, p.Body)
			continue
		}
		return &fetch.Page{URL: p.URL, Status: p.Status, Body: []byte(p.Body)}, nil
	}
	return nil, lastErr
}

// navigate loads a page, waits out loaders, and gives the user time to
// complete a human check (never automated).
func (b *Driven) navigate(ctx context.Context, rawURL string, wantHTML bool) (*fetch.Page, error) {
	raw, err := b.d.Load(rawURL)
	if err != nil {
		return nil, err
	}
	info := decode(raw)
	deadline := time.Now().Add(20 * time.Second)
	challenged := false
	for isChallenge(info.Title) || isInterstitial(info.Title) {
		if isChallenge(info.Title) && !challenged {
			challenged = true
			deadline = time.Now().Add(3 * time.Minute)
			b.d.Show(true)
			if b.opts.OnStatus != nil {
				b.opts.OnStatus("This site wants you to confirm you're human. Complete the check in the site browser.")
			}
			if b.opts.OnChallenge != nil {
				b.opts.OnChallenge(origin(rawURL))
			}
		}
		if time.Now().After(deadline) {
			if challenged {
				return nil, ErrChallengeTimeout
			}
			return nil, fmt.Errorf("page took too long to load: %s", rawURL)
		}
		select {
		case <-time.After(time.Second):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		info = decode(b.d.Current())
	}
	if challenged {
		b.d.Show(false)
		time.Sleep(1500 * time.Millisecond) // let the real page finish after the check
	}
	if !wantHTML {
		return &fetch.Page{URL: info.URL, Status: 200}, nil
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		raw, err := b.d.Capture()
		if err == nil {
			p := decode(raw)
			if p.Body != "" {
				return &fetch.Page{URL: p.URL, Status: 200, Body: []byte(p.Body)}, nil
			}
			err = fmt.Errorf("empty page")
		}
		lastErr = err
		time.Sleep(time.Second)
	}
	return nil, fmt.Errorf("couldn't read the page from the browser: %v", lastErr)
}
