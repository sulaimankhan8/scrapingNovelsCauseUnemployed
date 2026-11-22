package reader

import (
	"net/http/httptest"
	"strings"
	"testing"

	"novelarchiver/internal/library"
)

func setup(t *testing.T) *library.Store {
	store, err := library.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	b := &library.Book{Slug: "test-novel", Title: "Test Novel", Author: "A", SiteName: "Site", TOCMode: "list"}
	for i := 1; i <= 3; i++ {
		b.Chapters = append(b.Chapters, library.Chapter{Number: i, Title: "Ch " + string(rune('0'+i)), State: library.StateOK})
		if err := store.WriteChapter(b.Slug, i, `<h1 class="ch-title">Ch</h1><p class="sys">[Status]</p><p>Body `+string(rune('0'+i))+`</p>`); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Save(b); err != nil {
		t.Fatal(err)
	}
	return store
}

func get(h *Handler, path string) (int, string) {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec.Code, rec.Body.String()
}

func TestReaderPages(t *testing.T) {
	store := setup(t)
	h := &Handler{Store: store}

	code, body := get(h, "/library/test-novel/")
	if code != 200 || !strings.Contains(body, "Test Novel") || !strings.Contains(body, `href="2"`) {
		t.Fatalf("toc: %d %.300s", code, body)
	}
	code, body = get(h, "/library/test-novel/2")
	if code != 200 || !strings.Contains(body, `<p class="sys">[Status]</p>`) || !strings.Contains(body, `href="1" rel="prev"`) || !strings.Contains(body, `href="3" rel="next"`) {
		t.Fatalf("chapter: %d %.500s", code, body)
	}
	if strings.Contains(body, "fonts.googleapis") {
		t.Errorf("reader must not load web fonts")
	}
	if code, _ := get(h, "/library/../secret/"); code != 404 {
		t.Errorf("path escape not rejected: %d", code)
	}
	if code, _ := get(h, "/library/test-novel/img/../../x"); code == 200 {
		t.Errorf("image path escape served")
	}
}

func TestDeleteChapterRange(t *testing.T) {
	store := setup(t)
	n, err := store.DeleteChapters("test-novel", 1, 2)
	if err != nil || n != 2 {
		t.Fatalf("deleted %d, %v", n, err)
	}
	b, _ := store.Load("test-novel")
	ok, _, _, pending := b.Counts()
	if ok != 1 || pending != 2 {
		t.Errorf("ok=%d pending=%d", ok, pending)
	}
	code, body := get(&Handler{Store: store}, "/library/test-novel/1")
	if code != 200 || !strings.Contains(body, "isn't downloaded") {
		t.Errorf("deleted chapter page: %d", code)
	}
	if _, err := store.DeleteChapters("test-novel", 5, 2); err == nil {
		t.Errorf("reversed range accepted")
	}
}
