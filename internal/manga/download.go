package manga

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"novelarchiver/internal/fetch"
)

// ErrNotImage means a page link returned something that isn't a picture
// (an error page, a bot check).
var ErrNotImage = errors.New("the site sent something that isn't an image")

// imageType detects an image from its first bytes and returns its extension.
func imageType(b []byte) string {
	switch {
	case len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return ".jpg"
	case bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")):
		return ".png"
	case len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		return ".webp"
	case bytes.HasPrefix(b, []byte("GIF87a")), bytes.HasPrefix(b, []byte("GIF89a")):
		return ".gif"
	case len(b) >= 12 && string(b[4:8]) == "ftyp" && (string(b[8:12]) == "avif" || string(b[8:12]) == "avis"):
		return ".avif"
	}
	return ""
}

// FetchImage downloads one image and checks it really is one. Browser
// sites fall back to the site browser when the CDN refuses plain requests.
func (e *Engine) FetchImage(ctx context.Context, p *Profile, link, referer string) ([]byte, string, error) {
	var hdr map[string]string
	if p != nil && p.API == "mangadex" {
		hdr = p.Headers
	}
	pg, err := e.Images.Do(ctx, Request{URL: link, Referer: referer, Image: true, Header: hdr})
	if err != nil && e.Browser != nil && p != nil && p.IsBrowser() {
		var he *fetch.HTTPError
		if errors.Is(err, fetch.ErrBlocked) || (errors.As(err, &he) && he.Status == 403) {
			pg, err = e.Browser.Get(ctx, link, referer)
		}
	}
	if err != nil {
		return nil, "", err
	}
	ext := imageType(pg.Body)
	if ext == "" {
		return nil, "", fmt.Errorf("%w (%s)", ErrNotImage, shortURL(link))
	}
	return pg.Body, ext, nil
}

// ChapterResult is what DownloadChapter saved.
type ChapterResult struct {
	Pages int
	Bytes int64
}

// DownloadChapter saves every page of a chapter into dir. Pages go to a
// ".part" folder first, so a half-finished chapter never looks complete.
// progress(done, total) is called after each page.
func (e *Engine) DownloadChapter(ctx context.Context, b *Book, ch Chapter, dir string, workers int, progress func(done, total int)) (*ChapterResult, error) {
	p := e.Sites.ByID(b.SiteID)
	imgs, referer, err := e.Pages(ctx, b.SiteID, b.Vars, ChapterRef{Number: ch.Number, Title: ch.Title, URL: ch.URL, ID: ch.ID})
	if err != nil {
		return nil, err
	}
	part := dir + ".part"
	_ = os.RemoveAll(part)
	if err := os.MkdirAll(part, 0o755); err != nil {
		return nil, err
	}
	if workers <= 0 {
		workers = 4
	}
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
		done     atomic.Int32
		total    atomic.Int64
	)
	jobs := make(chan int)
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				data, ext, err := e.FetchImage(cctx, p, imgs[i], referer)
				if err == nil {
					err = os.WriteFile(filepath.Join(part, fmt.Sprintf("%03d%s", i+1, ext)), data, 0o644)
				}
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = fmt.Errorf("page %d of %d: %w", i+1, len(imgs), err)
					}
					mu.Unlock()
					cancel()
					continue
				}
				total.Add(int64(len(data)))
				if progress != nil {
					progress(int(done.Add(1)), len(imgs))
				}
			}
		}()
	}
feed:
	for i := range imgs {
		select {
		case jobs <- i:
		case <-cctx.Done():
			break feed
		}
	}
	close(jobs)
	wg.Wait()
	if firstErr == nil && ctx.Err() != nil {
		firstErr = ctx.Err()
	}
	if firstErr != nil {
		_ = os.RemoveAll(part)
		return nil, firstErr
	}
	_ = os.RemoveAll(dir)
	if err := os.Rename(part, dir); err != nil {
		return nil, err
	}
	return &ChapterResult{Pages: len(imgs), Bytes: total.Load()}, nil
}

// SaveCover downloads the cover next to series.json; failures are ignored
// (the library shows a placeholder).
func (e *Engine) SaveCover(ctx context.Context, b *Book, dir string) string {
	if b.CoverURL == "" {
		return ""
	}
	p := e.Sites.ByID(b.SiteID)
	c, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	data, ext, err := e.FetchImage(c, p, b.CoverURL, b.URL)
	if err != nil {
		return ""
	}
	name := "cover" + ext
	if os.WriteFile(filepath.Join(dir, name), data, 0o644) != nil {
		return ""
	}
	return name
}

func shortURL(u string) string {
	if len(u) > 90 {
		return u[:60] + "…" + u[len(u)-25:]
	}
	return strings.TrimSpace(u)
}
