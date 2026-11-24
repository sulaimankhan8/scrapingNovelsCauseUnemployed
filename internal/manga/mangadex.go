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
	s.Type = map[string]string{"ko": "Manhwa", "zh": "Manhua", "zh-hk": "Manhua"}[a.Language]
	if s.Type == "" {
		s.Type = "Manga"
	}
	s.Title = pickLang(a.Title)
	if a.Title["en"] == "" {
		for _, alt := range a.AltTitles {
			if alt["en"] != "" {
				s.Title = alt["en"]
				break
			}
		}
	}
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
