package translate

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBlockedPagePausesHalfAnHour(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Redirect(w, r, "https://www.google.com/sorry/index?continue=x", http.StatusFound)
	}))
	defer srv.Close()
	g := &Google{Client: newClient(), Endpoint: srv.URL, Gap: 1}
	_, err := g.Translate(context.Background(), []string{"第一章"}, "zh")
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("err = %v, want ErrBlocked", err)
	}
	if calls != 1 {
		t.Fatalf("asked Google %d times, want 1 (no retries into a block)", calls)
	}
	if d := g.PausedFor(); d < 29*60e9 {
		t.Fatalf("paused for %v, want about 30 min", d)
	}
}
