package fetch

import (
	"bytes"
	"mime"
	"regexp"
	"strings"

	"golang.org/x/net/html/charset"
)

var metaCharset = regexp.MustCompile(`(?i)<meta[^>]+charset\s*=\s*["']?\s*([a-z0-9_.:-]+)`)

// pageCharset names the text encoding a page declares: the Content-Type
// header's charset, else a <meta charset> near the top of the page. "" means
// none was declared.
func pageCharset(body []byte, contentType string) string {
	if _, params, err := mime.ParseMediaType(contentType); err == nil && params["charset"] != "" {
		return strings.ToLower(params["charset"])
	}
	head := body[:min(len(body), 4096)]
	if m := metaCharset.FindSubmatch(head); m != nil {
		return strings.ToLower(string(m[1]))
	}
	return ""
}

// ToUTF8 converts a text page that declares another encoding (Chinese
// sites often use GBK or GB18030, Japanese Shift_JIS, Korean EUC-KR) to
// UTF-8, which the rest of the app expects. Binary responses, UTF-8 pages
// and pages that declare nothing are returned unchanged.
func ToUTF8(body []byte, contentType string) []byte {
	ct := strings.ToLower(contentType)
	if ct != "" && !strings.HasPrefix(ct, "text/") && !strings.Contains(ct, "html") && !strings.Contains(ct, "xml") {
		return body
	}
	label := pageCharset(body, contentType)
	if label == "" {
		return body
	}
	enc, name := charset.Lookup(label)
	if enc == nil || name == "utf-8" {
		return body
	}
	out, err := enc.NewDecoder().Bytes(body)
	if err != nil {
		return body
	}
	// The page still says it's GBK; say UTF-8 so nothing decodes it twice.
	if loc := metaCharset.FindSubmatchIndex(out[:min(len(out), 4096)]); loc != nil {
		out = append(append(append([]byte{}, out[:loc[2]]...), "utf-8"...), out[loc[3]:]...)
	}
	return bytes.TrimPrefix(out, []byte("\xef\xbb\xbf"))
}
