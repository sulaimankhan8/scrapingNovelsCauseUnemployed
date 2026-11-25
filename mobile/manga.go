package mobile

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strings"

	"novelarchiver/internal/manga"
)

// The manga half of InkShelfs on Android. The page calls window.go.main.
// MangaApp.X, which android-shim.js sends here as "Manga.X"; the method
// names match the desktop bindings (manga_app.go), so manga.js runs
// unchanged. Its events reach the page as "manga:<event>".

// mangaRoot keeps the manga library beside the novel one:
// <files>/library (novels) and <files>/manga/library (manga).
func mangaRoot(novelLibrary string) string {
	return filepath.Join(filepath.Dir(novelLibrary), "manga", "library")
}

// callManga runs one "Manga.X" method. ok=false means the name isn't a
// manga method.
func (c *Core) callManga(method string, args []json.RawMessage) (out any, err error, ok bool) {
	name, found := strings.CutPrefix(method, "Manga.")
	if !found {
		return nil, nil, false
	}
	m := c.manga
	if m == nil {
		if name == "Available" {
			return false, nil, true
		}
		return nil, fmt.Errorf("the manga library couldn't be opened"), true
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
	ints := func(i int) []int {
		var v []int
		if i < len(args) {
			_ = json.Unmarshal(args[i], &v)
		}
		return v
	}
	flag := func(i int) bool {
		var v bool
		if i < len(args) {
			_ = json.Unmarshal(args[i], &v)
		}
		return v
	}
	switch name {
	case "Available":
		out = true
	case "Recognises":
		out = m.Recognises(str(0))
	case "Sites":
		out = m.Sites()
	case "Library":
		out, err = m.Library()
	case "Series":
		out, err = m.Series(str(0))
	case "Preview":
		out, err = m.Preview(str(0))
	case "CoverPreview":
		out = m.CoverPreview(str(0))
	case "Add":
		out, err = m.AddByURL(str(0), num(1), num(2))
	case "Download":
		err = m.Download(str(0), num(1), num(2))
	case "DownloadKeys":
		err = m.DownloadKeys(str(0), ints(1))
	case "RetryFailed":
		err = m.RetryFailed(str(0))
	case "CheckUpdates":
		err = m.CheckUpdates(str(0))
	case "CheckAll":
		out = m.CheckAll(flag(0))
	case "DeleteSeries":
		err = m.DeleteSeries(str(0))
	case "FreeKeys":
		out, err = m.FreeKeys(str(0), ints(1))
	case "Chapter":
		out, err = m.Chapter(str(0), num(1))
	case "SetReading":
		err = m.SetReading(str(0), num(1), num(2))
	case "SetReadMode":
		err = m.SetReadMode(str(0), str(1))
	case "Activity":
		out = m.Activity()
	case "RemoveQueued":
		m.RemoveQueued(num(0))
	case "CancelCurrent":
		m.CancelCurrent()
	case "CancelJob":
		m.CancelJob(num(0))
	case "CancelAll":
		m.CancelAll()
	case "GetSettings":
		out = m.GetSettings()
	case "SaveSettings":
		var st manga.Settings
		if len(args) > 0 {
			if err := json.Unmarshal(args[0], &st); err != nil {
				return nil, err, true
			}
		}
		out, err = m.SaveSettings(st)
	case "OpenSiteBrowser":
		err = m.OpenSiteBrowser(str(0))
	case "LibraryFolder":
		out = m.LibraryRoot()
	case "OpenFolder":
		// No file manager on Android; the page hides "Show folder".
	default:
		return nil, fmt.Errorf("unknown manga method %q", name), true
	}
	return out, err, true
}

// serveManga renders /mangalib/... (covers and page images).
func (c *Core) serveManga(path string) (*Response, bool) {
	if !strings.HasPrefix(path, manga.FilePrefix) {
		return nil, false
	}
	if c.mangaFiles == nil {
		return &Response{Status: 404, MimeType: "text/plain", Body: []byte("Not found")}, true
	}
	rec := httptest.NewRecorder()
	c.mangaFiles.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	mime := rec.Header().Get("Content-Type")
	if i := strings.Index(mime, ";"); i >= 0 {
		mime = mime[:i]
	}
	if mime == "" {
		mime = "application/octet-stream"
	}
	return &Response{Status: rec.Code, MimeType: mime, Body: rec.Body.Bytes()}, true
}

// ExportCBZ writes a manga series' downloaded chapters as one CBZ into dir
// and returns {"path","fileName","chapters","pages","bytes"} as JSON. The
// Android shell then copies it to the phone's Downloads folder.
func (c *Core) ExportCBZ(slug string, dir string) (string, error) {
	if c.manga == nil {
		return "", fmt.Errorf("the manga library couldn't be opened")
	}
	d, err := c.manga.Series(slug)
	if err != nil {
		return "", err
	}
	name := manga.FileName(d.Title)
	res, err := c.manga.ExportCBZ(slug, filepath.Join(dir, name))
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(map[string]any{"path": res.Path, "fileName": name, "chapters": res.Chapters, "pages": res.Pages, "bytes": res.Bytes})
	return string(b), err
}
