package normalize

import (
	"html"
	"regexp"
	"strings"
	"unicode"
)

// StripHTML removes markup and returns readable plain text.
//
// Tags become spaces rather than nothing, so that "<p>one</p><p>two</p>" reads
// as "one two" and not "onetwo". Script and style bodies are dropped entirely.
func StripHTML(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for i := 0; i < len(s); {
		if s[i] != '<' {
			b.WriteByte(s[i])
			i++
			continue
		}

		close := strings.IndexByte(s[i:], '>')
		if close < 0 {
			// Unterminated tag. Everything after it is markup we cannot read.
			break
		}
		name := tagName(s[i+1 : i+close])
		end := i + close + 1

		if name == "script" || name == "style" {
			b.WriteByte(' ')
			if skip := skipElement(s, end, name); skip > 0 {
				i = skip
				continue
			}
			// No closing tag; the rest of the document is inside the element.
			break
		}

		// Only block-level boundaries separate words. An inline tag must not,
		// or "<b>summary</b>." would come out as "summary .".
		if !inlineTags[name] {
			b.WriteByte(' ')
		}
		i = end
	}

	return CollapseSpace(html.UnescapeString(b.String()))
}

// skipElement returns the offset just past the closing tag for name, or -1.
func skipElement(s string, from int, name string) int {
	rest := strings.ToLower(s[from:])
	at := strings.Index(rest, "</"+name)
	if at < 0 {
		return -1
	}
	close := strings.IndexByte(s[from+at:], '>')
	if close < 0 {
		return -1
	}
	return from + at + close + 1
}

// inlineTags are elements that sit inside a run of text rather than breaking
// it. Removing one must not introduce a space.
var inlineTags = map[string]bool{
	"a": true, "abbr": true, "b": true, "bdi": true, "bdo": true, "cite": true,
	"code": true, "data": true, "dfn": true, "em": true, "i": true, "kbd": true,
	"mark": true, "q": true, "rp": true, "rt": true, "ruby": true, "s": true,
	"samp": true, "small": true, "span": true, "strong": true, "sub": true,
	"sup": true, "time": true, "tt": true, "u": true, "var": true,
}

// tagName extracts the lowercase element name from the inside of a tag.
func tagName(inner string) string {
	inner = strings.TrimPrefix(strings.TrimSpace(inner), "/")
	end := strings.IndexFunc(inner, func(r rune) bool {
		return unicode.IsSpace(r) || r == '/' || r == '>'
	})
	if end >= 0 {
		inner = inner[:end]
	}
	return strings.ToLower(inner)
}

// Runes that feeds emit but unicode.IsSpace does not cover.
const (
	zeroWidthSpace = rune(0x200B)
	byteOrderMark  = rune(0xFEFF)
)

// CollapseSpace reduces every run of whitespace to a single space and trims the
// ends. Non-breaking spaces are covered by unicode.IsSpace; zero-width spaces
// and byte-order marks are not, and feeds emit both.
func CollapseSpace(s string) string {
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || r == zeroWidthSpace || r == byteOrderMark
	}), " ")
}

// Excerpt turns raw feed content into plain text capped at maxChars runes.
//
// This is where R1 is enforced. Every Item is built through Build, which calls
// this, so no code path can persist more source text than the cap allows.
func Excerpt(s string, maxChars int) string {
	if maxChars <= 0 {
		return ""
	}
	text := StripHTML(s)

	runes := []rune(text)
	if len(runes) <= maxChars {
		return text
	}

	// Leave room for the ellipsis so the result never exceeds the cap.
	cut := runes[:maxChars-1]
	if at := lastSpace(cut); at > maxChars/2 {
		cut = cut[:at]
	}

	trimmed := strings.TrimRightFunc(string(cut), func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(",;:-–—(", r)
	})
	return trimmed + "…"
}

func lastSpace(rs []rune) int {
	for i := len(rs) - 1; i >= 0; i-- {
		if unicode.IsSpace(rs[i]) {
			return i
		}
	}
	return -1
}

var (
	// " ... (arXiv:2401.12345v1 [cs.LG])" — arXiv appends this to every title.
	arxivSuffixRE = regexp.MustCompile(`(?i)\s*\(\s*arxiv:[^)]*\)\s*\.?\s*$`)

	// A trailing " | Site Name". The pipe is a reliable site-name separator;
	// dashes are not, so they are only stripped when they match the source.
	pipeSuffixRE = regexp.MustCompile(`\s*[|·•]\s*[^|·•]{1,40}$`)
)

// titleSeparators are the characters a feed puts between an article title and
// its publication name.
var titleSeparators = []string{" | ", " - ", " – ", " — ", " · ", " • ", " :: "}

// CleanTitle strips the publication name and feed boilerplate that sources
// append to titles, so that two outlets covering one event produce comparable
// titles.
//
// sourceName may be empty; it makes dash-separated suffixes safe to remove,
// which they otherwise are not — "Llama 4 - faster and cheaper" must survive.
func CleanTitle(title, sourceName string) string {
	t := CollapseSpace(html.UnescapeString(title))
	t = arxivSuffixRE.ReplaceAllString(t, "")

	// An exact match on the source name is unambiguous, whatever the separator.
	if sourceName != "" {
		for _, sep := range titleSeparators {
			suffix := sep + sourceName
			if len(t) > len(suffix) && strings.EqualFold(t[len(t)-len(suffix):], suffix) {
				t = t[:len(t)-len(suffix)]
				break
			}
		}
	}

	// A pipe almost always separates the title from the site name, and a title
	// that is nothing but a site name is not worth salvaging.
	if loc := pipeSuffixRE.FindStringIndex(t); loc != nil && loc[0] > 0 {
		t = t[:loc[0]]
	}

	t = strings.TrimRightFunc(t, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune("|-–—·•", r)
	})
	return CollapseSpace(t)
}

// WordCount counts whitespace-separated words. Used by the summary limits in
// R1 and by the digest.
func WordCount(s string) int {
	return len(strings.Fields(s))
}
