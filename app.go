package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"novelarchiver/internal/archiver"
	"novelarchiver/internal/fetch"
	"novelarchiver/internal/library"
	"novelarchiver/internal/sites"
	"novelarchiver/internal/source"
)

// App is bound to the frontend (window.go.main.App.*).
type App struct {
	ctx     context.Context
	store   *library.Store
	engine  *source.Engine
	arch    *archiver.Archiver
	initErr error

	mu       sync.Mutex
	busy     bool
	cancel   context.CancelFunc
	lastDisc map[string]discovered
}

type discovered struct {
	novel *source.Novel
	at    time.Time
}

// NewApp wires the engine, library and archiver together.
func NewApp() *App {
	a := &App{lastDisc: map[string]discovered{}}
	store, err := library.Open(library.DefaultRoot())
	if err != nil {
		a.initErr = err
		return a
	}
	userSites := filepath.Join(filepath.Dir(store.Root), "sites")
	reg, err := sites.Load(userSites)
	if err != nil {
		a.initErr = err
		return a
	}
	httpF := fetch.NewHTTP(fetch.Options{})
	a.store = store
	a.engine = &source.Engine{Registry: reg, HTTP: httpF}
	a.arch = &archiver.Archiver{Engine: a.engine, Store: store, Files: httpF}
	return a
}

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

func (a *App) emit(event string, data any) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, event, data)
	}
}

func (a *App) ready() error {
	if a.initErr != nil {
		return fmt.Errorf("library could not start: %w", a.initErr)
	}
	return nil
}

// ---------- Library ----------

// NovelItem is a library card.
type NovelItem struct {
	Slug          string `json:"slug"`
	Title         string `json:"title"`
	Author        string `json:"author"`
	Cover         string `json:"cover"`
	Description   string `json:"description"`
	URL           string `json:"url"`
	Site          string `json:"site"`
	TotalChapters int    `json:"totalChapters"`
	RawCount      int    `json:"rawCount"`
	CleanCount    int    `json:"cleanCount"`
	FailedCount   int    `json:"failedCount"`
	LockedCount   int    `json:"lockedCount"`
	HasIndex      bool   `json:"hasIndex"`
	CatalogURL    string `json:"catalogUrl"`
	SizeBytes     int64  `json:"sizeBytes"`
	SizeMb        string `json:"sizeMb"`
}

func (a *App) item(b *library.Book) NovelItem {
	ok, failed, locked, _ := b.Counts()
	size := a.store.Size(b.Slug)
	cover := b.CoverURL
	if b.CoverFile != "" {
		cover = "/library/" + b.Slug + "/cover"
	}
	return NovelItem{
		Slug: b.Slug, Title: b.Title, Author: b.Author, Cover: cover, Description: b.Description,
		URL: b.URL, Site: b.SiteName, TotalChapters: len(b.Chapters),
		RawCount: ok, CleanCount: ok, FailedCount: failed, LockedCount: locked,
		HasIndex: ok > 0, CatalogURL: "/library/" + b.Slug + "/",
		SizeBytes: size, SizeMb: fmt.Sprintf("%.1f MB", float64(size)/(1<<20)),
	}
}

// GetNovels lists the library.
func (a *App) GetNovels() ([]NovelItem, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	books, err := a.store.List()
	if err != nil {
		return nil, err
	}
	out := make([]NovelItem, 0, len(books))
	for _, b := range books {
		out = append(out, a.item(b))
	}
	return out, nil
}

// NovelDetail is GetNovel's result.
type NovelDetail struct {
	NovelItem
	Chapters []ChapterRow `json:"chapters"`
}

// ChapterRow is one TOC row for the details panel.
type ChapterRow struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	State     string `json:"state"`
	Error     string `json:"error,omitempty"`
	ReaderURL string `json:"readerUrl"`
	IsClean   bool   `json:"isClean"`
}

// GetNovel returns a book with its chapter list.
func (a *App) GetNovel(slug string) (*NovelDetail, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	b, err := a.store.Load(slug)
	if err != nil {
		return nil, err
	}
	d := &NovelDetail{NovelItem: a.item(b)}
	for _, c := range b.Chapters {
		d.Chapters = append(d.Chapters, ChapterRow{
			Number: c.Number, Title: c.Title, State: c.State, Error: c.Error,
			ReaderURL: fmt.Sprintf("/library/%s/%d", b.Slug, c.Number), IsClean: c.State == library.StateOK,
		})
	}
	return d, nil
}

// SystemStats are library totals.
type SystemStats struct {
	TotalNovels        int    `json:"totalNovels"`
	TotalRawChapters   int    `json:"totalRawChapters"`
	TotalCleanChapters int    `json:"totalCleanChapters"`
	TotalBytes         int64  `json:"totalBytes"`
	SizeMb             string `json:"sizeMb"`
	LibraryPath        string `json:"libraryPath"`
}

// GetSystemStats sums the library.
func (a *App) GetSystemStats() (*SystemStats, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	books, err := a.store.List()
	if err != nil {
		return nil, err
	}
	s := &SystemStats{TotalNovels: len(books), LibraryPath: a.store.Root}
	for _, b := range books {
		ok, _, _, _ := b.Counts()
		s.TotalRawChapters += ok
		s.TotalCleanChapters += ok
		s.TotalBytes += a.store.Size(b.Slug)
	}
	s.SizeMb = fmt.Sprintf("%.1f MB", float64(s.TotalBytes)/(1<<20))
	return s, nil
}

// ---------- Discovery & downloads ----------

// DiscoverResult is what the preview card shows.
type DiscoverResult struct {
	*source.Novel
	TotalChapters int    `json:"totalChapters"`
	ExistingSlug  string `json:"existingSlug,omitempty"`
	Downloaded    int    `json:"downloaded"`
}

// DiscoverNovel accepts a novel page or chapter link and returns the
// preview (title, author, chapter count, warnings).
func (a *App) DiscoverNovel(url string) (*DiscoverResult, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	n, err := a.discover(context.Background(), url)
	if err != nil {
		return nil, friendly(err)
	}
	res := &DiscoverResult{Novel: n, TotalChapters: len(n.Chapters)}
	if slug := a.store.FindByURL(n.URL); slug != "" {
		res.ExistingSlug = slug
		if b, err := a.store.Load(slug); err == nil {
			res.Downloaded, _, _, _ = b.Counts()
		}
	}
	// The UI shows the cover straight from the site during preview.
	return res, nil
}

func (a *App) discover(ctx context.Context, url string) (*source.Novel, error) {
	key := strings.TrimSpace(url)
	a.mu.Lock()
	if d, ok := a.lastDisc[key]; ok && time.Since(d.at) < 10*time.Minute {
		a.mu.Unlock()
		return d.novel, nil
	}
	a.mu.Unlock()
	n, err := a.engine.Discover(ctx, url, func(msg string) {
		a.emit("status", map[string]any{"message": msg})
	})
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	a.lastDisc[key] = discovered{n, time.Now()}
	a.mu.Unlock()
	return n, nil
}

// ScrapeOptions come from the download form.
type ScrapeOptions struct {
	URL          string `json:"url"`
	StartChapter int    `json:"startChapter"`
	EndChapter   int    `json:"endChapter"`
	Concurrency  int    `json:"concurrency"`
	Force        bool   `json:"force"`
	SkipImages   bool   `json:"skipImages"`
	// FromPasted: when a chapter link was pasted, start from that chapter.
	FromPasted bool `json:"fromPasted"`
}

// StartScrape discovers (or reuses the preview) and downloads in the
// background. Progress arrives as "started/status/progress/done/error".
func (a *App) StartScrape(opts ScrapeOptions) (map[string]any, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	ctx, err := a.begin()
	if err != nil {
		return nil, err
	}
	go func() {
		defer a.end()
		a.emit("status", map[string]any{"message": "Reading the novel's page…"})
		n, err := a.discover(ctx, opts.URL)
		if err != nil {
			a.fail(ctx, err)
			return
		}
		a.download(ctx, n, opts)
	}()
	return map[string]any{"status": "started"}, nil
}

func (a *App) download(ctx context.Context, n *source.Novel, opts ScrapeOptions) {
	for _, w := range n.Warnings {
		a.emit("status", map[string]any{"message": w})
	}
	b, err := a.arch.BookFromNovel(n)
	if err != nil {
		a.fail(ctx, err)
		return
	}
	from := opts.StartChapter
	if opts.FromPasted && n.InputChapter > 0 && from == 0 {
		from = n.InputChapter
	}
	err = a.arch.Run(ctx, b, n, archiver.Options{
		From: from, To: opts.EndChapter, Concurrency: opts.Concurrency, Force: opts.Force, SkipImages: opts.SkipImages,
	}, a.emit)
	if err != nil {
		a.fail(ctx, err)
	}
}

// RetryNovel re-checks the source for new chapters and downloads anything
// missing or failed. It also serves as "Update".
func (a *App) RetryNovel(slug string) (map[string]any, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	b, err := a.store.Load(slug)
	if err != nil {
		return nil, err
	}
	ctx, err := a.begin()
	if err != nil {
		return nil, err
	}
	go func() {
		defer a.end()
		a.emit("status", map[string]any{"message": "Checking " + b.SiteName + " for missing and new chapters…"})
		input := b.URL
		if input == "" {
			input = b.FirstChapterURL
		}
		a.mu.Lock()
		delete(a.lastDisc, input)
		a.mu.Unlock()
		n, err := a.discover(ctx, input)
		if err != nil {
			a.fail(ctx, err)
			return
		}
		a.download(ctx, n, ScrapeOptions{})
	}()
	return map[string]any{"status": "started", "slug": slug}, nil
}

// CleanNovel is kept for the existing UI button; chapters are cleaned as
// they download, so this now means "fetch missing chapters".
func (a *App) CleanNovel(slug string) (map[string]any, error) { return a.RetryNovel(slug) }

// AuditNovel reports chapter states.
func (a *App) AuditNovel(slug string) (map[string]any, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	b, err := a.store.Load(slug)
	if err != nil {
		return nil, err
	}
	ok, failed, locked, pending := b.Counts()
	var invalid []int
	for _, c := range b.Chapters {
		if c.State == library.StateFailed || c.State == library.StatePending {
			invalid = append(invalid, c.Number)
		}
	}
	return map[string]any{
		"total": len(b.Chapters), "validCount": ok, "invalidCount": failed + pending,
		"failed": failed, "pending": pending, "locked": locked, "invalid": invalid,
	}, nil
}

// DeleteNovel removes a novel and all its files.
func (a *App) DeleteNovel(slug string) (map[string]any, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	if err := a.store.DeleteBook(slug); err != nil {
		return nil, err
	}
	return map[string]any{"success": true, "slug": slug}, nil
}

// DeleteChapters frees space by deleting downloaded chapters from..to.
func (a *App) DeleteChapters(slug string, from, to int) (map[string]any, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	n, err := a.store.DeleteChapters(slug, from, to)
	if err != nil {
		return nil, err
	}
	return map[string]any{"deleted": n, "sizeBytes": a.store.Size(slug)}, nil
}

// DeleteImages removes a novel's chapter images.
func (a *App) DeleteImages(slug string) (map[string]any, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	if err := a.store.DeleteImages(slug); err != nil {
		return nil, err
	}
	return map[string]any{"sizeBytes": a.store.Size(slug)}, nil
}

// CancelTask stops the running job. The job goroutine clears the busy flag
// itself when it has actually stopped.
func (a *App) CancelTask() error {
	a.mu.Lock()
	cancel := a.cancel
	a.mu.Unlock()
	if cancel != nil {
		cancel()
		a.emit("status", map[string]any{"message": "Stopping… chapters already saved are kept."})
	}
	return nil
}

// OpenLibraryFolder shows the library folder in Explorer/Finder.
func (a *App) OpenLibraryFolder() {
	if a.ctx != nil && a.store != nil {
		runtime.BrowserOpenURL(a.ctx, "file:///"+filepath.ToSlash(a.store.Root))
	}
}

// OpenExternalURL opens a link in the system browser.
func (a *App) OpenExternalURL(url string) {
	if a.ctx != nil && (strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://")) {
		runtime.BrowserOpenURL(a.ctx, url)
	}
}

// ---------- job bookkeeping ----------

func (a *App) begin() (context.Context, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.busy {
		return nil, errors.New("a download is already running. Cancel it or wait for it to finish")
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.busy, a.cancel = true, cancel
	return ctx, nil
}

func (a *App) end() {
	if r := recover(); r != nil {
		a.emit("error", map[string]any{"message": fmt.Sprintf("internal error: %v", r)})
	}
	a.mu.Lock()
	if a.cancel != nil {
		a.cancel()
	}
	a.busy, a.cancel = false, nil
	a.mu.Unlock()
}

func (a *App) fail(ctx context.Context, err error) {
	if ctx.Err() != nil {
		a.emit("status", map[string]any{"message": "Download stopped. Saved chapters are kept; press Retry to resume."})
		return
	}
	a.emit("error", map[string]any{"message": friendly(err).Error()})
}

// friendly turns engine errors into messages a reader understands.
func friendly(err error) error {
	switch {
	case errors.Is(err, fetch.ErrNeedsBrowser):
		return fmt.Errorf("%v. In-app browser mode isn't built yet; this site will work once it is", err)
	case errors.Is(err, fetch.ErrBlocked):
		return errors.New("the site showed a bot check or refused access. Open it in your browser to confirm it works, then try again")
	}
	var he *fetch.HTTPError
	if errors.As(err, &he) && he.Status == 404 {
		return errors.New("that page doesn't exist (404). Check the link")
	}
	return err
}

