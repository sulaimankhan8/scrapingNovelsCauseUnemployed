package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"novelarchiver/internal/library"
)

// fakeTr "translates" by prefixing EN; fail lists texts it refuses.
type fakeTr struct {
	calls  int
	paused time.Duration
	fail   string
}

func (f *fakeTr) Translate(_ context.Context, texts []string, _ string) ([]string, error) {
	f.calls++
	out := make([]string, len(texts))
	for i, t := range texts {
		if f.fail != "" && strings.Contains(t, f.fail) {
			return nil, errors.New("Google Translate returned no text")
		}
		out[i] = "EN " + t
	}
	return out, nil
}

func (f *fakeTr) PausedFor() time.Duration { return f.paused }

func trickleService(t *testing.T, chapters int) (*Service, *fakeTr) {
	t.Helper()
	a := New(filepath.Join(t.TempDir(), "lib"))
	if a.initErr != nil {
		t.Fatal(a.initErr)
	}
	a.stopTrickle() // drive it by hand
	tr := &fakeTr{}
	a.arch.Translator = tr
	a.tr.gap = 0
	b := &library.Book{Slug: "zh-1", Title: "斗破", OriginalTitle: "斗破", Language: "zh"}
	for i := 1; i <= chapters; i++ {
		b.Chapters = append(b.Chapters, library.Chapter{Number: i, Title: fmt.Sprintf("第%d章", i), TitleOriginal: "x", State: library.StateOK})
		if err := a.store.WriteChapter(b.Slug, i, fmt.Sprintf("<h1>第%d章</h1><p>你好，世界 %d。</p>", i, i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.store.Save(b); err != nil {
		t.Fatal(err)
	}
	a.wakeTrickle()
	return a, tr
}

func untranslatedNums(t *testing.T, a *Service) []int {
	b, err := a.store.Load("zh-1")
	if err != nil {
		t.Fatal(err)
	}
	return a.arch.Pending(b)
}

func TestTrickleOrderFrontAndDailyCap(t *testing.T) {
	a, _ := trickleService(t, 6)
	ctx := context.Background()
	a.tr.daily = 4

	a.trickleStep(ctx) // the title first
	if b, _ := a.store.Load("zh-1"); b.Title != "EN 斗破" {
		t.Fatalf("title = %q, want it translated first", b.Title)
	}
	a.translateFirst("zh-1", 5) // the reader opened chapter 4; 5 goes next
	a.trickleStep(ctx)
	if got := untranslatedNums(t, a); fmt.Sprint(got) != "[1 2 3 4 6]" {
		t.Fatalf("after the front of the line: pending %v, want chapter 5 done", got)
	}
	a.trickleStep(ctx)
	a.trickleStep(ctx)
	if got := untranslatedNums(t, a); fmt.Sprint(got) != "[3 4 6]" {
		t.Fatalf("pending %v, want 1 and 2 done next", got)
	}
	// Four used today: it stops until tomorrow.
	if wait := a.trickleStep(ctx); wait < time.Minute {
		t.Errorf("over the daily allowance it waited %v", wait)
	}
	if st := a.translateStatus("zh-1"); st.State != "daily" {
		t.Errorf("state %q, want daily", st.State)
	}
	if got := untranslatedNums(t, a); len(got) != 3 {
		t.Errorf("translated past the allowance: pending %v", got)
	}
}

func TestTrickleWaitsWhileGooglePauses(t *testing.T) {
	a, tr := trickleService(t, 2)
	tr.paused = 4 * time.Minute
	if wait := a.trickleStep(context.Background()); wait < 4*time.Minute {
		t.Errorf("waited %v during a 4 min pause", wait)
	}
	if tr.calls != 0 {
		t.Errorf("asked Google %d times during a pause", tr.calls)
	}
	if st := a.translateStatus("zh-1"); st.State != "paused" || st.Resume == 0 {
		t.Errorf("status %+v, want paused with a resume time", st)
	}
}

func TestTrickleSkipsAChapterThatKeepsFailing(t *testing.T) {
	a, tr := trickleService(t, 3)
	tr.fail = "世界 1"
	ctx := context.Background()
	for i := 0; i < 6; i++ {
		a.trickleStep(ctx)
	}
	if got := untranslatedNums(t, a); fmt.Sprint(got) != "[1]" {
		t.Fatalf("pending %v, want only the failing chapter 1 left", got)
	}
	calls := tr.calls
	a.trickleStep(ctx)
	if tr.calls != calls {
		t.Errorf("retried the failing chapter straight away")
	}
}
