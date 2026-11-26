// Package translate turns Chinese, Japanese and Korean chapters into
// English with Google Translate, the same free service Chrome's "Translate
// page" uses. Requests go one at a time with a pause between them, and the
// translator backs off (and then stops for a while) when Google refuses, so
// heavy downloads don't get the user's IP blocked.
package translate

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrBusy means Google is refusing requests for now (rate limit). The
// original chapter is kept; the next update translates it.
var ErrBusy = errors.New("Google Translate asked the app to slow down; the chapter is saved in its own language and is translated by itself once Google allows it again")

// ErrBlocked means Google has blocked this network's translation requests
// for a while (its "unusual traffic" page). Retrying soon only prolongs
// it, so the app stops asking for half an hour.
var ErrBlocked = errors.New("Google Translate is limiting requests from this network for a while (it saw too many). That usually lifts within a few hours; the chapter is saved in its own language and is translated by itself after that")

// statusBlocked is post's status for an answer from the unusual-traffic page.
const statusBlocked = -1

// quotes fixes Google's TeX-style quotes: Japanese 『』 come back as two
// backticks and two apostrophes.
var quotes = strings.NewReplacer("``", "“", "''", "”")

// Translator turns texts into English, keeping their order.
type Translator interface {
	Translate(ctx context.Context, texts []string, from string) ([]string, error)
}

// maxChars is how much text goes into one request (Google's web endpoint
// accepts ~5,000 characters).
const maxChars = 4500

// Google calls translate.googleapis.com's free web endpoint.
type Google struct {
	Client *http.Client
	// Gap is the pause between requests (default 1.2 s: at 400 ms Google
	// flagged the connection after ~260 requests).
	Gap time.Duration
	// Endpoint overrides the URL (tests).
	Endpoint string

	mu          sync.Mutex
	next        time.Time
	pausedUntil time.Time
	// strikes counts refusals close together: each one makes the next
	// pause longer. An hour without one resets it.
	strikes    int
	lastStrike time.Time
}

// Pauses after Google refuses, growing with each refusal in a row: a
// "slow down" (429) and a network block (the unusual-traffic page).
var (
	busyPauses    = []time.Duration{3 * time.Minute, 10 * time.Minute, 30 * time.Minute, time.Hour}
	blockedPauses = []time.Duration{30 * time.Minute, time.Hour, 2 * time.Hour, 4 * time.Hour}
)

// NewGoogle returns a translator with sensible defaults.
func NewGoogle() *Google {
	return &Google{Client: newClient()}
}

// newClient speaks HTTP/1.1 only. Google's translate endpoint answers Go's
// HTTP/2 requests with 429 every time (checked 2026-11-25: the same
// request over HTTP/1.1 gets 200), so HTTP/2 would look like a rate limit
// that never ends.
func newClient() *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.ForceAttemptHTTP2 = false
	tr.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
	// Offer only HTTP/1.1 in the TLS handshake too, or Google picks h2.
	tr.TLSClientConfig = &tls.Config{NextProtos: []string{"http/1.1"}}
	return &http.Client{Timeout: 30 * time.Second, Transport: tr,
		// A redirect is Google sending us to its unusual-traffic page:
		// read it as an answer instead of following it.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

var defaultClient = newClient()

// Translate translates texts to English. from is a language code ("zh",
// "ja", "ko") or "" to let Google detect it. Texts are packed into as few
// requests as fit, one paragraph per line.
func (g *Google) Translate(ctx context.Context, texts []string, from string) ([]string, error) {
	out := make([]string, len(texts))
	for start := 0; start < len(texts); {
		// Pack paragraphs up to maxChars. A paragraph that is longer on its
		// own is sent alone (split at sentence ends if it must be).
		end, size := start, 0
		for end < len(texts) {
			n := len([]rune(texts[end])) + 1
			if end > start && size+n > maxChars {
				break
			}
			size += n
			end++
		}
		batch := texts[start:end]
		got, err := g.batch(ctx, batch, from)
		if err != nil {
			return nil, err
		}
		copy(out[start:end], got)
		start = end
	}
	return out, nil
}

// batch translates paragraphs sent as lines of one request. If Google's
// answer doesn't come back with the same number of lines, the batch is
// split in half and each half tried again, so no paragraph is misplaced and
// a long list doesn't turn into one request per line.
func (g *Google) batch(ctx context.Context, paras []string, from string) ([]string, error) {
	clean := make([]string, len(paras))
	for i, p := range paras {
		clean[i] = strings.Join(strings.Fields(strings.ReplaceAll(p, "\n", " ")), " ")
	}
	if len(clean) == 1 {
		t, err := g.long(ctx, clean[0], from)
		return []string{t}, err
	}
	joined, err := g.call(ctx, strings.Join(clean, "\n"), from, "en")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimRight(joined, "\n"), "\n")
	if len(lines) == len(clean) {
		for i := range lines {
			lines[i] = strings.TrimSpace(lines[i])
		}
		return lines, nil
	}
	half := len(clean) / 2
	a, err := g.batch(ctx, clean[:half], from)
	if err != nil {
		return nil, err
	}
	b, err := g.batch(ctx, clean[half:], from)
	if err != nil {
		return nil, err
	}
	return append(a, b...), nil
}

// long translates one paragraph, splitting it at sentence ends if it's
// longer than one request allows.
func (g *Google) long(ctx context.Context, p, from string) (string, error) {
	if strings.TrimSpace(p) == "" {
		return "", nil
	}
	r := []rune(p)
	if len(r) <= maxChars {
		return g.call(ctx, p, from, "en")
	}
	cut := maxChars
	for i := maxChars; i > maxChars/2; i-- {
		if strings.ContainsRune("。！？.!?」”", r[i-1]) {
			cut = i
			break
		}
	}
	a, err := g.call(ctx, string(r[:cut]), from, "en")
	if err != nil {
		return "", err
	}
	b, err := g.long(ctx, string(r[cut:]), from)
	return strings.TrimSpace(a + " " + b), err
}

// Into translates a short text (a title) from English into another
// language ("zh-CN", "ja", "ko"), for searching sites in that language.
func (g *Google) Into(ctx context.Context, text, to string) (string, error) {
	return g.call(ctx, text, "en", to)
}

// call sends one request, paced and retried with backoff on refusals.
func (g *Google) call(ctx context.Context, text, from, to string) (string, error) {
	if from == "" {
		from = "auto"
	}
	backoff := []time.Duration{2 * time.Second, 6 * time.Second, 15 * time.Second}
	for attempt := 0; ; attempt++ {
		if err := g.wait(ctx); err != nil {
			return "", err
		}
		out, status, err := g.post(ctx, text, from, to)
		if err == nil {
			g.ok()
			return out, nil
		}
		if status == statusBlocked {
			g.strike(blockedPauses)
			return "", ErrBlocked
		}
		// Google says "slow down" with 429.
		limited := status == http.StatusTooManyRequests
		retry := limited || status >= 500 || status == 0
		if !retry || attempt >= len(backoff) || ctx.Err() != nil {
			if limited {
				g.strike(busyPauses)
				return "", ErrBusy
			}
			return "", err
		}
		select {
		case <-time.After(backoff[attempt]):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

func (g *Google) wait(ctx context.Context) error {
	g.mu.Lock()
	if time.Now().Before(g.pausedUntil) {
		g.mu.Unlock()
		return ErrBusy
	}
	gap := g.Gap
	if gap == 0 {
		gap = 1200 * time.Millisecond
	}
	at := g.next
	if now := time.Now(); at.Before(now) {
		at = now
	}
	g.next = at.Add(gap)
	g.mu.Unlock()
	select {
	case <-time.After(time.Until(at)):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// strike pauses after a refusal, longer for each one in a row.
func (g *Google) strike(pauses []time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now()
	if now.Sub(g.lastStrike) > time.Hour {
		g.strikes = 0
	}
	i := g.strikes
	if i >= len(pauses) {
		i = len(pauses) - 1
	}
	g.strikes++
	g.lastStrike = now
	g.pausedUntil = now.Add(pauses[i])
}

// ok forgets old refusals once Google has been answering for an hour.
func (g *Google) ok() {
	g.mu.Lock()
	if g.strikes > 0 && time.Since(g.lastStrike) > time.Hour {
		g.strikes = 0
	}
	g.mu.Unlock()
}

func (g *Google) post(ctx context.Context, text, from, to string) (string, int, error) {
	endpoint := g.Endpoint
	if endpoint == "" {
		endpoint = "https://translate.googleapis.com/translate_a/single"
	}
	q := url.Values{"client": {"gtx"}, "sl": {from}, "tl": {to}, "dt": {"t"}}
	form := url.Values{"q": {text}}
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint+"?"+q.Encode(), strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36")
	client := g.Client
	if client == nil {
		client = defaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", resp.StatusCode, err
	}
	if strings.Contains(resp.Request.URL.Path, "/sorry") || strings.Contains(resp.Header.Get("Location"), "/sorry") ||
		resp.StatusCode == http.StatusMethodNotAllowed {
		return "", statusBlocked, ErrBlocked
	}
	if resp.StatusCode != http.StatusOK {
		return "", resp.StatusCode, fmt.Errorf("Google Translate answered HTTP %d", resp.StatusCode)
	}
	// [[["Translated","Original",...],...],...]
	var data []any
	if err := json.Unmarshal(body, &data); err != nil || len(data) == 0 {
		return "", resp.StatusCode, fmt.Errorf("Google Translate sent an answer the app can't read")
	}
	segs, _ := data[0].([]any)
	var b strings.Builder
	for _, s := range segs {
		if seg, ok := s.([]any); ok && len(seg) > 0 {
			if t, ok := seg[0].(string); ok {
				b.WriteString(t)
			}
		}
	}
	if b.Len() == 0 && strings.TrimSpace(text) != "" {
		return "", resp.StatusCode, fmt.Errorf("Google Translate returned no text")
	}
	return quotes.Replace(b.String()), resp.StatusCode, nil
}

// PausedFor is how much longer Google's refusal pause lasts (0 if none).
func (g *Google) PausedFor() time.Duration {
	g.mu.Lock()
	defer g.mu.Unlock()
	if d := time.Until(g.pausedUntil); d > 0 {
		return d
	}
	return 0
}

// Status describes the translator for logs ("paused for 2m").
func (g *Google) Status() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	if d := time.Until(g.pausedUntil); d > 0 {
		return "paused for " + strconv.Itoa(int(d.Seconds())) + "s"
	}
	return "ready"
}
