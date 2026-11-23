package source

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

// Royal Road's table is cut to 10 visible rows by the page's own script,
// but window.chapters keeps the full list.
func TestScriptJSONChaptersBeatsPaginatedTable(t *testing.T) {
	var rows, items []string
	for i := 1; i <= 109; i++ {
		if i <= 10 {
			rows = append(rows, fmt.Sprintf(`<tr class="chapter-row"><td><a href="/fiction/1/x/chapter/%d/c">%d</a></td></tr>`, i, i))
		}
		unlocked := "true"
		if i == 109 {
			unlocked = "false"
		}
		items = append(items, fmt.Sprintf(`{"title":"%d. Title","url":"/fiction/1/x/chapter/%d/c","isUnlocked":%s}`, i, i, unlocked))
	}
	html := `<html><body><table id="chapters">` + strings.Join(rows, "") + `</table>
<script>window.fictionId = 1; window.chapters = [` + strings.Join(items, ",") + `]; window.other = 2;</script></body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	doc.Url, _ = url.Parse("https://www.royalroad.com/fiction/1/x")
	refs := scriptJSONChapters(doc, "window.chapters", "isUnlocked")
	if len(refs) != 109 {
		t.Fatalf("got %d chapters, want 109", len(refs))
	}
	if refs[0].URL != "https://www.royalroad.com/fiction/1/x/chapter/1/c" || refs[0].Title != "1. Title" {
		t.Fatalf("first = %+v", refs[0])
	}
	if refs[108].Locked != true || refs[0].Locked {
		t.Fatalf("locked flags wrong: first=%v last=%v", refs[0].Locked, refs[108].Locked)
	}
	if got := scriptJSONChapters(doc, "window.missing", ""); got != nil {
		t.Fatalf("missing variable should give nothing, got %d", len(got))
	}
}
