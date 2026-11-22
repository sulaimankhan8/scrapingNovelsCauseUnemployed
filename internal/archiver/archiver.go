// Package archiver runs download jobs: discover → merge with the stored
// book → download chapters (with images) → save, reporting progress.
package archiver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"novelarchiver/internal/clean"
	"novelarchiver/internal/fetch"
	"novelarchiver/internal/library"
	"novelarchiver/internal/source"
)

// Emit sends a UI event ("started", "status", "progress", "done", "error").
type Emit func(event string, data any)

// Options for one job.
type Options struct {
	From        int  // first chapter number (0 = 1)
	To          int  // last chapter number (0 = last)
	Concurrency int  // parallel chapter downloads (capped by per-host limit)
	Force       bool // re-download chapters already saved
	SkipImages  bool
	// Revise re-reads chapters that are already saved and replaces those
	// whose text changed on the site: 0 = off, -1 = every chapter,
	// N > 0 = only the last N saved chapters.
	Revise int
	// Gate, when set, is called before each chapter download. It may block
	// (e.g. "waiting for Wi-Fi") and returns an error to stop the job.
	Gate func(ctx context.Context) error
	// NoDone suppresses the "done" event so the caller can send its own
	// (with an update summary).
	NoDone bool
	// Result, when set, receives what the job changed.
	Result *Result
}

// Result reports what a job changed, for the update summary.
type Result struct {
	CoverChanged bool
	Revised      []int // chapter numbers whose saved text was replaced
	Checked      int   // saved chapters re-read for revisions
}

// Progress matches the event shape the frontend already understands.
type Progress struct {
	Stage        string `json:"stage"`
	Current      int    `json:"current"`
	Total        int    `json:"total"`
	Scraped      int    `json:"scraped"`
	Cached       int    `json:"cached"`
	Failed       int    `json:"failed"`
	Speed        string `json:"speed"`
	Eta          string `json:"eta"`
	Percentage   int    `json:"percentage"`
	ChapterNum   int    `json:"chapterNum,omitempty"`
	ChapterTitle string `json:"chapterTitle,omitempty"`
	Status       string `json:"status,omitempty"`
	Message      string `json:"message,omitempty"`
	Slug         string `json:"slug,omitempty"`
	Title        string `json:"title,omitempty"`
}

// Archiver ties the engine to the library.
type Archiver struct {
	Engine *source.Engine
	Store  *library.Store
	Files  fetch.Fetcher // used for covers and images
}

// BookFromNovel merges a fresh discovery into the stored book (if any), so
// re-running a job resumes instead of starting over and "update" picks up
// newly released chapters.
func (a *Archiver) BookFromNovel(n *source.Novel) (*library.Book, error) {
	slug := a.Store.FindByURL(n.URL)
	var b *library.Book
	if slug != "" {
		var err error
		if b, err = a.Store.Load(slug); err != nil {
			return nil, err
		}
	} else {
		b = &library.Book{Slug: a.Store.NewSlug(n.Title)}
	}
	b.SiteID, b.SiteName, b.URL = n.SiteID, n.SiteName, n.URL
	b.Title, b.Author, b.Description, b.Status = n.Title, n.Author, n.Description, n.Status
	b.CoverURL, b.TOCMode, b.Vars = n.Cover, n.TOCMode, n.Vars
	if b.FirstChapterURL == "" || n.InputKind != "chapter" {
		b.FirstChapterURL = n.FirstChapterURL
	}

	if n.TOCMode == "list" {
		old := map[string]library.Chapter{}
		for _, c := range b.Chapters {
			old[chapterKey(c.URL, c.Index)] = c
		}
		merged := make([]library.Chapter, 0, len(n.Chapters))
		for _, r := range n.Chapters {
			c := library.Chapter{Number: r.Number, Title: r.Title, URL: r.URL, Index: r.Index, Locked: r.Locked}
			if prev, ok := old[chapterKey(r.URL, r.Index)]; ok && prev.Number == r.Number {
				c.State, c.Error, c.Words = prev.State, prev.Error, prev.Words
			}
			if c.Locked {
				c.State = library.StateLocked
			}
			merged = append(merged, c)
		}
		// A saved chapter the site no longer lists is kept (marked Removed),
		// unless its number is now used by a different chapter.
		seen := map[string]bool{}
		used := map[int]bool{}
		for _, c := range merged {
			seen[chapterKey(c.URL, c.Index)] = true
			used[c.Number] = true
		}
		for _, c := range b.Chapters {
			if c.State != library.StateOK || seen[chapterKey(c.URL, c.Index)] || used[c.Number] {
				continue
			}
			c.Removed = true
			merged = append(merged, c)
			used[c.Number] = true
		}
		sort.SliceStable(merged, func(i, j int) bool { return merged[i].Number < merged[j].Number })
		b.Chapters = merged
	}
	return b, nil
}

func chapterKey(u string, idx int) string {
	if u != "" {
		return u
	}
	return fmt.Sprintf("#%d", idx)
}

// Run downloads chapters of b according to opts.
func (a *Archiver) Run(ctx context.Context, b *library.Book, novel *source.Novel, opts Options, emit Emit) error {
	if err := a.Store.Save(b); err != nil {
		return err
	}
	emit("started", map[string]any{"slug": b.Slug, "title": b.Title, "stage": "scraping"})
	if changed := a.saveCover(ctx, b); changed && opts.Result != nil {
		opts.Result.CoverChanged = true
	}

	var err error
	if b.TOCMode == "crawl" {
		err = a.crawl(ctx, b, novel, opts, emit)
	} else {
		err = a.downloadList(ctx, b, novel, opts, emit)
	}
	if err == nil && opts.Revise != 0 {
		err = a.revise(ctx, b, novel, opts, emit)
	}
	if saveErr := a.Store.Save(b); err == nil {
		err = saveErr
	}
	if err != nil {
		return err
	}
	if opts.NoDone {
		return nil
	}
	ok, failed, locked, _ := b.Counts()
	emit("done", map[string]any{
		"slug": b.Slug, "title": b.Title, "catalogUrl": "/library/" + b.Slug + "/",
		"ok": ok, "failed": failed, "locked": locked,
	})
	return nil
}

func (a *Archiver) selectChapters(b *library.Book, opts Options) ([]int, int) {
	from, to := opts.From, opts.To
	if from <= 0 {
		from = 1
	}
	if to <= 0 || to > len(b.Chapters) {
		to = len(b.Chapters)
	}
	var idx []int
	cached := 0
	for i, c := range b.Chapters {
		if c.Number < from || c.Number > to || c.State == library.StateLocked {
			continue
		}
		if c.State == library.StateOK && !opts.Force {
			cached++
			continue
		}
		idx = append(idx, i)
	}
	return idx, cached
}

func (a *Archiver) downloadList(ctx context.Context, b *library.Book, novel *source.Novel, opts Options, emit Emit) error {
	if opts.From > 0 && opts.To > 0 && opts.To < opts.From {
		return fmt.Errorf("chapter range %d–%d is empty", opts.From, opts.To)
	}
	if opts.From > len(b.Chapters) {
		return fmt.Errorf("this novel has %d chapters; can't start at %d", len(b.Chapters), opts.From)
	}
	todo, cached := a.selectChapters(b, opts)
	total := len(todo) + cached
	if len(todo) == 0 {
		emit("status", map[string]any{"message": "All selected chapters are already downloaded."})
		return nil
	}
	workers := opts.Concurrency
	if workers <= 0 {
		workers = 4
	}
	if workers > 8 {
		workers = 8
	}

	var (
		mu       sync.Mutex // guards b.Chapters entries and saves
		done     int64
		scraped  int64
		failed   int64
		blocked  atomic.Value
		start    = time.Now()
		queue    = make(chan int)
		wg       sync.WaitGroup
		jobCtx   context.Context
		stopJobs context.CancelFunc
	)
	jobCtx, stopJobs = context.WithCancel(ctx)
	defer stopJobs()

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range queue {
				if opts.Gate != nil {
					if err := opts.Gate(jobCtx); err != nil {
						stopJobs()
						return
					}
				}
				mu.Lock()
				ref := b.Chapters[i]
				mu.Unlock()
				state, words, title, errMsg := a.downloadOne(jobCtx, b, novel, ref, opts)
				if jobCtx.Err() != nil && state != library.StateOK {
					return
				}
				mu.Lock()
				c := &b.Chapters[i]
				c.State, c.Words, c.Error = state, words, errMsg
				if title != "" && (c.Title == "" || strings.HasPrefix(c.Title, "Chapter ") && len(c.Title) <= len("Chapter 00000")) {
					c.Title = title // sequential TOCs only know "Chapter N"
				}
				ref.Title = c.Title
				mu.Unlock()

				status := "ok"
				if state == library.StateOK {
					atomic.AddInt64(&scraped, 1)
				} else {
					status = "err"
					atomic.AddInt64(&failed, 1)
					if strings.Contains(errMsg, "blocked") || strings.Contains(errMsg, "in-app browser") {
						blocked.Store(errMsg)
						stopJobs()
					}
				}
				n := atomic.AddInt64(&done, 1)
				if n%10 == 0 {
					mu.Lock()
					_ = a.Store.Save(b)
					mu.Unlock()
				}
				cur := int(n) + cached
				elapsed := math.Max(time.Since(start).Seconds(), 0.1)
				speed := float64(n) / elapsed
				eta := time.Duration(float64(len(todo)-int(n))/math.Max(speed, 0.01)) * time.Second
				emit("progress", Progress{
					Stage: "scraping", Current: cur, Total: total,
					Scraped: int(atomic.LoadInt64(&scraped)), Cached: cached, Failed: int(atomic.LoadInt64(&failed)),
					Speed: fmt.Sprintf("%.1f ch/s", speed), Eta: eta.Round(time.Second).String(),
					Percentage: int(math.Round(float64(cur) / float64(total) * 100)),
					ChapterNum: ref.Number, ChapterTitle: ref.Title, Status: status, Slug: b.Slug, Title: b.Title,
				})
			}
		}()
	}

feed:
	for _, i := range todo {
		select {
		case queue <- i:
		case <-jobCtx.Done():
			break feed
		}
	}
	close(queue)
	wg.Wait()

	if msg, ok := blocked.Load().(string); ok {
		return fmt.Errorf("stopped: %s. Open the site in the in-app browser, then press Retry", msg)
	}
	return ctx.Err()
}

// crawl follows "next" links for sites without a usable chapter list.
func (a *Archiver) crawl(ctx context.Context, b *library.Book, novel *source.Novel, opts Options, emit Emit) error {
	next := b.FirstChapterURL
	// Resume after the last saved chapter.
	if len(b.Chapters) > 0 && !opts.Force {
		last := b.Chapters[len(b.Chapters)-1]
		if last.State == library.StateOK {
			next = b.Vars["crawlNext"]
			if next == "" {
				emit("status", map[string]any{"message": "Already up to date."})
				return nil
			}
		}
	}
	if next == "" {
		return fmt.Errorf("no starting chapter to crawl from")
	}
	limit := opts.To
	start := time.Now()
	count := 0
	seen := map[string]bool{}
	for next != "" && !seen[next] {
		if err := ctx.Err(); err != nil {
			return err
		}
		if opts.Gate != nil {
			if err := opts.Gate(ctx); err != nil {
				return err
			}
		}
		num := len(b.Chapters) + 1
		if limit > 0 && num > limit {
			break
		}
		seen[next] = true
		ref := source.ChapterRef{Number: num, Title: fmt.Sprintf("Chapter %d", num), URL: next}
		ch, err := a.Engine.FetchChapter(ctx, novel, ref)
		if err != nil {
			if errors.Is(err, source.ErrLocked) {
				emit("status", map[string]any{"message": fmt.Sprintf("Stopped at chapter %d: it's paid/locked on %s.", num, b.SiteName)})
				break
			}
			return fmt.Errorf("chapter %d (%s): %w", num, next, err)
		}
		words, err := a.saveChapter(ctx, b, num, ch, opts)
		if err != nil {
			return err
		}
		b.Chapters = append(b.Chapters, library.Chapter{Number: num, Title: ch.Title, URL: next, State: library.StateOK, Words: words})
		if b.Vars == nil {
			b.Vars = map[string]string{}
		}
		b.Vars["crawlNext"] = ch.NextURL
		next = ch.NextURL
		count++
		if count%10 == 0 {
			_ = a.Store.Save(b)
		}
		elapsed := math.Max(time.Since(start).Seconds(), 0.1)
		emit("progress", Progress{
			Stage: "scraping", Current: num, Total: max(num, limit), Scraped: count,
			Speed: fmt.Sprintf("%.1f ch/s", float64(count)/elapsed), Eta: "following links…",
			Percentage: pct(num, limit), ChapterNum: num, ChapterTitle: ch.Title, Status: "ok", Slug: b.Slug, Title: b.Title,
		})
	}
	return nil
}

func pct(n, limit int) int {
	if limit <= 0 {
		return 50
	}
	return int(math.Min(100, float64(n)/float64(limit)*100))
}

func (a *Archiver) downloadOne(ctx context.Context, b *library.Book, novel *source.Novel, c library.Chapter, opts Options) (state string, words int, title, errMsg string) {
	ref := source.ChapterRef{Number: c.Number, Title: c.Title, URL: c.URL, Index: c.Index, Locked: c.Locked}
	ch, err := a.Engine.FetchChapter(ctx, novel, ref)
	if err != nil {
		if errors.Is(err, source.ErrLocked) {
			return library.StateLocked, 0, "", err.Error()
		}
		return library.StateFailed, 0, "", err.Error()
	}
	words, err = a.saveChapter(ctx, b, c.Number, ch, opts)
	if err != nil {
		return library.StateFailed, 0, "", err.Error()
	}
	return library.StateOK, words, ch.Title, ""
}

// saveChapter cleans, downloads images (rewriting their links to local
// files) and writes the fragment.
func (a *Archiver) saveChapter(ctx context.Context, b *library.Book, num int, ch *source.Chapter, opts Options) (int, error) {
	res := clean.Chapter(ch.HTML, ch.Title, ch.BaseURL)
	html := res.HTML
	if !opts.SkipImages {
		for _, u := range res.ImageURLs {
			local, err := a.saveImage(ctx, b, u)
			if err != nil {
				continue // keep remote URL; text still works offline
			}
			html = strings.ReplaceAll(html, `src="`+htmlAttrEscape(u)+`"`, `src="img/`+local+`"`)
		}
	}
	header := "<h1 class=\"ch-title\">" + htmlEscape(ch.Title) + "</h1>\n"
	return res.Words, a.Store.WriteChapter(b.Slug, num, header+html)
}

// GetFile downloads a cover or image: plain HTTP first, then the site
// browser when the site refuses plain requests (e.g. behind Cloudflare).
func (a *Archiver) GetFile(ctx context.Context, u, referer string) (*fetch.Page, error) {
	var lastErr error
	if a.Files != nil {
		page, err := a.Files.Get(ctx, u, referer)
		if err == nil && looksLikeImage(page) {
			return page, nil
		}
		lastErr = err
	}
	if a.Engine != nil && a.Engine.Browser != nil {
		page, err := a.Engine.Browser.Get(ctx, u, referer)
		if err == nil && looksLikeImage(page) {
			return page, nil
		}
		if err != nil {
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no image at %s", u)
	}
	return nil, lastErr
}

// looksLikeImage rejects HTML error/challenge pages served instead of an image.
func looksLikeImage(p *fetch.Page) bool {
	if p == nil || len(p.Body) < 64 {
		return false
	}
	ct := strings.ToLower(p.Header.Get("Content-Type"))
	if strings.HasPrefix(ct, "image/") {
		return true
	}
	head := strings.ToLower(string(p.Body[:min(len(p.Body), 256)]))
	return !strings.Contains(head, "<html") && !strings.Contains(head, "<!doctype")
}

func (a *Archiver) saveImage(ctx context.Context, b *library.Book, u string) (string, error) {
	page, err := a.GetFile(ctx, u, b.URL)
	if err != nil {
		return "", err
	}
	name := library.ImageName(u, page.Header.Get("Content-Type"))
	return name, a.Store.WriteFile(b.Slug, "img/"+name, page.Body)
}

// saveCover downloads the site's poster. The saved one is replaced only
// when the new download worked and differs; it reports whether it changed.
func (a *Archiver) saveCover(ctx context.Context, b *library.Book) bool {
	if b.CoverURL == "" {
		return false
	}
	page, err := a.GetFile(ctx, b.CoverURL, b.URL)
	if err != nil || len(page.Body) < 200 {
		return false
	}
	if b.CoverFile != "" {
		if p, err := a.Store.FilePath(b.Slug, b.CoverFile); err == nil {
			if old, err := os.ReadFile(p); err == nil && bytes.Equal(old, page.Body) {
				return false
			}
		}
	}
	name := "cover" + path.Ext(library.ImageName(b.CoverURL, page.Header.Get("Content-Type")))
	if err := a.Store.WriteFile(b.Slug, name, page.Body); err != nil {
		return false
	}
	if b.CoverFile != "" && b.CoverFile != name {
		if p, err := a.Store.FilePath(b.Slug, b.CoverFile); err == nil {
			_ = os.Remove(p)
		}
	}
	b.CoverFile = name
	return true
}

func htmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;").Replace(s)
}

// htmlAttrEscape mirrors how bluemonday serialises '&' in attribute values.
func htmlAttrEscape(s string) string { return strings.ReplaceAll(s, "&", "&amp;") }

// revise re-reads saved chapters and replaces the ones whose text changed
// on the site. Unchanged chapters are left untouched (images included).
func (a *Archiver) revise(ctx context.Context, b *library.Book, novel *source.Novel, opts Options, emit Emit) error {
	var idx []int
	for i, c := range b.Chapters {
		if c.State != library.StateOK || c.Removed {
			continue
		}
		if b.TOCMode == "crawl" && c.URL == "" {
			continue // crawled chapters can only be re-read by their link
		}
		idx = append(idx, i)
	}
	if opts.Revise > 0 && len(idx) > opts.Revise {
		idx = idx[len(idx)-opts.Revise:]
	}
	if len(idx) == 0 {
		return nil
	}
	workers := opts.Concurrency
	if workers <= 0 {
		workers = 4
	}
	if workers > 8 {
		workers = 8
	}
	var (
		mu      sync.Mutex
		done    int64
		revised []int
		start   = time.Now()
		queue   = make(chan int)
		wg      sync.WaitGroup
		today   = time.Now().Format("2006-01-02")
		total   = len(idx)
	)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range queue {
				if opts.Gate != nil && opts.Gate(ctx) != nil {
					return
				}
				mu.Lock()
				c := b.Chapters[i]
				mu.Unlock()
				changed := a.reviseOne(ctx, b, novel, c, opts)
				if ctx.Err() != nil {
					return
				}
				mu.Lock()
				if changed {
					b.Chapters[i].Revised = today
					revised = append(revised, c.Number)
				}
				nrev := len(revised)
				mu.Unlock()
				n := atomic.AddInt64(&done, 1)
				elapsed := math.Max(time.Since(start).Seconds(), 0.1)
				speed := float64(n) / elapsed
				eta := time.Duration(float64(total-int(n))/math.Max(speed, 0.01)) * time.Second
				emit("progress", Progress{
					Stage: "revising", Current: int(n), Total: total, Scraped: nrev,
					Speed: fmt.Sprintf("%.1f ch/s", speed), Eta: eta.Round(time.Second).String(),
					Percentage: int(math.Round(float64(n) / float64(total) * 100)),
					ChapterNum: c.Number, ChapterTitle: c.Title, Slug: b.Slug, Title: b.Title,
				})
			}
		}()
	}
feed:
	for _, i := range idx {
		select {
		case queue <- i:
		case <-ctx.Done():
			break feed
		}
	}
	close(queue)
	wg.Wait()
	sort.Ints(revised)
	if opts.Result != nil {
		opts.Result.Revised = revised
		opts.Result.Checked = int(atomic.LoadInt64(&done))
	}
	return ctx.Err()
}

// reviseOne fetches one saved chapter again and rewrites it if the cleaned
// text differs from the saved copy. Images are only downloaded when the
// chapter actually changed.
func (a *Archiver) reviseOne(ctx context.Context, b *library.Book, novel *source.Novel, c library.Chapter, opts Options) bool {
	ref := source.ChapterRef{Number: c.Number, Title: c.Title, URL: c.URL, Index: c.Index, Locked: c.Locked}
	ch, err := a.Engine.FetchChapter(ctx, novel, ref)
	if err != nil {
		return false
	}
	old, err := a.Store.ReadChapter(b.Slug, c.Number)
	if err != nil {
		return false
	}
	res := clean.Chapter(ch.HTML, ch.Title, ch.BaseURL)
	fresh := "<h1 class=\"ch-title\">" + htmlEscape(ch.Title) + "</h1>\n" + res.HTML
	if TextOf(old) == TextOf(fresh) {
		return false
	}
	if _, err := a.saveChapter(ctx, b, c.Number, ch, opts); err != nil {
		return false
	}
	return true
}

// TextOf reduces a fragment to its visible words, so image paths (local
// vs remote) and markup details don't count as a revision.
func TextOf(html string) string {
	var out strings.Builder
	in := false
	for _, r := range html {
		switch {
		case r == '<':
			in = true
			out.WriteByte(' ')
		case r == '>':
			in = false
		case !in:
			out.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(out.String()), " ")
}
