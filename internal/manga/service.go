package manga

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"novelarchiver/internal/browser"
	"novelarchiver/internal/fetch"
)

// ChallengeMessage is shown when a site asks for a human check.
const ChallengeMessage = "This site wants you to confirm you're human. Tick the box in the site window that just opened; downloading continues by itself."

// SiteWindow is the site browser a host app can share.
type SiteWindow = siteBrowser

type siteBrowser interface {
	fetch.Fetcher
	Show(string) error
	Close()
}

// ServiceOptions configure a Service.
type ServiceOptions struct {
	NoBrowser bool
	// Browser, when set, is a site window shared with the rest of the app
	// (one Cloudflare session for novels and manga). The service never
	// closes a shared window.
	Browser  SiteWindow
	OnStatus func(string)
	// Emit receives events for the UI: "progress", "library", "done",
	// "status", "challenge".
	Emit func(event string, data any)
	// MayDownload, when set, is asked before each chapter; false pauses the
	// job until it turns true (Android: "Only on Wi-Fi" on mobile data).
	MayDownload func() bool
}

// Settings are the user's preferences (settings.json beside the library).
type Settings struct {
	ChapterWorkers  int    `json:"chapterWorkers"`  // chapters downloaded at once
	PageWorkers     int    `json:"pageWorkers"`     // pages per chapter at once
	AutoDownloadNew bool   `json:"autoDownloadNew"` // updates also download new chapters
	CheckOnOpen     bool   `json:"checkOnOpen"`     // check for new chapters at start
	ReadMode        string `json:"readMode"`        // auto, strip, rtl, ltr
	Notify          bool   `json:"notify"`
}

func defaultSettings() Settings {
	return Settings{ChapterWorkers: 2, PageWorkers: 4, AutoDownloadNew: true, ReadMode: "strip", Notify: true}
}

// Service is the whole app minus the window: library, queue, engine.
type Service struct {
	store *Store
	eng   *Engine
	win   siteBrowser
	opts  ServiceOptions

	mu         sync.Mutex
	settings   Settings
	queue      []*Job
	running    []*Job                     // jobs downloading now, oldest first
	cancels    map[int]context.CancelFunc // by job id
	siteIDs    map[string]string          // series slug -> site id (for canStart)
	nextID     int
	wake       chan struct{}
	stopped    chan struct{}
	ctx        context.Context
	stop       context.CancelFunc
	discovered map[string]cached
	sharedWin  bool
	bookMu     sync.Mutex // guards load-modify-save of series.json
}

type cached struct {
	s  *Series
	at time.Time
}

// Job is one queued task.
type Job struct {
	ID      int       `json:"id"`
	Kind    string    `json:"kind"` // download, update
	Slug    string    `json:"slug"`
	Title   string    `json:"title"`
	From    int       `json:"from,omitempty"`
	To      int       `json:"to,omitempty"`
	Keys    []int     `json:"keys,omitempty"` // only these chapters (new ones from an update)
	Auto    bool      `json:"auto,omitempty"`
	Message string    `json:"message"`
	Done    int       `json:"done"`
	Total   int       `json:"total"`
	Failed  int       `json:"failed"`
	Started time.Time `json:"started,omitzero"`
}

// Activity is the queue as the UI shows it.
type Activity struct {
	// Current is the oldest running job (nil when idle). Running lists
	// every job downloading now: different series run side by side, at
	// most one per site.
	Current *Job  `json:"current"`
	Running []Job `json:"running"`
	Queue   []Job `json:"queue"`
}

// maxParallel is how many series download at the same time.
const maxParallel = 3

// NewService opens the library and starts the download worker.
func NewService(root string, o ServiceOptions) (*Service, error) {
	st, err := OpenStore(root)
	if err != nil {
		return nil, err
	}
	sites, err := LoadSites(filepath.Join(filepath.Dir(st.Root), "sites"))
	if err != nil {
		return nil, err
	}
	s := &Service{
		store: st, opts: o, settings: defaultSettings(),
		eng: &Engine{
			Sites:  sites,
			HTTP:   NewClient(2, 450*time.Millisecond),
			Images: NewClient(6, 120*time.Millisecond),
		},
		wake: make(chan struct{}, 1), stopped: make(chan struct{}),
		discovered: map[string]cached{},
	}
	s.loadSettings()
	if o.Browser != nil {
		s.win, s.sharedWin = o.Browser, true
		s.eng.Browser = o.Browser
	} else if !o.NoBrowser {
		if win, err := browser.New(browser.Options{
			DataPath: filepath.Join(filepath.Dir(st.Root), "SiteBrowser"),
			Title:    "InkShelfs — site browser (leave open while downloading)",
			OnStatus: func(m string) { s.status(m) },
			OnChallenge: func(host string) {
				s.emit("challenge", map[string]any{"host": host, "message": ChallengeMessage})
			},
		}); err == nil && win != nil {
			s.win = win
			s.eng.Browser = win
		}
	}
	s.ctx, s.stop = context.WithCancel(context.Background())
	go s.worker()
	return s, nil
}

// Engine exposes the engine (for the command-line tool).
func (s *Service) Engine() *Engine { return s.eng }

// Store exposes the library.
func (s *Service) Store() *Store { return s.store }

// LibraryRoot is the library folder.
func (s *Service) LibraryRoot() string { return s.store.Root }

// SeriesDir is a series folder.
func (s *Service) SeriesDir(slug string) string { return s.store.Dir(slug) }

func (s *Service) emit(ev string, data any) {
	if s.opts.Emit != nil {
		s.opts.Emit(ev, data)
	}
}

func (s *Service) status(m string) {
	if s.opts.OnStatus != nil {
		s.opts.OnStatus(m)
	}
	s.emit("status", map[string]any{"message": m})
}

// Shutdown cancels work and closes the site browser.
func (s *Service) Shutdown() {
	s.CancelAll()
	s.stop()
	select {
	case <-s.stopped:
	case <-time.After(5 * time.Second):
	}
	if s.win != nil && !s.sharedWin {
		s.win.Close()
	}
}

// ---- settings ----

func (s *Service) settingsPath() string {
	return filepath.Join(filepath.Dir(s.store.Root), "settings.json")
}

func (s *Service) loadSettings() {
	if data, err := os.ReadFile(s.settingsPath()); err == nil {
		st := defaultSettings()
		if json.Unmarshal(data, &st) == nil {
			s.settings = normSettings(st)
		}
	}
}

func normSettings(st Settings) Settings {
	st.ChapterWorkers = max(1, min(st.ChapterWorkers, 4))
	st.PageWorkers = max(1, min(st.PageWorkers, 8))
	switch st.ReadMode {
	case "strip", "rtl", "ltr", "spread":
	default:
		// "auto" (pages for Japanese manga) is gone: scrolling is the default.
		st.ReadMode = "strip"
	}
	return st
}

// GetSettings returns the current settings.
func (s *Service) GetSettings() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings
}

// SaveSettings stores new settings.
func (s *Service) SaveSettings(st Settings) (Settings, error) {
	st = normSettings(st)
	data, _ := json.MarshalIndent(st, "", "  ")
	if err := os.WriteFile(s.settingsPath(), data, 0o644); err != nil {
		return s.GetSettings(), err
	}
	s.mu.Lock()
	s.settings = st
	s.mu.Unlock()
	return st, nil
}

// ---- sites ----

// SiteInfo describes a supported site for the UI.
type SiteInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Home    string `json:"home"`
	Status  string `json:"status"`
	Example string `json:"example"`
}

// Sites lists supported sites.
func (s *Service) Sites() []SiteInfo {
	var out []SiteInfo
	for _, p := range s.eng.Sites.Sites {
		out = append(out, SiteInfo{p.ID, p.Name, p.Type, p.Home, p.Status, p.Example})
	}
	return out
}

// OpenSiteBrowser shows the site window (to pass a check or browse).
func (s *Service) OpenSiteBrowser(u string) error {
	if s.win == nil {
		return errors.New("the site browser isn't available on this computer (Edge WebView2 is needed)")
	}
	if u != "" && !strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://") {
		u = "https://" + u
	}
	return s.win.Show(u)
}

// ---- discovery and adding ----

// Discover reads a link (cached for 30 minutes so "Download" doesn't
// fetch everything twice).
func (s *Service) Discover(link string) (*Series, error) {
	ctx, cancel := context.WithTimeout(s.ctx, 6*time.Minute)
	defer cancel()
	ser, err := s.eng.Discover(ctx, link, s.status)
	if err != nil {
		return nil, friendly(err)
	}
	s.mu.Lock()
	s.discovered[strings.TrimSpace(link)] = cached{ser, time.Now()}
	s.mu.Unlock()
	return ser, nil
}

// SeriesPreview is Discover plus whether the series is already saved.
type SeriesPreview struct {
	*Series
	ExistingSlug string `json:"existingSlug,omitempty"`
	Downloaded   int    `json:"downloaded"`
}

// Preview discovers a link for the add dialog.
func (s *Service) Preview(link string) (*SeriesPreview, error) {
	ser, err := s.Discover(link)
	if err != nil {
		return nil, err
	}
	pv := &SeriesPreview{Series: ser, ExistingSlug: s.store.FindByURL(ser.URL)}
	if pv.ExistingSlug != "" {
		if b, err := s.store.Load(pv.ExistingSlug); err == nil {
			pv.Downloaded, _, _, _ = b.Counts()
		}
	}
	return pv, nil
}

// CoverPreview returns a previewed series' cover as a data: URL. Many
// sites refuse cover images without their own Referer, so the window
// can't load them directly.
func (s *Service) CoverPreview(link string) string {
	s.mu.Lock()
	c, ok := s.discovered[strings.TrimSpace(link)]
	s.mu.Unlock()
	if !ok || c.s.Cover == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()
	data, ext, err := s.eng.FetchImage(ctx, s.eng.Sites.ByID(c.s.SiteID), c.s.Cover, c.s.URL)
	if err != nil || len(data) > 8<<20 {
		return ""
	}
	mime := map[string]string{".jpg": "image/jpeg", ".png": "image/png", ".webp": "image/webp", ".gif": "image/gif", ".avif": "image/avif"}[ext]
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// AddByURL adds (or extends) a series from a previewed link and queues
// chapters from..to (0 = all).
func (s *Service) AddByURL(link string, from, to int) (string, error) {
	s.mu.Lock()
	c, ok := s.discovered[strings.TrimSpace(link)]
	s.mu.Unlock()
	ser := c.s
	if !ok || time.Since(c.at) > 30*time.Minute {
		var err error
		if ser, err = s.Discover(link); err != nil {
			return "", err
		}
	}
	return s.AddSeries(ser, from, to)
}

// AddSeries stores a discovered series and queues a download.
func (s *Service) AddSeries(ser *Series, from, to int) (string, error) {
	s.bookMu.Lock()
	slug := s.store.FindByURL(ser.URL)
	var b *Book
	if slug != "" {
		var err error
		if b, err = s.store.Load(slug); err != nil {
			s.bookMu.Unlock()
			return "", err
		}
	} else {
		slug = s.store.NewSlug(ser.Title)
		b = &Book{Slug: slug, AddedAt: time.Now(), NextKey: 1}
	}
	b.SiteID, b.SiteName, b.Type, b.URL = ser.SiteID, ser.SiteName, ser.Type, ser.URL
	b.Title, b.Author, b.Description, b.Status = ser.Title, ser.Author, ser.Description, ser.Status
	if ser.Cover != "" {
		b.CoverURL = ser.Cover
	}
	b.Vars = ser.Vars
	mergeChapters(b, ser.Chapters)
	b.CheckedAt = time.Now()
	err := s.store.Save(b)
	s.bookMu.Unlock()
	if err != nil {
		return "", err
	}
	s.emit("library", map[string]any{"slug": slug})
	s.enqueue(&Job{Kind: "download", Slug: slug, Title: b.Title, From: from, To: to})
	return slug, nil
}

// mergeChapters folds a fresh chapter list into a book, keeping saved
// chapters (by link) and their folders. Returns the keys of new chapters.
func mergeChapters(b *Book, refs []ChapterRef) []int {
	byKey := map[string]*Chapter{}
	for i := range b.Chapters {
		c := &b.Chapters[i]
		byKey[chKey(c.URL, c.ID)] = c
	}
	if b.NextKey < 1 {
		b.NextKey = 1
		for _, c := range b.Chapters {
			b.NextKey = max(b.NextKey, c.Key+1)
		}
	}
	used := map[string]bool{}
	var out []Chapter
	var added []int
	for _, r := range refs {
		k := chKey(r.URL, r.ID)
		if used[k] {
			continue
		}
		used[k] = true
		if old := byKey[k]; old != nil {
			c := *old
			c.Title, c.URL, c.ID, c.Removed = r.Title, r.URL, r.ID, false
			out = append(out, c)
			continue
		}
		out = append(out, Chapter{Key: b.NextKey, Title: r.Title, URL: r.URL, ID: r.ID})
		added = append(added, b.NextKey)
		b.NextKey++
	}
	for _, c := range b.Chapters {
		if !used[chKey(c.URL, c.ID)] && c.State == StateOK {
			c.Removed = true
			out = append(out, c)
		}
	}
	for i := range out {
		out[i].Number = i + 1
	}
	b.Chapters = out
	return added
}

func chKey(u, id string) string {
	if id != "" {
		return "id:" + id
	}
	p, err := url.Parse(u)
	if err != nil {
		return u
	}
	return strings.TrimPrefix(p.Host, "www.") + strings.TrimRight(p.Path, "/") + "?" + p.RawQuery
}

// ---- queue ----

func (s *Service) enqueue(j *Job) {
	s.mu.Lock()
	// Merge with a queued job for the same series instead of stacking.
	for _, q := range s.queue {
		if q.Slug == j.Slug && q.Kind == j.Kind {
			if j.Kind == "download" {
				if len(q.Keys) > 0 && len(j.Keys) > 0 {
					q.Keys = append(q.Keys, j.Keys...)
				} else if q.From != j.From || q.To != j.To || len(q.Keys) != len(j.Keys) {
					q.From, q.To, q.Keys = 0, 0, nil
				}
			}
			s.mu.Unlock()
			s.emitQueue()
			return
		}
	}
	s.nextID++
	j.ID = s.nextID
	j.Message = "Waiting…"
	s.queue = append(s.queue, j)
	s.mu.Unlock()
	s.emitQueue()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Service) emitQueue() { s.emit("queue", s.Activity()) }

// Activity returns the running job and the queue.
func (s *Service) Activity() Activity {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := Activity{Queue: []Job{}, Running: []Job{}}
	for _, j := range s.running {
		a.Running = append(a.Running, *j)
	}
	if len(a.Running) > 0 {
		c := a.Running[0]
		a.Current = &c
	}
	for _, j := range s.queue {
		a.Queue = append(a.Queue, *j)
	}
	return a
}

// RemoveQueued drops a waiting job.
func (s *Service) RemoveQueued(id int) {
	s.mu.Lock()
	for i, j := range s.queue {
		if j.ID == id {
			s.queue = append(s.queue[:i], s.queue[i+1:]...)
			break
		}
	}
	s.mu.Unlock()
	s.emitQueue()
}

// CancelCurrent stops every running job (saved chapters stay).
func (s *Service) CancelCurrent() {
	s.mu.Lock()
	for _, c := range s.cancels {
		c()
	}
	s.mu.Unlock()
}

// CancelJob stops one running job, or drops it if it's still waiting.
func (s *Service) CancelJob(id int) {
	s.mu.Lock()
	if c, ok := s.cancels[id]; ok {
		c()
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	s.RemoveQueued(id)
}

// CancelAll empties the queue and stops the running job.
func (s *Service) CancelAll() {
	s.mu.Lock()
	s.queue = nil
	for _, c := range s.cancels {
		c()
	}
	s.mu.Unlock()
	s.emitQueue()
}

// worker starts queued jobs: up to maxParallel at once, never two for the
// same series or the same site (so each site sees one job's traffic, as
// before, and sites read through the one site window never collide).
func (s *Service) worker() {
	defer close(s.stopped)
	var wg sync.WaitGroup
	defer wg.Wait()
	for {
		s.mu.Lock()
		started := false
		if len(s.running) < maxParallel {
			for i, j := range s.queue {
				if !s.canStart(j) {
					continue
				}
				s.queue = append(s.queue[:i:i], s.queue[i+1:]...)
				ctx, cancel := context.WithCancel(s.ctx)
				if s.cancels == nil {
					s.cancels = map[int]context.CancelFunc{}
				}
				s.running = append(s.running, j)
				s.cancels[j.ID] = cancel
				j.Started = time.Now()
				started = true
				wg.Add(1)
				go func(j *Job) {
					defer wg.Done()
					s.run(ctx, j)
					cancel()
					s.mu.Lock()
					for k, r := range s.running {
						if r == j {
							s.running = append(s.running[:k:k], s.running[k+1:]...)
							break
						}
					}
					delete(s.cancels, j.ID)
					s.mu.Unlock()
					s.emitQueue()
					select {
					case s.wake <- struct{}{}:
					default:
					}
				}(j)
				break
			}
		}
		s.mu.Unlock()
		if started {
			s.emitQueue()
			continue
		}
		select {
		case <-s.wake:
		case <-s.ctx.Done():
			return
		}
	}
}

// canStart: j's series and site aren't busy. Call with s.mu held.
func (s *Service) canStart(j *Job) bool {
	site := s.siteOf(j.Slug)
	for _, r := range s.running {
		if r.Slug == j.Slug || (site != "" && s.siteOf(r.Slug) == site) {
			return false
		}
	}
	return true
}

// siteOf is the site id a series downloads from ("" if unknown).
func (s *Service) siteOf(slug string) string {
	if s.siteIDs == nil {
		s.siteIDs = map[string]string{}
	}
	if id, ok := s.siteIDs[slug]; ok {
		return id
	}
	b, err := s.store.Load(slug)
	if err != nil {
		return ""
	}
	s.siteIDs[slug] = b.SiteID
	return b.SiteID
}

func (s *Service) setJob(j *Job, f func(*Job)) {
	s.mu.Lock()
	f(j)
	snap := *j
	s.mu.Unlock()
	s.emit("progress", snap)
}

func (s *Service) run(ctx context.Context, j *Job) {
	var err error
	switch j.Kind {
	case "update":
		err = s.runUpdate(ctx, j)
	default:
		err = s.runDownload(ctx, j)
	}
	msg := ""
	switch {
	case errors.Is(err, context.Canceled):
		msg = "Stopped. Chapters already saved are kept."
	case err != nil:
		msg = friendly(err).Error()
	default:
		msg = j.Message
	}
	s.setJob(j, func(j *Job) { j.Message = msg })
	s.emit("done", map[string]any{"job": *j, "error": err != nil && !errors.Is(err, context.Canceled), "message": msg})
	s.emit("library", map[string]any{"slug": j.Slug})
}

func (s *Service) runDownload(ctx context.Context, j *Job) error {
	b, err := s.store.Load(j.Slug)
	if err != nil {
		return err
	}
	if b.CoverFile == "" && b.CoverURL != "" {
		s.setJob(j, func(j *Job) { j.Message = "Saving the cover…" })
		if name := s.eng.SaveCover(ctx, b, s.store.Dir(b.Slug)); name != "" {
			s.updateBook(b.Slug, func(nb *Book) { nb.CoverFile = name })
			s.emit("library", map[string]any{"slug": b.Slug})
		}
	}
	only := map[int]bool{}
	for _, k := range j.Keys {
		only[k] = true
	}
	var todo []Chapter
	for _, c := range b.Chapters {
		if (len(only) > 0 && !only[c.Key]) || c.State == StateOK || c.Removed || (j.From > 0 && c.Number < j.From) || (j.To > 0 && c.Number > j.To) {
			continue
		}
		todo = append(todo, c)
	}
	if len(todo) == 0 {
		s.setJob(j, func(j *Job) { j.Message = "Everything in that range is already saved." })
		return nil
	}
	st := s.GetSettings()
	s.setJob(j, func(j *Job) {
		j.Total, j.Done, j.Message = len(todo), 0, fmt.Sprintf("Downloading %d chapters…", len(todo))
	})

	sem := make(chan struct{}, st.ChapterWorkers)
	var wg sync.WaitGroup
	var blocked error
	var bmu sync.Mutex
	for _, c := range todo {
		if err := s.waitNetwork(ctx, j); err != nil {
			break
		}
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(c Chapter) {
			defer wg.Done()
			defer func() { <-sem }()
			res, err := s.eng.DownloadChapter(ctx, b, c, s.store.ChapterDir(b.Slug, c.Key), st.PageWorkers, func(done, total int) {
				s.setJob(j, func(j *Job) { j.Message = fmt.Sprintf("%s — page %d of %d", c.Title, done, total) })
			})
			if ctx.Err() != nil {
				return
			}
			s.updateBook(b.Slug, func(nb *Book) {
				if nc := nb.ByKey(c.Key); nc != nil {
					if err != nil {
						nc.State, nc.Error = StateFailed, friendly(err).Error()
					} else {
						nc.State, nc.Error, nc.Pages, nc.Bytes, nc.SavedAt = StateOK, "", res.Pages, res.Bytes, time.Now()
					}
				}
			})
			if err != nil && (errors.Is(err, fetch.ErrBlocked) || errors.Is(err, browser.ErrChallengeTimeout)) {
				bmu.Lock()
				blocked = err
				bmu.Unlock()
			}
			s.setJob(j, func(j *Job) {
				j.Done++
				if err != nil {
					j.Failed++
				}
			})
			s.emit("library", map[string]any{"slug": b.Slug, "chapter": c.Key})
		}(c)
	}
	wg.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	s.setJob(j, func(j *Job) {
		j.Message = fmt.Sprintf("Saved %d of %d chapters.", j.Done-j.Failed, j.Total)
		if j.Failed > 0 {
			j.Message += fmt.Sprintf(" %d failed; use Retry failed to try again.", j.Failed)
		}
	})
	if blocked != nil && j.Failed == j.Total {
		return blocked
	}
	return nil
}

// waitNetwork holds a download while MayDownload says no (on mobile data
// with "Only on Wi-Fi" on), checking every few seconds.
func (s *Service) waitNetwork(ctx context.Context, j *Job) error {
	ok := s.opts.MayDownload
	if ok == nil || ok() {
		return ctx.Err()
	}
	prev := ""
	s.setJob(j, func(j *Job) {
		prev, j.Message = j.Message, "Waiting for Wi-Fi. The download continues when you’re connected."
	})
	t := time.NewTicker(3 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			if ok() {
				s.setJob(j, func(j *Job) { j.Message = prev })
				return nil
			}
		}
	}
}

func (s *Service) runUpdate(ctx context.Context, j *Job) error {
	b, err := s.store.Load(j.Slug)
	if err != nil {
		return err
	}
	s.setJob(j, func(j *Job) { j.Message = "Checking " + b.SiteName + " for new chapters…" })
	ser, err := s.eng.Discover(ctx, b.URL, nil)
	if err != nil {
		return err
	}
	var added []int
	s.updateBook(b.Slug, func(nb *Book) {
		added = mergeChapters(nb, ser.Chapters)
		nb.Vars = ser.Vars
		nb.CheckedAt = time.Now()
		nb.NewChapters += len(added)
		if ser.Cover != "" {
			nb.CoverURL = ser.Cover
		}
		if ser.Status != "" {
			nb.Status = ser.Status
		}
		if ser.Title != "" && ser.Title != "Untitled" {
			nb.Title = ser.Title
		}
	})
	if len(added) == 0 {
		s.setJob(j, func(j *Job) { j.Message = "No new chapters." })
		return nil
	}
	s.setJob(j, func(j *Job) { j.Message = fmt.Sprintf("%d new chapters.", len(added)) })
	s.emit("newChapters", map[string]any{"slug": b.Slug, "title": b.Title, "count": len(added)})
	if s.GetSettings().AutoDownloadNew {
		ok, _, _, _ := b.Counts()
		if ok > 0 { // only follow series the user is actually downloading
			s.enqueue(&Job{Kind: "download", Slug: b.Slug, Title: b.Title, Keys: added})
		}
	}
	return nil
}

func countRemoved(b *Book) int {
	n := 0
	for _, c := range b.Chapters {
		if c.Removed {
			n++
		}
	}
	return n
}

// updateBook loads, changes and saves series.json under a lock.
func (s *Service) updateBook(slug string, f func(*Book)) {
	s.bookMu.Lock()
	defer s.bookMu.Unlock()
	b, err := s.store.Load(slug)
	if err != nil {
		return
	}
	f(b)
	_ = s.store.Save(b)
}

// ---- library API for the UI ----

// LibraryItem is one card on the library screen.
type LibraryItem struct {
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Type        string    `json:"type"`
	SiteName    string    `json:"siteName"`
	URL         string    `json:"url"`
	Cover       string    `json:"cover"`
	Total       int       `json:"total"`
	Downloaded  int       `json:"downloaded"`
	Failed      int       `json:"failed"`
	Bytes       int64     `json:"bytes"`
	ReadNumber  int       `json:"readNumber"`
	ReadKey     int       `json:"readKey"`
	FirstKey    int       `json:"firstKey"` // first saved chapter (0 = none)
	NewChapters int       `json:"newChapters"`
	Status      string    `json:"status"`
	AddedAt     time.Time `json:"addedAt"`
	ReadAt      time.Time `json:"readAt,omitzero"`
}

// Library lists saved series.
func (s *Service) Library() ([]LibraryItem, error) {
	books, err := s.store.List()
	if err != nil {
		return nil, err
	}
	out := []LibraryItem{}
	for _, b := range books {
		ok, failed, _, bytes := b.Counts()
		it := LibraryItem{Slug: b.Slug, Title: b.Title, Type: b.Type, SiteName: b.SiteName, URL: b.URL,
			Total: len(b.Chapters) - countRemoved(b), Downloaded: ok, Failed: failed, Bytes: bytes,
			ReadKey: b.ReadKey, NewChapters: b.NewChapters, Status: b.Status, AddedAt: b.AddedAt, ReadAt: b.ReadAt}
		it.Cover = s.coverPath(b)
		for _, c := range b.Chapters {
			if c.State == StateOK {
				it.FirstKey = c.Key
				break
			}
		}
		if c := b.ByKey(b.ReadKey); c != nil {
			it.ReadNumber = c.Number
		}
		out = append(out, it)
	}
	return out, nil
}

// coverPath is the cover's reader link; when the site wouldn't give the
// cover, the first saved page stands in for it.
func (s *Service) coverPath(b *Book) string {
	if b.CoverFile != "" {
		return FilePrefix + b.Slug + "/" + b.CoverFile
	}
	for _, c := range b.Chapters {
		if c.State == StateOK {
			if files, _ := s.store.PageFiles(b.Slug, c.Key); len(files) > 0 {
				return fmt.Sprintf(FilePrefix+"%s/ch/%05d/%s", b.Slug, c.Key, files[0])
			}
		}
	}
	return ""
}

// SeriesDetail is a series plus UI extras.
type SeriesDetail struct {
	*Book
	Cover    string `json:"cover"`
	Folder   string `json:"folder"`
	ReadMode string `json:"effectiveReadMode"`
}

// Series returns one series for the detail screen.
func (s *Service) Series(slug string) (*SeriesDetail, error) {
	b, err := s.store.Load(slug)
	if err != nil {
		return nil, err
	}
	d := &SeriesDetail{Book: b, Folder: s.store.Dir(slug), ReadMode: s.readMode(b)}
	d.Cover = s.coverPath(b)
	return d, nil
}

func (s *Service) readMode(b *Book) string {
	if b.ReadMode != "" {
		return b.ReadMode
	}
	if m := s.GetSettings().ReadMode; m != "" && m != "auto" {
		return m
	}
	// Scrolling down a long strip is the default for every kind; pages
	// (right-to-left or left-to-right) are a choice in the reader.
	return "strip"
}

// ChapterView is what the reader needs for one chapter.
type ChapterView struct {
	Slug      string   `json:"slug"`
	Series    string   `json:"series"`
	Key       int      `json:"key"`
	Number    int      `json:"number"`
	Total     int      `json:"total"`
	Title     string   `json:"title"`
	Pages     []string `json:"pages"`
	Dims      [][2]int `json:"dims"` // width, height of each page (0,0 if unknown)
	PrevKey   int      `json:"prevKey"`
	NextKey   int      `json:"nextKey"`
	NextSaved bool     `json:"nextSaved"`
	ReadMode  string   `json:"readMode"`
	ReadPage  int      `json:"readPage"`
	Source    string   `json:"source"`
}

// Chapter returns a saved chapter's page links for the reader.
func (s *Service) Chapter(slug string, key int) (*ChapterView, error) {
	b, err := s.store.Load(slug)
	if err != nil {
		return nil, err
	}
	idx := -1
	for i, c := range b.Chapters {
		if c.Key == key {
			idx = i
		}
	}
	if idx < 0 {
		return nil, fmt.Errorf("that chapter isn't in this series")
	}
	c := b.Chapters[idx]
	v := &ChapterView{Slug: slug, Series: b.Title, Key: key, Number: c.Number, Total: len(b.Chapters), Title: c.Title,
		Pages: []string{}, ReadMode: s.readMode(b), Source: c.URL}
	if c.State == StateOK {
		files, _ := s.store.PageFiles(slug, key)
		for _, f := range files {
			v.Pages = append(v.Pages, fmt.Sprintf(FilePrefix+"%s/ch/%05d/%s", slug, key, f))
			w, h := imageSize(filepath.Join(s.store.ChapterDir(slug, key), f))
			v.Dims = append(v.Dims, [2]int{w, h})
		}
	}
	if idx > 0 {
		v.PrevKey = b.Chapters[idx-1].Key
	}
	if idx+1 < len(b.Chapters) {
		v.NextKey = b.Chapters[idx+1].Key
		v.NextSaved = b.Chapters[idx+1].State == StateOK
	}
	if b.ReadKey == key {
		v.ReadPage = b.ReadPage
	}
	return v, nil
}

// SetReading remembers the reading place.
func (s *Service) SetReading(slug string, key, page int) error {
	s.updateBook(slug, func(b *Book) {
		b.ReadKey, b.ReadPage, b.ReadAt = key, page, time.Now()
		b.NewChapters = 0
	})
	return nil
}

// SetReadMode sets a series' reading mode ("" = default).
func (s *Service) SetReadMode(slug, mode string) error {
	switch mode {
	case "", "strip", "rtl", "ltr":
	default:
		return fmt.Errorf("unknown reading mode %q", mode)
	}
	s.updateBook(slug, func(b *Book) { b.ReadMode = mode })
	return nil
}

// Download queues chapters from..to (0 = all) of a saved series.
func (s *Service) Download(slug string, from, to int) error {
	b, err := s.store.Load(slug)
	if err != nil {
		return err
	}
	s.enqueue(&Job{Kind: "download", Slug: slug, Title: b.Title, From: from, To: to})
	return nil
}

// RetryFailed resets failed chapters and queues them.
func (s *Service) RetryFailed(slug string) error {
	b, err := s.store.Load(slug)
	if err != nil {
		return err
	}
	// Only the chapters that failed: chapters never asked for stay as
	// they are.
	var keys []int
	s.updateBook(slug, func(b *Book) {
		for i := range b.Chapters {
			if b.Chapters[i].State == StateFailed {
				b.Chapters[i].State, b.Chapters[i].Error = StatePending, ""
				keys = append(keys, b.Chapters[i].Key)
			}
		}
	})
	if len(keys) == 0 {
		return nil
	}
	s.enqueue(&Job{Kind: "download", Slug: slug, Title: b.Title, Keys: keys})
	return nil
}

// CheckUpdates queues a new-chapter check for one series.
func (s *Service) CheckUpdates(slug string) error {
	b, err := s.store.Load(slug)
	if err != nil {
		return err
	}
	s.enqueue(&Job{Kind: "update", Slug: slug, Title: b.Title})
	return nil
}

// CheckAll queues a check for every series.
func (s *Service) CheckAll(auto bool) int {
	books, _ := s.store.List()
	for _, b := range books {
		s.enqueue(&Job{Kind: "update", Slug: b.Slug, Title: b.Title, Auto: auto})
	}
	return len(books)
}

// DeleteSeries removes a series and its files.
func (s *Service) DeleteSeries(slug string) error {
	s.mu.Lock()
	busy := false
	for _, r := range s.running {
		busy = busy || r.Slug == slug
	}
	var keep []*Job
	for _, j := range s.queue {
		if j.Slug != slug {
			keep = append(keep, j)
		}
	}
	s.queue = keep
	s.mu.Unlock()
	if busy {
		s.mu.Lock()
		for _, r := range s.running {
			if r.Slug == slug {
				if c := s.cancels[r.ID]; c != nil {
					c()
				}
			}
		}
		s.mu.Unlock()
		time.Sleep(300 * time.Millisecond)
	}
	s.bookMu.Lock()
	err := s.store.Delete(slug)
	s.bookMu.Unlock()
	s.emit("library", map[string]any{"slug": slug, "deleted": true})
	return err
}

// FreeChapters deletes saved pages of chapters from..to to free space; the
// chapters stay in the list and can be downloaded again.
func (s *Service) FreeChapters(slug string, from, to int) (int64, error) {
	var freed int64
	s.bookMu.Lock()
	defer s.bookMu.Unlock()
	b, err := s.store.Load(slug)
	if err != nil {
		return 0, err
	}
	for i := range b.Chapters {
		c := &b.Chapters[i]
		if c.State != StateOK || c.Number < from || (to > 0 && c.Number > to) {
			continue
		}
		if os.RemoveAll(s.store.ChapterDir(slug, c.Key)) == nil {
			freed += c.Bytes
			c.State, c.Pages, c.Bytes = StatePending, 0, 0
		}
	}
	err = s.store.Save(b)
	s.emit("library", map[string]any{"slug": slug})
	return freed, err
}

// ExportCBZ writes a series' downloaded chapters to one CBZ file.
func (s *Service) ExportCBZ(slug, dest string) (*ExportResult, error) {
	b, err := s.store.Load(slug)
	if err != nil {
		return nil, err
	}
	return WriteCBZ(s.store, b, dest, 0, 0)
}

// friendly rewrites low-level errors into sentences.
func friendly(err error) error {
	if err == nil {
		return nil
	}
	var he *fetch.HTTPError
	switch {
	case errors.Is(err, browser.ErrChallengeTimeout):
		return errors.New("the site's 'verify you are human' check wasn't completed. Open the site window, tick the box, then retry")
	case errors.Is(err, fetch.ErrBlocked):
		return errors.New("the site is asking for a 'verify you are human' check. Retry; the site window will open so you can tick it")
	case errors.As(err, &he) && he.Status == 404:
		return fmt.Errorf("the site says this page doesn't exist (404). It may have moved: %s", shortURL(he.URL))
	case errors.As(err, &he) && (he.Status == 403 || he.Status == 401):
		return fmt.Errorf("the site refused the request (%d). Try again later", he.Status)
	case errors.Is(err, context.DeadlineExceeded):
		return errors.New("the site took too long to answer. Try again later")
	}
	msg := err.Error()
	if strings.Contains(msg, "no such host") || strings.Contains(msg, "dial tcp") {
		return errors.New("couldn't reach the site. Check your internet connection")
	}
	return err
}

// Recognises reports whether a link belongs to a supported manga site
// (no network), so a shared Add box can send it to the right engine.
func (s *Service) Recognises(link string) bool {
	_, err := s.eng.Sites.Match(link)
	return err == nil
}

// DownloadKeys queues exactly these chapters of a saved series.
func (s *Service) DownloadKeys(slug string, keys []int) error {
	b, err := s.store.Load(slug)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	s.updateBook(slug, func(nb *Book) {
		for _, k := range keys {
			if c := nb.ByKey(k); c != nil && c.State == StateFailed {
				c.State, c.Error = StatePending, ""
			}
		}
	})
	s.enqueue(&Job{Kind: "download", Slug: slug, Title: b.Title, Keys: append([]int(nil), keys...)})
	return nil
}

// FreeKeys deletes the saved pages of these chapters; they stay listed.
func (s *Service) FreeKeys(slug string, keys []int) (int64, error) {
	want := map[int]bool{}
	for _, k := range keys {
		want[k] = true
	}
	var freed int64
	s.bookMu.Lock()
	defer s.bookMu.Unlock()
	b, err := s.store.Load(slug)
	if err != nil {
		return 0, err
	}
	for i := range b.Chapters {
		c := &b.Chapters[i]
		if !want[c.Key] || c.State != StateOK {
			continue
		}
		if os.RemoveAll(s.store.ChapterDir(slug, c.Key)) == nil {
			freed += c.Bytes
			c.State, c.Pages, c.Bytes = StatePending, 0, 0
		}
	}
	err = s.store.Save(b)
	s.emit("library", map[string]any{"slug": slug})
	return freed, err
}
