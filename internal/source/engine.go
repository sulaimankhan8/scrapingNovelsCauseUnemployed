package source

import (
	"bytes"
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
	"novelarchiver/internal/sites"
)

// Engine runs discovery and chapter fetching for every site profile.
type Engine struct {
	Registry *sites.Registry
	HTTP     fetch.Fetcher
	Browser  fetch.Fetcher // nil until a browser fetcher is wired in
}

// StatusFunc receives human-readable progress messages.
type StatusFunc func(msg string)

func (e *Engine) fetcher(p *sites.Profile, stage string) (fetch.Fetcher, error) {
	if p != nil && p.NeedsBrowser(stage) {
		if e.Browser == nil {
			return nil, fmt.Errorf("%s: %w", p.Name, fetch.ErrNeedsBrowser)
		}
		return e.Browser, nil
	}
	return e.HTTP, nil
}

func (e *Engine) getDoc(ctx context.Context, f fetch.Fetcher, u, referer string) (*goquery.Document, *fetch.Page, error) {
	page, err := f.Get(ctx, u, referer)
	if err != nil {
		return nil, nil, err
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(page.Body))
	if err != nil {
		return nil, nil, err
	}
	if base, err := url.Parse(page.URL); err == nil {
		doc.Url = base
	}
	return doc, page, nil
}

// Discover resolves a pasted novel or chapter link into metadata and a TOC.
func (e *Engine) Discover(ctx context.Context, input string, status StatusFunc) (*Novel, error) {
	if status == nil {
		status = func(string) {}
	}
	m, err := e.Registry.Match(input)
	if err != nil {
		return nil, err
	}
	p := m.Profile
	n := &Novel{InputKind: string(m.Kind), Vars: map[string]string{}}
	for k, v := range m.Captures {
		n.Vars[k] = v
	}
	n.Vars["origin"] = m.Origin()
	if p != nil {
		n.SiteID, n.SiteName = p.ID, p.Name
		if p.NeedsBrowser("chapter") && e.Browser == nil {
			n.Warnings = append(n.Warnings, p.Name+" chapters only load in a real browser. In-app browser mode is still being built, so chapter downloads from this site will fail for now.")
		}
		if p.Status == "human-check" {
			n.Warnings = append(n.Warnings, p.Name+" may show a 'Verify you are human' check. If it does, the site browser window pops up so you can tick it once; it's remembered after that.")
		}
	} else {
		n.SiteID, n.SiteName = "generic", hostOf(m.URL)
	}

	// 1. Work out the novel page URL.
	novelURL := m.URL
	var chapterDoc, novelDoc *goquery.Document
	if m.Kind == sites.KindChapter {
		status("Link is a chapter page; finding the novel page…")
		n.FirstChapterURL = m.URL
		novelURL, chapterDoc, err = e.novelFromChapter(ctx, p, m)
		if err != nil {
			return nil, err
		}
	} else if m.Kind == sites.KindUnknown && p == nil {
		// Unknown site: the link could be either kind. Treat it as a novel
		// page; if no TOC is found we crawl from it as a chapter.
		n.FirstChapterURL = m.URL
	}
	n.URL = novelURL
	n.Vars["novelUrl"] = strings.TrimRight(novelURL, "/")

	// 2. Metadata.
	if novelURL != "" {
		status("Reading novel details…")
		novelDoc, err = e.readMetadata(ctx, p, n)
		if err != nil {
			if chapterDoc == nil {
				return nil, err
			}
			n.Warnings = append(n.Warnings, "Couldn't read the novel page ("+err.Error()+"); downloading by following chapter links.")
		}
	}

	// 3. Table of contents.
	status("Building the chapter list…")
	if err := e.readTOC(ctx, p, n, novelDoc); err != nil {
		if n.FirstChapterURL == "" {
			return nil, err
		}
		n.Warnings = append(n.Warnings, "Couldn't load the full chapter list ("+err.Error()+"); chapters will be found by following 'Next' links.")
		n.Chapters = nil
	}
	if len(n.Chapters) == 0 {
		if n.FirstChapterURL == "" {
			return nil, fmt.Errorf("no chapters found for %s", m.URL)
		}
		n.TOCMode = "crawl"
	} else {
		n.TOCMode = "list"
	}

	if n.Title == "" {
		n.Title = titleFromURL(m.URL)
	}
	// Mark which chapter the user pasted, so the UI can offer "from here".
	if m.Kind == sites.KindChapter {
		for _, c := range n.Chapters {
			if sameURL(c.URL, m.URL) {
				n.InputChapter = c.Number
				break
			}
		}
		if n.InputChapter == 0 {
			if num, err := strconv.Atoi(m.Captures["num"]); err == nil {
				n.InputChapter = num
			}
		}
	}
	locked := 0
	for _, c := range n.Chapters {
		if c.Locked {
			locked++
		}
	}
	if locked > 0 {
		n.Warnings = append(n.Warnings, fmt.Sprintf("%d chapters are paid or locked on %s and will be skipped.", locked, n.SiteName))
	}
	return n, nil
}

func (e *Engine) novelFromChapter(ctx context.Context, p *sites.Profile, m *sites.Match) (string, *goquery.Document, error) {
	rule := ""
	if p != nil {
		rule = p.Input.NovelFromChapter
	}
	// "url ~ regex": rewrite the chapter URL.
	if strings.HasPrefix(rule, "url ~ ") {
		re := regexp.MustCompile(strings.TrimPrefix(rule, "url ~ "))
		if sm := re.FindStringSubmatch(m.URL); len(sm) > 1 {
			return sm[1], nil, nil
		}
	}
	// "slug -> template" / "id -> template".
	if i := strings.Index(rule, " -> "); i >= 0 {
		vars := copyVars(m.Captures)
		vars["origin"] = m.Origin()
		return sites.Expand(strings.TrimSpace(rule[i+4:]), vars), nil, nil
	}
	// Otherwise a selector on the chapter page (or generic breadcrumb guess).
	f, err := e.fetcher(p, "chapter")
	if err != nil {
		return "", nil, err
	}
	doc, _, err := e.getDoc(ctx, f, m.URL, "")
	if err != nil {
		return "", nil, err
	}
	var href string
	if rule != "" {
		href = sites.ParseSelector(rule).First(doc.Selection)
	}
	if href == "" {
		href = guessNovelLink(doc, m.URL)
	}
	if href == "" {
		return "", doc, nil // crawl-only
	}
	return resolve(doc.Url, href), doc, nil
}

// readMetadata fills title/author/cover/description and profile vars.
func (e *Engine) readMetadata(ctx context.Context, p *sites.Profile, n *Novel) (*goquery.Document, error) {
	var sec map[string]any
	if p != nil {
		sec = p.Novel
	}
	// API-backed metadata (NovelBin).
	if api := sites.Str(sec, "api"); api != "" {
		f, err := e.fetcher(p, "novel")
		if err != nil {
			return nil, err
		}
		page, err := f.Get(ctx, sites.Expand(api, n.Vars), n.URL)
		if err != nil {
			return nil, err
		}
		var obj map[string]any
		if err := json.Unmarshal(page.Body, &obj); err != nil {
			return nil, fmt.Errorf("novel API returned invalid JSON: %w", err)
		}
		fields, _ := sec["fields"].(map[string]any)
		get := func(k string) string { // "title|name" tries keys in order
			keys, _ := fields[k].(string)
			for _, key := range strings.Split(keys, "|") {
				if v := jsonString(obj[strings.TrimSpace(key)]); v != "" {
					return v
				}
			}
			return ""
		}
		n.Vars["id"] = get("id")
		n.Title, n.Author, n.Cover, n.Description = get("title"), get("author"), get("cover"), get("description")
		n.Cover = absoluteURL(n.URL, n.Cover)
		return nil, nil
	}

	f, err := e.fetcher(p, "novel")
	if err != nil {
		return nil, err
	}
	doc, page, err := e.getDoc(ctx, f, n.URL, "")
	if err != nil {
		return nil, err
	}
	n.URL = page.URL
	n.Vars["novelUrl"] = strings.TrimRight(page.URL, "/")
	n.Vars["novelPage"] = "fetched"

	special := map[string]bool{"api": true, "fields": true, "jsonLd": true, "strategy": true, "fetchMode": true}
	for key, v := range sec {
		expr, ok := v.(string)
		if !ok || special[key] {
			continue
		}
		val := sites.ParseSelector(sites.Expand(expr, n.Vars)).First(doc.Selection)
		if val == "" {
			continue
		}
		switch key {
		case "title":
			n.Title = val
		case "author":
			n.Author = val
		case "cover":
			n.Cover = resolve(doc.Url, val)
		case "description":
			n.Description = descriptionText(doc, sites.ParseSelector(expr), val)
		case "status":
			n.Status = val
		case "firstChapter", "latestChapterUrl":
			n.Vars[key] = resolve(doc.Url, val)
		default:
			n.Vars[key] = val
		}
	}
	if fc := n.Vars["firstChapter"]; fc != "" && n.FirstChapterURL == "" {
		n.FirstChapterURL = fc
	}
	genericMetadata(doc, n)
	return doc, nil
}

// readTOC builds n.Chapters according to toc.strategy.
func (e *Engine) readTOC(ctx context.Context, p *sites.Profile, n *Novel, novelDoc *goquery.Document) error {
	var sec map[string]any
	strategy := "generic"
	if p != nil {
		sec = p.TOC
		if s := sites.Str(sec, "strategy"); s != "" {
			strategy = s
		}
	}
	f, err := e.fetcher(p, "toc")
	if err != nil {
		return err
	}
	items := sites.ParseSelector(sites.Str(sec, "items"))
	lockSel := sites.Str(sec, "lockedMarker")
	var refs []ChapterRef

	collect := func(doc *goquery.Document) {
		if items == nil {
			return
		}
		items.Nodes(doc.Selection).Each(func(_ int, a *goquery.Selection) {
			href, _ := a.Attr("href")
			if href == "" {
				return
			}
			title, _ := a.Attr("title")
			if strings.TrimSpace(title) == "" {
				title = strings.Join(strings.Fields(a.Text()), " ")
			}
			locked := lockSel != "" && a.Find(lockSel).Length() > 0
			refs = append(refs, ChapterRef{Title: strings.TrimSpace(title), URL: resolve(doc.Url, href), Locked: locked})
		})
	}

	switch strategy {
	case "inline":
		doc := novelDoc
		if pu := sites.Str(sec, "pageUrl"); pu != "" || doc == nil {
			target := n.URL
			if pu != "" {
				target = sites.Expand(pu, n.Vars)
			}
			if doc, _, err = e.getDoc(ctx, f, target, n.URL); err != nil {
				return err
			}
		}
		collect(doc)

	case "paginated":
		pageURL := sites.Str(sec, "pageUrl")
		lastSel := sites.ParseSelector(sites.Str(sec, "lastPage"))
		vars := copyVars(n.Vars)
		vars["n"] = "1"
		first, _, err := e.getDoc(ctx, f, sites.Expand(pageURL, vars), n.URL)
		if err != nil {
			return err
		}
		collect(first)
		last, _ := strconv.Atoi(lastSel.First(first.Selection))
		if last == 0 && novelDoc != nil {
			last, _ = strconv.Atoi(lastSel.First(novelDoc.Selection))
		}
		for pg := 2; pg <= last; pg++ {
			vars["n"] = strconv.Itoa(pg)
			doc, _, err := e.getDoc(ctx, f, sites.Expand(pageURL, vars), n.URL)
			if err != nil {
				return fmt.Errorf("chapter list page %d: %w", pg, err)
			}
			collect(doc)
		}

	case "ajax":
		u := sites.Str(sec, "url")
		if u == "" {
			return fmt.Errorf("no chapter list URL for this site yet")
		}
		doc, _, err := e.getDoc(ctx, f, sites.Expand(u, n.Vars), n.URL)
		if err != nil {
			return err
		}
		collect(doc)

	case "api":
		page, err := f.Get(ctx, sites.Expand(sites.Str(sec, "api"), n.Vars), n.URL)
		if err != nil {
			return err
		}
		var arr []map[string]any
		if err := json.Unmarshal(page.Body, &arr); err != nil {
			return fmt.Errorf("chapter API returned invalid JSON: %w", err)
		}
		for i, c := range arr {
			title := ""
			for _, k := range strings.Split(sites.Str(sec, "titleField"), "|") {
				if title = jsonString(c[k]); title != "" {
					break
				}
			}
			idx := i
			if v, ok := c["index"].(float64); ok {
				idx = int(v)
			}
			refs = append(refs, ChapterRef{Title: title, Index: idx})
		}

	case "sequential":
		to := evalVarRegex(sites.Str(sec, "to"), n.Vars)
		from, _ := sec["from"].(float64)
		if from == 0 {
			from = 1
		}
		if to <= 0 {
			return fmt.Errorf("couldn't find the latest chapter number")
		}
		vars := copyVars(n.Vars)
		for i := int(from); i <= to; i++ {
			vars["n"] = strconv.Itoa(i)
			refs = append(refs, ChapterRef{Title: fmt.Sprintf("Chapter %d", i), URL: sites.Expand(sites.Str(sec, "pattern"), vars)})
		}

	case "next-link":
		// Discovered during download.
		if n.FirstChapterURL == "" {
			return fmt.Errorf("no first chapter link found")
		}
		return nil

	default: // generic
		doc := novelDoc
		if doc == nil && n.URL != "" {
			if doc, _, err = e.getDoc(ctx, f, n.URL, ""); err != nil {
				return err
			}
		}
		if doc != nil {
			refs = genericTOC(doc)
		}
	}

	n.Chapters = numberRefs(refs)
	return nil
}

// FetchChapter downloads one chapter's content (not yet sanitised).
func (e *Engine) FetchChapter(ctx context.Context, n *Novel, ref ChapterRef) (*Chapter, error) {
	p := e.profile(n.SiteID)
	var sec map[string]any
	if p != nil {
		sec = p.Chapter
	}
	if ref.Locked {
		return nil, ErrLocked
	}
	f, err := e.fetcher(p, "chapter")
	if err != nil {
		return nil, err
	}

	if sites.Str(sec, "strategy") == "api" {
		vars := copyVars(n.Vars)
		vars["index"] = strconv.Itoa(ref.Index)
		page, err := f.Get(ctx, sites.Expand(sites.Str(sec, "api"), vars), n.URL)
		if err != nil {
			return nil, err
		}
		var obj map[string]any
		if err := json.Unmarshal(page.Body, &obj); err != nil {
			return nil, fmt.Errorf("chapter API returned invalid JSON: %w", err)
		}
		html := jsonString(obj[sites.Str(sec, "contentField")])
		if strings.TrimSpace(html) == "" {
			return nil, ErrNoContent
		}
		title := jsonString(obj[sites.Str(sec, "titleField")])
		if title == "" {
			title = ref.Title
		}
		return &Chapter{Title: title, HTML: html, BaseURL: page.URL}, nil
	}

	doc, page, err := e.getDoc(ctx, f, ref.URL, n.URL)
	if err != nil {
		return nil, err
	}
	ch := &Chapter{Title: ref.Title, BaseURL: page.URL}
	if t := sites.ParseSelector(sites.Str(sec, "title")).First(doc.Selection); t != "" {
		ch.Title = t
	}

	// Next link (used for crawl mode).
	nextExpr := sites.Str(sec, "next")
	if strings.HasPrefix(nextExpr, "embedded-json:") {
		ch.NextURL = embeddedJSONNext(string(page.Body), nextExpr, n.Vars, doc.Url)
	} else if nextExpr != "" {
		ch.NextURL = resolve(doc.Url, sites.ParseSelector(nextExpr).First(doc.Selection))
	}
	if ch.NextURL == "" {
		ch.NextURL = genericNext(doc)
	}

	content := sites.ParseSelector(sites.Str(sec, "content")).Nodes(doc.Selection).First()
	if content.Length() == 0 {
		content = readabilityContent(doc)
	}
	if content == nil || content.Length() == 0 {
		if bytes.Contains(page.Body, []byte(`"isFree":false`)) || bytes.Contains(bytes.ToLower(page.Body), []byte("unlock this chapter")) {
			return nil, ErrLocked
		}
		return nil, ErrNoContent
	}
	for _, sel := range sites.StrList(sec, "remove") {
		content.Find(sel).Remove()
	}
	if sites.Bool(sec, "removeHiddenByPageCss") {
		removeHiddenByCSS(doc, content)
	}
	html, _ := content.Html()
	if len(strings.TrimSpace(textOf(content))) < 40 {
		return nil, ErrNoContent
	}
	ch.HTML = html
	return ch, nil
}

func (e *Engine) profile(id string) *sites.Profile {
	for _, p := range e.Registry.Profiles {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// ---- helpers ----

func numberRefs(refs []ChapterRef) []ChapterRef {
	seen := map[string]bool{}
	out := make([]ChapterRef, 0, len(refs))
	for _, r := range refs {
		key := r.URL
		if key == "" {
			key = "idx:" + strconv.Itoa(r.Index)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		r.Number = len(out) + 1
		if r.Title == "" {
			r.Title = fmt.Sprintf("Chapter %d", r.Number)
		}
		out = append(out, r)
	}
	return out
}

func copyVars(v map[string]string) map[string]string {
	out := make(map[string]string, len(v)+2)
	for k, val := range v {
		out[k] = val
	}
	return out
}

// evalVarRegex evaluates "varName ~ regex" against n.Vars and returns an int.
func evalVarRegex(expr string, vars map[string]string) int {
	parts := strings.SplitN(expr, " ~ ", 2)
	val := vars[strings.TrimSpace(parts[0])]
	if len(parts) == 2 {
		if m := regexp.MustCompile(strings.TrimSpace(parts[1])).FindStringSubmatch(val); len(m) > 1 {
			val = m[1]
		}
	}
	n, _ := strconv.Atoi(val)
	return n
}

// embeddedJSONNext handles "embedded-json:nextChapter.slug -> /novel/{slug}/{value}".
func embeddedJSONNext(body, expr string, vars map[string]string, base *url.URL) string {
	spec := strings.TrimPrefix(expr, "embedded-json:")
	tmpl := ""
	if i := strings.Index(spec, " -> "); i >= 0 {
		spec, tmpl = strings.TrimSpace(spec[:i]), strings.TrimSpace(spec[i+4:])
	}
	path := strings.Split(spec, ".")
	if len(path) != 2 {
		return ""
	}
	re := regexp.MustCompile(`"` + regexp.QuoteMeta(path[0]) + `":\s*\{[^{}]*?"` + regexp.QuoteMeta(path[1]) + `":"([^"]+)"`)
	m := re.FindStringSubmatch(body)
	if m == nil {
		return ""
	}
	v := copyVars(vars)
	v["value"] = m[1]
	return resolve(base, sites.Expand(tmpl, v))
}

var hiddenClassRe = regexp.MustCompile(`\.([A-Za-z0-9_-]+)\s*\{[^}]*display\s*:\s*none`)

// removeHiddenByCSS drops elements hidden by the page's own <style> rules
// (Royal Road injects hidden "stolen content" decoy paragraphs this way).
func removeHiddenByCSS(doc *goquery.Document, content *goquery.Selection) {
	doc.Find("style").Each(func(_ int, s *goquery.Selection) {
		for _, m := range hiddenClassRe.FindAllStringSubmatch(s.Text(), -1) {
			content.Find("." + m[1]).Remove()
		}
	})
	content.Find(`[style*="display:none"], [style*="display: none"]`).Remove()
}

func resolve(base *url.URL, href string) string {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "javascript:") || href == "#" {
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

func absoluteURL(base, href string) string {
	b, _ := url.Parse(base)
	return resolve(b, href)
}

func sameURL(a, b string) bool {
	norm := func(s string) string {
		s = strings.TrimRight(strings.TrimPrefix(strings.TrimPrefix(s, "https://"), "http://"), "/")
		return strings.TrimPrefix(s, "www.")
	}
	return norm(a) == norm(b)
}

func hostOf(u string) string {
	p, err := url.Parse(u)
	if err != nil {
		return u
	}
	return strings.TrimPrefix(p.Host, "www.")
}

func titleFromURL(u string) string {
	p, err := url.Parse(u)
	if err != nil {
		return u
	}
	segs := strings.Split(strings.Trim(p.Path, "/"), "/")
	for i := len(segs) - 1; i >= 0; i-- {
		s := strings.TrimSuffix(segs[i], ".html")
		if s != "" && !strings.HasPrefix(s, "chapter") {
			if _, err := strconv.Atoi(s); err != nil {
				words := strings.Fields(strings.ReplaceAll(s, "-", " "))
				for j, w := range words {
					words[j] = strings.ToUpper(w[:1]) + w[1:]
				}
				return strings.Join(words, " ")
			}
		}
	}
	return p.Host
}

func jsonString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatInt(int64(t), 10)
	case nil:
		return ""
	default:
		return fmt.Sprint(t)
	}
}

func textOf(s *goquery.Selection) string { return strings.Join(strings.Fields(s.Text()), " ") }

// descriptionText keeps paragraph breaks in descriptions.
func descriptionText(doc *goquery.Document, sel *sites.Selector, fallback string) string {
	if sel == nil || len(sel.AttrOf()) > 0 {
		return fallback
	}
	node := sel.Nodes(doc.Selection).First()
	var paras []string
	node.Find("p").Each(func(_ int, p *goquery.Selection) {
		if t := textOf(p); t != "" {
			paras = append(paras, t)
		}
	})
	if len(paras) == 0 {
		return fallback
	}
	return strings.Join(paras, "\n\n")
}

// IsBlocked reports whether err came from a bot check / needs browser.
func IsBlocked(err error) bool {
	return errors.Is(err, fetch.ErrBlocked) || errors.Is(err, fetch.ErrNeedsBrowser)
}
