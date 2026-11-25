package source

import (
	"context"
	"strings"
	"testing"
)

// Search pages saved 2026-11-25. Each case checks the top hit and that every
// hit is a link the profile can download.
func TestSearchSites(t *testing.T) {
	cases := []struct {
		site, file, query string
		wantTitle         string
		wantURL           string
		wantChapters      int // 0: the page doesn't show a count
		wantCover         bool
	}{
		{"novelfull", "novelfull_search.html", "shadow slave", "Shadow Slave", "https://novelfull.com/shadow-slave.html", 3256, true},
		{"readnovelfull", "readnovelfull_search.html", "shadow slave", "Shadow Slave", "https://readnovelfull.com/shadow-slave.html", 3256, true},
		{"novelping", "novelping_search.html", "borrowed soul", "A Borrowed Soul", "https://novelping.com/book/a-borrowed-soul", 126, true},
		{"freewebnovel", "freewebnovel_search.html", "shadow slave", "Shadow Slave", "https://freewebnovel.com/novel/shadow-slave", 3256, true},
		{"novelfire", "novelfire_search.html", "shadow slave", "Shadow Slave", "https://novelfire.net/book/shadow-slave", 3256, true},
		{"royalroad", "royalroad_search.html", "mother of learning", "Mother of Learning", "https://www.royalroad.com/fiction/21220/mother-of-learning", 109, true},
		{"webnovel", "webnovel_search.html", "shadow slave", "Shadow Slave", "https://www.webnovel.com/book/shadow-slave_22196546206090805", 0, true},
	}
	for _, c := range cases {
		t.Run(c.site, func(t *testing.T) {
			e, ff := newEngine(t, map[string]string{`.`: c.file})
			p := e.profile(c.site)
			if p == nil {
				t.Fatalf("no profile %s", c.site)
			}
			hits, err := e.SearchSite(context.Background(), p, c.query)
			if err != nil {
				t.Fatal(err)
			}
			if len(hits) == 0 {
				t.Fatalf("no hits (fetched %v)", ff.hits)
			}
			if want := strings.ReplaceAll(c.query, " ", "+"); !strings.Contains(ff.hits[len(ff.hits)-1], want) {
				t.Errorf("search URL %s doesn't carry the query", ff.hits[len(ff.hits)-1])
			}
			h := hits[0]
			if h.Title != c.wantTitle || h.URL != c.wantURL {
				t.Errorf("top hit = %q %s, want %q %s", h.Title, h.URL, c.wantTitle, c.wantURL)
			}
			if h.Chapters != c.wantChapters {
				t.Errorf("chapters = %d, want %d (latest %q)", h.Chapters, c.wantChapters, h.Latest)
			}
			if c.wantCover && !strings.HasPrefix(h.Cover, "http") {
				t.Errorf("cover = %q", h.Cover)
			}
			for _, x := range hits {
				m, err := e.Registry.Match(x.URL)
				if err != nil || m.Profile == nil || m.Profile.ID != c.site || m.Kind != "novel" {
					t.Errorf("hit %s isn't a %s novel page", x.URL, c.site)
				}
			}
			t.Logf("%d hits; top: %+v", len(hits), h)
		})
	}
}

func TestSearchableSkipsBrowserSites(t *testing.T) {
	e, _ := newEngine(t, nil)
	ids := func(withOptIn bool) string {
		var s []string
		for _, p := range e.Searchable(withOptIn) {
			s = append(s, p.ID)
		}
		return "," + strings.Join(s, ",") + ","
	}
	if got := ids(false); strings.Contains(got, ",scribblehub,") || strings.Contains(got, ",royalroad,") || !strings.Contains(got, ",webnovel,") || !strings.Contains(got, ",novelfull,") {
		t.Errorf("default: %s", got)
	}
	if got := ids(true); !strings.Contains(got, ",scribblehub,") {
		t.Errorf("opt-in: %s", got)
	}
	e.Browser = nil
	if got := ids(true); strings.Contains(got, ",scribblehub,") || strings.Contains(got, ",royalroad,") || strings.Contains(got, ",webnovel,") || !strings.Contains(got, ",novelfull,") {
		t.Errorf("no browser wired in: %s", got)
	}
}
