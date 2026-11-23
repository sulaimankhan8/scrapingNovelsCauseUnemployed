package archiver

import (
	"path/filepath"
	"testing"

	"novelarchiver/internal/library"
	"novelarchiver/internal/source"
)

func TestBookFromNovelKeepsRemovedChapters(t *testing.T) {
	st, err := library.Open(filepath.Join(t.TempDir(), "lib"))
	if err != nil {
		t.Fatal(err)
	}
	old := &library.Book{Slug: "n", URL: "https://x.test/n", Title: "N", TOCMode: "list", Chapters: []library.Chapter{
		{Number: 1, URL: "https://x.test/1", State: library.StateOK},
		{Number: 2, URL: "https://x.test/2", State: library.StateOK}, // removed on the site
		{Number: 3, URL: "https://x.test/3", State: library.StateOK},
		{Number: 4, URL: "https://x.test/4-old", State: library.StateOK}, // number reused by a new chapter
	}}
	if err := st.Save(old); err != nil {
		t.Fatal(err)
	}
	a := &Archiver{Store: st}
	b, err := a.BookFromNovel(&source.Novel{URL: "https://x.test/n", Title: "N", TOCMode: "list", Chapters: []source.ChapterRef{
		{Number: 1, URL: "https://x.test/1"},
		{Number: 3, URL: "https://x.test/3"},
		{Number: 4, URL: "https://x.test/4-new"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var nums []int
	removed := map[int]bool{}
	for _, c := range b.Chapters {
		nums = append(nums, c.Number)
		removed[c.Number] = c.Removed
	}
	if len(nums) != 4 || nums[0] != 1 || nums[1] != 2 || nums[2] != 3 || nums[3] != 4 {
		t.Fatalf("chapters = %v, want 1 2 3 4", nums)
	}
	if !removed[2] || removed[4] || removed[1] {
		t.Fatalf("removed flags = %v", removed)
	}
	if b.Chapters[3].State != "" {
		t.Fatalf("chapter 4 is a different chapter now and must be downloaded again, state=%q", b.Chapters[3].State)
	}
}

func TestTextOfIgnoresMarkupAndImagePaths(t *testing.T) {
	a := `<h1 class="ch-title">One</h1>
<p>Hello   <b>world</b></p><img src="img/abc.jpg">`
	b := `<h1 class="ch-title">One</h1><p>Hello world</p><img src="https://cdn.test/abc.jpg">`
	if TextOf(a) != TextOf(b) {
		t.Fatalf("%q != %q", TextOf(a), TextOf(b))
	}
	if TextOf(a) == TextOf(`<h1>One</h1><p>Hello there</p>`) {
		t.Fatal("changed text must differ")
	}
}

func TestSelectChaptersOnlyFailed(t *testing.T) {
	b := &library.Book{Chapters: []library.Chapter{
		{Number: 1, State: library.StateOK},
		{Number: 2, State: library.StateFailed},
		{Number: 3},
		{Number: 4, State: library.StateFailed},
		{Number: 5, State: library.StateLocked},
	}}
	a := &Archiver{}
	idx, cached := a.selectChapters(b, Options{OnlyFailed: true})
	if len(idx) != 2 || b.Chapters[idx[0]].Number != 2 || b.Chapters[idx[1]].Number != 4 || cached != 0 {
		t.Fatalf("retry picked %v (cached %d), want chapters 2 and 4", idx, cached)
	}
	idx, cached = a.selectChapters(b, Options{})
	if len(idx) != 3 || cached != 1 {
		t.Fatalf("update picked %v (cached %d), want 2, 3, 4 and 1 cached", idx, cached)
	}
}
