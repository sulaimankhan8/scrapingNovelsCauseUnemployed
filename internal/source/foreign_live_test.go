package source

import (
	"context"
	"os"
	"testing"
	"time"

	"novelarchiver/internal/fetch"
	"novelarchiver/internal/sites"
)

// TestForeignLookupLive: the English site's page gives the Chinese title,
// and the Chinese site finds the novel by it. NA_LIVE=1 to run.
func TestForeignLookupLive(t *testing.T) {
	if os.Getenv("NA_LIVE") == "" {
		t.Skip("set NA_LIVE=1")
	}
	reg, err := sites.Load("")
	if err != nil {
		t.Fatal(err)
	}
	e := &Engine{Registry: reg, HTTP: fetch.NewHTTP(fetch.Options{})}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	for _, u := range []string{"https://novelfull.com/lord-of-the-mysteries.html", "https://readnovelfull.com/lord-of-the-mysteries-v1.html"} {
		names, err := e.AltNames(ctx, u)
		t.Logf("%s: %q err=%v", u, names, err)
	}
	hits, err := e.SearchSite(ctx, e.profile("uukanshu"), "诡秘之主")
	t.Logf("uukanshu: %+v err=%v", hits, err)
	if len(hits) == 0 {
		t.Error("uukanshu found nothing")
	}
}
