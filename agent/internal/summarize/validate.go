package summarize

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/model"
	"github.com/papitsho/airfoil/internal/normalize"
)

// Response is the schema from docs/PROMPTS.md §1.
type Response struct {
	Title           string   `json:"title"`
	Summary         string   `json:"summary"`
	Takeaways       []string `json:"takeaways"`
	Tags            []string `json:"tags"`
	BuilderRelevant bool     `json:"builder_relevant"`
	Confidence      string   `json:"confidence"`
}

// Confidence levels.
const (
	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"
)

// AllowedTags is the closed vocabulary from PROMPTS.md. A model that invents a
// tag gets it dropped rather than failing the whole story.
var AllowedTags = map[string]bool{
	"models": true, "agents": true, "rag": true, "infra": true,
	"open-source": true, "research": true, "policy": true, "business": true,
	"safety": true, "coding": true, "tools": true, "hardware": true,
}

// titleMaxChars comes from the schema in PROMPTS.md §1.
const titleMaxChars = 80

// ErrInvalid marks a response that failed validation. The summarizer retries
// once on this, then skips the cluster.
var ErrInvalid = errors.New("summarize: invalid response")

// Parse extracts the JSON object from a model response.
//
// The prompt forbids markdown fences, but models emit them anyway, so the
// fence stripping is defensive rather than optional.
func Parse(raw string) (Response, error) {
	s := stripFences(strings.TrimSpace(raw))

	var r Response
	if err := json.Unmarshal([]byte(s), &r); err == nil {
		return r, nil
	}

	// Models wrap the object in prose on either side — "Here is the JSON:"
	// before it, "Hope that helps!" after. Retry on the outermost braces
	// before giving up.
	i, j := strings.Index(s, "{"), strings.LastIndex(s, "}")
	if i < 0 || j <= i {
		return Response{}, fmt.Errorf("%w: no JSON object found", ErrInvalid)
	}
	if err := json.Unmarshal([]byte(s[i:j+1]), &r); err != nil {
		return Response{}, fmt.Errorf("%w: not JSON: %v", ErrInvalid, err)
	}
	return r, nil
}

func stripFences(s string) string {
	if !strings.HasPrefix(s, "```") {
		return s
	}
	// Drop the opening fence line (```json or plain ```) and the closing fence.
	if nl := strings.IndexByte(s, '\n'); nl >= 0 {
		s = s[nl+1:]
	}
	if i := strings.LastIndex(s, "```"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// Validate enforces every rule in PROMPTS.md that Go can check, and reports
// the tags it dropped. A returned error means the cluster should be retried
// once and then skipped — never written.
//
// The cleaned Response is returned by value, so a caller cannot accidentally
// write the unvalidated original.
func Validate(r Response, items []model.Item, limits config.Scoring) (Response, error) {
	lim := limits.Limits

	r.Title = strings.TrimSpace(r.Title)
	r.Summary = strings.TrimSpace(r.Summary)

	if r.Title == "" {
		return r, fmt.Errorf("%w: empty title", ErrInvalid)
	}
	if r.Summary == "" {
		return r, fmt.Errorf("%w: empty summary", ErrInvalid)
	}

	// Titles are trimmed rather than rejected: an over-long title is cosmetic,
	// while a rejected story is lost entirely.
	r.Title = strings.TrimRight(trimRunes(r.Title, titleMaxChars), " .,;:—-")

	// R1: word cap on the summary.
	if n := normalize.WordCount(r.Summary); n > lim.SummaryMaxWords {
		return r, fmt.Errorf("%w: summary is %d words, limit %d", ErrInvalid, n, lim.SummaryMaxWords)
	}

	// R1: no long verbatim run from any source excerpt.
	if overlap, ok := VerbatimOverlap(r.Summary, items, lim.VerbatimOverlapMaxWords); ok {
		return r, fmt.Errorf("%w: %d-word verbatim overlap with a source: %q",
			ErrInvalid, lim.VerbatimOverlapMaxWords, overlap)
	}

	// R2: quote count and length.
	quotes := QuotedSpans(r.Summary)
	if len(quotes) > lim.QuotesMax {
		return r, fmt.Errorf("%w: %d quoted spans, limit %d", ErrInvalid, len(quotes), lim.QuotesMax)
	}
	for _, q := range quotes {
		if n := normalize.WordCount(q); n >= lim.QuoteMaxWords {
			return r, fmt.Errorf("%w: quote is %d words, limit %d", ErrInvalid, n, lim.QuoteMaxWords)
		}
	}

	// Tags outside the vocabulary are dropped, not fatal.
	r.Tags = filterTags(r.Tags)

	// Takeaways are capped, not required.
	if len(r.Takeaways) > lim.TakeawaysMax {
		r.Takeaways = r.Takeaways[:lim.TakeawaysMax]
	}
	r.Takeaways = trimEach(r.Takeaways)

	switch strings.ToLower(strings.TrimSpace(r.Confidence)) {
	case ConfidenceHigh, ConfidenceMedium, ConfidenceLow:
		r.Confidence = strings.ToLower(strings.TrimSpace(r.Confidence))
	default:
		// An unparseable confidence is treated as the cautious value rather
		// than as a hard failure, so the story survives but gets demoted.
		r.Confidence = ConfidenceLow
	}

	return r, nil
}

// VerbatimOverlap reports the first run of n consecutive words that the summary
// shares with any source excerpt, and whether one was found.
//
// This is the R1 check that a prompt alone cannot guarantee. Comparison is on
// normalized words — lowercased, stripped of punctuation — so that changing
// only the punctuation around a copied sentence does not evade it.
func VerbatimOverlap(summary string, items []model.Item, n int) (string, bool) {
	if n <= 0 {
		return "", false
	}
	sum := words(summary)
	if len(sum) < n {
		return "", false
	}

	// Shingle every excerpt once, then scan the summary against the set.
	shingles := map[string]bool{}
	for _, item := range items {
		src := words(item.Excerpt)
		for i := 0; i+n <= len(src); i++ {
			shingles[strings.Join(src[i:i+n], " ")] = true
		}
	}
	if len(shingles) == 0 {
		return "", false
	}

	for i := 0; i+n <= len(sum); i++ {
		if run := strings.Join(sum[i:i+n], " "); shingles[run] {
			return run, true
		}
	}
	return "", false
}

// words normalizes text into comparable tokens: lowercase, letters and digits
// only. Hyphens and apostrophes are dropped so "open-source" and "open source"
// compare as the same two words.
func words(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// quotePairs are the opening/closing delimiters treated as quotation.
var quotePairs = [][2]rune{
	{'"', '"'},
	{'“', '”'}, // curly double quotes
	{'‘', '’'}, // curly single quotes
	{'«', '»'}, // guillemets
}

// QuotedSpans returns the text inside each quoted span in s.
//
// Straight and curly quotes both count: a model that emits typographic quotes
// is still quoting, and R2 caps quotation regardless of which glyph is used.
func QuotedSpans(s string) []string {
	var out []string
	rs := []rune(s)

	for i := 0; i < len(rs); i++ {
		for _, pair := range quotePairs {
			if rs[i] != pair[0] {
				continue
			}
			// Find the matching close after this opener.
			for j := i + 1; j < len(rs); j++ {
				if rs[j] == pair[1] {
					if inner := strings.TrimSpace(string(rs[i+1 : j])); inner != "" {
						out = append(out, inner)
					}
					i = j
					break
				}
			}
			break
		}
	}
	return out
}

func filterTags(tags []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, t := range tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if AllowedTags[t] && !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

func trimEach(ss []string) []string {
	var out []string
	for _, s := range ss {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// trimRunes truncates on a rune boundary, not a byte boundary.
func trimRunes(s string, max int) string {
	rs := []rune(s)
	if len(rs) <= max {
		return s
	}
	return strings.TrimSpace(string(rs[:max]))
}

// DemoteTier lowers a story one tier. PROMPTS.md requires this when the model
// reports low confidence: an uncertain summary should not lead the site.
func DemoteTier(tier string) string {
	switch tier {
	case model.TierMajor:
		return model.TierNotable
	case model.TierNotable:
		return model.TierMinor
	default:
		return model.TierMinor
	}
}
