package manga

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Library layout:
//
//	<root>/<slug>/series.json        metadata, chapter list and states
//	<root>/<slug>/cover.<ext>
//	<root>/<slug>/ch/<key>/001.<ext> page images of one chapter
//
// A chapter's folder is named by its Key, which never changes, so new
// chapters inserted by the site later don't shuffle saved folders.

// Chapter states.
const (
	StatePending = ""
	StateOK      = "ok"
	StateFailed  = "failed"
)

// Chapter is one stored chapter.
type Chapter struct {
	Key     int       `json:"key"`
	Number  int       `json:"number"`
	Title   string    `json:"title"`
	URL     string    `json:"url"`
	ID      string    `json:"id,omitempty"`
	State   string    `json:"state,omitempty"`
	Error   string    `json:"error,omitempty"`
	Pages   int       `json:"pages,omitempty"`
	Bytes   int64     `json:"bytes,omitempty"`
	SavedAt time.Time `json:"savedAt,omitzero"`
	// Removed marks a saved chapter the site no longer lists; its pages are
	// kept, never deleted silently.
	Removed bool `json:"removed,omitempty"`
}

// Book is series.json.
type Book struct {
	Slug        string            `json:"slug"`
	SiteID      string            `json:"siteId"`
	SiteName    string            `json:"siteName"`
	Type        string            `json:"type"`
	URL         string            `json:"url"`
	Title       string            `json:"title"`
	Author      string            `json:"author"`
	Description string            `json:"description"`
	Status      string            `json:"status,omitempty"`
	CoverURL    string            `json:"coverUrl,omitempty"`
	CoverFile   string            `json:"coverFile,omitempty"`
	Vars        map[string]string `json:"vars,omitempty"`
	Chapters    []Chapter         `json:"chapters"`
	NextKey     int               `json:"nextKey"`
	AddedAt     time.Time         `json:"addedAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
	CheckedAt   time.Time         `json:"checkedAt,omitzero"`
	// Reading place and preferred reading mode for this series.
	ReadKey  int       `json:"readKey,omitempty"`
	ReadPage int       `json:"readPage,omitempty"`
	ReadAt   time.Time `json:"readAt,omitzero"`
	ReadMode string    `json:"readMode,omitempty"` // strip, rtl, ltr
	// NewSince counts chapters an update found that haven't been opened.
	NewChapters int `json:"newChapters,omitempty"`
}

// Counts summarises chapter states.
func (b *Book) Counts() (ok, failed, pending int, bytes int64) {
	for _, c := range b.Chapters {
		switch c.State {
		case StateOK:
			ok++
			bytes += c.Bytes
		case StateFailed:
			failed++
		default:
			pending++
		}
	}
	return
}

// ByKey finds a chapter.
func (b *Book) ByKey(key int) *Chapter {
	for i := range b.Chapters {
		if b.Chapters[i].Key == key {
			return &b.Chapters[i]
		}
	}
	return nil
}

// Store is the library folder.
type Store struct {
	Root string
	mu   sync.Mutex
}

// DefaultRoot is %AppData%\MangaArchiver\library (or MANGAARCHIVER_LIBRARY).
func DefaultRoot() string {
	if v := os.Getenv("MANGAARCHIVER_LIBRARY"); v != "" {
		return v
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		dir, _ = os.UserHomeDir()
	}
	return filepath.Join(dir, "MangaArchiver", "library")
}

// OpenStore creates the folder if needed.
func OpenStore(root string) (*Store, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, err
	}
	return &Store{Root: abs}, nil
}

var slugBad = regexp.MustCompile(`[^a-z0-9]+`)

func validSlug(slug string) bool {
	return slug != "" && !strings.ContainsAny(slug, `/\.:`) && slug == strings.TrimSpace(slug)
}

// Dir returns a series folder.
func (s *Store) Dir(slug string) string { return filepath.Join(s.Root, slug) }

// ChapterDir returns the folder of a chapter's pages.
func (s *Store) ChapterDir(slug string, key int) string {
	return filepath.Join(s.Root, slug, "ch", fmt.Sprintf("%05d", key))
}

// Load reads series.json.
func (s *Store) Load(slug string) (*Book, error) {
	if !validSlug(slug) {
		return nil, fmt.Errorf("bad series id")
	}
	data, err := os.ReadFile(filepath.Join(s.Dir(slug), "series.json"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("that series isn't in your library")
		}
		return nil, err
	}
	var b Book
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("series.json for %s is damaged: %w", slug, err)
	}
	b.Slug = slug
	return &b, nil
}

// Save writes series.json atomically.
func (s *Store) Save(b *Book) error {
	if !validSlug(b.Slug) {
		return fmt.Errorf("bad series id")
	}
	b.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	dir := s.Dir(b.Slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(dir, "series.json.tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, "series.json"))
}

// List returns every series, most recently read or added first.
func (s *Store) List() ([]*Book, error) {
	entries, err := os.ReadDir(s.Root)
	if err != nil {
		return nil, err
	}
	var out []*Book
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if b, err := s.Load(e.Name()); err == nil {
			out = append(out, b)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return lastTouch(out[i]).After(lastTouch(out[j])) })
	return out, nil
}

func lastTouch(b *Book) time.Time {
	if b.ReadAt.After(b.AddedAt) {
		return b.ReadAt
	}
	return b.AddedAt
}

// FindByURL returns the slug of a stored series with this link, or "".
func (s *Store) FindByURL(u string) string {
	books, _ := s.List()
	for _, b := range books {
		if sameURL(b.URL, u) {
			return b.Slug
		}
	}
	return ""
}

// NewSlug makes a unique folder name from a title.
func (s *Store) NewSlug(title string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	base := strings.Trim(slugBad.ReplaceAllString(strings.ToLower(title), "-"), "-")
	if r := []rune(base); len(r) > 60 {
		base = strings.Trim(string(r[:60]), "-")
	}
	if base == "" {
		base = "series"
	}
	slug := base
	for i := 2; ; i++ {
		if _, err := os.Stat(s.Dir(slug)); errors.Is(err, fs.ErrNotExist) {
			_ = os.MkdirAll(s.Dir(slug), 0o755) // reserve it
			return slug
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

// Delete removes a series folder.
func (s *Store) Delete(slug string) error {
	if !validSlug(slug) {
		return fmt.Errorf("bad series id")
	}
	return os.RemoveAll(s.Dir(slug))
}

// PageFiles lists a chapter's saved page files in order.
func (s *Store) PageFiles(slug string, key int) ([]string, error) {
	entries, err := os.ReadDir(s.ChapterDir(slug, key))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && !strings.HasSuffix(e.Name(), ".part") && imageExt(e.Name()) {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

func imageExt(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".avif":
		return true
	}
	return false
}

// Size is the bytes on disk of one series.
func (s *Store) Size(slug string) int64 {
	var n int64
	_ = filepath.WalkDir(s.Dir(slug), func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if fi, err := d.Info(); err == nil {
				n += fi.Size()
			}
		}
		return nil
	})
	return n
}
