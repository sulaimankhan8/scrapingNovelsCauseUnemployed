package archiver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"novelarchiver/internal/lang"
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

// needsTranslation: saved, and no English copy yet (or the original was
// rewritten by an update after the copy was made).
func (a *Archiver) needsTranslation(b *library.Book, c library.Chapter) bool {
	if c.State != library.StateOK {
		return false
	}
	en, err := os.Stat(a.Store.TranslationPath(b.Slug, c.Number))
	if err != nil || !c.Translated {
		return true
	}
	orig, err := os.Stat(a.Store.ChapterPath(b.Slug, c.Number))
	return err == nil && orig.ModTime().After(en.ModTime())
}

// translatePending translates every saved chapter that has no English copy
// yet: ones whose translation failed earlier, and ones an update rewrote.
// It stops (without failing the job) while Google is refusing requests.
func (a *Archiver) translatePending(ctx context.Context, b *library.Book, emit Emit) error {
	if !a.shouldTranslate(b) {
		return nil
	}
	var todo []int
	for i, c := range b.Chapters {
		if a.needsTranslation(b, c) {
			todo = append(todo, i)
		}
	}
	// waits counts breaks taken; idle, breaks in a row that weren't
	// followed by a single success (a longer block: stop and leave the
	// rest for Translate now).
	waits, idle, okCount, bad := 0, 0, 0, 0
	for n, i := range todo {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		c := &b.Chapters[i]
		t := a.translateOne(ctx, b, c.Number)
		if !t.ok && strings.Contains(t.err, "limiting requests") && waits < 6 && idle < 2 {
			// Google asked for a short break: take it (up to six times),
			// then carry on, rather than leaving the rest untranslated.
			if d := pausedFor(a.Translator); d > 0 && d <= 5*time.Minute {
				waits++
				idle++
				_ = a.Store.Save(b)
				emit("status", map[string]any{"message": fmt.Sprintf("Google Translate asked to slow down. Translating again in about %d min; everything is saved.", int(d.Minutes()+0.99))})
				select {
				case <-time.After(d + 2*time.Second):
				case <-ctx.Done():
					return ctx.Err()
				}
				t = a.translateOne(ctx, b, c.Number)
			}
		}
		t.apply(c)
		if !t.ok && strings.Contains(t.err, "limiting requests") {
			emit("status", map[string]any{"message": "Google Translate is busy; the remaining chapters are saved in " + lang.Name(b.Language) + ". Press Translate now on the novel page later."})
			break
		}
		if t.ok {
			okCount++
			idle = 0
		} else {
			bad++
		}
		if (n+1)%10 == 0 {
			_ = a.Store.Save(b)
		}
		emit("progress", Progress{
			Stage: "translating", Current: n + 1, Total: len(todo), Scraped: okCount, Failed: bad,
			Percentage: (n + 1) * 100 / len(todo), ChapterNum: c.Number, ChapterTitle: c.Title,
			Status: map[bool]string{true: "ok", false: "err"}[t.ok], Slug: b.Slug, Title: b.Title,
		})
	}
	return nil
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
		if c.TitleOriginal == "" && c.Title != "" {
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

// TranslateAll translates the title, the chapter titles and every saved
// chapter that has no English copy yet, without downloading anything.
func (a *Archiver) TranslateAll(ctx context.Context, b *library.Book, emit Emit) error {
	if !a.shouldTranslate(b) {
		return fmt.Errorf("this novel isn't set to be translated")
	}
	a.translateBook(ctx, b, Options{})
	_ = a.Store.Save(b)
	err := a.translatePending(ctx, b, emit)
	if serr := a.Store.Save(b); err == nil {
		err = serr
	}
	return err
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
