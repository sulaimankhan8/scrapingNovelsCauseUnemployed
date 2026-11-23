package browser

import "testing"

func TestChallengeBody(t *testing.T) {
	check := `<html><head><title>www.empirenovel.com</title></head><body><h1>Performing security verification</h1><div class="cf-turnstile"></div><script src="/cdn-cgi/challenge-platform/h/g/orchestrate"></script></body></html>`
	if !challengeBody(check) {
		t.Fatal("Cloudflare check page with a non-challenge title must still count as a check")
	}
	chapter := `<html><head><title>Chapter 1 - Investing</title></head><body><div id="read-novel"><p>She smiled.</p></div></body></html>`
	if challengeBody(chapter) {
		t.Fatal("a normal chapter page is not a check")
	}
}
