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
