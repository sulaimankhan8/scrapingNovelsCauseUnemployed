package archiver

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

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
	for n, i := range todo {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		c := &b.Chapters[i]
		t := a.translateOne(ctx, b, c.Number)
		t.apply(c)
		if !t.ok && strings.Contains(t.err, "limiting requests") {
			emit("status", map[string]any{"message": "Google Translate is busy; the remaining chapters are saved in " + lang.Name(b.Language) + " and will be translated on the next update."})
			break
		}
		if (n+1)%10 == 0 {
			_ = a.Store.Save(b)
		}
		emit("progress", Progress{
			Stage: "translating", Current: n + 1, Total: len(todo), Scraped: n + 1,
			Percentage: (n + 1) * 100 / len(todo), ChapterNum: c.Number, ChapterTitle: c.Title,
			Status: map[bool]string{true: "ok", false: "err"}[t.ok], Slug: b.Slug, Title: b.Title,
		})
	}
	return nil
}

// translateBook puts the title, author, description and every chapter
// title in the list into English, keeping the originals. Each part runs
// only while it's still untranslated.
func (a *Archiver) translateBook(ctx context.Context, b *library.Book) {
	if !a.shouldTranslate(b) {
		return
	}
	a.translateChapterTitles(ctx, b)
	if b.OriginalTitle == "" || b.Title != b.OriginalTitle {
		return
	}
	got, err := translate.Texts(ctx, a.Translator, b.Language, b.Names, b.OriginalTitle, b.OriginalAuthor, b.OriginalDescription)
	if err != nil {
		return // stays in the original language; the next update tries again
	}
	if t := strings.TrimSpace(got[0]); t != "" {
		b.Title = t
	}
	b.Author, b.Description = strings.TrimSpace(got[1]), strings.TrimSpace(got[2])
}

// translateChapterTitles translates the titles of listed chapters that
// aren't translated yet, so the contents page reads in English before the
// chapters themselves are downloaded.
func (a *Archiver) translateChapterTitles(ctx context.Context, b *library.Book) {
	var idx []int
	var titles []string
	for i, c := range b.Chapters {
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
