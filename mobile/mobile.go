// Package mobile is the gomobile entry point for the Android app. It wraps
// the same internal/service used by the desktop app, so phones and PCs run
// identical discovery, cleaning, storage and download code.
//
// Build: gomobile bind -target=android -androidapi 24 -javapkg com.novelarchiver.core -o android/app/libs/core.aar ./mobile
package mobile

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"time"

	"novelarchiver/internal/browser"
	"novelarchiver/internal/manga"
	"novelarchiver/internal/reader"
	"novelarchiver/internal/service"
)

// Events receives UI events as (name, JSON payload); Java forwards them to
// the WebView's event bus.
type Events interface {
	Emit(event string, payloadJSON string)
}

// BrowserDriver is implemented in Java on top of an Android WebView.
// See internal/browser.Driver for the contract.
type BrowserDriver interface {
	Load(url string) (string, error)
	Current() string
	Fetch(url string) (string, error)
	Capture() (string, error)
	Show(visible bool)
}

// Core is one app instance: the novel service and, beside it, the manga
// service (nil if its library couldn't open).
type Core struct {
	svc        *service.Service
	reader     *reader.Handler
	manga      *manga.Service
	mangaFiles *manga.Handler
	unmetered  func() bool // set by SetNetwork
}

// New opens (or creates) the library in libraryDir. driver may be nil.
func New(libraryDir string, events Events, driver BrowserDriver) (*Core, error) {
	svc := service.New(libraryDir)
	emit := func(event string, data any) {
		if events == nil {
			return
		}
		b, err := json.Marshal(data)
		if err != nil {
			b = []byte(`{}`)
		}
		events.Emit(event, string(b))
	}
	svc.Emit = emit
	if svc.Store() == nil {
		return nil, errors.New("could not open the library folder")
	}
	// One site window for novels and manga: a check passed once counts for both.
	var site *browser.Driven
	if driver != nil {
		site = browser.NewDriven(driver, browser.Options{
			OnStatus: func(msg string) { emit("status", map[string]any{"message": msg}) },
			OnChallenge: func(host string) {
				emit("challenge", map[string]any{"host": host, "message": service.ChallengeMessage})
			},
		})
		svc.SetBrowser(site)
	}
	c := &Core{svc: svc, reader: &reader.Handler{Store: svc.Store(), Translate: svc.TranslateChapter, Paused: svc.TranslatePause}}

	mopts := manga.ServiceOptions{
		NoBrowser: site == nil,
		Emit:      func(ev string, data any) { emit("manga:"+ev, data) },
		// Manga follows the same "Only on Wi-Fi" setting as novels.
		MayDownload: func() bool {
			return !svc.GetSettings().WifiOnly || c.unmetered == nil || c.unmetered()
		},
	}
	if site != nil {
		mopts.Browser = site
	}
	if m, err := manga.NewService(mangaRoot(svc.LibraryRoot()), mopts); err == nil {
		c.manga = m
		c.mangaFiles = &manga.Handler{Store: m.Store()}
	}

	if svc.GetSettings().UpdateOnOpen {
		go func() {
			time.Sleep(3 * time.Second)
			_, _ = svc.UpdateAll(true)
		}()
	}
	if c.manga != nil && c.manga.GetSettings().CheckOnOpen {
		go func() {
			time.Sleep(4 * time.Second)
			c.manga.CheckAll(true)
		}()
	}
	return c, nil
}

// Network is implemented in Java: Unmetered is true on Wi-Fi/Ethernet.
type Network interface {
	Unmetered() bool
}

// SetNetwork lets "only on Wi-Fi" pause downloads on mobile data.
func (c *Core) SetNetwork(n Network) {
	if n == nil {
		c.svc.NetworkOK = nil
		c.unmetered = nil
		return
	}
	c.svc.NetworkOK = n.Unmetered
	c.unmetered = n.Unmetered
}

// NotifyEnabled says whether the user wants system notifications.
func (c *Core) NotifyEnabled() bool { return c.svc.GetSettings().Notify }

// Backup writes the whole library to a zip in dir (the Android shell then
// copies it to Downloads) and returns the BackupInfo as JSON.
func (c *Core) Backup(dir string) (string, error) {
	res, err := c.svc.Backup(filepath.Join(dir, service.BackupFileName()))
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(res)
	return string(b), err
}

// Call invokes a service method by name with a JSON array of arguments and
// returns the JSON result. The method names match the desktop bindings
// (window.go.main.App.*), so the shared frontend works unchanged.
func (c *Core) Call(method string, argsJSON string) (string, error) {
	var args []json.RawMessage
	if strings.TrimSpace(argsJSON) != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("bad arguments for %s: %w", method, err)
		}
	}
	str := func(i int) string {
		var s string
		if i < len(args) {
			_ = json.Unmarshal(args[i], &s)
		}
		return s
	}
	num := func(i int) int {
		var n int
		if i < len(args) {
			_ = json.Unmarshal(args[i], &n)
		}
		return n
	}
	if out, err, ok := c.callManga(method, args); ok {
		if err != nil {
			return "", err
		}
		b, err := json.Marshal(out)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	var out any
	var err error
	switch method {
	case "NovelSites":
		out = c.svc.Sites()
	case "GetNovels":
		out, err = c.svc.GetNovels()
	case "GetNovel":
		out, err = c.svc.GetNovel(str(0))
	case "GetSystemStats":
		out, err = c.svc.GetSystemStats()
	case "DiscoverNovel":
		out, err = c.svc.DiscoverNovel(str(0))
	case "StartScrape":
		var o service.ScrapeOptions
		if len(args) > 0 {
			if err := json.Unmarshal(args[0], &o); err != nil {
				return "", err
			}
		}
		out, err = c.svc.StartScrape(o)
	case "RetryNovel":
		out, err = c.svc.RetryNovel(str(0))
	case "CleanNovel":
		out, err = c.svc.CleanNovel(str(0))
	case "AuditNovel":
		out, err = c.svc.AuditNovel(str(0))
	case "DeleteNovel":
		out, err = c.svc.DeleteNovel(str(0))
	case "DeleteImages":
		out, err = c.svc.DeleteImages(str(0))
	case "DeleteChapters":
		out, err = c.svc.DeleteChapters(str(0), num(1), num(2))
	case "CancelTask":
		err = c.svc.CancelTask()
	case "OpenSiteBrowser":
		err = c.svc.OpenSiteBrowser(str(0))
	case "SearchNovels":
		out, err = c.svc.SearchNovels(str(0), str(1))
	case "CountChapters":
		out, err = c.svc.CountChapters(str(0))
	case "GetActivity":
		out = c.svc.GetActivity()
	case "GetSettings":
		out = c.svc.GetSettings()
	case "SaveSettings":
		var st service.Settings
		if len(args) > 0 {
			if err := json.Unmarshal(args[0], &st); err != nil {
				return "", err
			}
		}
		out, err = c.svc.SaveSettings(st)
	case "UpdateNovel":
		out, err = c.svc.UpdateNovel(str(0))
	case "RetryMissing":
		out, err = c.svc.RetryMissing(str(0))
	case "TranslateNovel":
		out, err = c.svc.TranslateNovel(str(0))
	case "CatchUp":
		out, err = c.svc.CatchUp()
	case "RedownloadNovel":
		out, err = c.svc.RedownloadNovel(str(0))
	case "UpdateAll":
		var auto bool
		if len(args) > 0 {
			_ = json.Unmarshal(args[0], &auto)
		}
		out, err = c.svc.UpdateAll(auto)
	case "RemoveQueued":
		err = c.svc.RemoveQueued(num(0))
	case "CancelAll":
		err = c.svc.CancelAll()
	case "SetReading":
		err = c.svc.SetReading(str(0), num(1))
	case "InspectBackup":
		out, err = c.svc.InspectBackup(str(0))
	case "RestoreBackup":
		out, err = c.svc.Restore(str(0))
	case "Platform":
		out = "android"
	case "ShowNovelFolder", "ShowFileInFolder", "OpenLibraryFolder":
		// No file manager to open on Android; the UI hides these.
	default:
		return "", fmt.Errorf("unknown method %q", method)
	}
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Response is a reader page or library file.
type Response struct {
	Status   int
	MimeType string
	Body     []byte
}

// Serve renders /library/... paths (contents page, chapters, cover, images)
// and /mangalib/... (manga covers and pages) for the WebView's request
// interceptor.
func (c *Core) Serve(path string) *Response {
	if r, ok := c.serveManga(path); ok {
		return r
	}
	rec := httptest.NewRecorder()
	c.reader.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	mime := rec.Header().Get("Content-Type")
	if i := strings.Index(mime, ";"); i >= 0 {
		mime = mime[:i]
	}
	if mime == "" {
		mime = "application/octet-stream"
	}
	return &Response{Status: rec.Code, MimeType: mime, Body: rec.Body.Bytes()}
}

// ExportEPUB writes a novel's downloaded chapters as an EPUB into dir and
// returns {"path","fileName","chapters","images","sizeMb"} as JSON. The
// Android shell then copies it to the phone's Downloads folder.
func (c *Core) ExportEPUB(slug string, dir string) (string, error) {
	res, err := c.svc.ExportEPUB(slug, filepath.Join(dir, c.svc.EPUBFileName(slug)), 0, 0)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(res)
	return string(b), err
}

// LibraryPath is the absolute library folder.
func (c *Core) LibraryPath() string { return c.svc.LibraryRoot() }

// Shutdown stops the running downloads (manga first: it shares the site
// window the novel side closes).
func (c *Core) Shutdown() {
	if c.manga != nil {
		c.manga.Shutdown()
	}
	c.svc.Shutdown()
}
