package manga

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// MangaDex has an official public API (api.mangadex.org). It asks clients
// to identify themselves rather than pose as a browser; the profile's
// User-Agent header does that.
const mdAPI = "https://api.mangadex.org"

type mdRel struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Attributes map[string]any `json:"attributes"`
}

func (e *Engine) mdGet(ctx context.Context, p *Profile, path string, out any) error {
	pg, err := e.get(ctx, p, Request{URL: mdAPI + path})
	if err != nil {
		return err
	}
	return json.Unmarshal(pg.Body, out)
}

func (e *Engine) mangadexDiscover(ctx context.Context, p *Profile, m *Match, status StatusFunc) (*Series, error) {
	id := m.Captures["id"]
	if m.Chapter {
		status("That's a chapter link; finding the series…")
		var ch struct {
			Data struct {
				Relationships []mdRel `json:"relationships"`
			} `json:"data"`
		}
		if err := e.mdGet(ctx, p, "/chapter/"+m.Captures["cid"], &ch); err != nil {
			return nil, err
		}
		for _, r := range ch.Data.Relationships {
			if r.Type == "manga" {
				id = r.ID
			}
		}
		if id == "" {
			return nil, fmt.Errorf("couldn't find the series for that MangaDex chapter")
		}
	}
	status("Reading series details from MangaDex…")
	var md struct {
		Data struct {
			ID         string `json:"id"`
			Attributes struct {
				Title       map[string]string   `json:"title"`
				AltTitles   []map[string]string `json:"altTitles"`
				Description map[string]string   `json:"description"`
				Status      string              `json:"status"`
				Language    string              `json:"originalLanguage"`
			} `json:"attributes"`
			Relationships []mdRel `json:"relationships"`
		} `json:"data"`
	}
	if err := e.mdGet(ctx, p, "/manga/"+id+"?includes[]=cover_art&includes[]=author", &md); err != nil {
		return nil, err
	}
	a := md.Data.Attributes
	s := &Series{SiteID: p.ID, SiteName: p.Name, URL: "https://mangadex.org/title/" + id,
		Vars: map[string]string{"id": id, "seriesUrl": "https://mangadex.org/title/" + id}}
	s.Type = mdType(a.Language)
	s.Title = mdTitle(a.Title, a.AltTitles)
	s.Description = clip(pickLang(a.Description), 4000)
	s.Status = a.Status
	var authors []string
	for _, r := range md.Data.Relationships {
		switch r.Type {
		case "author":
			if n, _ := r.Attributes["name"].(string); n != "" {
				authors = append(authors, n)
			}
		case "cover_art":
			if f, _ := r.Attributes["fileName"].(string); f != "" {
				s.Cover = "https://uploads.mangadex.org/covers/" + id + "/" + f + ".512.jpg"
			}
		}
	}
	s.Author = strings.Join(authors, ", ")

	status("Building the chapter list…")
	seen := map[string]bool{}
	var refs []ChapterRef
	external := 0
	for offset := 0; offset < 10000; offset += 500 {
		var feed struct {
			Data []struct {
				ID         string `json:"id"`
				Attributes struct {
					Volume   string `json:"volume"`
					Chapter  string `json:"chapter"`
					Title    string `json:"title"`
					External string `json:"externalUrl"`
					Pages    int    `json:"pages"`
				} `json:"attributes"`
			} `json:"data"`
			Total int `json:"total"`
		}
		q := url.Values{}
		q.Set("limit", "500")
		q.Set("offset", strconv.Itoa(offset))
		q.Add("translatedLanguage[]", "en")
		q.Set("order[volume]", "asc")
		q.Set("order[chapter]", "asc")
		for _, r := range []string{"safe", "suggestive", "erotica"} {
			q.Add("contentRating[]", r)
		}
		if err := e.mdGet(ctx, p, "/manga/"+id+"/feed?"+q.Encode(), &feed); err != nil {
			return nil, err
		}
		for _, c := range feed.Data {
			at := c.Attributes
			if at.External != "" || at.Pages == 0 {
				external++
				continue
			}
			key := at.Volume + "/" + at.Chapter
			if at.Chapter == "" {
				key = c.ID // oneshots: keep each
			}
			if seen[key] {
				continue // the same chapter from another group
			}
			seen[key] = true
			title := "Oneshot"
			if at.Chapter != "" {
				title = "Chapter " + at.Chapter
			}
			if at.Title != "" {
				title += ": " + at.Title
			}
			refs = append(refs, ChapterRef{Title: title, URL: "https://mangadex.org/chapter/" + c.ID, ID: c.ID})
		}
		if offset+500 >= feed.Total {
			break
		}
	}
	s.Chapters = numberChapters(refs)
	if external > 0 {
		s.Warnings = append(s.Warnings, fmt.Sprintf("%d chapters are only on the official publisher's site and were left out.", external))
	}
	if len(s.Chapters) == 0 {
		return nil, fmt.Errorf("this series has no English chapters hosted on MangaDex")
	}
	if m.Chapter {
		for _, c := range s.Chapters {
			if c.ID == m.Captures["cid"] {
				s.InputChapter = c.Number
			}
		}
	}
	return s, nil
}

func (e *Engine) mangadexPages(ctx context.Context, p *Profile, ch ChapterRef) ([]string, error) {
	id := ch.ID
	if id == "" {
		id = ch.URL[strings.LastIndex(ch.URL, "/")+1:]
	}
	var ah struct {
		BaseURL string `json:"baseUrl"`
		Chapter struct {
			Hash string   `json:"hash"`
			Data []string `json:"data"`
		} `json:"chapter"`
	}
	if err := e.mdGet(ctx, p, "/at-home/server/"+id, &ah); err != nil {
		return nil, err
	}
	if ah.BaseURL == "" || len(ah.Chapter.Data) == 0 {
		return nil, ErrNoPages
	}
	out := make([]string, len(ah.Chapter.Data))
	for i, f := range ah.Chapter.Data {
		out[i] = ah.BaseURL + "/data/" + ah.Chapter.Hash + "/" + f
	}
	return out, nil
}

func pickLang(m map[string]string) string {
	for _, k := range []string{"en", "ja-ro", "ko-ro", "zh-ro", "ja", "ko", "zh"} {
		if v := m[k]; v != "" {
			return v
		}
	}
	for _, v := range m {
		return v
	}
	return ""
}

// mangadexSearch asks the MangaDex API for series by title. Only series
// with English chapters hosted on MangaDex are listed, since those are the
// only ones that can be downloaded.
func (e *Engine) mangadexSearch(ctx context.Context, p *Profile, query string) ([]SearchHit, error) {
	q := url.Values{}
	q.Set("title", query)
	q.Set("limit", strconv.Itoa(maxSearchHits))
	q.Add("includes[]", "cover_art")
	q.Add("includes[]", "author")
	for _, r := range []string{"safe", "suggestive", "erotica"} {
		q.Add("contentRating[]", r)
	}
	q.Add("availableTranslatedLanguage[]", "en")
	q.Set("hasAvailableChapters", "true")
	q.Set("order[relevance]", "desc")
	pg, err := e.get(ctx, p, Request{URL: mdAPI + "/manga?" + q.Encode()})
	if err != nil {
		return nil, err
	}
	return parseMangadexSearch(p, pg.Body)
}

func parseMangadexSearch(p *Profile, body []byte) ([]SearchHit, error) {
	var res struct {
		Result string `json:"result"`
		Data   []struct {
			ID         string `json:"id"`
			Attributes struct {
				Title       map[string]string   `json:"title"`
				AltTitles   []map[string]string `json:"altTitles"`
				Language    string              `json:"originalLanguage"`
				LastChapter string              `json:"lastChapter"`
				Status      string              `json:"status"`
			} `json:"attributes"`
			Relationships []mdRel `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("MangaDex search returned invalid JSON: %w", err)
	}
	if res.Result != "ok" {
		return nil, fmt.Errorf("MangaDex search failed (%s)", res.Result)
	}
	var hits []SearchHit
	for _, d := range res.Data {
		a := d.Attributes
		h := SearchHit{SiteID: p.ID, SiteName: p.Name, URL: "https://mangadex.org/title/" + d.ID,
			Title: mdTitle(a.Title, a.AltTitles), Type: mdType(a.Language)}
		var authors []string
		for _, r := range d.Relationships {
			switch r.Type {
			case "author":
				if n, _ := r.Attributes["name"].(string); n != "" {
					authors = append(authors, n)
				}
			case "cover_art":
				if f, _ := r.Attributes["fileName"].(string); f != "" {
					h.Cover = "https://uploads.mangadex.org/covers/" + d.ID + "/" + f + ".256.jpg"
				}
			}
		}
		h.Author = strings.Join(authors, ", ")
		// lastChapter is the series' final chapter, set once it has
		// ended; the chapters hosted in English may be fewer, so it's
		// shown as the latest, not as a count.
		if a.LastChapter != "" {
			h.Latest = "Chapter " + a.LastChapter
		}
		if h.Title != "" {
			hits = append(hits, h)
		}
	}
	return hits, nil
}

// mdTitle is the English title when MangaDex has one, else the romanised one.
func mdTitle(title map[string]string, alts []map[string]string) string {
	if t := title["en"]; t != "" {
		return t
	}
	for _, alt := range alts {
		if alt["en"] != "" {
			return alt["en"]
		}
	}
	return pickLang(title)
}

func mdType(lang string) string {
	switch lang {
	case "ko":
		return "Manhwa"
	case "zh", "zh-hk":
		return "Manhua"
	}
	return "Manga"
}
