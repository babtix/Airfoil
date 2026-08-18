package write

import (
	"fmt"
	"strings"
	"time"
	"unicode"
)

// slugMaxChars bounds the title portion of a slug. The date prefix is extra.
const slugMaxChars = 60

// Slug builds the filename stem: YYYY-MM-DD-kebab-title.
//
// The slug is the story's permanent URL, so it is derived only from the date
// and title and never from anything that changes between runs.
func Slug(date time.Time, title string) string {
	return date.UTC().Format(time.DateOnly) + "-" + kebab(title)
}

// kebab lowercases and reduces to [a-z0-9-], collapsing runs of separators and
// truncating on a word boundary so a slug never ends mid-word.
func kebab(s string) string {
	var b strings.Builder
	lastDash := true // leading dashes are suppressed

	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			// Non-ASCII letters are dropped rather than transliterated:
			// guessing a romanization produces worse URLs than omitting.
			if r < unicode.MaxASCII {
				b.WriteRune(r)
				lastDash = false
			}
		case !lastDash:
			b.WriteByte('-')
			lastDash = true
		}
	}

	out := strings.Trim(b.String(), "-")
	if len(out) <= slugMaxChars {
		return orFallback(out)
	}

	out = out[:slugMaxChars]
	// Truncate at the last separator so the slug ends on a whole word.
	if i := strings.LastIndexByte(out, '-'); i > 0 {
		out = out[:i]
	}
	return orFallback(strings.Trim(out, "-"))
}

// orFallback guarantees a non-empty stem. A title of only punctuation or
// non-ASCII script would otherwise produce a file named just the date.
func orFallback(s string) string {
	if s == "" {
		return "story"
	}
	return s
}

// DedupeSlug appends -2, -3, … until the slug is unused.
//
// taken reports whether a slug is already claimed, which lets the caller check
// both the slugs written earlier in this run and the files already on disk.
func DedupeSlug(slug string, taken func(string) bool) string {
	if !taken(slug) {
		return slug
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s-%d", slug, n)
		if !taken(candidate) {
			return candidate
		}
	}
}
