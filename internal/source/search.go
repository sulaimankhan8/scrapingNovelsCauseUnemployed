package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"novelarchiver/internal/fetch"
	"novelarchiver/internal/lang"
	"novelarchiver/internal/sites"
)

// SearchHit is one novel found by a site's own search page.
type SearchHit struct {
	SiteID   string `json:"siteId"`
	SiteName string `json:"siteName"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	Author   string `json:"author,omitempty"`
	Cover    string `json:"cover,omitempty"`
	// Latest is the newest chapter the results page shows ("Chapter 3256
	// City of Gates"); Chapters is the count it shows, or the number of the
	// latest chapter. 0 means the page doesn't say.
	Latest   string `json:"latest,omitempty"`
	Chapters int    `json:"chapters,omitempty"`
	// Language of the site ("zh", "ja", ...); empty for English.
	Language string `json:"language,omitempty"`
	// TitleOriginal is the title as the site wrote it, when Title holds an
	// English translation (Chinese and Japanese sites).
	TitleOriginal string `json:"titleOriginal,omitempty"`
}

// maxHitsPerSite keeps one busy site from flooding the results.
const maxHitsPerSite = 12

// Searchable lists the profiles whose search can run now. Sites whose
// search needs the browser are skipped when no browser is wired in; sites
// marked "optIn" (their search page usually shows a human check) are skipped
// unless withOptIn is set.
func (e *Engine) Searchable(withOptIn bool) []*sites.Profile {
	var out []*sites.Profile
	for _, p := range e.Registry.Profiles {
		if sites.Str(p.Search, "page") == "" {
			continue
		}
		if p.NeedsBrowser("search") && e.Browser == nil {
			continue
		}
		if sites.Bool(p.Search, "optIn") && !withOptIn {
			continue
		}
		out = append(out, p)
	}
	return out
}

// SearchSite runs one site's search page for query and returns the novels
// it lists. Only links that the profile recognises as a novel page are
// kept, so every hit can be downloaded.
func (e *Engine) SearchSite(ctx context.Context, p *sites.Profile, query string) ([]SearchHit, error) {
	query = strings.Join(strings.Fields(query), " ")
	if query == "" {
		return nil, errors.New("type a title to search for")
	}
	tmpl := sites.Str(p.Search, "page")
	if tmpl == "" {
		return nil, fmt.Errorf("%s has no search page", p.Name)
	}
	f, err := e.fetcher(p, "search")
	if err != nil {
		return nil, err
	}
	page := strings.ReplaceAll(tmpl, "{query}", url.QueryEscape(query))
	if spec, ok := p.Search["json"].(map[string]any); ok {
		return e.searchJSON(ctx, f, p, page, spec)
	}
	doc, _, err := e.getDoc(ctx, f, page, "")
	if err != nil {
		return nil, err
	}
	base := doc.Url
	if base == nil {
		base, _ = url.Parse(page)
	}
	// Some sites answer an exact title with the novel's own page.
	if m, err := e.Registry.Match(base.String()); err == nil && m.Profile == p && m.Kind == sites.KindNovel && !sameURL(base.String(), page) {
		h := SearchHit{SiteID: p.ID, SiteName: p.Name, URL: m.URL, Language: lang.Norm(p.Language)}
		h.Title = sites.ParseSelector(sites.Str(p.Novel, "title")).First(doc.Selection)
		h.Author = cleanAuthor(sites.ParseSelector(sites.Str(p.Novel, "author")).First(doc.Selection))
		if c := sites.ParseSelector(sites.Str(p.Novel, "cover")).First(doc.Selection); c != "" {
			h.Cover = resolve(base, c)
		}
		if h.Title == "" {
			h.Title = titleFromURL(m.URL)
		}
		return []SearchHit{h}, nil
	}

	sel := func(k string) *sites.Selector { return sites.ParseSelector(sites.Str(p.Search, k)) }
	item, title, link := sel("item"), sel("title"), sel("link")
	cover, author, latest, chapters := sel("cover"), sel("author"), sel("latest"), sel("chapters")
	if item == nil || link == nil {
		return nil, fmt.Errorf("%s: search profile needs item and link selectors", p.Name)
	}

	var hits []SearchHit
	seen := map[string]bool{}
	item.Nodes(doc.Selection).EachWithBreak(func(_ int, n *goquery.Selection) bool {
		href := resolve(base, link.First(n))
		if href == "" {
			return true
		}
		// Keep only links this profile can download as a novel.
		m, err := e.Registry.Match(href)
		if err != nil || m.Profile != p || m.Kind != sites.KindNovel {
			return true
		}
		if seen[m.URL] {
			return true
		}
		seen[m.URL] = true
		h := SearchHit{
			SiteID: p.ID, SiteName: p.Name, URL: m.URL, Language: lang.Norm(p.Language),
			Title:  strings.TrimSpace(title.First(n)),
			Author: cleanAuthor(author.First(n)),
			Latest: strings.TrimSpace(latest.First(n)),
		}
		if c := cover.First(n); c != "" {
			h.Cover = resolve(base, c)
		}
		if v := strings.ReplaceAll(chapters.First(n), ",", ""); v != "" {
			h.Chapters, _ = strconv.Atoi(v)
		}
		if h.Title == "" {
			h.Title = titleFromURL(m.URL)
		}
		hits = append(hits, h)
		return len(hits) < maxHitsPerSite
	})
	return hits, nil
}

// searchJSON reads a search API that answers JSON (Wattpad). spec names the
// result array ("path") and each field's dotted path inside a result.
func (e *Engine) searchJSON(ctx context.Context, f fetch.Fetcher, p *sites.Profile, page string, spec map[string]any) ([]SearchHit, error) {
	resp, err := f.Get(ctx, page, "")
	if err != nil {
		return nil, err
	}
	var root any
	if err := json.Unmarshal(resp.Body, &root); err != nil {
		return nil, fmt.Errorf("%s search returned invalid JSON: %w", p.Name, err)
	}
	items, _ := jsonAt(root, sites.Str(spec, "path")).([]any)
	field := func(item any, k string) string {
		path := sites.Str(spec, k)
		if path == "" {
			return ""
		}
		// "https://ncode.syosetu.com/{ncode|lower}/": a template of fields.
		if strings.Contains(path, "{") {
			return jsonTemplate.ReplaceAllStringFunc(path, func(t string) string {
				name, lower := strings.CutSuffix(t[1:len(t)-1], "|lower")
				v := jsonString(jsonAt(item, name))
				if lower {
					v = strings.ToLower(v)
				}
				return v
			})
		}
		return strings.TrimSpace(jsonString(jsonAt(item, path)))
	}
	var hits []SearchHit
	seen := map[string]bool{}
	for _, it := range items {
		link := absoluteURL(page, field(it, "link"))
		m, err := e.Registry.Match(link)
		if err != nil || m.Profile != p || m.Kind != sites.KindNovel || seen[m.URL] {
			continue
		}
		seen[m.URL] = true
		h := SearchHit{SiteID: p.ID, SiteName: p.Name, URL: m.URL, Title: field(it, "title"), Language: lang.Norm(p.Language),
			Author: field(it, "author"), Cover: field(it, "cover"), Latest: field(it, "latest")}
		h.Chapters, _ = strconv.Atoi(field(it, "chapters"))
		if h.Title == "" {
			h.Title = titleFromURL(m.URL)
		}
		hits = append(hits, h)
		if len(hits) >= maxHitsPerSite {
			break
		}
	}
	return hits, nil
}

var jsonTemplate = regexp.MustCompile(`\{[A-Za-z0-9_.]+(\|lower)?\}`)

// cleanAuthor drops labels some sites put before the name.
func cleanAuthor(s string) string {
	s = strings.TrimSpace(s)
	for _, pre := range []string{"Author:", "Author :", "By "} {
		s = strings.TrimSpace(strings.TrimPrefix(s, pre))
	}
	return s
}
