package source

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// kakuyomuTOC reads Kakuyomu's chapter list from the work page's
// __NEXT_DATA__ script (an Apollo cache): Work:<id>.tableOfContentsV2 lists
// the arcs, each arc lists its episodes in reading order. It also fills the
// title, author and description, which the page's meta tags give only in
// part.
func kakuyomuTOC(doc *goquery.Document, n *Novel) ([]ChapterRef, error) {
	if doc == nil {
		return nil, fmt.Errorf("no work page")
	}
	raw := doc.Find("script#__NEXT_DATA__").Text()
	var data struct {
		Props struct {
			PageProps struct {
				State map[string]map[string]any `json:"__APOLLO_STATE__"`
			} `json:"pageProps"`
		} `json:"props"`
		Query map[string]any `json:"query"`
	}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, fmt.Errorf("couldn't read Kakuyomu's work data: %w", err)
	}
	st := data.Props.PageProps.State
	workID := jsonString(data.Query["workId"])
	if workID == "" {
		workID = n.Vars["id"]
	}
	work := st["Work:"+workID]
	if work == nil {
		return nil, fmt.Errorf("Kakuyomu's work data has no work %s", workID)
	}
	ref := func(v any) map[string]any {
		if m, ok := v.(map[string]any); ok {
			if r, ok := m["__ref"].(string); ok {
				return st[r]
			}
		}
		return nil
	}
	if t := jsonString(work["title"]); t != "" {
		n.Title = t
	}
	if a := ref(work["author"]); a != nil {
		if name := jsonString(a["activityName"]); name != "" {
			n.Author = name
		}
	}
	if d := strings.TrimSpace(jsonString(work["introduction"])); d != "" {
		n.Description = d
	}
	switch jsonString(work["serialStatus"]) {
	case "COMPLETED":
		n.Status = "Completed"
	case "RUNNING":
		n.Status = "Ongoing"
	}
	base := "https://kakuyomu.jp/works/" + workID + "/episodes/"
	var refs []ChapterRef
	arcs, _ := work["tableOfContentsV2"].([]any)
	for _, a := range arcs {
		arc := ref(a)
		if arc == nil {
			continue
		}
		eps, _ := arc["episodeUnions"].([]any)
		for _, e := range eps {
			ep := ref(e)
			if ep == nil || jsonString(ep["id"]) == "" {
				continue
			}
			refs = append(refs, ChapterRef{Title: strings.TrimSpace(jsonString(ep["title"])), URL: base + jsonString(ep["id"])})
		}
	}
	if len(refs) == 0 {
		return nil, fmt.Errorf("Kakuyomu listed no episodes for this work")
	}
	return refs, nil
}
