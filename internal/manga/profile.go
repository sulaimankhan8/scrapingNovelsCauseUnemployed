// Package manga downloads manga, manhwa and manhua for offline reading:
// site profiles (sites.json), discovery of a series and its chapters,
// page-image downloads, an on-disk library and CBZ export.
package manga

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

//go:embed sites.json
var builtinSites []byte

// Profile describes how to read one site. Selectors use the same mini
// language as the novel profiles: "css@attr|attr2, alt ~ regex".
type Profile struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Type     string   `json:"type"` // Manga, Manhwa, Manhua or a mix
	Home     string   `json:"home"`
	Domains  []string `json:"domains"`
	Status   string   `json:"status"`    // "tested" or "human-check"
	Fetch    string   `json:"fetchMode"` // "http" (default) or "browser"
	Verified string   `json:"verified,omitempty"`
	Example  string   `json:"example,omitempty"`
	// API selects built-in code for sites with a JSON API ("mangadex").
	API string `json:"api,omitempty"`
	// Headers are sent with every request to the site (not to image CDNs).
	Headers map[string]string `json:"headers,omitempty"`
	// ImageReferer is "chapter" (default: the chapter page) or "origin".
	ImageReferer string `json:"imageReferer,omitempty"`

	SeriesURL  string `json:"seriesUrl"`
	ChapterURL string `json:"chapterUrl"`
	// SeriesFromChapter finds the series page from a chapter link: a
	// "{var}" template using the chapterUrl captures, or a selector.
	SeriesFromChapter string `json:"seriesFromChapter"`

	Series   map[string]string `json:"series"` // title, cover, description, author, status
	Vars     map[string]string `json:"vars"`   // extra values read from the series page
	Chapters ChapterRules      `json:"chapters"`
	Pages    PageRules         `json:"pages"`
	// Search finds series by title (nil: the site can't be searched).
	Search *SearchRules `json:"search,omitempty"`

	seriesRe  *regexp.Regexp
	chapterRe *regexp.Regexp
}

// ChapterRules say where the chapter list is and how to read it.
type ChapterRules struct {
	URL    string `json:"url"`    // template; empty = the series page itself
	Method string `json:"method"` // GET or POST
	Body   string `json:"body"`
	Items  string `json:"items"` // selector for chapter links
	Title  string `json:"title"` // selector inside an item for its title
	Order  string `json:"order"` // "newest-first" (reversed) or "oldest-first"
	JSON   *struct {
		Script string `json:"script"` // read JSON from this <script> (e.g. #__NEXT_DATA__)
		Path   string `json:"path"`   // dotted path to the array
		Title  string `json:"title"`  // field names, "a|b" tries in order, or a "{field}" template
		URL    string `json:"url"`    // field name or "{field}" template
		Base   string `json:"base"`
	} `json:"json,omitempty"`
}

// PageRules say how to find a chapter's page images.
type PageRules struct {
	URL       string            `json:"url"`  // template; empty = the chapter page
	Vars      map[string]string `json:"vars"` // read from the chapter page first
	JSONField string            `json:"jsonField"`
	Images    string            `json:"images"`      // selector; elements or <img>
	SortAttr  string            `json:"sortAttr"`    // order by this numeric attribute
	Script    bool              `json:"scriptArray"` // longest JS array of image links
	Regex     string            `json:"regex"`       // every match in the raw page, in order
	Exclude   string            `json:"exclude"`     // drop matching links
}

// IsBrowser reports whether the site's pages need the site browser.
func (p *Profile) IsBrowser() bool { return p.Fetch == "browser" }

// Registry holds all site profiles.
type Registry struct{ Sites []*Profile }

// LoadSites parses the built-in profiles and any *.json files in userDir
// (a user profile with the same id replaces the built-in one).
func LoadSites(userDir string) (*Registry, error) {
	r := &Registry{}
	if err := r.add(builtinSites, "built-in sites.json"); err != nil {
		return nil, err
	}
	if userDir != "" {
		files, _ := filepath.Glob(filepath.Join(userDir, "*.json"))
		sort.Strings(files)
		for _, f := range files {
			if data, err := os.ReadFile(f); err == nil {
				if err := r.add(data, f); err != nil {
					return nil, err
				}
			}
		}
	}
	return r, nil
}

func (r *Registry) add(data []byte, src string) error {
	var ff struct {
		Sites []*Profile `json:"sites"`
	}
	if err := json.Unmarshal(data, &ff); err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}
	for _, p := range ff.Sites {
		var err error
		if p.seriesRe, err = regexp.Compile(p.SeriesURL); err != nil {
			return fmt.Errorf("%s: %s seriesUrl: %w", src, p.ID, err)
		}
		if p.chapterRe, err = regexp.Compile(p.ChapterURL); err != nil {
			return fmt.Errorf("%s: %s chapterUrl: %w", src, p.ID, err)
		}
		replaced := false
		for i, old := range r.Sites {
			if old.ID == p.ID {
				r.Sites[i], replaced = p, true
			}
		}
		if !replaced {
			r.Sites = append(r.Sites, p)
		}
	}
	return nil
}

// ByID finds a profile.
func (r *Registry) ByID(id string) *Profile {
	for _, p := range r.Sites {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// ByHost finds the profile whose domains include the link's host.
func (r *Registry) ByHost(link string) *Profile {
	u, err := url.Parse(link)
	if err != nil {
		return nil
	}
	host := strings.TrimPrefix(u.Hostname(), "www.")
	for _, p := range r.Sites {
		for _, d := range p.Domains {
			if host == d || strings.HasSuffix(host, "."+d) {
				return p
			}
		}
	}
	return nil
}

// Match is a classified pasted link.
type Match struct {
	Profile  *Profile
	Chapter  bool // the link is a chapter page
	URL      string
	Captures map[string]string
}

// Match classifies a pasted link. Chapter patterns are tried first since a
// chapter link usually extends the series link.
func (r *Registry) Match(raw string) (*Match, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, fmt.Errorf("paste a series or chapter link")
	}
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("%q is not a valid link", raw)
	}
	u.Fragment = ""
	s = u.String()
	for _, p := range r.Sites {
		if c := captures(p.chapterRe, s); c != nil {
			return &Match{Profile: p, Chapter: true, URL: s, Captures: c}, nil
		}
	}
	for _, p := range r.Sites {
		if c := captures(p.seriesRe, s); c != nil {
			return &Match{Profile: p, URL: s, Captures: c}, nil
		}
	}
	host := strings.TrimPrefix(u.Host, "www.")
	for _, p := range r.Sites {
		for _, d := range p.Domains {
			if host == d {
				return nil, fmt.Errorf("that %s link isn't a series or chapter page. Open the series on %s and copy the link from the address bar", p.Name, p.Name)
			}
		}
	}
	return nil, fmt.Errorf("%s isn't a supported site yet. See the Sites list for the ones that work", host)
}

func captures(re *regexp.Regexp, s string) map[string]string {
	m := re.FindStringSubmatch(s)
	if m == nil {
		return nil
	}
	out := map[string]string{}
	for i, n := range re.SubexpNames() {
		if n != "" && i < len(m) {
			out[n] = m[i]
		}
	}
	return out
}

var placeholder = regexp.MustCompile(`\{([a-zA-Z]+)\}`)

// expand fills {name} placeholders from vars.
func expand(t string, vars map[string]string) string {
	return placeholder.ReplaceAllStringFunc(t, func(m string) string {
		if v, ok := vars[m[1:len(m)-1]]; ok {
			return v
		}
		return m
	})
}
