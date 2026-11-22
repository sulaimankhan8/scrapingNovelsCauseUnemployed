package fetch

import "testing"

func TestLooksLikeBotCheck(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   bool
	}{
		{403, `<html><head><title>Just a moment...</title>`, true},
		{200, `<html><head><title>Just a moment...</title>`, true},
		{403, `<title>403 Forbidden</title>`, true},
		// Normal pages often carry Cloudflare's beacon script.
		{200, `<title>Mother of Learning</title><script src="/cdn-cgi/challenge-platform/scripts/x.js"></script>`, false},
		{404, `<title>Not found</title>`, false},
	}
	for _, c := range cases {
		if got := LooksLikeBotCheck(c.status, []byte(c.body)); got != c.want {
			t.Errorf("%d %q = %v, want %v", c.status, c.body, got, c.want)
		}
	}
}
