// Package source turns a pasted link into novel metadata, a chapter list and
// chapter content, driven by the site profiles in internal/sites.
package source

import "errors"

// Novel is what discovery returns.
type Novel struct {
	SiteID      string       `json:"siteId"`
	SiteName    string       `json:"siteName"`
	URL         string       `json:"url"`
	Title       string       `json:"title"`
	Author      string       `json:"author"`
	Cover       string       `json:"cover"`
	Description string       `json:"description"`
	Status      string       `json:"status,omitempty"`
	Chapters    []ChapterRef `json:"chapters"`

	// TOCMode is "list" when Chapters is the full table of contents, or
	// "crawl" when chapters are discovered by following "next" links from
	// FirstChapterURL during download.
	TOCMode         string `json:"tocMode"`
	FirstChapterURL string `json:"firstChapterUrl,omitempty"`

	// InputKind/InputChapter record what the user pasted, so the UI can
	// offer "download from this chapter onward".
	InputKind    string `json:"inputKind"`
	InputChapter int    `json:"inputChapter,omitempty"`

	// Vars are values the profile extracted (id, slug, ...) that later
	// stages need, e.g. NovelBin's numeric id for the chapter API.
	Vars map[string]string `json:"vars,omitempty"`

	// Warnings are shown to the user (paid chapters, browser needed, ...).
	Warnings []string `json:"warnings,omitempty"`
}

// ChapterRef is one entry in the table of contents.
type ChapterRef struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
	Index  int    `json:"index"` // 0-based position, used by API sources
	Locked bool   `json:"locked,omitempty"`
}

// Chapter is fetched chapter content before sanitising.
type Chapter struct {
	Title   string
	HTML    string // inner HTML of the content element, uncleaned
	NextURL string // absolute, or "" when this is the last chapter
	BaseURL string // for resolving relative image links
}

// ErrLocked means the chapter is paid or behind a login.
var ErrLocked = errors.New("chapter is locked (paid or login required)")

// ErrNoContent means the page loaded but no chapter text was found.
var ErrNoContent = errors.New("no chapter text found on the page")
