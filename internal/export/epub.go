// Package export writes a stored novel as an EPUB 3 file (with an EPUB 2
// toc.ncx for older readers), so it can be read in any e-reader app.
package export

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"html"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"novelarchiver/internal/library"
)

// Options limits what goes into the book.
type Options struct {
	From, To int // chapter range; 0 = all
}

// Result describes the written file.
type Result struct {
	Chapters int
	Images   int
	Bytes    int64
}

// ErrNothingToExport means no chapters in range are downloaded.
var ErrNothingToExport = errors.New("no downloaded chapters to export")

const css = `body{font-family:serif;line-height:1.6;margin:0 4%}
h1.ch-title{font-size:1.4em;margin:1em 0}
p{margin:0 0 .9em;text-indent:0}
img{max-width:100%;height:auto}
hr{border:0;border-top:1px solid #999;margin:1.5em 30%}
blockquote{margin:1em 1.5em;font-style:italic}
.sys{font-family:monospace;font-size:.9em;background:#eef3f8;border-left:3px solid #7d9bb8;padding:.4em .7em;margin:0}
.sys + p:not(.sys){margin-top:.9em}
table{border-collapse:collapse;width:100%;font-size:.9em;margin:1em 0}
td,th{border:1px solid #7d9bb8;padding:.3em .5em;vertical-align:top}
.center{text-align:center}
`

var imgSrcRe = regexp.MustCompile(`src="img/([^"]+)"`)

// EPUB writes book slug to w.
func EPUB(store *library.Store, slug string, w io.Writer, opts Options) (*Result, error) {
	b, err := store.Load(slug)
	if err != nil {
		return nil, err
	}
	var chapters []library.Chapter
	for _, c := range b.Chapters {
		if c.State != library.StateOK {
			continue
		}
		if (opts.From > 0 && c.Number < opts.From) || (opts.To > 0 && c.Number > opts.To) {
			continue
		}
		chapters = append(chapters, c)
	}
	if len(chapters) == 0 {
		return nil, ErrNothingToExport
	}

	zw := zip.NewWriter(w)
	// The mimetype entry must be first and stored uncompressed.
	now := time.Now()
	mw, err := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store, Modified: now})
	if err != nil {
		return nil, err
	}
	io.WriteString(mw, "application/epub+zip")

	add := func(name string, data []byte) error {
		f, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: now})
		if err != nil {
			return err
		}
		_, err = f.Write(data)
		return err
	}
	if err := add("META-INF/container.xml", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`)); err != nil {
		return nil, err
	}
	if err := add("OEBPS/style.css", []byte(css)); err != nil {
		return nil, err
	}

	var manifest, spine, navItems, ncxPoints strings.Builder
	images := map[string]bool{}

	// Cover
	coverID := ""
	if b.CoverFile != "" {
		if p, err := store.FilePath(slug, b.CoverFile); err == nil {
			if data, err := os.ReadFile(p); err == nil {
				name := "cover" + path.Ext(b.CoverFile)
				if err := add("OEBPS/"+name, data); err != nil {
					return nil, err
				}
				coverID = "cover-image"
				fmt.Fprintf(&manifest, `<item id="cover-image" href="%s" media-type="%s" properties="cover-image"/>`+"\n", name, mediaType(name))
				page := xhtmlPage("Cover", `<div class="center"><img src="`+name+`" alt="Cover"/></div>`, "")
				if err := add("OEBPS/cover.xhtml", []byte(page)); err != nil {
					return nil, err
				}
				manifest.WriteString(`<item id="cover" href="cover.xhtml" media-type="application/xhtml+xml"/>` + "\n")
				spine.WriteString(`<itemref idref="cover" linear="no"/>` + "\n")
			}
		}
	}

	// Title page
	info := `<h1>` + esc(b.Title) + `</h1>`
	if b.Author != "" {
		info += `<p><em>` + esc(b.Author) + `</em></p>`
	}
	for _, para := range strings.Split(b.Description, "\n") {
		if strings.TrimSpace(para) != "" {
			info += `<p>` + esc(strings.TrimSpace(para)) + `</p>`
		}
	}
	info += fmt.Sprintf(`<p><small>Chapters %d–%d · saved from %s with NovelArchiver</small></p>`,
		chapters[0].Number, chapters[len(chapters)-1].Number, esc(b.SiteName))
	if err := add("OEBPS/title.xhtml", []byte(xhtmlPage(b.Title, info, "../"))); err != nil {
		return nil, err
	}
	manifest.WriteString(`<item id="title" href="title.xhtml" media-type="application/xhtml+xml"/>` + "\n")
	spine.WriteString(`<itemref idref="title"/>` + "\n")

	for i, c := range chapters {
		frag, err := store.ReadChapter(slug, c.Number)
		if err != nil {
			continue
		}
		// Images: chapters reference "img/<name>"; in the EPUB they live at
		// OEBPS/img/ and chapters at OEBPS/text/.
		for _, m := range imgSrcRe.FindAllStringSubmatch(frag, -1) {
			name := m[1]
			if images[name] {
				continue
			}
			if p, err := store.FilePath(slug, "img/"+name); err == nil {
				if data, err := os.ReadFile(p); err == nil {
					if err := add("OEBPS/img/"+name, data); err != nil {
						return nil, err
					}
					images[name] = true
					fmt.Fprintf(&manifest, `<item id="img-%d" href="img/%s" media-type="%s"/>`+"\n", len(images), name, mediaType(name))
				}
			}
		}
		frag = imgSrcRe.ReplaceAllString(frag, `src="../img/$1"`)
		body, err := toXHTML(frag)
		if err != nil {
			body = "<p>" + esc(stripTags(frag)) + "</p>"
		}
		id := fmt.Sprintf("ch%04d", c.Number)
		file := "text/" + id + ".xhtml"
		if err := add("OEBPS/"+file, []byte(xhtmlPage(c.Title, body, "../"))); err != nil {
			return nil, err
		}
		fmt.Fprintf(&manifest, `<item id="%s" href="%s" media-type="application/xhtml+xml"/>`+"\n", id, file)
		fmt.Fprintf(&spine, `<itemref idref="%s"/>`+"\n", id)
		fmt.Fprintf(&navItems, `<li><a href="%s">%s</a></li>`+"\n", file, esc(c.Title))
		fmt.Fprintf(&ncxPoints, `<navPoint id="np%d" playOrder="%d"><navLabel><text>%s</text></navLabel><content src="%s"/></navPoint>`+"\n",
			i+1, i+1, esc(c.Title), file)
	}

	nav := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="en">
<head><title>Contents</title></head>
<body><nav epub:type="toc" id="toc"><h1>Contents</h1><ol>
` + navItems.String() + `</ol></nav></body></html>`
	if err := add("OEBPS/nav.xhtml", []byte(nav)); err != nil {
		return nil, err
	}

	uid := "urn:novelarchiver:" + slug
	ncx := `<?xml version="1.0" encoding="UTF-8"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
<head><meta name="dtb:uid" content="` + esc(uid) + `"/></head>
<docTitle><text>` + esc(b.Title) + `</text></docTitle>
<navMap>
` + ncxPoints.String() + `</navMap></ncx>`
	if err := add("OEBPS/toc.ncx", []byte(ncx)); err != nil {
		return nil, err
	}

	coverMeta := ""
	if coverID != "" {
		coverMeta = `<meta name="cover" content="cover-image"/>`
	}
	opf := `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid" xml:lang="en">
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
<dc:identifier id="uid">` + esc(uid) + `</dc:identifier>
<dc:title>` + esc(b.Title) + `</dc:title>
<dc:creator>` + esc(orUnknown(b.Author)) + `</dc:creator>
<dc:language>en</dc:language>
<dc:source>` + esc(b.URL) + `</dc:source>
<meta property="dcterms:modified">` + time.Now().UTC().Format("2006-01-02T15:04:05Z") + `</meta>
` + coverMeta + `
</metadata>
<manifest>
<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
<item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
<item id="css" href="style.css" media-type="text/css"/>
` + manifest.String() + `</manifest>
<spine toc="ncx">
` + spine.String() + `</spine>
</package>`
	if err := add("OEBPS/content.opf", []byte(opf)); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return &Result{Chapters: len(chapters), Images: len(images)}, nil
}

// EPUBFile writes the book to path atomically.
func EPUBFile(store *library.Store, slug, dest string, opts Options) (*Result, error) {
	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return nil, err
	}
	res, err := EPUB(store, slug, f, opts)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp)
		return nil, err
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return nil, err
	}
	if fi, err := os.Stat(dest); err == nil {
		res.Bytes = fi.Size()
	}
	return res, nil
}

// FileName suggests "Title.epub" with characters every OS accepts.
func FileName(title string) string {
	t := regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]+`).ReplaceAllString(title, " ")
	t = strings.Join(strings.Fields(t), " ")
	if len(t) > 120 {
		t = t[:120]
	}
	if t == "" {
		t = "novel"
	}
	return filepath.Clean(t) + ".epub"
}

func xhtmlPage(title, body, rel string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" lang="en">
<head><meta charset="utf-8"/><title>` + esc(title) + `</title>
<link rel="stylesheet" type="text/css" href="` + rel + `style.css"/></head>
<body>
` + body + `
</body></html>`
}

// toXHTML re-serialises an HTML fragment as well-formed XHTML.
func toXHTML(frag string) (string, error) {
	ctx := &xhtml.Node{Type: xhtml.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := xhtml.ParseFragment(strings.NewReader(frag), ctx)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	for _, n := range nodes {
		writeXHTML(&buf, n)
	}
	return buf.String(), nil
}

var voidElems = map[string]bool{"br": true, "hr": true, "img": true, "wbr": true, "col": true}

func writeXHTML(w *bytes.Buffer, n *xhtml.Node) {
	switch n.Type {
	case xhtml.TextNode:
		w.WriteString(esc(n.Data))
	case xhtml.ElementNode:
		w.WriteString("<" + n.Data)
		for _, a := range n.Attr {
			if a.Namespace != "" || strings.ContainsAny(a.Key, ":") {
				continue
			}
			w.WriteString(" " + a.Key + `="` + esc(a.Val) + `"`)
		}
		if n.Data == "img" && attr(n, "alt") == "" {
			w.WriteString(` alt=""`)
		}
		if voidElems[n.Data] {
			w.WriteString("/>")
			return
		}
		w.WriteString(">")
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			writeXHTML(w, c)
		}
		w.WriteString("</" + n.Data + ">")
	}
}

func attr(n *xhtml.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func esc(s string) string {
	// html.EscapeString escapes & < > ' " — all XML-safe.
	return html.EscapeString(s)
}

func stripTags(s string) string { return regexp.MustCompile(`<[^>]*>`).ReplaceAllString(s, " ") }

func orUnknown(s string) string {
	if s == "" {
		return "Unknown"
	}
	return s
}

func mediaType(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	default:
		return "image/jpeg"
	}
}
