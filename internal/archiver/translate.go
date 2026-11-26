package archiver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"novelarchiver/internal/library"
	"novelarchiver/internal/translate"
)

// translated is the outcome of translating one saved chapter.
type translated struct {
	ok    bool
	title string // English chapter title ("" if the chapter had none)
	words int
	err   string
}

// shouldTranslate reports whether this book's chapters get an English copy.
func (a *Archiver) shouldTranslate(b *library.Book) bool {
	return a.Translator != nil && b.Translating()
}

// translateOne translates the saved chapter num to English and stores it
// next to the original. It never touches the original.
func (a *Archiver) translateOne(ctx context.Context, b *library.Book, num int) translated {
	frag, err := a.Store.ReadChapter(b.Slug, num)
	if err != nil {
		return translated{err: err.Error()}
	}
	res, err := translate.Chapter(ctx, a.Translator, frag, b.Language, b.Names)
	if err != nil {
		return translated{err: err.Error()}
	}
	if err := a.Store.WriteTranslation(b.Slug, num, res.HTML); err != nil {
		return translated{err: err.Error()}
	}
	return translated{ok: true, title: res.Title, words: res.Words}
}

// apply records a translation on the chapter entry.
func (t translated) apply(c *library.Chapter) {
	c.Translated, c.TranslateError = t.ok, t.err
	if !t.ok {
		return
	}
	if t.title != "" {
		if c.TitleOriginal == "" {
			c.TitleOriginal = c.Title
		}
		c.Title = t.title
	}
	if t.words > 0 {
		c.Words = t.words
	}
}

// InlineTranslate is how many chapters a download translates as it saves
// them, so the start of a new novel can be read in English right away. The
// rest are left to the background translator, a few a minute, because
// translating a whole novel in one go gets Google to block the network.
const InlineTranslate = 5

// needsTranslation: saved, and no English copy yet (or the original was
// rewritten by an update after the copy was made). It goes by the files,
// so a book saved by another job without the Translated mark still counts
// as translated.
func (a *Archiver) needsTranslation(b *library.Book, c library.Chapter) bool {
	if c.State != library.StateOK {
		return false
	}
	en, err := os.Stat(a.Store.TranslationPath(b.Slug, c.Number))
	if err != nil {
		return true
	}
	orig, err := os.Stat(a.Store.ChapterPath(b.Slug, c.Number))
	return err == nil && orig.ModTime().After(en.ModTime())
}

// Pending lists saved chapters (by number) that have no English copy yet.
func (a *Archiver) Pending(b *library.Book) []int {
	if !a.shouldTranslate(b) {
		return nil
	}
	var out []int
	for _, c := range b.Chapters {
		if a.needsTranslation(b, c) {
			out = append(out, c.Number)
		}
	}
	return out
}

// MetaLeft counts what's still in the original language among the book's
// title (1) and its chapter titles.
func (a *Archiver) MetaLeft(b *library.Book) int {
	if !a.shouldTranslate(b) {
		return 0
	}
	n := 0
	if b.OriginalTitle != "" && b.Title == b.OriginalTitle {
		n++
	}
	for _, c := range b.Chapters {
		if c.TitleOriginal == "" && c.Title != "" && hasCJK(c.Title) {
			n++
		}
	}
	return n
}

// NeedsTranslation reports whether saved chapter num has no English copy.
func (a *Archiver) NeedsTranslation(b *library.Book, num int) bool {
	if !a.shouldTranslate(b) {
		return false
	}
	for _, c := range b.Chapters {
		if c.Number == num {
			return a.needsTranslation(b, c)
		}
	}
	return false
}

// TranslateMeta translates the title, author, description and up to
// titleBatch chapter titles, and saves the book.
func (a *Archiver) TranslateMeta(ctx context.Context, b *library.Book) error {
	if !a.shouldTranslate(b) {
		return nil
	}
	a.translateBook(ctx, b, Options{})
	return a.Store.Save(b)
}

func hasCJK(s string) bool {
	for _, r := range s {
		if r >= 0x3040 && r <= 0x9fff || r >= 0xac00 && r <= 0xd7af || r >= 0xf900 && r <= 0xfaff {
			return true
		}
	}
	return false
}

// translateBook puts the title, author and description into English
// first (they're what you see), then the titles of the chapters this run
// covers, keeping the originals. Each part runs only while it's still
// untranslated.
func (a *Archiver) translateBook(ctx context.Context, b *library.Book, opts Options) {
	if !a.shouldTranslate(b) {
		return
	}
	if b.OriginalTitle != "" && b.Title == b.OriginalTitle {
		got, err := translate.Texts(ctx, a.Translator, b.Language, b.Names, b.OriginalTitle, b.OriginalAuthor, b.OriginalDescription)
		if err == nil { // else it stays in the original language; the next update tries again
			if t := strings.TrimSpace(got[0]); t != "" {
				b.Title = t
			}
			b.Author, b.Description = strings.TrimSpace(got[1]), strings.TrimSpace(got[2])
		}
	}
	a.translateChapterTitles(ctx, b, opts.From, opts.To)
}

// titleBatch caps how many chapter titles one run translates up front, so
// a 1,500-chapter novel doesn't open with a burst of requests that gets
// Google to refuse the chapters themselves.
const titleBatch = 300

// translateChapterTitles translates the titles of listed chapters that
// aren't translated yet, so the contents page reads in English before the
// chapters themselves are downloaded.
func (a *Archiver) translateChapterTitles(ctx context.Context, b *library.Book, from, to int) {
	var idx []int
	var titles []string
	for i, c := range b.Chapters {
		if len(titles) >= titleBatch {
			break
		}
		if (from > 0 && c.Number < from) || (to > 0 && c.Number > to) {
			continue
		}
		if c.TitleOriginal == "" && c.Title != "" && hasCJK(c.Title) {
			idx = append(idx, i)
			titles = append(titles, c.Title)
		}
	}
	if len(titles) == 0 {
		return
	}
	got, err := translate.Texts(ctx, a.Translator, b.Language, b.Names, titles...)
	if err != nil {
		return
	}
	for k, i := range idx {
		if t := strings.TrimSpace(got[k]); t != "" && t != titles[k] {
			b.Chapters[i].TitleOriginal, b.Chapters[i].Title = titles[k], t
		}
	}
}

// TranslateChapter puts one saved chapter into English now and records it.
func (a *Archiver) TranslateChapter(ctx context.Context, b *library.Book, num int) error {
	if !a.shouldTranslate(b) {
		return fmt.Errorf("this novel isn't set to be translated")
	}
	for i := range b.Chapters {
		c := &b.Chapters[i]
		if c.Number != num {
			continue
		}
		if c.State != library.StateOK {
			return fmt.Errorf("chapter %d isn't downloaded", num)
		}
		t := a.translateOne(ctx, b, num)
		t.apply(c)
		if err := a.Store.Save(b); err != nil {
			return err
		}
		if !t.ok {
			return errors.New(t.err)
		}
		return nil
	}
	return fmt.Errorf("no chapter %d", num)
}

// Untranslated counts saved chapters with no English copy yet.
func (a *Archiver) Untranslated(b *library.Book) int {
	if !a.shouldTranslate(b) {
		return 0
	}
	n := 0
	for _, c := range b.Chapters {
		if a.needsTranslation(b, c) {
			n++
		}
	}
	return n
}

// pausedFor is how long the translator is refusing requests for, if it
// says.
func pausedFor(t translate.Translator) time.Duration {
	if p, ok := t.(interface{ PausedFor() time.Duration }); ok {
		return p.PausedFor()
	}
	return 0
}

// foreignSlug names a foreign novel's folder after its site and the id in
// its link ("uukanshu-2090", "syosetu-n2710db"): titles in Chinese or
// Japanese would all become "novel".
func foreignSlug(siteID, link string) string {
	ids := regexp.MustCompile(`[A-Za-z]*\d+[A-Za-z0-9]*`).FindAllString(link, -1)
	id := ""
	if len(ids) > 0 {
		id = ids[len(ids)-1]
	}
	return library.Slugify(fmt.Sprintf("%s %s", siteID, id))
}
