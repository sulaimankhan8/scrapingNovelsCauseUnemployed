// Package clean turns scraped chapter HTML into a safe, minimal fragment
// that keeps the author's formatting: bold/italic, tables, rules, images,
// centred lines and in-story "system screen" boxes.
package clean

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
)

var blockTags = map[string]bool{"p": true, "div": true, "table": true, "blockquote": true, "ul": true, "ol": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true, "hr": true, "figure": true, "pre": true}

// Result is a cleaned chapter.
type Result struct {
	HTML      string   // sanitised fragment, no <html>/<body>
	Words     int      // word count of the visible text
	ImageURLs []string // absolute URLs of <img> sources, in order
}

// junk elements that are never story content.
const junkSelectors = `script, style, noscript, iframe, ins, form, button, input, select, textarea, nav, ` +
	`svg, canvas, video, audio, object, embed, link, meta, ` +
	`[class*="adsbygoogle"], [id^="pf-"], [class^="ads"], [class*=" ads"], [id*="ad-slot"], [class*="banner"], ` +
	`[data-chapter-ad], .nf-ads, [class*="sharethis"], [class*="comment"]`

// Watermark lines: whole-line matches only, so story text that merely
// mentions "pirate" or "read at" is never removed.
var watermarkRes = []*regexp.Regexp{
	// "Read latest chapters at novelbin.com", "Source: n0velfull . net", ...
	// A domain (or an obfuscated/brand site name) is required, so story
	// sentences like "Read the novel she gave him." are never touched.
	regexp.MustCompile(`(?i)^[\s\p{P}]*(source|read|visit|find|follow|search|support|discover|updated?|new|the (source|original)|this (chapter|content|novel)|content|for the best|enhance your|chapters?)\b.{0,140}(\b[a-z0-9-]*n[o0]v[e3]l[a-z0-9-]*\s*(\.|\bdot\b)\s*(com|net|org|xyz|me|co|site|app)\b|\b(n[o0]v[e3]lbin|n[o0]v[e3]lfull|freewebn[o0]vel|readn[o0]velfull|lightn[o0]velpub|n[o0]velfire|wuxiaworld|webn[o0]vel|scribblehub|royal ?road)\b).{0,60}$`),
	regexp.MustCompile(`(?i)^[\s\p{P}]*(www\.)?[a-z0-9-]+\.(com|net|org|xyz|me|co|site)[\s\p{P}]*$`),
	// Royal Road style anti-piracy decoys (usually hidden by CSS too).
	regexp.MustCompile(`(?i)\b(royal ?road|amazon)\b.{0,120}\b(without (the author'?s )?(consent|permission)|stolen|report (the|this) (violation|story)|unauthori[sz]ed)`),
	regexp.MustCompile(`(?i)\b(stolen|pirated|unauthori[sz]ed (use|copy|reproduction))\b.{0,120}\b(royal ?road|amazon|report)\b`),
}

var (
	emptyParaRe     = regexp.MustCompile(`(?i)<p[^>]*>(\s|&nbsp;|<br\s*/?>)*</p>`)
	brBetweenBlocks = regexp.MustCompile(`(?i)(^|</p>|</h[1-6]>|</table>|</blockquote>|<hr\s*/?>)(\s*<br\s*/?>)+`)
	multiBrRe       = regexp.MustCompile(`(?i)(<br\s*/?>\s*){3,}`)
	sysOpenClose    = [][2]string{{"[", "]"}, {"【", "】"}, {"「", "」"}, {"<", ">"}, {"『", "』"}}
	alnumOnly       = regexp.MustCompile(`[^\p{L}\p{N}]+`)
	chapterPrefix   = regexp.MustCompile(`(?i)^(chapter|ch\.?|episode|ep\.?)\s*0*`)
)

var policy = func() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("p", "br", "hr", "strong", "b", "em", "i", "u", "s", "strike", "del", "sup", "sub", "small",
		"blockquote", "h1", "h2", "h3", "h4", "h5", "h6", "ul", "ol", "li", "pre", "code", "span", "div",
		"table", "thead", "tbody", "tfoot", "tr", "td", "th", "caption", "ruby", "rt", "rp", "center", "figure", "figcaption")
	p.AllowAttrs("colspan", "rowspan").OnElements("td", "th")
	p.AllowAttrs("src", "alt", "width", "height").OnElements("img")
	p.AllowElements("img")
	p.AllowURLSchemes("http", "https", "data")
	p.AllowAttrs("class").Matching(regexp.MustCompile(`^(sys|sys-box|center)( (sys|sys-box|center))*$`)).Globally()
	p.AllowStyles("text-align").MatchingEnum("center", "right", "left").Globally()
	p.AllowStyles("font-weight").MatchingEnum("bold", "700", "800", "900").Globally()
	p.AllowStyles("font-style").MatchingEnum("italic").Globally()
	return p
}()

// Chapter cleans one chapter. baseURL resolves relative image links.
func Chapter(rawHTML, title, baseURL string) Result {
	rawHTML = textToParagraphs(rawHTML)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<div id=\"__root\">" + rawHTML + "</div>"))
	if err != nil {
		return Result{HTML: "<p>" + escape(rawHTML) + "</p>"}
	}
	root := doc.Find("#__root")
	base, _ := url.Parse(baseURL)

	root.Find(junkSelectors).Remove()

	// Images: resolve lazy-load attributes and make URLs absolute.
	var images []string
	root.Find("img").Each(func(_ int, img *goquery.Selection) {
		src := ""
		for _, a := range []string{"data-src", "data-lazy-src", "data-original", "src"} {
			if v := strings.TrimSpace(img.AttrOr(a, "")); v != "" && !strings.HasPrefix(v, "data:image/gif") {
				src = v
				break
			}
		}
		abs := resolve(base, src)
		if abs == "" {
			img.Remove()
			return
		}
		img.SetAttr("src", abs)
		images = append(images, abs)
	})

	// Remove watermark / decoy blocks (short leaf blocks whose whole text matches).
	root.Find("p, div, span, h1, h2, h3, h4, h5, h6, center, strong, em, i, b").Each(func(_ int, s *goquery.Selection) {
		if s.Find("p, div, table, img").Length() > 0 {
			return
		}
		t := normSpace(s.Text())
		if t == "" || len([]rune(t)) > 300 {
			return
		}
		for _, re := range watermarkRes {
			if re.MatchString(t) {
				s.Remove()
				return
			}
		}
	})

	// Drop leading blocks that just repeat the chapter title (some sources
	// put it twice, e.g. "<h3>Chapter 1</h3><h3>Chapter 1</h3>").
	if title != "" {
		want := titleKey(title)
		for i := 0; i < 3; i++ {
			first := firstTextBlock(root)
			if first == nil {
				break
			}
			got := titleKey(first.Text())
			if got == "" || want == "" || !(got == want || strings.HasPrefix(want, got) || strings.HasPrefix(got, want)) {
				break
			}
			first.Remove()
		}
	}

	// Mark in-story system screens: a block whose text is wholly wrapped in
	// [ ] / 【 】 / 「 」 / < > becomes class "sys"; the reader styles it as a box.
	root.Find("p, div, td, li").Each(func(_ int, s *goquery.Selection) {
		if s.Find("p, div").Length() > 0 {
			return
		}
		if isSystemLine(normSpace(s.Text())) {
			s.SetAttr("class", "sys")
		} else {
			s.RemoveAttr("class")
		}
	})
	// Unwrap plain layout <div>s that only wrap other blocks (Webnovel wraps
	// every paragraph in two of them), innermost first.
	divs := root.Find("div")
	for i := divs.Length() - 1; i >= 0; i-- {
		d := divs.Eq(i)
		if d.AttrOr("class", "") != "" || d.AttrOr("style", "") != "" {
			continue
		}
		onlyBlocks := true
		for n := d.Nodes[0].FirstChild; n != nil; n = n.NextSibling {
			if n.Type == html.TextNode && strings.TrimSpace(n.Data) != "" {
				onlyBlocks = false
				break
			}
			if n.Type == html.ElementNode && !blockTags[n.Data] {
				onlyBlocks = false
				break
			}
		}
		if onlyBlocks {
			d.ReplaceWithSelection(d.Children())
		}
	}

	// Tables are commonly used for status windows; keep them, mark as box.
	root.Find("table").Each(func(_ int, t *goquery.Selection) { t.SetAttr("class", "sys-box") })
	root.Find("[align=center]").Each(func(_ int, s *goquery.Selection) {
		s.SetAttr("style", "text-align: center")
	})

	inner, _ := root.Html()
	out := policy.Sanitize(inner)
	out = emptyParaRe.ReplaceAllString(out, "")
	out = multiBrRe.ReplaceAllString(out, "<br/><br/>")
	// <br>s between paragraphs only add gaps the reader's spacing already gives.
	out = brBetweenBlocks.ReplaceAllString(out, "$1")
	out = strings.TrimSpace(out)

	plain, _ := goquery.NewDocumentFromReader(strings.NewReader(out))
	words := 0
	if plain != nil {
		words = len(strings.Fields(plain.Text()))
	}
	if out == "" {
		out = "<p><em>This chapter had no readable text.</em></p>"
	}
	return Result{HTML: out, Words: words, ImageURLs: images}
}

// textToParagraphs converts sources that use bare text + <br><br> instead of
// <p> into paragraphs, so the reader can space them properly.
func textToParagraphs(h string) string {
	if strings.Contains(strings.ToLower(h), "<p") {
		return h
	}
	parts := regexp.MustCompile(`(?i)(\s*<br\s*/?>\s*){2,}|\n\s*\n`).Split(h, -1)
	var b strings.Builder
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			b.WriteString("<p>" + strings.TrimSpace(p) + "</p>\n")
		}
	}
	return b.String()
}

func isSystemLine(t string) bool {
	r := []rune(t)
	if len(r) < 3 || len(r) > 600 {
		return false
	}
	for _, oc := range sysOpenClose {
		if strings.HasPrefix(t, oc[0]) && strings.HasSuffix(t, oc[1]) {
			return true
		}
	}
	return false
}

func firstTextBlock(root *goquery.Selection) *goquery.Selection {
	var found *goquery.Selection
	root.Find("h1, h2, h3, h4, h5, h6, p, div, strong").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if s.Find("p, div, h1, h2, h3, h4").Length() > 0 {
			return true
		}
		if normSpace(s.Text()) != "" {
			found = s
			return false
		}
		return true
	})
	return found
}

// titleKey normalises "Chapter 1 - Crimson" and "Chapter 1: Crimson" alike.
func titleKey(s string) string {
	s = strings.ToLower(normSpace(s))
	s = chapterPrefix.ReplaceAllString(s, "")
	return alnumOnly.ReplaceAllString(s, "")
}

func normSpace(s string) string {
	return strings.Join(strings.FieldsFunc(s, unicode.IsSpace), " ")
}

func resolve(base *url.URL, href string) string {
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "//") {
		return "https:" + href
	}
	ref, err := url.Parse(href)
	if err != nil {
		return ""
	}
	if base == nil {
		if ref.IsAbs() {
			return ref.String()
		}
		return ""
	}
	return base.ResolveReference(ref).String()
}

func escape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}
