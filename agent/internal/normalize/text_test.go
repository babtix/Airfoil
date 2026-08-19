package normalize

import (
	"strings"
	"testing"
)

// Invisible runes, written numerically so the test source stays readable and
// free of characters a Go source file may not contain.
var (
	nbsp = string(rune(0x00A0))
	zwsp = string(rune(0x200B))
	bom  = string(rune(0xFEFF))
)

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain text", "Just text", "Just text"},
		{"simple tags", "<p>Hello world</p>", "Hello world"},
		{"adjacent blocks do not merge words", "<p>one</p><p>two</p>", "one two"},
		{"inline tags", "context <b>window</b> doubled", "context window doubled"},
		{"attributes", `<a href="https://x.com" class="link">click</a>`, "click"},
		{"self closing", "line<br/>break", "line break"},
		{"entities", "AT&amp;T &lt;tag&gt; &quot;quoted&quot;", `AT&T <tag> "quoted"`},
		{"nbsp becomes a space", "a&nbsp;b", "a b"},
		{"script body is dropped", `a<script>var x = "<b>";</script>b`, "a b"},
		{"style body is dropped", "a<style>.x { color: red }</style>b", "a b"},
		{"uppercase script tag", "a<SCRIPT>bad()</SCRIPT>b", "a b"},
		{"script with attributes", `a<script type="text/javascript">x()</script>b`, "a b"},
		{"whitespace collapses", "one\n\n\ttwo   three", "one two three"},
		{"unterminated tag", "text<div", "text"},
		{"unclosed script drops the rest", "keep<script>drop this", "keep"},
		{"empty", "", ""},
		{"tags only", "<p></p>", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripHTML(tt.in); got != tt.want {
				t.Errorf("StripHTML(%q)\n got %q\nwant %q", tt.in, got, tt.want)
			}
		})
	}
}

// R1: nothing longer than the cap may leave this function, whatever the input.
func TestExcerptNeverExceedsCap(t *testing.T) {
	const limit = 300

	inputs := []string{
		strings.Repeat("word ", 500),
		strings.Repeat("a", 1000),                    // no spaces to break on
		strings.Repeat("<p>paragraph text</p>", 100), // markup inflates raw length
		strings.Repeat("héllo wörld ", 200),
		strings.Repeat("&amp;", 400), // entities expand on decode
		strings.Repeat("日本語", 400),
		"short",
		"",
	}

	for _, in := range inputs {
		got := Excerpt(in, limit)
		if n := len([]rune(got)); n > limit {
			t.Errorf("Excerpt(%.30q...) returned %d runes, want <= %d", in, n, limit)
		}
	}
}

func TestExcerpt(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"under the cap is untouched", "Short summary.", 300, "Short summary."},
		{"html is stripped first", "<p>Short <b>summary</b>.</p>", 300, "Short summary."},
		{"exactly at the cap", "abcde", 5, "abcde"},
		{"cuts on a word boundary", "alpha bravo charlie delta", 18, "alpha bravo…"},
		{"hard cut when there is no word boundary", "aaaaaaaaaaaaaaaaaaaa", 10, "aaaaaaaaa…"},
		{"trailing punctuation is trimmed", "alpha bravo, charlie", 15, "alpha bravo…"},
		{"zero cap yields nothing", "anything", 0, ""},
		{"negative cap yields nothing", "anything", -1, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Excerpt(tt.in, tt.max); got != tt.want {
				t.Errorf("Excerpt(%q, %d)\n got %q\nwant %q", tt.in, tt.max, got, tt.want)
			}
		})
	}
}

func TestCleanTitle(t *testing.T) {
	tests := []struct {
		name   string
		title  string
		source string
		want   string
	}{
		{"unchanged", "Anthropic ships Claude Opus 5", "Anthropic", "Anthropic ships Claude Opus 5"},

		{"pipe suffix", "Claude Opus 5 is out | TechCrunch", "TechCrunch", "Claude Opus 5 is out"},
		{"pipe suffix without a matching source", "Claude Opus 5 is out | TechCrunch", "", "Claude Opus 5 is out"},
		{"dash suffix matching the source", "Claude Opus 5 is out - The Verge", "The Verge", "Claude Opus 5 is out"},
		{"em dash suffix matching the source", "Claude Opus 5 is out — The Verge", "The Verge", "Claude Opus 5 is out"},
		{"source match is case insensitive", "Claude Opus 5 is out - the verge", "The Verge", "Claude Opus 5 is out"},

		// A dash is not a reliable separator, so an unmatched one must survive.
		{"dash that is part of the title", "Llama 4 - faster and cheaper", "Meta AI", "Llama 4 - faster and cheaper"},
		{"hyphenated words survive", "State-of-the-art results", "arXiv", "State-of-the-art results"},

		{"arxiv suffix", "Scaling Laws for Context. (arXiv:2401.12345v1 [cs.LG])", "arXiv cs.LG", "Scaling Laws for Context."},
		{"arxiv suffix with spaces", "Some Paper (arXiv: 2401.12345)", "arXiv", "Some Paper"},

		{"entities are decoded", "AT&amp;T ships a model", "", "AT&T ships a model"},
		{"whitespace collapses", "  Too   many\n spaces  ", "", "Too many spaces"},
		{"trailing separator", "Title -", "", "Title"},

		{"empty stays empty", "", "Anything", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CleanTitle(tt.title, tt.source); got != tt.want {
				t.Errorf("CleanTitle(%q, %q)\n got %q\nwant %q", tt.title, tt.source, got, tt.want)
			}
		})
	}
}

func TestCollapseSpace(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"runs collapse", "a   b", "a b"},
		{"newlines and tabs", "a\n\tb", "a b"},
		{"ends are trimmed", "  a b  ", "a b"},
		{"non breaking space", "a" + nbsp + "b", "a b"},
		{"zero width space", "a" + zwsp + "b", "a b"},
		{"byte order mark", bom + "a b", "a b"},
		{"empty", "", ""},
		{"whitespace only", "   \n ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CollapseSpace(tt.in); got != tt.want {
				t.Errorf("CollapseSpace(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestWordCount(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"one", 1},
		{"one two three", 3},
		{"  spaced   out  ", 2},
		{"line\nbreaks\tcount", 3},
	}

	for _, tt := range tests {
		if got := WordCount(tt.in); got != tt.want {
			t.Errorf("WordCount(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}
