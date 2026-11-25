package fetch

import (
	"strings"
	"testing"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestToUTF8(t *testing.T) {
	gbk, _ := simplifiedchinese.GBK.NewEncoder().String(`<html><head><meta charset="gbk"><title>霍格沃茨的学习面板</title></head><body>第568章 任意门</body></html>`)
	sjis, _ := japanese.ShiftJIS.NewEncoder().String(`<meta http-equiv="Content-Type" content="text/html; charset=Shift_JIS"><p>異世界に転生した</p>`)
	euckr, _ := korean.EUCKR.NewEncoder().String(`<p>전지적 독자 시점</p>`)
	cases := []struct {
		name, body, ct, want string
	}{
		{"gbk meta", gbk, "text/html", "第568章 任意门"},
		{"gbk header", gbk, "text/html; charset=GBK", "霍格沃茨的学习面板"},
		{"shift_jis meta", sjis, "", "異世界に転生した"},
		{"euc-kr header", euckr, "text/html; charset=euc-kr", "전지적 독자 시점"},
		{"utf-8 untouched", "<p>诡秘之主 — ok</p>", "text/html; charset=utf-8", "诡秘之主 — ok"},
		{"no charset untouched", "<p>plain café</p>", "text/html", "plain café"},
	}
	for _, c := range cases {
		got := string(ToUTF8([]byte(c.body), c.ct))
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: got %q, want it to contain %q", c.name, got, c.want)
		}
	}
	if got := string(ToUTF8([]byte(gbk), "text/html")); !strings.Contains(got, `charset="utf-8"`) {
		t.Errorf("meta charset not rewritten: %q", got[:60])
	}
	img := []byte{0xff, 0xd8, 0xff, 0xe0}
	if got := ToUTF8(img, "image/jpeg"); string(got) != string(img) {
		t.Error("binary changed")
	}
}
