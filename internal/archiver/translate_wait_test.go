package archiver

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"novelarchiver/internal/library"
	"novelarchiver/internal/translate"
)

// busyOnce refuses the first request like Google's rate limit, then works.
type busyOnce struct{ calls int }

func (b *busyOnce) Translate(_ context.Context, texts []string, _ string) ([]string, error) {
	b.calls++
	if b.calls == 1 {
		return nil, translate.ErrBusy
	}
	out := make([]string, len(texts))
	for i := range texts {
		out[i] = "Chapter one"
	}
	return out, nil
}

func (b *busyOnce) PausedFor() time.Duration {
	if b.calls == 1 {
		return 10 * time.Millisecond
	}
	return 0
}

func TestTranslatePendingWaitsOutShortPause(t *testing.T) {
	st, err := library.Open(filepath.Join(t.TempDir(), "lib"))
	if err != nil {
		t.Fatal(err)
	}
	b := &library.Book{Slug: "zh", Language: "zh", Chapters: []library.Chapter{{Number: 1, Title: "第一章", State: library.StateOK}}}
	if err := st.Save(b); err != nil {
		t.Fatal(err)
	}
	if err := st.WriteChapter("zh", 1, "<p>第一章</p>"); err != nil {
		t.Fatal(err)
	}
	tr := &busyOnce{}
	a := &Archiver{Store: st, Translator: tr}
	var msgs []string
	err = a.translatePending(context.Background(), b, func(kind string, data any) {
		if m, ok := data.(map[string]any); ok && kind == "status" {
			msgs = append(msgs, m["message"].(string))
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !b.Chapters[0].Translated {
		t.Fatalf("chapter not translated after the pause: %q", b.Chapters[0].TranslateError)
	}
	if len(msgs) != 1 || !strings.Contains(msgs[0], "slow down") {
		t.Fatalf("status messages = %q", msgs)
	}
}
