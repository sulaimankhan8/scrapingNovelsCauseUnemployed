package mobile

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

type sink struct{ events []string }

func (s *sink) Emit(event, payload string) { s.events = append(s.events, event+" "+payload) }

func TestCallAndServe(t *testing.T) {
	c, err := New(t.TempDir(), &sink{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := c.Call("GetNovels", "[]")
	if err != nil || out != "[]" {
		t.Fatalf("GetNovels = %q, %v", out, err)
	}
	out, err = c.Call("GetSystemStats", "")
	if err != nil {
		t.Fatal(err)
	}
	var stats map[string]any
	if json.Unmarshal([]byte(out), &stats) != nil || stats["totalNovels"].(float64) != 0 {
		t.Fatalf("stats = %s", out)
	}
	if _, err := c.Call("Nope", "[]"); err == nil {
		t.Errorf("unknown method accepted")
	}
	if _, err := c.Call("DeleteChapters", `["missing", 1, 2]`); err == nil {
		t.Errorf("delete on missing novel succeeded")
	}
	r := c.Serve("/library/missing/")
	if r.Status != 404 || !strings.HasPrefix(r.MimeType, "text/") {
		t.Errorf("serve = %d %s", r.Status, r.MimeType)
	}
}

func TestMangaOnAndroid(t *testing.T) {
	dir := t.TempDir()
	c, err := New(filepath.Join(dir, "library"), &sink{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Shutdown()
	for method, want := range map[string]string{
		"Manga.Available": "true",
		"Manga.Library":   "[]",
	} {
		if out, err := c.Call(method, "[]"); err != nil || out != want {
			t.Errorf("%s = %q, %v", method, out, err)
		}
	}
	if out, _ := c.Call("Manga.Recognises", `["https://mangadex.org/title/801513ba-a712-498c-8f57-cae55b38cc92/x"]`); out != "true" {
		t.Errorf("MangaDex link not recognised: %s", out)
	}
	if out, _ := c.Call("Manga.Recognises", `["https://novelfull.com/x.html"]`); out != "false" {
		t.Errorf("novel link taken as manga: %s", out)
	}
	out, err := c.Call("Manga.Sites", "[]")
	if err != nil || strings.Count(out, `"id"`) < 10 {
		t.Errorf("sites = %.80s, %v", out, err)
	}
	// The manga library sits beside the novels, not inside them.
	out, _ = c.Call("Manga.LibraryFolder", "[]")
	if !strings.Contains(out, "manga") || strings.Contains(out, filepath.Join("library", "manga")) {
		t.Errorf("manga folder = %s", out)
	}
	if _, err := c.Call("Manga.Nope", "[]"); err == nil {
		t.Errorf("unknown manga method accepted")
	}
	if _, err := c.Call("Manga.Series", `["missing"]`); err == nil {
		t.Errorf("missing series opened")
	}
	if r := c.Serve("/mangalib/missing/cover.jpg"); r.Status != 404 {
		t.Errorf("manga serve = %d", r.Status)
	}
	if _, err := c.ExportCBZ("missing", dir); err == nil {
		t.Errorf("CBZ of a missing series succeeded")
	}
}
