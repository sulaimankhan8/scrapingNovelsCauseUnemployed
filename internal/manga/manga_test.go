package manga

import (
	"archive/zip"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestEveryExampleMatchesItsSite(t *testing.T) {
	reg, err := LoadSites("")
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Sites) < 10 {
		t.Fatalf("only %d sites", len(reg.Sites))
	}
	for _, p := range reg.Sites {
		m, err := reg.Match(p.Example)
		if err != nil {
			t.Errorf("%s: example doesn't match: %v", p.ID, err)
			continue
		}
		if m.Profile.ID != p.ID || m.Chapter {
			t.Errorf("%s: example matched %s (chapter=%v)", p.ID, m.Profile.ID, m.Chapter)
		}
	}
}

func TestChapterLinksAndErrors(t *testing.T) {
	reg, _ := LoadSites("")
	cases := map[string]string{
		"https://mangapill.com/chapters/8-10895000/kingdom-chapter-895":                                  "mangapill",
		"https://mangadex.org/chapter/6310f6a1-17ee-4890-b837-2ec1b372905b":                              "mangadex",
		"https://www.webtoons.com/en/fantasy/tower-of-god/season-1-ep-0/viewer?title_no=95&episode_no=1": "webtoons",
		"https://toonily.com/serie/bj-archmage-09cc0281/chapter-1/":                                      "toonily",
		"https://mangakatana.com/manga/absolute-regression.27371/c12":                                    "mangakatana",
		"https://www.natomanga.com/manga/acting-gangster/chapter-1":                                      "natomanga",
		"https://asurascans.com/comics/damn-reincarnation-7de80a82/chapter/5":                            "asura",
	}
	for link, want := range cases {
		m, err := reg.Match(link)
		if err != nil || m.Profile.ID != want || !m.Chapter {
			t.Errorf("%s: got %+v, %v; want chapter of %s", link, m, err, want)
		}
	}
	if _, err := reg.Match("https://mangapill.com/search?q=x"); err == nil || !strings.Contains(err.Error(), "isn't a series") {
		t.Errorf("known site, wrong page: %v", err)
	}
	if _, err := reg.Match("https://example.com/manga/x"); err == nil || !strings.Contains(err.Error(), "isn't a supported site") {
		t.Errorf("unknown site: %v", err)
	}
}

func doc(t *testing.T, html string) *goquery.Document {
	d, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	d.Url, _ = url.Parse("https://site.test/manga/x/chapter-1")
	return d
}

func TestImagesSortedByIndexAndPlaceholdersSkipped(t *testing.T) {
	// ManhuaPlus: real link in src, a loading gif in data-src, out of order.
	d := doc(t, `<div class="separator" data-index="2"><img src="https://cdn.test/b.jpg" data-src="/themes/x/Loading/loading.gif"></div>
		<div class="separator" data-index="0"><img src="https://cdn.test/a.jpg" data-src="/themes/x/Loading/loading.gif"></div>
		<div class="separator" data-index="10"><img src="https://cdn.test/c.jpg"></div>`)
	got := cleanImages(d.Url, selectImages(d, "div.separator img@src|data-src", "data-index"), "")
	want := []string{"https://cdn.test/a.jpg", "https://cdn.test/b.jpg", "https://cdn.test/c.jpg"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("got %v", got)
	}
	// Madara: whitespace around the link, lazy data-src preferred.
	d = doc(t, `<div class="reading-content"><img src="data:image/gif;base64,xx" data-src="
		https://cdn.test/1.jpg "><img src=" /p/2.jpg"></div>`)
	got = cleanImages(d.Url, selectImages(d, ".reading-content img@data-src|src", ""), "")
	if len(got) != 2 || got[0] != "https://cdn.test/1.jpg" || got[1] != "https://site.test/p/2.jpg" {
		t.Errorf("got %v", got)
	}
}

func TestScriptArrayPicksTheRealList(t *testing.T) {
	body := `var ytaw=['https://i.test/decoy.jpg'];
		var thzq=['https://i.test/0.jpg','https://i.test/1.jpg','https://i.test/2.jpg',];`
	got := scriptArray(body)
	if len(got) != 3 || got[0] != "https://i.test/0.jpg" {
		t.Errorf("got %v", got)
	}
}

func TestTitles(t *testing.T) {
	for in, want := range map[string]string{
		"15-eng-li": "Chapter 15", "15-5-eng-li": "Chapter 15.5", "7": "Chapter 7",
		"  Episode   12 ": "Episode 12", "Chapter 3: Title": "Chapter 3: Title",
	} {
		if got := tidyTitle(in); got != want {
			t.Errorf("tidyTitle(%q) = %q, want %q", in, got, want)
		}
	}
	obj := map[string]any{"chapter": "225.00", "title": "", "series_id": float64(24), "token": "abc"}
	if got := fieldOrTemplate(obj, "Chapter {chapter} - {title}", nil); got != "Chapter 225" {
		t.Errorf("template title = %q", got)
	}
	if got := fieldOrTemplate(obj, "/series/{series_id}/{token}", nil); got != "/series/24/abc" {
		t.Errorf("template url = %q", got)
	}
	if got := fieldOrTemplate(map[string]any{"chapter_slug": "chapter-2"}, "{origin}/manga/{slug}/{chapter_slug}",
		map[string]string{"origin": "https://n.test", "slug": "x"}); got != "https://n.test/manga/x/chapter-2" {
		t.Errorf("template with series vars = %q", got)
	}
}

func TestUnwrapJSONFromBrowserPage(t *testing.T) {
	html := `<html><body><pre>{"data":{"chapters":[1,2]}}</pre></body></html>`
	got := unwrapJSON([]byte(html), doc(t, html))
	if string(got) != `{"data":{"chapters":[1,2]}}` {
		t.Errorf("got %s", got)
	}
}

func TestMergeKeepsSavedChaptersAndFolders(t *testing.T) {
	b := &Book{NextKey: 1}
	added := mergeChapters(b, []ChapterRef{{URL: "https://s.test/c1"}, {URL: "https://s.test/c2"}, {URL: "https://s.test/c3"}})
	if len(added) != 3 || b.NextKey != 4 {
		t.Fatalf("first merge: %v next=%d", added, b.NextKey)
	}
	b.Chapters[0].State, b.Chapters[2].State = StateOK, StateOK
	// The site inserts c1.5, drops c3 and adds c4.
	added = mergeChapters(b, []ChapterRef{{URL: "https://s.test/c1"}, {URL: "https://s.test/c1.5"}, {URL: "https://www.s.test/c2/"}, {URL: "https://s.test/c4"}})
	if len(added) != 2 {
		t.Errorf("added %v, want c1.5 and c4", added)
	}
	keys := map[string]int{}
	for _, c := range b.Chapters {
		keys[strings.TrimPrefix(strings.TrimPrefix(c.URL, "https://"), "www.")] = c.Key
	}
	if keys["s.test/c1"] != 1 || keys["s.test/c2/"] != 2 || keys["s.test/c3"] != 3 {
		t.Errorf("existing chapters lost their keys: %v", keys)
	}
	last := b.Chapters[len(b.Chapters)-1]
	if !last.Removed || last.State != StateOK || last.Key != 3 {
		t.Errorf("saved chapter the site dropped should be kept and marked: %+v", last)
	}
	for i, c := range b.Chapters {
		if c.Number != i+1 {
			t.Errorf("numbering: %+v", c)
		}
	}
}

func TestImageTypeRejectsPages(t *testing.T) {
	if imageType([]byte("<!DOCTYPE html><title>Just a moment...</title>")) != "" {
		t.Error("HTML accepted as an image")
	}
	if imageType([]byte{0xFF, 0xD8, 0xFF, 0xE0}) != ".jpg" || imageType([]byte("RIFF\x00\x00\x00\x00WEBPVP8 ")) != ".webp" {
		t.Error("real images not recognised")
	}
}

func TestCBZAndHandler(t *testing.T) {
	st, _ := OpenStore(t.TempDir())
	b := &Book{Slug: "demo", Title: "Demo: Part/1", Type: "Manga", Chapters: []Chapter{
		{Key: 1, Number: 1, State: StateOK}, {Key: 2, Number: 2, State: StatePending}, {Key: 5, Number: 3, State: StateOK},
	}}
	if err := st.Save(b); err != nil {
		t.Fatal(err)
	}
	jpg := []byte{0xFF, 0xD8, 0xFF, 0xE0, 1, 2, 3}
	for _, k := range []int{1, 5} {
		os.MkdirAll(st.ChapterDir("demo", k), 0o755)
		for _, n := range []string{"001.jpg", "002.jpg"} {
			os.WriteFile(filepath.Join(st.ChapterDir("demo", k), n), jpg, 0o644)
		}
	}
	dest := filepath.Join(t.TempDir(), FileName(b.Title))
	if filepath.Base(dest) != "Demo Part 1.cbz" {
		t.Errorf("file name %q", filepath.Base(dest))
	}
	res, err := WriteCBZ(st, b, dest, 0, 0)
	if err != nil || res.Chapters != 2 || res.Pages != 4 {
		t.Fatalf("cbz: %+v %v", res, err)
	}
	zr, err := zip.OpenReader(dest)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	zr.Close()
	if strings.Join(names, ",") != "c0001-p001.jpg,c0001-p002.jpg,c0003-p001.jpg,c0003-p002.jpg,ComicInfo.xml" {
		t.Errorf("entries %v", names)
	}

	h := &Handler{Store: st}
	for path, want := range map[string]int{
		"/mangalib/demo/ch/00005/002.jpg":    200,
		"/mangalib/demo/series.json":         404,
		"/mangalib/demo/ch/00005/../../x":    404,
		"/mangalib/../demo/ch/00001/001.jpg": 404,
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, &http.Request{Method: "GET", URL: &url.URL{Path: path}})
		if rec.Code != want {
			t.Errorf("%s: %d, want %d", path, rec.Code, want)
		}
	}
}

func TestImageSizeOfRealPages(t *testing.T) {
	// Real downloaded pages, when the developer's test library is present.
	lib := os.Getenv("MANGA_TEST_LIB")
	if lib == "" {
		t.Skip("set MANGA_TEST_LIB to a library folder to check real pages")
	}
	files, _ := filepath.Glob(filepath.Join(lib, "*", "ch", "*", "001.*"))
	if len(files) == 0 {
		t.Skip("no pages in MANGA_TEST_LIB")
	}
	for _, f := range files {
		if w, h := imageSize(f); w <= 0 || h <= 0 {
			t.Errorf("%s: no size", f)
		} else {
			t.Logf("%s: %dx%d", filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(f)))), w, h)
		}
	}
}
