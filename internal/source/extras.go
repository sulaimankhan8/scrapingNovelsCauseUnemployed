package source

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"novelarchiver/internal/fetch"
	"novelarchiver/internal/sites"
)

// jsonAt reads a dotted path ("user.name", "parts.0.id") from decoded JSON.
// "a|b" tries each path in order and returns the first non-empty value.
func jsonAt(v any, path string) any {
	for _, alt := range strings.Split(path, "|") {
		cur := v
		for _, key := range strings.Split(strings.TrimSpace(alt), ".") {
			if key == "" {
				continue
			}
			switch t := cur.(type) {
			case map[string]any:
				cur = t[key]
			case []any:
				i, err := strconv.Atoi(key)
				if err != nil || i < 0 || i >= len(t) {
					cur = nil
				} else {
					cur = t[i]
				}
			default:
				cur = nil
			}
		}
		if jsonString(cur) != "" {
			return cur
		}
		if _, ok := cur.([]any); ok {
			return cur
		}
	}
	return nil
}

// splitNames turns "Alternative names: Lord of Mysteries, LOTM, 诡秘之主"
// into its names.
func splitNames(s string) []string {
	if i := strings.IndexAny(s, ":："); i >= 0 && i < 40 && !strings.ContainsAny(s[:i], ",，") {
		s = s[i+len(string([]rune(s[i:])[0])):]
	}
	var out []string
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '，' || r == '、' || r == ';' || r == '/' }) {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func clipRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// queryFetcher adds a profile's fixed query (AO3's "view_adult=true") to
// every request, and reports pages under the link that was asked for, so
// the site's redirects (AO3 sends a work to its first chapter) don't change
// which link the library stores.
type queryFetcher struct {
	f     fetch.Fetcher
	query string
}

func (q queryFetcher) Get(ctx context.Context, rawURL, referer string) (*fetch.Page, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return q.f.Get(ctx, rawURL, referer)
	}
	add, _ := url.ParseQuery(q.query)
	vals := u.Query()
	for k, vs := range add {
		if vals.Get(k) == "" {
			vals[k] = vs
		}
	}
	u.RawQuery = vals.Encode()
	page, err := q.f.Get(ctx, u.String(), referer)
	if err != nil {
		return nil, err
	}
	cp := *page
	cp.URL = rawURL
	return &cp, nil
}

// AltNames reads the other titles a novel page lists ("Alternative names:
// Lord of Mysteries, LOTM, 诡秘之主") without loading the chapter list. It
// returns nothing for sites whose profile has no altNames selector.
func (e *Engine) AltNames(ctx context.Context, link string) ([]string, error) {
	m, err := e.Registry.Match(link)
	if err != nil || m.Profile == nil || m.Kind != sites.KindNovel {
		return nil, err
	}
	sel := sites.Str(m.Profile.Novel, "altNames")
	if sel == "" {
		return nil, nil
	}
	f, err := e.fetcher(m.Profile, "novel")
	if err != nil {
		return nil, err
	}
	doc, _, err := e.getDoc(ctx, f, m.URL, "")
	if err != nil {
		return nil, err
	}
	return splitNames(sites.ParseSelector(sel).First(doc.Selection)), nil
}

// HasAltNames reports whether a site's novel pages list other titles.
func HasAltNames(p *sites.Profile) bool { return sites.Str(p.Novel, "altNames") != "" }

// englishStatus turns Chinese and Japanese serial statuses into English.
func englishStatus(s string) string {
	t := strings.TrimSpace(s)
	switch {
	case t == "":
		return s
	case strings.ContainsAny(t, "完") && (strings.Contains(t, "完结") || strings.Contains(t, "完結") || strings.Contains(t, "完本") || strings.Contains(t, "全本")):
		return "Completed"
	case strings.Contains(t, "连载") || strings.Contains(t, "連載") || strings.Contains(t, "连载中"):
		return "Ongoing"
	case strings.Contains(t, "休載") || strings.Contains(t, "暂停") || strings.Contains(t, "斷更"):
		return "On hiatus"
	}
	return s
}
