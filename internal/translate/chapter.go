package translate

import (
	"context"
	"html"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/PuerkitoBio/goquery"
)

const blockSel = "h1, h2, h3, h4, h5, h6, p, li, dt, dd, td, th, blockquote, figcaption, div"

var brSplit = regexp.MustCompile(`(?i)<br\s*/?>`)
var tagRe = regexp.MustCompile(`<[^>]*>`)

// Result is a translated chapter.
type Result struct {
	HTML  string // the fragment, same structure as the original
	Title string // the translated chapter title (the fragment's <h1>), if any
	Words int    // English words
}

// Chapter translates a saved chapter fragment block by block: each
// paragraph, heading or list item gets its text translated and keeps its
// element, class (system boxes stay boxes) and a single wrapping <strong> or
// <em>. Lines split by <br> are translated line by line. Blocks with images,
// and blocks already in English, are left as they are. names replaces
// original-language names with the user's English spelling first.
func Chapter(ctx context.Context, t Translator, frag, from string, names map[string]string) (*Result, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(`<div id="__root">` + frag + `</div>`))
	if err != nil {
		return nil, err
	}
	root := doc.Find("#__root")
	replacer := namesReplacer(names)

	type slot struct {
		block *goquery.Selection
		lines int // >1: the block's lines were split at <br>
		first int // index into texts
	}
	var slots []slot
	var texts []string
	root.Find(blockSel).Each(func(_ int, s *goquery.Selection) {
		if s.Find(blockSel).Length() > 0 || s.Find("img").Length() > 0 {
			return
		}
		inner, _ := s.Html()
		parts := brSplit.Split(inner, -1)
		var lines []string
		for _, p := range parts {
			lines = append(lines, strings.TrimSpace(html.UnescapeString(tagRe.ReplaceAllString(p, ""))))
		}
		if !anyCJK(lines) {
			return
		}
		sl := slot{block: s, lines: len(lines), first: len(texts)}
		for _, l := range lines {
			texts = append(texts, replacer.Replace(l))
		}
		slots = append(slots, sl)
	})
	if len(texts) > 0 {
		got, err := t.Translate(ctx, texts, from)
		if err != nil {
			return nil, err
		}
		for _, sl := range slots {
			tr := got[sl.first : sl.first+sl.lines]
			if sl.lines > 1 {
				esc := make([]string, len(tr))
				for i, l := range tr {
					esc[i] = html.EscapeString(l)
				}
				sl.block.SetHtml(strings.Join(esc, "<br/>"))
				continue
			}
			// Keep a single wrapper such as <strong>[System]</strong>.
			kids := sl.block.Children()
			if kids.Length() == 1 && strings.TrimSpace(kids.Text()) == strings.TrimSpace(sl.block.Text()) {
				kids.SetText(tr[0])
			} else {
				sl.block.SetText(tr[0])
			}
		}
	}
	out, _ := root.Html()
	res := &Result{HTML: out, Words: len(strings.Fields(root.Text()))}
	if h := root.Find("h1").First(); h.Length() > 0 {
		res.Title = strings.TrimSpace(h.Text())
	}
	return res, nil
}

// Texts translates a few plain strings (a title, a description), applying
// the names list, and returns them in order; empty strings stay empty.
func Texts(ctx context.Context, t Translator, from string, names map[string]string, in ...string) ([]string, error) {
	replacer := namesReplacer(names)
	var idx []int
	var send []string
	out := make([]string, len(in))
	for i, s := range in {
		if strings.TrimSpace(s) == "" || !anyCJK([]string{s}) {
			out[i] = s
			continue
		}
		idx = append(idx, i)
		send = append(send, replacer.Replace(s))
	}
	if len(send) == 0 {
		return out, nil
	}
	got, err := t.Translate(ctx, send, from)
	if err != nil {
		return nil, err
	}
	for k, i := range idx {
		out[i] = got[k]
	}
	return out, nil
}

func namesReplacer(names map[string]string) *strings.Replacer {
	keys := make([]string, 0, len(names))
	for k := range names {
		if strings.TrimSpace(k) != "" && strings.TrimSpace(names[k]) != "" {
			keys = append(keys, k)
		}
	}
	// Longest first, so "克莱恩·莫雷蒂" wins over "克莱恩".
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	pairs := make([]string, 0, 2*len(keys))
	for _, k := range keys {
		pairs = append(pairs, k, " "+strings.TrimSpace(names[k])+" ")
	}
	return strings.NewReplacer(pairs...)
}

func anyCJK(lines []string) bool {
	for _, l := range lines {
		for _, r := range l {
			if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
				return true
			}
		}
	}
	return false
}
