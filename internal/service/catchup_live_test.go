package service

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"novelarchiver/internal/library"
)

// TestCatchUpLive checks "Get new chapters" against a real site: a novel
// saved when the site had 120 chapters (1-3 downloaded, the rest never
// chosen) must get exactly the chapters added since, and nothing else.
// Needs the network, so it runs only with NA_LIVE=1.
func TestCatchUpLive(t *testing.T) {
	if os.Getenv("NA_LIVE") == "" {
		t.Skip("set NA_LIVE=1 to run against novelping.com")
	}
	svc := newTestService(t)
	var mu sync.Mutex
	var done map[string]any
	svc.Emit = func(ev string, data any) {
		if ev == "done" {
			mu.Lock()
			done, _ = data.(map[string]any)
			mu.Unlock()
		}
	}
	ctx := context.Background()
	n, err := svc.engine.Discover(ctx, "https://novelping.com/book/a-borrowed-soul", nil)
	if err != nil {
		t.Fatal(err)
	}
	total := len(n.Chapters)
	if total < 121 {
		t.Fatalf("site lists %d chapters; the test needs more than 120", total)
	}
	b, err := svc.arch.BookFromNovel(n)
	if err != nil {
		t.Fatal(err)
	}
	b.Chapters = b.Chapters[:120]
	for i := 0; i < 3; i++ {
		b.Chapters[i].State = library.StateOK
		if err := svc.store.WriteChapter(b.Slug, b.Chapters[i].Number, "<p>saved earlier</p>"); err != nil {
			t.Fatal(err)
		}
	}
	b.Status = "Ongoing"
	if err := svc.store.Save(b); err != nil {
		t.Fatal(err)
	}

	r, err := svc.CatchUp()
	if err != nil || r["queued"] != 1 {
		t.Fatalf("CatchUp = %v, %v", r, err)
	}
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		a := svc.GetActivity()
		mu.Lock()
		finished := done != nil
		mu.Unlock()
		if finished && a.Current == nil && len(a.Queue) == 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	got, err := svc.store.Load(b.Slug)
	if err != nil {
		t.Fatal(err)
	}
	ok := 0
	for _, c := range got.Chapters {
		if c.State == library.StateOK {
			ok++
			if c.Number > 3 && c.Number <= 120 {
				t.Errorf("chapter %d was downloaded, but it was never chosen", c.Number)
			}
		}
	}
	want := 3 + (total - 120)
	if ok != want {
		t.Errorf("%d chapters saved, want %d (3 earlier + %d new)", ok, want, total-120)
	}
	mu.Lock()
	defer mu.Unlock()
	nw, _ := done["newChapters"].([]int)
	if len(nw) != total-120 || done["catchUp"] != true || len(done["revised"].([]int)) != 0 {
		t.Errorf("summary: new=%v catchUp=%v revised=%v", done["newChapters"], done["catchUp"], done["revised"])
	}
	t.Logf("site has %d chapters; caught up %v; saved %d", total, nw, ok)
}
