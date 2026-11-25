package translate

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

// fake "translates" by wrapping each text, so tests see where text went.
type fake struct{ calls int }

func (f *fake) Translate(_ context.Context, texts []string, _ string) ([]string, error) {
	f.calls++
	out := make([]string, len(texts))
	for i, t := range texts {
		out[i] = "EN(" + t + ")"
	}
	return out, nil
}

func TestChapterKeepsStructure(t *testing.T) {
	frag := `<h1 class="ch-title">第1章 伦敦孤儿</h1>
<p>希恩有些意外。</p>
<p class="sys"><strong>【罗恩删除了一条信息】</strong></p>
<p>第一行<br/>第二行</p>
<p>Already English here, keep it.</p>
<p><img src="img/a.jpg"/> 图</p>`
	f := &fake{}
	res, err := Chapter(context.Background(), f, frag, "zh", map[string]string{"希恩": "Sheen"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<h1 class="ch-title">EN(第1章 伦敦孤儿)</h1>`,
		`<p>EN( Sheen 有些意外。)</p>`,
		`<p class="sys"><strong>EN(【罗恩删除了一条信息】)</strong></p>`,
		`<p>EN(第一行)<br/>EN(第二行)</p>`,
		`<p>Already English here, keep it.</p>`,
		`<img src="img/a.jpg"/> 图`,
	} {
		if !strings.Contains(res.HTML, want) {
			t.Errorf("missing %s in\n%s", want, res.HTML)
		}
	}
	if res.Title != "EN(第1章 伦敦孤儿)" {
		t.Errorf("title = %q", res.Title)
	}
	if f.calls != 1 {
		t.Errorf("want one Translate call for the whole chapter, got %d", f.calls)
	}
}

func TestGoogleBatchesAndBacksOff(t *testing.T) {
	var hits, refused atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if refused.Load() < 1 { // first request: rate limited once
			refused.Add(1)
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))
		lines := strings.Split(form.Get("q"), "\n")
		var segs []any
		for i, l := range lines {
			s := "T:" + l
			if i < len(lines)-1 {
				s += "\n"
			}
			segs = append(segs, []any{s, l})
		}
		json.NewEncoder(w).Encode([]any{segs})
	}))
	defer srv.Close()
	g := &Google{Endpoint: srv.URL, Gap: 1}
	paras := []string{"一", "二", "三"}
	got, err := g.Translate(context.Background(), paras, "zh")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, "|") != "T:一|T:二|T:三" {
		t.Errorf("got %q", got)
	}
	if hits.Load() != 2 {
		t.Errorf("want 1 refused + 1 batched request, got %d", hits.Load())
	}
}

// TestGoogleLive translates a real paragraph. NA_LIVE=1 to run.
func TestGoogleLive(t *testing.T) {
	if os.Getenv("NA_LIVE") == "" {
		t.Skip("set NA_LIVE=1 to call Google Translate")
	}
	got, err := NewGoogle().Translate(context.Background(), []string{"希恩有些意外。", "罗恩面如死灰。"}, "zh")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(got)
	if len(got) != 2 || !strings.Contains(strings.ToLower(got[1]), "ron") {
		t.Errorf("got %q", got)
	}
}
