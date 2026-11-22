package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"novelarchiver/internal/browser"
	"novelarchiver/internal/library"
	"novelarchiver/internal/service"
)

// App is bound to the frontend (window.go.main.App.*). All behaviour lives
// in internal/service, shared with the Android app; this adds the Wails
// event bus, the WebView2 site browser and desktop-only helpers.
type App struct {
	svc *service.Service
	ctx context.Context
}

// NewApp opens the library in %AppData% and attaches the site browser.
func NewApp() *App {
	a := &App{svc: service.New(library.DefaultRoot())}
	a.svc.Emit = func(event string, data any) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, event, data)
		}
	}
	if root := a.svc.LibraryRoot(); root != "" {
		// Cookies live next to the library so a passed human check is remembered.
		if win, err := browser.New(browser.Options{
			DataPath: filepath.Join(filepath.Dir(root), "SiteBrowser"),
			OnStatus: func(msg string) { a.svc.Emit("status", map[string]any{"message": msg}) },
			OnChallenge: func(host string) {
				a.svc.Emit("challenge", map[string]any{"host": host, "message": service.ChallengeMessage})
			},
		}); err == nil {
			a.svc.SetBrowser(win)
		}
	}
	return a
}

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

func (a *App) shutdown(context.Context) { a.svc.Shutdown() }

// OpenLibraryFolder shows the library folder in Explorer.
func (a *App) OpenLibraryFolder() {
	if a.ctx != nil && a.svc.LibraryRoot() != "" {
		runtime.BrowserOpenURL(a.ctx, "file:///"+filepath.ToSlash(a.svc.LibraryRoot()))
	}
}

// OpenExternalURL opens a link in the system browser.
func (a *App) OpenExternalURL(url string) {
	if a.ctx != nil && (strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://")) {
		runtime.BrowserOpenURL(a.ctx, url)
	}
}

// ---- Methods exposed to the frontend (delegating to the shared service) ----

func (a *App) GetNovels() ([]service.NovelItem, error)            { return a.svc.GetNovels() }
func (a *App) GetNovel(slug string) (*service.NovelDetail, error) { return a.svc.GetNovel(slug) }
func (a *App) GetSystemStats() (*service.SystemStats, error)      { return a.svc.GetSystemStats() }
func (a *App) DiscoverNovel(url string) (*service.DiscoverResult, error) {
	return a.svc.DiscoverNovel(url)
}
func (a *App) StartScrape(o service.ScrapeOptions) (map[string]any, error) {
	return a.svc.StartScrape(o)
}
func (a *App) RetryNovel(slug string) (map[string]any, error)   { return a.svc.RetryNovel(slug) }
func (a *App) CleanNovel(slug string) (map[string]any, error)   { return a.svc.CleanNovel(slug) }
func (a *App) AuditNovel(slug string) (map[string]any, error)   { return a.svc.AuditNovel(slug) }
func (a *App) DeleteNovel(slug string) (map[string]any, error)  { return a.svc.DeleteNovel(slug) }
func (a *App) DeleteImages(slug string) (map[string]any, error) { return a.svc.DeleteImages(slug) }
func (a *App) DeleteChapters(slug string, from, to int) (map[string]any, error) {
	return a.svc.DeleteChapters(slug, from, to)
}
func (a *App) CancelTask() error                { return a.svc.CancelTask() }
func (a *App) OpenSiteBrowser(url string) error { return a.svc.OpenSiteBrowser(url) }

// ExportEPUB asks where to save, then writes the novel as an EPUB.
// Returns nil (no error) if the user cancels the dialog.
func (a *App) ExportEPUB(slug string) (*service.ExportResult, error) {
	if a.ctx == nil {
		return nil, errors.New("app not ready")
	}
	home, _ := os.UserHomeDir()
	dest, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:            "Save EPUB",
		DefaultDirectory: filepath.Join(home, "Downloads"),
		DefaultFilename:  a.svc.EPUBFileName(slug),
		Filters:          []runtime.FileFilter{{DisplayName: "EPUB e-book (*.epub)", Pattern: "*.epub"}},
	})
	if err != nil || dest == "" {
		return nil, err
	}
	if filepath.Ext(dest) == "" {
		dest += ".epub"
	}
	return a.svc.ExportEPUB(slug, dest, 0, 0)
}
