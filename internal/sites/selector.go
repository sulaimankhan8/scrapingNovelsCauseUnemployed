package sites

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Selector is a parsed profile selector expression:
//
//	"h3.title, meta[property='og:title']@content"   try alternatives in order
//	"figure.cover img@data-src|src"                  first non-empty attribute
//	"ul.pagination li.last a@href ~ page=(\d+)"     regex; group 1 is the value
//	".pagination a@href ~ page=(\d+) (max)"         largest value across matches
type Selector struct {
	Alts  []alt
	Regex *regexp.Regexp
	Max   bool
}

type alt struct {
	CSS   string
	Attrs []string // empty means text content
}

var attrSuffix = regexp.MustCompile(`@([a-zA-Z0-9_|:-]+)$`)

// ParseSelector parses the mini-language. An empty string gives a nil selector.
func ParseSelector(expr string) *Selector {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil
	}
	s := &Selector{}
	if strings.HasSuffix(expr, "(max)") {
		s.Max = true
		expr = strings.TrimSpace(strings.TrimSuffix(expr, "(max)"))
	}
	if i := strings.Index(expr, " ~ "); i >= 0 {
		s.Regex = regexp.MustCompile(strings.TrimSpace(expr[i+3:]))
		expr = expr[:i]
	}
	for _, part := range splitTopLevel(expr) {
		a := alt{CSS: strings.TrimSpace(part)}
		if m := attrSuffix.FindStringSubmatch(a.CSS); m != nil {
			a.Attrs = strings.Split(m[1], "|")
			a.CSS = strings.TrimSpace(strings.TrimSuffix(a.CSS, m[0]))
		}
		if a.CSS != "" {
			s.Alts = append(s.Alts, a)
		}
	}
	return s
}

// splitTopLevel splits on commas that are not inside [], () or quotes.
func splitTopLevel(s string) []string {
	var parts []string
	depth, start := 0, 0
	var quote rune
	for i, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
		case r == '[' || r == '(':
			depth++
		case r == ']' || r == ')':
			depth--
		case r == ',' && depth == 0:
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return append(parts, s[start:])
}

func (a alt) value(sel *goquery.Selection) string {
	if len(a.Attrs) == 0 {
		return strings.Join(strings.Fields(sel.Text()), " ")
	}
	for _, at := range a.Attrs {
		if v, ok := sel.Attr(at); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func (s *Selector) applyRegex(v string) string {
	if s.Regex == nil || v == "" {
		return v
	}
	m := s.Regex.FindStringSubmatch(v)
	if m == nil {
		return ""
	}
	if len(m) > 1 {
		return m[1]
	}
	return m[0]
}

// First returns the first non-empty value, or "" when nothing matches.
func (s *Selector) First(root *goquery.Selection) string {
	if s == nil {
		return ""
	}
	if s.Max {
		best := -1
		for _, v := range s.All(root) {
			if n, err := strconv.Atoi(v); err == nil && n > best {
				best = n
			}
		}
		if best < 0 {
			return ""
		}
		return strconv.Itoa(best)
	}
	for _, a := range s.Alts {
		var out string
		root.Find(a.CSS).EachWithBreak(func(_ int, e *goquery.Selection) bool {
			out = s.applyRegex(a.value(e))
			return out == ""
		})
		if out != "" {
			return out
		}
	}
	return ""
}

// All returns every non-empty value from the first alternative that matches.
func (s *Selector) All(root *goquery.Selection) []string {
	if s == nil {
		return nil
	}
	for _, a := range s.Alts {
		var out []string
		root.Find(a.CSS).Each(func(_ int, e *goquery.Selection) {
			if v := s.applyRegex(a.value(e)); v != "" {
				out = append(out, v)
			}
		})
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

// Nodes returns the elements matched by the first alternative that matches.
func (s *Selector) Nodes(root *goquery.Selection) *goquery.Selection {
	if s == nil {
		return root.Slice(0, 0)
	}
	for _, a := range s.Alts {
		if found := root.Find(a.CSS); found.Length() > 0 {
			return found
		}
	}
	return root.Slice(0, 0)
}

// AttrOf returns the attribute list of the first alternative (used when a
// caller needs to know whether an expression reads an attribute).
func (s *Selector) AttrOf() []string {
	if s == nil || len(s.Alts) == 0 {
		return nil
	}
	return s.Alts[0].Attrs
}
