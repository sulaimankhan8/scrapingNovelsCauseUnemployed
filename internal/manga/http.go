package manga

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

	"novelarchiver/internal/fetch"
)

const browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36"

// Request is one HTTP request made by the engine.
type Request struct {
	URL     string
	Method  string // GET (default) or POST
	Body    string // form body for POST
	Referer string
	Header  map[string]string
	Image   bool // asks for an image; bigger size limit, image Accept header
}

// Client is a polite HTTP client: per-host concurrency, a minimum gap
// between requests to one host, and retries with backoff.
type Client struct {
	PerHost  int
	MinDelay time.Duration
	Retries  int

	hc    *http.Client
	mu    sync.Mutex
	hosts map[string]*gate
}

type gate struct {
	sem  chan struct{}
	mu   sync.Mutex
	next time.Time
}

// NewClient builds a client. perHost and minDelay control politeness.
func NewClient(perHost int, minDelay time.Duration) *Client {
	return &Client{
		PerHost: perHost, MinDelay: minDelay, Retries: 3,
		hc:    &http.Client{Timeout: 60 * time.Second},
		hosts: map[string]*gate{},
	}
}

func (c *Client) gate(host string) *gate {
	c.mu.Lock()
	defer c.mu.Unlock()
	g := c.hosts[host]
	if g == nil {
		g = &gate{sem: make(chan struct{}, c.PerHost)}
		c.hosts[host] = g
	}
	return g
}

func (g *gate) wait(ctx context.Context, d time.Duration) error {
	g.mu.Lock()
	at := g.next
	if now := time.Now(); at.Before(now) {
		at = now
	}
	g.next = at.Add(d)
	g.mu.Unlock()
	select {
	case <-time.After(time.Until(at)):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Do performs a request with retries. Bot checks return fetch.ErrBlocked;
// 4xx other than 429 are returned at once as *fetch.HTTPError.
func (c *Client) Do(ctx context.Context, r Request) (*fetch.Page, error) {
	u, err := url.Parse(r.URL)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("bad link %q", r.URL)
	}
	g := c.gate(u.Host)
	select {
	case g.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-g.sem }()
	var last error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if attempt > 0 {
			d := time.Duration(1<<uint(attempt-1))*time.Second + time.Duration(rand.Intn(400))*time.Millisecond
			select {
			case <-time.After(d):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		if err := g.wait(ctx, c.MinDelay); err != nil {
			return nil, err
		}
		p, err := c.once(ctx, r)
		if err == nil {
			return p, nil
		}
		last = err
		var he *fetch.HTTPError
		if errors.Is(err, fetch.ErrBlocked) || (errors.As(err, &he) && he.Status < 500 && he.Status != 429) || ctx.Err() != nil {
			return nil, err
		}
	}
	return nil, last
}

func (c *Client) once(ctx context.Context, r Request) (*fetch.Page, error) {
	method := r.Method
	if method == "" {
		method = http.MethodGet
	}
	var body io.Reader
	if r.Body != "" {
		body = strings.NewReader(r.Body)
	}
	req, err := http.NewRequestWithContext(ctx, method, r.URL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	if r.Image {
		req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/*,*/*;q=0.8")
	} else {
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/json;q=0.9,*/*;q=0.8")
	}
	if r.Body != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	}
	if r.Referer != "" {
		req.Header.Set("Referer", r.Referer)
	}
	for k, v := range r.Header {
		req.Header.Set(k, v)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	limit := int64(32 << 20)
	if r.Image {
		limit = 64 << 20
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return nil, err
	}
	p := &fetch.Page{URL: resp.Request.URL.String(), Status: resp.StatusCode, Body: data, Header: resp.Header, FromHTTP: true}
	if resp.StatusCode == 429 {
		if secs, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && secs > 0 {
			select {
			case <-time.After(time.Duration(min(secs, 30)) * time.Second):
			case <-ctx.Done():
			}
		}
		return nil, &fetch.HTTPError{Status: 429, URL: r.URL}
	}
	if fetch.LooksLikeBotCheck(resp.StatusCode, data) || isEdgeChallenge(p.URL) {
		return nil, fmt.Errorf("%w: %s", fetch.ErrBlocked, r.URL)
	}
	if resp.StatusCode >= 400 {
		return nil, &fetch.HTTPError{Status: resp.StatusCode, URL: r.URL}
	}
	return p, nil
}

// isEdgeChallenge spots redirects to a bot-check page (MangaDex's
// /edge/challenge for browser-looking clients).
func isEdgeChallenge(u string) bool {
	return strings.Contains(u, "/edge/challenge") || strings.Contains(u, "/cdn-cgi/challenge")
}
