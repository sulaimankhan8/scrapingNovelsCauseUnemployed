package service

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings are the user's preferences. They live next to the library
// (settings.json), so they survive clearing the WebView's data and travel
// in backups.
type Settings struct {
	Theme        string `json:"theme"`        // light, sepia, dark, system
	Chime        bool   `json:"chime"`        // sound when a download finishes
	Notify       bool   `json:"notify"`       // system notifications
	Speed        string `json:"speed"`        // gentle, balanced, fast
	KeepImages   bool   `json:"keepImages"`   // default for new novels
	Revise       string `json:"revise"`       // all, recent, off
	UpdateOnOpen bool   `json:"updateOnOpen"` // update all when the app opens
	WifiOnly     bool   `json:"wifiOnly"`     // Android: pause on mobile data
}

// DefaultSettings are used until the user changes something.
func DefaultSettings() Settings {
	return Settings{Theme: "system", Chime: true, Notify: true, Speed: "balanced",
		KeepImages: true, Revise: "all", WifiOnly: true}
}

func (s Settings) speed() int {
	switch s.Speed {
	case "gentle":
		return 2
	case "fast":
		return 8
	}
	return 4
}

func (s Settings) reviseCount() int {
	switch s.Revise {
	case "off":
		return 0
	case "recent":
		return 100
	}
	return -1
}

func (s *Settings) normalise() {
	d := DefaultSettings()
	switch s.Theme {
	case "light", "sepia", "dark", "system":
	default:
		s.Theme = d.Theme
	}
	switch s.Speed {
	case "gentle", "balanced", "fast":
	default:
		s.Speed = d.Speed
	}
	switch s.Revise {
	case "all", "recent", "off":
	default:
		s.Revise = d.Revise
	}
}

func (a *Service) settingsPath() string {
	if a.store == nil {
		return ""
	}
	return filepath.Join(filepath.Dir(a.store.Root), "settings.json")
}

func (a *Service) loadSettings() {
	a.settings = DefaultSettings()
	p := a.settingsPath()
	if p == "" {
		return
	}
	if data, err := os.ReadFile(p); err == nil {
		s := DefaultSettings()
		if json.Unmarshal(data, &s) == nil {
			s.normalise()
			a.settings = s
		}
	}
}

// GetSettings returns the saved preferences.
func (a *Service) GetSettings() Settings {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings
}

// SaveSettings stores the preferences.
func (a *Service) SaveSettings(s Settings) (Settings, error) {
	if err := a.ready(); err != nil {
		return s, err
	}
	s.normalise()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return s, err
	}
	p := a.settingsPath()
	if err := os.WriteFile(p+".tmp", data, 0o644); err != nil {
		return s, err
	}
	if err := os.Rename(p+".tmp", p); err != nil {
		return s, err
	}
	a.mu.Lock()
	a.settings = s
	a.mu.Unlock()
	return s, nil
}
