// Package sites loads per-site parser profiles (sites.json) and maps pasted
// URLs to a profile and a page kind (novel page or chapter page).
package sites

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

//go:embed sites.json
var builtinProfiles []byte

// Profile describes how to read one site. Selector strings use the mini
// language documented in README.md (see Selector).
type Profile struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Domains   []string `json:"domains"`
	Status    string   `json:"status"`
	FetchMode string   `json:"fetchMode"`
	Verified  string   `json:"verified,omitempty"`
	// Home and Example are shown on the Supported sites page.
	Home    string `json:"home,omitempty"`
	Example string `json:"example,omitempty"`
	// CanonicalHost, when set, replaces the host of any matching link
	// (e.g. a mirror domain that is down or that the site's own servers
	// refuse to serve).
	CanonicalHost string `json:"canonicalHost,omitempty"`

	Input struct {
		NovelURL         string `json:"novelUrl"`
		ChapterURL       string `json:"chapterUrl"`
		NovelFromChapter string `json:"novelFromChapter"`
	} `json:"input"`

	Novel   map[string]any `json:"novel"`
	TOC     map[string]any `json:"toc"`
	Chapter map[string]any `json:"chapter"`

	novelRe   *regexp.Regexp
	chapterRe *regexp.Regexp
}

// Str returns a string field from one of the profile's sections.
func Str(section map[string]any, key string) string {
	if section == nil {
		return ""
	}
	if v, ok := section[key].(string); ok {
		return v
	}
	return ""
}

// StrList returns a []string field (e.g. chapter.remove).
func StrList(section map[string]any, key string) []string {
	raw, _ := section[key].([]any)
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		if s, ok := r.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// Bool returns a bool field.
func Bool(section map[string]any, key string) bool {
	b, _ := section[key].(bool)
	return b
}

// NeedsBrowser reports whether the given stage ("novel", "toc", "chapter")
// must be fetched through a real browser engine.
func (p *Profile) NeedsBrowser(stage string) bool {
	var sec map[string]any
	switch stage {
	case "novel":
		sec = p.Novel
	case "toc":
		sec = p.TOC
	case "chapter":
		sec = p.Chapter
	}
	if m := Str(sec, "fetchMode"); m != "" {
		return m == "browser"
	}
	return p.FetchMode == "browser"
}

// Registry holds all known profiles.
type Registry struct {
	Profiles []*Profile
	Generic  map[string]any
}

type fileFormat struct {
	Version int            `json:"version"`
	Sites   []*Profile     `json:"sites"`
	Generic map[string]any `json:"generic"`
}

// Load parses the built-in profiles, then overlays any *.json profile files
// found in userDir (user profiles with the same id replace built-in ones).
func Load(userDir string) (*Registry, error) {
	reg := &Registry{}
	if err := reg.add(builtinProfiles, "built-in sites.json"); err != nil {
		return nil, err
	}
	if userDir != "" {
		files, _ := filepath.Glob(filepath.Join(userDir, "*.json"))
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			if err := reg.add(data, f); err != nil {
				return nil, err
			}
		}
	}
	return reg, nil
}

func (r *Registry) add(data []byte, source string) error {
	var ff fileFormat
	if err := json.Unmarshal(data, &ff); err != nil {
		return fmt.Errorf("%s: %w", source, err)
	}
	if ff.Generic != nil {
		r.Generic = ff.Generic
	}
	for _, p := range ff.Sites {
		var err error
		if p.novelRe, err = regexp.Compile(p.Input.NovelURL); err != nil {
			return fmt.Errorf("%s: site %s novelUrl: %w", source, p.ID, err)
		}
		if p.chapterRe, err = regexp.Compile(p.Input.ChapterURL); err != nil {
			return fmt.Errorf("%s: site %s chapterUrl: %w", source, p.ID, err)
		}
		replaced := false
		for i, existing := range r.Profiles {
			if existing.ID == p.ID {
				r.Profiles[i] = p
				replaced = true
			}
		}
		if !replaced {
			r.Profiles = append(r.Profiles, p)
		}
	}
	return nil
}

// PageKind says what kind of page a pasted URL points to.
type PageKind string

const (
	KindNovel   PageKind = "novel"
	KindChapter PageKind = "chapter"
	KindUnknown PageKind = "unknown"
)

// Match is the result of classifying a pasted URL.
type Match struct {
	Profile  *Profile          // nil when no profile matches (use generic)
	Kind     PageKind          // novel, chapter or unknown
	URL      string            // normalised input URL
	Captures map[string]string // named regex groups (slug, id, num, ...)
}

// Origin returns scheme://host of the matched URL.
func (m *Match) Origin() string {
	u, err := url.Parse(m.URL)
	if err != nil {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// Match classifies a pasted URL. Chapter patterns are tried before novel
// patterns because a chapter URL usually extends the novel URL.
func (r *Registry) Match(raw string) (*Match, error) {
	u := strings.TrimSpace(raw)
	if u == "" {
		return nil, fmt.Errorf("paste a novel page or chapter link")
	}
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		u = "https://" + u
	}
	parsed, err := url.Parse(u)
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("%q is not a valid link", raw)
	}
	parsed.Fragment = ""
	u = parsed.String()

	for _, p := range r.Profiles {
		if caps := captures(p.chapterRe, u); caps != nil {
			return &Match{Profile: p, Kind: KindChapter, URL: canonical(p, parsed), Captures: caps}, nil
		}
	}
	for _, p := range r.Profiles {
		if caps := captures(p.novelRe, u); caps != nil {
			return &Match{Profile: p, Kind: KindNovel, URL: canonical(p, parsed), Captures: caps}, nil
		}
	}
	// A known domain with an unrecognised path, or an unknown site.
	host := strings.TrimPrefix(parsed.Host, "www.")
	for _, p := range r.Profiles {
		for _, d := range p.Domains {
			if host == d {
				return &Match{Profile: p, Kind: KindUnknown, URL: u, Captures: map[string]string{}}, nil
			}
		}
	}
	return &Match{Kind: KindUnknown, URL: u, Captures: map[string]string{}}, nil
}

// canonical moves a link onto the profile's canonical host, if it has one.
func canonical(p *Profile, u *url.URL) string {
	if p.CanonicalHost == "" {
		return u.String()
	}
	c := *u
	c.Host = p.CanonicalHost
	c.Scheme = "https"
	return c.String()
}

func captures(re *regexp.Regexp, s string) map[string]string {
	m := re.FindStringSubmatch(s)
	if m == nil {
		return nil
	}
	out := map[string]string{}
	for i, name := range re.SubexpNames() {
		if name != "" && i < len(m) {
			out[name] = m[i]
		}
	}
	return out
}

// Expand replaces {name} placeholders with values from vars.
func Expand(tmpl string, vars map[string]string) string {
	return regexp.MustCompile(`\{([a-zA-Z]+)\}`).ReplaceAllStringFunc(tmpl, func(m string) string {
		if v, ok := vars[m[1:len(m)-1]]; ok {
			return v
		}
		return m
	})
}
