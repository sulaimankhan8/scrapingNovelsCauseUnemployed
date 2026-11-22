package service

import (
	"os"
	"path/filepath"
	"testing"

	"novelarchiver/internal/library"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	svc := New(filepath.Join(t.TempDir(), "library"))
	if err := svc.ready(); err != nil {
		t.Fatal(err)
	}
	return svc
}

func addBook(t *testing.T, s *library.Store, slug, url string, n int, okUpTo int) {
	t.Helper()
	b := &library.Book{Slug: slug, Title: slug, URL: url, SiteName: "Test", TOCMode: "list"}
	for i := 1; i <= n; i++ {
		c := library.Chapter{Number: i, Title: "Chapter", URL: url + "/" + string(rune('a'+i)), Index: i}
		if i <= okUpTo {
			c.State = library.StateOK
			if err := s.WriteChapter(slug, i, "<p>text</p>"); err != nil {
				t.Fatal(err)
			}
		}
		b.Chapters = append(b.Chapters, c)
	}
	if err := s.Save(b); err != nil {
		t.Fatal(err)
	}
}

func TestBackupRestoreRoundTripAndMerge(t *testing.T) {
	src := newTestService(t)
	addBook(t, src.store, "alpha", "https://example.com/alpha", 5, 5)
	addBook(t, src.store, "beta", "https://example.com/beta", 3, 3)
	if err := src.store.SetReading("alpha", 4); err != nil {
		t.Fatal(err)
	}
	if _, err := src.SaveSettings(Settings{Theme: "sepia", Speed: "fast", Revise: "off"}); err != nil {
		t.Fatal(err)
	}
	zip := filepath.Join(t.TempDir(), "backup.zip")
	info, err := src.Backup(zip)
	if err != nil {
		t.Fatal(err)
	}
	if info.Novels != 2 || info.Chapters != 8 {
		t.Fatalf("backup info = %+v", info)
	}

	// A device that already has alpha with only 2 chapters and an earlier
	// reading place.
	dst := newTestService(t)
	addBook(t, dst.store, "alpha", "https://example.com/alpha", 5, 2)
	if err := dst.store.SetReading("alpha", 1); err != nil {
		t.Fatal(err)
	}
	insp, err := dst.InspectBackup(zip)
	if err != nil {
		t.Fatal(err)
	}
	if insp.NewHere != 1 || insp.Merged != 1 {
		t.Fatalf("inspect = %+v", insp)
	}
	res, err := dst.Restore(zip)
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 1 || res.Merged != 1 || res.Chapters != 3 {
		t.Fatalf("restore = %+v", res)
	}
	a, err := dst.store.Load("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if ok, _, _, _ := a.Counts(); ok != 5 {
		t.Fatalf("alpha has %d saved chapters after merge, want 5", ok)
	}
	if _, err := os.Stat(dst.store.ChapterPath("alpha", 5)); err != nil {
		t.Fatalf("chapter file not restored: %v", err)
	}
	if r := dst.store.Reading("alpha"); r.Chapter != 4 {
		t.Fatalf("reading place = %d, want the further one (4)", r.Chapter)
	}
	if _, err := dst.store.Load("beta"); err != nil {
		t.Fatalf("beta not added: %v", err)
	}
	// This device had no settings file, so the backup's settings apply.
	if got := dst.GetSettings(); got.Theme != "sepia" || got.Speed != "fast" {
		t.Fatalf("settings = %+v", got)
	}
}

func TestQueueDedupAndRemove(t *testing.T) {
	svc := newTestService(t)
	// Hold the worker so jobs stay queued.
	svc.mu.Lock()
	svc.running = true
	svc.cur = &job{ID: 99, Kind: "update", Slug: "busy"}
	svc.curState = &Current{}
	svc.mu.Unlock()

	r1, _ := svc.enqueue(&job{Kind: "download", URL: "https://www.example.com/n/"})
	r2, _ := svc.enqueue(&job{Kind: "download", URL: "http://example.com/n"})
	if r1["status"] != "queued" || r2["id"] != r1["id"] {
		t.Fatalf("duplicate link should map to the same queued job: %v %v", r1, r2)
	}
	r3, _ := svc.enqueue(&job{Kind: "update", Slug: "busy"})
	if r3["status"] != "running" {
		t.Fatalf("updating the running novel again should report running, got %v", r3)
	}
	if err := svc.RemoveQueued(r1["id"].(int)); err != nil {
		t.Fatal(err)
	}
	if a := svc.GetActivity(); len(a.Queue) != 0 || a.Current == nil {
		t.Fatalf("activity = %+v", a)
	}
}
