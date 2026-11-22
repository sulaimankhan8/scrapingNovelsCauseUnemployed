// Package browser provides a fetch.Fetcher backed by a real browser engine
// (WebView2 on Windows). Pages load exactly as they would in Edge, in a
// window the user can see. When a site shows a "Verify you are human" check,
// the window is brought to the front so the user can complete it; the app
// never automates or bypasses such checks.
package browser

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
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

func origin(u string) string {
	p, err := url.Parse(u)
	if err != nil {
		return ""
	}
	return p.Scheme + "://" + p.Host
}
