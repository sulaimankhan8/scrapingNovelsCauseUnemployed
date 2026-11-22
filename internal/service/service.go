// Package service is the platform-neutral application layer shared by the
// desktop (Wails) app and the Android app: library queries, discovery,
// download jobs and storage actions. Platforms supply an event sink and,
// optionally, a site browser.
package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"novelarchiver/internal/archiver"
	"novelarchiver/internal/browser"
	"novelarchiver/internal/export"
	"novelarchiver/internal/fetch"
	"novelarchiver/internal/library"
	"novelarchiver/internal/sites"
	"novelarchiver/internal/source"
)

// SiteBrowser is a real-browser fetcher the user can also see and use.
type SiteBrowser interface {
	fetch.Fetcher
	Show(url string) error
	Close()
}

// Service holds the library, engine and the single running job.
type Service struct {
	// Emit delivers UI events ("started", "status", "progress", "done",
	// "error", "challenge"). Set it before starting jobs.
	Emit func(event string, data any)

	store   *library.Store
	engine  *source.Engine
	arch    *archiver.Archiver
	browser SiteBrowser
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

// New opens the library at root (library.DefaultRoot() on desktop).
// Errors are reported by every method, so the UI can still start.
func New(root string) *Service {
	a := &Service{lastDisc: map[string]discovered{}}
	store, err := library.Open(root)
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

// SetBrowser plugs in the platform's site browser (WebView2 on Windows,
// Android WebView on phones).
func (a *Service) SetBrowser(b SiteBrowser) {
	a.browser = b
	if a.engine != nil {
		a.engine.Browser = b
	}
}

// LibraryRoot is the absolute library folder ("" if it failed to open).
func (a *Service) LibraryRoot() string {
	if a.store == nil {
		return ""
	}
	return a.store.Root
}

// Store exposes the library (for the reader handler).
func (a *Service) Store() *library.Store { return a.store }

// ChallengeMessage is what the UI shows when a human check is pending.
const ChallengeMessage = "Please complete the 'Verify you are human' check in the NovelArchiver site browser. The download continues automatically."

// Shutdown cancels the running job and closes the site browser.
func (a *Service) Shutdown() {
	a.mu.Lock()
	if a.cancel != nil {
		a.cancel()
	}
	a.mu.Unlock()
	if a.browser != nil {
		a.browser.Close()
	}
}

// OpenSiteBrowser shows the site browser window at url, so the user can
// complete a human check or sign in before downloading.
func (a *Service) OpenSiteBrowser(url string) error {
	if a.browser == nil {
		return browser.ErrUnsupported
	}
	if url != "" && !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return errors.New("enter a full https:// link")
	}
	return a.browser.Show(url)
}

func (a *Service) emit(event string, data any) {
	if a.Emit != nil {
		a.Emit(event, data)
	}
}

func (a *Service) ready() error {
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

func (a *Service) item(b *library.Book) NovelItem {
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
func (a *Service) GetNovels() ([]NovelItem, error) {
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
func (a *Service) GetNovel(slug string) (*NovelDetail, error) {
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
func (a *Service) GetSystemStats() (*SystemStats, error) {
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
	// CoverData is the cover as a data: URL fetched by the engine, so it
	// shows even when the site refuses hot-linking (Cloudflare etc.).
	CoverData  string `json:"coverData,omitempty"`
	Downloaded int    `json:"downloaded"`
}

// DiscoverNovel accepts a novel page or chapter link and returns the
// preview (title, author, chapter count, warnings).
func (a *Service) DiscoverNovel(url string) (*DiscoverResult, error) {
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
	res.CoverData = a.coverData(n)
	return res, nil
}

func (a *Service) discover(ctx context.Context, url string) (*source.Novel, error) {
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
func (a *Service) StartScrape(opts ScrapeOptions) (map[string]any, error) {
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

func (a *Service) download(ctx context.Context, n *source.Novel, opts ScrapeOptions) {
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
func (a *Service) RetryNovel(slug string) (map[string]any, error) {
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
func (a *Service) CleanNovel(slug string) (map[string]any, error) { return a.RetryNovel(slug) }

// AuditNovel reports chapter states.
func (a *Service) AuditNovel(slug string) (map[string]any, error) {
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
func (a *Service) DeleteNovel(slug string) (map[string]any, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	if err := a.store.DeleteBook(slug); err != nil {
		return nil, err
	}
	return map[string]any{"success": true, "slug": slug}, nil
}

// DeleteChapters frees space by deleting downloaded chapters from..to.
func (a *Service) DeleteChapters(slug string, from, to int) (map[string]any, error) {
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
func (a *Service) DeleteImages(slug string) (map[string]any, error) {
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
func (a *Service) CancelTask() error {
	a.mu.Lock()
	cancel := a.cancel
	a.mu.Unlock()
	if cancel != nil {
		cancel()
		a.emit("status", map[string]any{"message": "Stopping… chapters already saved are kept."})
	}
	return nil
}

// ---------- job bookkeeping ----------

func (a *Service) begin() (context.Context, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.busy {
		return nil, errors.New("a download is already running. Cancel it or wait for it to finish")
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.busy, a.cancel = true, cancel
	return ctx, nil
}

func (a *Service) end() {
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

func (a *Service) fail(ctx context.Context, err error) {
	if ctx.Err() != nil {
		a.emit("status", map[string]any{"message": "Download stopped. Saved chapters are kept; press Retry to resume."})
		return
	}
	a.emit("error", map[string]any{"message": friendly(err).Error()})
}

// friendly turns engine errors into messages a reader understands.
func friendly(err error) error {
	switch {
	case errors.Is(err, fetch.ErrNeedsBrowser), errors.Is(err, browser.ErrUnsupported):
		return fmt.Errorf("%v. The site browser isn't available on this platform yet", err)
	case errors.Is(err, browser.ErrChallengeTimeout):
		return errors.New("the site's 'Verify you are human' check wasn't completed. Press Update to try again")
	case errors.Is(err, browser.ErrClosed):
		return errors.New("the site browser window was closed. Press Update to resume; it reopens automatically")
	case errors.Is(err, fetch.ErrBlocked):
		return errors.New("the site showed a bot check or refused access. Open it in your browser to confirm it works, then try again")
	}
	var he *fetch.HTTPError
	if errors.As(err, &he) && he.Status == 404 {
		return errors.New("that page doesn't exist (404). Check the link")
	}
	return err
}

// ExportResult describes a written EPUB.
type ExportResult struct {
	Path     string `json:"path"`
	FileName string `json:"fileName"`
	Chapters int    `json:"chapters"`
	Images   int    `json:"images"`
	SizeMb   string `json:"sizeMb"`
}

// EPUBFileName suggests a file name for a novel's EPUB.
func (a *Service) EPUBFileName(slug string) string {
	if a.store == nil {
		return "novel.epub"
	}
	if b, err := a.store.Load(slug); err == nil {
		return export.FileName(b.Title)
	}
	return export.FileName(slug)
}

// ExportEPUB writes the downloaded chapters (from..to, 0 = all) of a novel
// to dest as an EPUB file.
func (a *Service) ExportEPUB(slug, dest string, from, to int) (*ExportResult, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	res, err := export.EPUBFile(a.store, slug, dest, export.Options{From: from, To: to})
	if err != nil {
		return nil, err
	}
	return &ExportResult{Path: dest, FileName: filepath.Base(dest), Chapters: res.Chapters, Images: res.Images,
		SizeMb: fmt.Sprintf("%.1f MB", float64(res.Bytes)/(1<<20))}, nil
}

// coverData fetches the preview cover (plain HTTP, then the site browser)
// and returns it as a data: URL, or "" if it can't be fetched.
func (a *Service) coverData(n *source.Novel) string {
	if n.Cover == "" || a.arch == nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	page, err := a.arch.GetFile(ctx, n.Cover, n.URL)
	if err != nil || len(page.Body) > 3<<20 {
		return ""
	}
	ct := page.Header.Get("Content-Type")
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = ct[:i]
	}
	if !strings.HasPrefix(ct, "image/") {
		ct = http.DetectContentType(page.Body)
	}
	return "data:" + ct + ";base64," + base64.StdEncoding.EncodeToString(page.Body)
}
