// Package library stores downloaded novels on disk:
//
//	<root>/<slug>/book.json          metadata + chapter list + per-chapter state
//	<root>/<slug>/cover.<ext>        downloaded cover
//	<root>/<slug>/chapters/0001.html sanitised chapter fragment (no CSS)
//	<root>/<slug>/img/<hash>.<ext>   images used inside chapters
package library

import (
	"crypto/sha1"
	"encoding/hex"
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

// Chapter states.
const (
	StatePending = ""
	StateOK      = "ok"
	StateFailed  = "failed"
	StateLocked  = "locked"
)

// Chapter is one entry of a stored book.
type Chapter struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url,omitempty"`
	Index  int    `json:"index"`
	Locked bool   `json:"locked,omitempty"`
	State  string `json:"state,omitempty"`
	Error  string `json:"error,omitempty"`
	Words  int    `json:"words,omitempty"`
	// Revised is the date (YYYY-MM-DD) an update found the site's text had
	// changed and replaced the saved copy.
	Revised string `json:"revised,omitempty"`
	// Removed marks a saved chapter the site no longer lists. Its file is
	// kept; it is never deleted silently.
	Removed bool `json:"removed,omitempty"`
	// TitleOriginal is the chapter title in the novel's own language, when
	// Title holds its English translation.
	TitleOriginal string `json:"titleOriginal,omitempty"`
	// Translated means an English copy of the chapter is saved next to the
	// original (chapters/0001.en.html). TranslateError says why the last
	// try failed; the next update tries again.
	Translated     bool   `json:"translated,omitempty"`
	TranslateError string `json:"translateError,omitempty"`
}

// Book is the contents of book.json.
type Book struct {
	Slug            string            `json:"slug"`
	SiteID          string            `json:"siteId"`
	SiteName        string            `json:"siteName"`
	URL             string            `json:"url"`
	Title           string            `json:"title"`
	Author          string            `json:"author"`
	Description     string            `json:"description"`
	Status          string            `json:"status,omitempty"`
	CoverURL        string            `json:"coverUrl,omitempty"`
	CoverFile       string            `json:"coverFile,omitempty"`
	TOCMode         string            `json:"tocMode"`
	FirstChapterURL string            `json:"firstChapterUrl,omitempty"`
	Vars            map[string]string `json:"vars,omitempty"`
	Chapters        []Chapter         `json:"chapters"`
	AddedAt         time.Time         `json:"addedAt"`
	UpdatedAt       time.Time         `json:"updatedAt"`
	// CheckedAt is when an update last finished for this book.
	CheckedAt time.Time `json:"checkedAt,omitempty"`

	// Language is "zh", "ja" or "ko" for a novel that is translated to
	// English as it downloads; empty for English novels.
	Language string `json:"language,omitempty"`
	// OriginalTitle, OriginalAuthor and OriginalDescription keep the
	// novel's details in its own language; Title etc. hold the English.
	OriginalTitle       string `json:"originalTitle,omitempty"`
	OriginalAuthor      string `json:"originalAuthor,omitempty"`
	OriginalDescription string `json:"originalDescription,omitempty"`
	// NoTranslate keeps a foreign novel in its own language only.
	NoTranslate bool `json:"noTranslate,omitempty"`
	// Names are the user's fixes for names the translation gets wrong or
	// spells differently between chapters: original -> English.
	Names map[string]string `json:"names,omitempty"`
}

// Foreign reports whether the book is in another language than English.
func (b *Book) Foreign() bool { return b.Language != "" && b.Language != "en" }

// Translating reports whether the book's chapters are translated.
func (b *Book) Translating() bool { return b.Foreign() && !b.NoTranslate }

// Counts summarises chapter states.
func (b *Book) Counts() (ok, failed, locked, pending int) {
	for _, c := range b.Chapters {
		switch c.State {
		case StateOK:
			ok++
		case StateFailed:
			failed++
		case StateLocked:
			locked++
		default:
			pending++
		}
	}
	return
}

// Store is the on-disk library.
type Store struct {
	Root string
	mu   sync.Mutex
}

// DefaultRoot is %AppData%/NovelArchiver/library (or the platform
// equivalent). NOVELARCHIVER_LIBRARY overrides it.
func DefaultRoot() string {
	if v := os.Getenv("NOVELARCHIVER_LIBRARY"); v != "" {
		return v
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		dir, _ = os.UserHomeDir()
	}
	return filepath.Join(dir, "NovelArchiver", "library")
}

// Open creates the root directory if needed.
func Open(root string) (*Store, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, err
	}
	return &Store{Root: abs}, nil
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify makes a filesystem-safe folder name.
func Slugify(s string) string {
	s = slugRe.ReplaceAllString(strings.ToLower(s), "-")
	s = strings.Trim(s, "-")
	if len(s) > 80 {
		s = strings.Trim(s[:80], "-")
	}
	if s == "" {
		s = "novel"
	}
	return s
}

// validSlug guards every path built from a slug coming from the UI.
func validSlug(slug string) error {
	if slug == "" || slug != Slugify(slug) {
		return fmt.Errorf("invalid novel id %q", slug)
	}
	return nil
}

func (s *Store) dir(slug string) string { return filepath.Join(s.Root, slug) }

// ChapterPath is where a chapter fragment lives.
func (s *Store) ChapterPath(slug string, num int) string {
	return filepath.Join(s.dir(slug), "chapters", fmt.Sprintf("%04d.html", num))
}

// TranslationPath is where a chapter's English translation lives.
func (s *Store) TranslationPath(slug string, num int) string {
	return filepath.Join(s.dir(slug), "chapters", fmt.Sprintf("%04d.en.html", num))
}

// WriteTranslation stores a chapter's English translation.
func (s *Store) WriteTranslation(slug string, num int, html string) error {
	p := s.TranslationPath(slug, num)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(html), 0o644)
}

// ReadTranslation loads a chapter's English translation.
func (s *Store) ReadTranslation(slug string, num int) (string, error) {
	if err := validSlug(slug); err != nil {
		return "", err
	}
	b, err := os.ReadFile(s.TranslationPath(slug, num))
	return string(b), err
}

// ImageDir holds chapter images.
func (s *Store) ImageDir(slug string) string { return filepath.Join(s.dir(slug), "img") }

// FindByURL returns the slug of a stored book with this source URL.
func (s *Store) FindByURL(u string) string {
	books, _ := s.List()
	norm := func(x string) string {
		x = strings.TrimPrefix(strings.TrimPrefix(x, "https://"), "http://")
		return strings.TrimRight(strings.TrimPrefix(x, "www."), "/")
	}
	for _, b := range books {
		if norm(b.URL) == norm(u) {
			return b.Slug
		}
	}
	return ""
}

// NewSlug picks an unused slug for a title.
func (s *Store) NewSlug(title string) string {
	base := Slugify(title)
	slug := base
	for i := 2; ; i++ {
		if _, err := os.Stat(s.dir(slug)); errors.Is(err, fs.ErrNotExist) {
			return slug
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

// Load reads book.json.
func (s *Store) Load(slug string) (*Book, error) {
	if err := validSlug(slug); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(s.dir(slug), "book.json"))
	if err != nil {
		return nil, err
	}
	var b Book
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// Save writes book.json atomically (temp file + rename), so a crash never
// leaves a half-written file.
func (s *Store) Save(b *Book) error {
	if err := validSlug(b.Slug); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	b.UpdatedAt = time.Now()
	if b.AddedAt.IsZero() {
		b.AddedAt = b.UpdatedAt
	}
	dir := s.dir(b.Slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, "book.json.tmp")
	if err := writeSynced(tmp, data); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, "book.json"))
}

// List returns every stored book, newest first.
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
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

// WriteChapter stores a cleaned fragment.
func (s *Store) WriteChapter(slug string, num int, html string) error {
	p := s.ChapterPath(slug, num)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(html), 0o644)
}

// ReadChapter loads a cleaned fragment.
func (s *Store) ReadChapter(slug string, num int) (string, error) {
	if err := validSlug(slug); err != nil {
		return "", err
	}
	b, err := os.ReadFile(s.ChapterPath(slug, num))
	return string(b), err
}

// ImageName is the stored file name for an image URL.
func ImageName(url, contentType string) string {
	h := sha1.Sum([]byte(url))
	ext := ".jpg"
	switch {
	case strings.Contains(contentType, "png"), strings.HasSuffix(strings.ToLower(url), ".png"):
		ext = ".png"
	case strings.Contains(contentType, "webp"), strings.HasSuffix(strings.ToLower(url), ".webp"):
		ext = ".webp"
	case strings.Contains(contentType, "gif"), strings.HasSuffix(strings.ToLower(url), ".gif"):
		ext = ".gif"
	}
	return hex.EncodeToString(h[:8]) + ext
}

// WriteFile stores an arbitrary file (cover or image) under the book folder.
func (s *Store) WriteFile(slug, rel string, data []byte) error {
	p := filepath.Join(s.dir(slug), filepath.FromSlash(rel))
	if !strings.HasPrefix(p, s.dir(slug)+string(os.PathSeparator)) {
		return fmt.Errorf("bad path %q", rel)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

// FilePath resolves a file inside a book folder, refusing path escapes.
func (s *Store) FilePath(slug, rel string) (string, error) {
	if err := validSlug(slug); err != nil {
		return "", err
	}
	p := filepath.Join(s.dir(slug), filepath.FromSlash(rel))
	if !strings.HasPrefix(p, s.dir(slug)+string(os.PathSeparator)) {
		return "", fmt.Errorf("bad path %q", rel)
	}
	return p, nil
}

// DeleteBook removes a novel completely.
func (s *Store) DeleteBook(slug string) error {
	if err := validSlug(slug); err != nil {
		return err
	}
	return os.RemoveAll(s.dir(slug))
}

// DeleteChapters frees space by removing downloaded chapters from..to
// (inclusive). They stay in the chapter list so they can be re-downloaded.
func (s *Store) DeleteChapters(slug string, from, to int) (int, error) {
	b, err := s.Load(slug)
	if err != nil {
		return 0, err
	}
	if from < 1 || to < from {
		return 0, fmt.Errorf("invalid range %d–%d", from, to)
	}
	n := 0
	for i := range b.Chapters {
		c := &b.Chapters[i]
		if c.Number < from || c.Number > to || c.State != StateOK {
			continue
		}
		if err := os.Remove(s.ChapterPath(slug, c.Number)); err == nil || errors.Is(err, fs.ErrNotExist) {
			c.State, c.Words = StatePending, 0
			n++
		}
	}
	return n, s.Save(b)
}

// DeleteImages removes chapter images (text stays readable; images show as
// broken until re-downloaded).
func (s *Store) DeleteImages(slug string) error {
	if err := validSlug(slug); err != nil {
		return err
	}
	return os.RemoveAll(s.ImageDir(slug))
}

// Size returns the bytes used by one book.
func (s *Store) Size(slug string) int64 {
	var total int64
	_ = filepath.WalkDir(s.dir(slug), func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

// Reading is where the reader last stopped in a book. It lives in its own
// file (reading.json) so a download saving book.json never overwrites it.
type Reading struct {
	Chapter int       `json:"chapter"`
	At      time.Time `json:"at"`
}

// Reading returns the saved reading place (zero if none).
func (s *Store) Reading(slug string) Reading {
	var r Reading
	if validSlug(slug) != nil {
		return r
	}
	if data, err := os.ReadFile(filepath.Join(s.dir(slug), "reading.json")); err == nil {
		_ = json.Unmarshal(data, &r)
	}
	return r
}

// SetReading records the chapter the reader is on.
func (s *Store) SetReading(slug string, chapter int) error {
	if err := validSlug(slug); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(s.dir(slug), "book.json")); err != nil {
		return err
	}
	data, _ := json.Marshal(Reading{Chapter: chapter, At: time.Now()})
	tmp := filepath.Join(s.dir(slug), "reading.json.tmp")
	if err := writeSynced(tmp, data); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(s.dir(slug), "reading.json"))
}

// ImageBytes returns the bytes used by a book's chapter images.
func (s *Store) ImageBytes(slug string) int64 {
	var total int64
	_ = filepath.WalkDir(s.ImageDir(slug), func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

// writeSynced writes a file and flushes it to disk before it is renamed
// into place, so a crash or power loss never leaves an empty book.json.
func writeSynced(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
