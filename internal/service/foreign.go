package service

import (
	"context"
	"strings"
	"sync"
	"time"
	"unicode"

	"novelarchiver/internal/lang"
	"novelarchiver/internal/sites"
	"novelarchiver/internal/source"
	"novelarchiver/internal/translate"
)

// foreignTimeout bounds the whole foreign stage (name lookups, searches,
// title translation).
const foreignTimeout = 40 * time.Second

// searchForeign is the "foreign" stage: it searches the Chinese, Japanese
// and Korean sites of the chosen languages. Those sites only match titles
// written in their language, and a machine translation of an English title
// rarely gives the real one ("Lord of the Mysteries" is 诡秘之主, not
// 神秘之主), so the original title is looked up first:
//
//  1. a query already written in Chinese / Japanese / Korean is used as is;
//  2. else the English sites' novel pages list "Alternative names", which
//     usually include the original title (NovelFull, ReadNovelFull);
//  3. else Google's translation of the query is tried as a guess.
//
// Each foreign result's title is translated to English. A result that is
// the same novel as the English sites' best match (same translated title,
// or the site's single exact match) joins that novel's card.
func (a *Service) searchForeign(query string, langs []string, c *searchCache) {
	ctx, cancel := context.WithTimeout(context.Background(), foreignTimeout)
	defer cancel()
	want := map[string]bool{}
	for _, l := range langs {
		want[lang.Norm(l)] = true
	}
	var run []*sites.Profile
	for _, p := range a.engine.Searchable(true) {
		if stageOf(p) == StageForeign && want[lang.Norm(p.Language)] {
			run = append(run, p)
		}
	}
	if len(run) == 0 {
		return
	}

	// The English sites' best match, to attach the foreign copies to.
	c.mu.Lock()
	var perSite [][]source.SearchHit
	for _, r := range c.sites {
		if lang.Norm(r.lang) == lang.English {
			perSite = append(perSite, r.hits)
		}
	}
	c.mu.Unlock()
	var best *SearchGroup
	if groups := groupHits(query, perSite, nil, normLink); len(groups) > 0 {
		best = &groups[0]
	}

	names := a.originalTitles(ctx, query, best, want)

	var wg sync.WaitGroup
	for _, p := range run {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code := lang.Norm(p.Language)
			if len(names[code]) == 0 {
				// No title in this language to search with: say so, rather
				// than leave the site out without a word.
				r := siteResult{info: SiteSearch{ID: p.ID, Name: p.Name, Error: "couldn't work out the " + lang.Name(code) + " title; type it in " + lang.Name(code) + " to search this site"}, lang: code}
				c.mu.Lock()
				if c.query == query {
					c.sites[p.ID] = r
				}
				c.mu.Unlock()
				return
			}
			var hits []source.SearchHit
			var lastErr error
			seen := map[string]bool{}
			for _, name := range names[code] {
				got, err := a.engine.SearchSite(ctx, p, name)
				if err != nil {
					lastErr = err
					continue
				}
				for _, h := range got {
					if !seen[h.URL] {
						seen[h.URL] = true
						hits = append(hits, h)
					}
				}
				if len(hits) > 0 {
					break
				}
			}
			a.englishTitles(ctx, code, hits, best, names[code])
			r := siteResult{info: SiteSearch{ID: p.ID, Name: p.Name, Found: len(hits)}, hits: hits, lang: code}
			if len(hits) == 0 && lastErr != nil {
				r.info.Error = searchError(ctx, lastErr)
			}
			c.mu.Lock()
			if c.query == query {
				c.sites[p.ID] = r
			}
			c.mu.Unlock()
		}()
	}
	wg.Wait()
}

// originalTitles finds the title in each wanted language.
func (a *Service) originalTitles(ctx context.Context, query string, best *SearchGroup, want map[string]bool) map[string][]string {
	out := map[string][]string{}
	add := func(code, name string) {
		name = strings.TrimSpace(name)
		if name == "" || !want[code] {
			return
		}
		for _, have := range out[code] {
			if have == name {
				return
			}
		}
		out[code] = append(out[code], name)
	}
	classify := func(name string) {
		switch script(name) {
		case lang.Korean:
			add(lang.Korean, name)
		case lang.Japanese:
			add(lang.Japanese, name)
		case lang.Chinese: // kanji-only titles can be Japanese too
			add(lang.Chinese, name)
			add(lang.Japanese, name)
		}
	}
	// 1. Already in the original language.
	if script(query) != "" {
		classify(query)
		return out
	}
	// 2. "Alternative names" on the English sites' pages.
	if best != nil {
		tried := 0
		for _, h := range best.Hits {
			p := a.profileByID(h.SiteID)
			if p == nil || !source.HasAltNames(p) || tried >= 2 {
				continue
			}
			tried++
			alts, err := a.engine.AltNames(ctx, h.URL)
			if err != nil {
				continue
			}
			for _, n := range alts {
				classify(n)
			}
			if len(out) > 0 {
				break
			}
		}
	}
	// 3. A machine translation of the query, as a last guess.
	if a.translator != nil {
		for code := range want {
			if len(out[code]) > 0 {
				continue
			}
			target := map[string]string{lang.Chinese: "zh-CN", lang.Japanese: "ja", lang.Korean: "ko"}[code]
			if t, err := a.translator.Into(ctx, query, target); err == nil {
				add(code, t)
			}
		}
	}
	return out
}

// englishTitles translates the foreign hits' titles, keeping the original
// in TitleOriginal. A hit that is the same novel as best (its title is the
// one searched for, its translation matches best's title, or it's the
// site's only result) takes best's title, so it lands in best's card.
func (a *Service) englishTitles(ctx context.Context, code string, hits []source.SearchHit, best *SearchGroup, searched []string) {
	if len(hits) == 0 {
		return
	}
	orig := make([]string, len(hits))
	for i, h := range hits {
		orig[i] = h.Title
	}
	en := orig
	if a.translator != nil {
		if got, err := translate.Texts(ctx, a.translator, code, nil, orig...); err == nil {
			en = got
		}
	}
	for i := range hits {
		hits[i].TitleOriginal = orig[i]
		hits[i].Title = strings.TrimSpace(en[i])
		if hits[i].Title == "" {
			hits[i].Title = orig[i]
		}
		same := len(hits) == 1 || titleKey(hits[i].Title) == best0Key(best)
		for _, n := range searched {
			same = same || sameTitle(orig[i], n)
		}
		if best != nil && same {
			hits[i].Title = best.Title
		}
	}
}

func (a *Service) profileByID(id string) *sites.Profile {
	for _, p := range a.engine.Registry.Profiles {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// script names the language a short title is written in ("" for Latin):
// Hangul is Korean, kana Japanese, Chinese characters alone Chinese.
func script(s string) string {
	var han, kana, hangul int
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Hangul, r):
			hangul++
		case unicode.Is(unicode.Hiragana, r), unicode.Is(unicode.Katakana, r):
			kana++
		case unicode.Is(unicode.Han, r):
			han++
		}
	}
	switch {
	case hangul > 0:
		return lang.Korean
	case kana > 0:
		return lang.Japanese
	case han > 0:
		return lang.Chinese
	}
	return ""
}

// sameTitle compares two titles ignoring spaces and punctuation.
func sameTitle(a, b string) bool {
	norm := func(s string) string {
		var out []rune
		for _, r := range s {
			if unicode.IsLetter(r) || unicode.IsNumber(r) {
				out = append(out, unicode.ToLower(r))
			}
		}
		return string(out)
	}
	return norm(a) == norm(b)
}

func best0Key(g *SearchGroup) string {
	if g == nil {
		return "-"
	}
	return g.Key
}
