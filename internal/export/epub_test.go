package export

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"regexp"
	"strings"
	"testing"

	"novelarchiver/internal/library"
)

func makeBook(t *testing.T) *library.Store {
	store, err := library.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	b := &library.Book{Slug: "test", Title: `Horror & "Games" <1>`, Author: "A. Writer", SiteName: "Site",
		Description: "Line one.\n\nLine two.", CoverFile: "cover.jpg", URL: "https://example.com/n"}
	frags := []string{
		`<h1 class="ch-title">Chapter 1</h1><p class="sys"><strong>[Quit?]</strong></p><p>Tom&nbsp;&amp;&nbsp;Jerry<br>next</p><p><img src="img/abc.png"></p><hr>`,
		`<h1 class="ch-title">Chapter 2</h1><table class="sys-box"><tr><td>HP</td><td>10</td></tr></table><p>Text</p>`,
		`<h1 class="ch-title">Chapter 3</h1><p>not saved</p>`,
	}
	for i, f := range frags {
		state := library.StateOK
		if i == 2 {
			state = library.StateFailed
		} else if err := store.WriteChapter(b.Slug, i+1, f); err != nil {
			t.Fatal(err)
		}
		b.Chapters = append(b.Chapters, library.Chapter{Number: i + 1, Title: "Chapter " + string(rune('1'+i)), State: state})
	}
	if err := store.Save(b); err != nil {
		t.Fatal(err)
	}
	store.WriteFile("test", "cover.jpg", []byte("\xff\xd8\xff fake jpeg"))
	store.WriteFile("test", "img/abc.png", []byte("\x89PNG fake"))
	return store
}

func TestEPUBIsWellFormed(t *testing.T) {
	store := makeBook(t)
	var buf bytes.Buffer
	res, err := EPUB(store, "test", &buf, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Chapters != 2 || res.Images != 1 {
		t.Errorf("result = %+v", res)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if zr.File[0].Name != "mimetype" || zr.File[0].Method != zip.Store {
		t.Fatalf("first entry %s method %d", zr.File[0].Name, zr.File[0].Method)
	}
	files := map[string]string{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		files[f.Name] = string(b)
	}
	if files["mimetype"] != "application/epub+zip" {
		t.Errorf("mimetype = %q", files["mimetype"])
	}
	// Every XML document must parse strictly.
	for name, body := range files {
		if !strings.HasSuffix(name, ".xhtml") && !strings.HasSuffix(name, ".opf") && !strings.HasSuffix(name, ".ncx") && !strings.HasSuffix(name, ".xml") {
			continue
		}
		d := xml.NewDecoder(strings.NewReader(body))
		d.Strict = true
		d.Entity = map[string]string{} // no HTML entities allowed in XHTML
		for {
			_, err := d.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Errorf("%s is not well-formed: %v", name, err)
				break
			}
		}
	}
	// Manifest hrefs must exist.
	for _, m := range regexp.MustCompile(`href="([^"]+)"`).FindAllStringSubmatch(files["OEBPS/content.opf"], -1) {
		if _, ok := files["OEBPS/"+m[1]]; !ok {
			t.Errorf("manifest lists missing file %s", m[1])
		}
	}
	ch1 := files["OEBPS/text/ch0001.xhtml"]
	for _, want := range []string{`<p class="sys"><strong>[Quit?]</strong></p>`, `src="../img/abc.png"`, `<br/>`, `<hr/>`, "Tom\u00a0&amp;\u00a0Jerry"} {
		if !strings.Contains(ch1, want) {
			t.Errorf("chapter 1 missing %q:\n%s", want, ch1)
		}
	}
	if _, ok := files["OEBPS/text/ch0003.xhtml"]; ok {
		t.Errorf("failed chapter exported")
	}
	if !strings.Contains(files["OEBPS/content.opf"], `properties="cover-image"`) {
		t.Errorf("no cover in manifest")
	}
	if !strings.Contains(files["OEBPS/nav.xhtml"], `href="text/ch0002.xhtml"`) {
		t.Errorf("nav missing chapter 2")
	}
}

func TestEPUBRangeAndEmpty(t *testing.T) {
	store := makeBook(t)
	var buf bytes.Buffer
	res, err := EPUB(store, "test", &buf, Options{From: 2, To: 2})
	if err != nil || res.Chapters != 1 {
		t.Fatalf("range: %+v %v", res, err)
	}
	if _, err := EPUB(store, "test", &buf, Options{From: 3}); err != ErrNothingToExport {
		t.Errorf("want ErrNothingToExport, got %v", err)
	}
	if got := FileName(`Re:Zero / "Ep" ?`); got != "Re Zero Ep.epub" {
		t.Errorf("FileName = %q", got)
	}
}
