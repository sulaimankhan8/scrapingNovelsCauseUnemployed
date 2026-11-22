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

	"novelarchiver/internal/browser"
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

// Core is one app instance.
type Core struct {
	svc    *service.Service
	reader *reader.Handler
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
	if driver != nil {
		svc.SetBrowser(browser.NewDriven(driver, browser.Options{
			OnStatus: func(msg string) { emit("status", map[string]any{"message": msg}) },
			OnChallenge: func(host string) {
				emit("challenge", map[string]any{"host": host, "message": service.ChallengeMessage})
			},
		}))
	}
	return &Core{svc: svc, reader: &reader.Handler{Store: svc.Store()}}, nil
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
	var out any
	var err error
	switch method {
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
// for the WebView's request interceptor.
func (c *Core) Serve(path string) *Response {
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

// Shutdown stops the running download.
func (c *Core) Shutdown() { c.svc.Shutdown() }
