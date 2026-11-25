package service

import "novelarchiver/internal/lang"

// SiteInfo describes a supported novel site for the Supported sites page.
type SiteInfo struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Domains  []string `json:"domains"`
	Home     string   `json:"home"`
	Example  string   `json:"example"`
	Status   string   `json:"status"`    // verified, partial, browser-only, human-check
	Fetch    string   `json:"fetchMode"` // http, browser, http+browser
	Verified string   `json:"verified"`  // what was tested and when
	Language string   `json:"language"`  // en, zh, ja, ko
}

// Sites lists the novel site profiles (built-in and user-added).
func (a *Service) Sites() []SiteInfo {
	out := []SiteInfo{}
	if a.engine == nil || a.engine.Registry == nil {
		return out
	}
	for _, p := range a.engine.Registry.Profiles {
		home := p.Home
		if home == "" && len(p.Domains) > 0 {
			home = "https://" + p.Domains[0]
		}
		out = append(out, SiteInfo{ID: p.ID, Name: p.Name, Domains: p.Domains, Home: home, Example: p.Example,
			Status: p.Status, Fetch: p.FetchMode, Verified: p.Verified, Language: lang.Norm(p.Language)})
	}
	return out
}
