package manga

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"novelarchiver/internal/sites"
)

// SearchRules say how to read a site's search page. Selectors use the same
// mini language as the rest of the profile and run inside each result.
type SearchRules struct {
	// URL is the search page: {query} is the URL-encoded title, {slug} the
	// title as a link slug ("solo-leveling"). Empty for API sites.
	URL      string `json:"url,omitempty"`
	Items    string `json:"items,omitempty"`    // one element per result
	Link     string `json:"link,omitempty"`     // the series link
	Title    string `json:"title,omitempty"`    // results without one are dropped
	Cover    string `json:"cover,omitempty"`    // cover image
	Author   string `json:"author,omitempty"`   // author(s)
	Type     string `json:"type,omitempty"`     // "Manga", "Manhwa", "Manhua" per result
	Latest   string `json:"latest,omitempty"`   // newest chapter's title
	Chapters string `json:"chapters,omitempty"` // a chapter count or the latest chapter's number
	// Skip drops results whose type matches this regex (MangaPill lists
	// novels too).
	Skip     string `json:"skip,omitempty"`
	Verified string `json:"verified,omitempty"`
}

// SearchHit is one series in a site's search results.
type SearchHit struct {
	SiteID   string `json:"siteId"`
	SiteName string `json:"siteName"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	Author   string `json:"author,omitempty"`
	Cover    string `json:"cover,omitempty"`
	// Type is "Manga", "Manhwa" or "Manhua" ("" when the site doesn't say
	// and carries more than one kind).
	Type     string `json:"type,omitempty"`
	Latest   string `json:"latest,omitempty"`
	Chapters int    `json:"chapters,omitempty"` // 0 = the results page doesn't say
	// ExistingSlug is set when this series is already in the manga library.
	ExistingSlug string `json:"existingSlug,omitempty"`
}

// maxSearchHits keeps one busy site from flooding the results.
const maxSearchHits = 12

// Searchable lists the sites whose search runs over plain HTTP.
func (r *Registry) Searchable() []*Profile {
	var out []*Profile
	for _, p := range r.Sites {
		if p.Search != nil && !p.IsBrowser() && (p.Search.URL != "" || p.API != "") {
			out = append(out, p)
		}
	}
	return out
}

// Search runs one site's search for a title.
func (e *Engine) Search(ctx context.Context, p *Profile, query string) ([]SearchHit, error) {
	query = strings.Join(strings.Fields(query), " ")
	if query == "" {
		return nil, errors.New("type a title to search for")
	}
	if p.Search == nil {
		return nil, fmt.Errorf("%s has no search", p.Name)
	}
	if p.API == "mangadex" {
		return e.mangadexSearch(ctx, p, query)
	}
	if p.Search.URL == "" {
		return nil, fmt.Errorf("%s has no search page", p.Name)
	}
	page := strings.NewReplacer("{query}", url.QueryEscape(query), "{slug}", titleSlug(query)).Replace(p.Search.URL)
	pg, err := e.get(ctx, p, Request{URL: page})
	if err != nil {
		return nil, err
	}
	doc, err := parseDoc(pg)
	if err != nil {
		return nil, err
	}
	return e.Sites.parseSearch(p, doc)
}

var slugBreak = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// titleSlug turns "Solo Leveling: Ragnarok" into "solo-leveling-ragnarok".
func titleSlug(q string) string {
	return strings.Trim(slugBreak.ReplaceAllString(strings.ToLower(q), "-"), "-")
}

// parseSearch reads a search results page. Only links the profile itself
// recognises as a series page are kept, so every hit can be downloaded.
func (r *Registry) parseSearch(p *Profile, doc *goquery.Document) ([]SearchHit, error) {
	rule := p.Search
	sel := sites.ParseSelector
	items, link := sel(rule.Items), sel(rule.Link)
	if items == nil || link == nil {
		return nil, fmt.Errorf("%s: search needs items and link selectors", p.Name)
	}
	title, cover, author := sel(rule.Title), sel(rule.Cover), sel(rule.Author)
	kind, latest, chapters := sel(rule.Type), sel(rule.Latest), sel(rule.Chapters)
	var skip *regexp.Regexp
	if rule.Skip != "" {
		var err error
		if skip, err = regexp.Compile(rule.Skip); err != nil {
			return nil, fmt.Errorf("%s: search skip: %w", p.Name, err)
		}
	}
	var hits []SearchHit
	seen := map[string]bool{}
	items.Nodes(doc.Selection).EachWithBreak(func(_ int, n *goquery.Selection) bool {
		href := resolve(doc.Url, link.First(n))
		if href == "" {
			return true
		}
		m, err := r.Match(href)
		if err != nil || m.Profile != p || m.Chapter || seen[m.URL] {
			return true
		}
		rawType := squash(kind.First(n))
		if skip != nil && rawType != "" && skip.MatchString(rawType) {
			return true
		}
		seen[m.URL] = true
		h := SearchHit{SiteID: p.ID, SiteName: p.Name, URL: m.URL,
			Title:  clip(badge.ReplaceAllString(squash(title.First(n)), ""), 200),
			Author: clip(squash(author.First(n)), 200),
			Type:   searchType(rawType, p.Type),
			Latest: squash(latest.First(n)),
		}
		if h.Title == "" {
			return true
		}
		if c := cover.First(n); c != "" {
			h.Cover = resolve(doc.Url, c)
		}
		if v := strings.ReplaceAll(chapters.First(n), ",", ""); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				h.Chapters = int(f)
			}
		}
		hits = append(hits, h)
		return len(hits) < maxSearchHits
	})
	return hits, nil
}

var kindWord = regexp.MustCompile(`(?i)\b(manhwa|manhua|manga|webtoon)\b`)

// searchType names a result's kind: from the result itself when the page
// says, else from the site when it carries one kind only.
func searchType(raw, site string) string {
	if m := kindWord.FindString(raw); m != "" {
		if strings.EqualFold(m, "webtoon") {
			return "Manhwa"
		}
		return strings.ToUpper(m[:1]) + strings.ToLower(m[1:])
	}
	if m := kindWord.FindAllString(site, -1); len(m) == 1 {
		return strings.ToUpper(m[0][:1]) + strings.ToLower(m[0][1:])
	}
	return ""
}
