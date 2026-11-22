// Package browser provides a fetch.Fetcher backed by a real browser engine
// (WebView2 on Windows). Pages load exactly as they would in Edge, in a
// window the user can see. When a site shows a "Verify you are human" check,
// the window is brought to the front so the user can complete it; the app
// never automates or bypasses such checks.
package browser

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"

	"novelarchiver/internal/fetch"
)

// ErrUnsupported is returned on platforms without a browser backend yet.
var ErrUnsupported = errors.New("the in-app browser is not available on this platform yet")

// ErrClosed means the user closed the browser window mid-request.
var ErrClosed = errors.New("the site browser window was closed")

// ErrChallengeTimeout means a human check was shown but not completed.
var ErrChallengeTimeout = errors.New("the site's 'Verify you are human' check wasn't completed in time")

// Options configures the browser window.
type Options struct {
	DataPath string // profile folder; cookies persist here between runs
	// OnChallenge is called when a site shows a human check and the window
	// has been brought to the front for the user.
	OnChallenge func(host string)
	// OnStatus receives short progress messages.
	OnStatus func(msg string)
}

var challengeTitle = regexp.MustCompile(`(?i)just a moment|attention required|verify you are human|security check|checking your browser|ddos-guard`)
var interstitialTitle = regexp.MustCompile(`(?i)^\s*(loading\.*|redirecting\.*|please wait\.*)\s*$`)

// isChallenge reports whether a loaded page is a human-check interstitial.
func isChallenge(title string) bool { return challengeTitle.MatchString(title) }

// isInterstitial reports a JS redirect/loader page that will move on by itself.
func isInterstitial(title string) bool { return interstitialTitle.MatchString(title) }

// looksBlocked inspects an in-page fetch() result.
func looksBlocked(status int, body string) bool {
	head := strings.ToLower(body[:min(len(body), 16384)])
	if strings.Contains(head, "<title>just a moment") || strings.Contains(head, "<title>attention required") {
		return true
	}
	return status == 403 || status == 503 && strings.Contains(head, "cf-chl")
}

// looksLikeLoader spots tiny JS loader pages that only redirect.
func looksLikeLoader(body string) bool {
	if len(body) > 8000 {
		return false
	}
	m := regexp.MustCompile(`(?is)<title>(.*?)</title>`).FindStringSubmatch(body)
	return m != nil && isInterstitial(m[1])
}

// fetchJS is the in-page fetch used by every platform. Text responses come
// back as-is; binary ones (images) as base64, so covers and chapter images
// survive the trip through the JavaScript bridge. cb(status, url, body,
// contentType, isBase64) receives the result.
func fetchJS(urlJSON, cb string) string {
	return `(async function(){ const cb = ` + cb + `; try {
  const r = await fetch(` + urlJSON + `, {credentials: 'same-origin', redirect: 'follow'});
  const ct = (r.headers.get('content-type') || '').toLowerCase();
  if (ct === '' || /^text\/|json|xml|javascript/.test(ct)) {
    cb(r.status, r.url, await r.text(), ct, false);
  } else {
    const a = new Uint8Array(await r.arrayBuffer()); let s = '';
    for (let i = 0; i < a.length; i += 0x8000) s += String.fromCharCode.apply(null, a.subarray(i, i + 0x8000));
    cb(r.status, r.url, btoa(s), ct, true);
  }
} catch (e) { cb(-1, '', String(e), '', false); } })();`
}

// isAsset reports URLs that must be fetched from inside a page (never
// navigated to): images and JSON.
func isAsset(u string) bool {
	p, err := url.Parse(u)
	if err != nil {
		return false
	}
	switch strings.ToLower(path.Ext(p.Path)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".avif", ".json":
		return true
	}
	return false
}

// pageFromResult turns an in-page fetch result into a fetch.Page,
// decoding base64 bodies (binary responses such as images).
func pageFromResult(status int, u, body, ct string, b64 bool) (*fetch.Page, error) {
	data := []byte(body)
	if b64 {
		var err error
		if data, err = base64.StdEncoding.DecodeString(body); err != nil {
			return nil, fmt.Errorf("bad binary response for %s: %w", u, err)
		}
	}
	h := http.Header{}
	if ct != "" {
		h.Set("Content-Type", ct)
	}
	return &fetch.Page{URL: u, Status: status, Body: data, Header: h}, nil
}

func origin(u string) string {
	p, err := url.Parse(u)
	if err != nil {
		return ""
	}
	return p.Scheme + "://" + p.Host
}
