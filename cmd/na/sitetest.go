package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"novelarchiver/internal/sites"
	"novelarchiver/internal/source"
)

// testSites checks every site profile against the live site: discover its
// example novel, fetch the first free chapter, and search the site for the
// novel's title, which must find the example. only limits it to one site
// id; optIn also runs searches that usually ask for a human check.
func testSites(ctx context.Context, eng *source.Engine, only string, optIn bool) int {
	failed := 0
	fail := func(format string, a ...any) {
		fmt.Printf("  FAIL: "+format+"\n", a...)
		failed++
	}
	for _, p := range eng.Registry.Profiles {
		if p.Example == "" || (only != "" && p.ID != only) {
			continue
		}
		if ctx.Err() != nil {
			break
		}
		start := time.Now()
		fmt.Printf("\n=== %s (%s)\n", p.Name, p.Example)
		n, err := eng.Discover(ctx, p.Example, nil)
		if err != nil {
			fail("discover: %v", err)
			continue
		}
		locked := 0
		for _, c := range n.Chapters {
			if c.Locked {
				locked++
			}
		}
		fmt.Printf("  %q by %q: %d chapters (%d locked, %s), cover=%v\n", n.Title, n.Author, len(n.Chapters), locked, n.TOCMode, n.Cover != "")

		var ref *source.ChapterRef
		for i := range n.Chapters {
			if !n.Chapters[i].Locked {
				ref = &n.Chapters[i]
				break
			}
		}
		if ref == nil && n.FirstChapterURL != "" {
			ref = &source.ChapterRef{Number: 1, Title: "Chapter 1", URL: n.FirstChapterURL}
		}
		if ref == nil {
			fail("no free chapter to fetch")
		} else if ch, err := eng.FetchChapter(ctx, n, *ref); err != nil {
			fail("chapter %d: %v", ref.Number, err)
		} else {
			doc, _ := goquery.NewDocumentFromReader(strings.NewReader(ch.HTML))
			text := strings.Join(strings.Fields(doc.Text()), " ")
			fmt.Printf("  chapter %d %q: %d characters, starts %q\n", ref.Number, ch.Title, len(text), clip(text, 70))
			if len(text) < 500 {
				fail("chapter %d has only %d characters of text", ref.Number, len(text))
			}
		}

		if sites.Str(p.Search, "page") != "" {
			switch {
			case sites.Bool(p.Search, "optIn") && !optIn:
				fmt.Println("  search: skipped (asks for a human check; run with -optin)")
			case p.NeedsBrowser("search") && eng.Browser == nil:
				fmt.Println("  search: skipped (needs the site browser)")
			default:
				q := searchWords(n.Title)
				hits, err := eng.SearchSite(ctx, p, q)
				if err != nil {
					fail("search %q: %v", q, err)
					break
				}
				pos := 0
				for i, h := range hits {
					if eng.Registry.NovelKey(h.URL) == eng.Registry.NovelKey(n.URL) {
						pos = i + 1
						break
					}
				}
				fmt.Printf("  search %q: %d results, example at #%d\n", q, len(hits), pos)
				if pos == 0 {
					fail("search for %q didn't list the example", q)
				}
			}
		}
		fmt.Printf("  (%s)\n", time.Since(start).Round(time.Second))
	}
	fmt.Printf("\n%d check(s) failed\n", failed)
	return failed
}

// searchWords is the title, cut to its first five words.
func searchWords(title string) string {
	w := strings.Fields(title)
	if len(w) > 5 {
		w = w[:5]
	}
	return strings.Join(w, " ")
}

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
