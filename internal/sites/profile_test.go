package sites

import (
	"strings"
	"testing"
)

func TestNovelBinLinksMoveToCanonicalHost(t *testing.T) {
	reg, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, in := range []string{
		"https://novelbin.me/novel/investing-in-my-crippled-wife-every-return-makes-me-stronger",
		"http://www.novelbin.com/novel/investing-in-my-crippled-wife-every-return-makes-me-stronger",
	} {
		m, err := reg.Match(in)
		if err != nil {
			t.Fatal(err)
		}
		if m.Profile == nil || m.Profile.ID != "novelbin" {
			t.Fatalf("%s: not matched to novelbin", in)
		}
		if !strings.HasPrefix(m.URL, "https://novelbin.xyz/novel/") || m.Origin() != "https://novelbin.xyz" {
			t.Fatalf("%s -> %s (origin %s), want novelbin.xyz", in, m.URL, m.Origin())
		}
	}
	// Sites without a canonical host keep their link.
	m, err := reg.Match("https://www.empirenovel.com/novel/some-novel")
	if err != nil {
		t.Fatal(err)
	}
	if m.URL != "https://www.empirenovel.com/novel/some-novel" {
		t.Fatalf("empirenovel link changed: %s", m.URL)
	}
}

func TestNovelKey(t *testing.T) {
	reg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	same := [][2]string{
		{"https://www.webnovel.com/book/32824130400116105", "https://www.webnovel.com/book/cleaver-of-sin_32824130400116105"},
		{"https://novelfull.com/shadow-slave.html", "http://www.novelfull.com/shadow-slave.html"},
		{"https://www.wattpad.com/story/32736", "https://www.wattpad.com/story/32736-the-vampire%27s-maid"},
		{"https://example.com/a/", "http://www.example.com/a"},
	}
	for _, c := range same {
		if a, b := reg.NovelKey(c[0]), reg.NovelKey(c[1]); a != b {
			t.Errorf("%s -> %q, %s -> %q", c[0], a, c[1], b)
		}
	}
	if reg.NovelKey("https://www.wattpad.com/story/1") == reg.NovelKey("https://www.wattpad.com/story/2") {
		t.Error("different stories share a key")
	}
}
