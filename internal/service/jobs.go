package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"novelarchiver/internal/archiver"
	"novelarchiver/internal/library"
	"novelarchiver/internal/source"
)

// Jobs run one at a time from a queue, so adding a novel while another
// downloads waits its turn instead of failing with "busy".

type job struct {
	ID    int
	Kind  string // "download" or "update"
	Slug  string
	Title string
	URL   string
	Cover string
	Opts  ScrapeOptions
	Auto  bool // started by "update all when the app opens"
	Quick bool // retry: only chapters that aren't saved; no revision check
	// CatchUp: only chapters the site added since the last check (and any
	// that failed), no revision check. "Get new chapters" and the update
	// when the app opens.
	CatchUp bool
	Added   time.Time
	Status  string
}

// QueueItem is one job as the UI sees it.
type QueueItem struct {
	ID    int    `json:"id"`
	Kind  string `json:"kind"`
	Slug  string `json:"slug,omitempty"`
	Title string `json:"title"`
	URL   string `json:"url"`
	Cover string `json:"cover,omitempty"`
}

// Activity is the current job and the queue, so a page that reloads (after
// coming back from the reader) can restore the download card.
type Activity struct {
	Current  *Current    `json:"current"`
	Queue    []QueueItem `json:"queue"`
	Waiting  string      `json:"waiting,omitempty"` // e.g. "wifi"
	Settings Settings    `json:"settings"`
}

// Current is the running job and its latest progress.
type Current struct {
	QueueItem
	Phase     string             `json:"phase"` // finding, downloading, revising
	Message   string             `json:"message,omitempty"`
	Progress  *archiver.Progress `json:"progress,omitempty"`
	Challenge string             `json:"challenge,omitempty"` // host waiting for a human check
	StartedAt time.Time          `json:"startedAt"`
}

func (j *job) item() QueueItem {
	return QueueItem{ID: j.ID, Kind: j.Kind, Slug: j.Slug, Title: j.Title, URL: j.URL, Cover: j.Cover}
}

// GetActivity returns the running job and the queue.
func (a *Service) GetActivity() *Activity {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.activityLocked()
}

func (a *Service) activityLocked() *Activity {
	act := &Activity{Queue: []QueueItem{}, Waiting: a.waiting, Settings: a.settings}
	if a.cur != nil {
		c := *a.curState
		act.Current = &c
	}
	for _, j := range a.queue {
		act.Queue = append(act.Queue, j.item())
	}
	return act
}

func (a *Service) emitQueue() {
	a.mu.Lock()
	act := a.activityLocked()
	a.mu.Unlock()
	a.emitRaw("queue", act)
}

// enqueue adds a job and starts the worker if it is idle.
func (a *Service) enqueue(j *job) (map[string]any, error) {
	a.mu.Lock()
	key := func(x *job) string {
		if x.Slug != "" {
			return x.Slug
		}
		return normURL(x.URL)
	}
	k := key(j)
	if a.cur != nil && key(a.cur) == k {
		a.mu.Unlock()
		return map[string]any{"status": "running", "id": a.cur.ID}, nil
	}
	for _, q := range a.queue {
		if key(q) == k {
			a.mu.Unlock()
			return map[string]any{"status": "queued", "id": q.ID}, nil
		}
	}
	a.nextID++
	j.ID, j.Added = a.nextID, time.Now()
	a.queue = append(a.queue, j)
	status := "queued"
	if a.cur == nil && len(a.queue) == 1 {
		status = "started"
	}
	start := !a.running
	if start {
		a.running = true
	}
	a.mu.Unlock()
	a.emitQueue()
	if start {
		go a.worker()
	}
	return map[string]any{"status": status, "id": j.ID}, nil
}

func normURL(x string) string {
	x = strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(x), "https://"), "http://")
	return strings.TrimRight(strings.TrimPrefix(x, "www."), "/")
}

func (a *Service) worker() {
	for {
		a.mu.Lock()
		if len(a.queue) == 0 || a.stopping {
			a.running, a.cur, a.curState, a.cancel = false, nil, nil, nil
			a.mu.Unlock()
			a.emitQueue()
			return
		}
		j := a.queue[0]
		a.queue = a.queue[1:]
		ctx, cancel := context.WithCancel(context.Background())
		a.cur, a.cancel = j, cancel
		phase := "finding"
		a.curState = &Current{QueueItem: j.item(), Phase: phase, StartedAt: time.Now()}
		a.mu.Unlock()
		a.emitQueue()
		a.runJob(ctx, j)
		cancel()
		a.mu.Lock()
		a.cur, a.curState, a.cancel = nil, nil, nil
		a.mu.Unlock()
	}
}

func (a *Service) runJob(ctx context.Context, j *job) {
	defer func() {
		if r := recover(); r != nil {
			a.emit("error", map[string]any{"message": fmt.Sprintf("internal error: %v", r), "slug": j.Slug})
		}
	}()
	switch j.Kind {
	case "update":
		a.runUpdate(ctx, j)
	default:
		a.emit("status", map[string]any{"message": "Reading the novel's page…"})
		n, err := a.discover(ctx, j.URL)
		if err != nil {
			a.fail(ctx, err, j)
			return
		}
		a.runDownload(ctx, j, n, j.Opts, false)
	}
}

// before is what an update compares against to build its summary.
type before struct {
	title, author, description, status string
	keys                               map[string]string // chapter key -> state
	ok                                 int
}

func snapshot(b *library.Book) before {
	s := before{keys: map[string]string{}}
	if b == nil {
		return s
	}
	s.title, s.author, s.description, s.status = b.Title, b.Author, b.Description, b.Status
	for _, c := range b.Chapters {
		s.keys[chapterKey(c)] = c.State
		if c.State == library.StateOK {
			s.ok++
		}
	}
	return s
}

func chapterKey(c library.Chapter) string {
	if c.URL != "" {
		return c.URL
	}
	return fmt.Sprintf("#%d", c.Index)
}

func (a *Service) runUpdate(ctx context.Context, j *job) {
	old, err := a.store.Load(j.Slug)
	if err != nil {
		a.fail(ctx, err, j)
		return
	}
	a.emit("status", map[string]any{"message": "Checking " + old.SiteName + " for changes…"})
	input := old.URL
	if input == "" {
		input = old.FirstChapterURL
	}
	a.mu.Lock()
	delete(a.lastDisc, strings.TrimSpace(input))
	a.mu.Unlock()
	n, err := a.discover(ctx, input)
	if err != nil {
		a.fail(ctx, err, j)
		return
	}
	a.runDownload(ctx, j, n, ScrapeOptions{Concurrency: a.GetSettings().speed()}, true)
}

// runDownload merges the discovery into the stored book, downloads, and
// (for updates) checks saved chapters for revisions, then reports a summary.
func (a *Service) runDownload(ctx context.Context, j *job, n *source.Novel, opts ScrapeOptions, update bool) {
	for _, w := range n.Warnings {
		a.emit("status", map[string]any{"message": w})
	}
	var prev *library.Book
	if slug := a.store.FindByURL(n.URL); slug != "" {
		prev, _ = a.store.Load(slug)
	}
	snap := snapshot(prev)
	b, err := a.arch.BookFromNovel(n)
	if err != nil {
		a.fail(ctx, err, j)
		return
	}
	a.mu.Lock()
	if a.cur == j {
		j.Slug, j.Title = b.Slug, b.Title
		a.curState.Slug, a.curState.Title = b.Slug, b.Title
	}
	a.mu.Unlock()

	from := opts.StartChapter
	if opts.FromPasted && n.InputChapter > 0 && from == 0 {
		from = n.InputChapter
	}
	res := &archiver.Result{}
	ao := archiver.Options{
		From: from, To: opts.EndChapter, Concurrency: opts.Concurrency, Force: opts.Force,
		SkipImages: opts.SkipImages, NoDone: true, Result: res, Gate: a.gate,
	}
	// Re-reading every saved chapter for edits is the slow part of an
	// update. Only a full Update you start does it; a retry of missing
	// chapters and the automatic update when the app opens don't.
	if update && !j.Quick && !j.Auto && !j.CatchUp {
		ao.Revise = a.GetSettings().reviseCount()
	}
	ao.OnlyFailed = j.Quick
	if j.CatchUp && prev != nil {
		ao.Only = func(c library.Chapter) bool {
			st, had := snap.keys[chapterKey(c)]
			return !had || st == library.StateFailed
		}
	}
	started := time.Now()
	if err := a.arch.Run(ctx, b, n, ao, a.emit); err != nil {
		a.fail(ctx, err, j)
		return
	}

	// Summary: what changed compared with before this job.
	var newNums []int
	fixed, added := 0, 0
	for _, c := range b.Chapters {
		st, had := snap.keys[chapterKey(c)]
		switch {
		case !had && c.State == library.StateOK:
			newNums = append(newNums, c.Number)
		case had && st == library.StateFailed && c.State == library.StateOK:
			fixed++ // failed last time
		case had && st != library.StateOK && c.State == library.StateOK:
			added++ // listed before but never saved (a range, or deleted to save space)
		}
	}
	sort.Ints(newNums)
	var details []string
	if prev != nil {
		if snap.title != b.Title {
			details = append(details, "title")
		}
		if snap.author != b.Author {
			details = append(details, "author")
		}
		if snap.description != b.Description {
			details = append(details, "description")
		}
		if snap.status != b.Status && b.Status != "" {
			details = append(details, "status")
		}
	}
	removed := 0
	for _, c := range b.Chapters {
		if c.Removed {
			removed++
		}
	}
	if update {
		b.CheckedAt = time.Now()
	}
	if update && len(newNums) > 0 {
		if b.Vars == nil {
			b.Vars = map[string]string{}
		}
		b.Vars["lastNew"] = fmt.Sprint(len(newNums))
		b.Vars["lastNewAt"] = time.Now().Format(time.RFC3339)
	}
	_ = a.store.Save(b)

	ok, failed, locked, _ := b.Counts()
	a.emit("done", map[string]any{
		"kind": j.Kind, "slug": b.Slug, "title": b.Title, "catalogUrl": "/library/" + b.Slug + "/",
		"ok": ok, "failed": failed, "locked": locked, "total": len(b.Chapters),
		"newChapters": newNums, "fixed": fixed, "added": added, "revised": res.Revised, "checked": res.Checked,
		"coverChanged": res.CoverChanged && prev != nil, "details": details, "status": b.Status,
		"removed": removed, "seconds": int(time.Since(started).Seconds()), "auto": j.Auto, "catchUp": j.CatchUp,
		"sizeBytes": a.store.Size(b.Slug),
	})
}

// gate pauses a job while "only on Wi-Fi" is on and the phone is on mobile data.
func (a *Service) gate(ctx context.Context) error {
	if !a.GetSettings().WifiOnly || a.NetworkOK == nil || a.NetworkOK() {
		return ctx.Err()
	}
	a.mu.Lock()
	a.waiting = "wifi"
	a.mu.Unlock()
	a.emit("status", map[string]any{"message": "Waiting for Wi-Fi. The download continues when you're connected."})
	a.emitQueue()
	defer func() {
		a.mu.Lock()
		a.waiting = ""
		a.mu.Unlock()
		a.emitQueue()
	}()
	t := time.NewTicker(3 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			if a.NetworkOK() || !a.GetSettings().WifiOnly {
				return nil
			}
		}
	}
}

// RemoveQueued drops a job that hasn't started.
func (a *Service) RemoveQueued(id int) error {
	a.mu.Lock()
	for i, j := range a.queue {
		if j.ID == id {
			a.queue = append(a.queue[:i], a.queue[i+1:]...)
			a.mu.Unlock()
			a.emitQueue()
			return nil
		}
	}
	a.mu.Unlock()
	return errors.New("that download isn't in the queue any more")
}

// CancelAll empties the queue and stops the running job.
func (a *Service) CancelAll() error {
	a.mu.Lock()
	a.queue = nil
	cancel := a.cancel
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	a.emitQueue()
	return nil
}

// UpdateAll queues an update for every novel. auto=true (app start) skips
// novels checked in the last 12 hours and completed novels.
func (a *Service) UpdateAll(auto bool) (map[string]any, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	books, err := a.store.List()
	if err != nil {
		return nil, err
	}
	n := 0
	for _, b := range books {
		if auto {
			if time.Since(b.CheckedAt) < 12*time.Hour || strings.EqualFold(b.Status, "completed") {
				continue
			}
			// An automatic update that failed or was interrupted (the
			// phone closed the app, say) waits 12 hours like a finished
			// one, instead of starting over on every launch.
			if t, err := time.Parse(time.RFC3339, b.Vars["autoTriedAt"]); err == nil && time.Since(t) < 12*time.Hour {
				continue
			}
			if b.Vars == nil {
				b.Vars = map[string]string{}
			}
			b.Vars["autoTriedAt"] = time.Now().Format(time.RFC3339)
			_ = a.store.Save(b)
		}
		if _, err := a.enqueue(&job{Kind: "update", Slug: b.Slug, Title: b.Title, URL: b.URL, Cover: a.coverPath(b), Auto: auto, CatchUp: auto}); err == nil {
			n++
		}
	}
	return map[string]any{"queued": n}, nil
}

// CatchUp queues "get new chapters" for every novel that is still coming
// out: the chapter list is checked and only chapters added since last time
// (plus any that failed) are downloaded. Completed novels are skipped, and
// saved chapters aren't re-read.
func (a *Service) CatchUp() (map[string]any, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	books, err := a.store.List()
	if err != nil {
		return nil, err
	}
	n, skipped := 0, 0
	for _, b := range books {
		if strings.Contains(strings.ToLower(b.Status), "complet") || (b.URL == "" && b.FirstChapterURL == "") {
			skipped++
			continue
		}
		if _, err := a.enqueue(&job{Kind: "update", Slug: b.Slug, Title: b.Title, URL: b.URL, Cover: a.coverPath(b), CatchUp: true}); err == nil {
			n++
		}
	}
	return map[string]any{"queued": n, "skipped": skipped}, nil
}

func (a *Service) coverPath(b *library.Book) string {
	if b.CoverFile != "" {
		return "/library/" + b.Slug + "/cover"
	}
	return ""
}
