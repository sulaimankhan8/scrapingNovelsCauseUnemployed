package source

import (
	"encoding/json"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// genericMetadata fills any empty fields from JSON-LD and og: meta tags.
func genericMetadata(doc *goquery.Document, n *Novel) {
	doc.Find(`script[type="application/ld+json"]`).Each(func(_ int, s *goquery.Selection) {
		var v any
		if json.Unmarshal([]byte(s.Text()), &v) != nil {
			return
		}
		var objs []map[string]any
		switch t := v.(type) {
		case map[string]any:
			objs = append(objs, t)
			if g, ok := t["@graph"].([]any); ok {
				for _, x := range g {
					if m, ok := x.(map[string]any); ok {
						objs = append(objs, m)
					}
				}
			}
		case []any:
			for _, x := range t {
				if m, ok := x.(map[string]any); ok {
					objs = append(objs, m)
				}
			}
		}
		for _, o := range objs {
			typ := jsonString(o["@type"])
			if typ != "Book" && typ != "CreativeWork" && typ != "CreativeWorkSeries" && typ != "Novel" {
				continue
			}
			if n.Title == "" {
				n.Title = jsonString(o["name"])
			}
			if n.Description == "" {
				n.Description = jsonString(o["description"])
			}
			if n.Author == "" {
				switch a := o["author"].(type) {
				case map[string]any:
					n.Author = jsonString(a["name"])
				case []any:
					if len(a) > 0 {
						if m, ok := a[0].(map[string]any); ok {
							n.Author = jsonString(m["name"])
						}
					}
				case string:
					n.Author = a
				}
			}
			if n.Cover == "" {
				switch im := o["image"].(type) {
				case string:
					n.Cover = im
				case map[string]any:
					n.Cover = jsonString(im["url"])
				case []any:
					if len(im) > 0 {
						n.Cover = jsonString(im[0])
					}
				}
			}
		}
	})
	meta := func(prop string) string {
		v, _ := doc.Find(`meta[property="` + prop + `"], meta[name="` + prop + `"]`).First().Attr("content")
		return strings.TrimSpace(v)
	}
	if n.Title == "" {
		n.Title = meta("og:title")
	}
	if n.Title == "" {
		n.Title = textOf(doc.Find("h1").First())
	}
	if n.Cover == "" {
		n.Cover = resolve(doc.Url, meta("og:image"))
	}
	if n.Description == "" {
		n.Description = meta("og:description")
	}
	if n.Author == "" {
		n.Author = meta("og:novel:author")
	}
	if n.Author == "" {
		n.Author = meta("author")
	}
}

var chapterWord = regexp.MustCompile(`(?i)chapter|/c/|/read/|ch-\d|/\d+/?$`)

// genericTOC finds the largest group of chapter-looking links that share a
// URL directory, in page order.
func genericTOC(doc *goquery.Document) []ChapterRef {
	groups := map[string][]ChapterRef{}
	var order []string
	doc.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
		href := resolve(doc.Url, a.AttrOr("href", ""))
		if href == "" || !chapterWord.MatchString(href) {
			return
		}
		u, err := url.Parse(href)
		if err != nil || (doc.Url != nil && u.Host != doc.Url.Host) {
			return
		}
		dir := path.Dir(u.Path)
		if _, ok := groups[dir]; !ok {
			order = append(order, dir)
		}
		title := a.AttrOr("title", "")
		if title == "" {
			title = textOf(a)
		}
		groups[dir] = append(groups[dir], ChapterRef{Title: title, URL: href})
	})
	best := ""
	for _, d := range order {
		if len(groups[d]) > len(groups[best]) {
			best = d
		}
	}
	refs := groups[best]
	if len(refs) < 3 {
		return nil
	}
	return refs
}

// guessNovelLink looks for a breadcrumb / "back to novel" link on a chapter page.
func guessNovelLink(doc *goquery.Document, chapterURL string) string {
	for _, sel := range []string{
		`.breadcrumb a`, `[itemtype*="BreadcrumbList"] a`, `a.novel-title`, `a.truyen-title`,
		`a[rel="up"]`, `a[title*="Table of Contents"]`, `a:contains("Index")`, `a:contains("Table of Contents")`,
	} {
		var found string
		doc.Find(sel).EachWithBreak(func(_ int, a *goquery.Selection) bool {
			h := resolve(doc.Url, a.AttrOr("href", ""))
			if h != "" && h != chapterURL && !chapterWord.MatchString(h) && strings.Count(h, "/") > 3 {
				found = h
			}
			return found == ""
		})
		if found != "" {
			return found
		}
	}
	// Strip the last path segment ("…/novel-name/chapter-1" → "…/novel-name").
	if u, err := url.Parse(chapterURL); err == nil {
		dir := path.Dir(strings.TrimRight(u.Path, "/"))
		if dir != "/" && dir != "." {
			u.Path, u.RawQuery = dir, ""
			return u.String()
		}
	}
	return ""
}

// genericNext finds a "next chapter" link.
func genericNext(doc *goquery.Document) string {
	for _, sel := range []string{
		`link[rel="next"]`, `a[rel="next"]`, `#next_chap`, `#next_url`, `a.next_page`, `a.nextchap`,
		`a:contains("Next Chapter")`, `a:contains("Next")`,
	} {
		var found string
		doc.Find(sel).EachWithBreak(func(_ int, a *goquery.Selection) bool {
			found = resolve(doc.Url, a.AttrOr("href", ""))
			return found == ""
		})
		if found != "" {
			return found
		}
	}
	return ""
}

// readabilityContent picks the element whose direct <p> children hold the
// most text — a small, predictable version of Readability.
func readabilityContent(doc *goquery.Document) *goquery.Selection {
	var best *goquery.Selection
	bestScore := 0
	doc.Find("div, article, section, main").Each(func(_ int, el *goquery.Selection) {
		score := 0
		el.ChildrenFiltered("p").Each(func(_ int, p *goquery.Selection) {
			score += len(textOf(p))
		})
		if score > bestScore {
			best, bestScore = el, score
		}
	})
	if bestScore < 200 {
		return nil
	}
	return best
}
