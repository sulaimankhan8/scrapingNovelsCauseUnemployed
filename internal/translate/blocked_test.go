package translate

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

func TestRefusalsInARowPauseLonger(t *testing.T) {
	g := &Google{}
	var got []time.Duration
	for i := 0; i < 5; i++ {
		g.strike(blockedPauses)
		got = append(got, g.PausedFor().Round(time.Minute))
	}
	want := []time.Duration{30 * time.Minute, time.Hour, 2 * time.Hour, 4 * time.Hour, 4 * time.Hour}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pauses %v, want %v", got, want)
		}
	}
	// An hour after the last refusal, the next one starts small again.
	g.lastStrike = time.Now().Add(-2 * time.Hour)
	g.strike(busyPauses)
	if d := g.PausedFor().Round(time.Minute); d != 3*time.Minute {
		t.Fatalf("after a quiet hour the pause was %v, want 3m", d)
	}
}
