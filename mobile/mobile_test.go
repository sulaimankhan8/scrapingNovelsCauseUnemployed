package mobile

import (
	"encoding/json"
	"strings"
	"testing"
)

type sink struct{ events []string }

func (s *sink) Emit(event, payload string) { s.events = append(s.events, event+" "+payload) }

func TestCallAndServe(t *testing.T) {
	c, err := New(t.TempDir(), &sink{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := c.Call("GetNovels", "[]")
	if err != nil || out != "[]" {
		t.Fatalf("GetNovels = %q, %v", out, err)
	}
	out, err = c.Call("GetSystemStats", "")
	if err != nil {
		t.Fatal(err)
	}
	var stats map[string]any
	if json.Unmarshal([]byte(out), &stats) != nil || stats["totalNovels"].(float64) != 0 {
		t.Fatalf("stats = %s", out)
	}
	if _, err := c.Call("Nope", "[]"); err == nil {
		t.Errorf("unknown method accepted")
	}
	if _, err := c.Call("DeleteChapters", `["missing", 1, 2]`); err == nil {
		t.Errorf("delete on missing novel succeeded")
	}
	r := c.Serve("/library/missing/")
	if r.Status != 404 || !strings.HasPrefix(r.MimeType, "text/") {
		t.Errorf("serve = %d %s", r.Status, r.MimeType)
	}
}
