package clean

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func fixture(t *testing.T, parts ...string) string {
	b, err := os.ReadFile(filepath.Join(append([]string{"..", "..", "test-fixtures"}, parts...)...))
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	return string(b)
}

func TestKeepsFormattingAndSystemScreens(t *testing.T) {
	raw := fixture(t, "sample-chapters", "horror-game-dev_ch1_raw.html")
	r := Chapter(raw, "Chapter 1: Prologue", "https://novelbin.xyz/")
	for _, want := range []string{
		`<p class="sys"><strong>[Would you like to quit the game?]</strong></p>`,
		`<p class="sys">[▶ Yes] [▷ No]</p>`,
		`<i><strong>Click. Click.</strong></i>`,
		`urge to <i>vomit</i>`,
	} {
		if !strings.Contains(r.HTML, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Count(r.HTML, "Prologue") > 0 && strings.HasPrefix(r.HTML, "<h3>") {
		t.Errorf("duplicate title heading not removed: %.80s", r.HTML)
	}
	if r.Words < 500 {
		t.Errorf("words = %d", r.Words)
	}
}

func TestStoryWordsAreNotTreatedAsAds(t *testing.T) {
	in := `<p>The pirate ship drew closer.</p><p>Read the novel she gave him.</p><p>Read at your own risk, he said.</p>` +
		`<p>Source: novelbin.com</p><p>Read latest chapters at n0velfull . net</p><p>novelbin.me</p>`
	r := Chapter(in, "", "")
	for _, keep := range []string{"pirate ship", "Read the novel she gave him", "Read at your own risk"} {
		if !strings.Contains(r.HTML, keep) {
			t.Errorf("story line removed: %q", keep)
		}
	}
	for _, drop := range []string{"Source: novelbin", "n0velfull", "novelbin.me"} {
		if strings.Contains(r.HTML, drop) {
			t.Errorf("watermark kept: %q", drop)
		}
	}
}

func TestRoyalRoadDecoyAndTables(t *testing.T) {
	page := fixture(t, "pages", "royalroad_chapter.html")
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(page))
	content := doc.Find(".chapter-content")
	// Simulate the engine's removeHiddenByPageCss step NOT running: the
	// text-based decoy filter must still catch it.
	h, _ := content.Html()
	r := Chapter(h, "1. Good Morning Brother", "https://www.royalroad.com/")
	if strings.Contains(r.HTML, "without consent") {
		t.Errorf("royal road decoy kept")
	}
	if !strings.Contains(r.HTML, "<em>MORNING</em>") {
		t.Errorf("emphasis lost")
	}
	if strings.Contains(r.HTML, ` class="cn`) {
		t.Errorf("random classes kept")
	}
}

func TestAdsRemovedAndTitleDeduped(t *testing.T) {
	page := fixture(t, "pages", "novelfull_chapter.html")
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(page))
	h, _ := doc.Find("#chapter-content").Html()
	r := Chapter(h, "Chapter 1 - Crimson", "https://novelfull.com/")
	if strings.Contains(r.HTML, "iframe") || strings.Contains(r.HTML, "a-ads") {
		t.Errorf("ad iframe kept")
	}
	if strings.Contains(r.HTML, ">Chapter 1: Crimson<") {
		t.Errorf("title line not removed")
	}
	if !strings.Contains(r.HTML, "<em>Painful!</em>") {
		t.Errorf("italic text lost: %.400s", r.HTML)
	}
}

func TestImagesKeptAndCollected(t *testing.T) {
	r := Chapter(`<p>Map:</p><p><img data-src="/img/map.png" src="data:image/gif;base64,R0lGOD"></p><p>end of chapter text here</p>`, "", "https://site.example/novel/ch-1")
	if len(r.ImageURLs) != 1 || r.ImageURLs[0] != "https://site.example/img/map.png" {
		t.Fatalf("images = %v", r.ImageURLs)
	}
	if !strings.Contains(r.HTML, `<img src="https://site.example/img/map.png"`) {
		t.Errorf("img not kept: %s", r.HTML)
	}
}

func TestBrOnlyTextBecomesParagraphs(t *testing.T) {
	r := Chapter("First line of text.<br><br>Second line.<br/><br/>[Status Window]", "", "")
	if !strings.Contains(r.HTML, "<p>First line of text.</p>") || !strings.Contains(r.HTML, `<p class="sys">[Status Window]</p>`) {
		t.Errorf("got %s", r.HTML)
	}
}

func TestUnwrapsLayoutDivs(t *testing.T) {
	r := Chapter(`<div><div> <p>One.</p> </div></div><div><div><p>Two.</p></div></div><div class="x">Keep <b>inline</b> text</div>`, "", "")
	if strings.Contains(r.HTML, "<div><div>") || !strings.Contains(r.HTML, "<p>One.</p>") || !strings.Contains(r.HTML, "Keep <b>inline</b> text") {
		t.Errorf("got %s", r.HTML)
	}
}

func TestDropsBrBetweenParagraphs(t *testing.T) {
	r := Chapter(`<h3>Chapter 3 Turncoats</h3><br/><br/><p>One.</p><br/><p>Two<br/>lines.</p>`, "Chapter 3 Turncoats", "")
	if strings.Contains(r.HTML, "<br/><p>") || strings.HasPrefix(r.HTML, "<br") || !strings.Contains(r.HTML, "Two<br/>lines.") {
		t.Errorf("got %s", r.HTML)
	}
}

func TestCJKQuotesAreNotSystemLines(t *testing.T) {
	got := Chapter("<p>「継村さん。あなたには今月いっぱいで辞めてもらいます」</p><p>【罗恩删除了一条信息】</p><p>「Skill acquired: Appraisal」</p>", "", "").HTML
	if strings.Count(got, `class="sys"`) != 2 {
		t.Errorf("want the 【】 line and the English 「」 line boxed, not Japanese dialogue: %s", got)
	}
	if strings.Contains(got, `<p class="sys">「継村`) {
		t.Errorf("Japanese dialogue boxed: %s", got)
	}
}
