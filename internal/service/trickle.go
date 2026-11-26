package service

// The background translator. Downloads translate only their first few
// chapters (archiver.InlineTranslate); everything else is translated here,
// one chapter at a time with a pause in between and a daily allowance, so
// the app stays well inside what Google's free translator accepts. The
// chapter being read, and the two after it, jump the line.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"novelarchiver/internal/library"
)

const (
	trickleGap   = 20 * time.Second // between chapters: three a minute
	trickleDaily = 300              // chapters a day
	trickleIdle  = 15 * time.Minute // look for new work this often anyway
	trickleRetry = 6 * time.Hour    // before trying failed work again
)

// trickleItem is one piece of work: a chapter, the book's titles (num 0)
// or everything a book still needs (num -1).
type trickleItem struct {
	slug string
	num  int
}

type trickler struct {
	mu    sync.Mutex
	wake  chan struct{}
	front []trickleItem // asked for first: the chapter being read, Translate now
	scan  []trickleItem // everything else, rebuilt when dirty
	dirty bool
	last  time.Time // when the last chapter was translated
	day   string    // the allowance's day ("2026-11-26") and how much is used
	count int
	gap   time.Duration
	daily int
	// tried remembers work that failed (or made no progress) recently, so
	// one chapter Google can't handle isn't asked for every 20 seconds.
	tried  map[trickleItem]time.Time
	state  string    // "", "paused" (Google asked for a break), "daily" (allowance used)
	resume time.Time // when a pause or the allowance ends
	cancel context.CancelFunc
}

// TranslateStatus is what the novel page says about the background
// translator.
type TranslateStatus struct {
	State  string `json:"state"`            // "queued", "paused", "daily", "busy" (being downloaded)
	Resume int64  `json:"resume,omitempty"` // unix ms when a pause or the allowance ends
	PerMin int    `json:"perMin"`
	Daily  int    `json:"daily"`
}

func (a *Service) startTrickle() {
	ctx, cancel := context.WithCancel(context.Background())
	a.tr = &trickler{wake: make(chan struct{}, 1), dirty: true, gap: trickleGap, daily: trickleDaily, cancel: cancel, tried: map[trickleItem]time.Time{}}
	a.loadBudget()
	go a.trickleLoop(ctx)
}

func (a *Service) stopTrickle() {
	if a.tr != nil {
		a.tr.cancel()
	}
}

// wakeTrickle tells the translator there may be new work (a download
// finished, a chapter was read).
func (a *Service) wakeTrickle() {
	t := a.tr
	if t == nil {
		return
	}
	t.mu.Lock()
	t.dirty = true
	t.mu.Unlock()
	select {
	case t.wake <- struct{}{}:
	default:
	}
}

// translateFirst puts chapters of slug at the front of the line; with no
// numbers, everything the book still needs.
func (a *Service) translateFirst(slug string, nums ...int) {
	t := a.tr
	if t == nil {
		return
	}
	items := []trickleItem{}
	if len(nums) == 0 {
		items = append(items, trickleItem{slug, -1})
	}
	for _, n := range nums {
		items = append(items, trickleItem{slug, n})
	}
	t.mu.Lock()
	keep := t.front[:0:0]
	for _, it := range t.front {
		dup := false
		for _, n := range items {
			dup = dup || it == n
		}
		if !dup {
			keep = append(keep, it)
		}
	}
	t.front = append(items, keep...)
	t.mu.Unlock()
	a.wakeTrickle()
}

func (a *Service) trickleLoop(ctx context.Context) {
	for {
		wait := a.trickleStep(ctx)
		if wait <= 0 {
			wait = trickleIdle
		}
		select {
		case <-ctx.Done():
			return
		case <-a.tr.wake:
		case <-time.After(wait):
		}
	}
}

// trickleStep translates at most one thing and says how long to wait
// before the next (0: nothing to do).
func (a *Service) trickleStep(ctx context.Context) time.Duration {
	t := a.tr
	if a.arch == nil || ctx.Err() != nil {
		return 0
	}
	if d := translatorPause(a); d > 0 {
		t.setState("paused", time.Now().Add(d))
		return d + 5*time.Second
	}
	t.mu.Lock()
	if since := time.Since(t.last); since < t.gap {
		t.mu.Unlock()
		return t.gap - since
	}
	now := time.Now()
	if today := now.Format("2006-01-02"); t.day != today {
		t.day, t.count = today, 0
	}
	full := t.count >= t.daily
	t.mu.Unlock()
	if full {
		y, m, d := now.Date()
		midnight := time.Date(y, m, d+1, 0, 1, 0, 0, now.Location())
		t.setState("daily", midnight)
		return time.Until(midnight)
	}
	it, b, ok := a.nextTrickle()
	if !ok {
		t.setState("", time.Time{})
		return 0
	}
	t.setState("", time.Time{})
	var err error
	if it.num == 0 {
		before := a.arch.MetaLeft(b)
		err = a.arch.TranslateMeta(ctx, b)
		if err == nil && a.arch.MetaLeft(b) >= before {
			t.markTried(it) // Google left the titles as they were
		}
	} else {
		err = a.arch.TranslateChapter(ctx, b, it.num)
	}
	if err != nil && translatorPause(a) == 0 {
		t.markTried(it)
	}
	a.spend()
	a.emitRaw("translated", map[string]any{"slug": it.slug, "number": it.num, "ok": err == nil})
	if d := translatorPause(a); d > 0 {
		t.setState("paused", time.Now().Add(d))
		return d + 5*time.Second
	}
	return t.gap
}

// spend counts one translation against today's allowance.
func (a *Service) spend() {
	t := a.tr
	if t == nil {
		return
	}
	t.mu.Lock()
	t.last = time.Now()
	if today := t.last.Format("2006-01-02"); t.day != today {
		t.day, t.count = today, 0
	}
	t.count++
	t.mu.Unlock()
	a.saveBudget()
}

func (t *trickler) markTried(it trickleItem) {
	t.mu.Lock()
	t.tried[it] = time.Now()
	t.mu.Unlock()
}

// fresh reports whether it hasn't failed recently.
func (t *trickler) fresh(it trickleItem) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	at, ok := t.tried[it]
	return !ok || time.Since(at) > trickleRetry
}

func (t *trickler) setState(s string, resume time.Time) {
	t.mu.Lock()
	t.state, t.resume = s, resume
	t.mu.Unlock()
}

// nextTrickle picks what to translate next: the front of the line first,
// then every other foreign novel. Novels a download is working on are
// skipped (the download saves its own copy of the book).
func (a *Service) nextTrickle() (trickleItem, *library.Book, bool) {
	t := a.tr
	rebuilt := false
	for {
		t.mu.Lock()
		var it trickleItem
		fromFront := len(t.front) > 0
		if fromFront {
			it = t.front[0]
		} else {
			if (t.dirty || len(t.scan) == 0) && !rebuilt {
				rebuilt = true // once per call: work that's all cooling down ends the search
				t.mu.Unlock()
				scan := a.buildScan()
				t.mu.Lock()
				t.scan, t.dirty = scan, false
			}
			if len(t.scan) == 0 {
				t.mu.Unlock()
				return trickleItem{}, nil, false
			}
			it = t.scan[0]
		}
		t.mu.Unlock()

		b, err := a.store.Load(it.slug)
		pick, done := trickleItem{}, true
		if err == nil && b.Translating() && !a.busyWith(it.slug) {
			meta := trickleItem{it.slug, 0}
			switch {
			case it.num == -1 && a.arch.MetaLeft(b) > 0 && t.fresh(meta):
				pick, done = meta, false
			case it.num == -1:
				var left []int
				for _, n := range a.arch.Pending(b) {
					if t.fresh(trickleItem{it.slug, n}) {
						left = append(left, n)
					}
				}
				if len(left) > 0 {
					pick, done = trickleItem{it.slug, left[0]}, len(left) == 1
				}
			case it.num == 0 && a.arch.MetaLeft(b) > 0 && t.fresh(it):
				pick = it
			case it.num > 0 && a.arch.NeedsTranslation(b, it.num) && t.fresh(it):
				pick = it
			}
		}
		if done {
			t.mu.Lock()
			if fromFront {
				if len(t.front) > 0 && t.front[0] == it {
					t.front = t.front[1:]
				}
			} else if len(t.scan) > 0 && t.scan[0] == it {
				t.scan = t.scan[1:]
			}
			t.mu.Unlock()
		}
		if pick.slug != "" {
			return pick, b, true
		}
	}
}

// buildScan lists the work left in every foreign novel, newest first.
func (a *Service) buildScan() []trickleItem {
	books, err := a.store.List()
	if err != nil {
		return nil
	}
	var out []trickleItem
	for _, b := range books {
		if !b.Translating() || a.busyWith(b.Slug) {
			continue
		}
		if it := (trickleItem{b.Slug, 0}); a.arch.MetaLeft(b) > 0 && a.tr.fresh(it) {
			out = append(out, it)
		}
		for _, n := range a.arch.Pending(b) {
			if it := (trickleItem{b.Slug, n}); a.tr.fresh(it) {
				out = append(out, it)
			}
		}
	}
	return out
}

// busyWith reports whether the running job is working on slug.
func (a *Service) busyWith(slug string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cur != nil && a.cur.Slug == slug
}

// translateStatus describes the background translator for one novel.
func (a *Service) translateStatus(slug string) *TranslateStatus {
	t := a.tr
	if t == nil {
		return nil
	}
	t.mu.Lock()
	st := &TranslateStatus{State: "queued", PerMin: int(time.Minute / max(t.gap, time.Second)), Daily: t.daily}
	state, resume := t.state, t.resume
	t.mu.Unlock()
	if d := translatorPause(a); d > 0 {
		state, resume = "paused", time.Now().Add(d)
	}
	if state != "" {
		st.State, st.Resume = state, resume.UnixMilli()
	}
	if a.busyWith(slug) {
		st.State, st.Resume = "busy", 0
	}
	return st
}

func translatorPause(a *Service) time.Duration {
	if p, ok := a.arch.Translator.(interface{ PausedFor() time.Duration }); ok {
		return p.PausedFor()
	}
	return 0
}

// The allowance survives restarts (phones close apps all the time).
type budget struct {
	Day   string `json:"day"`
	Count int    `json:"count"`
}

func (a *Service) budgetPath() string {
	if a.store == nil {
		return ""
	}
	return filepath.Join(filepath.Dir(a.store.Root), "translate-budget.json")
}

func (a *Service) loadBudget() {
	p := a.budgetPath()
	if p == "" {
		return
	}
	var bu budget
	if data, err := os.ReadFile(p); err == nil && json.Unmarshal(data, &bu) == nil {
		a.tr.day, a.tr.count = bu.Day, bu.Count
	}
}

func (a *Service) saveBudget() {
	p := a.budgetPath()
	if p == "" {
		return
	}
	a.tr.mu.Lock()
	data, _ := json.Marshal(budget{a.tr.day, a.tr.count})
	a.tr.mu.Unlock()
	if os.WriteFile(p+".tmp", data, 0o644) == nil {
		_ = os.Rename(p+".tmp", p)
	}
}
