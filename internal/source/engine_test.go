package source

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"novelarchiver/internal/fetch"
	"novelarchiver/internal/sites"
)

// fixtureFetcher serves saved pages from test-fixtures/pages by URL pattern.
type fixtureFetcher struct {
	t      *testing.T
	routes []route
	hits   []string
}

type route struct {
	re   *regexp.Regexp
	file string
}

func (f *fixtureFetcher) Get(_ context.Context, u, _ string) (*fetch.Page, error) {
	f.hits = append(f.hits, u)
	for _, r := range f.routes {
		if r.re.MatchString(u) {
			body, err := os.ReadFile(filepath.Join("..", "..", "test-fixtures", "pages", r.file))
			if err != nil {
				f.t.Skipf("fixture %s missing: %v", r.file, err)
			}
			return &fetch.Page{URL: u, Status: 200, Body: body}, nil
		}
	}
	return nil, &fetch.HTTPError{Status: 404, URL: u}
}

func newEngine(t *testing.T, routes map[string]string) (*Engine, *fixtureFetcher) {
	reg, err := sites.Load("")
	if err != nil {
		t.Fatal(err)
	}
	ff := &fixtureFetcher{t: t}
	for pat, file := range routes {
		ff.routes = append(ff.routes, route{regexp.MustCompile(pat), file})
	}
	return &Engine{Registry: reg, HTTP: ff, Browser: ff}, ff // fixtures stand in for both fetchers
}

type siteCase struct {
	name        string
	routes      map[string]string
	input       string
	wantSite    string
	wantTitle   string
	wantAuthor  string
	minChapters int
	wantMode    string
	chapterURL  string // fetch this chapter and check content
	wantInText  string
	wantNext    string
	inputChap   int
}

func TestSites(t *testing.T) {
	cases := []siteCase{
		{
			name:   "novelfull novel page",
			routes: map[string]string{`novelfull\.com/lord-of-the-mysteries\.html`: "novelfull_novel.html", `/chapter-1-crimson\.html`: "novelfull_chapter.html"},
			input:  "https://novelfull.com/lord-of-the-mysteries.html", wantSite: "novelfull",
			wantTitle: "Lord of the Mysteries", wantAuthor: "Cuttlefish", minChapters: 50, wantMode: "list",
			chapterURL: "https://novelfull.com/lord-of-the-mysteries/chapter-1-crimson.html", wantInText: "Painful", wantNext: "chapter-2-situation.html",
		},
		{
			name:   "novelfull chapter link",
			routes: map[string]string{`novelfull\.com/lord-of-the-mysteries\.html`: "novelfull_novel.html", `/chapter-1-crimson\.html`: "novelfull_chapter.html"},
			input:  "https://novelfull.com/lord-of-the-mysteries/chapter-1-crimson.html", wantSite: "novelfull",
			wantTitle: "Lord of the Mysteries", minChapters: 50, wantMode: "list", inputChap: 1,
		},
		{
			name: "readnovelfull",
			routes: map[string]string{`readnovelfull\.com/lord-of-the-mysteries-v1\.html`: "readnovelfull_novel.html",
				`ajax/chapter-archive\?novelId=459`: "readnovelfull_toc.html", `chapter-1-crimson-v1\.html`: "readnovelfull_chapter.html"},
			input: "https://readnovelfull.com/lord-of-the-mysteries-v1.html", wantSite: "readnovelfull",
			wantTitle: "Lord of the Mysteries", minChapters: 1400, wantMode: "list",
			chapterURL: "https://readnovelfull.com/lord-of-the-mysteries/chapter-1-crimson-v1.html", wantInText: "Painful", wantNext: "chapter-2-situation.html",
		},
		{
			name: "royalroad chapter link",
			routes: map[string]string{`royalroad\.com/fiction/21220/mother-of-learning$`: "royalroad_novel.html",
				`/chapter/301778/`: "royalroad_chapter.html"},
			input:    "https://www.royalroad.com/fiction/21220/mother-of-learning/chapter/301778/1-good-morning-brother",
			wantSite: "royalroad", wantTitle: "Mother of Learning", wantAuthor: "nobody103", minChapters: 100, wantMode: "list", inputChap: 1,
			chapterURL: "https://www.royalroad.com/fiction/21220/mother-of-learning/chapter/301778/1-good-morning-brother",
			wantInText: "Good morning, brother", wantNext: "/chapter/301781/",
		},
		{
			name: "wuxiaworld",
			routes: map[string]string{`wuxiaworld\.com/novel/martial-world$`: "wuxiaworld_novel.html",
				`mw-chapter-1$`: "wuxiaworld_chapter.html"},
			input: "https://www.wuxiaworld.com/novel/martial-world", wantSite: "wuxiaworld",
			wantTitle: "Martial World", wantMode: "crawl",
			chapterURL: "https://www.wuxiaworld.com/novel/martial-world/mw-chapter-1", wantInText: "Lin Ming", wantNext: "/novel/martial-world/mw-chapter-2",
		},
		{
			name: "freewebnovel",
			routes: map[string]string{`freewebnovel\.com/novel/last-gun-alchemist$`: "freewebnovel_novel.html",
				`chapter-1$`: "freewebnovel_chapter.html"},
			input: "https://freewebnovel.com/novel/last-gun-alchemist", wantSite: "freewebnovel",
			wantTitle: "Last Gun Alchemist", wantAuthor: "JOE_JOE_Z", minChapters: 776, wantMode: "list",
			chapterURL: "https://freewebnovel.com/novel/last-gun-alchemist/chapter-1", wantInText: "Ezra woke up", wantNext: "chapter-2",
		},
		{
			name: "novelfire",
			routes: map[string]string{`novelfire\.net/book/lord-of-the-mysteries$`: "novelfire_novel.html",
				`chapters\?page=\d+`: "novelfire_toc.html"},
			input: "https://novelfire.net/book/lord-of-the-mysteries", wantSite: "novelfire",
			wantTitle: "Lord of the Mysteries", minChapters: 100, wantMode: "list",
		},
		{
			name: "webnovel",
			routes: map[string]string{`webnovel\.com/book/32824130400116105$`: "webnovel_novel.html", `/catalog$`: "webnovel_catalog.html",
				`lectures_88133292359994362$`: "webnovel_chapter.html"},
			input:    "https://www.webnovel.com/book/32824130400116105",
			wantSite: "webnovel", wantTitle: "CLEAVER OF SIN", wantAuthor: "LORDTEE", minChapters: 1274, wantMode: "list",
			chapterURL: "https://www.webnovel.com/book/cleaver-of-sin_32824130400116105/lectures_88133292359994362",
			wantInText: "no older than twenty-three", wantNext: "/book/32824130400116105/88133316921845011",
		},
		{
			name: "empirenovel chapter link",
			routes: map[string]string{`empirenovel\.com/novel/genetic-ascension$`: "empirenovel_novel.html",
				`genetic-ascension/1$`: "empirenovel_chapter.html"},
			input:    "https://www.empirenovel.com/novel/genetic-ascension/1",
			wantSite: "empirenovel", wantTitle: "Genetic Ascension", wantAuthor: "Awespec", minChapters: 2531, wantMode: "list", inputChap: 1,
			chapterURL: "https://www.empirenovel.com/novel/genetic-ascension/1",
			wantInText: "Sylas shrugged off his lab coat", wantNext: "/novel/genetic-ascension/2",
		},
		{
			name: "scribblehub",
			routes: map[string]string{`scribblehub\.com/series/747093/collide-gamer/$`: "scribblehub_novel.html",
				`chapter/747095/$`: "scribblehub_chapter.html"},
			input:    "https://www.scribblehub.com/series/747093/collide-gamer/",
			wantSite: "scribblehub", wantTitle: "Collide Gamer", wantAuthor: "Funatic", wantMode: "crawl",
			chapterURL: "https://www.scribblehub.com/read/747093-collide-gamer/chapter/747095/",
			wantInText: "The following story demands some context", wantNext: "/chapter/747104/",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, ff := newEngine(t, c.routes)
			n, err := e.Discover(context.Background(), c.input, nil)
			if c.minChapters == 0 && c.wantMode == "" {
				// metadata-only check; TOC may legitimately fail.
				if n == nil {
					t.Skipf("discover: %v", err)
				}
			} else if err != nil {
				t.Fatalf("discover: %v (fetched %v)", err, ff.hits)
			}
			if n.SiteID != c.wantSite {
				t.Errorf("site = %q, want %q", n.SiteID, c.wantSite)
			}
			if !strings.Contains(n.Title, c.wantTitle) {
				t.Errorf("title = %q, want %q", n.Title, c.wantTitle)
			}
			if c.wantAuthor != "" && !strings.Contains(n.Author, c.wantAuthor) {
				t.Errorf("author = %q, want %q", n.Author, c.wantAuthor)
			}
			if n.Cover == "" {
				t.Errorf("no cover")
			}
			if c.wantMode != "" && n.TOCMode != c.wantMode {
				t.Errorf("tocMode = %q, want %q (first=%q)", n.TOCMode, c.wantMode, n.FirstChapterURL)
			}
			if len(n.Chapters) < c.minChapters {
				t.Errorf("chapters = %d, want >= %d", len(n.Chapters), c.minChapters)
			}
			if c.wantSite == "webnovel" && (n.Chapters[0].Title != "Lectures" || len(n.Warnings) == 0) {
				t.Errorf("webnovel: first=%q warnings=%v", n.Chapters[0].Title, n.Warnings)
			}
			if c.inputChap != 0 && n.InputChapter != c.inputChap {
				t.Errorf("inputChapter = %d, want %d", n.InputChapter, c.inputChap)
			}
			if len(n.Chapters) > 0 && (n.Chapters[0].Number != 1 || n.Chapters[0].Title == "") {
				t.Errorf("first chapter = %+v", n.Chapters[0])
			}
			t.Logf("%s: %q by %q, %d chapters, mode=%s, first=%s, warnings=%v", n.SiteID, n.Title, n.Author, len(n.Chapters), n.TOCMode, n.FirstChapterURL, n.Warnings)

			if c.chapterURL == "" {
				return
			}
			ch, err := e.FetchChapter(context.Background(), n, ChapterRef{Number: 1, Title: "Chapter 1", URL: c.chapterURL})
			if err != nil {
				t.Fatalf("fetch chapter: %v", err)
			}
			if !strings.Contains(ch.HTML, c.wantInText) {
				t.Errorf("chapter HTML missing %q: %.300s", c.wantInText, ch.HTML)
			}
			if !strings.Contains(ch.NextURL, c.wantNext) {
				t.Errorf("next = %q, want contains %q", ch.NextURL, c.wantNext)
			}
			if strings.Contains(ch.HTML, "<iframe") || strings.Contains(ch.HTML, "<script") {
				t.Errorf("chapter still contains iframe/script")
			}
			t.Logf("chapter %q: %d bytes, next=%s", ch.Title, len(ch.HTML), ch.NextURL)
		})
	}
}

func TestMatchURLs(t *testing.T) {
	reg, err := sites.Load("")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"https://novelbin.xyz/novel/lord-of-the-mysteries":                                                 "novelbin novel",
		"https://novelbin.xyz/read/lord-of-the-mysteries/chapter-1":                                        "novelbin chapter",
		"https://novelfull.com/lord-of-the-mysteries.html":                                                 "novelfull novel",
		"https://novelfull.com/lord-of-the-mysteries/chapter-1-crimson.html":                               "novelfull chapter",
		"https://readnovelfull.com/lord-of-the-mysteries-v1.html":                                          "readnovelfull novel",
		"https://readnovelfull.com/lord-of-the-mysteries/chapter-1-crimson-v1.html":                        "readnovelfull chapter",
		"https://www.royalroad.com/fiction/21220/mother-of-learning":                                       "royalroad novel",
		"https://www.royalroad.com/fiction/21220/mother-of-learning/chapter/301778/1-good-morning-brother": "royalroad chapter",
		"https://www.wuxiaworld.com/novel/martial-world":                                                   "wuxiaworld novel",
		"https://www.wuxiaworld.com/novel/martial-world/mw-chapter-1":                                      "wuxiaworld chapter",
		"https://freewebnovel.com/novel/last-gun-alchemist":                                                "freewebnovel novel",
		"https://freewebnovel.com/novel/last-gun-alchemist/chapter-1":                                      "freewebnovel chapter",
		"https://novelfire.net/book/lord-of-the-mysteries":                                                 "novelfire novel",
		"https://novelphoenix.com/novel/lord-of-the-mysteries/chapter-1":                                   "novelfire chapter",
		"https://www.webnovel.com/book/32824130400116105":                                                  "webnovel novel",
		"https://www.scribblehub.com/series/2/":                                                            "scribblehub novel",
		"https://www.empirenovel.com/novel/genetic-ascension":                                              "empirenovel novel",
		"https://www.empirenovel.com/novel/genetic-ascension/12":                                           "empirenovel chapter",
		"https://www.scribblehub.com/read/747093-collide-gamer/chapter/747095/":                            "scribblehub chapter",
		"novelfull.com/lord-of-the-mysteries.html":                                                         "novelfull novel",
		"https://example.org/some-story/chapter-3":                                                         "generic unknown",
	}
	for u, w := range want {
		m, err := reg.Match(u)
		if err != nil {
			t.Fatalf("%s: %v", u, err)
		}
		id := "generic"
		if m.Profile != nil {
			id = m.Profile.ID
		}
		if got := fmt.Sprintf("%s %s", id, m.Kind); got != w {
			t.Errorf("%s: got %q, want %q", u, got, w)
		}
	}
}
