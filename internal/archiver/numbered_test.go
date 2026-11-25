package archiver

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"novelarchiver/internal/fetch"
	"novelarchiver/internal/library"
	"novelarchiver/internal/sites"
	"novelarchiver/internal/source"
)

// fakeSite serves EmpireNovel-shaped chapter pages from a map of
// URL -> next URL. Missing URLs answer 404.
type fakeSite map[string]string

func (s fakeSite) Get(_ context.Context, u, _ string) (*fetch.Page, error) {
	next, ok := s[u]
	if !ok {
		return nil, &fetch.HTTPError{Status: 404, URL: u}
	}
	link := ""
	if next != "" {
		link = fmt.Sprintf(`<a id="chapter-right" href="%s">Next</a>`, next)
	}
	body := fmt.Sprintf(`<html><body><div id="read-novel"><h3>%s</h3><p>Text of %s, long enough to count as a chapter.</p></div>%s</body></html>`, u, u, link)
	return &fetch.Page{URL: u, Status: 200, Body: []byte(body), Header: http.Header{}}, nil
}

func TestNumberedTOCGapsAndMovedChapters(t *testing.T) {
	reg, err := sites.Load("")
	if err != nil {
		t.Fatal(err)
	}
	const base = "https://www.empirenovel.com/novel/x/"
	u := func(n string) string { return base + n }
	// 1 -> 2 -> (3 and 4 skipped) -> 5 -> 6-1 (6 lives at another address) -> 7
	site := fakeSite{
		u("1"): u("2"), u("2"): u("5"), u("5"): u("6-1"), u("6-1"): u("7"), u("7"): "",
	}
	st, err := library.Open(filepath.Join(t.TempDir(), "lib"))
	if err != nil {
		t.Fatal(err)
	}
	a := &Archiver{Store: st, Engine: &source.Engine{Registry: reg, HTTP: site, Browser: site}}
	b := &library.Book{Slug: "x", SiteID: "empirenovel", SiteName: "EmpireNovel", URL: base[:len(base)-1], TOCMode: "list"}
	for i := 1; i <= 7; i++ {
		b.Chapters = append(b.Chapters, library.Chapter{Number: i, Title: fmt.Sprintf("Chapter %d", i), URL: u(fmt.Sprint(i))})
	}
	if err := st.Save(b); err != nil {
		t.Fatal(err)
	}
	novel := &source.Novel{SiteID: "empirenovel", SiteName: "EmpireNovel", URL: b.URL}

	want := map[int]string{1: library.StateOK, 2: library.StateOK, 3: library.StateGone, 4: library.StateGone, 5: library.StateOK, 6: library.StateOK, 7: library.StateOK}
	for i := range b.Chapters {
		c := &b.Chapters[i]
		state, _, _, errMsg, alt := a.downloadOne(context.Background(), b, novel, *c, Options{})
		c.State, c.Error = state, errMsg
		if alt != "" {
			c.AltURL = alt
		}
		if state != want[c.Number] {
			t.Errorf("chapter %d: state %q (%s), want %q", c.Number, state, errMsg, want[c.Number])
		}
	}
	if got := b.Chapters[5].AltURL; got != u("6-1") {
		t.Errorf("chapter 6 AltURL = %q, want %q", got, u("6-1"))
	}
	if b.Chapters[2].AltURL != "" || b.Chapters[3].AltURL != "" {
		t.Errorf("skipped chapters must not borrow chapter 5's address: %q %q", b.Chapters[2].AltURL, b.Chapters[3].AltURL)
	}
	// Retry (OnlyFailed) has nothing left to do.
	if idx, _ := a.selectChapters(b, Options{OnlyFailed: true}); len(idx) != 0 {
		t.Errorf("retry would fetch %v again", idx)
	}
	// A refreshed TOC keeps the states and the real address.
	refs := make([]source.ChapterRef, 0, 7)
	for i := 1; i <= 7; i++ {
		refs = append(refs, source.ChapterRef{Number: i, URL: u(fmt.Sprint(i))})
	}
	if err := st.Save(b); err != nil {
		t.Fatal(err)
	}
	nb, err := a.BookFromNovel(&source.Novel{SiteID: "empirenovel", URL: b.URL, Title: "X", TOCMode: "list", Chapters: refs})
	if err != nil {
		t.Fatal(err)
	}
	if nb.Chapters[2].State != library.StateGone || nb.Chapters[5].AltURL != u("6-1") {
		t.Errorf("after refresh: ch3 %q, ch6 alt %q", nb.Chapters[2].State, nb.Chapters[5].AltURL)
	}
	if _, failed, _, pending := nb.Counts(); failed != 0 || pending != 0 {
		t.Errorf("counts failed=%d pending=%d, want 0 0", failed, pending)
	}
}
