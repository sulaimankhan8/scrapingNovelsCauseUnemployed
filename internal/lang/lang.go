// Package lang knows the languages InkShelfs reads and translates, and
// tells them apart by the scripts a text is written in.
package lang

import (
	"html"
	"regexp"
	"strings"
	"unicode"
)

// Language codes. "" is treated as English.
const (
	English  = "en"
	Chinese  = "zh"
	Japanese = "ja"
	Korean   = "ko"
)

// Name is the language's English name ("Chinese").
func Name(code string) string {
	switch Norm(code) {
	case Chinese:
		return "Chinese"
	case Japanese:
		return "Japanese"
	case Korean:
		return "Korean"
	}
	return "English"
}

// Norm maps a code or tag ("zh-TW", "ja-JP", "") to one of the constants.
func Norm(code string) string {
	c := strings.ToLower(strings.TrimSpace(code))
	switch {
	case strings.HasPrefix(c, "zh"):
		return Chinese
	case strings.HasPrefix(c, "ja"):
		return Japanese
	case strings.HasPrefix(c, "ko"):
		return Korean
	}
	return English
}

// NeedsTranslation reports whether text in this language is translated
// to English.
func NeedsTranslation(code string) bool { return Norm(code) != English }

var tags = regexp.MustCompile(`<[^>]*>`)

// Detect names the language a text (or HTML fragment) is written in, from
// the scripts it uses: Hangul means Korean; hiragana or katakana mean
// Japanese; Chinese characters without either mean Chinese; mostly Latin
// letters mean English. It returns "" when there are too few letters to
// tell.
func Detect(text string) string {
	if strings.Contains(text, "<") {
		text = html.UnescapeString(tags.ReplaceAllString(text, " "))
	}
	var han, kana, hangul, latin int
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Hangul, r):
			hangul++
		case unicode.Is(unicode.Hiragana, r), unicode.Is(unicode.Katakana, r):
			kana++
		case unicode.Is(unicode.Han, r):
			han++
		case unicode.Is(unicode.Latin, r):
			latin++
		}
	}
	total := han + kana + hangul + latin
	if total < 20 {
		return ""
	}
	cjk := han + kana + hangul
	switch {
	case hangul*5 >= total: // Korean mixes in a few Chinese characters and Latin words
		return Korean
	case kana*20 >= total && (kana+han)*3 >= total:
		return Japanese
	case han*3 >= total && kana*50 < total:
		return Chinese
	case latin*2 >= total && cjk*10 < total:
		return English
	}
	return ""
}
