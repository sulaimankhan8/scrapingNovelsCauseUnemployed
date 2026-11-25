package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"novelarchiver/internal/manga"
)

// MangaApp is the manga half of InkShelfs, bound as window.go.main.MangaApp.
// Its library stays in %AppData%\MangaArchiver; its events reach the page as
// "manga:<event>" so they never mix with the novel events.
type MangaApp struct {
	svc *manga.Service
	ctx context.Context
}

// newMangaApp opens the manga library, sharing the novel side's site window
// (one "verify you are human" session for both).
func newMangaApp(win manga.SiteWindow) *MangaApp {
	m := &MangaApp{}
	opts := manga.ServiceOptions{
		Browser: win,
		Emit: func(ev string, data any) {
			if m.ctx != nil {
				runtime.EventsEmit(m.ctx, "manga:"+ev, data)
			}
		},
	}
	if win == nil {
		opts.NoBrowser = true
	}
	svc, err := manga.NewService(manga.DefaultRoot(), opts)
	if err != nil {
		println("manga library unavailable:", err.Error())
		return m
	}
	m.svc = svc
	return m
}

var errNoManga = errors.New("the manga library couldn't be opened")

func (m *MangaApp) ok() error {
	if m.svc == nil {
		return errNoManga
	}
	return nil
}

func (m *MangaApp) startup(ctx context.Context) {
	m.ctx = ctx
	if m.svc != nil && m.svc.GetSettings().CheckOnOpen {
		go m.svc.CheckAll(true)
	}
}

func (m *MangaApp) shutdown() {
	if m.svc != nil {
		m.svc.Shutdown()
	}
}

// Store is the manga library (for the file handler); nil if unavailable.
func (m *MangaApp) store() *manga.Store {
	if m.svc == nil {
		return nil
	}
	return m.svc.Store()
}

// ---- library ----

func (m *MangaApp) Available() bool { return m.svc != nil }
func (m *MangaApp) Recognises(url string) bool {
	return m.svc != nil && m.svc.Recognises(url)
}
func (m *MangaApp) Sites() []manga.SiteInfo {
	if m.svc == nil {
		return []manga.SiteInfo{}
	}
	return m.svc.Sites()
}
func (m *MangaApp) Library() ([]manga.LibraryItem, error) {
	if err := m.ok(); err != nil {
		return nil, err
	}
	return m.svc.Library()
}
func (m *MangaApp) Series(slug string) (*manga.SeriesDetail, error) {
	if err := m.ok(); err != nil {
		return nil, err
	}
	return m.svc.Series(slug)
}
func (m *MangaApp) Preview(url string) (*manga.SeriesPreview, error) {
	if err := m.ok(); err != nil {
		return nil, err
	}
	return m.svc.Preview(url)
}
func (m *MangaApp) CoverPreview(url string) string {
	if m.svc == nil {
		return ""
	}
	return m.svc.CoverPreview(url)
}
func (m *MangaApp) Add(url string, from, to int) (string, error) {
	if err := m.ok(); err != nil {
		return "", err
	}
	return m.svc.AddByURL(url, from, to)
}
func (m *MangaApp) Download(slug string, from, to int) error {
	if err := m.ok(); err != nil {
		return err
	}
	return m.svc.Download(slug, from, to)
}
func (m *MangaApp) DownloadKeys(slug string, keys []int) error {
	if err := m.ok(); err != nil {
		return err
	}
	return m.svc.DownloadKeys(slug, keys)
}
func (m *MangaApp) RetryFailed(slug string) error {
	if err := m.ok(); err != nil {
		return err
	}
	return m.svc.RetryFailed(slug)
}
func (m *MangaApp) CheckUpdates(slug string) error {
	if err := m.ok(); err != nil {
		return err
	}
	return m.svc.CheckUpdates(slug)
}

// CheckAll queues a new-chapter check for every series. auto=true keeps
// quiet about series with nothing new.
func (m *MangaApp) CheckAll(auto bool) int {
	if m.svc == nil {
		return 0
	}
	return m.svc.CheckAll(auto)
}
func (m *MangaApp) DeleteSeries(slug string) error {
	if err := m.ok(); err != nil {
		return err
	}
	return m.svc.DeleteSeries(slug)
}
func (m *MangaApp) FreeKeys(slug string, keys []int) (int64, error) {
	if err := m.ok(); err != nil {
		return 0, err
	}
	return m.svc.FreeKeys(slug, keys)
}

// ---- reader ----

func (m *MangaApp) Chapter(slug string, key int) (*manga.ChapterView, error) {
	if err := m.ok(); err != nil {
		return nil, err
	}
	return m.svc.Chapter(slug, key)
}
func (m *MangaApp) SetReading(slug string, key, page int) error {
	if err := m.ok(); err != nil {
		return err
	}
	return m.svc.SetReading(slug, key, page)
}
func (m *MangaApp) SetReadMode(slug, mode string) error {
	if err := m.ok(); err != nil {
		return err
	}
	return m.svc.SetReadMode(slug, mode)
}

// ---- queue and settings ----

func (m *MangaApp) Activity() manga.Activity {
	if m.svc == nil {
		return manga.Activity{Queue: []manga.Job{}}
	}
	return m.svc.Activity()
}
func (m *MangaApp) RemoveQueued(id int) {
	if m.svc != nil {
		m.svc.RemoveQueued(id)
	}
}
// CancelJob stops one manga download (or drops it from the queue).
func (m *MangaApp) CancelJob(id int) {
	if m.svc != nil {
		m.svc.CancelJob(id)
	}
}

func (m *MangaApp) CancelCurrent() {
	if m.svc != nil {
		m.svc.CancelCurrent()
	}
}
func (m *MangaApp) CancelAll() {
	if m.svc != nil {
		m.svc.CancelAll()
	}
}
func (m *MangaApp) GetSettings() manga.Settings {
	if m.svc == nil {
		return manga.Settings{}
	}
	return m.svc.GetSettings()
}
func (m *MangaApp) SaveSettings(s manga.Settings) (manga.Settings, error) {
	if err := m.ok(); err != nil {
		return s, err
	}
	return m.svc.SaveSettings(s)
}
func (m *MangaApp) OpenSiteBrowser(url string) error {
	if err := m.ok(); err != nil {
		return err
	}
	return m.svc.OpenSiteBrowser(url)
}
func (m *MangaApp) LibraryFolder() string {
	if m.svc == nil {
		return ""
	}
	return m.svc.LibraryRoot()
}

// ---- desktop helpers ----

// ExportCBZ asks where to save, then writes the series as one CBZ file.
// Returns nil when the dialog is cancelled.
func (m *MangaApp) ExportCBZ(slug string) (*manga.ExportResult, error) {
	if err := m.ok(); err != nil {
		return nil, err
	}
	if m.ctx == nil {
		return nil, errors.New("app not ready")
	}
	d, err := m.svc.Series(slug)
	if err != nil {
		return nil, err
	}
	home, _ := os.UserHomeDir()
	dest, err := runtime.SaveFileDialog(m.ctx, runtime.SaveDialogOptions{
		Title:            "Save as CBZ",
		DefaultDirectory: filepath.Join(home, "Downloads"),
		DefaultFilename:  manga.FileName(d.Title),
		Filters:          []runtime.FileFilter{{DisplayName: "Comic book archive (*.cbz)", Pattern: "*.cbz"}},
	})
	if err != nil || dest == "" {
		return nil, err
	}
	if filepath.Ext(dest) == "" {
		dest += ".cbz"
	}
	return m.svc.ExportCBZ(slug, dest)
}

// OpenFolder shows a series folder (or the manga library) in Explorer.
func (m *MangaApp) OpenFolder(slug string) {
	if m.ctx == nil || m.svc == nil {
		return
	}
	dir := m.svc.LibraryRoot()
	if slug != "" {
		if strings.ContainsAny(slug, `/\.`) {
			return
		}
		dir = m.svc.SeriesDir(slug)
	}
	runtime.BrowserOpenURL(m.ctx, "file:///"+filepath.ToSlash(dir))
}
