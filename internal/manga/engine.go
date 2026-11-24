package manga

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"novelarchiver/internal/fetch"
	"novelarchiver/internal/sites"
)

// Series is what discovery returns.
type Series struct {
	SiteID      string            `json:"siteId"`
	SiteName    string            `json:"siteName"`
	Type        string            `json:"type"`
	URL         string            `json:"url"`
	Title       string            `json:"title"`
	Author      string            `json:"author"`
	Cover       string            `json:"cover"`
	Description string            `json:"description"`
	Status      string            `json:"status,omitempty"`
	Chapters    []ChapterRef      `json:"chapters"`
	Vars        map[string]string `json:"vars,omitempty"`
	Warnings    []string          `json:"warnings,omitempty"`
	// InputChapter is the chapter number the user pasted (0 = series link).
	InputChapter int `json:"inputChapter,omitempty"`
}

// ChapterRef is one chapter in reading order.
type ChapterRef struct {
	Number int    `json:"number"` // 1-based position, oldest first
	Title  string `json:"title"`
	URL    string `json:"url"`
	ID     string `json:"id,omitempty"` // API id (MangaDex)
}

// ErrNoPages means a chapter page loaded but had no images.
var ErrNoPages = errors.New("no page images found in this chapter")

// Engine discovers series and finds page images.
type Engine struct {
	Sites   *Registry
	HTTP    *Client       // pages and APIs
	Images  *Client       // image files
	Browser fetch.Fetcher // site browser for Cloudflare-protected sites; may be nil
}

// StatusFunc receives short progress messages.
type StatusFunc func(string)

// get loads a page of a site: through the site browser for browser sites,
// otherwise over HTTP, falling back to the browser if HTTP is blocked.
func (e *Engine) get(ctx context.Context, p *Profile, r Request) (*fetch.Page, error) {
	if p.IsBrowser() && e.Browser != nil && r.Method != "POST" {
		return e.Browser.Get(ctx, r.URL, r.Referer)
	}
	if r.Header == nil {
		r.Header = p.Headers
	}
	pg, err := e.HTTP.Do(ctx, r)
	if errors.Is(err, fetch.ErrBlocked) {
		if e.Browser != nil && r.Method != "POST" {
			return e.Browser.Get(ctx, r.URL, r.Referer)
		}
		return nil, fmt.Errorf("%s asked for a 'verify you are human' check: %w", p.Name, err)
	}
	return pg, err
}

// Fetch loads any page of a supported site the way the engine would.
func (e *Engine) Fetch(ctx context.Context, link string) (*fetch.Page, error) {
	p := e.Sites.ByHost(link)
	if p == nil {
		return nil, fmt.Errorf("%s isn't a supported site", link)
	}
	return e.get(ctx, p, Request{URL: link})
}

func parseDoc(pg *fetch.Page) (*goquery.Document, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(pg.Body))
	if err != nil {
		return nil, err
	}
	doc.Url, _ = url.Parse(pg.URL)
	return doc, nil
}

// Discover reads a series (or chapter) link into metadata and a chapter list.
func (e *Engine) Discover(ctx context.Context, link string, status StatusFunc) (*Series, error) {
	if status == nil {
		status = func(string) {}
	}
	m, err := e.Sites.Match(link)
	if err != nil {
		return nil, err
	}
	p := m.Profile
	if p.API == "mangadex" {
		return e.mangadexDiscover(ctx, p, m, status)
	}
	vars := map[string]string{"origin": origin(m.URL)}
	for k, v := range m.Captures {
		vars[k] = v
	}
	seriesURL := m.URL
	if m.Chapter {
		status("That's a chapter link; finding the series page…")
		if seriesURL, err = e.seriesFromChapter(ctx, p, m, vars); err != nil {
			return nil, err
		}
	}
	status("Reading series details from " + p.Name + "…")
	pg, err := e.get(ctx, p, Request{URL: seriesURL})
	if err != nil {
		return nil, err
	}
	doc, err := parseDoc(pg)
	if err != nil {
		return nil, err
	}
	s := &Series{SiteID: p.ID, SiteName: p.Name, Type: p.Type, URL: pg.URL, Vars: vars}
	vars["seriesUrl"] = strings.TrimRight(pg.URL, "/")
	for k, expr := range p.Vars {
		if v := readValue(doc, pg.Body, expr); v != "" {
			vars[k] = v
		}
	}
	e.readMeta(p, doc, s)

	status("Building the chapter list…")
	if err := e.readChapters(ctx, p, s, doc, pg); err != nil {
		return nil, err
	}
	if len(s.Chapters) == 0 {
		return nil, fmt.Errorf("no chapters found on %s for this series", p.Name)
	}
	if m.Chapter {
		for _, c := range s.Chapters {
			if sameURL(c.URL, m.URL) {
				s.InputChapter = c.Number
			}
		}
	}
	if p.Status == "human-check" {
		s.Warnings = append(s.Warnings, p.Name+" sometimes shows a 'Verify you are human' check. If it does, the site window pops up: tick the box once and downloading carries on.")
	}
	return s, nil
}

func (e *Engine) seriesFromChapter(ctx context.Context, p *Profile, m *Match, vars map[string]string) (string, error) {
	rule := p.SeriesFromChapter
	if strings.Contains(rule, "{") {
		return expand(rule, vars), nil
	}
	pg, err := e.get(ctx, p, Request{URL: m.URL})
	if err != nil {
		return "", err
	}
	doc, err := parseDoc(pg)
	if err != nil {
		return "", err
	}
	if href := sites.ParseSelector(rule).First(doc.Selection); href != "" {
		return resolve(doc.Url, href), nil
	}
	return "", fmt.Errorf("couldn't find the series page from that chapter. Paste the series link instead")
}

// readValue evaluates "re:<regex>" against the raw page, or a selector.
func readValue(doc *goquery.Document, body []byte, expr string) string {
	if strings.HasPrefix(expr, "re:") {
		if m := regexp.MustCompile(expr[3:]).FindSubmatch(body); len(m) > 1 {
			return string(m[1])
		}
		return ""
	}
	return sites.ParseSelector(expr).First(doc.Selection)
}

var (
	badge      = regexp.MustCompile(`(\s+(END|HOT|NEW|COMPLETED|UP))+$`)
	authorJunk = regexp.MustCompile(`(?i)\s*author info\s*`)
)

var titleJunk = regexp.MustCompile(`(?i)^\s*read\s+|\s+(manga|manhwa|manhua|comic)?\s*(online)?\s*(for free|free)?\s*([|–—-].*)?$`)

func (e *Engine) readMeta(p *Profile, doc *goquery.Document, s *Series) {
	get := func(key, fallback string) string {
		if expr := p.Series[key]; expr != "" {
			if v := sites.ParseSelector(expr).First(doc.Selection); v != "" {
				return v
			}
		}
		if fallback == "" {
			return ""
		}
		return sites.ParseSelector(fallback).First(doc.Selection)
	}
	s.Title = get("title", "meta[property='og:title']@content, h1")
	if p.Series["title"] == "" {
		s.Title = strings.TrimSpace(titleJunk.ReplaceAllString(s.Title, ""))
	}
	s.Author = get("author", "")
	s.Status = get("status", "")
	s.Description = get("description", "meta[property='og:description']@content, meta[name='description']@content")
	if c := get("cover", "meta[property='og:image']@content"); c != "" {
		s.Cover = resolve(doc.Url, c)
	}
	s.Title = clip(badge.ReplaceAllString(s.Title, ""), 200)
	s.Author = clip(strings.TrimSpace(authorJunk.ReplaceAllString(s.Author, "")), 200)
	s.Description = clip(s.Description, 4000)
	if s.Title == "" {
		s.Title = "Untitled"
	}
}

func (e *Engine) readChapters(ctx context.Context, p *Profile, s *Series, doc *goquery.Document, pg *fetch.Page) error {
	r := p.Chapters
	body := pg.Body
	if r.URL != "" {
		lp, err := e.get(ctx, p, Request{URL: expand(r.URL, s.Vars), Method: r.Method, Body: expand(r.Body, s.Vars), Referer: s.URL,
			Header: mergeHeaders(p.Headers, map[string]string{"X-Requested-With": "XMLHttpRequest"})})
		if err != nil {
			return fmt.Errorf("chapter list: %w", err)
		}
		if doc, err = parseDoc(lp); err != nil {
			return err
		}
		body = lp.Body
	}
	var refs []ChapterRef
	if r.JSON != nil {
		var root any
		if r.JSON.Script != "" {
			body = []byte(doc.Find(r.JSON.Script).First().Text())
		} else {
			body = unwrapJSON(body, doc)
		}
		if err := json.Unmarshal(body, &root); err != nil {
			return fmt.Errorf("chapter list isn't valid JSON: %w", err)
		}
		arr, _ := walk(root, r.JSON.Path).([]any)
		base, _ := url.Parse(expand(r.JSON.Base, s.Vars))
		for _, it := range arr {
			obj, _ := it.(map[string]any)
			href := fieldOrTemplate(obj, r.JSON.URL, s.Vars)
			if href == "" {
				continue
			}
			refs = append(refs, ChapterRef{Title: fieldOrTemplate(obj, r.JSON.Title, s.Vars), URL: resolve(base, href)})
		}
	} else {
		var titleSel *sites.Selector
		if r.Title != "" {
			titleSel = sites.ParseSelector(r.Title)
		}
		sites.ParseSelector(expand(r.Items, s.Vars)).Nodes(doc.Selection).Each(func(_ int, a *goquery.Selection) {
			href, _ := a.Attr("href")
			if href == "" {
				href, _ = a.Find("a").First().Attr("href")
			}
			if href = resolve(doc.Url, href); href == "" {
				return
			}
			if i := strings.IndexByte(href, '#'); i >= 0 {
				href = href[:i] // "chapter/1#resume" is chapter 1
			}
			title := ""
			if titleSel != nil {
				title = titleSel.First(a)
			}
			if title == "" {
				title = squash(a.Text())
			}
			if title == "" {
				title, _ = a.Attr("title")
			}
			refs = append(refs, ChapterRef{Title: squash(title), URL: href})
		})
	}
	if r.Order == "newest-first" {
		for i, j := 0, len(refs)-1; i < j; i, j = i+1, j-1 {
			refs[i], refs[j] = refs[j], refs[i]
		}
	}
	s.Chapters = numberChapters(refs)
	return nil
}

func numberChapters(refs []ChapterRef) []ChapterRef {
	seen := map[string]bool{}
	out := make([]ChapterRef, 0, len(refs))
	for _, r := range refs {
		key := r.URL
		if r.ID != "" {
			key = r.ID
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		r.Number = len(out) + 1
		r.Title = clip(tidyTitle(r.Title), 160)
		if r.Title == "" {
			r.Title = fmt.Sprintf("Chapter %d", r.Number)
		}
		out = append(out, r)
	}
	return out
}

// Pages finds the image links of one chapter, in reading order. referer is
// what image requests should send.
func (e *Engine) Pages(ctx context.Context, siteID string, vars map[string]string, ch ChapterRef) (imgs []string, referer string, err error) {
	p := e.Sites.ByID(siteID)
	if p == nil {
		return nil, "", fmt.Errorf("unknown site %q", siteID)
	}
	if p.API == "mangadex" {
		imgs, err := e.mangadexPages(ctx, p, ch)
		return imgs, "https://mangadex.org/", err
	}
	r := p.Pages
	v := map[string]string{}
	for k, val := range vars {
		v[k] = val
	}
	v["chapterUrl"] = ch.URL
	for k, val := range captures(p.chapterRe, ch.URL) {
		v[k] = val
	}
	referer = ch.URL
	if p.ImageReferer == "origin" {
		referer = origin(ch.URL) + "/"
	}
	pg, err := e.get(ctx, p, Request{URL: ch.URL, Referer: vars["seriesUrl"]})
	if err != nil {
		return nil, referer, err
	}
	doc, err := parseDoc(pg)
	if err != nil {
		return nil, referer, err
	}
	for k, expr := range r.Vars {
		if val := readValue(doc, pg.Body, expr); val != "" {
			v[k] = val
		}
	}
	body := pg.Body
	if r.URL != "" {
		u := expand(r.URL, v)
		if placeholder.MatchString(u) {
			return nil, referer, fmt.Errorf("couldn't read the chapter's image list id on %s", p.Name)
		}
		ip, err := e.get(ctx, p, Request{URL: u, Referer: ch.URL, Header: mergeHeaders(p.Headers, map[string]string{"X-Requested-With": "XMLHttpRequest"})})
		if err != nil {
			return nil, referer, fmt.Errorf("image list: %w", err)
		}
		body = ip.Body
		if doc, err = parseDoc(ip); err != nil {
			return nil, referer, err
		}
		doc.Url, _ = url.Parse(ch.URL)
	}
	if r.JSONField != "" {
		var obj map[string]any
		if err := json.Unmarshal(body, &obj); err != nil {
			return nil, referer, fmt.Errorf("image list isn't valid JSON: %w", err)
		}
		html := field(obj, r.JSONField)
		body = []byte(html)
		if doc, err = goquery.NewDocumentFromReader(strings.NewReader(html)); err != nil {
			return nil, referer, err
		}
		doc.Url, _ = url.Parse(ch.URL)
	}
	switch {
	case r.Script:
		imgs = scriptArray(string(body))
	case r.Regex != "":
		for _, m := range regexp.MustCompile(expand(r.Regex, v)).FindAllStringSubmatch(string(body), -1) {
			imgs = append(imgs, m[len(m)-1])
		}
	}
	if len(imgs) == 0 && r.Images != "" {
		imgs = selectImages(doc, r.Images, r.SortAttr)
	}
	imgs = cleanImages(doc.Url, imgs, r.Exclude)
	if len(imgs) == 0 {
		return nil, referer, ErrNoPages
	}
	return imgs, referer, nil
}

// selectImages reads image links from matching elements. When an element
// isn't an <img> (or lacks the attribute) its first <img> is used.
func selectImages(doc *goquery.Document, expr, sortAttr string) []string {
	sel := sites.ParseSelector(expr)
	attrs := sel.AttrOf()
	if len(attrs) == 0 {
		attrs = []string{"data-src", "data-lazy-src", "data-original", "data-url", "src"}
	}
	type item struct {
		key int
		pos int
		src string
	}
	var items []item
	sel.Nodes(doc.Selection).Each(func(i int, el *goquery.Selection) {
		src := firstAttr(el, attrs)
		if src == "" {
			src = firstAttr(el.Find("img").First(), attrs)
		}
		if src == "" {
			return
		}
		k := i
		if sortAttr != "" {
			if v, ok := el.Closest("[" + sortAttr + "]").Attr(sortAttr); ok {
				if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
					k = n
				}
			}
		}
		items = append(items, item{k, i, src})
	})
	if sortAttr != "" {
		sort.SliceStable(items, func(a, b int) bool { return items[a].key < items[b].key })
	}
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.src
	}
	return out
}

func firstAttr(el *goquery.Selection, attrs []string) string {
	for _, a := range attrs {
		if v, ok := el.Attr(a); ok {
			if v = strings.TrimSpace(v); v != "" && !strings.HasPrefix(v, "data:") && !defaultExclude.MatchString(v) {
				return v
			}
		}
	}
	return ""
}

var (
	jsArray  = regexp.MustCompile(`\[\s*(?:['"]https?:[^'"\]]+['"]\s*,?\s*)+\]`)
	jsString = regexp.MustCompile(`['"](https?:[^'"]+)['"]`)
)

// scriptArray returns the longest JavaScript array of links on the page
// (MangaKatana keeps its page list in a script and adds a short decoy).
func scriptArray(body string) []string {
	var best []string
	for _, arr := range jsArray.FindAllString(body, -1) {
		var links []string
		for _, m := range jsString.FindAllStringSubmatch(arr, -1) {
			links = append(links, m[1])
		}
		if len(links) > len(best) {
			best = links
		}
	}
	return best
}

var defaultExclude = regexp.MustCompile(`(?i)/loading[^/]*\.gif|placeholder|bg_transparency|/lazy[^/]*\.(gif|png|svg)|blank\.gif|pixel\.quantserve`)

func cleanImages(base *url.URL, in []string, exclude string) []string {
	var ex *regexp.Regexp
	if exclude != "" {
		ex = regexp.MustCompile(exclude)
	}
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(strings.ReplaceAll(s, `\/`, `/`))
		s = resolve(base, s)
		if s == "" || seen[s] || defaultExclude.MatchString(s) || (ex != nil && ex.MatchString(s)) {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

var (
	fieldRef  = regexp.MustCompile(`\{([a-zA-Z0-9_.]+)\}`)
	zeroFrac  = regexp.MustCompile(`^(\d+)\.0+$`)
	bareNum   = regexp.MustCompile(`(?i)^(\d+)(?:[.-](\d+))?(?:-eng(?:-li)?)?$`)
	spaceDash = regexp.MustCompile(`\s+[-:–]\s*$`)
)

// fieldOrTemplate reads a field name ("a|b") or fills a "{field}" template
// from a JSON object ("Chapter {chapter} {title}"); names the object lacks
// come from the series values (vars).
func fieldOrTemplate(obj map[string]any, spec string, vars map[string]string) string {
	if !strings.Contains(spec, "{") {
		return field(obj, spec)
	}
	out := fieldRef.ReplaceAllStringFunc(spec, func(m string) string {
		name := m[1 : len(m)-1]
		v := field(obj, name)
		if v == "" {
			v = vars[name]
		}
		if z := zeroFrac.FindStringSubmatch(v); z != nil {
			v = z[1] // "225.00" -> "225"
		}
		return v
	})
	return strings.TrimSpace(spaceDash.ReplaceAllString(squash(out), ""))
}

// unwrapJSON returns JSON that a browser showed as a page (<pre>{…}</pre>).
func unwrapJSON(body []byte, doc *goquery.Document) []byte {
	t := bytes.TrimSpace(body)
	if len(t) == 0 || t[0] == '{' || t[0] == '[' || doc == nil {
		return body
	}
	if pre := strings.TrimSpace(doc.Find("pre").First().Text()); pre != "" {
		return []byte(pre)
	}
	return []byte(strings.TrimSpace(doc.Find("body").Text()))
}

// tidyTitle turns bare numbers ("15", "15-eng-li", "15-5") into
// "Chapter 15" / "Chapter 15.5".
func tidyTitle(t string) string {
	t = squash(t)
	if m := bareNum.FindStringSubmatch(t); m != nil {
		if m[2] != "" {
			return "Chapter " + m[1] + "." + m[2]
		}
		return "Chapter " + m[1]
	}
	return t
}

// ---- small helpers ----

func walk(v any, path string) any {
	if path == "" {
		return v
	}
	for _, k := range strings.Split(path, ".") {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[k]
	}
	return v
}

// field reads "a|b" (first non-empty) from a JSON object as a string.
func field(obj map[string]any, keys string) string {
	for _, k := range strings.Split(keys, "|") {
		switch t := walk(obj, strings.TrimSpace(k)).(type) {
		case string:
			if strings.TrimSpace(t) != "" {
				return t
			}
		case float64:
			return strconv.FormatFloat(t, 'f', -1, 64)
		}
	}
	return ""
}

func mergeHeaders(a, b map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

func resolve(base *url.URL, href string) string {
	href = strings.TrimSpace(href)
	if href == "" || href == "#" || strings.HasPrefix(href, "javascript:") || strings.HasPrefix(href, "data:") {
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
		return ref.String()
	}
	return base.ResolveReference(ref).String()
}

func origin(u string) string {
	p, err := url.Parse(u)
	if err != nil {
		return ""
	}
	return p.Scheme + "://" + p.Host
}

func sameURL(a, b string) bool {
	n := func(s string) string {
		s = strings.TrimPrefix(strings.TrimPrefix(s, "https://"), "http://")
		return strings.TrimRight(strings.TrimPrefix(s, "www."), "/")
	}
	return n(a) == n(b)
}

func squash(s string) string { return strings.Join(strings.Fields(s), " ") }

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		return strings.TrimSpace(string(r[:n-1])) + "…"
	}
	return s
}
