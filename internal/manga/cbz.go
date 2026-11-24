package manga

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ExportResult describes a written CBZ.
type ExportResult struct {
	Path     string `json:"path"`
	Chapters int    `json:"chapters"`
	Pages    int    `json:"pages"`
	Bytes    int64  `json:"bytes"`
}

type comicInfo struct {
	XMLName   xml.Name `xml:"ComicInfo"`
	Title     string   `xml:"Title,omitempty"`
	Series    string   `xml:"Series"`
	Summary   string   `xml:"Summary,omitempty"`
	Writer    string   `xml:"Writer,omitempty"`
	Web       string   `xml:"Web,omitempty"`
	PageCount int      `xml:"PageCount"`
	Manga     string   `xml:"Manga,omitempty"`
	Notes     string   `xml:"Notes,omitempty"`
}

var fileBad = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]+`)

// FileName is a safe file name for a series' CBZ.
func FileName(title string) string {
	t := strings.TrimSpace(fileBad.ReplaceAllString(title, " "))
	t = strings.Trim(strings.Join(strings.Fields(t), " "), ". ")
	if r := []rune(t); len(r) > 80 {
		t = string(r[:80])
	}
	if t == "" {
		t = "manga"
	}
	return t + ".cbz"
}

// WriteCBZ writes the downloaded chapters from..to (0 = all) of a series
// into one CBZ: pages are named c0001-p001.jpg so every comic reader keeps
// them in order, and ComicInfo.xml carries the title and description.
func WriteCBZ(st *Store, b *Book, dest string, from, to int) (*ExportResult, error) {
	tmp := dest + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			f.Close()
			os.Remove(tmp)
		}
	}()
	zw := zip.NewWriter(f)
	res := &ExportResult{Path: dest}
	for _, c := range b.Chapters {
		if c.State != StateOK || (from > 0 && c.Number < from) || (to > 0 && c.Number > to) {
			continue
		}
		files, err := st.PageFiles(b.Slug, c.Key)
		if err != nil || len(files) == 0 {
			continue
		}
		res.Chapters++
		for i, name := range files {
			src := filepath.Join(st.ChapterDir(b.Slug, c.Key), name)
			w, err := zw.CreateHeader(&zip.FileHeader{
				Name:   fmt.Sprintf("c%04d-p%03d%s", c.Number, i+1, strings.ToLower(filepath.Ext(name))),
				Method: zip.Store, // pictures are already compressed
			})
			if err != nil {
				return nil, err
			}
			in, err := os.Open(src)
			if err != nil {
				return nil, err
			}
			n, err := io.Copy(w, in)
			in.Close()
			if err != nil {
				return nil, err
			}
			res.Pages++
			res.Bytes += n
		}
	}
	if res.Chapters == 0 {
		return nil, fmt.Errorf("no downloaded chapters to export yet")
	}
	ci := comicInfo{Series: b.Title, Title: b.Title, Summary: b.Description, Writer: b.Author, Web: b.URL,
		PageCount: res.Pages, Notes: "Saved with InkShelfs from " + b.SiteName}
	if strings.Contains(b.Type, "Manga") && !strings.Contains(b.Type, "Manhwa") {
		ci.Manga = "YesAndRightToLeft"
	}
	if w, err := zw.Create("ComicInfo.xml"); err == nil {
		io.WriteString(w, xml.Header)
		enc := xml.NewEncoder(w)
		enc.Indent("", "  ")
		_ = enc.Encode(ci)
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, dest); err != nil {
		return nil, err
	}
	ok = true
	if fi, err := os.Stat(dest); err == nil {
		res.Bytes = fi.Size()
	}
	return res, nil
}
