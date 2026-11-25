package service

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"novelarchiver/internal/browser"
	"novelarchiver/internal/fetch"
	"novelarchiver/internal/lang"
	"novelarchiver/internal/sites"
	"novelarchiver/internal/source"
)

// Search stages. The UI runs "quick" first and shows its results, then
// "browser" (merged into the same results), and "optin" only when asked.
const (
	StageQuick   = "quick"   // sites whose search page answers plain HTTP
	StageBrowser = "browser" // sites searched through the site browser
	StageOptIn   = "optin"   // sites whose search page usually asks for a human check
	// StageForeign searches Chinese, Japanese and Korean sites, with the
	// title in their language (see SearchForeign).
	StageForeign = "foreign"
)

// Per-site time limits. Site-browser searches run one after another.
const (
	quickTimeout   = 20 * time.Second
	browserTimeout = 60 * time.Second
	// quickWait is how long the first stage waits before showing what it
	// has; most sites answer in 1-4 seconds.
	quickWait = 6 * time.Second
)

var errBrowserBusy = errors.New("the site window is busy with a download")

// browserInUse reports whether the running download reads its site through
// the site browser, which a search must not navigate away.
func (a *Service) browserInUse() bool {
	a.mu.Lock()
	j := a.cur
	a.mu.Unlock()
	if j == nil {
		return false
	}
	m, err := a.engine.Registry.Match(j.URL)
	if err != nil || m.Profile == nil {
		return false
	}
	p := m.Profile
	return p.NeedsBrowser("novel") || p.NeedsBrowser("toc") || p.NeedsBrowser("chapter")
}

// SearchHit is one site's copy of a novel in the search results.
type SearchHit struct {
	source.SearchHit
	// ExistingSlug is set when this exact link is already in the library.
	ExistingSlug string `json:"existingSlug,omitempty"`
}

// SearchGroup is one novel, with every site that has it.
type SearchGroup struct {
	Key    string `json:"key"`
	Title  string `json:"title"`
	Author string `json:"author,omitempty"`
	Cover  string `json:"cover,omitempty"`
	// MostChapters is the highest chapter count any site's results page
	// showed (0 when none showed one).
	MostChapters int         `json:"mostChapters"`
	Hits         []SearchHit `json:"hits"`
}

// SiteSearch says how one site's search went.
type SiteSearch struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Found int    `json:"found"`
	Error string `json:"error,omitempty"`
}

// SearchResult is the answer to SearchNovels: every site searched so far
// for this query, grouped.
type SearchResult struct {
	Query  string        `json:"query"`
	Groups []SearchGroup `json:"groups"`
	Sites  []SiteSearch  `json:"sites"`
	// Pending names the sites whose results aren't in yet: the ones the
	// "browser" stage searches, and slow sites from the first stage that are
	// still answering. The "browser" stage waits for all of them.
	Pending []string `json:"pending,omitempty"`
	// OptInSites haven't been searched; the "optin" stage searches them.
	OptInSites []string `json:"optInSites,omitempty"`
	// ForeignLanguages are the languages ("zh", "ja") that have sites the
	// "foreign:zh,ja" stage can search, for the language chips.
	ForeignLanguages []string `json:"foreignLanguages,omitempty"`
}

// searchCache keeps the latest query's per-site results, so each stage adds
// to the ones before it.
type searchCache struct {
	mu       sync.Mutex
	query    string
	sites    map[string]siteResult
	inflight map[string]chan struct{} // closed when that site's search ends
}

type siteResult struct {
	info SiteSearch
	hits []source.SearchHit
	lang string // "" for English sites
}

func stageOf(p *sites.Profile) string {
	switch {
	case lang.NeedsTranslation(p.Language):
		return StageForeign
	case sites.Bool(p.Search, "optIn"):
		return StageOptIn
	case p.NeedsBrowser("search"):
		return StageBrowser
	}
	return StageQuick
}

// SearchNovels searches one stage's sites for query, all at once, and
// returns every site's results so far for that query, grouped so each novel
// appears once with the sites that carry it. stage "" means "quick", which
// also starts a new search.
func (a *Service) SearchNovels(query, stage string) (*SearchResult, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	query = strings.Join(strings.Fields(query), " ")
	if len([]rune(query)) < 2 {
		return nil, errors.New("type at least two letters of the title")
	}
	if stage == "" {
		stage = StageQuick
	}
	var foreignLangs []string
	if rest, ok := strings.CutPrefix(stage, StageForeign); ok {
		stage = StageForeign
		for _, l := range strings.Split(strings.TrimPrefix(rest, ":"), ",") {
			if l = strings.TrimSpace(l); l != "" {
				foreignLangs = append(foreignLangs, l)
			}
		}
	}
	all := a.engine.Searchable(true)
	var run []*sites.Profile
	for _, p := range all {
		if stageOf(p) == stage {
			run = append(run, p)
		}
	}
	if len(all) == 0 {
		return nil, errors.New("no supported site has a search page")
	}

	c := &a.search
	c.mu.Lock()
	if stage == StageQuick || c.query != query {
		c.query, c.sites, c.inflight = query, map[string]siteResult{}, map[string]chan struct{}{}
	}
	c.mu.Unlock()

	one := func(p *sites.Profile, limit time.Duration) {
		ctx, cancel := context.WithTimeout(context.Background(), limit)
		defer cancel()
		var hits []source.SearchHit
		var err error
		if p.NeedsBrowser("search") && a.browserInUse() {
			err = errBrowserBusy
		} else {
			hits, err = a.engine.SearchSite(ctx, p, query)
		}
		r := siteResult{info: SiteSearch{ID: p.ID, Name: p.Name, Found: len(hits)}, hits: hits}
		if err != nil {
			r.info.Error = searchError(ctx, err)
		}
		c.mu.Lock()
		if c.query == query {
			c.sites[p.ID] = r
		}
		c.mu.Unlock()
	}
	if stage == StageForeign {
		a.searchForeign(query, foreignLangs, c)
	} else if stage == StageQuick {
		// Answer after quickWait at most; a slow site keeps going and its
		// results join in the "browser" stage.
		var wg sync.WaitGroup
		for _, p := range run {
			done := make(chan struct{})
			c.mu.Lock()
			c.inflight[p.ID] = done
			c.mu.Unlock()
			wg.Add(1)
			go func() { defer wg.Done(); defer close(done); one(p, quickTimeout) }()
		}
		finished := make(chan struct{})
		go func() { wg.Wait(); close(finished) }()
		select {
		case <-finished:
		case <-time.After(quickWait):
		}
	} else {
		// The site browser is one window: two sites at once would navigate
		// it out from under each other, so they take turns.
		for _, p := range run {
			one(p, browserTimeout)
		}
		// Then wait for the first stage's slow sites (each has its own
		// time limit, so this ends).
		c.mu.Lock()
		var waits []chan struct{}
		if c.query == query {
			for _, ch := range c.inflight {
				waits = append(waits, ch)
			}
		}
		c.mu.Unlock()
		for _, ch := range waits {
			<-ch
		}
	}

	res := &SearchResult{Query: query, Groups: []SearchGroup{}, Sites: []SiteSearch{}}
	seenLang := map[string]bool{}
	for _, p := range all {
		if l := lang.Norm(p.Language); stageOf(p) == StageForeign && !seenLang[l] {
			seenLang[l] = true
			res.ForeignLanguages = append(res.ForeignLanguages, l)
		}
	}
	var perSite [][]source.SearchHit
	c.mu.Lock()
	for _, p := range all {
		r, done := c.sites[p.ID]
		if c.query != query {
			done = false
		}
		switch {
		case done:
			res.Sites = append(res.Sites, r.info)
			perSite = append(perSite, r.hits)
		case stageOf(p) == StageQuick || stageOf(p) == StageBrowser:
			res.Pending = append(res.Pending, p.Name)
		case stageOf(p) == StageOptIn:
			res.OptInSites = append(res.OptInSites, p.Name)
		case stageOf(p) == StageForeign:
			// searched only when a language chip is on
		}
	}
	c.mu.Unlock()
	res.Groups = groupHits(query, perSite, a.librarySlugs(), a.engine.Registry.NovelKey)
	return res, nil
}

func searchError(ctx context.Context, err error) string {
	switch {
	case ctx.Err() == context.DeadlineExceeded:
		return "didn't answer in time"
	case errors.Is(err, fetch.ErrBlocked):
		return "it showed a bot check"
	case isHTTPStatus(err, 429):
		return "it's limiting requests right now; try again in a few minutes"
	case errors.Is(err, browser.ErrChallengeTimeout):
		return "the human check wasn't ticked in time"
	case errors.Is(err, browser.ErrClosed):
		return "the site window was closed"
	case errors.Is(err, errBrowserBusy):
		return "the site window is busy with a download; search again when it finishes"
	}
	return friendly(err).Error()
}

// librarySlugs maps each saved novel's link (normalised) to its slug.
func (a *Service) librarySlugs() map[string]string {
	out := map[string]string{}
	books, _ := a.store.List()
	for _, b := range books {
		out[a.engine.Registry.NovelKey(b.URL)] = b.Slug
	}
	return out
}

func normLink(u string) string {
	u = strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
	return strings.ToLower(strings.TrimRight(strings.TrimPrefix(u, "www."), "/"))
}

// groupHits merges the sites' results into one entry per novel. Groups are
// ordered by how well the title matches the query, then by how many sites
// carry the novel, then by how high the sites ranked it.
func groupHits(query string, perSite [][]source.SearchHit, library map[string]string, novelKey func(string) string) []SearchGroup {
	type acc struct {
		g     SearchGroup
		rank  int // best position in any site's results
		sites map[string]bool
	}
	byKey := map[string]*acc{}
	var order []string
	for _, hits := range perSite {
		for pos, h := range hits {
			key := titleKey(h.Title)
			if key == "" {
				continue
			}
			x := byKey[key]
			if x == nil {
				x = &acc{g: SearchGroup{Key: key, Title: h.Title}, rank: pos, sites: map[string]bool{}}
				byKey[key] = x
				order = append(order, key)
			}
			if x.sites[h.SiteID] {
				continue // one row per site
			}
			x.sites[h.SiteID] = true
			if pos < x.rank {
				x.rank = pos
			}
			if x.g.Author == "" {
				x.g.Author = h.Author
			}
			if x.g.Cover == "" {
				x.g.Cover = h.Cover
			}
			if h.Chapters > x.g.MostChapters {
				x.g.MostChapters = h.Chapters
			}
			x.g.Hits = append(x.g.Hits, SearchHit{SearchHit: h, ExistingSlug: library[novelKey(h.URL)]})
		}
	}
	qKey, qWords := titleKey(query), keyWords(query)
	score := func(x *acc) int {
		switch {
		case x.g.Key == qKey:
			return 4
		case qKey != "" && strings.HasPrefix(x.g.Key, qKey):
			// "Omniscient Reader's Viewpoint" for "omniscient reader":
			// the title starts with the query, ahead of fan works that
			// only mention it ("SKYFALL | Omniscient Reader").
			return 3
		case containsAll(keyWords(x.g.Title), qWords):
			return 2
		case strings.Contains(x.g.Key, qKey):
			return 1
		}
		return 0
	}
	list := make([]*acc, 0, len(order))
	for _, k := range order {
		x := byKey[k]
		// Most chapters first; unknown counts last.
		sort.SliceStable(x.g.Hits, func(i, j int) bool { return x.g.Hits[i].Chapters > x.g.Hits[j].Chapters })
		list = append(list, x)
	}
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if sa, sb := score(a), score(b); sa != sb {
			return sa > sb
		}
		if len(a.sites) != len(b.sites) {
			return len(a.sites) > len(b.sites)
		}
		return a.rank < b.rank
	})
	out := make([]SearchGroup, len(list))
	for i, x := range list {
		out[i] = x.g
	}
	return out
}

var (
	// Bits sites add to a title that don't change which novel it is.
	titleNoise = regexp.MustCompile(`(?i)[(\[](web ?novel|light novel|ln|wn|novel|completed|ongoing|official|raw|mtl)[)\]]`)
	nonWord    = regexp.MustCompile(`[^\p{L}\p{N}]+`)
	stopWords  = map[string]bool{"a": true, "an": true, "the": true, "of": true, "and": true}
)

// keyWords lowercases a title into its significant words.
func keyWords(t string) []string {
	t = titleNoise.ReplaceAllString(strings.ToLower(t), " ")
	t = strings.NewReplacer("’", "", "'", "", "&", " and ").Replace(t)
	var out []string
	for _, w := range strings.Fields(nonWord.ReplaceAllString(t, " ")) {
		if !stopWords[w] {
			out = append(out, w)
		}
	}
	return out
}

// titleKey is what two sites' titles for the same novel have in common:
// "Lord of the Mysteries", "Lord Of Mysteries" and "LORD OF THE MYSTERIES
// (Web Novel)" all give "lordmysteries".
func titleKey(t string) string { return strings.Join(keyWords(t), "") }

func containsAll(have, want []string) bool {
	set := map[string]bool{}
	for _, w := range have {
		set[w] = true
	}
	for _, w := range want {
		if !set[w] && !set[w+"s"] { // "reader" matches "Reader's"
			return false
		}
	}
	return len(want) > 0
}

// ChapterCount is the exact chapter count for one search hit, read from the
// site's full chapter list.
type ChapterCount struct {
	URL      string `json:"url"`
	Total    int    `json:"total"`
	Free     int    `json:"free"` // not paid or locked
	Latest   string `json:"latest,omitempty"`
	Status   string `json:"status,omitempty"`
	Unknown  bool   `json:"unknown,omitempty"` // the site only allows following "Next" links
	Existing string `json:"existingSlug,omitempty"`
}

// CountChapters reads a novel's chapter list from its site without
// showing download progress. The result is cached, so choosing Download for
// the same link afterwards doesn't fetch it again.
func (a *Service) CountChapters(url string) (*ChapterCount, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	n, err := a.discoverWith(context.Background(), url, nil)
	if err != nil {
		return nil, friendly(err)
	}
	c := &ChapterCount{URL: n.URL, Total: len(n.Chapters), Status: n.Status, Unknown: n.TOCMode == "crawl"}
	for _, ch := range n.Chapters {
		if !ch.Locked {
			c.Free++
		}
	}
	if len(n.Chapters) > 0 {
		c.Latest = n.Chapters[len(n.Chapters)-1].Title
	}
	c.Existing = a.store.FindByURL(n.URL)
	return c, nil
}

func isHTTPStatus(err error, status int) bool {
	var he *fetch.HTTPError
	return errors.As(err, &he) && he.Status == status
}
