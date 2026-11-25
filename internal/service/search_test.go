package service

import (
	"testing"

	"novelarchiver/internal/sites"
	"novelarchiver/internal/source"
)

func TestTitleKey(t *testing.T) {
	same := [][]string{
		{"Lord of the Mysteries", "Lord Of Mysteries", "LORD OF THE MYSTERIES (Web Novel)", "Lord of the Mysteries [WN]"},
		{"Shadow Slave", "shadow slave", "Shadow-Slave"},
		{"Martial Peak", "Martial Peak (Completed)"},
	}
	for _, set := range same {
		for _, t2 := range set[1:] {
			if a, b := titleKey(set[0]), titleKey(t2); a != b {
				t.Errorf("%q -> %q but %q -> %q", set[0], a, t2, b)
			}
		}
	}
	if titleKey("Shadow Slave") == titleKey("Shadow Slaves") {
		t.Error("different novels share a key")
	}
}

func TestGroupHits(t *testing.T) {
	hit := func(site, title, url string, ch int) source.SearchHit {
		return source.SearchHit{SiteID: site, SiteName: site, Title: title, URL: url, Chapters: ch}
	}
	perSite := [][]source.SearchHit{
		{hit("novelfire", "Shadow Slave", "https://novelfire.net/book/shadow-slave", 3256),
			hit("novelfire", "The Noble Queen-A Shadow Slave Fanfic", "https://novelfire.net/book/noble", 40)},
		{hit("novelfull", "Shadow Slave", "https://novelfull.com/shadow-slave.html", 3250)},
		{hit("webnovel", "Rise of the Shadow Slave", "https://www.webnovel.com/book/x_1", 0),
			hit("webnovel", "Shadow Slave", "https://www.webnovel.com/book/shadow-slave_2", 0)},
	}
	lib := map[string]string{normLink("https://www.novelfull.com/shadow-slave.html/"): "shadow-slave"}
	g := groupHits("shadow slave", perSite, lib, normLink)
	if len(g) != 3 {
		t.Fatalf("got %d groups: %+v", len(g), g)
	}
	top := g[0]
	if top.Title != "Shadow Slave" || len(top.Hits) != 3 || top.MostChapters != 3256 {
		t.Fatalf("top group = %+v", top)
	}
	if top.Hits[0].SiteID != "novelfire" || top.Hits[1].SiteID != "novelfull" || top.Hits[2].SiteID != "webnovel" {
		t.Errorf("rows not sorted by chapters: %v, %v, %v", top.Hits[0].SiteID, top.Hits[1].SiteID, top.Hits[2].SiteID)
	}
	if top.Hits[1].ExistingSlug != "shadow-slave" {
		t.Errorf("library copy not marked: %+v", top.Hits[1])
	}
	// Both other titles contain every query word; the one ranked higher on
	// its site comes first.
	if g[1].Title != "The Noble Queen-A Shadow Slave Fanfic" && g[1].Title != "Rise of the Shadow Slave" {
		t.Errorf("second group = %q", g[1].Title)
	}
}

func TestSearchStages(t *testing.T) {
	reg, err := sites.Load("")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"novelfull": StageQuick, "readnovelfull": StageQuick, "novelping": StageQuick,
		"freewebnovel": StageQuick, "novelfire": StageQuick, "wattpad": StageQuick, "ao3": StageQuick, "uukanshu": StageForeign, "syosetu": StageForeign,
		"royalroad": StageOptIn, "webnovel": StageBrowser, "scribblehub": StageOptIn,
	}
	for _, p := range reg.Profiles {
		if sites.Str(p.Search, "page") == "" {
			continue
		}
		if got := stageOf(p); got != want[p.ID] {
			t.Errorf("%s: stage %q, want %q", p.ID, got, want[p.ID])
		}
		delete(want, p.ID)
	}
	for id := range want {
		t.Errorf("%s has no search page", id)
	}
}
