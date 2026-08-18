package score

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// containsPhrase reports whether hay contains phrase bounded by non-word
// characters on both sides. Both arguments must already be lowercased.
//
// Plain strings.Contains is wrong here: the keyword list contains "api", which
// appears inside "rapid", "capital" and "therapist", and "code", which appears
// inside "decode" and "encoded". Unbounded matching fires the builder signal on
// nearly every article and flattens the ranking.
func containsPhrase(hay, phrase string) bool {
	if phrase == "" || len(phrase) > len(hay) {
		return false
	}
	for offset := 0; offset <= len(hay)-len(phrase); {
		i := strings.Index(hay[offset:], phrase)
		if i < 0 {
			return false
		}
		start := offset + i
		end := start + len(phrase)
		if !wordRuneBefore(hay, start) && !wordRuneAt(hay, end) {
			return true
		}
		offset = start + 1
	}
	return false
}

// matchAll returns every phrase present in hay, deduplicated and sorted, so the
// signal list is stable across runs regardless of config file ordering.
func matchAll(hay string, phrases []string) []string {
	var found []string
	seen := map[string]bool{}
	for _, p := range phrases {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" || seen[p] {
			continue
		}
		if containsPhrase(hay, p) {
			seen[p] = true
			found = append(found, p)
		}
	}
	sortStrings(found)
	return found
}

func wordRuneBefore(s string, i int) bool {
	if i <= 0 {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(s[:i])
	return isWordRune(r)
}

func wordRuneAt(s string, i int) bool {
	if i < 0 || i >= len(s) {
		return false
	}
	r, _ := utf8.DecodeRuneInString(s[i:])
	return isWordRune(r)
}

// isWordRune treats letters and digits as word characters. Hyphens and dots are
// boundaries, so "open-source" still matches inside "non-open-source", and
// "fine-tune" does not match inside "fine-tuning" — which is listed separately.
func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
